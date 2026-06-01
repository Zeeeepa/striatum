package mutations

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// seedDeadLaneRepoWriteJob seeds the RFC 0101 Phase 3 Slice 1 (#121) failure: a
// supervised implement lane died (operator `session close`, dead pane, missed
// heartbeat) leaving its repo-write job in "running-limbo":
//   - jobs.state='running', current_lease_id=NULL, zero artifacts published,
//   - the lease already 'released' (NOT 'expired') so the existing
//     HandleRecoveryRequeueStale JOIN to an expired lease finds nothing,
//   - the owning session 'closed'.
//
// It also seeds a downstream job 'verify' BLOCKED on the implement job so the
// test can assert the operational requeue leaves downstream untouched.
//
// Returns the run/implement-job/released-lease/work-message/downstream-job ids.
func seedDeadLaneRepoWriteJob(t *testing.T, ctx context.Context, runner db.Runner, repoID string) (runID, jobID, leaseID, msgID, downstreamJobID string) {
	t.Helper()
	runID = "run_" + repoID
	jobID = "job_impl_" + repoID
	leaseID = "lease_dead_" + repoID
	msgID = "msg_impl_" + repoID
	downstreamJobID = "job_verify_" + repoID
	deadSession := "sess_dead_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}, "verifier": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs": []any{
			map[string]any{"id": "implement", "type": "build", "role_id": "implementer"},
			map[string]any{"id": "verify", "type": "review", "role_id": "verifier"},
		},
	})
	// The owning session is CLOSED — the lane died.
	intgSeedSession(t, ctx, runner, repoID, runID, deadSession, "implementer", "claude", []string{"write"}, "closed")

	now := time.Now().UTC()
	wsArg, err := db.JSONBArg(runner, map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []any{"src/"}})
	if err != nil {
		t.Fatal(err)
	}
	// The implement job is in running-limbo: running, lease pointer cleared, lane
	// selector pinned so a fresh claude implementer session can re-claim it.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, current_lease_id, created_at, started_at
		) VALUES ($1,$2,$3,'implement',1,'running','implementer','Implement','build',
		          'idem_impl_'||$1,'[]'::jsonb,$4::jsonb,'{"lane_id":"claude"}'::jsonb,NULL,$5,$5)`,
		repoID, jobID, runID, wsArg, now); err != nil {
		t.Fatalf("insert running-limbo repo-write job: %v", err)
	}
	// The work message it was working — left acked (the lane had claimed it). A
	// live (non-terminal) message that requeue must flip back to pending.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','acked',0,'implementer','claude',$5,$5)`,
		repoID, msgID, runID, jobID, now); err != nil {
		t.Fatalf("insert work message: %v", err)
	}
	// The lease is already RELEASED (the dead lane / session-close released it) —
	// this is the crux: there is no 'expired' lease for the JOIN to find.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'released',NOW() - INTERVAL '2 hours',NOW() - INTERVAL '1 hour',NOW() - INTERVAL '30 minutes','session_closed')`,
		repoID, leaseID, runID, jobID, deadSession); err != nil {
		t.Fatalf("insert released lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET current_message_id = $3
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID, msgID); err != nil {
		t.Fatalf("link job message pointer: %v", err)
	}
	// Downstream job blocked on the implement job.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  created_at
		) VALUES ($1,$2,$3,'verify',1,'blocked','verifier','Verify','review',
		          'idem_verify_'||$1,'[]'::jsonb,'{}'::jsonb,$4)`,
		repoID, downstreamJobID, runID, now); err != nil {
		t.Fatalf("insert downstream job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id)
		VALUES ($1,$2,$3)`, repoID, downstreamJobID, jobID); err != nil {
		t.Fatalf("insert job dependency: %v", err)
	}
	return runID, jobID, leaseID, msgID, downstreamJobID
}

