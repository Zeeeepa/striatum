# RFC 0040 V1.5 Design: Daemon MCP Dispatch, Composite Atomicity, Progress Watcher Hardening

author: designer-unknown-model-001
date: 2026-05-13
status: draft

## Objective

RFC 0040 V1 landed the operator-side dogfood harness but dogfood-040 left six accepted follow-up findings. V1.5 should repair those gaps without changing daemon RPC envelope-v1, without adding hosted semantics, and without changing existing local web chat dispatch for `generate_workflow_preview`, `generate_workflow_write`, or the RPC-thin dogfood lifecycle tools.

The implementation target is the current Python daemon stack. The prompt mentions `src/striatum/process_progress.py`; that file does not exist in this checkout. The existing watcher implementation is `src/striatum/daemon_supervisor/progress_watcher.py`.

## F1: MCP `tools/call` Must Dispatch Through The Method Registry

The broken entry point is `DaemonRpcServer.call_daemon_tool()` in `src/striatum/mcp.py:554`. It reads `name`, checks `METHOD_REGISTRY`, authorizes, appends an audit row, and appends `rpc_request_log`; after successful authorization it currently returns `_daemon_tool_result(..., ok=True)` at `src/striatum/mcp.py:632` without invoking the method. The registry it already consults is `METHOD_REGISTRY` from `src/striatum/daemon_rpc/registry.py:102`, whose dogfood entries are declared at `src/striatum/daemon_rpc/registry.py:77`.

The real dispatcher already exists in `DaemonRpcRouter` in `src/striatum/daemon_rpc/server.py:51`. Its `_route()` method maps registry methods to CLI routes, apply routes, cross-repo routes, and dogfood composites at `src/striatum/daemon_rpc/server.py:146`; dogfood methods are dispatched by `_route_dogfood()` at `src/striatum/daemon_rpc/server.py:200`.

Design change: `DaemonRpcServer.call_daemon_tool()` should keep the existing MCP authorization/audit row as the visibility/authorization record, then construct an `RpcEnvelope` with `method=name`, `params=arguments`, `capability_token=token_value`, and `request_id=f"{request_id}:dispatch"` or another deterministic child request id. It should then call a `DaemonRpcRouter(pg_conn=self.pg_conn, repo_root=resolved_repo_root).handle(envelope, connection_id="mcp")`. The child id avoids colliding with the pre-existing MCP request-log row, because `append_request_log()` refuses duplicate `request_id` values in `src/striatum/daemon_rpc/request_log.py:38`.

The return shape should preserve MCP compatibility: `content`, `structuredContent`, and `isError`. `structuredContent` should include the outer MCP authorization `audit_id`, plus the router response data or error. This keeps the existing "allowed" audit row from `src/striatum/mcp.py:605` and appends the actual dispatch audit/request-log rows through `DaemonRpcRouter._record_and_return()` at `src/striatum/daemon_rpc/server.py:104`. Authorization remains rechecked by the router, preserving the "tools/list is not a security boundary" rule.

Repository resolution must not trust arbitrary paths in MCP arguments. For single-repo methods, `repository_id` should continue to come from arguments as today at `src/striatum/mcp.py:571`; the router can resolve the active repo root from `striatumd.repositories` through `_repo_root_for()` at `src/striatum/daemon_rpc/server.py:129`. If the requested repository does not match the router root, the existing `repo_not_registered` failure applies.

Backward compatibility: `LocalRpcServer.call_tool()` at `src/striatum/mcp.py:433` and `src/striatum/web/chat_tools.py:536` stay untouched. Web chat tools still route through `striatum.api.invoke()` in `_tool_dogfood_lifecycle()` at `src/striatum/web/chat_tools.py:835`, so `generate_workflow_*` and the local service mutation gate keep their current behavior.

## F2/F3: Composite Atomicity And Verdict Recording

The composite code path is `src/striatum/dogfood/operator_tools.py`. `publish_on_behalf()` currently looks up the active lease/message, optionally runs `ack_work()`, publishes the artifact, records a review verdict or completes a non-review job, then writes a `dogfood.publish_on_behalf` event at `src/striatum/dogfood/operator_tools.py:166`. The sequence is not wrapped in one transaction; `ack_work()` opens its own transaction at `src/striatum/cli/mutations.py:462`, artifact publication has its own behavior, and `complete_job()` / `record_review_verdict()` mutate live state after the artifact record exists.

V1.5 should make `dogfood.publish_on_behalf` all-or-nothing for live SQLite state. Add a new `publish_on_behalf_atomic()` helper or refactor the existing helper so validation happens first, then all state mutations run inside one `transaction(conn)` block. Inside that transaction, use lower-level mutation helpers that do not open nested transactions, or extract transaction-free internal helpers from `ack_work`, `publish_artifact`, `record_review_verdict`, and `complete_job`. The tables involved are repo-local `leases`, `queue_messages`, `jobs`, `artifacts`, `verdicts`, and `events`; these are the same live workflow-state tables named in `docs/SPEC.md`.

