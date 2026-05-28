---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["adversarial-review", "design-review", "tmux-liveness", "concurrency", "security"]
---

# Design Review finding: RFC 0089 Phase 1 (rerun2)
author: reviewer-gemini-3.5-flash-high-001
verdict: needs_revision
round_count: 3
stop_reason: interrogation_completed_with_concessions

## Summary of Findings

This adversarial review uncovered three significant, load-bearing vulnerabilities and design inconsistencies in the proposed `DESIGN_SYNTHESIS.md` for RFC 0089 Phase 1. During the live interrogation, the Synthesizer fully conceded these flaws and proposed concrete revisions (helper self-heal, a thread-safe `bridgedWriter` indirection, and hardened start-token enforcement).

The proposed synthesis **needs revision** to incorporate these changes before implementation can safely begin.

---

## 1. Finding 1: Permanent Loss of Byte-Delivery Bridge on Graceful Detach
**Severity**: High
**Status**: Conceded & Resolved

### Description
The synthesis (§4.2) specified that when the helper's internal attach client process exits, the helper should:
> "emit HelperEventAttachExited with payload {attach_pid, exit_code, observed_at, tmux_liveness: live.Class} and return nil without tearing down the supervisor."

However, if `RunHelper` returns `nil`, the helper binary process itself terminates. Because the helper is the process running the `forwardPacketStream` loop and reading from the `stdin.pipe` FIFO to pump bytes into the attach PTY, its termination permanently severs byte delivery. While the daemon would correctly keep the state as `attached` (avoiding false lost transitions), subsequent packet deliveries would fail due to the lack of an active helper process, resulting in a permanent delivery stall.

### Resolution
The synthesis must be revised to adopt **Option 1: Helper Self-Heal**. Instead of exiting on attach client termination, the helper must:
1. Retain its supervising process and remain alive.
2. Gracefully close the dead `ptmx`.
3. Spawn a fresh attach client using `pty.Start` with a bounded retry budget (up to 3 tries within a 60s window).
4. Emit a curated `HelperEventAttachRecovered` event so the daemon can update `attach_client_pid` in pointer metadata.
5. Fall through to `agent_exited` only if the retry budget is exhausted.

---

## 2. Finding 2: Concurrency & Atomicity Hazards During Helper Self-Heal
**Severity**: Medium
**Status**: Conceded & Resolved

### Description
Under the newly adopted helper self-heal model, two concurrency hazards arise:
1. **In-Process Write Race**: The helper's `forwardPacketStream` goroutine captures `result.StdinWriter` by value. If the helper swaps `result.StdinWriter` in-place while `forwardPacketStream` is actively writing, it risks writing to a closed/nil file descriptor, triggering a thread crash.
2. **Metadata Staleness Race**: The helper updates the daemon's metadata asynchronously via `HelperEventAttachRecovered`. A concurrent `supervise.send` call could read a stale `AttachPID` or attempt a write before the database state is synchronized.

### Resolution
1. **Bridged Writer Indirection**: Introduce a new thread-safe `bridgedWriter` type using `sync.RWMutex` to wrap the raw PTY writer. `forwardPacketStream` will write through this indirection. While self-heal is in progress, the write lock blocks the packet pump. Once a new `ptmx` is successfully swapped in, the pump resumes atomically.
2. **Delivery Path Sanitization**: Confirm that the delivery path (`supervise.send`) is isolated from helper client churn: it writes to the persistent `stdin_pipe_path` FIFO rather than consulting `attach_client_pid`, and it relies on stable pane identity (`pane_pid`/`pane_start_time`). This ensures soft staleness of the diagnostic `attach_client_pid` does not impact delivery correctness.

---

## 3. Finding 3: Security & Attestation Bypass Under Probe Disabling
**Severity**: Medium
**Status**: Conceded & Resolved

### Description
The synthesis (§11) describes `STRIATUM_TMUX_PROBE_DISABLE=1` as a rollback escape hatch that falls back to "PID-based liveness using the recorded pane pid." If this fallback only performs a bare `signal(0)` check, PID recycling becomes completely invisible, allowing a hijacked environment to bypass the attestation requirements of D080/D149 and falsely retain model byline attestation.

### Resolution
The synthesis must explicitly specify:
1. **Harden Fallback to `PIDLiveWithStartToken`**: Bypassing the tmux probe must fall back to the full `PIDLiveWithStartToken` path, comparing the live process start-time against `pane_start_token` from metadata.
2. **Harden Launch Metadata Verification**: Refuse `tmux.state="backed"` metadata if `pane_start_token` is empty. The token must be mandatory on backed launches; a failure to capture it must either demote to plain PTY or fail closed when `RequireTmux=true`.
3. **Visibility & Warning**: Add a derived status field `tmux.liveness.probe_disabled=true` and append `tmux_probe_disabled` to the lane attestation reason so operators are clearly alerted to the degraded security posture.
