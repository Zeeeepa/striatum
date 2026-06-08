package mutations

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/pgtest"
)

func TestWorktreeGCDecisionRemovesAnchoredTerminalWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runID := "run_gc_anchor"
	jobID := "job_gc_anchor"
	runBranch := "wf/gc-anchor"
	worktreeRel, worktreeRoot, worktreeHead := seedGCWorktreeCommit(t, repoRoot, runBranch, baseSHA, "wt_gc_anchor")
	gitRun(t, repoRoot, "update-ref", "refs/heads/"+runBranch, worktreeHead)

	decision, err := worktreeGCDecision(context.Background(), repoRoot, map[string]any{
		"run_id":        runID,
		"job_id":        jobID,
		"worktree_path": worktreeRel,
		"base_branch":   runBranch,
		"branch_name":   runBranch,
		"job_state":     "completed",
	})
	if err != nil {
		t.Fatalf("worktreeGCDecision: %v", err)
	}
	if !decision.Remove || decision.Reason != "" || decision.Target != worktreeRoot {
		t.Fatalf("decision = %#v, want removable target %s", decision, worktreeRoot)
	}
	if decision.Reachability.Head != worktreeHead || !decision.Reachability.Reachable {
		t.Fatalf("reachability = %#v, want reachable head %s", decision.Reachability, worktreeHead)
	}
}

func TestWorktreeGCDecisionSkipsUnanchoredTerminalWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runBranch := "wf/gc-unanchored"
	worktreeRel, _, worktreeHead := seedGCWorktreeCommit(t, repoRoot, runBranch, baseSHA, "wt_gc_unanchored")

	decision, err := worktreeGCDecision(context.Background(), repoRoot, map[string]any{
		"run_id":        "run_gc_unanchored",
		"job_id":        "job_gc_unanchored",
		"worktree_path": worktreeRel,
		"base_branch":   runBranch,
		"branch_name":   runBranch,
		"job_state":     "completed",
	})
	if err != nil {
		t.Fatalf("worktreeGCDecision: %v", err)
	}
	if decision.Remove || decision.Reason != "head_unreachable" {
		t.Fatalf("decision = %#v, want head_unreachable skip", decision)
	}
	if decision.Reachability.Head != worktreeHead || decision.Reachability.Reachable {
		t.Fatalf("reachability = %#v, want unreachable head %s", decision.Reachability, worktreeHead)
	}
}

func TestWorktreeGCDecisionSkipsNonTerminalJob(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runBranch := "wf/gc-running"
	worktreeRel, _, _ := seedGCWorktreeCommit(t, repoRoot, runBranch, baseSHA, "wt_gc_running")

	decision, err := worktreeGCDecision(context.Background(), repoRoot, map[string]any{
		"run_id":        "run_gc_running",
		"job_id":        "job_gc_running",
		"worktree_path": worktreeRel,
		"base_branch":   runBranch,
		"branch_name":   runBranch,
		"job_state":     "running",
	})
	if err != nil {
		t.Fatalf("worktreeGCDecision: %v", err)
	}
	if decision.Remove || decision.Reason != "job_not_terminal" {
		t.Fatalf("decision = %#v, want job_not_terminal skip", decision)
	}
}

func TestWorktreeGCDecisionSkipsMissingOnDiskWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	runBranch := "wf/gc-missing"
	worktreeRel, worktreeRoot, _ := seedGCWorktreeCommit(t, repoRoot, runBranch, baseSHA, "wt_gc_missing")
	gitRun(t, repoRoot, "worktree", "remove", "--force", worktreeRoot)

	decision, err := worktreeGCDecision(context.Background(), repoRoot, map[string]any{
		"run_id":        "run_gc_missing",
		"job_id":        "job_gc_missing",
		"worktree_path": worktreeRel,
		"base_branch":   runBranch,
		"branch_name":   runBranch,
		"job_state":     "completed",
	})
	if err != nil {
		t.Fatalf("worktreeGCDecision: %v", err)
	}
	if decision.Remove || decision.Reason != "missing_on_disk" {
		t.Fatalf("decision = %#v, want missing_on_disk skip", decision)
	}
}

