# PTY Supervision Deep Dive and Triage Report (Issues #49 & #54)

## Summary of Findings
This report details the read-only exploration and analysis of the Striatum codebase regarding two PTY supervision issues:
1. **Issue #49**: Re-queued packets after checkpoint resolution fail to resume because the `HandleClaimNext` SQL query in `go/pkg/mutations/claim.go` restricts sessions that have *already* processed a work packet in the run when `fresh_session_required` is `true`. Even when the job itself is re-queued/resumed, the presence of the original work packet under the same `session_id` blocks the claim.
2. **Issue #54**: The supervision rebridge mechanism successfully launches the background `striatum-supervisor-helper` process, captures its PID (`helper_pid`), and attaches to the Tmux PTY. However, standard status projections (like `lanehealth.Check` and `supervise.status`) fail to verify if the helper process (`helper_pid`) is still running. If the helper dies, packet delivery breaks silently while the lane is reported as healthy.

---

## 1. Issue #49: Re-queued Packet After Checkpoint Resolution Fails to Resume

### Analysis of Transitions & Dispatch Path
1. **Checkpoint Resolution**:
   - An operator resolves a blocker with severity `human_checkpoint` using `checkpoint.resolve` (handled by `HandleCheckpointResolve` in `go/pkg/mutations/operator.go`).
   - If `action == "continue"`, the blocker state is updated to `resolved`.
   - If the blocker is associated with a job (`blockerJobID != nil`), the job state is updated:
     - If the job has a `current_message_id`, the queue message (`striatumd.queue_messages`) is updated to state `pending` and the job's state is updated to `queued` (Lines 191-202).
     - If the message ID is nil, the job state is set to `blocked`, and the system enqueues it, bringing it to `queued` (Lines 204-213).
2. **Agent-Loop Await Loop**:
   - The long-running agent PTY process runs `startDaemonReceiverLoop` in `go/pkg/agentloop/loop.go`.
   - It polls the daemon status using `daemonReceiverReady(ctx, client, repositoryID, sessionID)` (Lines 312-321).
   - Once the lease is released (which happened when the job got blocked), `active_lease_id` is empty. The `last_work_block_at` field is non-nil, so `daemonReceiverReady` returns `true`.
   - The receiver loop then starts awaiting packets by calling the MCP method `work.await_packet` (Lines 324-329).
3. **Claim Failure**:
   - `work.await_packet` triggers `HandleAwaitPacket` in `go/pkg/mutations/claim.go`.
   - This calls `HandleClaimNext(ctx, runner, envelope)`.
   - Inside `HandleClaimNext`, the system executes the following SQL query to claim the next pending work message (Lines 58-81):
     ```sql
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
      FOR UPDATE OF qm SKIP LOCKED
     ```

### Root Cause
When the job is re-queued/resumed, it is the **same** job in the **same** session.
However, because this session previously claimed the *original* work packet for this same run before hitting the checkpoint, a row with `session_id = $4` already exists in `striatumd.work_packets`.
As a result:
- `NOT EXISTS (SELECT 1 FROM striatumd.work_packets WHERE ... AND session_id = $4)` evaluates to **FALSE**.
- If the job has `fresh_session_required = true`, this entire clause is evaluated to **FALSE**, filtering the queue message out.
- Consequently, `HandleClaimNext` returns `status: "no_work"`, and the session is never able to reclaim and resume the job.

### Recommendation / Proposed Fix
Modify the `NOT EXISTS` subquery in `go/pkg/mutations/claim.go` at Line 72 to allow reclaiming the **same** job in the same session. Since `fresh_session_required` specifies that a session must not process *different/multiple* jobs, executing the same job is perfectly legal.
Add the condition `wp.job_id != qm.job_id`:
```go
// go/pkg/mutations/claim.go Line 70-76
			   AND (
			     j.fresh_session_required = false
			     OR NOT EXISTS (
			       SELECT 1 FROM striatumd.work_packets wp
			        WHERE wp.repository_id = qm.repository_id
			          AND wp.run_id = qm.run_id
			          AND wp.session_id = $4
			          AND wp.job_id != qm.job_id
			     )
			   )
```
This ensures that the session is only blocked from claiming the queue message if it has claimed a *different* job in the run, resolving Issue #49 perfectly.

---

## 2. Issue #54: PTY Supervision Rebridge and Status Details (RFC 0089 Phase 2)

