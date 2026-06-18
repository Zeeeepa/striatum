package mutations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/reads"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestSweepRequeueResolvesDanglingAutonomousBlocker is #373A: a dead-lane
// running-limbo job carrying an open NON-escalation autonomous blocker (an
// agent-raised branch_contamination, severity='blocked', is_escalation=false) is
// requeued by the sweep AND its dangling blocker is resolved, so the requeued
// attempt can proceed instead of wedging behind its own open blocker. Before the
// fix the requeue left blk_* open forever (the #373A repro).
func TestSweepRequeueResolvesDanglingAutonomousBlocker(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_373a_dangling_blocker"
	runID, jobID, _, _, _ := seedDeadLaneRepoWriteJob(t, ctx, runner, repoID)

	// The codex diverger raised a branch_contamination blocker (severity blocked,
	// NOT an escalation) before its session exited unsealed. The blocker carries
	// the now-dead session id (the lane that raised it) — a valid existing row.
	deadSession := "sess_dead_" + repoID
	blockerID := "blk_branch_contamination_" + repoID
	seedOpenBlocker(t, ctx, runner, repoID, runID, jobID, deadSession, blockerID, "blocked", "branch_contamination")

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// The job was requeued.
	summary := recoveryActionsFromSweep(t, result)
	if intValue(summary["acted_count"]) != 1 {
		t.Fatalf("acted_count = %v, want 1; %#v", summary["acted_count"], summary)
	}
	acts, _ := summary["actions"].([]map[string]any)
	if len(acts) != 1 || acts[0]["action"] != "requeue_same_attempt" || acts[0]["acted"] != true {
		t.Fatalf("expected one requeue_same_attempt acted=true; got %#v", summary["actions"])
	}
	if got := intValue(acts[0]["dangling_blockers_resolved"]); got != 1 {
		t.Fatalf("dangling_blockers_resolved = %d, want 1", got)
	}

	// The dangling autonomous blocker is now resolved.
	if got := blockerState(t, ctx, runner, repoID, blockerID); got != "resolved" {
		t.Fatalf("branch_contamination blocker state = %q, want resolved (#373A: requeue must clear it)", got)
	}
	// Job back to queued + claimable.
	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued", got)
	}
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, jobID); n != 1 {
		t.Fatalf("pending work message count = %d, want 1", n)
	}
}

// TestSweepRequeueLeavesEscalationBlockerOpen is the #373A guardrail: the requeue
// path must NEVER auto-resolve an escalation-class blocker (here a
// human_checkpoint and an ambiguous_goal). It is the same dead-lane requeue, but
// the blockers stay open for the human.
func TestSweepRequeueLeavesEscalationBlockerOpen(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_373a_escalation_blocker"
	runID, jobID, _, _, _ := seedDeadLaneRepoWriteJob(t, ctx, runner, repoID)

	// A human_checkpoint and an escalation-class kind, both on the requeued job.
	deadSession := "sess_dead_" + repoID
	checkpointID := "blk_human_checkpoint_" + repoID
	escalationID := "blk_ambiguous_goal_" + repoID
	seedOpenBlocker(t, ctx, runner, repoID, runID, jobID, deadSession, checkpointID, "human_checkpoint", "needs_human")
	seedOpenBlocker(t, ctx, runner, repoID, runID, jobID, deadSession, escalationID, "blocked", "ambiguous_goal")

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if got := blockerState(t, ctx, runner, repoID, checkpointID); got != "open" {
		t.Fatalf("human_checkpoint blocker state = %q, want open (must be left for the human)", got)
	}
	if got := blockerState(t, ctx, runner, repoID, escalationID); got != "open" {
		t.Fatalf("ambiguous_goal escalation blocker state = %q, want open (must be left for the human)", got)
	}
}

