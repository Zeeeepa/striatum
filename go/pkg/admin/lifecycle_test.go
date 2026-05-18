package admin

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
)

type lifecycleRow struct {
	values []any
	err    error
}

func (r lifecycleRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		if i >= len(r.values) {
			break
		}
		switch target := dest[i].(type) {
		case *string:
			*target = r.values[i].(string)
		case *int:
			*target = r.values[i].(int)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

type lifecycleExec struct {
	sql  string
	args []any
}

type lifecycleRunner struct {
	rows  map[string]lifecycleRow
	execs []lifecycleExec
}

func (r *lifecycleRunner) Exec(_ context.Context, sql string, args ...any) error {
	r.execs = append(r.execs, lifecycleExec{sql: sql, args: append([]any(nil), args...)})
	return nil
}

func (r *lifecycleRunner) QueryRow(_ context.Context, sql string, args ...any) db.Row {
	for key, row := range r.rows {
		if strings.Contains(sql, key) {
			return row
		}
	}
	return lifecycleRow{err: pgx.ErrNoRows}
}

func (r *lifecycleRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}

func (r *lifecycleRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("transactions not supported")
}

func TestRegisterInstallsLifecycleHandlers(t *testing.T) {
	server := rpc.NewServer()
	Service{Runner: &fakeRunner{}, DaemonVersion: "test"}.Register(server)

	for _, method := range []string{
		"repo.init",
		"daemon.shutdown",
		"daemon.key.rotate",
	} {
		if server.Handlers[method] == nil {
			t.Fatalf("%s was not registered", method)
		}
	}
}

func TestShutdownFailsClosedWhenHookIsMissing(t *testing.T) {
	_, err := Service{Runner: &fakeRunner{}}.Shutdown(context.Background(), rpc.Envelope{Params: map[string]any{}})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected rpc error, got %v", err)
	}
	if rpcErr.Code != "shutdown_unavailable" {
		t.Fatalf("code = %s", rpcErr.Code)
	}
}

func TestKeyRotateFailsClosedWhenHookIsMissing(t *testing.T) {
	_, err := Service{Runner: &fakeRunner{}}.KeyRotate(context.Background(), rpc.Envelope{Params: map[string]any{}})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected rpc error, got %v", err)
	}
	if rpcErr.Code != "key_rotation_unavailable" {
		t.Fatalf("code = %s", rpcErr.Code)
	}
}

func TestRepoInitCreatesPostgresRegistrationAndOperationalScratch(t *testing.T) {
	repo := t.TempDir()
	runner := &lifecycleRunner{}
	result, err := Service{Runner: runner}.RepoInit(context.Background(), rpc.Envelope{
		Params: map[string]any{"path": repo, "repository_id": "repo_test"},
	})
	if err != nil {
		t.Fatalf("repo init: %v", err)
	}
	if result["repository_id"] != "repo_test" {
		t.Fatalf("repository_id = %v", result["repository_id"])
	}
	if result["substrate"] != "postgres" {
		t.Fatalf("substrate = %v", result["substrate"])
	}
	if result["sqlite_dependency"] != false {
		t.Fatalf("sqlite_dependency = %v", result["sqlite_dependency"])
	}
	if _, err := os.Stat(filepath.Join(repo, ".striatum", "scratch")); err != nil {
		t.Fatalf("scratch was not created: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatalf("read gitignore: %v", err)
	}
	if !strings.Contains(string(body), ".striatum/\n") {
		t.Fatalf("missing .striatum ignore entry: %q", string(body))
	}
	if len(runner.execs) != 1 || !strings.Contains(runner.execs[0].sql, "INSERT INTO striatumd.repositories") {
		t.Fatalf("expected repository insert, got %#v", runner.execs)
	}
	if _, err := os.Stat(filepath.Join(repo, ".striatum", "state.sqlite3")); !os.IsNotExist(err) {
		t.Fatalf("repo.init created or touched SQLite state: %v", err)
	}
}

func TestRepoInitRefusesRepoLocalSQLite(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".striatum"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".striatum", "state.sqlite3"), []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &lifecycleRunner{}
	_, err := Service{Runner: runner}.RepoInit(context.Background(), rpc.Envelope{
		Params: map[string]any{"path": repo, "repository_id": "repo_test"},
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected rpc error, got %v", err)
	}
	if rpcErr.Code != "sqlite_retired" {
		t.Fatalf("code = %s", rpcErr.Code)
	}
	if len(runner.execs) != 0 {
		t.Fatalf("unexpected database writes: %#v", runner.execs)
	}
}
