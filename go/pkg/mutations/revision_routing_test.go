package mutations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// revisionFixture seeds a repo + run with a synthesis job (`synth`) that is
// already completed and a design-review job (`review_design_codex`) that is
// running with an active lease, mirroring run_651e20f3... from issue #63 F1.
// The `cycles` argument is written verbatim into the workflow snapshot.
func revisionFixture(t *testing.T, ctx context.Context, runner db.Runner, repoID string, cycles []any) (runID, reviewJobID, synthJobID, sessionID, leaseID string) {
	t.Helper()
	runID = "run_" + repoID
	reviewJobID = "job_review_" + repoID
	synthJobID = "job_synth_" + repoID
	sessionID = "sess_reviewer_" + repoID
	leaseID = "lease_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"reviewer": map[string]any{}, "synthesizer": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{}, "claude": map[string]any{}},
		"jobs": []any{
			map[string]any{"id": "synth", "type": "synthesis", "role_id": "synthesizer"},
			map[string]any{"id": "review_design_codex", "type": "review", "role_id": "reviewer", "review_posture": "threat_model"},
		},
		"cycles": cycles,
	})

	now := time.Now().UTC()

	// Completed upstream synthesis job (the cycle target).
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, created_at, completed_at
		) VALUES ($1,$2,$3,'synth',1,'completed','synthesizer','Synthesis','synthesis','idem_synth_'||$1,'[]'::jsonb,$4,$4)`,
		repoID, synthJobID, runID, now); err != nil {
		t.Fatalf("insert synth job: %v", err)
	}

	// Running design-review job (the work message FK requires the job to
	// exist first, so insert the job, then the message, then back-fill the
	// job's current_message_id / current_lease_id pointers).
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, created_at, started_at
		) VALUES ($1,$2,$3,'review_design_codex',1,'running','reviewer','Design Review','review','idem_review_'||$1,'[]'::jsonb,$4,$4)`,
		repoID, reviewJobID, runID, now); err != nil {
		t.Fatalf("insert review job: %v", err)
	}

	msgID := "msg_review_" + repoID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','acked',0,'reviewer','codex',$5,$5)`,
		repoID, msgID, runID, reviewJobID, now); err != nil {
		t.Fatalf("insert review message: %v", err)
	}

	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "reviewer", "codex", []string{"review"}, "active")

	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',NOW(),NOW() + INTERVAL '1 hour')`,
		repoID, leaseID, runID, reviewJobID, sessionID); err != nil {
		t.Fatalf("insert lease: %v", err)
	}

	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET current_message_id = $3, current_lease_id = $4
		 WHERE repository_id = $1 AND job_id = $2`,
		repoID, reviewJobID, msgID, leaseID); err != nil {
		t.Fatalf("link review job pointers: %v", err)
	}
	return runID, reviewJobID, synthJobID, sessionID, leaseID
}

func jobState(t *testing.T, ctx context.Context, runner any, repoID, jobID string) string {
	t.Helper()
	row, err := oneRow(ctx, runner, `SELECT state FROM striatumd.jobs WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil {
		t.Fatalf("select job %s: %v", jobID, err)
	}
	return fmt.Sprint(row["state"])
}

