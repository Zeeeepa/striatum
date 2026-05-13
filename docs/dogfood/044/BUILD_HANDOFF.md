author: implementer-codex-1

# Dogfood-044 Build Handoff — RFC 0040 V1.5 Daemon-Side Dispatch + Composite Tools + Watcher

Run: `run_4fbb957eccfd4fc0aaaf91bc91b37c30`
Branch: `striatum/dogfood-044-rfc-0040-v1-5`
Workflow: `docs/dogfood/044/workflow.json` — 9-job single-track for RFC
0040 V1.5 (F1-F6 codex findings from dogfood-040)

This handoff consolidates the implementation HANDOFF
(`docs/dogfood/044/build/HANDOFF.md`) into the combined per-finding
narrative plus the three build review verdicts. The per-finding HANDOFF
remains authoritative for file-level detail.

## Scope

RFC 0040 V1.5 fixes only the six dogfood-040 follow-up findings (F1-F6)
deferred by the original cycle-exhaustion call (D-?? on dogfood-040
iteration 2). No new public CLI verbs, no new MCP tool names, no daemon
RPC envelope-v1 changes. `mcp.py` keeps JSON-RPC framing and
`tools/list`; the per-call dispatch body moves into a new daemon-side
helper.

## Per-finding implementation

### F1 — Daemon MCP `tools/call` dispatch through the method registry

- New `src/striatum/daemon_pg/mcp_dispatch.py::dispatch_mcp_tool_call`
  owns lookup, capability authorization, envelope build, and routing
  through `DaemonRpcRouter.handle(...)`. The previous stub that
  returned a fake `ok: true` is gone.
- `src/striatum/mcp.py::DaemonRpcServer.call_daemon_tool` parses
  `name`/`arguments`/`token`/`request_id` and delegates to the helper.
  Constructor gained `repo_root` and `substrate_schema` so the helper
  can spin up a router when one is not supplied.
- `src/striatum/daemon_rpc/server.py::DaemonRpcRouter.handle` accepts
  `transport` (default `"rpc"`; MCP passes `"mcp"`) and
  `require_handshake` (default `True`; MCP passes `False` because the
  token-gated bridge does not run `daemon.hello`).
- Audit rows are post-dispatch: unknown methods and authorization
  denials emit one `transport="mcp"` deny row from the helper; allowed
  calls emit exactly one row from
  `DaemonRpcRouter._record_and_return` carrying the real handler exit
  code. MCP response shape `{content, structuredContent, isError}` is
  preserved; `structuredContent` carries `ok`, `method`, `audit_id`,
  and `data` on success, or `error`/`error_message` on failure.

### F2 / F3 — `dogfood.publish_on_behalf` atomicity + verdict semantics

- `src/striatum/dogfood/operator_tools.py::publish_on_behalf` validates
  inputs, then runs every authoritative mutation inside one outer
  `with transaction(conn):` block. Transaction-free helpers
  `_ack_on_behalf_locked`, `_publish_artifact_locked`,
  `_record_verdict_locked`, and `_complete_locked` execute the four
  composite steps without nesting transactions.
- Review jobs require `verdict` up front; `accept`,
  `accept_with_findings`, `needs_revision`, and `reject` are validated
  against the same enum the direct-Python helper accepts. If the
  published artifact is kind `finding`, its id defaults to
  `findings_artifact_id`; otherwise the caller must pass an explicit
  `findings_artifact_id`, and `_validate_findings_artifact` confirms
  the artifact already exists for the same job.
- `_record_verdict_locked` inserts the `verdicts` row, emits
  `verdict.recorded`, and routes through `_complete_review_job` /
  `request_revision_for_cycle` / `_fail_review_job` depending on the
  verdict. The returned result includes `artifact_id`,
  `findings_artifact_id`, and `verdict_id`.
- On success, exactly one `dogfood.publish_on_behalf` event is inserted
  in-transaction with `composition_steps` covering ack, publish, and
  verdict or completion. On failure (`ArtifactError` /
  `InvalidTransitionError` / `sqlite3.Error`) the transaction rolls
  back and `_record_publish_on_behalf_failure` writes a best-effort
  `dogfood.publish_on_behalf_failed` event tagged
  `outcome: "rolled_back"`. `surgical_recovery` keeps its
  single-transaction shape; its V1.5 change is purely that daemon MCP
  dispatch now reports the real outcome.

### F4 — Watcher invocation in the daemon supervisor lifecycle

- New `src/striatum/process_progress.py::progress_loop_once` runs one
  bounded supervised-progress pass per repository, joined to the
  `runs` table so only attached supervisors under running/paused runs
  tick.
- `src/striatum/daemon.py::daemon_sweep_once` calls
  `progress_loop_once` inside `connect_repo(repo)` immediately before
  per-run auto-sweep work, and folds the result into the sweep return
  payload as `"progress"`. No per-supervisor background threads;
  lifecycle is owned by the existing synchronous sweep loop.
- The loop materializes each row as a `SupervisedProgressTarget` and
  ticks `SupervisedProgressWatcher`, whose heartbeat callback calls
  `striatum.cli.mutations.heartbeat` on the same repo connection.
- Metadata-only events emitted:
  `supervisor.progress_watcher_heartbeat`,
  `supervisor.progress_watcher_idle`,
  `supervisor.progress_watcher_lost`. Log contents are never read.

### F5 — Race / signal hardening

- `ProcessProgressConfig.startup_grace_seconds` defaults to 60 s.
  Within grace, a missing scratch path returns `waiting_for_log` with
  no warning. The watcher catches `FileNotFoundError`/`OSError` while
  scanning `*.log` files, so rotated logs (`packet-0001.log` →
  `packet-0002.log`) follow without recreating the target.
