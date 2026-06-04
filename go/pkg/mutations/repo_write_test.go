package mutations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type repoWriteRunner struct {
	repoRoot     string
	writeScope   map[string]any
	worktreePath string
}

func (r repoWriteRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected exec")
}

func (r repoWriteRunner) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: pgx.ErrNoRows}
}

func (r repoWriteRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}

func (r repoWriteRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return &repoWriteTx{repoWriteRunner: r}, nil
}

func (r repoWriteRunner) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.jobs"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"job_id":           "job_1",
			"run_id":           "run_1",
			"write_scope_json": r.writeScope,
		}}), nil
	case strings.Contains(sql, "FROM striatumd.leases"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"lease_id":         "lease_1",
			"state":            "active",
			"owner_session_id": "session_1",
			"resource_id":      "job_1",
			"expires_at":       time.Now().Add(time.Hour),
		}}), nil
	case strings.Contains(sql, "FROM striatumd.runs"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"run_id":    "run_1",
			"repo_root": r.repoRoot,
		}}), nil
	case strings.Contains(sql, "FROM striatumd.job_worktrees"):
		if r.worktreePath == "" {
			return runPrepareRowsFromMaps(nil), nil
		}
		return runPrepareRowsFromMaps([]map[string]any{{
			"worktree_id":   "wt_1",
			"state":         "active",
			"worktree_path": r.worktreePath,
		}}), nil
	default:
		return nil, errors.New("unexpected query: " + sql)
	}
}

type repoWriteTx struct {
	repoWriteRunner
}

func (tx *repoWriteTx) Commit(context.Context) error {
	return nil
}

func (tx *repoWriteTx) Rollback(context.Context) error {
	return nil
}

func TestRepoWriteWritesInsideScope(t *testing.T) {
	repoRoot := t.TempDir()
	runner := repoWriteRunner{
		repoRoot:   repoRoot,
		writeScope: map[string]any{"mode": "repo_write", "allowed_paths": []any{"docs/"}, "forbidden_paths": []any{".striatum/"}},
	}

	result, err := HandleRepoWrite(context.Background(), runner, repoWriteEnvelope(map[string]any{
		"path":    "docs/out.md",
		"content": "hello\n",
	}))
	if err != nil {
		t.Fatalf("repo.write failed: %v", err)
	}
	if result["status"] != "written" || result["bytes"] != 6 {
		t.Fatalf("result = %#v", result)
	}
	body, err := os.ReadFile(filepath.Join(repoRoot, "docs", "out.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestRepoWriteRefusesOutsideScopeBeforeMutation(t *testing.T) {
	repoRoot := t.TempDir()
	runner := repoWriteRunner{
		repoRoot:   repoRoot,
		writeScope: map[string]any{"mode": "repo_write", "allowed_paths": []any{"docs/"}, "forbidden_paths": []any{".striatum/"}},
	}

	_, err := HandleRepoWrite(context.Background(), runner, repoWriteEnvelope(map[string]any{
		"path":    "src/out.md",
		"content": "should not be written\n",
	}))
	assertRepoWriteRPCCode(t, err, "path_outside_scope")
	if _, statErr := os.Stat(filepath.Join(repoRoot, "src", "out.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("out-of-scope file was created: %v", statErr)
	}
}

func TestRepoWriteRefusesForbiddenPathBeforeMutation(t *testing.T) {
	repoRoot := t.TempDir()
	runner := repoWriteRunner{
		repoRoot:   repoRoot,
		writeScope: map[string]any{"mode": "repo_write", "allowed_paths": []any{"docs/"}, "forbidden_paths": []any{"docs/secrets/"}},
	}

	_, err := HandleRepoWrite(context.Background(), runner, repoWriteEnvelope(map[string]any{
		"path":    "docs/secrets/token.txt",
		"content": "should not be written\n",
	}))
	assertRepoWriteRPCCode(t, err, "path_outside_scope")
	if _, statErr := os.Stat(filepath.Join(repoRoot, "docs", "secrets", "token.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("forbidden file was created: %v", statErr)
	}
}

func TestRepoWriteRequiresRepoWriteScope(t *testing.T) {
	repoRoot := t.TempDir()
	runner := repoWriteRunner{
		repoRoot:   repoRoot,
		writeScope: map[string]any{"mode": "review_only_artifact", "allowed_paths": []any{"docs/"}, "forbidden_paths": []any{".striatum/"}},
	}

	_, err := HandleRepoWrite(context.Background(), runner, repoWriteEnvelope(map[string]any{
		"path":    "docs/out.md",
		"content": "should not be written\n",
	}))
	assertRepoWriteRPCCode(t, err, "invalid_transition")
	if _, statErr := os.Stat(filepath.Join(repoRoot, "docs", "out.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("review-only file was created: %v", statErr)
	}
}

func TestRepoWriteUsesActiveWorktree(t *testing.T) {
	repoRoot := t.TempDir()
	worktreePath := filepath.ToSlash(filepath.Join(".striatum", "worktrees", "wt_1"))
	if err := os.MkdirAll(filepath.Join(repoRoot, worktreePath), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := repoWriteRunner{
		repoRoot:     repoRoot,
		worktreePath: worktreePath,
		writeScope:   map[string]any{"mode": "repo_write", "allowed_paths": []any{"docs/"}, "forbidden_paths": []any{".striatum/"}},
	}

	_, err := HandleRepoWrite(context.Background(), runner, repoWriteEnvelope(map[string]any{
		"path":    "docs/out.md",
		"content": "worktree\n",
	}))
	if err != nil {
		t.Fatalf("repo.write failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "docs", "out.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("shared worktree file was created: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(repoRoot, worktreePath, "docs", "out.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "worktree\n" {
		t.Fatalf("body = %q", body)
	}
}

func repoWriteEnvelope(overrides map[string]any) rpc.Envelope {
	params := map[string]any{
		"repository_id": "repo_1",
		"session_id":    "session_1",
		"job_id":        "job_1",
		"lease_id":      "lease_1",
	}
	for key, value := range overrides {
		params[key] = value
	}
	return rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "req_repo_write",
		Method:        "repo.write",
		Params:        params,
	}
}

func assertRepoWriteRPCCode(t *testing.T, err error, code string) {
	t.Helper()
	rpcErr := &rpc.Error{}
	if !errors.As(err, &rpcErr) || rpcErr.Code != code {
		t.Fatalf("error = %v, want rpc code %s", err, code)
	}
}
