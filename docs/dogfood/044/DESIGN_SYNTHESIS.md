---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/044/design/codex/DESIGN.md", "docs/dogfood/044/design/claude_code/DESIGN.md", "docs/dogfood/044/design/gemini/DESIGN.md"]
---

author: designer-unknown-model-002

# RFC 0040 V1.5 Design Synthesis

## Implementation Boundary

RFC 0040 V1.5 fixes only the six dogfood-040 follow-up findings: daemon MCP dispatch, composite atomicity and review verdict recording, supervised-progress watcher invocation and hardening, and end-to-end tests. It does not add public CLI verbs, does not change daemon RPC envelope-v1, and does not change the existing web chat tool names, MCP tool names, or lifecycle argument shapes.

The build should treat `src/striatum/mcp.py::DaemonRpcServer.call_daemon_tool` as the external MCP entry point, but move the allowed-call dispatch body into a new daemon-side helper `src/striatum/daemon_pg/mcp_dispatch.py::dispatch_mcp_tool_call`. That satisfies the daemon-owned boundary while keeping JSON-RPC framing and `tools/list` compatibility in `mcp.py`.

## F1: Daemon MCP Dispatch Wiring

Add `src/striatum/daemon_pg/mcp_dispatch.py` with:

```python
def dispatch_mcp_tool_call(
    *,
    pg_conn: Any,
    router: DaemonRpcRouter,
    name: str,
    arguments: JsonObject,
    token: str | None,
    request_id: str,
) -> JsonObject:
    ...
```

`DaemonRpcServer.call_daemon_tool` calls this helper after parsing `name`, `arguments`, `token`, and `request_id`. The helper looks up `METHOD_REGISTRY[name]`, authorizes with `authorize(...)`, and for allowed methods builds `RpcEnvelope(schema_version="striatum.daemon_rpc.envelope.v1", request_id=request_id, method=name, params=arguments, capability_token=token)`. It then routes through `DaemonRpcRouter.handle(envelope, connection_id="mcp", transport="mcp", require_handshake=False)`.

`DaemonRpcRouter.handle` needs two small compatibility parameters: `transport` flows into `_record_and_return` instead of the hardcoded `"rpc"`, and `require_handshake=False` bypasses the `daemon.hello` first-message rule only for the already-token-gated MCP bridge. The actual registry handle remains `METHOD_REGISTRY[name]`; the dogfood composites execute through `DaemonRpcRouter._route_dogfood`, which already calls `publish_on_behalf` and `surgical_recovery`.

Use a unified post-dispatch audit row for allowed calls. Unknown methods and authorization denials still append one `transport: "mcp"` deny row from `dispatch_mcp_tool_call`; allowed calls append exactly one row from `DaemonRpcRouter._record_and_return` after the handler returns, with `exit_code = None` on success and the handler failure code on error. This replaces the current misleading allow-row that reports success before any method runs.

The MCP response remains `{content, structuredContent, isError}`. On success, `structuredContent` includes `ok: true`, `method`, `audit_id`, and `data: response.data`. On failure, it includes `ok: false`, `method`, `audit_id`, `error`, and `error_message`, with `isError: true`.

## F2/F3: Composite Atomicity And Verdict Recording

Refactor `src/striatum/dogfood/operator_tools.py::publish_on_behalf` so validation and lookup happen first, then every live-state mutation runs inside one outer `with transaction(conn):` block. Use transaction-free internal helpers for the four steps: `_ack_on_behalf_locked`, `_publish_artifact_locked`, `_record_verdict_locked`, and `_complete_locked`. If existing helpers still open independent transactions, extract their mutation bodies instead of nesting them.

This chooses a single SQL transaction wrapping the composite because every authoritative side effect is a repo-local SQLite row; artifact bytes already exist on disk before the operator invokes the tool, so rollback only needs to remove the artifact record, verdict row, job/message/lease state changes, and event rows.

