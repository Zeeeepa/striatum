package mutations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// reopenReviewToAttempt2 takes a completed-round-1 review fixture and re-opens
// its job to attempt 2 (the shape a revision cycle leaves behind): the job is
// running again on a fresh lease and the attempt is bumped. It returns the new
// lease id. The attempt-1 finding artifact row is left in place (artifacts are
// append-only), which is exactly what the #206 guard keys on.
func reopenReviewToAttempt2(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, sessionID, jobID string) (leaseID string) {
	t.Helper()
	now := time.Now().UTC()
	leaseID = "lease_a2_" + jobID
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'running', attempt = 2, started_at = $4,
		       current_lease_id = $5, completed_at = NULL
		 WHERE repository_id = $1 AND run_id = $2 AND job_id = $3`,
		repoID, runID, jobID, now, leaseID); err != nil {
		t.Fatalf("reopen job to attempt 2: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',$6,$7)`,
		repoID, leaseID, runID, jobID, sessionID, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert attempt-2 lease: %v", err)
	}
	// This fixture re-claims the SAME session at attempt 2, so it clears the prior
	// verdict to free the (job, session) verdict uniqueness constraint. (Production
	// no longer DELETEs verdicts on re-open — RFC 0126 P0 / D194 made verdict
	// history append-only; a real re-review is claimed by a DIFFERENT session per
	// the fresh-review lineage gate, which sidesteps the constraint without a
	// clear. This is fixture convenience, not the production reset path.) The
	// durable verdict.recorded EVENT survives regardless and is what
	// revisionContextForPacket reads, so prior_verdict stays recoverable.
	if err := runner.Exec(ctx, `
		DELETE FROM striatumd.verdicts
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID); err != nil {
		t.Fatalf("delete prior verdicts on reopen: %v", err)
	}
	return leaseID
}

// TestSubmitReviewRefusesByteIdenticalPriorAttemptFinding guards #206
// (RFC 0095 Goal 8): a re-claimed review job (attempt 2) that republishes the
// prior round's finding verbatim must be refused with fresh_review_byte_identical
// — it re-asserts a stale verdict against an already-revised target and poisons
// the bounded revision cycle. A finding with CHANGED content succeeds.
func TestSubmitReviewRefusesByteIdenticalPriorAttemptFinding(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	payload := findingArtifactPayload("needs_revision")
	repoID, sessionID, jobID, leaseID := seedReviewFindingFixture(t, ctx, runner, payload)
	runID := "run_" + repoID

	// Round 1: publish + submit the finding F (verdict needs_revision). This
	// records the attempt-1 finding artifact the guard later compares against.
	if _, err := HandleSubmitReview(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   leaseID,
		"path":       "artifacts/review/FINDING.md",
		"verdict":    "needs_revision",
		"rationale":  "round 1 found issues",
	})); err != nil {
		t.Fatalf("round-1 submit-review: %v", err)
	}

	// Re-open the review job for a fresh round (attempt 2). The prior round's
	// FINDING.md is still on disk — the exact #206 trap.
	lease2 := reopenReviewToAttempt2(t, ctx, runner, repoID, runID, sessionID, jobID)

	// Round 2 republishing the byte-identical finding must be refused.
	_, err := HandleSubmitReview(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   lease2,
		"path":       "artifacts/review/FINDING.md",
		"verdict":    "needs_revision",
		"rationale":  "adopting the prior round's finding (the bug)",
	}))
	if err == nil {
		t.Fatal("expected fresh_review_byte_identical refusal for a byte-identical prior-attempt finding, got nil (#206 regression)")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error is not an rpc.Error: %T %v", err, err)
	}
	if rpcErr.Code != "fresh_review_byte_identical" {
		t.Fatalf("error code = %q, want fresh_review_byte_identical", rpcErr.Code)
	}

	// The same job, same attempt, with CHANGED content succeeds (a genuine fresh
	// review of the revised target).
	changed := findingArtifactPayload("accept") + "\nThe revision addressed the issues.\n"
	if err := os.WriteFile(filepath.Join(repoRootForRun(t, ctx, runner, repoID, runID), "artifacts", "review", "FINDING.md"), []byte(changed), 0o644); err != nil {
		t.Fatalf("write changed finding: %v", err)
	}
	res, err := HandleSubmitReview(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   lease2,
		"path":       "artifacts/review/FINDING.md",
		"verdict":    "accept",
		"rationale":  "fresh round-2 review of the revised target",
	}))
	if err != nil {
		t.Fatalf("changed-content round-2 submit-review must succeed, got: %v", err)
	}
	if res["job_state"] != "completed" {
		t.Fatalf("job_state = %#v, want completed", res["job_state"])
	}
}

// TestRevisionContextForPacketCarriesPriorFindingAndVerdict guards #206
// (RFC 0095 Goal 8) packet-legibility half: a verdict-capable job re-opened to
// attempt 2 carries the prior finding artifact id + prior verdict so the lane
// knows it must review the CURRENT revision.
func TestRevisionContextForPacketCarriesPriorFindingAndVerdict(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	payload := findingArtifactPayload("needs_revision")
	repoID, sessionID, jobID, leaseID := seedReviewFindingFixture(t, ctx, runner, payload)
	runID := "run_" + repoID

	// Round 1 records the finding artifact + verdict.recorded event.
	if _, err := HandleSubmitReview(ctx, runner, intgEnv(repoID, map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   leaseID,
		"path":       "artifacts/review/FINDING.md",
		"verdict":    "needs_revision",
		"rationale":  "round 1 found issues",
	})); err != nil {
		t.Fatalf("round-1 submit-review: %v", err)
	}
	reopenReviewToAttempt2(t, ctx, runner, repoID, runID, sessionID, jobID)

	job, err := rowByID(ctx, runner, repoID, "jobs", "job_id", jobID, false)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	revision, err := revisionContextForPacket(ctx, runner, repoID, job)
	if err != nil {
		t.Fatalf("revisionContextForPacket: %v", err)
	}
	if revision == nil {
		t.Fatal("revision_context must be present for a re-opened attempt-2 review job (#206 regression)")
	}
	if fmt.Sprint(revision["reopened_attempt"]) != "2" {
		t.Fatalf("reopened_attempt = %v, want 2", revision["reopened_attempt"])
	}
	if revision["prior_finding_artifact_id"] == nil || fmt.Sprint(revision["prior_finding_artifact_id"]) == "" {
		t.Fatalf("prior_finding_artifact_id missing: %#v", revision)
	}
	if fmt.Sprint(revision["prior_finding_attempt"]) != "1" {
		t.Fatalf("prior_finding_attempt = %v, want 1", revision["prior_finding_attempt"])
	}
	if fmt.Sprint(revision["prior_verdict"]) != "needs_revision" {
		t.Fatalf("prior_verdict = %v, want needs_revision", revision["prior_verdict"])
	}

	// A fresh attempt-1 job has no prior round -> no revision context.
	job1 := map[string]any{"job_type": "review", "attempt": 1, "job_id": jobID, "run_id": runID}
	rev1, err := revisionContextForPacket(ctx, runner, repoID, job1)
	if err != nil {
		t.Fatalf("revisionContextForPacket(attempt 1): %v", err)
	}
	if rev1 != nil {
		t.Fatalf("attempt-1 job must carry no revision_context, got %#v", rev1)
	}
}

func repoRootForRun(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID string) string {
	t.Helper()
	row, err := rowByID(ctx, runner, repoID, "runs", "run_id", runID, false)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	return fmt.Sprint(row["repo_root"])
}
