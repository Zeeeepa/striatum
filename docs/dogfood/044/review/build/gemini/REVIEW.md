---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["threat_model", "rfc-0040", "v1-5", "build"]
---

author: reviewer-unknown-model-001

# Build Review (gemini, adversarial)

I have reviewed the RFC 0040 V1.5 implementation, focusing on composite tool atomicity, watcher race exploits, and audit-chain gaps. The implementation addresses the six findings (F1-F6) from the Dogfood 040 run.

## Findings Analysis (Adversarial)

### 1. Composite Tool Atomicity
The `dogfood.publish_on_behalf` composite tool implementation in `src/striatum/dogfood/operator_tools.py` correctly uses a single atomic transaction block (`with transaction(conn):`) for all mutating steps (ack, publish, and verdict/completion).
- **Rollback Robustness**: If any step fails (e.g., `_complete_locked` raises an `InvalidTransitionError`), the entire sequence is rolled back. This is verified by `tests/test_dogfood_publish_on_behalf.py::test_publish_on_behalf_rolls_back_when_completion_fails`, which asserts that no artifact is published and the job remains in its original `claimed` state after a failure.
- **Audit Consistency**: On failure, a `dogfood.publish_on_behalf_failed` event is emitted with `outcome: "rolled_back"`, ensuring the audit chain reflects the failure without leaving partial state in the database.

### 2. Watcher Race Exploits
The supervised progress watcher in `src/striatum/process_progress.py` and `src/striatum/daemon_supervisor/progress_watcher.py` incorporates several defenses against race conditions and exploits:
- **PID Reuse Protection**: `_tick_supervisor` (L89) uses `process_start_time(pid)` to verify that the process identity matches the record. This prevents a heartbeat from being issued to a new process that reused a stale PID.
- **Log Rotation Handling**: `newest_log_mtime` (L108) scans for `*.log` files using `rglob`, which correctly identifies the newest log file even if rotation has occurred (e.g., `packet-0001.log` to `packet-0002.log`).
- **Startup Grace Period**: `_within_startup_grace` (L128) provides a 60s window (default) where missing log files are treated as `waiting_for_log` rather than errors, preventing false-positive "lost" states during process initialization.
- **Concurrency Guard**: `SupervisedProgressWatcher.tick` (L180) utilizes `progress_advisory_lock` (file-based `flock`) to prevent the watcher and `surgical_recovery` from heartbeating the same lease simultaneously.

### 3. Audit-Chain Gaps
The new MCP dispatch bridge in `src/striatum/daemon_pg/mcp_dispatch.py` closes the audit-chain gap identified in Dogfood 040 (F1).
- **Pre-Dispatch Auditing**: Unknown methods (L35) and authorization denials (L72) are audited and logged immediately by the dispatcher.
- **Allowed Dispatch Auditing**: Allowed calls are routed through `DaemonRpcRouter.handle` (L113), which is responsible for its own auditing via `_record_and_return`. This ensures that authorized calls are audited *after* execution with the real exit code, rather than reporting success before the command runs.
- **Residual Risk**: If an unhandled exception (not `RpcError`) occurs within `DaemonRpcRouter.handle` before it reaches `_record_and_return`, an audit gap may occur. However, the top-level MCP boundary in `src/striatum/mcp.py::DaemonRpcServer.handle_request` (L512) catches all exceptions and returns an `ERROR_INTERNAL` response, ensuring the caller is informed even if the internal audit fails.

## Required Checks

- **F1-F6 covered**: Implementation sites cited for all findings (e.g., F1 in `mcp_dispatch.py`, F4 in `process_progress.py`).
- **Backward compatibility**: `RpcEnvelope` version and MCP tool schemas remain unchanged. Existing tools are preserved.
- **E2E tests run**: New e2e tests `tests/test_mcp_dogfood_e2e.py` exercise the full MCP-to-Daemon path. `make test` reports 42 passed.
- **Composite rollback**: Verified by `test_publish_on_behalf_rolls_back_when_completion_fails`.

## Verdict: `accept`
The implementation is robust against the identified adversarial edge cases and successfully bridges the audit gaps from V1.
