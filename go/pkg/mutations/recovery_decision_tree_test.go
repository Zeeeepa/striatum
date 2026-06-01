package mutations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// jobRecoveryCounts reads the per-job recovery budget row counters.
func jobRecoveryCounts(t *testing.T, ctx context.Context, runner any, repoID, jobID string) (requeue, transfer int, escalationPending bool) {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT requeue_count, transfer_count, escalation_pending
		  FROM striatumd.job_recovery_state
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil {
		if isNoRows(err) {
			return 0, 0, false
		}
		t.Fatalf("read job_recovery_state for %s: %v", jobID, err)
	}
	return intValue(row["requeue_count"]), intValue(row["transfer_count"]), row["escalation_pending"] == true
}

// recoveryActionsFromSweep extracts the recovery_actions summary from a
// HandleRecoveryAuto / SweepRun result map.
func recoveryActionsFromSweep(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	summary, ok := result["recovery_actions"].(map[string]any)
	if !ok {
		t.Fatalf("recovery_actions missing or wrong type: %#v", result["recovery_actions"])
	}
	return summary
}

// TestSweepAutoRequeuesDeadLaneRunningLimboJob: the RFC 0101 Phase 3 Slice 2
// decision tree, driven by a single SweepRun, reclaims a dead-lane running-limbo
// job (owning session closed, lease released, zero artifacts) on the SAME
// attempt -> queued + pending, requeue_count -> 1, and a fresh session can claim
// it. This is the autonomous in-daemon equivalent of the Slice-1 manual
// `recovery.requeue_stale --force`.
func TestSweepAutoRequeuesDeadLaneRunningLimboJob(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_sweep_dead_lane"
	runID, jobID, _, msgID, downstreamJobID := seedDeadLaneRepoWriteJob(t, ctx, runner, repoID)

	priorAttempt := jobAttempt(t, ctx, runner, repoID, jobID)

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// Surface assertion: recovery_actions reports the requeue.
	summary := recoveryActionsFromSweep(t, result)
	if intValue(summary["acted_count"]) != 1 {
		t.Fatalf("acted_count = %v, want 1; %#v", summary["acted_count"], summary)
	}
	acts, _ := summary["actions"].([]map[string]any)
	if len(acts) != 1 || acts[0]["action"] != "requeue_same_attempt" || acts[0]["acted"] != true {
		t.Fatalf("expected one requeue_same_attempt acted=true; got %#v", summary["actions"])
	}

	// Job back to queued + a pending work message, on the SAME attempt.
	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued", got)
	}
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, jobID); n != 1 {
		t.Fatalf("pending work message count = %d, want 1", n)
	}
	if got := messageState(t, ctx, runner, repoID, msgID); got != "pending" {
		t.Fatalf("reused message state = %q, want pending", got)
	}
	if n := activeLeaseCount(t, ctx, runner, repoID, jobID); n != 0 {
		t.Fatalf("active lease count = %d, want 0", n)
	}
	if got := jobAttempt(t, ctx, runner, repoID, jobID); got != priorAttempt {
		t.Fatalf("attempt = %d, want %d (autonomous requeue must NOT bump attempt)", got, priorAttempt)
	}

	// Budget counter advanced to 1.
	requeue, _, escalation := jobRecoveryCounts(t, ctx, runner, repoID, jobID)
	if requeue != 1 {
		t.Fatalf("requeue_count = %d, want 1", requeue)
	}
	if escalation {
		t.Fatalf("escalation_pending = true, want false on first requeue")
	}

	// Downstream job untouched.
	if got := jobState(t, ctx, runner, repoID, downstreamJobID); got != "blocked" {
		t.Fatalf("downstream job state = %q, want blocked (untouched)", got)
	}

	// A fresh implementer session in the claude lane must now claim the requeued job.
	freshSession := "sess_fresh_" + repoID
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, freshSession, "implementer", "claude", []string{"write"}, "active", 2)
	claim, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": freshSession,
	}))
	if err != nil {
		t.Fatalf("fresh claim of requeued job must succeed, got: %v", err)
	}
	if fmt.Sprint(claim["status"]) != "claimed" {
		t.Fatalf("fresh claim status = %v, want claimed; res=%#v", claim["status"], claim)
	}
	if got := fmt.Sprint(asMap(asMap(claim["packet"])["job"])["job_id"]); got != jobID {
		t.Fatalf("fresh claim packet job_id = %v, want %s", got, jobID)
	}
}

