package mutations

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	readspkg "github.com/halbritt/striatum/go/pkg/reads"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// seedTerminalDebrisArtifact drives the seeded repo-write job into the #303
// terminal-debris shape: the run is failed (terminal), the job is completed, and
// a required handoff artifact was published whose repo_path is absent from every
// durable ref and the default branch. The doctor artifact-anchor pass therefore
// classifies it as artifact_debris_terminal_run, and ArtifactDebrisVerdicts marks
// it eligible for prune.
func seedTerminalDebrisArtifact(t *testing.T, ctx context.Context, runner db.Runner, ids worktreeRequiredFixtureIDs, artifactID, repoPath string) {
	t.Helper()
	now := time.Now().UTC()
	// Run -> failed (terminal debris), job -> completed.
	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs SET state = 'failed' WHERE repository_id = $1 AND run_id = $2`,
		ids.repoID, ids.runID); err != nil {
		t.Fatalf("fail run: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET state = 'completed', completed_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND job_id = $3`, now, ids.repoID, ids.jobID); err != nil {
		t.Fatalf("complete job: %v", err)
	}
	// Published artifact whose path is GONE from every ref (no blob_key -> git
	// publication placement, content nowhere on disk/refs).
	seedPublishedArtifact(t, ctx, runner, ids, artifactID, "out", repoPath, []byte("lost debris body\n"), nil)
}

func pruneDebrisEventCount(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, artifactID string) int {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT COUNT(*) AS c
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2
		   AND event_type = $3 AND artifact_id = $4`,
		repoID, runID, readspkg.DebrisPrunedEventType, artifactID)
	if err != nil {
		t.Fatalf("count prune events: %v", err)
	}
	switch v := row["c"].(type) {
	case int64:
		return int(v)
	case int32:
		return int(v)
	case int:
		return v
	default:
		t.Fatalf("count type = %T", row["c"])
		return 0
	}
}

// TestRecoveryPruneDebrisPrunesTerminalDeadAnchor is pgtest #1: a failed run's
// lost artifact is pruned (pruned_count=1), a tombstone event is written, and the
// doctor debris classification is suppressed (ArtifactDebrisVerdicts still flags
// the lost artifact eligible, but the tombstone the doctor pass reads is now
// present so the debris code is cleared from the report).
func TestRecoveryPruneDebrisPrunesTerminalDeadAnchor(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "prune_dead", false)
	seedTerminalDebrisArtifact(t, ctx, runner, ids, "art_prune_dead", "docs/gone.md")

	// Precondition: the doctor classifier flags this artifact as eligible debris.
	verdicts, err := readspkg.ArtifactDebrisVerdicts(ctx, runner, ids.repoID, ids.runID)
	if err != nil {
		t.Fatalf("verdicts: %v", err)
	}
	if len(verdicts) != 1 || !verdicts[0].Eligible {
		t.Fatalf("verdicts = %#v, want one eligible", verdicts)
	}

	result, err := HandleRecoveryPruneDebris(ctx, runner, intgEnv(ids.repoID, map[string]any{"run_id": ids.runID}))
	if err != nil {
		t.Fatalf("prune-debris: %v", err)
	}
	if result["status"] != "pruned" {
		t.Fatalf("status = %#v, want pruned", result["status"])
	}
	if result["pruned_count"] != 1 {
		t.Fatalf("pruned_count = %#v, want 1", result["pruned_count"])
	}
	// Tombstone event written, keyed on the artifact_id (the doctor suppression key).
	if got := pruneDebrisEventCount(t, ctx, runner, ids.repoID, ids.runID, "art_prune_dead"); got != 1 {
		t.Fatalf("tombstone events = %d, want 1", got)
	}
	// The doctor suppression set now contains the artifact, so the debris code is
	// cleared from the report (the exact mechanism doctorArtifactAnchorIntegrity reads).
	pruned, err := debrisPrunedArtifactIDsForRun(ctx, runner, ids.repoID, ids.runID)
	if err != nil {
		t.Fatalf("suppression set: %v", err)
	}
	if !pruned["art_prune_dead"] {
		t.Fatalf("suppression set = %#v, want art_prune_dead tombstoned", pruned)
	}
}

// TestRecoveryPruneDebrisRefusesNonTerminalRun is pgtest #2: an active (running)
// run is refused at the run-state gate; nothing is tombstoned.
func TestRecoveryPruneDebrisRefusesNonTerminalRun(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "prune_active", false)
	// Job completed but the run stays running (non-terminal): not prunable.
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET state = 'completed', completed_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND job_id = $3`, time.Now().UTC(), ids.repoID, ids.jobID); err != nil {
		t.Fatalf("complete job: %v", err)
	}
	seedPublishedArtifact(t, ctx, runner, ids, "art_active", "out", "docs/gone.md", []byte("body\n"), nil)

	_, err := HandleRecoveryPruneDebris(ctx, runner, intgEnv(ids.repoID, map[string]any{"run_id": ids.runID}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("err = %v, want rpc error", err)
	}
	if rpcErr.Code != "invalid_transition" {
		t.Fatalf("code = %q, want invalid_transition", rpcErr.Code)
	}
	if got := pruneDebrisEventCount(t, ctx, runner, ids.repoID, ids.runID, "art_active"); got != 0 {
		t.Fatalf("tombstone events = %d, want 0 (refused)", got)
	}
}