// seedNoLeaseAttentionSession seeds the #373B repro: a still-active session in
// the ProtocolAttention posture (agent_escalation_pending) holding a job in
// running-limbo with NO active lease. last_session_escalate_at is the newest
// activity timestamp (so attentionPending fires) and there is no fresh
// progress/PTY/tool-call signal. Returns the run/job/owning-session ids.
func seedNoLeaseAttentionSession(t *testing.T, ctx context.Context, runner db.Runner, repoID string) (runID, jobID, sessionID string) {
	t.Helper()
	runID = "run_" + repoID
	jobID = "job_impl_" + repoID
	leaseID := "lease_attn_" + repoID
	msgID := "msg_impl_" + repoID
	sessionID = "sess_attn_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "implement", "type": "build", "role_id": "implementer"}},
	})
	// Active session, escalation-pending: last_session_escalate_at is newer than
	// every other signal, no progress after it -> ProtocolAttention.
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "implementer", "claude", []string{"write"}, "active")
	stale := time.Now().UTC().Add(-30 * time.Minute)
	escalateAt := time.Now().UTC().Add(-13 * time.Minute)
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET registered_at = $3,
		       last_session_heartbeat_at = $3,
		       last_mcp_request_at = $3,
		       last_session_escalate_at = $4,
		       last_pty_activity_at = NULL,
		       last_tool_call_started_at = NULL,
		       last_tool_call_finished_at = NULL
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID, stale, escalateAt); err != nil {
		t.Fatalf("set escalation-pending activity columns: %v", err)
	}

	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, current_message_id, current_lease_id, created_at, started_at
		) VALUES ($1,$2,$3,'implement',1,'running','implementer','Implement','build',
		          'idem_impl_'||$1,'[]'::jsonb,'{}'::jsonb,'{"lane_id":"claude"}'::jsonb,$4,NULL,$5,$5)`,
		repoID, jobID, runID, msgID, now); err != nil {
		t.Fatalf("insert running attention-lane job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','acked',0,'implementer','claude',$5,$5)`,
		repoID, msgID, runID, jobID, now); err != nil {
		t.Fatalf("insert work message: %v", err)
	}
	// A RELEASED lease (no_lease) owned by the still-active escalation-pending
	// session, so the decision-tree scan resolves the owning session.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'released',NOW() - INTERVAL '1 hour',NOW() - INTERVAL '30 minutes',NOW() - INTERVAL '30 minutes','session_idle')`,
		repoID, leaseID, runID, jobID, sessionID); err != nil {
		t.Fatalf("insert released lease: %v", err)
	}
	return runID, jobID, sessionID
}

// TestSweepReapsNoLeaseAttentionSessionWithoutHumanBlocker is #373B path (1): a
// still-active, no-lease session in agent_escalation_pending with NO genuine
// human-attention blocker is reaped (closed with recovery_stalled_transfer) and
// the job is transfer-requeued so a fresh lane can take it. Before the fix this
// fell to the default branch and (confirmedDead indeterminate) was skipped — the
// job wedged forever until a manual `session close`.
func TestSweepReapsNoLeaseAttentionSessionWithoutHumanBlocker(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_373b_attention_reap"
	runID, jobID, sessionID := seedNoLeaseAttentionSession(t, ctx, runner, repoID)

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	summary := recoveryActionsFromSweep(t, result)
	if intValue(summary["acted_count"]) != 1 {
		t.Fatalf("acted_count = %v, want 1 (no-lease attention session must be reaped); %#v", summary["acted_count"], summary)
	}
	acts, _ := summary["actions"].([]map[string]any)
	if len(acts) != 1 || acts[0]["action"] != "transfer_requeue" || acts[0]["acted"] != true {
		t.Fatalf("expected one transfer_requeue acted=true; got %#v", summary["actions"])
	}

	// The escalation-pending session is closed with the recovery reason.
	if got := sessionStateOf(t, ctx, runner, repoID, sessionID); got != "closed" {
		t.Fatalf("attention session state = %q, want closed (reaped)", got)
	}
	if got := sessionCloseReasonOf(t, ctx, runner, repoID, sessionID); got != "recovery_stalled_transfer" {
		t.Fatalf("attention session close_reason = %q, want recovery_stalled_transfer", got)
	}
	// Job back to queued + claimable.
	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued", got)
	}
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, jobID); n != 1 {
		t.Fatalf("pending work message count = %d, want 1", n)
	}
}

