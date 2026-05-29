# QA & Adversarial Review Report — Milestone 3

**Date**: 2026-05-29T12:17:45Z
**Role**: Reviewer 2 (QA & Adversarial Critic)
**Agent Working Directory**: `~/git/striatum/.agents/teamwork_preview_reviewer_m3_2_gen3`
**Worker 3 Handoff under Review**: `~/git/striatum/.agents/teamwork_preview_worker_m3_gen3/handoff.md`
**Worker 3 Changes under Review**: `~/git/striatum/.agents/teamwork_preview_worker_m3_gen3/changes.md`

---

## Review Summary

**Verdict**: **APPROVE**

Worker 3 has successfully addressed **Issue #49** (PTY Supervision, re-queued packet after checkpoint resolution does not resume) and **Issue #54** (PTY Supervision, RFC 0089 Phase 2 supervision rebridge and status details).

The implementation is structurally clean, robustly tested, and shows excellent attention to defensive programming and performance indexing. No critical bugs or regressions were introduced.

---

## Verified Claims

1. **Claim 1 (Issue #49)**: Appending `AND wp.job_id != qm.job_id` inside `HandleClaimNext`'s SQL query allows the claiming session to reclaim its own job while preserving run-level session isolation.
   - *Verification method*: Inspected `go/pkg/mutations/claim.go` and executed `go test -v -run TestClaimNextReclaimRequeuedJobWithFreshSessionRequired ./pkg/mutations/...` with race detection.
   - *Result*: **PASS**. The query correctly identifies and isolates jobs based on the relaxed constraint, and the regression test asserts both successful reclaim and isolation boundary enforcement.

2. **Claim 2 (Issue #54)**: Helper process liveness checks (`IsHelperAlive`) and start time verification accurately identify helper process degradation.
   - *Verification method*: Inspected `go/pkg/lanehealth/lanehealth.go` and `go/pkg/reads/supervision.go`. Executed integration tests `TestLoadHelperProcessLiveness` and `TestHandleSuperviseStatusSurfacesHelperProcessGone` with race detection.
   - *Result*: **PASS**.

3. **Claim 3 (Race-Free Execution)**: The entire Go test suite compiles and runs cleanly without any race conditions.
   - *Verification method*: Executed `STRIATUM_PG_TEST_URL="postgres:///postgres" go test -race ./...` under full environment.
   - *Result*: **PASS**. All tests finished cleanly, compiling and running with zero race warnings.

---

## Findings

None. The implementation was found to be fully conformant with project specifications and has excellent test coverage.

---

## Coverage Gaps

- *Unexplored area*: Probing processes running inside containerized/namespaces setups (e.g. docker) where `procfs` might be mounted differently or restricted.
  - *Risk Level*: **LOW** (Striatum runs locally as a standalone workflow runner on the host machine).
  - *Recommendation*: Accept risk.

---

## Unverified Items

None. All claims have been independently and rigorously verified.

---

## Challenge Summary (Adversarial Critic)

**Overall risk assessment**: **LOW**

Our stress-testing focused on challenging three key areas:
1. Signal-0 permission boundaries (ESRCH/EPERM).
2. Pointer metadata JSON format parsing robustness (empty or corrupted metadata strings).
3. `ClaimNext` relaxed query indexing and performance.

---

## Challenges

### [Low] Challenge 1: Process Start Token / PID Recycling Under Multi-User Permissions
- **Assumption challenged**: That signal-0 checks or procfs reads will work reliably if PIDs are recycled by other users on the system.
- **Attack scenario**: A helper process dies, and a new process belonging to a *different* user gets assigned the same PID. If `IsHelperAlive` only checked `syscall.Kill(pid, 0)` and ignored permissions, it could return `true` (false positive).
- **Blast radius**: The lane would be falsely projected as healthy, while the delivery bridge is actually dead.
- **Mitigation**: The current implementation of `IsHelperAlive` is robust:
  1. It performs `syscall.Kill(pid, 0)`. If it returns `EPERM` (process belongs to another user), it safely returns `false` (helper dead/gone).
  2. If `Kill` succeeds, it reads `/proc/<pid>/stat` to check the kernel start token (`ProcessStartToken`). Since `/proc/<pid>/stat` is readable across users on standard Linux, even if the PID was recycled, the start time tokens will not match. This completely eliminates the threat of PID recycling attacks.

### [Low] Challenge 2: JSON Parse Robustness Against Corrupted Metadata
- **Assumption challenged**: That the pointer metadata JSON string saved in PostgreSQL will always be well-formed.
- **Attack scenario**: A transient database corruption or an external manual tool inserts a null, empty, or truncated/corrupted JSON string (e.g. `"{helper_pid:"`) in `pointer_metadata_json`.
- **Blast radius**: If the parsing library panicked, any dashboard or lane health check would crash the daemon.
- **Mitigation**: Every parser method (`superviseObject`, `parseTmuxMeta`, `parseHelperFields`) uses defensive switches on `value` type and invokes safe two-value type assertions (e.g. `startTime, _ := m["helper_pid_start_time"].(string)`). Corrupted or empty strings return clean zero-value/fallback maps/structs safely without any panic risk.

### [Low] Challenge 3: Relaxed Query Indexing and Performance Under Large Transaction Volumes
- **Assumption challenged**: That the relaxed `NOT EXISTS` check on `work_packets` does not trigger performance issues (e.g., sequential table scans).
- **Attack scenario**: Under heavy runner load with thousands of previous sessions/packets, a slow `NOT EXISTS` check could cause `HandleClaimNext` to slow down, potentially locking `queue_messages` rows too long.
- **Blast radius**: Increased transaction latency, deadlocks, and general worker starvation.
- **Mitigation**: The schema defines a composite index `idx_work_packets_run_session` on `(repository_id, run_id, session_id)`. The nested query compares these three columns using exact equality, which maps directly to a high-speed index seek.

---

## Stress Test Results

- **Corrupted JSON Metadata Stress Test** → Injecting `{"helper_pid": "corrupted"}` or `"{invalid_json"` → Safely falls back to defaults without panic → **PASS**
- **Signal-0 Permission Boundaries (EPERM)** → Simulating permission denied / foreign process on helper PID → Safely flags helper process as gone (`health.Deliverable` = `false`, reason = `"helper_process_gone"`) → **PASS**
- **Race Condition Under Concurrent Claims** → Executing entire test suite under race detection (`go test -race ./...`) → Zero race conditions detected → **PASS**
