author: designer-gemini-pro-001

# Design: RFC 0040 MCP-Driven Dogfood Harness

This design addresses the friction patterns identified in RFC 0040 by introducing MCP-based dogfood-lifecycle tools, a daemon-side supervised-progress heartbeat, and a CLI utility for backporting harness-profile improvements.

## 1. Supervised-Progress Heartbeat (Daemon)

The root cause of lease expiry during active load (e.g., long-running builds) is that the supervised wrapper is blocked on stdin/process wait and cannot heartbeat the lease. We fix this by having the daemon monitor the supervised process's progress at the file-system level.

### Implementation: `ProgressWatcher`
- **Location:** `src/striatum/daemon_supervisor/progress_watcher.py`
- **Behavior:**
    - The `StriatumService` (daemon) manages a background task for each `attached` supervisor.
    - Every 30s (polling interval), the watcher checks for progress signals in the supervisor's `scratch_path`.
- **Progress Signal (mtime):**
    - The watcher reads `os.stat(log_path).st_mtime` for all `.log` files in the `<scratch>/<model>-logs/` directories.
    - If any `st_mtime` is within the last 60s (configurable `mtime_window`), the daemon internally calls `heartbeat` on the supervisor's session's active lease.
    - **Cross-Platform Safety:** `os.stat().st_mtime` is a standard POSIX behavior supported by both Linux and macOS. While `st_mtime` precision varies, 30s polling against a 60s window is sufficiently coarse to be reliable across local filesystems.
- **Concurrency Safety:**
    - One watcher task per supervisor.
    - Watchers are stored in a `dict[supervisor_id, asyncio.Task]` on the daemon's supervisor manager.
    - No global locks; DB heartbeats use the existing transactional safety in `striatum.db`.
- **Cleanup Semantics:**
    - The watcher task is created when a supervisor transitions to `attached`.
    - The watcher task is canceled and removed when the supervisor transitions to `stopped` or `lost`.
    - If the daemon is SIGKILLed, the next daemon start will (via `doctor` or status check) identify the supervisor as `lost` and no watcher will be resumed.

## 2. MCP Dogfood-Lifecycle Tools

Expose the existing RPC state-transition verbs as chat tools in `src/striatum/web/chat_tools.py` to allow AI operators to drive dogfoods without manual ID copying.

### Tool Expansion
The following tools will be added to the `_TOOLS` registry:
- `run_prepare`, `run_start`, `register_session`, `supervise_start`, `claim_next`, `ack`, `publish_artifact`, `verdict`, `complete`, `run_summary`, `evidence_export`, `supervise_stop`.
- Implementation: Each tool delegates to `striatum.api.invoke` with the corresponding CLI-equivalent `argv`.

### Composite Tool: `dogfood.publish_on_behalf`
- **Goal:** Single-call replacement for the routine `ack` + `publish` + `complete` sequence when a supervised lane denies internal `ack`.
- **Implementation:**
    - Lookup active lease and claimed-but-unacked message for the provided `session_id`.
    - Execute the sequence inside a single database transaction.
    - Record a single `operation: "publish_on_behalf"` event in the audit chain.

### Composite Tool: `dogfood.surgical_recovery`
- **Goal:** Reactivate a lease that expired under active load.
- **Implementation:**
    - **Validation:** Refuse if the supervisor is still marked `attached` (requires `supervise_stop` or `lost` first) or if the job is already completed/failed.
    - **Atomic Transition:** 
        1. Reactivate the expired lease (new `expires_at`).
        2. Re-attach the supervisor (state `attached`).
        3. Restore queue message state to `acked` and job state to `running`.
    - **Audit:** Requires an `operator_reason` that lands in the audit trail.
    - **Capability:** New `surgical_recovery` capability (admin-bound).

## 3. Workflow Upgrade CLI

A new CLI verb `striatum workflow upgrade <path>` to backport harness-profile improvements to existing workflows.

- **Location:** `src/striatum/cli/workflow.py` (verb) and `src/striatum/workflow_generator/core.py` (logic).
- **Transformation Logic:**
    1. Parse `workflow.json`.
    2. In each `harness_profiles` entry:
        - Append/update the "no-questions" fragment to `native_delegation.instruction`.
        - For Gemini profiles, append the "front-matter completeness" callout.
    3. **Safety Semantics:**
        - **Refuse-by-default:** If the existing instruction has been heavily customized (detected via template-checksum mismatch or heuristic), refuse to overwrite.
        - **`--force`:** Overwrite regardless of customization.
        - **`--dry-run`:** Print the JSON diff without writing to disk.

## 4. Test Strategy

The RFC 0035 multi-repo harness will be extended to cover these new capabilities.

### Adversarial Cases
- **Stale Recovery Clash:** Operator calls `surgical_recovery` while the supervisor process is still alive and heartbeat-ticking (e.g., a racing heartbeat).
    - *Expected:* The DB transaction should ensure lease reactivation is atomic; the supervisor re-attachment should check current state.
- **Missing Logs:** Supervised-progress watcher polls a `scratch_path` where the model-wrapper hasn't created the log directory yet.
    - *Expected:* Watcher handles `FileNotFoundError` gracefully and continues polling next interval.
- **Concurrent Composite Calls:** Two operator sessions attempt `publish_on_behalf` for the same job.
    - *Expected:* The second call fails with `InvalidTransitionError` because the queue message is no longer in the expected state.
- **Cross-Platform Mtime:** Test simulation of log growth on both Linux (CI) and macOS (local dev) to ensure the 30s/60s thresholds behave identically.

### Verification Matrix
| Feature | Test Case | Success Metric |
| :--- | :--- | :--- |
| Heartbeat | Simulate log file write | Lease `expires_at` is pushed forward in DB |
| Publish-on-behalf | Mock `ack` failure from lane | Job completes via composite tool |
| Surgical Recovery | Expire lease manually, then recover | Job returns to `running` with new lease |
| Workflow Upgrade | Run against dogfood-039 workflow | `workflow.json` contains "no-questions" fragment |