// TestSweepDeadLaneRequeueIsConvergent: re-running the sweep on the now
// queued+pending job is a no-op — the requeue_count does not climb and no
// duplicate pending message is minted. (G: idempotent + convergent under the
// 60s re-run.)
func TestSweepDeadLaneRequeueIsConvergent(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_sweep_convergent"
	runID, jobID, _, _, _ := seedDeadLaneRepoWriteJob(t, ctx, runner, repoID)

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	requeueAfterFirst, _, _ := jobRecoveryCounts(t, ctx, runner, repoID, jobID)
	if requeueAfterFirst != 1 {
		t.Fatalf("requeue_count after first sweep = %d, want 1", requeueAfterFirst)
	}

	// The job is now queued (not claimed/running/stale_lease), so the decision
	// tree's scan no longer selects it at all — the cleanest convergence.
	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued", got)
	}

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	summary := recoveryActionsFromSweep(t, result)
	if intValue(summary["acted_count"]) != 0 {
		t.Fatalf("second sweep acted_count = %v, want 0 (convergent); %#v", summary["acted_count"], summary)
	}

	requeueAfterSecond, _, escalation := jobRecoveryCounts(t, ctx, runner, repoID, jobID)
	if requeueAfterSecond != requeueAfterFirst {
		t.Fatalf("requeue_count climbed to %d on convergent re-run, want %d", requeueAfterSecond, requeueAfterFirst)
	}
	if escalation {
		t.Fatalf("escalation_pending = true, want false (still under budget)")
	}
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, jobID); n != 1 {
		t.Fatalf("pending work message count = %d, want exactly 1 (no duplicate)", n)
	}
}

// TestSweepDoesNotRequeueLiveLocalWorkJob: a job whose owning session is
// producing fresh PTY output (working_local) is NOT requeued — only genuinely
// stuck (dead/stalled) jobs are touched.
func TestSweepDoesNotRequeueLiveLocalWorkJob(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_sweep_live_local"
	runID := "run_" + repoID
	jobID := "job_impl_" + repoID
	leaseID := "lease_live_" + repoID
	msgID := "msg_impl_" + repoID
	sessionID := "sess_live_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "implement", "type": "build", "role_id": "implementer"}},
	})
	// An ACTIVE session that is producing fresh PTY output right now.
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "implementer", "claude", []string{"write"}, "active")
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET last_pty_activity_at = $3, last_tools_list_at = $3
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID, now); err != nil {
		t.Fatalf("set fresh pty activity: %v", err)
	}

	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, current_message_id, current_lease_id, created_at, started_at
		) VALUES ($1,$2,$3,'implement',1,'running','implementer','Implement','build',
		          'idem_impl_'||$1,'[]'::jsonb,'{}'::jsonb,'{"lane_id":"claude"}'::jsonb,$4,$5,$6,$6)`,
		repoID, jobID, runID, msgID, leaseID, now); err != nil {
		t.Fatalf("insert running job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','acked',0,'implementer','claude',$5,$5)`,
		repoID, msgID, runID, jobID, now); err != nil {
		t.Fatalf("insert work message: %v", err)
	}
	// A fresh ACTIVE lease, heartbeating, not expired.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, last_heartbeat_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',NOW(),NOW() + INTERVAL '10 minutes',NOW())`,
		repoID, leaseID, runID, jobID, sessionID); err != nil {
		t.Fatalf("insert active lease: %v", err)
	}

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	summary := recoveryActionsFromSweep(t, result)
	if intValue(summary["acted_count"]) != 0 {
		t.Fatalf("acted_count = %v, want 0 (live working_local job must NOT be requeued); %#v", summary["acted_count"], summary)
	}
	// Job untouched: still running, still holding its active lease.
	if got := jobState(t, ctx, runner, repoID, jobID); got != "running" {
		t.Fatalf("job state = %q, want running (untouched)", got)
	}
	if n := activeLeaseCount(t, ctx, runner, repoID, jobID); n != 1 {
		t.Fatalf("active lease count = %d, want 1 (untouched)", n)
	}
	requeue, _, _ := jobRecoveryCounts(t, ctx, runner, repoID, jobID)
	if requeue != 0 {
		t.Fatalf("requeue_count = %d, want 0 (no action on live job)", requeue)
	}
}

