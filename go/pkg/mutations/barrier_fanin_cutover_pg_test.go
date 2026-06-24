package mutations

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

func TestFaninCutoverStagesPinsAndAssemblesBeforeDownstreamQueues(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	pool := pgtest.Pool(t)
	runner := pool.Runner

	repoRoot := t.TempDir()
	base := gitInit(t, repoRoot)
	runBranch := "wf/fanin-cutover"
	gitRun(t, repoRoot, "branch", runBranch, base)

	wtAPath := filepath.ToSlash(filepath.Join(".striatum", "worktrees", "wt_fanin_a"))
	wtBPath := filepath.ToSlash(filepath.Join(".striatum", "worktrees", "wt_fanin_b"))
	wtARoot := filepath.Join(repoRoot, filepath.FromSlash(wtAPath))
	wtBRoot := filepath.Join(repoRoot, filepath.FromSlash(wtBPath))
	gitRun(t, repoRoot, "worktree", "add", "--detach", wtARoot, runBranch)
	gitRun(t, repoRoot, "worktree", "add", "--detach", wtBRoot, runBranch)
	headA := commitWorktreeFile(t, wtARoot, "docs/a.txt", "A\n")
	headB := commitWorktreeFile(t, wtBRoot, "docs/b.txt", "B\n")

	repoID := "repo_fanin_cutover"
	runID := "run_fanin_cutover"
	jobA := "job_author_a"
	jobB := "job_author_b"
	jobSynth := "job_synth"
	seedFaninDispatchRepo(t, ctx, runner, repoID, repoRoot)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"lanes": map[string]any{
			"codex": map[string]any{"worktree_isolation": "per_job"},
		},
	})
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		UPDATE striatumd.runs
		   SET repo_root = $1,
		       branch_name = $2,
		       branch_base = $3,
		       branch_confirmed_at = $4,
		       branch_confirmed_by = 'test'
		 WHERE repository_id = $5 AND run_id = $6`,
		repoRoot, runBranch, base, now, repoID, runID); err != nil {
		t.Fatalf("confirm run branch: %v", err)
	}

	seedFaninCutoverJob(t, ctx, runner, repoID, runID, jobA, "author_a", "running", true)
	seedFaninCutoverJob(t, ctx, runner, repoID, runID, jobB, "author_b", "running", true)
	seedFaninCutoverJob(t, ctx, runner, repoID, runID, jobSynth, "synth", "blocked", false)
	seedFaninCutoverWorktree(t, ctx, runner, repoID, runID, jobA, "wt_fanin_a", "lease_fanin_a", wtAPath, runBranch)
	seedFaninCutoverWorktree(t, ctx, runner, repoID, runID, jobB, "wt_fanin_b", "lease_fanin_b", wtBPath, runBranch)
	seedFaninCutoverDependency(t, ctx, runner, repoID, jobSynth, jobA)
	seedFaninCutoverDependency(t, ctx, runner, repoID, jobSynth, jobB)

	tx := beginBarrierTx(t, ctx, pool)
	withFaninAssemblyFlag(t, true, func() {
		if err := recordRunFaninFreezePoints(ctx, tx, repoID, runID, base, []dependencyEdge{
			{fromID: "author_a", toID: "synth"},
			{fromID: "author_b", toID: "synth"},
		}); err != nil {
			t.Fatalf("record run fan-in freeze points: %v", err)
		}

		rowA, err := rowByID(ctx, tx, repoID, "jobs", "job_id", jobA, true)
		if err != nil {
			t.Fatalf("load job A: %v", err)
		}
		payloadA, err := anchorActiveWorktreeForJob(ctx, tx, repoID, rowA)
		if err != nil {
			t.Fatalf("anchor job A through fan-in barrier: %v", err)
		}
		if payloadA["anchor"] != "job_pin" || payloadA["fanin_staged_ref"] == "" {
			t.Fatalf("job A payload = %#v, want pin plus fan-in staging ref", payloadA)
		}
		if got := gitRevParse(t, repoRoot, "refs/heads/"+runBranch); got != base {
			t.Fatalf("first fan-in sibling moved run branch to %s; want frozen base %s until barrier assembly", got, base)
		}
		if err := markFaninCutoverJobCompleted(ctx, tx, repoID, jobA); err != nil {
			t.Fatalf("complete job A: %v", err)
		}
		if err := maybeEnqueueDownstream(ctx, tx, repoID, jobA); err != nil {
			t.Fatalf("maybe enqueue after job A: %v", err)
		}
		assertFaninCutoverJobState(t, ctx, tx, repoID, jobSynth, "blocked")

		rowB, err := rowByID(ctx, tx, repoID, "jobs", "job_id", jobB, true)
		if err != nil {
			t.Fatalf("load job B: %v", err)
		}
		payloadB, err := anchorActiveWorktreeForJob(ctx, tx, repoID, rowB)
		if err != nil {
			t.Fatalf("anchor job B through fan-in barrier: %v", err)
		}
		if payloadB["anchor"] != "job_pin" || payloadB["fanin_staged_ref"] == "" {
			t.Fatalf("job B payload = %#v, want pin plus fan-in staging ref", payloadB)
		}
		if got := gitRevParse(t, repoRoot, "refs/heads/"+runBranch); got != base {
			t.Fatalf("second fan-in sibling moved run branch before gate assembly to %s; want frozen base %s", got, base)
		}
		if err := markFaninCutoverJobCompleted(ctx, tx, repoID, jobB); err != nil {
			t.Fatalf("complete job B: %v", err)
		}
		if err := maybeEnqueueDownstream(ctx, tx, repoID, jobB); err != nil {
			t.Fatalf("maybe enqueue after job B: %v", err)
		}

		assertFaninCutoverJobState(t, ctx, tx, repoID, jobSynth, "queued")
		barrierID := faninBarrierID(runID, "synth")
		state, err := loadBarrierAssemblyState(ctx, tx, repoID, barrierID)
		if err != nil {
			t.Fatalf("load barrier assembly state: %v", err)
		}
		if !state.Present || state.State != barrierStateCommitted {
			t.Fatalf("barrier state = %+v, want committed", state)
		}
		runTip := gitRevParse(t, repoRoot, "refs/heads/"+runBranch)
		if runTip != state.TargetCommitSHA {
			t.Fatalf("run branch tip = %s, barrier target = %s", runTip, state.TargetCommitSHA)
		}
		tree := gitRun(t, repoRoot, "ls-tree", "-r", "--name-only", "refs/heads/"+runBranch)
		for _, want := range []string{"docs/a.txt", "docs/b.txt"} {
			if !strings.Contains(tree, want) {
				t.Fatalf("assembled run branch missing %s; tree:\n%s", want, tree)
			}
		}
		if got := gitRevParse(t, repoRoot, attemptPinRef(runID, jobA, 1)); got != headA {
			t.Fatalf("job A pin = %s, want %s", got, headA)
		}
		if got := gitRevParse(t, repoRoot, attemptPinRef(runID, jobB, 1)); got != headB {
			t.Fatalf("job B pin = %s, want %s", got, headB)
		}
	})
	_ = tx.Rollback(ctx)
}

func seedFaninCutoverJob(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, workflowJobID, state string, repoWrite bool) {
	t.Helper()
	laneArg, err := db.JSONBArg(runner, map[string]any{"lane_id": "codex"})
	if err != nil {
		t.Fatal(err)
	}
	scope := map[string]any{}
	if repoWrite {
		scope = map[string]any{
			"mode":            "repo_write",
			"allowed_paths":   []any{"docs/"},
			"forbidden_paths": []any{".striatum/"},
		}
	}
	scopeArg, err := db.JSONBArg(runner, scope)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, title, job_type, role_id,
		  lane_selector_json, state, attempt, max_attempts, write_scope_json,
		  idempotency_key, created_at
		)
		VALUES ($1,$2,$3,$4,'Fan-in cutover','build','author',$5::jsonb,$6,1,1,$7::jsonb,$8,$9)`,
		repoID, jobID, runID, workflowJobID, laneArg, state, scopeArg, "idem_"+jobID, time.Now().UTC()); err != nil {
		t.Fatalf("insert job %s: %v", jobID, err)
	}
}

