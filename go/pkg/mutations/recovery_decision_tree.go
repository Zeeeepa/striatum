package mutations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
	"github.com/jackc/pgx/v5"
)

// recoveryPolicy holds the per-job autonomous-recovery budgets read from the
// workflow's optional top-level `recovery_policy` block (RFC 0020 §Step 2),
// extended here for RFC 0101 Phase 3 Slice 2 with `max_requeues` /
// `max_transfers`. The block is documented but was never parsed in the Go
// daemon, so reading it here is the first consumer; we extend the existing
// block rather than inventing a parallel one. A workflow that omits the block
// gets the slice defaults.
type recoveryPolicy struct {
	maxRequeues int
	// maxUnsealedRequeues bounds requeues for the agent_exited_unsealed class
	// (#289): a confirmed-dead supervised agent that engaged the work protocol and
	// emitted output but never called work.complete. A respawn can re-author and
	// seal the work, so one retry is worthwhile, but a systematic unsealed exit
	// (turn-end / context-budget / rate-limit death mid-task) rarely self-heals on
	// repeat — so this class escalates to the operator faster than a hard crash,
	// burning less of the requeue budget than the symptom the issue reported.
	maxUnsealedRequeues int
	maxTransfers        int
}

const (
	defaultMaxRequeues         = 2
	defaultMaxUnsealedRequeues = 1
	defaultMaxTransfers        = 3
)

// Recovery-specific stall classifications for a confirmed-dead supervised agent.
// These are distinct from the sessionliveness.* protocol stall classes (which
// describe a still-present session that is not making progress): here the agent
// PROCESS is gone.
const (
	// stallClassAgentPIDDead — the supervised agent process/pane is dead and the
	// session showed no evidence of having engaged the work protocol with output.
	stallClassAgentPIDDead = "agent_pid_dead"
	// stallClassAgentExitedUnsealed — the agent engaged the work protocol (claimed
	// / made MCP tool calls) and emitted PTY output, then its process died WITHOUT
	// calling work.complete. The work may be complete-but-unsealed; recovery cannot
	// seal on its behalf (attestation), so it respawns once then escalates with a
	// distinct, inspect-the-worktree remediation (#289).
	stallClassAgentExitedUnsealed = "agent_exited_unsealed"
)

// recoveryPolicyFromWorkflow reads the recovery budgets from a workflow JSON
// map. Honors the documented RFC 0020 `max_total_requeues_per_job` as the
// requeue fallback when the slice-2 `max_requeues` key is absent, so an
// existing recovery_policy block keeps meaning what it says.
func recoveryPolicyFromWorkflow(workflow map[string]any) recoveryPolicy {
	policy := recoveryPolicy{
		maxRequeues:         defaultMaxRequeues,
		maxUnsealedRequeues: defaultMaxUnsealedRequeues,
		maxTransfers:        defaultMaxTransfers,
	}
	block := asMap(workflow["recovery_policy"])
	if len(block) == 0 {
		return policy
	}
	if v, ok := block["max_requeues"]; ok {
		policy.maxRequeues = intFromAny(v, policy.maxRequeues)
	} else if v, ok := block["max_total_requeues_per_job"]; ok {
		// Backward-compatible: the documented RFC 0020 field bounds total
		// requeues per job, which is exactly this budget.
		policy.maxRequeues = intFromAny(v, policy.maxRequeues)
	}
	if v, ok := block["max_unsealed_requeues"]; ok {
		policy.maxUnsealedRequeues = intFromAny(v, policy.maxUnsealedRequeues)
	}
	if v, ok := block["max_transfers"]; ok {
		policy.maxTransfers = intFromAny(v, policy.maxTransfers)
	}
	if policy.maxRequeues < 0 {
		policy.maxRequeues = 0
	}
	if policy.maxUnsealedRequeues < 0 {
		policy.maxUnsealedRequeues = 0
	}
	// The unsealed budget is meant to escalate no later than a hard crash, so cap
	// it at maxRequeues — a smaller-or-equal bound holds even under an operator
	// override that sets max_requeues below the unsealed default.
	if policy.maxUnsealedRequeues > policy.maxRequeues {
		policy.maxUnsealedRequeues = policy.maxRequeues
	}
	if policy.maxTransfers < 0 {
		policy.maxTransfers = 0
	}
	return policy
}

