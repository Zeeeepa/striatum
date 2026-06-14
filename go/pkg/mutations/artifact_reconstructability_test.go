package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// RFC 0125 P0-3 (GH #285): the completion gate must re-verify the BODY of every
// required artifact is reconstructable from its declared placement, not merely
// that the row + size_bytes survive. These tests cover the #275 regression
// fence (a present row whose body is gone fails the run closed), the pass path
// (a durable git-anchored body completes), and the degrade ladder (an
// unverifiable substrate WARNS, never false-blocks a run that integrates after
// terminal state).

type reconFixture struct {
	repoID     string
	runID      string
	jobID      string
	sessionID  string
	runBranch  string
	repoRoot   string
	repoPath   string
	contentSHA string
	baseSHA    string
	body       []byte
}

// seedReconReviewRun seeds a running run whose single review job is completed
// with an accepting + attested verdict and one REQUIRED expected artifact (the
// row is published so verifyRequiredArtifacts passes — only the BODY is under
// test). The repository points at a real git repo so the git-anchor probe runs.
func seedReconReviewRun(t *testing.T, ctx context.Context, runner db.Runner, suffix, kind, placement string, blobKey any) reconFixture {
	t.Helper()
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runBranch := "wf/" + suffix
	gitRun(t, repoRoot, "branch", runBranch, baseSHA)

	repoPath := "docs/decisions/DECISION_" + suffix + ".md"
	body := []byte("durable decision body for " + suffix)
	sum := sha256.Sum256(body)
	contentSHA := hex.EncodeToString(sum[:])

	repoID := "repo_recon_" + suffix
	runID := "run_recon_" + suffix
	jobID := "job_recon_" + suffix
	sessionID := "sess_recon_" + suffix
	now := time.Now().UTC()

	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,$2,$3,$4,'repo',$5,16,'active')`,
		repoID, "ident_"+suffix, repoRoot, filepath.Join(repoRoot, ".striatum"), now); err != nil {
		t.Fatalf("insert repository: %v", err)
	}

	workflowArg, err := db.JSONBArg(runner, map[string]any{
		"workflow_id": "recon_wf",
		"lanes":       map[string]any{"reviewer": map[string]any{"display_model": "Claude"}},
		"jobs":        []any{map[string]any{"id": "review", "type": "review", "require_attested_lane": true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,$2,'recon_wf','sha',$3::jsonb,$4)`,
		repoID, "snap_"+suffix, workflowArg, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state,
		  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at
		) VALUES ($1,$2,$3,$4,'running',$5,$6,$7,'human',$7)`,
		repoID, runID, "snap_"+suffix, repoRoot, runBranch, baseSHA, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal, state, registered_at
		) VALUES ($1,$2,$3,'reviewer','reviewer','reviewer-1',1,'active',$4)`,
		repoID, sessionID, runID, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	expectedArg, err := db.JSONBArg(runner, []any{map[string]any{
		"logical_name": "decision",
		"kind":         kind,
		"path":         repoPath,
		"placement":    placement,
		"required":     true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, expected_artifacts_json, idempotency_key, created_at,
		  started_at, completed_at, fresh_session_required
		) VALUES ($1,$2,$3,'review',1,'completed','reviewer','Review','review',
		  $4::jsonb,'idem_'||$2,$5,$5,$5,false)`,
		repoID, jobID, runID, expectedArg, now); err != nil {
		t.Fatalf("insert completed review job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.verdicts (
		  repository_id, verdict_id, run_id, job_id, session_id, verdict,
		  rationale, created_at, posture, lane_attestation_at_record,
		  review_provenance_override
		) VALUES ($1,'verdict_'||$2,$3,$2,$4,'accept','seeded',$5,'neutral','attested',false)`,
		repoID, jobID, runID, sessionID, now); err != nil {
		t.Fatalf("insert verdict: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, job_id, session_id, logical_name,
		  artifact_kind, repo_path, content_sha256, size_bytes, publish_mode,
		  created_at, author_line, blob_key, attempt
		) VALUES ($1,$2,$3,$4,$5,'decision',$6,$7,$8,$9,'create',$10,NULL,$11,1)`,
		repoID, "art_recon_"+suffix, runID, jobID, sessionID, kind, repoPath,
		contentSHA, len(body), now, blobKey); err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	return reconFixture{
		repoID: repoID, runID: runID, jobID: jobID, sessionID: sessionID,
		runBranch: runBranch, repoRoot: repoRoot, repoPath: repoPath,
		contentSHA: contentSHA, baseSHA: baseSHA, body: body,
	}
}

// anchorBodyOnRunBranch commits body at repoPath onto runBranch (via a transient
// worktree) and returns the resulting commit, simulating a durably published
// git body that the porter/anchoring left on the run branch.
func anchorBodyOnRunBranch(t *testing.T, repoRoot, runBranch, repoPath string, body []byte) string {
	t.Helper()
	wt := t.TempDir()
	gitRun(t, repoRoot, "worktree", "add", wt, runBranch)
	defer gitRun(t, repoRoot, "worktree", "remove", "--force", wt)
	abs := filepath.Join(wt, filepath.FromSlash(repoPath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, body, 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wt, "add", repoPath)
	gitRun(t, wt, "commit", "-q", "-m", "anchored body")
	return gitRun(t, wt, "rev-parse", "HEAD")
}

// Test obligation 1 (#275 regression fence): a present required-artifact row
// whose body is gone — the refs/striatum anchor exists but does not contain the
// body — must route maybeCompleteRun to needs_operator/provenance_gate_failed,
// NOT a clean completion, with the failing gate keyed
// required_artifact_unreconstructable.
func TestRunCompletionFailsClosedOnUnreconstructableRequiredArtifact(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	fx := seedReconReviewRun(t, ctx, runner, "unreconstructable", "decision", "git_publication", nil)

	// The job WAS anchored (a refs/striatum pin exists) but the pin points at the
	// base commit, which does not contain the decision body — the body is gone.
	gitRun(t, fx.repoRoot, "update-ref", attemptPinRef(fx.runID, fx.jobID, 1), fx.baseSHA)

	driveMaybeCompleteRun(t, ctx, runner, fx.repoID, fx.runID)

	state, stopReason := runStateAndStopReason(t, ctx, runner, fx.repoID, fx.runID)
	if state != "needs_operator" || stopReason != "provenance_gate_failed" {
		t.Fatalf("run state/stop_reason = %s/%s, want needs_operator/provenance_gate_failed", state, stopReason)
	}
	blocker, err := oneRow(ctx, runner, `
		SELECT payload_json FROM striatumd.blockers
		 WHERE repository_id = $1 AND run_id = $2 AND blocker_kind = 'provenance_gate_failed'`,
		fx.repoID, fx.runID)
	if err != nil {
		t.Fatalf("read provenance gate blocker: %v", err)
	}
	payload := asMap(blocker["payload_json"])
	gates := asList(payload["failing_gates"])
	if len(gates) != 1 {
		t.Fatalf("failing_gates = %#v, want exactly the seeded gate", payload["failing_gates"])
	}
	gate := asMap(gates[0])
	if fmt.Sprint(gate["failure"]) != "required_artifact_unreconstructable" {
		t.Fatalf("gate failure = %v, want required_artifact_unreconstructable", gate["failure"])
	}
	unrec := asList(gate["unreconstructable"])
	if len(unrec) != 1 || fmt.Sprint(asMap(unrec[0])["reason"]) != "missing_from_git_anchor" {
		t.Fatalf("unreconstructable detail = %#v, want one missing_from_git_anchor", gate["unreconstructable"])
	}
	if intValue(payload["unreconstructable_count"]) != 1 {
		t.Fatalf("unreconstructable_count = %v, want 1", payload["unreconstructable_count"])
	}
}

// The pass path: a durable git body (committed to the run branch and anchored by
// a refs/striatum pin) re-reads + hash-matches, so the run completes cleanly and
// the gate ledger records the artifact as readback_verified.
func TestRunCompletionPassesWithReconstructableGitBody(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	fx := seedReconReviewRun(t, ctx, runner, "reconstructable", "decision", "git_publication", nil)

	commit := anchorBodyOnRunBranch(t, fx.repoRoot, fx.runBranch, fx.repoPath, fx.body)
	gitRun(t, fx.repoRoot, "update-ref", attemptPinRef(fx.runID, fx.jobID, 1), commit)

	driveMaybeCompleteRun(t, ctx, runner, fx.repoID, fx.runID)

	state, _ := runStateAndStopReason(t, ctx, runner, fx.repoID, fx.runID)
	if state != "completed" {
		t.Fatalf("run state = %s, want completed (durable git body is reconstructable)", state)
	}
	event, err := oneRow(ctx, runner, `
		SELECT payload_json FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'run.completed'
		 ORDER BY event_id DESC LIMIT 1`, fx.repoID, fx.runID)
	if err != nil {
		t.Fatalf("read run.completed event: %v", err)
	}
	ledger := asList(asMap(event["payload_json"])["provenance_gate"])
	if len(ledger) != 1 {
		t.Fatalf("provenance_gate ledger = %#v, want 1 entry", ledger)
	}
	arts := asList(asMap(ledger[0])["artifacts"])
	if len(arts) != 1 {
		t.Fatalf("gate artifacts[] = %#v, want 1 entry", arts)
	}
	art := asMap(arts[0])
	if fmt.Sprint(art["status"]) != "verified" || art["readback_verified"] != true {
		t.Fatalf("artifact recon = %#v, want verified + readback_verified", art)
	}
	if fmt.Sprint(art["git_anchor_ref"]) != attemptPinRef(fx.runID, fx.jobID, 1) {
		t.Fatalf("git_anchor_ref = %v, want the attempt pin", art["git_anchor_ref"])
	}
}

// Degrade ladder (no false-block): a git_publication required artifact with NO
// anchor pin yet and no blob mirror is a legitimately-pending body — it must
// WARN, never fail the gate (a run that git-integrates after terminal state
// must not be blocked).
func TestReconstructabilityWarnsWhenGitAnchorPending(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	fx := seedReconReviewRun(t, ctx, runner, "anchorpending", "decision", "git_publication", nil)

	job, err := rowByID(ctx, runner, fx.repoID, "jobs", "job_id", fx.jobID, false)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	results, err := verifyRequiredArtifactReconstructable(ctx, runner, fx.repoID, job)
	if err != nil {
		t.Fatalf("verifyRequiredArtifactReconstructable: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %#v, want 1", results)
	}
	if results[0].Status != reconstructionWarning || results[0].Reason != "git_anchor_pending_unverifiable" {
		t.Fatalf("result = %+v, want warning/git_anchor_pending_unverifiable", results[0])
	}
	if len(failedReconstructions(results)) != 0 {
		t.Fatalf("a pending anchor must not be a hard failure: %#v", results)
	}
}

// Degrade ladder (no false-block): a blob_exhaust required artifact with a
// recorded blob key cannot be verified when no blob client is configured (the
// common pgtest / blob-less environment) — it must WARN, never fail the gate.
func TestReconstructabilityWarnsWhenBlobClientUnavailable(t *testing.T) {
	if !haveGit(t) {
		return
	}
	if packageBlobClient != nil {
		t.Skip("a blob client is configured; this guards the blob-less degrade path")
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	fx := seedReconReviewRun(t, ctx, runner, "blobless", "finding", "blob_exhaust", "artifacts/run/job/finding")

	job, err := rowByID(ctx, runner, fx.repoID, "jobs", "job_id", fx.jobID, false)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	results, err := verifyRequiredArtifactReconstructable(ctx, runner, fx.repoID, job)
	if err != nil {
		t.Fatalf("verifyRequiredArtifactReconstructable: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %#v, want 1", results)
	}
	if results[0].Status != reconstructionWarning || results[0].Reason != "blob_client_unavailable" {
		t.Fatalf("result = %+v, want warning/blob_client_unavailable", results[0])
	}
	if len(failedReconstructions(results)) != 0 {
		t.Fatalf("an unconfigured blob client must not be a hard failure: %#v", results)
	}
}
