package mutations

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestRunIntegratePreservesInterveningMainWork is the #299 regression guard
// (base-drift). A run branch forks from a point-in-time main, then main advances
// with DISJOINT work before the run integrates. The merge-based HandleRunIntegrate
// performs a 3-way merge against the real merge-base — NOT a fast-forward and NOT
// a naive diff-apply — so main's intervening work is an addition on main's side
// relative to the merge-base and is PRESERVED, never reverted, while the run
// branch's own change still lands. (#299 was reported as base-drift but is
// already fixed by the merge-based integrate that landed 2026-06-04; this test
// nails the invariant so a future reversion to a fast-forward integrate is caught.)
func TestRunIntegratePreservesInterveningMainWork(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	now := time.Now().UTC()
	runID := "run_integrate_299"
	runBranch := "striatum/run-299"

	// The run branch forks from main at baseSHA and makes a DISJOINT change.
	gitRun(t, repoRoot, "branch", runBranch, baseSHA)
	wt := filepath.Join(repoRoot, ".wt299")
	gitRun(t, repoRoot, "worktree", "add", "--detach", wt, runBranch)
	if err := os.MkdirAll(filepath.Join(wt, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "src", "run.go"), []byte("run branch change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wt, "add", "src/run.go")
	gitRun(t, wt, "commit", "-q", "-m", "run branch change")
	gitRun(t, repoRoot, "update-ref", "refs/heads/"+runBranch, gitRevParse(t, wt, "HEAD"))

	// main advances AFTER the fork with disjoint intervening work (gitInit leaves
	// the primary checkout on main).
	if err := os.WriteFile(filepath.Join(repoRoot, "landed.md"), []byte("landed on main after the fork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoRoot, "add", "landed.md")
	gitRun(t, repoRoot, "commit", "-q", "-m", "intervening main work")

	// Seed the repository + a completed, branch-confirmed run.
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ('repo_299','ident_299',$1,$2,'repo',$3,16,'active')`,
		repoRoot, filepath.Join(repoRoot, ".striatum"), now); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	wfArg, err := db.JSONBArg(runner, map[string]any{"workflow_id": "wf_299"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ('repo_299','snap_299','wf_299','sha',$1::jsonb,$2)`, wfArg, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state,
		  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at, completed_at
		) VALUES ('repo_299',$1,'snap_299',$2,'completed',$3,$4,$5,'human',$5,$5)`,
		runID, repoRoot, runBranch, baseSHA, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}

	res, err := HandleRunIntegrate(ctx, runner, intgEnv("repo_299", map[string]any{"run_id": runID, "into": "main"}))
	if err != nil {
		t.Fatalf("integrate: %v", err)
	}
	mergeCommit := fmt.Sprint(res["merge_commit"])
	if !isFullGitSHA(mergeCommit) {
		t.Fatalf("integrate result missing merge_commit: %#v", res)
	}
	// A true merge commit (two parents), not a fast-forward.
	parents := strings.Fields(gitRun(t, repoRoot, "rev-list", "--parents", "-n", "1", mergeCommit))
	if len(parents) != 3 {
		t.Fatalf("merge commit should have 2 parents (3-way merge, not FF), got %v", parents)
	}
	// BOTH the intervening main work AND the run branch change are present at main's tip.
	tree := gitRun(t, repoRoot, "ls-tree", "-r", "--name-only", "refs/heads/main")
	for _, want := range []string{"landed.md", "src/run.go"} {
		if !strings.Contains(tree, want) {
			t.Fatalf("main tip missing %q after integrate (base-drift must not revert intervening work); tree:\n%s", want, tree)
		}
	}
	if got := gitRun(t, repoRoot, "show", "refs/heads/main:landed.md"); !strings.Contains(got, "landed on main after the fork") {
		t.Fatalf("intervening main work was reverted by integrate: %q", got)
	}
}
