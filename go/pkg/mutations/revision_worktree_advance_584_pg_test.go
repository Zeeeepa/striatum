package mutations

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// gitRunEnvStdin runs git in repoRoot with extra env and optional stdin, failing
// the test on a non-zero exit. It is the plumbing primitive the #584 fixture uses
// to advance a run branch without touching any checkout.
func gitRunEnvStdin(t *testing.T, repoRoot string, extraEnv []string, stdin []byte, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, repoRoot, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitFixOntoRunBranch advances the run branch ref to a new commit that adds
// relPath=body on top of its current tip, WITHOUT checking the branch out
// anywhere (the per-job worktree stays detached at the old attempt's HEAD and the
// primary checkout stays on main). It models the operator/revision fix landing on
// the run branch via the daemon's plumbing (hash-object / read-tree / write-tree /
// commit-tree / update-ref), exactly the shape run integration uses. It returns
// the new tip sha.
func commitFixOntoRunBranch(t *testing.T, repoRoot, runBranch, relPath, body string) string {
	t.Helper()
	ref := "refs/heads/" + runBranch
	parent := gitRevParse(t, repoRoot, ref)
	// Write the new blob into the object store.
	blob := gitRunEnvStdin(t, repoRoot, nil, []byte(body), "hash-object", "-w", "--stdin")
	// Build a tree that is the parent tree plus the new path, via a temporary
	// index file so the primary checkout's index is never touched.
	idx := filepath.Join(t.TempDir(), "fix-index")
	env := []string{"GIT_INDEX_FILE=" + idx}
	gitRunEnvStdin(t, repoRoot, env, nil, "read-tree", parent+"^{tree}")
	gitRunEnvStdin(t, repoRoot, env, nil, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+relPath)
	tree := gitRunEnvStdin(t, repoRoot, env, nil, "write-tree")
	commit := gitRunEnvStdin(t, repoRoot, nil, []byte("revision fix\n"), "commit-tree", tree, "-p", parent)
	gitRun(t, repoRoot, "update-ref", ref, commit, parent)
	return commit
}

// TestReopenAdvancesWorktreeToRunBranchTip is the GH #584 regression: when a job
// is re-opened for a fresh attempt (the revision-routing continue /
// reopenJobForAttempt path), its EXISTING per-job worktree must be advanced from
// the stale prior-attempt HEAD to the CURRENT run-branch tip, so the
// re-dispatched lane reviews the integrated/fixed code instead of re-emitting
// already-fixed blockers forever.
func TestReopenAdvancesWorktreeToRunBranchTip(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "reopen_advance_584", true)

	// Commit A is where the worktree (and the run branch) start.
	commitA := gitRevParse(t, repoRoot, "refs/heads/"+ids.runBranch)
	if head := gitRevParse(t, ids.worktreeRoot, "HEAD"); head != commitA {
		t.Fatalf("precondition: worktree HEAD = %s, want commit A %s", head, commitA)
	}
	fixRel := "docs/REVISION_FIX.md"
	fixAbs := filepath.Join(ids.worktreeRoot, filepath.FromSlash(fixRel))
	if _, err := os.Stat(fixAbs); !os.IsNotExist(err) {
		t.Fatalf("precondition: fix file %s must be ABSENT in the worktree before reopen (stat err=%v)", fixRel, err)
	}

	// The operator/revision fix lands on the run branch: commit B adds the fix
	// file. The worktree is still pinned at A and is blind to it (the #584 bug).
	commitB := commitFixOntoRunBranch(t, repoRoot, ids.runBranch, fixRel, "the fix the reviewer must now see\n")
	if commitB == commitA {
		t.Fatalf("commitFixOntoRunBranch did not advance the run branch")
	}
	if head := gitRevParse(t, ids.worktreeRoot, "HEAD"); head != commitA {
		t.Fatalf("worktree HEAD moved before reopen = %s, want still A %s", head, commitA)
	}

	// Drive the reopen path exactly as production does: inside the resolve/route
	// transaction, calling the shared helper.
	job, err := rowByID(ctx, runner, ids.repoID, "jobs", "job_id", ids.jobID, false)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if _, err := reopenJobForAttempt(ctx, tx, ids.repoID, job, "checkpoint_continue"); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	}); err != nil {
		t.Fatalf("reopenJobForAttempt: %v", err)
	}

	// The worktree HEAD has been advanced to the run-branch tip (commit B)...
	if head := gitRevParse(t, ids.worktreeRoot, "HEAD"); head != commitB {
		t.Fatalf("worktree HEAD after reopen = %s, want run-branch tip B %s (the #584 advance)", head, commitB)
	}
	// ...and the fix file the operator committed is now present in the worktree's
	// working tree, so the re-dispatched lane reviews the fixed code.
	got, err := os.ReadFile(fixAbs)
	if err != nil {
		t.Fatalf("fix file %s must be present in the worktree after reopen: %v", fixRel, err)
	}
	if strings.TrimSpace(string(got)) != "the fix the reviewer must now see" {
		t.Fatalf("fix file body = %q, want the fix content", string(got))
	}

	// The advance recorded a worktree.advanced event with the from/to commits for
	// provenance.
	ev, err := oneRow(ctx, runner, `
		SELECT payload_json->>'from_head' AS from_head, payload_json->>'to_tip' AS to_tip
		  FROM striatumd.events
		 WHERE repository_id = $1 AND event_type = 'worktree.advanced' AND job_id = $2
		 ORDER BY event_id DESC LIMIT 1`, ids.repoID, ids.jobID)
	if err != nil {
		t.Fatalf("expected a worktree.advanced event: %v", err)
	}
	if from := strings.TrimSpace(fmt.Sprint(ev["from_head"])); from != commitA {
		t.Fatalf("worktree.advanced from_head = %q, want A %s", from, commitA)
	}
	if to := strings.TrimSpace(fmt.Sprint(ev["to_tip"])); to != commitB {
		t.Fatalf("worktree.advanced to_tip = %q, want B %s", to, commitB)
	}
}

// TestReopenWorktreeAdvanceIsNoopWithoutWorktree guards the read-only-reviewer
// case: a re-opened job with NO active per-job worktree must reopen cleanly (the
// advance is a no-op), never failing the resolve transaction.
func TestReopenWorktreeAdvanceIsNoopWithoutWorktree(t *testing.T) {
	if !haveGit(t) {
		return
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	// withWorktree=false: no job_worktrees row, no on-disk worktree.
	ids := seedWorktreeRequiredJob(t, ctx, runner, repoRoot, "reopen_noop_584", false)

	job, err := rowByID(ctx, runner, ids.repoID, "jobs", "job_id", ids.jobID, false)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if _, err := reopenJobForAttempt(ctx, tx, ids.repoID, job, "checkpoint_continue"); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	}); err != nil {
		t.Fatalf("reopen with no worktree must succeed (advance is a no-op): %v", err)
	}

	// No worktree.advanced event should have been recorded.
	if _, err := oneRow(ctx, runner, `
		SELECT 1 FROM striatumd.events
		 WHERE repository_id = $1 AND event_type = 'worktree.advanced' AND job_id = $2
		 LIMIT 1`, ids.repoID, ids.jobID); err == nil {
		t.Fatalf("no worktree.advanced event should be recorded when the job has no worktree")
	}
}
