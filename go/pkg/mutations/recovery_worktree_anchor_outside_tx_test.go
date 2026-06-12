package mutations

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

func TestSweepAnchorsAbandonedWorktreeOutsideTransaction(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	repoID := "repo_sweep_worktree_anchor"
	runID := "run_sweep_worktree_anchor"
	jobID := "job_sweep_worktree_anchor"
	leaseID := "lease_sweep_worktree_anchor"
	msgID := "msg_sweep_worktree_anchor"
	sessionID := "sess_sweep_worktree_anchor"
	worktreeID := "wt_sweep_worktree_anchor"
	runBranch := "wf/sweep-worktree-anchor"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))

	gitRun(t, repoRoot, "branch", runBranch, baseSHA)
	gitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "sweep.txt"), []byte("anchored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktreeRoot, "add", "sweep.txt")
	gitRun(t, worktreeRoot, "commit", "-q", "-m", "sweep worktree change")

	seedExpiredRepoWriteWorktreeLease(t, ctx, runner, repoID, runID, jobID, leaseID, msgID, sessionID, worktreeID, repoRoot, runBranch, worktreeRel)

	type gitCall struct{ inTx bool }
	var (
		mu    sync.Mutex
		calls []gitCall
	)
	origRunGit := runGitWorktreeCommand
	runGitWorktreeCommand = func(gctx context.Context, repoRoot string, args ...string) (gitWorktreeResult, error) {
		mu.Lock()
		calls = append(calls, gitCall{inTx: inSweepTx(gctx)})
		mu.Unlock()
		return origRunGit(gctx, repoRoot, args...)
	}
	t.Cleanup(func() { runGitWorktreeCommand = origRunGit })

	result, err := SweepRun(ctx, runner, repoID, runID, "")
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if got := intValue(result["published_count"]); got != 0 {
		t.Fatalf("published_count = %d, want 0", got)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) == 0 {
		t.Fatalf("expected sweep to anchor the abandoned worktree with git commands")
	}
	for i, c := range calls {
		if c.inTx {
			t.Fatalf("git anchor command #%d ran INSIDE the sweep transaction (#198 recurrence): abandoned worktree anchoring must run before the lock-holding transaction", i)
		}
	}
}

func seedExpiredRepoWriteWorktreeLease(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, leaseID, msgID, sessionID, worktreeID, repoRoot, runBranch, worktreeRel string) {
	t.Helper()
	intgSeedRepo(t, ctx, runner, repoID)
	if err := runner.Exec(ctx, `
		UPDATE striatumd.repositories
		   SET repo_root = $2
		 WHERE repository_id = $1`, repoID, repoRoot); err != nil {
		t.Fatalf("update repository root: %v", err)
	}
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"implementer": map[string]any{}},
		"lanes":       map[string]any{"codex": map[string]any{}},
		"jobs":        []any{map[string]any{"id": "implement", "type": "build", "role_id": "implementer"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "implementer", "codex", []string{"write"}, "active")

	now := time.Now().UTC()
	wsArg, err := db.JSONBArg(runner, map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []any{"."}})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json, write_scope_json,
		  lane_selector_json, current_message_id, current_lease_id, created_at, started_at
		) VALUES ($1,$2,$3,'implement',1,'running','implementer','Implement','build',
		          'idem_sweep_anchor_'||$1,'[]'::jsonb,$4::jsonb,'{"lane_id":"codex"}'::jsonb,$5,$6,$7,$7)`,
		repoID, jobID, runID, wsArg, msgID, leaseID, now); err != nil {
		t.Fatalf("insert running repo-write job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.queue_messages (
		  repository_id, message_id, run_id, job_id, kind, state, priority,
		  target_role_id, target_lane_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,'work','acked',0,'implementer','codex',$5,$5)`,
		repoID, msgID, runID, jobID, now); err != nil {
		t.Fatalf("insert queue message: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id,
		  owner_session_id, state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',$6,$7)`,
		repoID, leaseID, runID, jobID, sessionID, now.Add(-2*time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatalf("insert expired active lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_worktrees (
		  repository_id, worktree_id, run_id, job_id, lease_id,
		  base_branch, worktree_path, state, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'active',$8)`,
		repoID, worktreeID, runID, jobID, leaseID, runBranch, worktreeRel, now); err != nil {
		t.Fatalf("insert active worktree: %v", err)
	}
}