// jobRecoveryBudget mirrors a striatumd.job_recovery_state row.
type jobRecoveryBudget struct {
	requeueCount       int
	transferCount      int
	respawnCount       int
	escalationPending  bool
	lastRecoveryAction string
}

// readJobRecoveryBudget reads (without upserting) the current budget row for a
// job. A missing row is reported as a zeroed budget — the upsert happens only
// when an action is actually recorded.
func readJobRecoveryBudget(ctx context.Context, tx db.TxRunner, repositoryID, jobID string) (jobRecoveryBudget, error) {
	row, err := oneRow(ctx, tx, `
		SELECT requeue_count, transfer_count, respawn_count, escalation_pending,
		       last_recovery_action
		  FROM striatumd.job_recovery_state
		 WHERE repository_id = $1 AND job_id = $2`, repositoryID, jobID)
	if err != nil {
		if isNoRows(err) {
			return jobRecoveryBudget{}, nil
		}
		return jobRecoveryBudget{}, err
	}
	return jobRecoveryBudget{
		requeueCount:       intFromAny(row["requeue_count"], 0),
		transferCount:      intFromAny(row["transfer_count"], 0),
		respawnCount:       intFromAny(row["respawn_count"], 0),
		escalationPending:  row["escalation_pending"] == true,
		lastRecoveryAction: fmt.Sprint(nullable(row["last_recovery_action"])),
	}, nil
}

// recordRecoveryAction increments the named budget counter and stamps the last
// action metadata. counterColumn is one of requeue_count / transfer_count /
// respawn_count. The row is created on first action (idempotent upsert).
func recordRecoveryAction(ctx context.Context, tx db.TxRunner, repositoryID, runID, jobID, counterColumn, action, stallClass string) error {
	now := nowString()
	// counterColumn is a fixed internal constant (never user input), so it is
	// safe to interpolate into the SQL text.
	sql := fmt.Sprintf(`
		INSERT INTO striatumd.job_recovery_state (
		  repository_id, run_id, job_id, %[1]s,
		  last_recovery_action, last_recovery_at, last_stall_class,
		  created_at, updated_at
		) VALUES ($1, $2, $3, 1, $4, $5, $6, $5, $5)
		ON CONFLICT (repository_id, job_id) DO UPDATE
		   SET %[1]s = striatumd.job_recovery_state.%[1]s + 1,
		       last_recovery_action = EXCLUDED.last_recovery_action,
		       last_recovery_at = EXCLUDED.last_recovery_at,
		       last_stall_class = EXCLUDED.last_stall_class,
		       updated_at = EXCLUDED.updated_at`, counterColumn)
	return tx.Exec(ctx, sql, repositoryID, runID, jobID, action, now, nullable(stallClass))
}

// markRecoveryEscalation flags the budget row as escalation_pending so Phase 4
// can flip the run to needs_operator. It does NOT increment a counter — the
// budget was already exhausted. It records the action and stall class for
// audit. Idempotent: re-flagging an already-pending row only refreshes the
// metadata, not escalated_at (set once).
func markRecoveryEscalation(ctx context.Context, tx db.TxRunner, repositoryID, runID, jobID, action, stallClass string) error {
	now := nowString()
	return tx.Exec(ctx, `
		INSERT INTO striatumd.job_recovery_state (
		  repository_id, run_id, job_id,
		  last_recovery_action, last_recovery_at, last_stall_class,
		  escalation_pending, escalated_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, true, $5, $5, $5)
		ON CONFLICT (repository_id, job_id) DO UPDATE
		   SET escalation_pending = true,
		       escalated_at = COALESCE(striatumd.job_recovery_state.escalated_at, EXCLUDED.escalated_at),
		       last_recovery_action = EXCLUDED.last_recovery_action,
		       last_recovery_at = EXCLUDED.last_recovery_at,
		       last_stall_class = EXCLUDED.last_stall_class,
		       updated_at = EXCLUDED.updated_at`,
		repositoryID, runID, jobID, action, now, nullable(stallClass))
}

const recoveryDecisionAuthor = "striatumd-recovery"

