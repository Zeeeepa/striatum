package repositories

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestHasParentTraversalRejectsBeforeCleaning(t *testing.T) {
	if !hasParentTraversal("repo/../other") {
		t.Fatal("expected parent traversal to be rejected before filepath.Clean")
	}
	if hasParentTraversal("repo/subdir") {
		t.Fatal("ordinary repo path should not be rejected")
	}
}

func TestOperationalScratchInitCreatesScratchAndGitignore(t *testing.T) {
	repo := t.TempDir()

	stateDir, err := operationalScratch(repo, true)
	if err != nil {
		t.Fatalf("operationalScratch: %v", err)
	}

	if stateDir != filepath.Join(repo, ".striatum") {
		t.Fatalf("stateDir = %q", stateDir)
	}
	info, err := os.Stat(filepath.Join(stateDir, "scratch"))
	if err != nil {
		t.Fatalf("scratch missing: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("scratch is not a directory")
	}
	body, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(body), ".striatum/\n") {
		t.Fatalf(".gitignore missing .striatum entry: %q", string(body))
	}
}

func TestResolveReturnsActiveRepositoryMetadata(t *testing.T) {
	repo := t.TempDir()
	runner := &resolveFakeRunner{
		rows: []map[string]any{repositoryRow("repo_active", repo, "active")},
	}

	result, err := Service{Runner: runner}.Resolve(context.Background(), rpc.Envelope{
		Params: map[string]any{"path": repo},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if result["repository_id"] != "repo_active" || result["repo_root"] != repo {
		t.Fatalf("resolved repository = %#v", result)
	}
	if result["display_name"] != filepath.Base(repo) || result["schema_version"] != int64(16) {
		t.Fatalf("metadata = %#v", result)
	}
	if len(runner.execs) > 0 {
		t.Fatalf("repo.resolve should be read-only, execs = %#v", runner.execs)
	}
	if !strings.Contains(runner.query, "FROM striatumd.repositories") || !strings.Contains(runner.query, "state != 'removed'") {
		t.Fatalf("query did not read active repositories only: %s", runner.query)
	}
}

func TestListNormalizesStaleScratchFileProjection(t *testing.T) {
	repo := t.TempDir()
	staleStatePath := filepath.Join(repo, ".striatum", "retired-local-state")
	runner := &resolveFakeRunner{
		rows: []map[string]any{repositoryRowWithStatePath("repo_stale", repo, "active", staleStatePath)},
	}

	result, err := Service{Runner: runner}.List(context.Background(), rpc.Envelope{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	items, ok := result["repositories"].([]map[string]any)
	if !ok || len(items) != 1 {
		t.Fatalf("repositories = %#v", result["repositories"])
	}
	if items[0]["state_db_path"] != filepath.Join(repo, ".striatum") {
		t.Fatalf("state_db_path = %#v", items[0]["state_db_path"])
	}
	if runner.rows[0]["state_db_path"] != staleStatePath {
		t.Fatalf("stored row was rewritten: %#v", runner.rows[0]["state_db_path"])
	}
}

func TestResolveNormalizesStaleScratchFileProjection(t *testing.T) {
	repo := t.TempDir()
	staleStatePath := filepath.Join(repo, ".striatum", "retired-local-state")
	runner := &resolveFakeRunner{
		rows: []map[string]any{repositoryRowWithStatePath("repo_stale", repo, "active", staleStatePath)},
	}

	result, err := Service{Runner: runner}.Resolve(context.Background(), rpc.Envelope{
		Params: map[string]any{"path": repo},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if result["state_db_path"] != filepath.Join(repo, ".striatum") {
		t.Fatalf("state_db_path = %#v", result["state_db_path"])
	}
	if runner.rows[0]["state_db_path"] != staleStatePath {
		t.Fatalf("stored row was rewritten: %#v", runner.rows[0]["state_db_path"])
	}
}

func TestAddAlreadyRegisteredNormalizesStaleScratchFileProjection(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".striatum"), 0o700); err != nil {
		t.Fatalf("mkdir .striatum: %v", err)
	}
	identity, err := repoIdentity(repo)
	if err != nil {
		t.Fatalf("repoIdentity: %v", err)
	}
	staleStatePath := filepath.Join(repo, ".striatum", "retired-local-state")
	row := repositoryRowWithStatePath("repo_existing", repo, "active", staleStatePath)
	row["repo_identity"] = identity
	runner := &resolveFakeRunner{rows: []map[string]any{row}}

	result, err := Service{Runner: runner}.Add(context.Background(), rpc.Envelope{
		Params: map[string]any{"path": repo},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if result["already_registered"] != true {
		t.Fatalf("already_registered = %#v", result["already_registered"])
	}
	if result["state_db_path"] != filepath.Join(repo, ".striatum") {
		t.Fatalf("state_db_path = %#v", result["state_db_path"])
	}
	if runner.rows[0]["state_db_path"] != staleStatePath {
		t.Fatalf("stored row was rewritten: %#v", runner.rows[0]["state_db_path"])
	}
	if len(runner.execs) > 0 {
		t.Fatalf("already-registered repo.add should not write, execs = %#v", runner.execs)
	}
}

func TestResolveNormalizesPathBeforeLookup(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(parent); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	runner := &resolveFakeRunner{
		rows: []map[string]any{repositoryRow("repo_normalized", repo, "active")},
	}

	result, err := Service{Runner: runner}.Resolve(context.Background(), rpc.Envelope{
		Params: map[string]any{"path": "." + string(os.PathSeparator) + filepath.Base(repo)},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if result["repository_id"] != "repo_normalized" {
		t.Fatalf("result = %#v", result)
	}
	if len(runner.args) != 1 || runner.args[0] != repo {
		t.Fatalf("resolve lookup args = %#v, want normalized repo root %q", runner.args, repo)
	}
}

func TestResolveMissingPathReturnsNotRegistered(t *testing.T) {
	repo := t.TempDir()
	_, err := Service{Runner: &resolveFakeRunner{}}.Resolve(context.Background(), rpc.Envelope{
		Params: map[string]any{"path": repo},
	})
	assertRPCErrorCode(t, err, "repo_not_registered")
}

func TestResolveRemovedPathReturnsNotRegistered(t *testing.T) {
	repo := t.TempDir()
	runner := &resolveFakeRunner{
		rows: []map[string]any{repositoryRow("repo_removed", repo, "removed")},
	}

	_, err := Service{Runner: runner}.Resolve(context.Background(), rpc.Envelope{
		Params: map[string]any{"repo_root": repo},
	})

	assertRPCErrorCode(t, err, "repo_not_registered")
}

func repositoryRow(repositoryID string, repoRoot string, state string) map[string]any {
	return repositoryRowWithStatePath(repositoryID, repoRoot, state, filepath.Join(repoRoot, ".striatum"))
}

func repositoryRowWithStatePath(repositoryID string, repoRoot string, state string, stateDBPath string) map[string]any {
	return map[string]any{
		"repository_id":        repositoryID,
		"repo_identity":        "inode:1:2:root:" + repoRoot,
		"repo_root":            repoRoot,
		"state_db_path":        stateDBPath,
		"display_name":         filepath.Base(repoRoot),
		"registered_at":        "2026-05-17T00:00:00Z",
		"removed_at":           nil,
		"last_seen_at":         "2026-05-17T00:00:00Z",
		"last_schema_version":  int64(16),
		"state":                state,
		"unexpected_private":   "not returned",
		"settings_json":        map[string]any{"ignored": true},
		"unrelated_column_nil": nil,
	}
}

func assertRPCErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error = %v, want rpc error %s", err, code)
	}
	if rpcErr.Code != code {
		t.Fatalf("error code = %s, want %s", rpcErr.Code, code)
	}
}

type resolveFakeRunner struct {
	query string
	args  []any
	rows  []map[string]any
	execs []string
}

func (r *resolveFakeRunner) Exec(_ context.Context, sql string, _ ...any) error {
	r.execs = append(r.execs, sql)
	return errors.New("repo.resolve must not write")
}

func (r *resolveFakeRunner) QueryRow(context.Context, string, ...any) db.Row {
	return resolveFakeRow{}
}

func (r *resolveFakeRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected scalar query")
}

func (r *resolveFakeRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("repo.resolve must not open a transaction")
}

func (r *resolveFakeRunner) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.query = sql
	r.args = args
	matched := make([]map[string]any, 0, len(r.rows))
	if len(args) == 0 {
		matched = append(matched, r.rows...)
		return resolveRowsFromMaps(matched), nil
	}
	value, _ := args[0].(string)
	for _, row := range r.rows {
		if strings.Contains(sql, "repo_identity") && row["repo_identity"] == value && row["state"] != "removed" {
			matched = append(matched, row)
			continue
		}
		if row["repo_root"] == value && row["state"] != "removed" {
			matched = append(matched, row)
		}
	}
	return resolveRowsFromMaps(matched), nil
}

type resolveFakeRow struct{}

func (resolveFakeRow) Scan(...any) error {
	return errors.New("unexpected row scan")
}

type resolveFakeRows struct {
	fields []string
	rows   [][]any
	index  int
}

func resolveRowsFromMaps(items []map[string]any) pgx.Rows {
	fields := []string{}
	if len(items) > 0 {
		for key := range items[0] {
			fields = append(fields, key)
		}
	}
	rows := make([][]any, 0, len(items))
	for _, item := range items {
		row := make([]any, 0, len(fields))
		for _, field := range fields {
			row = append(row, item[field])
		}
		rows = append(rows, row)
	}
	return &resolveFakeRows{fields: fields, rows: rows, index: -1}
}

func (r *resolveFakeRows) Close() {}

func (r *resolveFakeRows) Err() error { return nil }

func (r *resolveFakeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *resolveFakeRows) FieldDescriptions() []pgconn.FieldDescription {
	out := make([]pgconn.FieldDescription, 0, len(r.fields))
	for _, field := range r.fields {
		out = append(out, pgconn.FieldDescription{Name: field})
	}
	return out
}

func (r *resolveFakeRows) Next() bool {
	r.index++
	return r.index < len(r.rows)
}

func (r *resolveFakeRows) Scan(dest ...any) error {
	if len(dest) == 1 {
		if scanner, ok := dest[0].(pgx.RowScanner); ok {
			return scanner.ScanRow(r)
		}
	}
	values, err := r.Values()
	if err != nil {
		return err
	}
	for i := range dest {
		if i >= len(values) {
			break
		}
		switch target := dest[i].(type) {
		case *any:
			*target = values[i]
		case *string:
			if value, ok := values[i].(string); ok {
				*target = value
			}
		case *int64:
			if value, ok := values[i].(int64); ok {
				*target = value
			}
		}
	}
	return nil
}

func (r *resolveFakeRows) Values() ([]any, error) {
	if r.index < 0 || r.index >= len(r.rows) {
		return nil, errors.New("no current row")
	}
	return r.rows[r.index], nil
}

func (r *resolveFakeRows) RawValues() [][]byte { return nil }

func (r *resolveFakeRows) Conn() *pgx.Conn { return nil }