// #121 ask #1: a dead-lane repo-write job in running-limbo (running, lease
// released, owning session closed, zero artifacts) must be reclaimable on the
// SAME attempt via `recovery.requeue_stale --force --justification`. Today the
// verb errors "no stale expired lease" because there is no expired lease for
// its JOIN; the fix routes the running-limbo case through requeueJobSameAttempt.
func TestRequeueStaleForceReclaimsDeadLaneRepoWriteSameAttempt(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_dead_lane_requeue"
	runID, jobID, _, msgID, downstreamJobID := seedDeadLaneRepoWriteJob(t, ctx, runner, repoID)

	priorAttempt := jobAttempt(t, ctx, runner, repoID, jobID)

	result, err := HandleRecoveryRequeueStale(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "job_id": jobID, "force": true,
		"justification": "dead implement lane (operator session close); zero artifacts, returning to queue for a fresh lane",
	}))
	if err != nil {
		t.Fatalf("force requeue of dead-lane repo-write job: %v", err)
	}
	if result["status"] != "requeued" {
		t.Fatalf("status = %v, want requeued; %#v", result["status"], result)
	}
	if result["operator_override"] != true {
		t.Fatalf("expected operator_override=true, got %#v", result)
	}
	if result["repo_write"] != true {
		t.Fatalf("expected repo_write=true, got %#v", result)
	}

	// Job back to queued, claimable.
	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state = %q, want queued", got)
	}
	// A pending work message exists.
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, jobID); n != 1 {
		t.Fatalf("pending work message count = %d, want 1", n)
	}
	if got := messageState(t, ctx, runner, repoID, msgID); got != "pending" {
		t.Fatalf("reused message state = %q, want pending", got)
	}
	// No dangling active lease.
	if n := activeLeaseCount(t, ctx, runner, repoID, jobID); n != 0 {
		t.Fatalf("active lease count = %d, want 0", n)
	}
	// Attempt UNCHANGED — the whole point of the operational (not content) requeue.
	if got := jobAttempt(t, ctx, runner, repoID, jobID); got != priorAttempt {
		t.Fatalf("attempt = %d, want %d (operational requeue must NOT bump attempt)", got, priorAttempt)
	}
	// Downstream job untouched.
	if got := jobState(t, ctx, runner, repoID, downstreamJobID); got != "blocked" {
		t.Fatalf("downstream job state = %q, want blocked (untouched)", got)
	}
	if got := jobAttempt(t, ctx, runner, repoID, downstreamJobID); got != 1 {
		t.Fatalf("downstream attempt = %d, want 1 (untouched)", got)
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
	// Attempt STILL unchanged after the fresh claim.
	if got := jobAttempt(t, ctx, runner, repoID, jobID); got != priorAttempt {
		t.Fatalf("attempt after fresh claim = %d, want %d", got, priorAttempt)
	}
}

// #121 / D036: the dead-lane repo-write requeue still refuses WITHOUT --force
// (the operator inspection gate is preserved).
func TestRequeueStaleDeadLaneRepoWriteRefusedWithoutForce(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_dead_lane_no_force"
	runID, jobID, _, _, _ := seedDeadLaneRepoWriteJob(t, ctx, runner, repoID)

	_, err := HandleRecoveryRequeueStale(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "job_id": jobID,
	}))
	if err == nil || !strings.Contains(err.Error(), "manual inspection") {
		t.Fatalf("expected D036 manual-inspection refusal without --force, got %v", err)
	}
	// The job must be untouched by the refused call.
	if got := jobState(t, ctx, runner, repoID, jobID); got != "running" {
		t.Fatalf("job state after refused requeue = %q, want running (untouched)", got)
	}
}

// #121: when an ACTIVE lease still exists (a LIVE claimant, not a dead lane),
// the verb keeps the #82 transfer guidance instead of silently requeueing.
func TestRequeueStaleLiveClaimantGuidesToTransfer(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_live_claimant_guide"
	runID, jobID, _, _ := seedRepoWriteClaimedJob(t, ctx, runner, repoID, "sess_live_"+repoID)

	_, err := HandleRecoveryRequeueStale(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "job_id": jobID,
	}))
	if err == nil || !strings.Contains(err.Error(), "live claimant") {
		t.Fatalf("expected live-claimant transfer guidance, got %v", err)
	}
}

// #121: requeueJobSameAttempt is idempotent — a second --force requeue of the
// now-queued+pending job is an already_reclaimable no-op (no attempt bump, no
// duplicate pending message).
func TestRequeueStaleDeadLaneIdempotent(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_dead_lane_idempotent"
	runID, jobID, _, _, _ := seedDeadLaneRepoWriteJob(t, ctx, runner, repoID)

	first, err := HandleRecoveryRequeueStale(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "job_id": jobID, "force": true, "justification": "first reclaim",
	}))
	if err != nil {
		t.Fatalf("first requeue: %v", err)
	}
	if first["status"] != "requeued" {
		t.Fatalf("first status = %v, want requeued", first["status"])
	}
	attemptAfterFirst := jobAttempt(t, ctx, runner, repoID, jobID)

	second, err := HandleRecoveryRequeueStale(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "job_id": jobID, "force": true, "justification": "second reclaim (idempotent)",
	}))
	if err != nil {
		t.Fatalf("second requeue: %v", err)
	}
	if second["status"] != "already_reclaimable" {
		t.Fatalf("second status = %v, want already_reclaimable; %#v", second["status"], second)
	}
	if got := jobAttempt(t, ctx, runner, repoID, jobID); got != attemptAfterFirst {
		t.Fatalf("attempt after idempotent second requeue = %d, want %d", got, attemptAfterFirst)
	}
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, jobID); n != 1 {
		t.Fatalf("pending work message count after idempotent requeue = %d, want exactly 1", n)
	}
}

