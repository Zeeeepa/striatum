package mutations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// applyQuarantineOwnerBundle widens the jobs state CHECK to permit 'quarantined'
// (owner bundle 0012). The mutations pgtests run against a runtime-migrated DB
// that does NOT apply owner bundles, so the #311 quarantine path needs the
// owner-table DDL applied first. pgtest.Pool returns the privileged
// (owner=current user) runner, which ApplyOwnerBundles requires.
func applyQuarantineOwnerBundle(t *testing.T, ctx context.Context, runner db.Runner) {
	t.Helper()
	if _, _, err := db.ApplyOwnerBundles(ctx, runner, "test"); err != nil {
		t.Fatalf("apply owner bundles (0012 quarantine state): %v", err)
	}
}

// quarantineSeedOptions configures the offending exhausted job + its siblings.
type quarantineSeedOptions struct {
	// offendingJobType / offendingWorkflowJobID describe the exhausted job. A
	// 'build' leaf is quarantine-eligible; a 'review' fresh job is a
	// provenance-required reviewer (guard b).
	offendingJobType        string
	offendingWorkflowJobID  string
	offendingFreshSession   bool
	offendingRequireAttest  bool
	withBlockedDownstream   bool // adds a blocked downstream job depending on the offending job (guard a)
	withSecondExhaustedLeaf bool // adds a second independent dead-lane leaf job past budget (cap test)
	secondWorkflowJobID     string
}

// seedQuarantineFixture seeds a run with:
//   - a COMPLETED sibling deliverable job ('implement', build, state=completed),
//   - the OFFENDING dead-lane job in running-limbo (running, lease released,
//     owning session closed) with its recovery budget pre-exhausted.
//
// It mirrors seedDeadLaneRepoWriteJob's dead-lane shape so the recovery sweep
// reaches the budget-exhaustion escalation path. Returns the run id and the
// offending job id (and the optional second offending leaf id).
func seedQuarantineFixture(t *testing.T, ctx context.Context, runner db.Runner, repoID string, opts quarantineSeedOptions) (runID, offendingJobID, secondJobID string) {
	t.Helper()
	if opts.offendingJobType == "" {
		opts.offendingJobType = "build"
	}
	if opts.offendingWorkflowJobID == "" {
		opts.offendingWorkflowJobID = "flaky"
	}
	runID = "run_" + repoID
	completedJobID := "job_done_" + repoID
	offendingJobID = "job_flaky_" + repoID
	deadSession := "sess_dead_" + repoID
	now := time.Now().UTC()

	// Build the workflow def for the offending job carrying any provenance flags.
	offendingDef := map[string]any{
		"id":      opts.offendingWorkflowJobID,
		"type":    opts.offendingJobType,
		"role_id": "flaky_role",
	}
	if opts.offendingRequireAttest {
		offendingDef["require_attested_lane"] = true
	}
	if opts.offendingFreshSession {
		offendingDef["reviewer_context_policy"] = "fresh"
	}
	jobs := []any{
		map[string]any{"id": "done", "type": "build", "role_id": "doer"},
		offendingDef,
	}
	if opts.withBlockedDownstream {
		jobs = append(jobs, map[string]any{"id": "after", "type": "review", "role_id": "reviewer"})
	}
	if opts.withSecondExhaustedLeaf {
		jobs = append(jobs, map[string]any{"id": opts.secondWorkflowJobID, "type": "build", "role_id": "flaky_role"})
	}

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"doer": map[string]any{}, "flaky_role": map[string]any{}, "reviewer": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{}},
		"jobs":        jobs,
	})

	// Completed sibling deliverable.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, created_at, completed_at
		) VALUES ($1,$2,$3,'done',1,'completed','doer','Done','build',
		          'idem_done_'||$1,'[]'::jsonb,'{}'::jsonb,'{}'::jsonb,$4,$4)`,
		repoID, completedJobID, runID, now); err != nil {
		t.Fatalf("insert completed sibling job: %v", err)
	}

	// The offending dead-lane job in running-limbo.
	seedDeadLaneJobRow(t, ctx, runner, repoID, runID, offendingJobID, opts.offendingWorkflowJobID,
		opts.offendingJobType, "flaky_role", deadSession, 1, opts.offendingFreshSession, now)
	preseedExhaustedBudget(t, ctx, runner, repoID, runID, offendingJobID)

	if opts.withBlockedDownstream {
		downstreamID := "job_after_" + repoID
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.jobs (
			  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
			  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
			  created_at
			) VALUES ($1,$2,$3,'after',1,'blocked','reviewer','After','review',
			          'idem_after_'||$1,'[]'::jsonb,'{}'::jsonb,$4)`,
			repoID, downstreamID, runID, now); err != nil {
			t.Fatalf("insert blocked downstream job: %v", err)
		}
		if err := runner.Exec(ctx, `
			INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id)
			VALUES ($1,$2,$3)`, repoID, downstreamID, offendingJobID); err != nil {
			t.Fatalf("insert downstream dependency: %v", err)
		}
	}

	if opts.withSecondExhaustedLeaf {
		secondJobID = "job_flaky2_" + repoID
		secondSession := "sess_dead2_" + repoID
		seedDeadLaneJobRow(t, ctx, runner, repoID, runID, secondJobID, opts.secondWorkflowJobID,
			"build", "flaky_role", secondSession, 2, false, now)
		preseedExhaustedBudget(t, ctx, runner, repoID, runID, secondJobID)
	}
	return runID, offendingJobID, secondJobID
}

