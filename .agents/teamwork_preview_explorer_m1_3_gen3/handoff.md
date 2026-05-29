# Handoff Report — Explorer 3 (teamwork_preview_explorer)

## 1. Observation

### Issue #49 Observations:
- **File**: `go/pkg/mutations/claim.go`
- **Lines**: 58-87 (in `HandleClaimNext`)
- **Verbatim Code**:
  ```go
		rows, err := queryRows(ctx, tx, `
			SELECT qm.*
			  FROM striatumd.queue_messages qm
			  JOIN striatumd.jobs j
			    ON j.repository_id = qm.repository_id
			   AND j.job_id = qm.job_id
			 WHERE qm.repository_id = $1
			   AND qm.kind = 'work'
			   AND qm.state = 'pending'
			   AND qm.target_role_id = $2
			   AND (qm.target_lane_id IS NULL OR qm.target_lane_id = $3)
			   AND (
			     j.fresh_session_required = false
			     OR NOT EXISTS (
			       SELECT 1 FROM striatumd.work_packets wp
			        WHERE wp.repository_id = qm.repository_id
			          AND wp.run_id = qm.run_id
			          AND wp.session_id = $4
			     )
			   )
			   AND qm.run_id = $5
			 ORDER BY qm.priority DESC, qm.created_at ASC
			 LIMIT 1
			 FOR UPDATE OF qm SKIP LOCKED`,
			repositoryID,
			session["role_id"],
			session["lane_id"],
			sessionID,
			runID,
		)
  ```
- **Checkpoint Resolution Transition**:
  - In `go/pkg/mutations/operator.go` (Lines 189-218), `HandleCheckpointResolve` updates `queue_messages` to state `'pending'` and the `jobs` state to `'queued'` (or calls `enqueueJob` which does the same) for a continue action.
  - The session's receiver loop in `go/pkg/agentloop/loop.go` (Lines 324-329) calls `work.await_packet`, which dispatches `HandleClaimNext`.

### Issue #54 Observations:
- **File**: `go/pkg/mutations/supervision_control.go`
- **Rebridge Mechanism Entry Point**: `HandleSuperviseRebridge` (Lines 491-617) launches a background `striatum-supervisor-helper` process via `supervisionRebridgeLaunch` / `launchRebridgeHelper` (Lines 1476-1572).
- **Pointer Metadata Update**:
  - `HandleSuperviseRebridge` updates the pointer's metadata with `helper_pid`, `helper_pid_start_time`, `helper_events_path`, and `helper_events_offset` (Lines 557-560).
- **Liveness Checking Projections**:
  - **File**: `go/pkg/reads/supervision.go` — `HandleSuperviseStatus` (Lines 54-228) and `sessionProtocolLiveness` (Lines 231-275) load metadata and liveness parameters.
  - **File**: `go/pkg/lanehealth/lanehealth.go` — `Check` (Lines 225-352) reads `pointer_metadata_json` and runs `ProbeLane` inside the pure state-machine classifier `Classify` (Lines 138-214).
- **Probing Gap**:
  - `lanehealth.Check` triggers active signal-0/start-time probes only for the agent process PID (`f.PID` / `ps.pid`) (Lines 346-349).
  - The `helper_pid` (the process id of the `striatum-supervisor-helper` process acting as the PTY delivery bridge) is never verified or probed during liveness checks.

---

## 2. Logic Chain