For review jobs, require `verdict` before the transaction starts. The published artifact id becomes `findings_artifact_id` when the caller does not supply one, but only if the artifact kind is `finding`; otherwise require an explicit existing `findings_artifact_id` for the same job. The result must include `artifact_id`, `findings_artifact_id`, and `verdict_id`.

On success, insert one `dogfood.publish_on_behalf` event inside the transaction with `composition_steps` covering `ack`, `publish_artifact`, `verdict` for review jobs, and `complete` for non-review jobs. On failure, the transaction rolls back and the function returns the existing `{ok:false, status:"refused", error:{...}}` shape; a best-effort `dogfood.publish_on_behalf_failed` event may be written after rollback, but it must not claim any rolled-back step as durable.

Keep `surgical_recovery` on its current single-transaction shape. Its V1.5 change is not atomicity structure; it is that daemon MCP dispatch now reports the real success or failure instead of a stubbed success.

## F4: Watcher Invocation

Use the daemon's existing synchronous sweep loop, not per-supervisor threads. Add `src/striatum/daemon_supervisor/progress_loop.py::progress_loop_once`, and call it from `src/striatum/daemon.py::run_daemon_foreground` immediately after `daemon_sweep_once()`.

`progress_loop_once` enumerates active registered repositories from the daemon registry, opens each repo-local state DB, selects attached rows from `process_supervisors`, maps each row to `SupervisedProgressTarget`, and calls `SupervisedProgressWatcher.tick(target)`. The heartbeat callback calls `striatum.cli.mutations.heartbeat` with `extend_seconds=config.heartbeat_extend_seconds` on the same short-lived repo connection used for that tick.

This is the lifecycle hook: `run_daemon_foreground` owns startup and shutdown, and each daemon sweep owns watcher work for all currently attached supervisors. There is no per-supervisor background task to join; the join-on-shutdown path is the existing `while not stopping` loop exiting after the current bounded `progress_loop_once` pass. This is deliberate because the Python daemon is already a synchronous sweep process.

Emit metadata-only events `supervisor.progress_watcher_heartbeat`, `supervisor.progress_watcher_idle`, and `supervisor.progress_watcher_lost` from the repo-local connection when the watcher observes those statuses. Do not read log contents; only stat log metadata.

## F5: Race And Signal Hardening

Guard these race windows:

- Rotated logs: production targets pass `scratch_path` and `log_path=None`, so `newest_log_mtime` scans the newest `*.log` under the supervisor scratch tree. Catch `FileNotFoundError` and `OSError` while walking/statting files and skip files being renamed.
- First log not created yet: add `startup_grace_seconds` to `ProgressWatcherConfig`. Before grace expires, missing logs return `waiting_for_log` without a warning; after grace, emit one idle-style metadata warning and keep polling.
- SIGTERM during a tick: `run_daemon_foreground` checks `stopping` before starting each sweep. Inside `progress_loop_once`, check the stop predicate between repositories and between supervisors; do not start a new heartbeat after shutdown is requested. If SIGTERM arrives during the short heartbeat transaction, let it finish and exit before the next target.
- Surgical recovery versus heartbeat: keep the existing `progress_advisory_lock(repo, job_id=...)` in both `SupervisedProgressWatcher.tick` and `surgical_recovery`. Watcher returns `lock_busy`; surgical recovery returns `progress_lock_busy` when it cannot acquire the same lock.
- PID reuse: extend `SupervisedProgressTarget` with `pid_start_time` and validate it with `process_start_time(pid)` before heartbeating. A mismatch returns `process_identity_changed`, marks or reports the supervisor as lost, and never refreshes the lease.
- State drift: after observing fresh log mtime but before heartbeat, reload the active lease and job state. If the job is no longer running/claimed with an active lease for the same session, return `no_active_lease` or `not_heartbeatable` rather than reviving stale work.

The rotated-log behavior means the watcher follows `packet-0002.log` without recreating the supervisor target. The signal behavior is bounded by one repository/supervisor tick and one SQLite transaction, so daemon shutdown does not hang on a watcher thread.

## F6: End-To-End Tests