// #121 ask #1 parenthetical: `session close --requeue-job` returns the closing
// session's in-flight job to the queue on the same attempt so a fresh lane can
// pick it up. The session's lease is already released (session.close refuses an
// active lease), so this is the dead-lane running-limbo case.
func TestSessionCloseRequeueJobReturnsInflightJob(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_close_requeue"
	runID := "run_" + repoID
	jobID := "job_impl_" + repoID
	leaseID := "lease_rel_" + repoID
	msgID := "msg_impl_" + repoID
	sessionID := "sess_owner_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "implement", "type": "build", "role_id": "implementer"}},
	})
	// The owning session is still ACTIVE (operator is about to close it).
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "implementer", "claude", []string{"write"}, "active")

	now := time.Now().UTC()
	wsArg, err := db.JSONBArg(runner, map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []any{"src/"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, current_message_id, created_at, started_at
		) VALUES ($1,$2,$3,'implement',1,'running','implementer','Implement','build',
		          'idem_impl_'||$1,'[]'::jsonb,$4::jsonb,'{"lane_id":"claude"}'::jsonb,$5,$6,$6)`,
		repoID, jobID, runID, wsArg, msgID, now); err != nil {
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
	// Lease already RELEASED (operator released before closing) — session.close
	// refuses an active lease.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'released',NOW() - INTERVAL '1 hour',NOW() + INTERVAL '1 hour',NOW(),'operator_release')`,
		repoID, leaseID, runID, jobID, sessionID); err != nil {
		t.Fatalf("insert released lease: %v", err)
	}

	priorAttempt := jobAttempt(t, ctx, runner, repoID, jobID)

	res, err := HandleCloseSession(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id":  sessionID,
		"reason":      "lane died mid-build; returning job for a fresh lane",
		"requeue_job": true,
	}))
	if err != nil {
		t.Fatalf("session close --requeue-job: %v", err)
	}
	if res["state"] != "closed" {
		t.Fatalf("session state = %v, want closed", res["state"])
	}
	requeued, ok := res["requeued_job"].(map[string]any)
	if !ok {
		t.Fatalf("expected requeued_job in result, got %#v", res["requeued_job"])
	}
	if fmt.Sprint(requeued["job_id"]) != jobID {
		t.Fatalf("requeued job_id = %v, want %s", requeued["job_id"], jobID)
	}
	if requeued["repo_write"] != true {
		t.Fatalf("expected repo_write=true on requeued job, got %#v", requeued)
	}

	if got := jobState(t, ctx, runner, repoID, jobID); got != "queued" {
		t.Fatalf("job state after close --requeue-job = %q, want queued", got)
	}
	if n := pendingWorkMessageCount(t, ctx, runner, repoID, jobID); n != 1 {
		t.Fatalf("pending work message count = %d, want 1", n)
	}
	if got := jobAttempt(t, ctx, runner, repoID, jobID); got != priorAttempt {
		t.Fatalf("attempt = %d, want %d (close --requeue-job must NOT bump attempt)", got, priorAttempt)
	}

	// A fresh implementer session claims it.
	freshSession := "sess_fresh_" + repoID
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, freshSession, "implementer", "claude", []string{"write"}, "active", 2)
	claim, err := HandleClaimNext(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": freshSession,
	}))
	if err != nil {
		t.Fatalf("fresh claim must succeed, got: %v", err)
	}
	if fmt.Sprint(claim["status"]) != "claimed" {
		t.Fatalf("fresh claim status = %v, want claimed", claim["status"])
	}
}

// Without --requeue-job, session close leaves the in-flight job alone.
func TestSessionCloseWithoutRequeueLeavesJobRunning(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_close_no_requeue"
	runID := "run_" + repoID
	jobID := "job_impl_" + repoID
	leaseID := "lease_rel_" + repoID
	msgID := "msg_impl_" + repoID
	sessionID := "sess_owner_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "implement", "type": "build", "role_id": "implementer"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "implementer", "claude", []string{"write"}, "active")

	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  current_message_id, created_at, started_at
		) VALUES ($1,$2,$3,'implement',1,'running','implementer','Implement','build',
		          'idem_impl_'||$1,'[]'::jsonb,'{}'::jsonb,$4,$5,$5)`,
		repoID, jobID, runID, msgID, now); err != nil {
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
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'released',NOW(),NOW() + INTERVAL '1 hour',NOW(),'operator_release')`,
		repoID, leaseID, runID, jobID, sessionID); err != nil {
		t.Fatalf("insert released lease: %v", err)
	}

	res, err := HandleCloseSession(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"reason":     "done",
	}))
	if err != nil {
		t.Fatalf("session close: %v", err)
	}
	if _, present := res["requeued_job"]; present {
		t.Fatalf("requeued_job must be absent without --requeue-job, got %#v", res["requeued_job"])
	}
	if got := jobState(t, ctx, runner, repoID, jobID); got != "running" {
		t.Fatalf("job state after plain close = %q, want running (untouched)", got)
	}
}