// seedDeadLaneJobRow seeds a single dead-lane job in running-limbo: running,
// lease released (not expired), owning session closed, zero artifacts. This is
// the budget-exhaustion escalation precondition (mirrors seedDeadLaneRepoWriteJob
// without the implicit downstream).
func seedDeadLaneJobRow(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, workflowJobID, jobType, roleID, sessionID string, ordinal int, freshSession bool, now time.Time) {
	t.Helper()
	intgSeedSessionOrdinal(t, ctx, runner, repoID, runID, sessionID, roleID, "claude", []string{"write"}, "closed", ordinal)
	leaseID := "lease_" + jobID
	msgID := "msg_" + jobID
	fresh := "false"
	if freshSession {
		fresh = "true"
	}
	if err := runner.Exec(ctx, fmt.Sprintf(`
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, fresh_session_required, current_lease_id, created_at, started_at
		) VALUES ($1,$2,$3,$4,1,'running',$5,'Flaky',$6,
		          'idem_'||$2,'[]'::jsonb,'{}'::jsonb,'{"lane_id":"claude"}'::jsonb,%s,NULL,$7,$7)`, fresh),
		repoID, jobID, runID, workflowJobID, roleID, jobType, now); err != nil {
		t.Fatalf("insert dead-lane job %s: %v", jobID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','acked',0,$5,'claude',$6,$6)`,
		repoID, msgID, runID, jobID, roleID, now); err != nil {
		t.Fatalf("insert work message for %s: %v", jobID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'released',NOW() - INTERVAL '2 hours',NOW() - INTERVAL '1 hour',NOW() - INTERVAL '30 minutes','session_closed')`,
		repoID, leaseID, runID, jobID, sessionID); err != nil {
		t.Fatalf("insert released lease for %s: %v", jobID, err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET current_message_id = $3
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID, msgID); err != nil {
		t.Fatalf("link job message pointer for %s: %v", jobID, err)
	}
}

// jobQuarantineManifestFromCompletedEvent reads the quarantine_manifest array
// from the run's terminal run.completed (or run.canceled) event payload.
func quarantineManifestFromTerminalEvent(t *testing.T, ctx context.Context, runner any, repoID, runID string) []any {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT payload_json FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2
		   AND event_type IN ('run.completed','run.canceled')
		 ORDER BY event_id DESC LIMIT 1`, repoID, runID)
	if err != nil {
		t.Fatalf("read terminal event: %v", err)
	}
	manifest, _ := asMap(row["payload_json"])["quarantine_manifest"].([]any)
	return manifest
}

// TestSweepQuarantinesLeafJobAndFinalizesMajority is the #311 P0 core: a leaf job
// that exhausts its recovery budget after its sibling completed and with nothing
// depending on it is moved to 'quarantined' (NOT completed, NO artifact sealed),
// the run reaches 'completed' carrying a quarantine manifest naming the job, and
// a recovery.job_quarantined event exists. The whole run is NOT flipped to
// needs_operator.
func TestSweepQuarantinesLeafJobAndFinalizesMajority(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	applyQuarantineOwnerBundle(t, ctx, runner)
	repoID := "repo_quarantine_leaf"
	runID, offendingJobID, _ := seedQuarantineFixture(t, ctx, runner, repoID, quarantineSeedOptions{})

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// The offending job is quarantined — NOT completed, NOT canceled.
	if got := jobState(t, ctx, runner, repoID, offendingJobID); got != "quarantined" {
		t.Fatalf("offending job state = %q, want quarantined (no false completion)", got)
	}
	// No artifact was sealed on its behalf.
	sealed, err := existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.artifacts WHERE repository_id = $1 AND job_id = $2 LIMIT 1`, repoID, offendingJobID)
	if err != nil {
		t.Fatalf("check artifacts: %v", err)
	}
	if sealed {
		t.Fatalf("an artifact was sealed for the quarantined job; must never seal on its behalf")
	}

	// The run reached 'completed' (finalize-the-majority on the completed sibling).
	if got := runStateOf(t, ctx, runner, repoID, runID); got != "completed" {
		t.Fatalf("run state = %q, want completed (finalize-the-majority)", got)
	}
	// stop_reason names the quarantine.
	srRow, err := oneRow(ctx, runner, `SELECT stop_reason FROM striatumd.runs WHERE repository_id = $1 AND run_id = $2`, repoID, runID)
	if err != nil {
		t.Fatalf("read stop_reason: %v", err)
	}
	if got := fmt.Sprint(nullable(srRow["stop_reason"])); got != "quarantined_jobs" {
		t.Fatalf("stop_reason = %q, want quarantined_jobs", got)
	}

	// A recovery.job_quarantined event exists, naming the offending job + lane.
	qev, err := oneRow(ctx, runner, `
		SELECT payload_json FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'recovery.job_quarantined'
		 ORDER BY event_id DESC LIMIT 1`, repoID, runID)
	if err != nil {
		t.Fatalf("read recovery.job_quarantined event: %v", err)
	}
	qpayload := asMap(qev["payload_json"])
	if got := fmt.Sprint(qpayload["job_id"]); got != offendingJobID {
		t.Fatalf("quarantined event job_id = %q, want %s", got, offendingJobID)
	}
	if got := fmt.Sprint(qpayload["lane"]); got != "claude" {
		t.Fatalf("quarantined event lane = %q, want claude", got)
	}

	// The terminal run.completed event carries the quarantine manifest naming the job.
	manifest := quarantineManifestFromTerminalEvent(t, ctx, runner, repoID, runID)
	if len(manifest) != 1 {
		t.Fatalf("quarantine_manifest = %#v, want exactly 1 entry", manifest)
	}
	entry := asMap(manifest[0])
	if got := fmt.Sprint(entry["job_id"]); got != offendingJobID {
		t.Fatalf("manifest job_id = %q, want %s", got, offendingJobID)
	}
	if got := fmt.Sprint(entry["workflow_job_id"]); got != "flaky" {
		t.Fatalf("manifest workflow_job_id = %q, want flaky", got)
	}
	if got := fmt.Sprint(entry["lane"]); got != "claude" {
		t.Fatalf("manifest lane = %q, want claude", got)
	}

	// The run was NOT flipped to needs_operator at any point: no run.needs_operator event.
	if hasNO, err := existsRow(ctx, runner, `
		SELECT 1 FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'run.needs_operator' LIMIT 1`, repoID, runID); err != nil {
		t.Fatalf("check needs_operator event: %v", err)
	} else if hasNO {
		t.Fatalf("run was flipped to needs_operator; #311 should finalize-the-majority instead")
	}
}

// TestSweepDoesNotQuarantineJobWithUnfinishedDownstream: a job whose blocked
// downstream dependent is still unfinished is a hard dependency (guard a), so it
// must NOT be quarantined — the whole run flips to needs_operator as before.
func TestSweepDoesNotQuarantineJobWithUnfinishedDownstream(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	applyQuarantineOwnerBundle(t, ctx, runner)
	repoID := "repo_quarantine_downstream"
	runID, offendingJobID, _ := seedQuarantineFixture(t, ctx, runner, repoID, quarantineSeedOptions{
		withBlockedDownstream: true,
	})

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if got := jobState(t, ctx, runner, repoID, offendingJobID); got == "quarantined" {
		t.Fatalf("job with unfinished downstream dependent must NOT be quarantined")
	}
	if got := runStateOf(t, ctx, runner, repoID, runID); got != "needs_operator" {
		t.Fatalf("run state = %q, want needs_operator (hard downstream dependency)", got)
	}
	// The needs_operator escalation names the offending job.
	_, payload := recoveryExhaustedEscalation(t, ctx, runner, repoID, runID)
	if got := fmt.Sprint(payload["job_id"]); got != offendingJobID {
		t.Fatalf("escalation job_id = %q, want %s", got, offendingJobID)
	}
}

// TestSweepDoesNotQuarantineProvenanceRequiredReviewer: a fresh-session review
// job is a provenance-required reviewer the RFC 0118 completion gate would refuse
// (guard b), so it must NOT be quarantine-finalized — the run flips to
// needs_operator.
func TestSweepDoesNotQuarantineProvenanceRequiredReviewer(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	applyQuarantineOwnerBundle(t, ctx, runner)
	repoID := "repo_quarantine_reviewer"
	runID, offendingJobID, _ := seedQuarantineFixture(t, ctx, runner, repoID, quarantineSeedOptions{
		offendingJobType:       "review",
		offendingWorkflowJobID: "gate",
		offendingFreshSession:  true,
	})

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if got := jobState(t, ctx, runner, repoID, offendingJobID); got == "quarantined" {
		t.Fatalf("provenance-required reviewer must NOT be quarantined (guard b)")
	}
	if got := runStateOf(t, ctx, runner, repoID, runID); got != "needs_operator" {
		t.Fatalf("run state = %q, want needs_operator (provenance gate would refuse)", got)
	}
}

// TestSweepQuarantineCapEscalatesSecondCandidate: with the default
// max_quarantinable_jobs=1, a run with TWO simultaneous quarantine candidates
// quarantines (at most) one and escalates the rest to needs_operator rather than
// silently swallowing large-scale failure.
func TestSweepQuarantineCapEscalatesSecondCandidate(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	applyQuarantineOwnerBundle(t, ctx, runner)
	repoID := "repo_quarantine_cap"
	runID, firstJobID, secondJobID := seedQuarantineFixture(t, ctx, runner, repoID, quarantineSeedOptions{
		withSecondExhaustedLeaf: true,
		secondWorkflowJobID:     "flaky2",
	})

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	// Exactly one job quarantined; the other escalated.
	first := jobState(t, ctx, runner, repoID, firstJobID)
	second := jobState(t, ctx, runner, repoID, secondJobID)
	quarantinedCount := 0
	for _, s := range []string{first, second} {
		if s == "quarantined" {
			quarantinedCount++
		}
	}
	if quarantinedCount != 1 {
		t.Fatalf("quarantined count = %d (first=%q second=%q), want exactly 1 (cap=1)", quarantinedCount, first, second)
	}
	// The run is needs_operator (the over-cap candidate keeps it frozen).
	if got := runStateOf(t, ctx, runner, repoID, runID); got != "needs_operator" {
		t.Fatalf("run state = %q, want needs_operator (quarantine cap exceeded)", got)
	}
}

// TestAcceptQuarantinedResolvesBlockerAndCancels: recovery.accept_quarantined on
// a quarantined job resolves its recovery_exhausted blocker + escalation and
// marks the job canceled (terminal), and is idempotent.
func TestAcceptQuarantinedResolvesBlockerAndCancels(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	applyQuarantineOwnerBundle(t, ctx, runner)
	repoID := "repo_accept_quarantined"
	runID, offendingJobID, _ := seedQuarantineFixture(t, ctx, runner, repoID, quarantineSeedOptions{})

	if _, err := SweepRun(ctx, runner, repoID, runID, ""); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := jobState(t, ctx, runner, repoID, offendingJobID); got != "quarantined" {
		t.Fatalf("pre-accept job state = %q, want quarantined", got)
	}

	res, err := HandleRecoveryAcceptQuarantined(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "job_id": offendingJobID,
	}))
	if err != nil {
		t.Fatalf("accept-quarantined: %v", err)
	}
	if fmt.Sprint(res["status"]) != "accepted" {
		t.Fatalf("status = %v, want accepted; %#v", res["status"], res)
	}

	// Job is now canceled (terminal).
	if got := jobState(t, ctx, runner, repoID, offendingJobID); got != "canceled" {
		t.Fatalf("post-accept job state = %q, want canceled", got)
	}
	// The recovery_exhausted blocker + escalation are resolved.
	row, err := oneRow(ctx, runner, `
		SELECT b.state AS b_state, ei.state AS ei_state
		  FROM striatumd.blockers b
		  LEFT JOIN striatumd.escalation_inbox ei
		    ON ei.repository_id = b.repository_id AND ei.blocker_id = b.blocker_id
		 WHERE b.repository_id = $1 AND b.job_id = $2 AND b.blocker_kind = 'recovery_exhausted'`,
		repoID, offendingJobID)
	if err != nil {
		t.Fatalf("read blocker/escalation: %v", err)
	}
	if got := fmt.Sprint(row["b_state"]); got != "resolved" {
		t.Fatalf("blocker state = %q, want resolved", got)
	}
	if got := fmt.Sprint(row["ei_state"]); got != "resolved" {
		t.Fatalf("escalation state = %q, want resolved", got)
	}

	// Idempotent: a second accept returns already_accepted.
	res2, err := HandleRecoveryAcceptQuarantined(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID, "job_id": offendingJobID,
	}))
	if err != nil {
		t.Fatalf("idempotent re-accept: %v", err)
	}
	if fmt.Sprint(res2["status"]) != "already_accepted" {
		t.Fatalf("re-accept status = %v, want already_accepted; %#v", res2["status"], res2)
	}
	if got := jobState(t, ctx, runner, repoID, offendingJobID); got != "canceled" {
		t.Fatalf("job state after re-accept = %q, want canceled (unchanged)", got)
	}
}