// TestSweepDoesNotReapAttentionSessionWithHumanCheckpoint is the #373B
// CRITICAL GUARDRAIL: a no-lease attention session that DID raise a genuine
// human_checkpoint blocker must NOT be reaped — it is left for the human. The
// only difference from the reap test above is the open human_checkpoint blocker.
func TestSweepDoesNotReapAttentionSessionWithHumanCheckpoint(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_373b_attention_human_gate"
	runID, jobID, sessionID := seedNoLeaseAttentionSession(t, ctx, runner, repoID)

	// A genuine human gate: an open human_checkpoint blocker on the job.
	checkpointID := "blk_human_checkpoint_" + repoID
	seedOpenBlocker(t, ctx, runner, repoID, runID, jobID, sessionID, checkpointID, "human_checkpoint", "needs_human")

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	summary := recoveryActionsFromSweep(t, result)
	if intValue(summary["acted_count"]) != 0 {
		t.Fatalf("acted_count = %v, want 0 (human-gated attention session must NOT be reaped); %#v", summary["acted_count"], summary)
	}
	// The session is left active for the human.
	if got := sessionStateOf(t, ctx, runner, repoID, sessionID); got != "active" {
		t.Fatalf("human-gated attention session state = %q, want active (left for the human)", got)
	}
	// The human_checkpoint blocker is left open.
	if got := blockerState(t, ctx, runner, repoID, checkpointID); got != "open" {
		t.Fatalf("human_checkpoint blocker state = %q, want open", got)
	}
	// Job left running (not requeued).
	if got := jobState(t, ctx, runner, repoID, jobID); got != "running" {
		t.Fatalf("job state = %q, want running (untouched)", got)
	}
}