func TestWorktreeGCRemovesAnchoredTerminalWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoRoot := t.TempDir()
	baseSHA := gitInit(t, repoRoot)
	repoID := "repo_gc_remove"
	runID := "run_gc_remove"
	jobID := "job_gc_remove"
	sessionID := "sess_gc_remove"
	leaseID := "lease_gc_remove"
	worktreeID := "wt_gc_remove"
	runBranch := "wf/gc-remove"
	worktreeRel, worktreeRoot, worktreeHead := seedGCWorktreeCommit(t, repoRoot, runBranch, baseSHA, worktreeID)
	gitRun(t, repoRoot, "update-ref", "refs/heads/"+runBranch, worktreeHead)

	now := time.Now().UTC()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.repositories (
		  repository_id, repo_identity, repo_root, state_db_path, display_name,
		  registered_at, last_schema_version, state
		) VALUES ($1,'ident_gc_remove',$2,$3,'repo',$4,16,'active')`,
		repoID, repoRoot, filepath.Join(repoRoot, ".striatum"), now); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.workflow_snapshots (
		  repository_id, workflow_snapshot_id, workflow_id, content_sha256, workflow_json, loaded_at
		) VALUES ($1,'snap_gc_remove','wf_gc_remove','sha','{}'::jsonb,$2)`, repoID, now); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.runs (
		  repository_id, run_id, workflow_snapshot_id, repo_root, state,
		  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at
		) VALUES ($1,$2,'snap_gc_remove',$3,'running',$4,$5,$6,'human',$6)`,
		repoID, runID, repoRoot, runBranch, baseSHA, now); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.sessions (
		  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal, state, registered_at
		) VALUES ($1,$2,$3,'author','codex','author-codex-1',1,'closed',$4)`,
		repoID, sessionID, runID, now); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.jobs (
		  repository_id, job_id, run_id, workflow_job_id, attempt, state, role_id,
		  title, job_type, idempotency_key, expected_artifacts_json,
		  write_scope_json, created_at, completed_at
		) VALUES ($1,$2,$3,'author_draft',1,'completed','author',
		  'Author draft','build','idem_gc_remove','[]'::jsonb,
		  '{"mode":"repo_write","repo_write":true}'::jsonb,$4,$4)`,
		repoID, jobID, runID, now); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.leases (
		  repository_id, lease_id, run_id, resource_type, resource_id, owner_session_id,
		  state, acquired_at, expires_at, released_at, release_reason
		) VALUES ($1,$2,$3,'job',$4,$5,'released',$6,$7,$6,'completed')`,
		repoID, leaseID, runID, jobID, sessionID, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("insert lease: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_worktrees (
		  repository_id, worktree_id, run_id, job_id, lease_id,
		  base_branch, worktree_path, state, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'active',$8)`,
		repoID, worktreeID, runID, jobID, leaseID, runBranch, worktreeRel, now); err != nil {
		t.Fatalf("insert worktree: %v", err)
	}

	result, err := HandleWorktreeGC(ctx, runner, intgEnv(repoID, map[string]any{"run_id": runID}))
	if err != nil {
		t.Fatalf("HandleWorktreeGC: %v", err)
	}
	if result["removed_count"] != 1 || result["skipped_count"] != 0 {
		t.Fatalf("gc result = %#v", result)
	}
	if _, err := os.Stat(worktreeRoot); !os.IsNotExist(err) {
		t.Fatalf("worktree path still exists or unexpected stat error: %v", err)
	}
	row, err := oneRow(ctx, runner, `
		SELECT state
		  FROM striatumd.job_worktrees
		 WHERE repository_id = $1 AND worktree_id = $2`, repoID, worktreeID)
	if err != nil {
		t.Fatalf("read worktree row: %v", err)
	}
	if row["state"] != "removed" {
		t.Fatalf("worktree state = %#v, want removed", row["state"])
	}
	event, err := oneRow(ctx, runner, `
		SELECT payload_json
		  FROM striatumd.events
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND job_id = $3
		   AND event_type = 'worktree.gc_removed'`, repoID, runID, jobID)
	if err != nil {
		t.Fatalf("missing worktree.gc_removed event: %v", err)
	}
	payload := asMap(event["payload_json"])
	if payload["head"] != worktreeHead || payload["reachable"] != true {
		t.Fatalf("event payload = %#v, want reachable head %s", payload, worktreeHead)
	}
}

func seedGCWorktreeCommit(t *testing.T, repoRoot, runBranch, baseSHA, worktreeID string) (string, string, string) {
	t.Helper()
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))
	gitRun(t, repoRoot, "branch", runBranch, baseSHA)
	gitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "worktree.txt"), []byte("worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, worktreeRoot, "add", "worktree.txt")
	gitRun(t, worktreeRoot, "commit", "-q", "-m", "worktree change")
	return worktreeRel, worktreeRoot, gitRevParse(t, worktreeRoot, "HEAD")
}
