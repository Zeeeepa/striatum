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

type repoPatchRunner struct {
	repoRoot   string
	writeScope map[string]any
	execs      []repoPatchExec
}

type repoPatchExec struct {
	sql  string
	args []any
}

func (r *repoPatchRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec")
}

func (r *repoPatchRunner) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: pgx.ErrNoRows}
}

func (r *repoPatchRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}

func (r *repoPatchRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return &repoPatchTx{runner: r}, nil
}

type repoPatchTx struct {
	runner *repoPatchRunner
}

func (tx *repoPatchTx) Exec(_ context.Context, sql string, args ...any) error {
	tx.runner.execs = append(tx.runner.execs, repoPatchExec{sql: sql, args: append([]any(nil), args...)})
	return nil
}

func (tx *repoPatchTx) QueryRow(_ context.Context, sql string, _ ...any) db.Row {
	switch {
	case strings.Contains(sql, "repo_event_chain_heads"):
		return fakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "nextval"):
		return decisionRecordRow{values: []any{int64(1)}}
	default:
		return fakeRow{err: pgx.ErrNoRows}
	}
}

func (tx *repoPatchTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}

func (tx *repoPatchTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.jobs"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"job_id":           "job_1",
			"run_id":           "run_1",
			"write_scope_json": tx.runner.writeScope,
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
			"repo_root": tx.runner.repoRoot,
		}}), nil
	case strings.Contains(sql, "FROM striatumd.job_worktrees"):
		return runPrepareRowsFromMaps(nil), nil
	default:
		return nil, errors.New("unexpected query: " + sql)
	}
}

func (tx *repoPatchTx) Commit(context.Context) error {
	return nil
}

func (tx *repoPatchTx) Rollback(context.Context) error {
	return nil
}

func TestRepoPatchPreviewAndApply(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(repoRoot, "docs", "out.txt")
	if err := os.WriteFile(target, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &repoPatchRunner{
		repoRoot:   repoRoot,
		writeScope: map[string]any{"mode": "repo_write", "allowed_paths": []any{"docs/"}, "forbidden_paths": []any{".striatum/"}},
	}
	patch := repoPatchText("docs/out.txt", "old", "new")

	preview, err := HandleRepoPatchPreview(context.Background(), runner, repoPatchEnvelope(map[string]any{"patch": patch}))
	if err != nil {
		t.Fatalf("repo.patch-preview failed: %v", err)
	}
	if preview["status"] != "previewed" {
		t.Fatalf("preview = %#v", preview)
	}
	if body, err := os.ReadFile(target); err != nil || string(body) != "old\n" {
		t.Fatalf("preview mutated file: %q err=%v", body, err)
	}

	applied, err := HandleRepoPatchApply(context.Background(), runner, repoPatchEnvelope(map[string]any{"patch": patch}))
	if err != nil {
		t.Fatalf("repo.patch-apply failed: %v", err)
	}
	if applied["status"] != "applied" {
		t.Fatalf("apply = %#v", applied)
	}
	if body, err := os.ReadFile(target); err != nil || string(body) != "new\n" {
		t.Fatalf("apply body = %q err=%v", body, err)
	}
}

func TestRepoPatchRefusesOutsideScopeBeforeMutation(t *testing.T) {
	repoRoot := t.TempDir()
	runner := &repoPatchRunner{
		repoRoot:   repoRoot,
		writeScope: map[string]any{"mode": "repo_write", "allowed_paths": []any{"docs/"}, "forbidden_paths": []any{".striatum/"}},
	}
	patch := `diff --git a/src/out.txt b/src/out.txt
new file mode 100644
index 0000000..3bd1f0e
--- /dev/null
+++ b/src/out.txt
@@ -0,0 +1 @@
+outside
`

	_, err := HandleRepoPatchApply(context.Background(), runner, repoPatchEnvelope(map[string]any{"patch": patch}))
	assertRepoWriteRPCCode(t, err, "path_outside_scope")
	if _, statErr := os.Stat(filepath.Join(repoRoot, "src", "out.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("out-of-scope patch was applied: %v", statErr)
	}
}

func repoPatchEnvelope(overrides map[string]any) rpc.Envelope {
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
		RequestID:     "req_repo_patch",
		Method:        "repo.patch_apply",
		Params:        params,
	}
}

func repoPatchText(path, oldText, newText string) string {
	return "diff --git a/" + path + " b/" + path + "\n" +
		"--- a/" + path + "\n" +
		"+++ b/" + path + "\n" +
		"@@ -1 +1 @@\n" +
		"-" + oldText + "\n" +
		"+" + newText + "\n"
}