Add `tests/test_mcp_dogfood_e2e.py` with:

- `test_mcp_publish_on_behalf_dispatches_and_completes_job`: create a repo-local run/job, register a daemon repository/token, call `DaemonRpcServer.handle_request({"method":"tools/call", ... name:"dogfood.publish_on_behalf"})`, then assert job `completed`, artifact row exists, request log row exists with `transport: "mcp"`, and the response includes handler data.
- `test_mcp_publish_on_behalf_records_review_verdict`: same path for a review job, asserting a `verdicts` row and `structuredContent.data.verdict_id`.
- `test_mcp_publish_on_behalf_rolls_back_on_complete_failure`: force completion failure after the ack step would have run, then assert leases, queue_messages, jobs, artifacts, and verdicts match the pre-call snapshot and the request log records `ok:false`.
- `test_mcp_unknown_and_denied_tools_have_no_repo_side_effects`: preserve default-deny behavior while proving no repo-local rows changed.

Add `tests/test_supervisor_progress_lifecycle.py` with:

- `test_progress_loop_once_heartbeats_attached_supervisor`: create an attached supervisor row, active lease, and fresh `packet-0001.log`, run `progress_loop_once`, and assert `leases.expires_at` advanced plus a `lease.heartbeat` event exists.
- `test_progress_loop_once_tracks_rotated_logs`: create `packet-0001.log` and newer `packet-0002.log` under the same scratch path and assert the newer mtime drives the heartbeat.
- `test_progress_loop_once_respects_startup_grace`: no log during grace returns `waiting_for_log` and emits no warning.
- `test_progress_loop_once_refuses_pid_identity_mismatch`: mismatched `pid_start_time` does not heartbeat and reports `process_identity_changed`.
- `test_watcher_and_surgical_recovery_share_progress_lock`: cover both directions, watcher sees `lock_busy` while recovery owns the lock and recovery returns `progress_lock_busy` while heartbeat owns it.

Extend `tests/test_mcp_mutation_capabilities.py` to assert allowed `tools/call` dispatches through a fake router rather than returning the old stub. Keep the existing denial and unknown-method tests. Extend `tests/test_dogfood_publish_on_behalf.py` with the rollback fixture if the end-to-end fixture is too expensive for every local run.

The smoke harness hook is the existing `make smoke`; add only a focused pytest invocation to the build handoff, not a new Makefile target. The implementer should run:

```text
pytest tests/test_mcp_mutation_capabilities.py tests/test_mcp_dogfood_e2e.py tests/test_dogfood_publish_on_behalf.py tests/test_dogfood_surgical_recovery.py tests/test_supervised_progress_watcher.py tests/test_supervisor_progress_lifecycle.py
make test
make smoke
```

## Backward Compatibility Fixtures

The daemon RPC envelope-v1 remains unchanged: `RpcEnvelope`, `RpcResponse`, `daemon.hello`, `daemon.welcome`, and `METHODS_ETAG` semantics stay as-is. Existing MCP tool names and arguments remain unchanged, including the RFC 0040 dogfood lifecycle tools and the composite names `dogfood.publish_on_behalf` and `dogfood.surgical_recovery`.

Regression coverage that must keep passing:

- `tests/test_daemon_rpc.py` for method registry and envelope behavior.
- `tests/test_daemon_rpc_registry.py` for `surgical_recovery` capability registration.
- `tests/test_mcp_mutation_capabilities.py` for `tools/list`, denial, and unknown-method audit behavior.
- `tests/test_chat_tools.py` and `tests/test_web_chat.py` for local web chat tool visibility and `generate_workflow_*` behavior.
- `tests/test_dogfood_publish_on_behalf.py` and `tests/test_dogfood_surgical_recovery.py` for direct Python helper behavior.
- `tests/test_supervised_progress_watcher.py` for unit-level watcher behavior.

The only observable change for an allowed daemon MCP `tools/call` is that it now executes the registered method and reports the real handler result instead of returning a fake `ok: true`.