// seedRecoveryExhaustedSessionlessJob seeds the #388 wedge: a job left in
// 'running' with a dead session and a SPENT requeue budget, plus its open
// recovery_exhausted escalation (blocker + escalation_inbox) and the run flipped
// to needs_operator — exactly the state after the autonomous recovery loop
// exhausts and the driver exits. Returns the run/job/escalation ids.
func seedRecoveryExhaustedSessionlessJob(t *testing.T, ctx context.Context, runner db.Runner, repoID string) (runID, jobID, escalationID string) {
	t.Helper()
	runID = "run_" + repoID
	jobID = "job_review_" + repoID
	leaseID := "lease_dead_" + repoID
	msgID := "msg_review_" + repoID
	deadSession := "sess_dead_" + repoID
	escalationID = "blk_recovery_exhausted_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"reviewer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "review", "type": "build", "role_id": "reviewer"}},
	})
	// Flip the run to needs_operator (the escalation already fired).
	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs SET state = 'needs_operator'
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID); err != nil {
		t.Fatalf("flip run to needs_operator: %v", err)
	}
	// The owning session is CLOSED (the lane exited unsealed).
	intgSeedSession(t, ctx, runner, repoID, runID, deadSession, "reviewer", "claude", []string{"write"}, "closed")

	now := time.Now().UTC()
	// A non-verdict-capable build job left in running-limbo (lease released).
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, current_message_id, current_lease_id, created_at, started_at
		) VALUES ($1,$2,$3,'review',1,'running','reviewer','Review','build',
		          'idem_review_'||$1,'[]'::jsonb,'{}'::jsonb,'{"lane_id":"claude"}'::jsonb,$4,NULL,$5,$5)`,
		repoID, jobID, runID, msgID, now); err != nil {
		t.Fatalf("insert running-limbo job: %v", err)
	}
	// The work message it was working, left in a terminal state (dead), so requeue
	// mints a fresh pending one.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','dead',0,'reviewer','claude',$5,$5)`,
		repoID, msgID, runID, jobID, now); err != nil {
		t.Fatalf("insert work message: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'released',NOW() - INTERVAL '2 hours',NOW() - INTERVAL '1 hour',NOW() - INTERVAL '30 minutes','session_closed')`,
		repoID, leaseID, runID, jobID, deadSession); err != nil {
		t.Fatalf("insert released lease: %v", err)
	}
	// The spent budget: requeue exhausted, escalation_pending=true, run escalated.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_recovery_state (
		  repository_id, run_id, job_id, requeue_count, transfer_count, respawn_count,
		  escalation_pending, escalated_at, run_escalated_at,
		  last_recovery_action, last_recovery_at, last_stall_class, created_at, updated_at
		) VALUES ($1,$2,$3,1,0,0,true,$4,$4,
		          'requeue_same_attempt_budget_exhausted',$4,'agent_exited_unsealed',$4,$4)`,
		repoID, runID, jobID, now); err != nil {
		t.Fatalf("insert exhausted budget: %v", err)
	}
	// The open recovery_exhausted blocker + escalation_inbox row.
	payloadArg, err := db.JSONBArg(runner, map[string]any{
		"schema_version": "striatum.recovery_escalation.v1",
		"is_escalation":  true,
		"blocker_kind":   "recovery_exhausted",
		"stall_class":    "agent_exited_unsealed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.blockers (
		  repository_id, blocker_id, run_id, job_id, session_id, severity,
		  blocker_kind, description, state, created_at, payload_json
		) VALUES ($1,$2,$3,$4,NULL,'blocked','recovery_exhausted','autonomous recovery exhausted','open',$5,$6::jsonb)`,
		repoID, escalationID, runID, jobID, now, payloadArg); err != nil {
		t.Fatalf("insert recovery_exhausted blocker: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.escalation_inbox (
		  repository_id, escalation_id, run_id, job_id, session_id,
		  blocker_id, blocker_kind, severity, state, created_at, payload_json
		) VALUES ($1,$2,$3,$4,NULL,$2,'recovery_exhausted','blocked','pending',$5,$6::jsonb)`,
		repoID, escalationID, runID, jobID, now, payloadArg); err != nil {
		t.Fatalf("insert escalation inbox: %v", err)
	}
	return runID, jobID, escalationID
}

// TestEscalationResolveRedispatchesExhaustedSessionlessJob is #388: resolving a
// recovery_exhausted escalation on a sessionless 'running' job re-dispatches it —
// the job returns to 'queued' with a re-pended work message, the recovery budget
// is reset (escalation_pending=false, requeue_count=0), and a re-run of the sweep
// does NOT immediately re-escalate. The hook is registered as in production
// (mutations.Register); here we wire just the RecoveryExhaustedRedispatchHook +
// RunLockHook to exercise the redispatch through the real escalation.resolve.
func TestEscalationResolveRedispatchesExhaustedSessionlessJob(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_388_redispatch"
	runID, jobID, escalationID := seedRecoveryExhaustedSessionlessJob(t, ctx, runner, repoID)

	// Wire the hooks the daemon registers in production (reads cannot import
	// mutations, so the redispatch lives behind a hook). RunCompletionRedriveHook
	// is intentionally left as-is; on the redispatch path it must NOT be invoked.
	prevRedispatch := reads.RecoveryExhaustedRedispatchHook
	prevLock := reads.RunLockHook
	reads.RecoveryExhaustedRedispatchHook = func(ctx context.Context, tx db.TxRunner, repositoryID, runID, jobID string) (bool, error) {
		return redispatchRecoveryExhaustedJob(ctx, tx, repositoryID, runID, jobID)
	}
	reads.RunLockHook = func(ctx context.Context, tx db.TxRunner, repositoryID, runID string) error {
		return LockRun(ctx, tx, repositoryID, runID)
	}
	t.Cleanup(func() {
		reads.RecoveryExhaustedRedispatchHook = prevRedispatch
		reads.RunLockHook = prevLock
	})

	if _, err := reads.HandleEscalationResolve(ctx, runner, rpc.Envelope{Params: map[string]any{
		"repository_id":   repoID,
		"escalation_id":   escalationID,
		"resolution_note": "re-dispatch the exhausted job",
	}}); err != nil {
		t.Fatalf("escalation resolve: %v", err)
	}

	// The job is back to queued with a fresh pending work message.
	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued (#388: escalation resolve must re-dispatch)", got)
	}
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, jobID); n != 1 {
		t.Fatalf("pending work message count = %d, want 1", n)
	}
	// The recovery budget is reset.
	requeue, transfer, escalation := jobRecoveryCounts(t, ctx, runner, repoID, jobID)
	if requeue != 0 || transfer != 0 {
		t.Fatalf("budget after redispatch = {requeue:%d transfer:%d}, want {0 0}", requeue, transfer)
	}
	if escalation {
		t.Fatalf("escalation_pending = true, want false after redispatch")
	}
	// run_escalated_at cleared so a genuinely-recurring stall can re-escalate.
	row, err := oneRow(ctx, runner, `
		SELECT run_escalated_at IS NULL AS cleared FROM striatumd.job_recovery_state
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil {
		t.Fatalf("read run_escalated_at: %v", err)
	}
	if row["cleared"] != true {
		t.Fatalf("run_escalated_at not cleared after redispatch")
	}
	// The escalation blocker is resolved and the run is back to running.
	if got := blockerState(t, ctx, runner, repoID, escalationID); got != "resolved" {
		t.Fatalf("recovery_exhausted blocker state = %q, want resolved", got)
	}
	if got := runStateOf(t, ctx, runner, repoID, runID); got != "running" {
		t.Fatalf("run state = %q, want running", got)
	}

	// A fresh reviewer session can claim the re-dispatched job.
	freshSession := "sess_fresh_" + repoID
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, freshSession, "reviewer", "claude", []string{"write"}, "active", 2)
	intgAttest(t, ctx, runner, repoID, runID, freshSession, "claude")
	claim, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": freshSession,
	}))
	if err != nil {
		t.Fatalf("fresh claim of re-dispatched job must succeed, got: %v", err)
	}
	if fmt.Sprint(claim["status"]) != "claimed" {
		t.Fatalf("fresh claim status = %v, want claimed; res=%#v", claim["status"], claim)
	}
}

