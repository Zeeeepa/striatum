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

// RFC 0118 P1-6 (GH #240): recovery.invalidate_job reopens a single
// compromised completed review job while the run stays running. The
// invalidation receipt is DURABLE: the compromised verdict row is superseded
// (stamped with the authorizing decision), never deleted — its frozen P0-1
// provenance stamp survives for the run_completion_record and any
// retrospective. The fresh attempt must itself pass the attested admission
// gate to re-complete.

func seedInvalidateJobFixture(t *testing.T, ctx context.Context, runner db.Runner, repoID string) (runID, jobID, sessionID, decisionID string) {
	t.Helper()
	runID = "run_" + repoID
	jobID = "job_review_" + repoID
	sessionID = "sess_reviewer_" + repoID
	decisionID = "dec_invalidate_" + repoID
	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "invalidate_wf",
		"lanes":       map[string]any{"reviewer": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "review", "type": "review"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "reviewer", "reviewer", []string{"review"}, "active")
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, expected_artifacts_json, idempotency_key, created_at,
		  started_at, completed_at
		) VALUES ($1,$2,$3,'review',1,'completed','reviewer','Review','review','[]'::jsonb,
		  'idem_'||$2,$4,$4,$4)`,
		repoID, jobID, runID, now); err != nil {
		t.Fatalf("insert completed review job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.verdicts (
		  repository_id, verdict_id, run_id, job_id, session_id, verdict,
		  rationale, created_at, posture,
		  lane_attestation_at_record, review_provenance_override
		) VALUES ($1,'verdict_'||$2,$3,$2,$4,'accept','compromised accept',$5,'neutral',
		  'attested',false)`,
		repoID, jobID, runID, sessionID, now); err != nil {
		t.Fatalf("insert compromised verdict: %v", err)
	}
	// The authorizing operator decision, scoped to the exact
	// (session_id, job_id) — the same scoped-decision shape work.claim_override
	// requires (#222).
	artifactID := "art_" + decisionID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, job_id, session_id,
		  logical_name, artifact_kind, repo_path, content_sha256,
		  size_bytes, publish_mode, created_at
		) VALUES ($1,$2,$3,NULL,NULL,$4,'decision',$5,'sha_'||$2,1,'create',$6)`,
		repoID, artifactID, runID, decisionID, "decisions/"+decisionID+".md", now); err != nil {
		t.Fatalf("insert decision artifact: %v", err)
	}
	if _, err := appendEvent(ctx, runner, repoID, runID, "decision.recorded", nil, nil, nil, artifactID, nil, map[string]any{
		"decision_id":        decisionID,
		"outcome":            "accepted",
		"subject_session_id": sessionID,
		"subject_job_id":     jobID,
		"escape_surface":     "review_provenance",
	}); err != nil {
		t.Fatalf("append decision event: %v", err)
	}
	return runID, jobID, sessionID, decisionID
}

func TestRecoveryInvalidateJobSupersedesVerdictAndReopens(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_invalidate_ok"
	runID, jobID, sessionID, decisionID := seedInvalidateJobFixture(t, ctx, runner, repoID)

	result, err := HandleRecoveryInvalidateJob(ctx, runner, intgEnv(repoID, map[string]any{
		"job_id":      jobID,
		"decision_id": decisionID,
	}))
	if err != nil {
		t.Fatalf("recovery.invalidate_job: %v", err)
	}
	if fmt.Sprint(result["status"]) != "invalidated" {
		t.Fatalf("status = %v, want invalidated", result["status"])
	}

	// Durable receipt: the verdict row SURVIVES, superseded by the decision,
	// with its frozen provenance stamp intact.
	row, err := oneRow(ctx, runner, `
		SELECT verdict, superseded_by_decision_id, superseded_at,
		       lane_attestation_at_record, session_id
		  FROM striatumd.verdicts
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil {
		t.Fatalf("read superseded verdict row: %v", err)
	}
	if fmt.Sprint(row["superseded_by_decision_id"]) != decisionID || row["superseded_at"] == nil {
		t.Fatalf("verdict row = %#v, want superseded by %s (durable receipt, not deleted)", row, decisionID)
	}
	if fmt.Sprint(row["lane_attestation_at_record"]) != "attested" {
		t.Fatalf("frozen stamp = %v, want preserved attested stamp", row["lane_attestation_at_record"])
	}
	if fmt.Sprint(row["session_id"]) != sessionID {
		t.Fatalf("superseded row session = %v, want %s", row["session_id"], sessionID)
	}

	// The job is reopened at attempt 2 with a fresh work message; the run
	// stays running.
	job, err := oneRow(ctx, runner, `
		SELECT state, attempt FROM striatumd.jobs
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil {
		t.Fatalf("read reopened job: %v", err)
	}
	if intValue(job["attempt"]) != 2 {
		t.Fatalf("job attempt = %v, want 2 after reopen", job["attempt"])
	}
	if state := fmt.Sprint(job["state"]); state != "queued" && state != "blocked" {
		t.Fatalf("job state = %q, want queued/blocked (reopened)", state)
	}
	if state, _ := runStateAndStopReason(t, ctx, runner, repoID, runID); state != "running" {
		t.Fatalf("run state = %s, want running (invalidate keeps the run live)", state)
	}

	// The receipt event names the decision and the superseded verdict.
	event, err := oneRow(ctx, runner, `
		SELECT payload_json FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'recovery.job_invalidated'
		 ORDER BY event_id DESC LIMIT 1`, repoID, runID)
	if err != nil {
		t.Fatalf("read recovery.job_invalidated event: %v", err)
	}
	payload := asMap(event["payload_json"])
	if fmt.Sprint(payload["decision_id"]) != decisionID || fmt.Sprint(payload["invalidated_verdict_id"]) != "verdict_"+jobID {
		t.Fatalf("event payload = %#v, want decision + invalidated verdict id", payload)
	}
}

func TestRecoveryInvalidateJobRefusesBroadDecision(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_invalidate_broad"
	runID, jobID, _, _ := seedInvalidateJobFixture(t, ctx, runner, repoID)
	broadID := "dec_broad_" + repoID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, job_id, session_id,
		  logical_name, artifact_kind, repo_path, content_sha256,
		  size_bytes, publish_mode, created_at
		) VALUES ($1,'art_'||$2,$3,NULL,NULL,$2,'decision',$4,'sha_'||$2,1,'create',now())`,
		repoID, broadID, runID, "decisions/"+broadID+".md"); err != nil {
		t.Fatalf("insert broad decision artifact: %v", err)
	}
	if _, err := appendEvent(ctx, runner, repoID, runID, "decision.recorded", nil, nil, nil, "art_"+broadID, nil, map[string]any{
		"decision_id": broadID,
		"outcome":     "accepted",
	}); err != nil {
		t.Fatalf("append broad decision event: %v", err)
	}

	_, err := HandleRecoveryInvalidateJob(ctx, runner, intgEnv(repoID, map[string]any{
		"job_id":      jobID,
		"decision_id": broadID,
	}))
	if err == nil || !strings.Contains(err.Error(), "broad decision") {
		t.Fatalf("broad decision should be refused; got %v", err)
	}
	// Nothing mutated: verdict still active, job still completed at attempt 1.
	row, err := oneRow(ctx, runner, `
		SELECT superseded_by_decision_id FROM striatumd.verdicts
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
	if err != nil {
		t.Fatalf("read verdict: %v", err)
	}
	if row["superseded_by_decision_id"] != nil {
		t.Fatalf("verdict superseded by %v, want untouched after refusal", row["superseded_by_decision_id"])
	}
}

func TestRecoveryInvalidateJobRequiresRunningRunAndCompletedJob(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_invalidate_guards"
	runID, jobID, _, decisionID := seedInvalidateJobFixture(t, ctx, runner, repoID)

	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs SET state = 'completed'
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID); err != nil {
		t.Fatalf("set run terminal: %v", err)
	}
	_, err := HandleRecoveryInvalidateJob(ctx, runner, intgEnv(repoID, map[string]any{
		"job_id":      jobID,
		"decision_id": decisionID,
	}))
	if err == nil || !strings.Contains(err.Error(), "running run") {
		t.Fatalf("terminal run should be refused; got %v", err)
	}
}
