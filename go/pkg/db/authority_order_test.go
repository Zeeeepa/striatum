package db

import (
	"context"
	"strings"
	"testing"
)

// recordedStmt captures one statement issued on a transaction, tagging whether
// it went through the extended-protocol ExecBound path or plain Exec.
type recordedStmt struct {
	kind string // "exec-bound" | "exec"
	sql  string
}

type recordingOrderTx struct {
	stmts *[]recordedStmt
}

func (t *recordingOrderTx) Exec(_ context.Context, sql string, _ ...any) error {
	*t.stmts = append(*t.stmts, recordedStmt{kind: "exec", sql: sql})
	return nil
}

func (t *recordingOrderTx) ExecBound(_ context.Context, sql string, _ ...any) error {
	*t.stmts = append(*t.stmts, recordedStmt{kind: "exec-bound", sql: sql})
	return nil
}

func (t *recordingOrderTx) QueryRow(_ context.Context, _ string, _ ...any) Row { return nil }
func (t *recordingOrderTx) QueryScalar(_ context.Context, _ string, _ ...any) (string, error) {
	return "", nil
}
func (t *recordingOrderTx) Commit(context.Context) error   { return nil }
func (t *recordingOrderTx) Rollback(context.Context) error { return nil }

type recordingOrderRunner struct {
	stmts *[]recordedStmt
}

func (r recordingOrderRunner) Exec(context.Context, string, ...any) error { return nil }
func (r recordingOrderRunner) QueryRow(context.Context, string, ...any) Row { return nil }
func (r recordingOrderRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}
func (r recordingOrderRunner) BeginTx(context.Context) (TxRunner, error) {
	return &recordingOrderTx{stmts: r.stmts}, nil
}

// checkAuthorityPreludeFirst is the T-SQL-ORDER assertion: the authority secret
// must be set as the transaction's first statement, over the extended protocol,
// before any other statement (DML or write-function call).
func checkAuthorityPreludeFirst(stmts []recordedStmt) error {
	if len(stmts) == 0 {
		return errNoStatements
	}
	first := stmts[0]
	if first.kind != "exec-bound" {
		return errPreludeNotBound
	}
	if !strings.Contains(first.sql, "set_config('striatum.daemon_auth'") {
		return errPreludeNotDaemonAuth
	}
	return nil
}

var (
	errNoStatements         = preludeError("no statements were recorded on the transaction")
	errPreludeNotBound      = preludeError("first statement did not ride the extended protocol (ExecBound)")
	errPreludeNotDaemonAuth = preludeError("first statement did not set striatum.daemon_auth")
)

type preludeError string

func (e preludeError) Error() string { return string(e) }

// TestAuthorityPreludeIsFirstStatement is T-SQL-ORDER (RFC 0110 §4.3,
// C-AUTH-TX-WRAPPER): a transaction opened through BeginAuthorizedMutation
// records the daemon-authority set_config as its first statement, over the
// extended protocol, before any handler DML. The deliberately non-compliant
// path (a handler that opens its own transaction and skips the prelude) must
// fail the same check, proving it has teeth.
func TestAuthorityPreludeIsFirstStatement(t *testing.T) {
	ctx := context.Background()
	attr := AuthorityContext{Secret: "s", RPCID: "r", PrincipalID: "p", SessionID: "sess"}

	// Compliant: the production constructor installs the prelude first.
	var compliant []recordedStmt
	tx, err := BeginAuthorizedMutation(ctx, recordingOrderRunner{stmts: &compliant}, attr)
	if err != nil {
		t.Fatalf("BeginAuthorizedMutation: %v", err)
	}
	if err := tx.Exec(ctx, "INSERT INTO striatumd.example(x) VALUES ($1)", 1); err != nil {
		t.Fatalf("handler exec: %v", err)
	}
	if err := checkAuthorityPreludeFirst(compliant); err != nil {
		t.Fatalf("compliant handler flagged by T-SQL-ORDER: %v (stmts=%+v)", err, compliant)
	}
	if compliant[0].kind != "exec-bound" {
		t.Fatalf("prelude must be exec-bound, got %q", compliant[0].kind)
	}

	// Non-compliant: a handler that begins its own tx and skips the prelude.
	var bypass []recordedStmt
	rawRunner := recordingOrderRunner{stmts: &bypass}
	rawTx, err := rawRunner.BeginTx(ctx)
	if err != nil {
		t.Fatalf("raw begin: %v", err)
	}
	if err := rawTx.Exec(ctx, "INSERT INTO striatumd.example(x) VALUES ($1)", 1); err != nil {
		t.Fatalf("raw exec: %v", err)
	}
	if err := checkAuthorityPreludeFirst(bypass); err == nil {
		t.Fatal("a prelude-skipping handler passed T-SQL-ORDER (guard has no teeth)")
	}
}
