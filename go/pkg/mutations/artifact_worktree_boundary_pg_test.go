package mutations

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// #310: a repo-write job under a per-job-isolated lane must NEVER write the
// operator's shared, tracked checkout directly — only the daemon porter advances
// the run branch. The claim packet resolves the lane via a session fallback when
// the job row's lane_selector_json carries no lane_id, so the lane supervises
// (as `striatum-lane`) believing it is per-job isolated. The publish-time gate
// historically read ONLY the job selector, so an empty selector made the per-job
// worktree NOT required and artifactSourcePath silently fell back to repoRoot.
//
// These tests pin the boundary: with the session fallback, an isolated repo-write
// job with NO active worktree now REFUSES publish (worktree_required) instead of
// writing the shared tree; the normal per-job-worktree path still succeeds; and a
// genuinely shared-checkout (worktree_isolation: off) repo-write workflow is NOT
// broken by the refusal.

// clearJobLaneSelector NULLs the job's lane_selector_json lane_id, reproducing
// the #310 condition where the job row carries no lane and the effective lane is
// resolvable only from the session that owns the active lease.
func clearJobLaneSelector(t *testing.T, ctx context.Context, runner db.Runner, repoID, jobID string) {
	t.Helper()
	emptyArg, err := db.JSONBArg(runner, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.jobs SET lane_selector_json = $3::jsonb
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID, emptyArg); err != nil {
		t.Fatalf("clear lane selector: %v", err)
	}
}

// setLaneSharedCheckout rewrites the workflow snapshot so the lane is an explicit
// shared-checkout repo-write lane (worktree_isolation: off,
// allow_shared_checkout_repo_write: true) — the one legitimate path that writes
// the shared checkout directly and must NOT be refused.
func setLaneSharedCheckout(t *testing.T, ctx context.Context, runner db.Runner, repoID, snapshotID, laneID string) {
	t.Helper()
	workflowArg, err := db.JSONBArg(runner, map[string]any{
		"workflow_id": "wf_shared",
		"lanes": map[string]any{
			laneID: map[string]any{
				"worktree_isolation":               "off",
				"allow_shared_checkout_repo_write": true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.workflow_snapshots SET workflow_json = $3::jsonb
		 WHERE repository_id = $1 AND workflow_snapshot_id = $2`, repoID, snapshotID, workflowArg); err != nil {
		t.Fatalf("set shared-checkout lane: %v", err)
	}
}

// snapshotIDForRun reads the run's workflow_snapshot_id (seedWorktreeRequiredJob
// names it "snap_<suffix>", but resolve it from the run row to stay robust).
func snapshotIDForRun(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID string) string {
	t.Helper()
	row, err := oneRow(ctx, runner, `
		SELECT workflow_snapshot_id FROM striatumd.runs
		 WHERE repository_id = $1 AND run_id = $2`, repoID, runID)
	if err != nil {
		t.Fatalf("read run snapshot id: %v", err)
	}
	return fmt.Sprint(row["workflow_snapshot_id"])
}

// (a) An isolated repo-write job whose lane is resolvable only via the session
// fallback, with NO active worktree, REFUSES publish with worktree_required —
// and writes NOTHING to the shared repoRoot.
func TestPublishRefusesIsolatedRepoWriteWithoutWorktreeViaSessionFallback(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	// withWorktree=false: the per-job worktree does not exist.
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "wt_boundary_refuse", false)
	// The job row carries no lane; only the session (lane_id='codex', which the
	// workflow marks worktree_isolation: per_job) identifies the lane.
	clearJobLaneSelector(t, ctx, runner, ids.repoID, ids.jobID)

	repoPath := "docs/out.txt"
	_, err := HandlePublishArtifact(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id":   ids.sessionID,
		"job_id":       ids.jobID,
		"lease_id":     ids.leaseID,
		"kind":         "handoff",
		"logical_name": "out",
		"path":         repoPath,
		"body_base64":  base64.StdEncoding.EncodeToString([]byte("must not reach the shared tree\n")),
	}))
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "worktree_required" {
		t.Fatalf("publish error = %v, want worktree_required (session fallback must make the worktree required)", err)
	}
	// The shared, tracked checkout must be untouched: no silent repoRoot write.
	if _, statErr := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(repoPath))); !os.IsNotExist(statErr) {
		t.Fatalf("publish wrote %s into the shared tracked checkout; the boundary must refuse instead (stat err=%v)", repoPath, statErr)
	}
}

// (b) The normal per-job-worktree path still succeeds even when the lane is
// resolvable only via the session fallback (job selector empty, worktree active).
func TestPublishSucceedsWithActiveWorktreeViaSessionFallback(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	// withWorktree=true: the active per-job worktree exists.
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "wt_boundary_ok", true)
	clearJobLaneSelector(t, ctx, runner, ids.repoID, ids.jobID)

	repoPath := "docs/out.txt"
	if _, err := HandlePublishArtifact(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id":   ids.sessionID,
		"job_id":       ids.jobID,
		"lease_id":     ids.leaseID,
		"kind":         "handoff",
		"logical_name": "out",
		"path":         repoPath,
		"body_base64":  base64.StdEncoding.EncodeToString([]byte("into the per-job worktree\n")),
	})); err != nil {
		t.Fatalf("publish with active worktree should succeed: %v", err)
	}
	// The body was materialized INSIDE the per-job worktree, not the shared tree.
	if _, err := os.Stat(filepath.Join(ids.worktreeRoot, filepath.FromSlash(repoPath))); err != nil {
		t.Fatalf("expected body inside the per-job worktree: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(repoPath))); !os.IsNotExist(statErr) {
		t.Fatalf("publish must not also write the shared checkout (stat err=%v)", statErr)
	}
}

// (c) A genuinely shared-checkout repo-write workflow (worktree_isolation: off,
// allow_shared_checkout_repo_write: true) is NOT broken by the #310 refusal: it
// still publishes to repoRoot. This is the explicit interactive-human
// compatibility path; the refusal must be scoped to per_job-isolated lanes only.
func TestPublishSharedCheckoutRepoWriteStillWritesRepoRoot(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	// No per-job worktree; this lane is intentionally shared-checkout.
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "wt_boundary_shared", false)
	clearJobLaneSelector(t, ctx, runner, ids.repoID, ids.jobID)
	snapshotID := snapshotIDForRun(t, ctx, runner, ids.repoID, ids.runID)
	setLaneSharedCheckout(t, ctx, runner, ids.repoID, snapshotID, "codex")

	repoPath := "docs/out.txt"
	if _, err := HandlePublishArtifact(ctx, runner, intgEnv(ids.repoID, map[string]any{
		"session_id":   ids.sessionID,
		"job_id":       ids.jobID,
		"lease_id":     ids.leaseID,
		"kind":         "handoff",
		"logical_name": "out",
		"path":         repoPath,
		"body_base64":  base64.StdEncoding.EncodeToString([]byte("shared-checkout legitimate write\n")),
	})); err != nil {
		t.Fatalf("shared-checkout repo-write publish should succeed (not refuse): %v", err)
	}
	// The shared-checkout path writes to repoRoot by design.
	if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(repoPath))); err != nil {
		t.Fatalf("shared-checkout publish should write repoRoot: %v", err)
	}
}
