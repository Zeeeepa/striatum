package mutations

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

type decisionRecordRunner struct {
	tx *decisionRecordTx
}

func (r *decisionRecordRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected runner exec")
}

func (r *decisionRecordRunner) QueryRow(context.Context, string, ...any) db.Row {
	return fakeRow{err: pgx.ErrNoRows}
}

func (r *decisionRecordRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}

func (r *decisionRecordRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return r.tx, nil
}

type decisionRecordTx struct {
	repoRoot   string
	execs      []decisionRecordExec
	committed  bool
	rolledBack bool
}

type decisionRecordExec struct {
	sql  string
	args []any
}

func (tx *decisionRecordTx) Exec(_ context.Context, sql string, args ...any) error {
	tx.execs = append(tx.execs, decisionRecordExec{sql: sql, args: append([]any(nil), args...)})
	return nil
}

func (tx *decisionRecordTx) QueryRow(_ context.Context, sql string, _ ...any) db.Row {
	switch {
	case strings.Contains(sql, "repo_event_chain_heads"):
		return fakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "nextval"):
		return decisionRecordRow{values: []any{int64(1)}}
	default:
		return fakeRow{err: pgx.ErrNoRows}
	}
}

func (tx *decisionRecordTx) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}

func (tx *decisionRecordTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.runs"):
		return runPrepareRowsFromMaps([]map[string]any{{
			"run_id":    "run_1",
			"repo_root": tx.repoRoot,
			"state":     "running",
		}}), nil
	case strings.Contains(sql, "FROM striatumd.artifacts"):
		return runPrepareRowsFromMaps(nil), nil
	default:
		return nil, errors.New("unexpected query: " + sql)
	}
}

func (tx *decisionRecordTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *decisionRecordTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

type decisionRecordRow struct {
	values []any
	err    error
}

func (r decisionRecordRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		if i >= len(r.values) {
			break
		}
		switch target := dest[i].(type) {
		case *int64:
			if value, ok := r.values[i].(int64); ok {
				*target = value
			}
		}
	}
	return nil
}

func TestDecisionRecordPersistsEscapeDecisionMetadata(t *testing.T) {
	repoRoot := t.TempDir()
	tx := &decisionRecordTx{repoRoot: repoRoot}
	runner := &decisionRecordRunner{tx: tx}

	result, err := HandleDecisionRecord(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{
			"repository_id":  "repo_1",
			"run_id":         "run_1",
			"path":           "docs/decisions/ESCAPE.md",
			"outcome":        "accepted",
			"title":          "Permit targeted shell command",
			"decision_id":    "escape_shell_1",
			"rationale":      "Need one declared diagnostic outside the constrained surface.",
			"escape_surface": "shell_command",
			"escape_action":  "run go test ./pkg/reads -run TestDoctor",
		},
	})
	if err != nil {
		t.Fatalf("HandleDecisionRecord: %v", err)
	}
	if result["escape_decision"] != true || result["escape_surface"] != "shell_command" {
		t.Fatalf("result = %#v", result)
	}
	if !tx.committed {
		t.Fatal("transaction was not committed")
	}
	body, err := os.ReadFile(filepath.Join(repoRoot, "docs", "decisions", "ESCAPE.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"escape_decision: true",
		`escape_surface: "shell_command"`,
		`escape_action: "run go test ./pkg/reads -run TestDoctor"`,
		"## Escape Decision",
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("decision body missing %q:\n%s", want, string(body))
		}
	}
	event := decisionRecordEventExec(t, tx)
	payload, ok := event.args[9].(map[string]any)
	if !ok {
		t.Fatalf("event payload arg = %#v", event.args[9])
	}
	if payload["escape_decision"] != true || payload["escape_surface"] != "shell_command" || payload["escape_action"] == "" {
		t.Fatalf("event payload = %#v", payload)
	}
}

func TestDecisionRecordEscapeRequiresRationale(t *testing.T) {
	tx := &decisionRecordTx{repoRoot: t.TempDir()}
	runner := &decisionRecordRunner{tx: tx}

	_, err := HandleDecisionRecord(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{
			"repository_id":  "repo_1",
			"run_id":         "run_1",
			"path":           "docs/decisions/ESCAPE.md",
			"outcome":        "accepted",
			"title":          "Permit targeted shell command",
			"escape_surface": "shell_command",
			"escape_action":  "run go test",
		},
	})
	rpcErr := &rpc.Error{}
	if !errors.As(err, &rpcErr) || rpcErr.Code != "schema_invalid" {
		t.Fatalf("error = %v, want schema_invalid", err)
	}
	if tx.committed || len(tx.execs) != 0 {
		t.Fatalf("escape without rationale wrote rows: committed=%v execs=%#v", tx.committed, tx.execs)
	}
}

func decisionRecordEventExec(t *testing.T, tx *decisionRecordTx) decisionRecordExec {
	t.Helper()
	for _, exec := range tx.execs {
		if strings.Contains(exec.sql, "INSERT INTO striatumd.events") {
			return exec
		}
	}
	t.Fatalf("event insert not found in %#v", tx.execs)
	return decisionRecordExec{}
}
