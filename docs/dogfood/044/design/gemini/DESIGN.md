author: designer-unknown-model-001

# DESIGN: RFC 0040 V1.5 (F1-F6)

This design addresses the six findings (F1-F6) from the RFC 0040 V1 dogfooding, focusing on the systems-half implementation: daemon-side dispatch, composite tool atomicity, and the supervised-progress watcher.

## F1 — Daemon MCP `tools/call` Dispatch

**Finding:** The daemon MCP surface in `src/striatum/mcp.py` authorizes and audits `tools/call` requests but does not actually dispatch them to the method registry. It returns `ok=True` without executing the underlying logic.

**Design:**
- Modify `DaemonRpcServer.call_daemon_tool` in `src/striatum/mcp.py` to use `DaemonRpcRouter` from `src/striatum/daemon_rpc/server.py`.
- Construct an `RpcEnvelope` from the MCP `tools/call` parameters (method name, arguments, token, request_id).
- Pass the envelope to `DaemonRpcRouter.handle()`.
- Map the `RpcResponse` back to the MCP `tools/call` result shape.
- Ensure the existing authorization and auditing in `mcp.py` is either preserved or unified with `DaemonRpcRouter`'s logic to avoid duplicate work.

**Citations:**
- `src/striatum/mcp.py:567-642`: `call_daemon_tool` authorizing but returning a stub.
- `src/striatum/daemon_rpc/server.py:51`: `DaemonRpcRouter` which owns the dispatch logic.

## F2/F3 — Composite Tool Atomicity + Verdict Recording

**Finding:** The `dogfood.publish_on_behalf` composite tool in `src/striatum/dogfood/operator_tools.py` is not atomic. It calls `ack`, `publish_artifact`, and `complete_job` as separate operations. A failure mid-chain records a single "success" audit row in the daemon (because the RPC call started), but leaves the repo-local state inconsistent.

**Design:**
- Refactor `publish_on_behalf` and `surgical_recovery` in `src/striatum/dogfood/operator_tools.py` to use a single `with transaction(conn):` block covering all repo-local mutations.
- Ensure that `publish_artifact`, `record_review_verdict`, and `complete_job` are compatible with an external transaction or can be called in a way that participates in one.
- For `dogfood.publish_on_behalf`, specifically ensure it correctly records a review verdict when applicable (F2).
- The daemon RPC audit row will naturally record the overall success/failure of the composite call.

**Citations:**
- `src/striatum/dogfood/operator_tools.py:52`: `publish_on_behalf` implementation.
- `src/striatum/dogfood/operator_tools.py:197`: `surgical_recovery` implementation.
- `src/striatum/db.py`: `transaction` context manager for SQLite.
- **SQLite Tables:** `leases` (state, expires_at), `queue_messages` (state), `jobs` (state, current_lease_id), `verdicts` (for F2), and `events` (for the composite audit row).

## F4 — Watcher Invocation in Supervisor Lifecycle

**Finding:** The `SupervisedProgressWatcher` is implemented but not invoked. The daemon starts supervisors via `supervise.start` (CLI → `supervisor.py`) but does not poll their logs for heartbeats.

**Design:**
- Integrate `SupervisedProgressWatcher` into the daemon's main loop in `src/striatum/daemon.py:run_daemon_foreground`.
- On each tick of the daemon loop (after or alongside `daemon_sweep_once`), the daemon should:
    1. Query the daemon DB for all supervisors in an `attached` state.
    2. Instantiate a `SupervisedProgressWatcher` (or use a shared one).
    3. Iterate over the attached supervisors and call `watcher.tick(supervisor)`.
    4. Provide a `heartbeat_callback` that calls `striatum.db.heartbeat_lease` (or equivalent) to extend the lease.
- Ensure the watcher task is joined or stopped cleanly during daemon shutdown in `run_daemon_foreground`.

**Citations:**
- `src/striatum/daemon_supervisor/progress_watcher.py:154`: `SupervisedProgressWatcher` class.
- `src/striatum/daemon.py:862`: `run_daemon_foreground` main loop.
- `src/striatum/supervisor.py:47`: `supervise_start` where the supervisor is created.
- **SQLite Tables:** `process_supervisors` (state, pid, scratch_path) and `process_supervisor_pointers` (state).

## F5 — Watcher Race + Signal Hardening

**Finding:** Potential races in log polling and signal handling during heartbeats.

**Design:**
- **Log Rotation:** `newest_log_mtime` already uses `rglob("*.log")` and catches `FileNotFoundError`. To further harden, ensure it ignores files that are currently being renamed (by catching `OSError` and skipping).
- **Early Tick:** If `tick()` is called before the wrapper has written the first log byte, it returns `no_log`. This is acceptable; it will heartbeat on the next tick once the log appears.
- **Signal Hardening:** Ensure the `heartbeat_callback` in the daemon is resilient to `SIGTERM`. The daemon's shutdown event should be checked *before* starting a new round of watcher ticks, and the heartbeat call itself should be wrapped in a try/except that logs but doesn't crash the daemon if a timeout/interruption occurs.
- **Locking:** Utilize `progress_advisory_lock` in `src/striatum/daemon_supervisor/progress_watcher.py` to ensure the watcher and `surgical_recovery` don't collide.

**Citations:**
- `src/striatum/daemon_supervisor/progress_watcher.py:90`: `newest_log_mtime` implementation.
- `src/striatum/daemon_supervisor/progress_watcher.py:126`: `progress_advisory_lock` using `fcntl.flock`.

## F6 — End-to-End Tests

**Finding:** No e2e tests for the composite tools and watcher lifecycle.

**Design:**
- Create `tests/test_dogfood_e2e_systems.py`.
- Utilize the `MultiRepoHarness` from `tests/_harness/multi_repo.py` to:
    1. Boot a daemon with a PostgreSQL substrate.
    2. Start a supervised process that writes to a log.
    3. Assert the daemon background watcher heartbeats the lease.
    4. Simulate a stale lease and use `dogfood.surgical_recovery` to restore it.
    5. Use `dogfood.publish_on_behalf` to complete a job and verify the composite audit row in the daemon DB.
- These tests will verify the integration between `mcp.py`, `daemon_rpc`, `operator_tools.py`, and `progress_watcher.py`.

**Citations:**
- `tests/_harness/multi_repo.py`: The `MultiRepoHarness` fixture.
- `tests/test_dogfood_publish_on_behalf.py`: Existing partial tests for `publish_on_behalf`.