func openRevisionBlockers(t *testing.T, ctx context.Context, runner any, repoID, jobID string) int {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT count(*) AS n FROM striatumd.blockers
		 WHERE repository_id = $1 AND job_id = $2 AND state = 'open'
		   AND blocker_kind = 'revision_routing'`, repoID, jobID)
	if err != nil {
		t.Fatalf("count blockers: %v", err)
	}
	return intValue(row["n"])
}

// F1 (a): a needs_revision verdict with a matching cycle routes to the target
// and re-opens it; no revision_routing human_checkpoint is created.
func TestNeedsRevisionRoutesToMatchingCycle(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_revision_route"
	cycles := []any{
		map[string]any{
			"from":            "review_design_codex",
			"to":              "synth",
			"on_verdict":      "needs_revision",
			"max_iterations":  float64(2),
			"allow_same_lane": true,
		},
	}
	runID, reviewJobID, synthJobID, sessionID, leaseID := revisionFixture(t, ctx, runner, repoID, cycles)

	result, err := HandleRecordVerdict(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     reviewJobID,
		"lease_id":   leaseID,
		"verdict":    "needs_revision",
	}))
	if err != nil {
		t.Fatalf("record verdict: %v", err)
	}
	if fmt.Sprint(result["status"]) != "revision_routed" {
		t.Fatalf("status = %v, want revision_routed; result=%#v", result["status"], result)
	}
	if fmt.Sprint(result["cycle_target_job_id"]) != synthJobID {
		t.Fatalf("cycle_target_job_id = %v, want %s", result["cycle_target_job_id"], synthJobID)
	}
	if intValue(result["cycle_iteration"]) != 1 {
		t.Fatalf("cycle_iteration = %v, want 1", result["cycle_iteration"])
	}

	if got := jobState(t, ctx, runner, repoID, reviewJobID); got != "completed" {
		t.Fatalf("review job state = %q, want completed", got)
	}
	if got := jobState(t, ctx, runner, repoID, synthJobID); got != "queued" {
		t.Fatalf("synth job state = %q, want queued (re-opened for revision)", got)
	}
	if n := openRevisionBlockers(t, ctx, runner, repoID, reviewJobID); n != 0 {
		t.Fatalf("expected no revision_routing blocker, got %d", n)
	}

	// A fresh pending work message must exist for the re-opened synth job.
	msgRow, err := oneRow(ctx, runner, `
		SELECT count(*) AS n FROM striatumd.queue_messages
		 WHERE repository_id = $1 AND job_id = $2 AND kind = 'work' AND state = 'pending'`,
		repoID, synthJobID)
	if err != nil {
		t.Fatalf("count synth messages: %v", err)
	}
	if intValue(msgRow["n"]) != 1 {
		t.Fatalf("expected 1 pending work message for synth, got %v", msgRow["n"])
	}

	// The cycle-routed event must be recorded for max_iterations accounting.
	evtRow, err := oneRow(ctx, runner, `
		SELECT count(*) AS n FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'revision.cycle_routed'`,
		repoID, runID)
	if err != nil {
		t.Fatalf("count routed events: %v", err)
	}
	if intValue(evtRow["n"]) != 1 {
		t.Fatalf("expected 1 revision.cycle_routed event, got %v", evtRow["n"])
	}
}

// F1 (b): a needs_revision verdict with NO matching cycle still produces the
// revision_routing human_checkpoint (preserve current behavior).
func TestNeedsRevisionWithoutCycleOpensCheckpoint(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_revision_no_cycle"
	// No cycles declared.
	_, reviewJobID, synthJobID, sessionID, leaseID := revisionFixture(t, ctx, runner, repoID, []any{})

	result, err := HandleRecordVerdict(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     reviewJobID,
		"lease_id":   leaseID,
		"verdict":    "needs_revision",
	}))
	if err != nil {
		t.Fatalf("record verdict: %v", err)
	}
	if fmt.Sprint(result["status"]) != "waiting_human" {
		t.Fatalf("status = %v, want waiting_human; result=%#v", result["status"], result)
	}
	if fmt.Sprint(result["blocker_id"]) == "" || result["blocker_id"] == nil {
		t.Fatalf("expected a blocker_id, got %#v", result)
	}
	if got := jobState(t, ctx, runner, repoID, reviewJobID); got != "waiting_human" {
		t.Fatalf("review job state = %q, want waiting_human", got)
	}
	// Upstream synth must remain untouched (still completed).
	if got := jobState(t, ctx, runner, repoID, synthJobID); got != "completed" {
		t.Fatalf("synth job state = %q, want completed (untouched)", got)
	}
	if n := openRevisionBlockers(t, ctx, runner, repoID, reviewJobID); n != 1 {
		t.Fatalf("expected 1 revision_routing blocker, got %d", n)
	}
}

// F1 (c): when the cycle's max_iterations budget is exhausted, the verdict
// falls back to a human checkpoint rather than re-routing forever.
func TestNeedsRevisionExhaustedCycleOpensCheckpoint(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_revision_exhausted"
	cycles := []any{
		map[string]any{
			"from":            "review_design_codex",
			"to":              "synth",
			"on_verdict":      "needs_revision",
			"max_iterations":  float64(1),
			"allow_same_lane": true,
		},
	}
	runID, reviewJobID, synthJobID, sessionID, leaseID := revisionFixture(t, ctx, runner, repoID, cycles)

	// Pre-record one prior routing so the budget (1) is already spent.
	now := time.Now().UTC()
	if _, err := appendEvent(ctx, runner, repoID, runID, "revision.cycle_routed", nil, reviewJobID, nil, nil, nil, map[string]any{
		"from_workflow_job_id": "review_design_codex",
		"to_workflow_job_id":   "synth",
		"iteration":            1,
	}); err != nil {
		t.Fatalf("seed prior routing event: %v", err)
	}
	_ = now

	result, err := HandleRecordVerdict(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     reviewJobID,
		"lease_id":   leaseID,
		"verdict":    "needs_revision",
	}))
	if err != nil {
		t.Fatalf("record verdict: %v", err)
	}
	if fmt.Sprint(result["status"]) != "waiting_human" {
		t.Fatalf("status = %v, want waiting_human (budget exhausted); result=%#v", result["status"], result)
	}
	if got := jobState(t, ctx, runner, repoID, synthJobID); got != "completed" {
		t.Fatalf("synth job state = %q, want completed (not re-opened when exhausted)", got)
	}
	if n := openRevisionBlockers(t, ctx, runner, repoID, reviewJobID); n != 1 {
		t.Fatalf("expected 1 revision_routing blocker, got %d", n)
	}
}

// F3 (a): a completed cycle-target job can be re-opened for revision through
// the operator run.retry_job path.
func TestRetryJobReopensCompletedCycleTarget(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_retry_cycle_target"
	cycles := []any{
		map[string]any{
			"from":            "review_design_codex",
			"to":              "synth",
			"on_verdict":      "needs_revision",
			"max_iterations":  float64(2),
			"allow_same_lane": true,
		},
	}
	runID, _, synthJobID, _, _ := revisionFixture(t, ctx, runner, repoID, cycles)

	result, err := HandleRunRetryJob(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID,
		"job_id": synthJobID,
	}))
	if err != nil {
		t.Fatalf("retry completed cycle target: %v", err)
	}
	if fmt.Sprint(result["previous_state"]) != "completed" {
		t.Fatalf("previous_state = %v, want completed", result["previous_state"])
	}
	if fmt.Sprint(result["new_state"]) != "queued" {
		t.Fatalf("new_state = %v, want queued", result["new_state"])
	}
	if got := jobState(t, ctx, runner, repoID, synthJobID); got != "queued" {
		t.Fatalf("synth job state = %q, want queued", got)
	}
}

// F3 (b): a completed job that is NOT a cycle target remains non-retriable.
func TestRetryJobRejectsCompletedNonCycleTarget(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_retry_non_cycle"
	// review_design_codex -> synth, but we'll try to retry the review (a
	// completed non-target) instead.
	cycles := []any{
		map[string]any{
			"from":            "review_design_codex",
			"to":              "synth",
			"on_verdict":      "needs_revision",
			"max_iterations":  float64(2),
			"allow_same_lane": true,
		},
	}
	runID, reviewJobID, _, _, _ := revisionFixture(t, ctx, runner, repoID, cycles)

	// Force the review job to completed so it is a completed non-cycle-target.
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET state = 'completed', current_lease_id = NULL
		 WHERE repository_id = $1 AND job_id = $2`, repoID, reviewJobID); err != nil {
		t.Fatalf("force review completed: %v", err)
	}

	_, err := HandleRunRetryJob(ctx, runner, intgEnv(repoID, map[string]any{
		"run_id": runID,
		"job_id": reviewJobID,
	}))
	if err == nil {
		t.Fatal("expected retry of completed non-cycle-target to be rejected")
	}
}