// recoverStuckJobs is the RFC 0101 Phase 3 Slice 2 autonomous in-daemon
// recovery decision tree (OQ4 resolved in-daemon, D094). It runs INSIDE
// HandleRecoveryAuto's existing withTx, after the auto-publish pass and before
// refreshRunLiveness, so it sees the same snapshot the auto-publish loop did and
// the liveness refresh classifies any sessions it leaves untouched.
//
// For each UNFINISHED job (state in claimed/running/stale_lease) it classifies
// the owning session (if any) via sessionliveness.Classify and decides:
//
//   - owning session dead/closed/absent AND lease released/expired/absent AND
//     no recoverable artifact (auto-publish already skipped it) ->
//     requeueJobSameAttempt (requeue_count budget). repo-write jobs are
//     reclaimed with operatorOverride: the bounded daemon loop IS the inspection
//     (D036's manual gate is for the interactive CLI, not autonomous recovery).
//   - stalled past its deadline with a still-active-but-expired lease ->
//     force-expire + requeueJobSameAttempt (transfer_count budget).
//   - working_* / quiet (pre-deadline) sessions are left untouched.
//
// Leaked interrogation windows (an awaiting-interrogation target session with no
// pending panel consumers) are closed via releaseInterrogationTargetForCompletedReview
// / maybeCloseInterrogationTarget (no budget).
//
// It is idempotent + convergent: re-running on an already-requeued (now
// queued+pending) job is a no-op (requeueJobSameAttempt detects
// already_reclaimable and we skip the budget increment in that case).
func recoverStuckJobs(ctx context.Context, tx db.TxRunner, repositoryID, runID string, policy recoveryPolicy) ([]map[string]any, error) {
	now := time.Now().UTC().Truncate(time.Second)
	actions := []map[string]any{}

	// Scan unfinished jobs with their most-recent lease (any state) and that
	// lease's owning session's full liveness-activity columns. The lease is
	// resolved as the latest lease on the job resource so a job whose
	// current_lease_id was cleared (running-limbo) still resolves its dead
	// owner. expected_artifacts_json lets us re-check the auto-publish skip.
	rows, err := queryRows(ctx, tx, `
		SELECT j.job_id, j.run_id, j.workflow_job_id, j.state AS job_state,
		       j.role_id, j.lane_selector_json, j.max_attempts,
		       j.write_scope_json, j.current_message_id, j.current_lease_id,
		       j.expected_artifacts_json,
		       l.lease_id, l.state AS lease_state, l.expires_at AS lease_expires_at,
		       l.owner_session_id,
		       s.session_id, s.state AS session_state, s.registered_at,
		       s.last_mcp_request_at, s.last_tools_list_at, s.last_await_packet_at,
		       s.last_packet_delivered_at, s.last_ack_at, s.last_work_block_at,
		       s.last_work_release_at, s.last_work_complete_at, s.last_work_heartbeat_at,
		       s.last_session_ready_at, s.last_session_heartbeat_at,
		       s.last_session_question_at, s.last_session_escalate_at,
		       s.last_pty_activity_at, s.last_tool_call_started_at,
		       s.last_tool_call_finished_at,
		       s.liveness_stall_class, s.liveness_stall_since,
		       al.lease_id AS active_lease_id, al.acquired_at AS active_lease_acquired_at,
		       al.expires_at AS active_lease_expires_at,
		       al.last_heartbeat_at AS active_lease_last_heartbeat_at,
		       sp.pid AS supervisor_pointer_pid,
		       sp.pid_start_time AS supervisor_pointer_pid_start_time,
		       sp.state AS supervisor_pointer_state,
		       sp.metadata_json AS supervisor_pointer_metadata_json
		  FROM striatumd.jobs j
		  LEFT JOIN LATERAL (
		    SELECT lz.lease_id, lz.state, lz.expires_at, lz.owner_session_id
		      FROM striatumd.leases lz
		     WHERE lz.repository_id = j.repository_id
		       AND lz.resource_id = j.job_id
		     -- Prefer the ACTIVE lease first (#145): a same-second acquired_at tie
		     -- between a prior attempt's released lease and the live attempt's active
		     -- lease must resolve the live one, not let the random lease_id tiebreak
		     -- pick the released (closed-session) lease and falsely requeue while the
		     -- live lease still holds the job.
		     -- Then prefer the job's own current_lease_id (the inverse #145 shape,
		     -- caught by the ACE explicit-consumer fault fixture): when BOTH the
		     -- prior attempt's lease and the current attempt's lease are released
		     -- within the same second, the random lease_id tiebreak can resolve the
		     -- PRIOR attempt's lease, whose owner session is still active, masking
		     -- the genuinely dead current owner so the dead lane is never requeued.
		     -- IS NOT DISTINCT FROM keeps the predicate false (not NULL) when the
		     -- job's current_lease_id was already cleared (running-limbo).
		     ORDER BY (lz.state = 'active') DESC,
		              (lz.lease_id IS NOT DISTINCT FROM j.current_lease_id) DESC,
		              lz.acquired_at DESC, lz.lease_id DESC
		     LIMIT 1
		  ) l ON true
		  LEFT JOIN striatumd.sessions s
		    ON s.repository_id = j.repository_id
		   AND s.session_id = l.owner_session_id
		  LEFT JOIN LATERAL (
		    SELECT az.lease_id, az.acquired_at, az.expires_at, az.last_heartbeat_at
		      FROM striatumd.leases az
		     WHERE az.repository_id = s.repository_id
		       AND az.owner_session_id = s.session_id
		       AND az.state = 'active'
		     ORDER BY az.acquired_at DESC, az.lease_id DESC
		     LIMIT 1
		  ) al ON true
		  LEFT JOIN LATERAL (
		    SELECT pp.pid, pp.pid_start_time, pp.state, pp.metadata_json
		      FROM striatumd.process_supervisor_pointers pp
		     WHERE pp.repository_id = s.repository_id
		       AND pp.session_id = s.session_id
		     ORDER BY pp.updated_at DESC, pp.supervisor_id DESC
		     LIMIT 1
		  ) sp ON true
		 WHERE j.repository_id = $1
		   AND j.run_id = $2
		   AND j.state IN ('claimed','running','stale_lease')
		 ORDER BY j.workflow_job_id
		 FOR UPDATE OF j`, repositoryID, runID)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		jobID := fmt.Sprint(row["job_id"])
		workflowJobID := fmt.Sprint(row["workflow_job_id"])

		// Classify the owning session. A job with no resolvable owning session
		// row is treated as absent (dead lane / closed session) — Classify on an
		// empty activity reports the inactive/dead protocol.
		activity := sessionliveness.ActivityFromRow(row)
		result := sessionliveness.Classify(activity, sessionliveness.DefaultPolicy(), now)
		protocol := result.Protocol
		stallClass := result.StallClass
		if stallClass == "" {
			stallClass = protocol
		}

		sessionID := fmt.Sprint(nullable(row["session_id"]))
		sessionState := fmt.Sprint(nullable(row["session_state"]))
		sessionAbsent := sessionID == "" || sessionID == "<nil>"
		sessionDead := sessionAbsent ||
			protocol == sessionliveness.ProtocolDead ||
			(sessionState != "" && sessionState != "active" && sessionState != "<nil>")

		leaseState := fmt.Sprint(nullable(row["lease_state"]))
		hasActiveLease := leaseState == "active"
		// "released/expired/absent" — the dead-lane requeue precondition.
		leaseClearedOrGone := !hasActiveLease

		// CASE 3: a leaked interrogation window. An awaiting-interrogation target
		// session with no pending panel consumers blocks the run. Handle it by
		// releasing the interrogable target for any completed review depending on
		// this job; maybeCloseInterrogationTarget is the idempotent guard.
		if sessionID != "" && sessionID != "<nil>" {
			closed, ierr := closeLeakedInterrogationWindow(ctx, tx, repositoryID, runID, jobID, sessionID)
			if ierr != nil {
				return nil, ierr
			}
			if closed {
				actions = append(actions, map[string]any{
					"workflow_job_id": workflowJobID,
					"job_id":          jobID,
					"action":          "interrogation_window_closed",
					"session_id":      sessionID,
				})
				// Re-fetch nothing; the window close does not requeue the job. Fall
				// through so a genuinely dead job is still reclaimed below.
			}
		}

		// confirmedDead memoizes the supervised-agent liveness probe (oracle-backed,
		// so this is a cache read on the production sweep). It is consulted lazily by
		// CASE 2 and the default branch, so a job that hits CASE 1 first never probes.
		deadProbed := false
		deadResult := false
		confirmedDead := func() bool {
			if !deadProbed {
				deadResult = supervisedAgentConfirmedDead(ctx, row)
				deadProbed = true
			}
			return deadResult
		}

		// Decide the operational recovery action.
		var action, counterColumn string
		var forceExpire bool
		var closeStalledOwner bool
		switch {
		case sessionDead && leaseClearedOrGone && !leaseStaleActive(row):
			// CASE 1: dead/closed/absent owner, lease released/expired/absent, and
			// the auto-publish pass already ran (any recoverable artifact would have
			// completed the job, so a job still unfinished here has none). Requeue
			// on the same attempt. CASE 1 takes precedence for a dead/absent session;
			// the !sessionDead guard on CASE 2 makes the precedence unambiguous (a
			// closed/absent session is sessionDead and never broadens into CASE 2).
			action = "requeue_same_attempt"
			counterColumn = "requeue_count"
		case !sessionDead && protocol == sessionliveness.ProtocolStalled && !confirmedDead():
			// CASE 2 (RFC 0101 Phase 3 Slice 2b, #121 parked agent): the owning
			// session is still present and state='active' but HONESTLY stalled — and
			// its supervised agent PROCESS is NOT confirmed dead. The confirmed-dead
			// exclusion (#289) matters because a dead agent's activity timestamps
			// freeze at the moment of death, so a sweep that runs after they age past
			// the liveness deadlines would otherwise misread a dead lane as a live
			// parked one and transfer it on the transfer_count budget. A confirmed-dead
			// agent must instead fall through to the dead-lane requeue path below, where
			// the unsealed-exit classification and its smaller budget apply. The
			// Phase 1 honest-liveness contract guarantees protocol==stalled means NO
			// Phase 1 honest-liveness contract guarantees protocol==stalled means NO
			// protocol + NO PTY + NO tool-call progress past the deadline — i.e.
			// genuinely stuck — so acting on it is safe regardless of lease state.
			// This fires whether the lease is expired, released, or absent: by the
			// time the decision tree runs, HandleRecoveryAuto's expireLeases pass has
			// already flipped a past-expiry active lease to 'expired', so the old
			// `hasActiveLease && leaseExpired` precondition could never observe the
			// live #121 case. It is a transfer to a fresh session, on the
			// transfer_count budget. requeueJobSameAttempt already force-expires any
			// residual active lease, so dropping the hasActiveLease precondition is
			// safe; forceExpire stays true as belt-and-suspenders. Because the
			// session never closed itself, also close the superseded stalled owner
			// below so the parked lane cannot wake up to double-work or reclaim.
			action = "transfer_requeue"
			counterColumn = "transfer_count"
			forceExpire = true
			closeStalledOwner = true
		default:
			// working_* / quiet (pre-deadline), or a live lease that has not yet
			// expired: not genuinely stuck BY the session/lease liveness signal.
			// But lease/heartbeat freshness is forgeable by an out-of-band operator
			// `striatum heartbeat` loop (the documented long-command lease-expiry
			// mitigation), while an actually-dead agent PROCESS is not. If the
			// supervised agent is confirmed dead, this is a masked dead lane (#147
			// Symptom B) that would otherwise sit `running` forever — requeue it on
			// the same attempt and close the falsely-active owning session. The
			// probe is positive (a dead PID / dead pane), so a genuinely-working
			// lane — including one whose supervisor bridge merely detached while the
			// agent runs (#147 Symptom A) — is never requeued, and an indeterminate
			// probe leaves the job untouched.
			if !confirmedDead() {
				continue
			}
			action = "requeue_same_attempt"
			counterColumn = "requeue_count"
			forceExpire = true
			closeStalledOwner = true
			// #289: distinguish an agent that engaged the work protocol and produced
			// output but never sealed (no work.complete) from a hard early crash that
			// never got going. Both respawn (we cannot seal on the agent's behalf), but
			// the unsealed variant gets a distinct class, a smaller requeue budget, and
			// an inspect-the-worktree escalation remediation. The signal is coarse on
			// purpose: it fires for any confirmed-dead agent that made an MCP tool call
			// and emitted PTY output — whether it finished a deliverable then exited at
			// turn-end / context-budget / rate-limit, or merely got going then died —
			// and both populations are well served by "inspect the worktree, retry
			// once, then escalate". Only the never-engaged early crash stays
			// agent_pid_dead.
			if deadAgentExitedUnsealed(activity) {
				stallClass = stallClassAgentExitedUnsealed
			} else {
				stallClass = stallClassAgentPIDDead
			}
		}

		budget, berr := readJobRecoveryBudget(ctx, tx, repositoryID, jobID)
		if berr != nil {
			return nil, berr
		}
		current := budget.requeueCount
		limit := policy.maxRequeues
		if counterColumn == "transfer_count" {
			current = budget.transferCount
			limit = policy.maxTransfers
		} else if stallClass == stallClassAgentExitedUnsealed {
			// #289: the unsealed-exit class shares the requeue_count counter but
			// escalates after a smaller budget — a respawn rarely heals a systematic
			// unsealed exit, so hand it to the operator sooner with a precise reason.
			limit = policy.maxUnsealedRequeues
		}
		if current >= limit {
			// Budget exhausted: do NOT act. Flag escalation_pending (Phase 4
			// consumes it) and record it once. Idempotent.
			if eerr := markRecoveryEscalation(ctx, tx, repositoryID, runID, jobID, action+"_budget_exhausted", stallClass); eerr != nil {
				return nil, eerr
			}
			if _, eerr := appendEvent(ctx, tx, repositoryID, runID, "recovery.budget_exhausted", nullable(sessionID), jobID, nil, nil, nil, map[string]any{
				"workflow_job_id":    workflowJobID,
				"action":             action,
				"budget":             counterColumn,
				"count":              current,
				"limit":              limit,
				"stall_class":        stallClass,
				"escalation_pending": true,
			}); eerr != nil {
				return nil, eerr
			}
			actions = append(actions, map[string]any{
				"workflow_job_id":    workflowJobID,
				"job_id":             jobID,
				"action":             action,
				"budget":             counterColumn,
				"count":              current,
				"limit":              limit,
				"escalation_pending": true,
				"acted":              false,
			})
			continue
		}

		// Force-expire a residual active-but-expired lease before requeue (the
		// transfer case). requeueJobSameAttempt also force-expires any active
		// lease pinned to the job, so this is belt-and-suspenders for clarity.
		if forceExpire {
			if err := tx.Exec(ctx, `
				UPDATE striatumd.leases
				   SET state = 'expired', released_at = COALESCE(released_at, $1),
				       release_reason = 'recovery_transfer'
				 WHERE repository_id = $2 AND resource_id = $3 AND state = 'active'`,
				nowString(), repositoryID, jobID); err != nil {
				return nil, err
			}
		}

		repoWrite := isRepoWrite(row)
		opts := requeueSameAttemptOptions{
			author:        recoveryDecisionAuthor,
			justification: fmt.Sprintf("autonomous recovery: owning session %s (%s); %s", sessionStateLabel(sessionID, sessionState), stallClass, action),
		}
		if repoWrite {
			// The bounded daemon sweep IS the inspection D036 requires of an
			// interactive operator, so autonomous recovery overrides the repo-write
			// manual gate.
			opts.operatorOverride = true
		}
		res, rqerr := requeueJobSameAttempt(ctx, tx, repositoryID, row, opts)
		if rqerr != nil {
			return nil, rqerr
		}
		if res.alreadyReclaimable {
			// Convergent no-op: the job was already queued+pending (a prior sweep
			// requeued it). Do NOT increment the budget; just note it.
			actions = append(actions, map[string]any{
				"workflow_job_id": workflowJobID,
				"job_id":          jobID,
				"action":          action,
				"acted":           false,
				"reason":          "already_reclaimable",
			})
			continue
		}
		if err := recordRecoveryAction(ctx, tx, repositoryID, runID, jobID, counterColumn, action, stallClass); err != nil {
			return nil, err
		}
		// RFC 0101 Phase 3 Slice 2b: when CASE 2 transferred a job away from a
		// still-active stalled owning session, close that session so the parked
		// lane cannot wake up to double-work or reclaim the job a fresh lane now
		// owns. Mirrors the #121 manual flow (the operator did `session close`).
		// Only the session that OWNS this job is touched; interrogation-target
		// sessions are handled by the panel-window logic (closeLeakedInterrogationWindow),
		// not here. The close is guarded on still-active (idempotent).
		ownerClosed := false
		if closeStalledOwner && !sessionAbsent {
			closed, cerr := closeStalledOwningSession(ctx, tx, repositoryID, runID, jobID, sessionID, stallClass)
			if cerr != nil {
				return nil, cerr
			}
			ownerClosed = closed
		}
		actions = append(actions, map[string]any{
			"workflow_job_id":       workflowJobID,
			"job_id":                jobID,
			"action":                action,
			"budget":                counterColumn,
			"count":                 current + 1,
			"limit":                 limit,
			"repo_write":            repoWrite,
			"stall_class":           stallClass,
			"acted":                 true,
			"stalled_owner_closed":  ownerClosed,
			"stalled_owner_session": nullable(sessionID),
		})
	}
	return actions, nil
}