// TestEscalationResolveRedispatchSweepDoesNotReescalate is the #388 companion:
// after the redispatch resets the budget, a fresh sweep must NOT immediately
// re-escalate the (now claimed-by-a-live-lane) job. We re-dispatch, claim with a
// live session, then sweep and assert no new recovery_exhausted escalation.
func TestEscalationResolveRedispatchSweepDoesNotReescalate(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_388_no_reescalate"
	runID, jobID, escalationID := seedRecoveryExhaustedSessionlessJob(t, ctx, runner, repoID)

	prevRedispatch := reads.RecoveryExhaustedRedispatchHook
	prevLock := reads.RunLockHook
	reads.RecoveryExhaustedRedispatchHook = func(ctx context.Context, tx db.TxRunner, repositoryID, runID, jobID string) (bool, error) {
		return redispatchRecoveryExhaustedJob(ctx, tx, repositoryID, runID, jobID)
	}
	reads.RunLockHook = func(ctx context.Context, tx db.TxRunner, repositoryID, runID string) error {
		return LockRun(ctx, tx, repositoryID, runID)
	}
	t.Cleanup(func() {
		reads.RecoveryExhaustedRedispatchHook = prevRedispatch
		reads.RunLockHook = prevLock
	})

	if _, err := reads.HandleEscalationResolve(ctx, runner, rpc.Envelope{Params: map[string]any{
		"repository_id": repoID,
		"escalation_id": escalationID,
	}}); err != nil {
		t.Fatalf("escalation resolve: %v", err)
	}

	// A fresh live lane claims the re-dispatched job.
	freshSession := "sess_fresh_" + repoID
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, freshSession, "reviewer", "claude", []string{"write"}, "active", 2)
	intgAttest(t, ctx, runner, repoID, runID, freshSession, "claude")
	if _, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": freshSession,
	})); err != nil {
		t.Fatalf("fresh claim: %v", err)
	}
	// Keep the fresh session demonstrably live so the sweep does not requeue it.
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions SET last_mcp_request_at = NOW(), last_pty_activity_at = NOW()
		 WHERE repository_id = $1 AND session_id = $2`, repoID, freshSession); err != nil {
		t.Fatalf("freshen claiming session: %v", err)
	}

	// Re-run the sweep: the budget was reset, so it must NOT re-escalate.
	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("re-sweep: %v", err)
	}
	summary := recoveryActionsFromSweep(t, result)
	if intValue(summary["escalation_pending_count"]) != 0 {
		t.Fatalf("escalation_pending_count = %v, want 0 (budget was reset; must not re-escalate); %#v", summary["escalation_pending_count"], summary)
	}
	_, _, escalation := jobRecoveryCounts(t, ctx, runner, repoID, jobID)
	if escalation {
		t.Fatalf("escalation_pending = true after re-sweep, want false (budget reset must hold)")
	}
}