// TestSweepBudgetExhaustionSetsEscalationPending: after max_requeues requeues
// (default 2), the sweep stops requeuing and flags escalation_pending instead.
// We simulate the budget being already at the limit, re-create the dead-lane
// limbo, and assert the sweep refuses to act + sets escalation_pending.
func TestSweepBudgetExhaustionSetsEscalationPending(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_sweep_budget"
	runID, jobID, _, _, _ := seedDeadLaneRepoWriteJob(t, ctx, runner, repoID)

	// Pre-seed the budget at the default max_requeues (2) so this sweep is the
	// over-budget attempt. The job is still in dead-lane running-limbo.
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_recovery_state (
		  repository_id, run_id, job_id, requeue_count, last_recovery_action,
		  last_recovery_at, created_at, updated_at
		) VALUES ($1,$2,$3,2,'requeue_same_attempt',$4,$4,$4)`,
		repoID, runID, jobID, now); err != nil {
		t.Fatalf("preseed exhausted budget: %v", err)
	}

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	summary := recoveryActionsFromSweep(t, result)
	if intValue(summary["escalation_pending_count"]) != 1 {
		t.Fatalf("escalation_pending_count = %v, want 1; %#v", summary["escalation_pending_count"], summary)
	}
	acts, _ := summary["actions"].([]map[string]any)
	if len(acts) != 1 || acts[0]["acted"] != false || acts[0]["escalation_pending"] != true {
		t.Fatalf("expected one un-acted escalation action; got %#v", summary["actions"])
	}

	// The job was NOT requeued: still in running-limbo.
	if got := jobState(t, ctx, runner, repoID, jobID); got != "running" {
		t.Fatalf("job state = %q, want running (budget exhausted, NOT requeued)", got)
	}
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, jobID); n != 0 {
		t.Fatalf("pending work message count = %d, want 0 (no requeue past budget)", n)
	}
	// Budget row flags escalation_pending; requeue_count did not climb past 2.
	requeue, _, escalation := jobRecoveryCounts(t, ctx, runner, repoID, jobID)
	if requeue != 2 {
		t.Fatalf("requeue_count = %d, want 2 (must not increment past budget)", requeue)
	}
	if !escalation {
		t.Fatalf("escalation_pending = false, want true after budget exhaustion")
	}

	// escalated_at was stamped.
	var escalatedSet bool
	row, err := oneRow(ctx, runner, `
		SELECT escalated_at IS NOT NULL AS set FROM striatumd.job_recovery_state
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil {
		t.Fatalf("read escalated_at: %v", err)
	}
	escalatedSet = row["set"] == true
	if !escalatedSet {
		t.Fatalf("escalated_at not set after escalation")
	}
}

// TestRecoveryPolicyFromWorkflowOverrides: the budgets are read from the
// workflow's recovery_policy block, honoring the documented RFC 0020
// max_total_requeues_per_job fallback and the slice-2 max_requeues/max_transfers.
func TestRecoveryPolicyFromWorkflowOverrides(t *testing.T) {
	// Defaults when block absent.
	def := recoveryPolicyFromWorkflow(map[string]any{})
	if def.maxRequeues != defaultMaxRequeues || def.maxTransfers != defaultMaxTransfers {
		t.Fatalf("defaults = %+v, want {%d %d}", def, defaultMaxRequeues, defaultMaxTransfers)
	}
	// Slice-2 explicit keys win.
	p := recoveryPolicyFromWorkflow(map[string]any{
		"recovery_policy": map[string]any{
			"max_requeues":  float64(5),
			"max_transfers": float64(7),
		},
	})
	if p.maxRequeues != 5 || p.maxTransfers != 7 {
		t.Fatalf("explicit override = %+v, want {5 7}", p)
	}
	// RFC 0020 max_total_requeues_per_job is the requeue fallback.
	q := recoveryPolicyFromWorkflow(map[string]any{
		"recovery_policy": map[string]any{
			"max_total_requeues_per_job": float64(4),
		},
	})
	if q.maxRequeues != 4 {
		t.Fatalf("rfc-0020 fallback maxRequeues = %d, want 4", q)
	}
	if q.maxTransfers != defaultMaxTransfers {
		t.Fatalf("maxTransfers = %d, want default %d", q.maxTransfers, defaultMaxTransfers)
	}
}