### For Issue #49:
1. When a job is blocked at a checkpoint (e.g. `work.block` with severity `human_checkpoint`), the active lease is released and the job transitions to `waiting_human`.
2. When the operator resolves the checkpoint (`checkpoint.resolve` with `continue`), the queue message is set back to `pending`, and the job transitions to `queued`.
3. The long-running agent PTY session is waiting, checks status via `daemonReceiverReady`, and calls `work.await_packet`, which delegates to `HandleClaimNext`.
4. In `HandleClaimNext`, the claim query retrieves the next pending work message.
5. If `fresh_session_required` is `true` for that job, the query asserts that `NOT EXISTS` a work packet in `striatumd.work_packets` for this run and session ID.
6. However, because this session previously claimed the *original* work packet for this same run prior to hitting the checkpoint, a row with this `session_id` already exists.
7. Consequently, the `NOT EXISTS` subquery returns `FALSE`. The query filters the pending message out, preventing the session from reclaiming and resuming its own job.
8. Adding `wp.job_id != qm.job_id` to the `NOT EXISTS` query sub-clause ensures that a session is only blocked if it has claimed a *different* job in the run, permitting the reclaiming and resumption of the same job.

### For Issue #54:
1. `HandleSuperviseRebridge` correctly creates the delivery bridge process, records its `helper_pid`, and links it in the pointer metadata.
2. If the background helper process (`striatum-supervisor-helper`) exits or crashes, the delivery pipeline between the daemon FIFO/MCP and the agent PTY is completely severed.
3. However, `lanehealth.Check` and `sessionProtocolLiveness` only probe the lane's main PTY process PID (`f.PID` / `ps.pid`).
4. Since the lane's PTY process under tmux is still running and alive, `lanehealth.Classify` classifies the lane as healthy and attested.
5. As a result, the broken delivery bridge is never reported as degraded or unattested, creating a silent failure in supervision.
6. Probing the `helper_pid` alongside `f.PID` in `lanehealth.Check` resolves this gap by ensuring delivery state is marked degraded/gone if the helper process dies.

---

## 3. Caveats
- **Read-Only Scope**: In accordance with the dispatch constraints, no code changes were performed. The recommendations are purely architectural/strategic.
- **Process Start Time Readers**: Operating system differences in reading start times (e.g. `/proc` under Linux vs `ps` under Darwin) were not modified; the proposed helper process probe relies on existing robust platform wrappers.

---

## 4. Conclusion
- **Issue #49 Root Cause**: The SQL query in `HandleClaimNext` (`go/pkg/mutations/claim.go`) prevents the same session from reclaiming its own job when `fresh_session_required = true` because the original claim's work packet already exists under that `session_id`.
- **Issue #49 Fix**: Append `AND wp.job_id != qm.job_id` to the `NOT EXISTS` work packet clause.
- **Issue #54 Root Cause**: Standard status detail projections (`supervise.status` and `lanehealth.Check`) do not signal-probe the PTY helper process (`helper_pid`), resulting in silent delivery bridge failures when the helper dies.
- **Issue #54 Fix**: Update `lanehealth.Check` and `Facts` to parse `helper_pid`/`helper_pid_start_time` from pointer metadata, verify helper process liveness, and flag `f.DeliveryDegraded = true` with reason `"helper_process_gone"` if the helper is dead.

---

## 5. Verification Method

### How to Verify the Recommended Fix for Issue #49:
1. Inspect the SQL query in `go/pkg/mutations/claim.go`.
2. To test, run a target job with `fresh_session_required = true`, block it using `work.block` with severity `human_checkpoint`, resolve the checkpoint with `checkpoint.resolve` (continue action), and confirm that the session successfully claims the re-queued packet.
3. Check using the standard project test suite:
   ```bash
   go test -v ./go/pkg/mutations/...
   ```

### How to Verify the Recommended Fix for Issue #54:
1. Inspect `go/pkg/lanehealth/lanehealth.go` and `go/pkg/reads/supervision.go`.
2. Simulate a helper process death by killing the process PID listed as `helper_pid` in the pointer's metadata.
3. Call `supervise.status` or `lanehealth.Check` and verify that the delivery status transitions to `"degraded"` and lane liveness/attestation details correctly identify `"helper_process_gone"`.
4. Check using:
   ```bash
   go test -v ./go/pkg/lanehealth/...
   go test -v ./go/pkg/reads/...
   ```