// TestRecoveryPruneDebrisRetainsPreservedArtifact is pgtest #3: a terminal run
// whose artifact content is still present at its repo_path on the default branch
// is retained (anchored_on_durable_ref), never tombstoned.
func TestRecoveryPruneDebrisRetainsPreservedArtifact(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "prune_keep", false)

	// Commit the deliverable to the default branch (main) so its content is durably
	// preserved at its repo_path.
	body := "preserved deliverable\n"
	repoPath := "docs/keep.md"
	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs SET state = 'failed' WHERE repository_id = $1 AND run_id = $2`,
		ids.repoID, ids.runID); err != nil {
		t.Fatalf("fail run: %v", err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET state = 'completed', completed_at = $1, current_lease_id = NULL
		 WHERE repository_id = $2 AND job_id = $3`, time.Now().UTC(), ids.repoID, ids.jobID); err != nil {
		t.Fatalf("complete job: %v", err)
	}
	commitWorktreeFile(t, repoRoot, repoPath, body)
	// content_sha256 must match the committed body so the preservation guard fires.
	seedPublishedArtifact(t, ctx, runner, ids, "art_keep", "out", repoPath, []byte(body), nil)

	result, err := HandleRecoveryPruneDebris(ctx, runner, intgEnv(ids.repoID, map[string]any{"run_id": ids.runID}))
	if err != nil {
		t.Fatalf("prune-debris: %v", err)
	}
	if result["pruned_count"] != 0 {
		t.Fatalf("pruned_count = %#v, want 0 (preserved)", result["pruned_count"])
	}
	retained, _ := result["retained"].([]map[string]any)
	if len(retained) != 1 {
		t.Fatalf("retained = %#v, want one", result["retained"])
	}
	reason := retained[0]["reason"]
	if reason != "anchored_on_durable_ref" && reason != "file_present_on_default_branch" {
		t.Fatalf("retained reason = %#v, want anchored/present", reason)
	}
	if got := pruneDebrisEventCount(t, ctx, runner, ids.repoID, ids.runID, "art_keep"); got != 0 {
		t.Fatalf("tombstone events = %d, want 0 (retained)", got)
	}
}

// TestRecoveryPruneDebrisIsIdempotent is pgtest #4: a second prune of the same
// run tombstones nothing new and writes no duplicate event.
func TestRecoveryPruneDebrisIsIdempotent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "prune_idem", false)
	seedTerminalDebrisArtifact(t, ctx, runner, ids, "art_idem", "docs/gone.md")

	first, err := HandleRecoveryPruneDebris(ctx, runner, intgEnv(ids.repoID, map[string]any{"run_id": ids.runID}))
	if err != nil {
		t.Fatalf("first prune: %v", err)
	}
	if first["pruned_count"] != 1 {
		t.Fatalf("first pruned_count = %#v, want 1", first["pruned_count"])
	}

	second, err := HandleRecoveryPruneDebris(ctx, runner, intgEnv(ids.repoID, map[string]any{"run_id": ids.runID}))
	if err != nil {
		t.Fatalf("second prune: %v", err)
	}
	if second["pruned_count"] != 0 {
		t.Fatalf("second pruned_count = %#v, want 0", second["pruned_count"])
	}
	if second["already_pruned_count"] != 1 {
		t.Fatalf("second already_pruned_count = %#v, want 1", second["already_pruned_count"])
	}
	// No duplicate tombstone: still exactly one event.
	if got := pruneDebrisEventCount(t, ctx, runner, ids.repoID, ids.runID, "art_idem"); got != 1 {
		t.Fatalf("tombstone events = %d, want 1 (no duplicate)", got)
	}
}

// TestRecoveryPruneDebrisDryRunWritesNothing is pgtest #5: dry_run previews the
// partition with status would_prune and writes no events.
func TestRecoveryPruneDebrisDryRunWritesNothing(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "prune_dry", false)
	seedTerminalDebrisArtifact(t, ctx, runner, ids, "art_dry", "docs/gone.md")

	result, err := HandleRecoveryPruneDebris(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"run_id":  ids.runID,
		"dry_run": true,
	}))
	if err != nil {
		t.Fatalf("dry-run prune: %v", err)
	}
	if result["status"] != "would_prune" {
		t.Fatalf("status = %#v, want would_prune", result["status"])
	}
	if result["pruned_count"] != 1 {
		t.Fatalf("pruned_count = %#v, want 1 preview", result["pruned_count"])
	}
	// Nothing was written.
	if got := pruneDebrisEventCount(t, ctx, runner, ids.repoID, ids.runID, "art_dry"); got != 0 {
		t.Fatalf("tombstone events = %d, want 0 (dry-run)", got)
	}
}
