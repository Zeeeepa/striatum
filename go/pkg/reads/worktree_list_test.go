package reads

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type worktreeListFakeRunner struct {
	query string
	args  []any
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
	return dashboardAllRowsFromMaps([]map[string]any{{
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
	}}), nil
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
