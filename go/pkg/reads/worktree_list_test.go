package reads

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type worktreeListFakeRunner struct {
	query string
	args  []any
	rows  []map[string]any
}

func (r *worktreeListFakeRunner) Exec(context.Context, string, ...any) error {
	return errors.New("worktree.list must be read-only")
}

func (r *worktreeListFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return worktreeListFakeRow{}
}

func (r *worktreeListFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (r *worktreeListFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("worktree.list must not open a transaction")
}

func (r *worktreeListFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.query = sql
	r.args = args
	rows := r.rows
	if rows == nil {
		rows = []map[string]any{{
			"worktree_id":     "wt_1",
			"run_id":          "run_1",
			"job_id":          "job_1",
			"lease_id":        "lease_1",
			"base_branch":     "main",
			"worktree_path":   ".striatum/worktrees/wt_1",
			"state":           "active",
			"created_at":      "2026-05-17T00:00:00Z",
			"released_at":     nil,
			"removed_at":      nil,
			"workflow_job_id": "draft",
		}}
	}
	return dashboardAllRowsFromMaps(rows), nil
}

type worktreeListRepoFakeRunner struct {
	worktreeListFakeRunner
}

func (r *worktreeListRepoFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.query = sql
	r.args = args
	return dashboardAllRowsFromMaps(r.rows), nil
}

func TestHandleWorktreeListProjectsAnchorFromGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	repoRoot := t.TempDir()
	baseSHA := readsGitInit(t, repoRoot)
	runID := "run_fake_anchor"
	jobID := "job_fake_anchor"
	worktreeID := "wt_fake_anchor"
	runBranch := "wf/fake-anchor"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))

	readsGitRun(t, repoRoot, "branch", runBranch, baseSHA)
	readsGitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "worktree.txt"), []byte("worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, worktreeRoot, "add", "worktree.txt")
	readsGitRun(t, worktreeRoot, "commit", "-q", "-m", "worktree change")
	worktreeHead := readsGitRevParse(t, worktreeRoot, "HEAD")
	readsGitRun(t, repoRoot, "update-ref", "refs/heads/"+runBranch, worktreeHead)

	runner := &worktreeListRepoFakeRunner{worktreeListFakeRunner: worktreeListFakeRunner{rows: []map[string]any{{
		"worktree_id":     "wt_1",
		"run_id":          runID,
		"job_id":          jobID,
		"lease_id":        "lease_1",
		"base_branch":     runBranch,
		"branch_name":     runBranch,
		"repo_root":       repoRoot,
		"worktree_path":   worktreeRel,
		"state":           "active",
		"created_at":      "2026-05-17T00:00:00Z",
		"released_at":     nil,
		"removed_at":      nil,
		"workflow_job_id": "draft",
		"job_state":       "completed",
	}}}}
	result, err := HandleWorktreeList(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "run_id": runID},
	})
	if err != nil {
		t.Fatalf("HandleWorktreeList: %v", err)
	}
	worktrees := result["worktrees"].([]map[string]any)
	if len(worktrees) != 1 {
		t.Fatalf("worktrees = %#v", worktrees)
	}
	row := worktrees[0]
	if row["head"] != worktreeHead {
		t.Fatalf("head = %#v, want %s", row["head"], worktreeHead)
	}
	if row["reachable"] != true || row["anchor"] != "run_branch" || row["anchored_ref"] != "refs/heads/"+runBranch {
		t.Fatalf("anchor projection = %#v", row)
	}
}

func TestHandleWorktreeListProjectsJobPinAnchorFromGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	repoRoot := t.TempDir()
	baseSHA := readsGitInit(t, repoRoot)
	runID := "run_fake_pin"
	jobID := "job_fake_pin"
	worktreeID := "wt_fake_pin"
	runBranch := "wf/fake-pin"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))

	readsGitRun(t, repoRoot, "branch", runBranch, baseSHA)
	readsGitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "worktree.txt"), []byte("worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, worktreeRoot, "add", "worktree.txt")
	readsGitRun(t, worktreeRoot, "commit", "-q", "-m", "worktree change")
	worktreeHead := readsGitRevParse(t, worktreeRoot, "HEAD")
	pinRef := "refs/striatum/" + runID + "/" + jobID
	readsGitRun(t, repoRoot, "update-ref", pinRef, worktreeHead)

	runner := &worktreeListRepoFakeRunner{worktreeListFakeRunner: worktreeListFakeRunner{rows: []map[string]any{{
		"worktree_id":     worktreeID,
		"run_id":          runID,
		"job_id":          jobID,
		"lease_id":        "lease_1",
		"base_branch":     runBranch,
		"branch_name":     runBranch,
		"repo_root":       repoRoot,
		"worktree_path":   worktreeRel,
		"state":           "active",
		"created_at":      "2026-05-17T00:00:00Z",
		"released_at":     nil,
		"removed_at":      nil,
		"workflow_job_id": "draft",
		"job_state":       "completed",
	}}}}
	result, err := HandleWorktreeList(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "run_id": runID},
	})
	if err != nil {
		t.Fatalf("HandleWorktreeList: %v", err)
	}
	worktrees := result["worktrees"].([]map[string]any)
	row := worktrees[0]
	if row["head"] != worktreeHead {
		t.Fatalf("head = %#v, want %s", row["head"], worktreeHead)
	}
	if row["reachable"] != true || row["anchor"] != "job_pin" || row["anchored_ref"] != pinRef {
		t.Fatalf("anchor projection = %#v", row)
	}
}