### The Rebridge Mechanism
When a supervised lane's PTY delivery bridge is broken or detached, the operator can rebridge it.
1. **API Endpoint**: `supervise.rebridge` handled by `HandleSuperviseRebridge` in `go/pkg/mutations/supervision_control.go`.
2. **Pre-checks**:
   - Ensures the supervisor state is `attached`.
   - Probes the tmux pane and captures the tmux identity using `requireRebridgeableTmuxPane(ctx, supervisor)` (checking pane liveness and that it is tmux-backed).
   - Configures the stdin FIFO pipe using `ensureSupervisorFIFO`.
3. **Helper Process Launch**:
   - Calls `supervisionRebridgeLaunch` (which points to `launchRebridgeHelper` in `supervision_control.go` at Line 1476).
   - Starts a background `striatum-supervisor-helper` process, writing the launch spec as JSON via stdin. The spec contains `RebridgeTmux: &identity`.
   - The helper binary attaches to the tmux session (`attachTmuxPTY` executes `tmux attach-session -t sessionName`) and pipes stdin/stdout.
   - It reads events from `helper-events.jsonl` and waits for `HelperEventAgentStarted` before completing the launch.
4. **Pointer Metadata Update**:
   - In `HandleSuperviseRebridge`, previous pointer metadata is read and updated with `helper_pid`, `helper_pid_start_time`, `helper_events_path`, and `helper_events_offset`.
   - If an exit event was *not* detected during the launch, the `delivery_liveness` block is cleared to reset degraded status.
   - Refreshes supervisor heartbeat and logs a `supervisor.rebridged` event.

### Where Status Details are Loaded/Monitored
Status details for active process supervisors are queried across multiple endpoints:
1. **`HandleSuperviseStatus`** (`go/pkg/reads/supervision.go` line 54): Mirrors the status projection. Refreshes/drains helper events via `DrainHelperEventsHook` and fetches active pointers. Probes lane liveness dynamically using `lanehealth.Check`.
2. **`sessionProtocolLiveness`** (`go/pkg/reads/supervision.go` line 231): Formulates session activity timestamps (`last_mcp_request_at`, `last_await_packet_at`, etc.) and the active lease ID.
3. **`lanehealth.Check`** (`go/pkg/lanehealth/lanehealth.go` line 225): Unified checker that aggregates DB details, triggers process checks (signal-0 and start time), and evaluates tmux and delivery liveness.
4. **`reattachStatusRows` / `reattachStatusView`** (`go/pkg/reads/supervision.go` lines 415, 496): Projects supervisor, pointer, and daemon supervisor status, evaluating reattach states (`reattachable`, `lost_candidate`, `needs_verification`, `needs_repair`).

### Gaps in Status Detail Reporting
There is a significant gap in the PTY supervision status details pipeline:
1. **Helper Process Liveness Verification Gap**:
   - Standard lane health checks (`lanehealth.Check`) probe the PTY process PID (`f.PID` / `ps.pid`) and tmux state (`f.PointerTmuxMeta`).
   - However, the health checks **never** verify if the `helper_pid` (the background `striatum-supervisor-helper` process, which acts as the delivery bridge between the daemon/FIFO and the tmux master PTY) is actually still running.
   - If the helper process dies (e.g. killed by the OS or crashed), `lanehealth.Check` still reports the lane as healthy and attested because tmux itself is alive and the PTY PID exists.
   - Packet delivery will be silently broken, and the daemon will not automatically detect that the delivery bridge is degraded or lost.
2. **No Automated Degradation Trigger**:
   - Because `lanehealth.Classify` does not take `helper_pid` liveness into account, the lane's `Attested` and `Deliverable` states remain `true`.
   - To fix this gap, the status checker (`lanehealth.Check` and `reattachStatusView`) should extract `helper_pid` and `helper_pid_start_time` from the pointer's metadata and verify if the helper process is still alive. If the helper is dead, the delivery status should be flagged as degraded/broken, prompting the operator or supervisor to trigger a rebridge.

### Step-by-Step Recommendation for Fixing Issue #54
1. **Update `lanehealth.Facts`**:
   - Add `HelperPID` (int) and `HelperPIDStartTime` (string) to `Facts` struct in `go/pkg/lanehealth/lanehealth.go`.
2. **Extract Helper Details in `lanehealth.Check`**:
   - In `Check(...)`, parse `helper_pid` and `helper_pid_start_time` from `pointer_metadata_json` (which is already loaded from the database).
   - Check if `helper_pid` is still alive (e.g. using a signal-0 check and start time verification). If it is dead, set `f.DeliveryDegraded = true` and `f.DeliveryReason = "helper_process_gone"`.
3. **Refine `lanehealth.Classify`**:
   - If `f.DeliveryDegraded` is true, the health state should not mark `Deliverable = true`, forcing the lane to be reported as degraded/unattested with a clear remediation message.