// seedLeakedInterrogationWindow seeds a run with an interrogable target session
// still holding a job in claimed/running limbo and an interrogation row that has
// been closed (the panel finished) leaving the target session live with no
// active lease and no open interrogations — a leaked window the sweep closes.
func seedLeakedInterrogationWindow(t *testing.T, ctx context.Context, runner db.Runner, repoID string) (runID, targetSession, jobID string) {
	t.Helper()
	runID = "run_" + repoID
	targetSession = "sess_target_" + repoID
	jobID = "job_design_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"designer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "design", "type": "build", "role_id": "designer"}},
	})
	// The interrogable target session is still active (leaked window) with no
	// active lease and a job in limbo so the decision-tree scan visits it.
	intgSeedSession(t, ctx, runner, repoID, runID, targetSession, "designer", "claude", nil, "active")

	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, current_lease_id, created_at, started_at
		) VALUES ($1,$2,$3,'design',1,'running','designer','Design','build',
		          'idem_design_'||$1,'[]'::jsonb,'{}'::jsonb,'{"lane_id":"claude"}'::jsonb,NULL,$4,$4)`,
		repoID, jobID, runID, now); err != nil {
		t.Fatalf("insert interrogable job: %v", err)
	}
	// A released lease whose owner is the target session, so the decision-tree
	// scan resolves the owning session.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,'lease_'||$2,$3,'job',$2,$4,'released',NOW() - INTERVAL '1 hour',NOW() - INTERVAL '30 minutes',NOW() - INTERVAL '20 minutes','session_idle')`,
		repoID, jobID, runID, targetSession); err != nil {
		t.Fatalf("insert released lease: %v", err)
	}
	// A CLOSED interrogation against the target session — the panel finished but
	// the target window was never retired (the leak).
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.interrogations (
		  repository_id, interrogation_id, run_id, interrogator_session_id,
		  target_session_id, topic, state, opened_at, closed_at
		) VALUES ($1,'intg_'||$2,$3,'sess_reviewer_'||$2,$4,'design review','closed',
		          NOW() - INTERVAL '40 minutes', NOW() - INTERVAL '25 minutes')`,
		repoID, jobID, runID, targetSession); err != nil {
		t.Fatalf("insert closed interrogation: %v", err)
	}
	return runID, targetSession, jobID
}

// TestSweepClosesLeakedInterrogationWindow: an awaiting-interrogation target
// session whose panel finished (interrogation closed) but whose session was
// never retired (no active lease, no open interrogations, no pending consumers)
// is closed by the sweep.
func TestSweepClosesLeakedInterrogationWindow(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_sweep_leaked_intg"
	runID, targetSession, _ := seedLeakedInterrogationWindow(t, ctx, runner, repoID)

	// Sanity: the target session is active before the sweep.
	if got := sessionStateOf(t, ctx, runner, repoID, targetSession); got != "active" {
		t.Fatalf("pre-sweep target session state = %q, want active", got)
	}

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	summary := recoveryActionsFromSweep(t, result)
	acts, _ := summary["actions"].([]map[string]any)
	foundClose := false
	for _, a := range acts {
		if a["action"] == "interrogation_window_closed" {
			foundClose = true
		}
	}
	if !foundClose {
		t.Fatalf("expected an interrogation_window_closed action; got %#v", summary["actions"])
	}

	// The leaked target session is now closed.
	if got := sessionStateOf(t, ctx, runner, repoID, targetSession); got != "closed" {
		t.Fatalf("post-sweep target session state = %q, want closed", got)
	}
}

func sessionStateOf(t *testing.T, ctx context.Context, runner any, repoID, sessionID string) string {
	t.Helper()
	row, err := oneRow(ctx, runner, `SELECT state FROM striatumd.sessions WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID)
	if err != nil {
		t.Fatalf("read session state for %s: %v", sessionID, err)
	}
	return fmt.Sprint(row["state"])
}