type worktreeListFakeRow struct{}

func (worktreeListFakeRow) Scan(...any) error {
	return errors.New("unexpected row scan")
}

func TestHandleWorktreeListScopesByRepositoryAndOptionalRun(t *testing.T) {
	runner := &worktreeListFakeRunner{}
	result, err := HandleWorktreeList(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "run_id": "run_1"},
	})
	if err != nil {
		t.Fatalf("HandleWorktreeList: %v", err)
	}
	if !strings.Contains(runner.query, "w.repository_id = $1") || !strings.Contains(runner.query, "w.run_id = $2") {
		t.Fatalf("query is not repository/run scoped: %s", runner.query)
	}
	if len(runner.args) != 2 || runner.args[0] != "repo_1" || runner.args[1] != "run_1" {
		t.Fatalf("args = %#v", runner.args)
	}
	worktrees := result["worktrees"].([]map[string]any)
	if len(worktrees) != 1 || worktrees[0]["workflow_job_id"] != "draft" {
		t.Fatalf("worktrees = %#v", worktrees)
	}
}

func TestWorktreeListShowsAnchor(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	baseSHA := readsGitInit(t, repoRoot)
	runID := "run_anchor_list"
	jobID := "job_anchor_list"
	worktreeID := "wt_anchor_list"
	runBranch := "wf/anchor-list"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))

	readsGitRun(t, repoRoot, "branch", runBranch, baseSHA)
	readsGitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "worktree.txt"), []byte("worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, worktreeRoot, "add", "worktree.txt")
	readsGitRun(t, worktreeRoot, "commit", "-q", "-m", "worktree change")
	worktreeHead := readsGitRevParse(t, worktreeRoot, "HEAD")
	readsGitRun(t, repoRoot, "update-ref", "refs/heads/"+runBranch, worktreeHead)

	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ('repo_anchor_list','ident_anchor_list',$1,$2,'repo',$3,16,'active')`,
		repoRoot, filepath.Join(repoRoot, ".striatum"), now); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ('repo_anchor_list','snap_anchor_list','wf_anchor_list','sha','{}'::jsonb,$1)`, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state,
		  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at
		) VALUES ('repo_anchor_list',$1,'snap_anchor_list',$2,'running',$3,$4,$5,'human',$5)`,
		runID, repoRoot, runBranch, baseSHA, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json,
		  write_scope_json, created_at
		) VALUES ('repo_anchor_list',$1,$2,'author_draft',1,'completed','author',
		  'Author draft','build','idem_anchor_list','[]'::jsonb,
		  '{"mode":"repo_write","repo_write":true}'::jsonb,$3)`,
		jobID, runID, now); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_worktrees (
		  repository_id, worktree_id, run_id, job_id, lease_id,
		  base_branch, worktree_path, state, created_at
		) VALUES ('repo_anchor_list',$1,$2,$3,NULL,$4,$5,'active',$6)`,
		worktreeID, runID, jobID, runBranch, worktreeRel, now); err != nil {
		t.Fatalf("insert worktree: %v", err)
	}

	result, err := HandleWorktreeList(ctx, runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_anchor_list", "run_id": runID},
	})
	if err != nil {
		t.Fatalf("HandleWorktreeList: %v", err)
	}
	worktrees := result["worktrees"].([]map[string]any)
	if len(worktrees) != 1 {
		t.Fatalf("worktrees = %#v", worktrees)
	}
	row := worktrees[0]
	if row["head"] != worktreeHead {
		t.Fatalf("head = %#v, want %s", row["head"], worktreeHead)
	}
	if row["reachable"] != true || row["anchor"] != "run_branch" || row["anchored_ref"] != "refs/heads/"+runBranch {
		t.Fatalf("anchor projection = %#v", row)
	}
	checked, ok := row["checked_refs"].([]string)
	if !ok || len(checked) == 0 || checked[0] != "refs/heads/"+runBranch {
		t.Fatalf("checked_refs = %#v", row["checked_refs"])
	}
}

func readsGitInit(t *testing.T, repoRoot string) string {
	t.Helper()
	readsGitRun(t, repoRoot, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, repoRoot, "add", ".")
	readsGitRun(t, repoRoot, "commit", "-q", "-m", "seed")
	return readsGitRevParse(t, repoRoot, "HEAD")
}

func readsGitRun(t *testing.T, repoRoot string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, repoRoot, err, out)
	}
	return strings.TrimSpace(string(out))
}

func readsGitRevParse(t *testing.T, repoRoot, ref string) string {
	t.Helper()
	return readsGitRun(t, repoRoot, "rev-parse", ref)
}