- The loop accepts a `should_stop` predicate and checks it between
  supervisors so SIGTERM cannot start a new heartbeat after shutdown.
- `progress_advisory_lock(repo, job_id=...)` is shared with
  `surgical_recovery`: watcher tick returns `lock_busy`, surgical
  recovery returns `progress_lock_busy`. Neither revives stale work.
- PID-reuse guard: `process_start_time(pid)` is consulted before each
  tick. A mismatch versus the stored `pid_start_time` flips the row
  to `state='lost'`, emits `supervisor.progress_watcher_lost`, and
  skips heartbeat.

### F6 — End-to-end execution-path tests

- `tests/test_mcp_dogfood_e2e.py` (new) drives MCP `tools/call`
  round-trips for `dogfood.publish_on_behalf` covering completion and
  review-verdict paths. Marked `pytest.mark.multi_repo`; skips when
  the PG harness is unavailable.
- `tests/test_supervised_progress_watcher.py` extended with
  `test_progress_loop_once_heartbeats_attached_supervisor` and
  `test_progress_loop_once_refuses_pid_identity_mismatch` covering
  loop wiring and the PID-reuse refusal.
- `tests/test_mcp_mutation_capabilities.py` continues to assert
  `tools/list`, denial, and unknown-method audit behavior against the
  new dispatcher. `tests/test_dogfood_publish_on_behalf.py`,
  `tests/test_mcp_capability_scope_e2e.py`, and the `_harness`
  fixtures were updated to match the new dispatch contract.

## Build review verdicts

Three-way build review with distinct postures:

| Reviewer | Verdict | Severity |
|----------|---------|----------|
| codex | needs_revision | high |
| claude | accept_with_findings | medium |
| gemini | accept | low |

**Codex `needs_revision` overridden via D098**
(`dec_242ea0b026d547c9baad9b353b149033`, `accepted_with_follow_up`).
The override applies the same logic as D095 / D096 / D097: the codex
implementer + codex reviewer pairing produces convergent-blind-spot
findings that 2-of-3 cross-lane majority overrides. Dogfood-044 is the
**fourth** independent recurrence of the same anti-pattern. Codex
findings absorbed into RFC 0040 V1.6 follow-up (TODO item 28).

## Test status

`.venv/bin/python -m pytest tests/ -k "mcp or progress or dispatch or
daemon_pg" --tb=line` reports **42 passed, 10 skipped** in 13.93s. The
10 skips are `pytestmark = pytest.mark.multi_repo` fixtures requiring
the multi-repo PostgreSQL harness; they collect cleanly and skip with
no failures locally. Slice highlights: `test_daemon_pg.py` 9 passed,
`test_mcp_mutation_capabilities.py` 4 passed,
`test_supervised_progress_watcher.py` 7 passed,
`test_mcp_dogfood_e2e.py` 2 skipped,
`test_mcp_capability_scope_e2e.py` 8 skipped.

## Backward compatibility

- Daemon RPC envelope-v1 (`RpcEnvelope`, `RpcResponse`, `daemon.hello`,
  `daemon.welcome`, `METHODS_ETAG`) unchanged. MCP `tools/list`, tool
  names, and argument shapes unchanged. `dogfood.publish_on_behalf`
  and `dogfood.surgical_recovery` keep their registered capabilities
  (`write` and `surgical_recovery`).
- Direct-Python `publish_on_behalf` callers see the same
  `{ok, status, ...}` success shape and the same
  `{ok:false, status:"refused", error}` refusal shape; new keys
  (`findings_artifact_id`, `verdict_id`) are additive.
- The only observable change for an allowed daemon MCP `tools/call`
  is that it now executes the real registered method and reports the
  real handler result instead of a stubbed `ok: true`.

## Known V1.5 follow-up gaps

- Multi-repo MCP e2e tests skip without the PG harness; smoke-only
  environments will not exercise them.
- Synthesis named `src/striatum/daemon_supervisor/progress_loop.py`,
  but the packet's write scope authorized
  `src/striatum/process_progress.py`; the loop ships there. Future
  callers grafting it under `daemon_supervisor/` must update the
  import in `src/striatum/daemon.py`.
- Surgical-recovery composite keeps its single-transaction shape; an
  explicit rollback-event emission consistent with publish-on-behalf
  is left for a future cycle.
- Codex needs_revision delta from dogfood-044 review (RFC 0040 V1.6
  follow-up, TODO item 28).

## Pointers

- Per-finding implementation HANDOFF:
  `docs/dogfood/044/build/HANDOFF.md`
- Build review verdicts:
  `docs/dogfood/044/review/build/codex/REVIEW.md`,
  `docs/dogfood/044/review/build/claude/REVIEW.md`,
  `docs/dogfood/044/review/build/gemini/REVIEW.md`
- Decision: `docs/dogfood/044/decisions/D098_cycle_exhaustion.md`
- Operator notes: `docs/dogfood/044/PHASE_1_OPERATOR_NOTES.md`
- Design synthesis: `docs/dogfood/044/DESIGN_SYNTHESIS.md`
- Operator report (per-intervention narrative):
  `docs/dogfood/044/OPERATOR_REPORT.md`
- `CHANGELOG.md` v1.33.0 — promotion entry.
- `docs/TODO.md` items 20 (✅ done) and 28 (V1.6 follow-up).
- `docs/rfcs/README.md` RFC 0040 row — status bumped to
  `accepted (V1 + V1.5 daemon-dispatch + watcher landed)`.