For review jobs, verdict recording is mandatory and should use the artifact id just published unless `findings_artifact_id` is explicitly supplied. The current code has the right intent at `src/striatum/dogfood/operator_tools.py:137`, but V1.5 should reject a review invocation unless both `verdict` and either the published artifact kind is `finding` or `findings_artifact_id` points at an existing artifact for the same job. The result must include `verdict_id` and `findings_artifact_id`, and the event payload must include the `verdict` step in `composition_steps`.

Failure semantics: validation failures return the existing `{ok:false,status:"refused",error:{...}}` envelope before mutation. Runtime failures inside the transaction roll back repo-local state and return `ok:false` through the daemon RPC response. The outer MCP authorization audit row remains append-only; the router's dispatch audit/request-log row records the error response. No envelope-v1 schema change is needed because the response body already carries method-specific structured data.

`dogfood.surgical_recovery()` is closer to the desired model: it validates, takes `progress_advisory_lock()`, and restores `leases`, `queue_messages`, `jobs`, `process_supervisors`, `process_supervisor_pointers`, and `events` inside one transaction at `src/striatum/dogfood/operator_tools.py:328`. Keep that pattern. The V1.5 change is to ensure any MCP dispatch error returns through the router instead of being hidden behind the current fake success path.

## F4: Start And Stop The Watcher With Supervisor Lifecycle

The current repo-local supervisor lifecycle starts in `supervise_start()` at `src/striatum/supervisor.py:47`, creates the scratch directory and FIFO at `src/striatum/supervisor.py:92`, launches the process at `src/striatum/supervisor.py:148`, and marks the row attached at `src/striatum/supervisor.py:197`. It stops in `supervise_stop()` at `src/striatum/supervisor.py:307`.

The daemon-owned supervisor metadata table already exists as `striatumd.daemon_supervisors` in `src/striatum/daemon_pg/sql/0002_rpc_supervision_apply.sql:15`, but there is no daemon supervisor service in `src/striatum/daemon_pg/` that starts watcher tasks. For V1.5, add a small lifecycle manager module, for example `src/striatum/daemon_supervisor/lifecycle.py`, that wraps `supervise_start()` and `supervise_stop()` for daemon-mediated paths. After `supervise_start()` returns `state="attached"`, it should create a `SupervisedProgressTarget` and start `watch_progress()` from `src/striatum/daemon_supervisor/progress_watcher.py:331` on a daemon-owned thread or task with a per-supervisor stop event.

The watcher should call `heartbeat()` from `src/striatum/cli/mutations.py:490` using a fresh repo-local connection per tick or a carefully scoped connection factory. Do not share one SQLite connection across a long-lived thread. The heartbeat callback must pass `extend_seconds` so the configured `heartbeat_extend_seconds` in `ProgressWatcherConfig` at `src/striatum/daemon_supervisor/progress_watcher.py:31` is honored.

Shutdown path: `supervise_stop()` must signal the watcher stop event before or immediately after SIGTERM, wait no longer than one poll interval plus a small grace, and then continue the existing stop behavior. If the supervised process is already lost, the watcher should also exit after `process_lost`, matching `watch_progress()` at `src/striatum/daemon_supervisor/progress_watcher.py:350`. Add a `supervisor.progress_watcher_started` and `supervisor.progress_watcher_stopped` event so `why` and tests can distinguish "watcher absent" from "watcher idle".

## F5: Race And Signal Hardening

There are four race windows to close.

First, log rotation and per-packet log rollover: `check_progress_once()` currently stats only `target.log_path` at `src/striatum/daemon_supervisor/progress_watcher.py:306`, while `SupervisedProgressWatcher.tick()` can scan newest `*.log` under `scratch_path` through `newest_log_mtime()` at `src/striatum/daemon_supervisor/progress_watcher.py:88`. V1.5 should standardize on scratch-directory scanning for production targets so a new `packet-NNNN.log` is seen without recreating the watcher. Keep explicit `log_path` only for narrow unit tests.

Second, watcher start before first write: a missing log currently returns `no_log` at `src/striatum/daemon_supervisor/progress_watcher.py:186` or `log_missing` at `src/striatum/daemon_supervisor/progress_watcher.py:306`. That should be normal during a startup grace window. Add `startup_grace_seconds` to `ProgressWatcherConfig`; before grace expires, missing logs return `waiting_for_log` without warning. After grace, emit one warning and continue polling rather than terminating.

Third, SIGTERM during heartbeat: `watch_progress()` loops until `stop_event` is set but cannot interrupt an in-flight heartbeat at `src/striatum/daemon_supervisor/progress_watcher.py:343`. The heartbeat callback should be bounded: open a fresh connection, call `heartbeat()`, close it, and catch `InvalidTransitionError` / SQLite operational errors. If the stop event is set before the callback starts, skip the heartbeat. If it is set after the callback starts, let the short transaction finish and then exit before the next sleep.