func seedFaninCutoverWorktree(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, jobID, worktreeID, leaseID, worktreePath, runBranch string) {
	t.Helper()
	sessionID := "sess_" + jobID
	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal, state, registered_at
		) VALUES ($1,$2,$3,$4,'codex',$5,1,'active',$6)`,
		repoID, sessionID, runID, "role_"+jobID, "slug_"+jobID, now); err != nil {
		t.Fatalf("insert session for %s: %v", jobID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at
		) VALUES ($1,$2,$3,'job',$4,$5,'active',$6,$7)`,
		repoID, leaseID, runID, jobID, sessionID, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease for %s: %v", jobID, err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_worktrees (
		  repository_id, worktree_id, run_id, job_id, lease_id,
		  base_branch, worktree_path, state, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'active',$8)`,
		repoID, worktreeID, runID, jobID, leaseID, runBranch, worktreePath, now); err != nil {
		t.Fatalf("insert worktree for %s: %v", jobID, err)
	}
}

func seedFaninCutoverDependency(t *testing.T, ctx context.Context, runner db.Runner, repoID, jobID, dependsOn string) {
	t.Helper()
	gateArg, err := db.JSONBArg(runner, map[string]any{"on": "completed"})
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id, gate_json)
		VALUES ($1,$2,$3,$4::jsonb)`,
		repoID, jobID, dependsOn, gateArg); err != nil {
		t.Fatalf("insert dependency %s -> %s: %v", dependsOn, jobID, err)
	}
}

func markFaninCutoverJobCompleted(ctx context.Context, runner db.TxRunner, repoID, jobID string) error {
	return runner.Exec(ctx, `
		UPDATE striatumd.jobs
		   SET state = 'completed', completed_at = now(), current_lease_id = NULL
		 WHERE repository_id = $1 AND job_id = $2`, repoID, jobID)
}

func assertFaninCutoverJobState(t *testing.T, ctx context.Context, runner db.TxRunner, repoID, jobID, want string) {
	t.Helper()
	row, err := rowByID(ctx, runner, repoID, "jobs", "job_id", jobID, false)
	if err != nil {
		t.Fatalf("load job %s: %v", jobID, err)
	}
	if got := row["state"]; got != want {
		t.Fatalf("job %s state = %v, want %s", jobID, got, want)
	}
}