// closeStalledOwningSession closes a still-active stalled session that OWNS a job
// the decision tree just transferred (CASE 2). It is guarded on state='active'
// so a session another actor already closed (or that was never active) is left
// untouched — making it idempotent and safe to re-run. It records a
// session.closed event with the recovery reason so the audit trail is honest.
// It deliberately does NOT touch interrogation-target sessions: those are the
// panel-window logic's responsibility (closeLeakedInterrogationWindow).
func closeStalledOwningSession(ctx context.Context, tx db.TxRunner, repositoryID, runID, jobID, sessionID, stallClass string) (bool, error) {
	if sessionID == "" || sessionID == "<nil>" {
		return false, nil
	}
	// Guard: only close a session that is STILL active (idempotent — a session
	// some other actor already closed, or that was never active, is left alone).
	// The decision tree already holds FOR UPDATE on the owning job; the session
	// row itself is read fresh here so a concurrent close is observed.
	stillActive, err := existsRow(ctx, tx, `
		SELECT 1 FROM striatumd.sessions
		 WHERE repository_id = $1 AND session_id = $2 AND state = 'active'
		 LIMIT 1`, repositoryID, sessionID)
	if err != nil {
		return false, err
	}
	if !stillActive {
		return false, nil
	}
	now := nowString()
	const closeReason = "recovery_stalled_transfer"
	if err := tx.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET state = 'closed', closed_at = $1, close_reason = $2
		 WHERE repository_id = $3 AND session_id = $4 AND state = 'active'`,
		now, closeReason, repositoryID, sessionID); err != nil {
		return false, err
	}
	if _, err := appendEvent(ctx, tx, repositoryID, runID, "session.closed", sessionID, jobID, nil, nil, nil, map[string]any{
		"session_id":  sessionID,
		"reason":      closeReason,
		"source":      "recovery_decision_tree",
		"stall_class": stallClass,
	}); err != nil {
		return false, err
	}
	return true, nil
}

// closeLeakedInterrogationWindow closes a leaked interrogation window owned by
// sessionID for this job: if the session is an interrogable target whose panel
// has no pending consumers, maybeCloseInterrogationTarget retires it. Returns
// whether a window was actually closed. Best-effort + idempotent.
func closeLeakedInterrogationWindow(ctx context.Context, tx db.TxRunner, repositoryID, runID, jobID, sessionID string) (bool, error) {
	// Only attempt a close when the session is currently sitting in an
	// awaiting-interrogation / interrogation target posture; maybeCloseInterrogationTarget
	// itself guards on active state, no open interrogations, no active lease, and
	// no pending panel consumers, so calling it unconditionally is safe but we
	// avoid the work when there is plainly no interrogation row for the session.
	hasInterrogation, err := existsRow(ctx, tx, `
		SELECT 1 FROM striatumd.interrogations
		 WHERE repository_id = $1 AND target_session_id = $2
		 LIMIT 1`, repositoryID, sessionID)
	if err != nil {
		return false, err
	}
	if !hasInterrogation {
		return false, nil
	}
	return maybeCloseInterrogationTarget(ctx, tx, repositoryID, runID, sessionID)
}

// deadAgentExitedUnsealed reports whether a confirmed-dead supervised agent had
// engaged the work protocol and produced output but never sealed it with
// work.complete (#289). The signal is: no work.complete for the owning session,
// AND it both made at least one MCP tool call (LastToolCall* — stamped by any
// tools/call: work.claim_next / await_packet / ack / heartbeat / publish) and
// emitted PTY output (LastPTYActivityAt). This is deliberately coarse — it does
// NOT prove a finished deliverable; it means "claimed a packet and printed
// something, then the process died unsealed." That covers both the
// finished-but-unsealed self-driving exit (turn-end / context-budget / rate-limit
// death) and the got-going-then-crashed case; recovery cannot tell them apart
// (the issue's own premise) and both warrant the same response — a smaller budget
// and an inspect-the-worktree escalation, never sealing on the agent's behalf
// (attestation forbids it). Only the never-engaged early crash (no tool call or
// no output) stays the generic agent_pid_dead class.
func deadAgentExitedUnsealed(activity sessionliveness.Activity) bool {
	if activity.LastWorkCompleteAt != nil {
		return false
	}
	engagedProtocol := activity.LastToolCallStartedAt != nil || activity.LastToolCallFinishedAt != nil
	producedOutput := activity.LastPTYActivityAt != nil
	return engagedProtocol && producedOutput
}

// supervisedAgentConfirmedDead reports whether the owning session's supervised
// agent PROCESS is positively dead — the #147 Symptom B signal that, unlike
// lease/heartbeat freshness, an out-of-band operator `striatum heartbeat` loop
// cannot forge. It requires a recorded supervisor pointer: a pointer already
// marked lost/stopped is conclusive; otherwise the recorded agent is probed with
// the same ProbeLaneLiveness the process reconcile uses (the tmux pane for tmux
// lanes, the PID + start-token for plain lanes, so a reused PID is not mistaken
// for the original). An UNAVAILABLE probe (e.g. tmux not reachable) returns
// false: a lane we cannot positively judge dead is left alone, never requeued.
func supervisedAgentConfirmedDead(ctx context.Context, row map[string]any) bool {
	state := fmt.Sprint(nullable(row["supervisor_pointer_state"]))
	if state == "lost" || state == "stopped" {
		return true
	}
	pid := intFromAny(row["supervisor_pointer_pid"], 0)
	if pid <= 0 {
		return false // no recorded agent process to judge
	}
	metadata := asMap(row["supervisor_pointer_metadata_json"])
	expectedStart := fmt.Sprint(nullable(row["supervisor_pointer_pid_start_time"]))
	if expectedStart == "<nil>" {
		expectedStart = ""
	}
	// #198: in the periodic sweep this reads the pre-tx liveness snapshot
	// (probeLaneLivenessCached), so the `tmux list-panes` / /proc probe never runs
	// while the sweep transaction holds the per-run advisory lock + FOR UPDATE row
	// locks. Operator RPCs and unit tests carry no oracle and probe live.
	live := probeLaneLivenessCached(ctx, metadata, pid, expectedStart)
	switch live.Class {
	case string(gosupervisor.TmuxLivenessUnavailable), gosupervisor.PIDLivenessIdentityUnavailable:
		// Cannot determine; do not requeue a possibly-live lane.
		// pid_identity_unavailable means kill(pid,0) SUCCEEDED (the process is
		// signalable, i.e. alive) but /proc/<pid>/stat was momentarily
		// unreadable — treating that as dead force-expired a LIVE agent's lease
		// (the #145/#147 false-requeue class), and the #198 pre-tx oracle would
		// cache the transient failure for the whole sweep window.
		return false
	}
	return !live.Alive
}

// leaseStaleActive reports whether the job's resolved lease is still 'active'
// (not yet expired) — used to exclude live claimants from the dead-lane requeue
// case even when the owning session looks dead (defensive: a fresh claimant
// would have a brand-new active lease).
func leaseStaleActive(row map[string]any) bool {
	return fmt.Sprint(nullable(row["lease_state"])) == "active"
}

func sessionStateLabel(sessionID, sessionState string) string {
	if sessionID == "" || sessionID == "<nil>" {
		return "absent"
	}
	if sessionState == "" || sessionState == "<nil>" {
		return "unknown"
	}
	return sessionState
}

// isNoRows reports a pgx no-rows error.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