Fourth, surgical recovery versus watcher heartbeat: this already has the right guard shape. `SupervisedProgressWatcher.tick()` takes `progress_advisory_lock()` before heartbeat at `src/striatum/daemon_supervisor/progress_watcher.py:203`, and `surgical_recovery()` takes the same lock at `src/striatum/dogfood/operator_tools.py:336`. Keep the lock keyed by job id and add tests for both directions: watcher sees `lock_busy`, and surgical recovery refuses with `progress_lock_busy` while a heartbeat is in progress.

Also tighten PID identity checks. The watcher currently tests only `os.kill(pid, 0)` through `process_alive()` at `src/striatum/daemon_supervisor/progress_watcher.py:111`. Production targets should carry `pid_start_time` and verify it before heartbeating, matching the surgical recovery identity checks in `src/striatum/dogfood/operator_tools.py:718`.

## F6: End-To-End Tests

Existing coverage is useful but too shallow for the failure mode. `tests/test_mcp_mutation_capabilities.py:46` proves `tools/call` reauthorizes and audits denial with monkeypatches; it does not prove dispatch changes repo-local state. `tests/test_dogfood_publish_on_behalf.py:156` exercises the Python helper directly; it bypasses MCP. `tests/test_supervised_progress_watcher.py:135` verifies watcher heartbeat with a direct `SupervisedProgressWatcher.tick()`; it does not prove lifecycle invocation.

Add these tests:

1. `tests/test_mcp_dogfood_e2e.py::test_mcp_publish_on_behalf_dispatches_and_completes_job`: use the existing fixture helpers from `tests/test_dogfood_publish_on_behalf.py` or move them into `tests/_harness/repos.py`. Bootstrap repo-local state, issue a real daemon token with `write` for the repo, call `tests/_harness/mcp.py:McpClient.call_tool("dogfood.publish_on_behalf", ...)`, then assert the job is `completed`, the artifact row exists, and both an MCP allowed audit row plus a dispatch request-log row exist.

2. `test_mcp_publish_on_behalf_records_review_verdict`: same path for a review job. Assert `verdicts` row exists and `structuredContent.verdict_id` is present. This closes F3.

3. `test_mcp_publish_on_behalf_rolls_back_on_complete_failure`: create a required-artifact mismatch or otherwise force completion failure after ack would have occurred. Assert `jobs`, `queue_messages`, `leases`, `artifacts`, and `verdicts` remain in pre-call state, and assert request-log response is `ok:false`. This is the atomicity acceptance test.

4. `test_mcp_tools_call_dispatches_unknown_and_denied_without_side_effects`: keep the existing denial tests but assert no repo-local mutation happened. This protects the new router bridge.

5. `tests/test_supervisor_progress_lifecycle.py::test_supervise_start_launches_progress_watcher`: use a lightweight process lane that writes a log file under its supervisor scratch directory. Start through the daemon-mediated lifecycle wrapper, claim work, touch/write the log, wait one short poll interval, and assert the lease `expires_at` moved and a `lease.heartbeat` event exists.

6. `test_progress_watcher_handles_rotated_logs_and_startup_grace`: create `packet-0001.log`, then `packet-0002.log` with a newer mtime under the same scratch root and assert the watcher heartbeats from newest log. Start with no log and assert `waiting_for_log` until grace expires.

7. Add a smoke-level test to the existing daemon/MCP harness, not the local web chat tests. `tests/_harness/mcp.py:23` already wraps `DaemonRpcServer.call_daemon_tool`; extend it only after `call_daemon_tool` really dispatches through `DaemonRpcRouter`.

Run scope for implementation should be focused first, then full: `pytest tests/test_mcp_mutation_capabilities.py tests/test_dogfood_publish_on_behalf.py tests/test_dogfood_surgical_recovery.py tests/test_supervised_progress_watcher.py tests/test_mcp_dogfood_e2e.py`, followed by `make test`.

## Acceptance Criteria

- MCP `tools/call` for registered daemon methods invokes `DaemonRpcRouter`, changes state for mutating methods, and reports router failures instead of fake success.
- The existing MCP authorization audit row remains, and actual dispatch appends its own audit/request-log evidence.
- `dogfood.publish_on_behalf` is atomic across ack, artifact publication, verdict-or-complete, and event insertion.
- Review publish-on-behalf always records a verdict and returns the verdict id.
- Daemon-mediated supervisor start launches one watcher per attached supervisor, and stop/lost paths terminate it.
- Watcher production mode scans supervisor scratch logs, tolerates startup missing logs, handles rotation, bounds heartbeat shutdown, and validates PID identity.
- End-to-end tests prove MCP tool call to dispatch to state change to audit row, plus rollback on composite failure.
