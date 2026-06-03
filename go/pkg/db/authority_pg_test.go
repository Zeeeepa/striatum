package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// singleConnRunner is a db.Runner bound to a single pooled connection, so a
// transaction and the post-transaction GUC check below run on the same physical
// backend — the only way to prove the authority/attribution GUCs are actually
// cleared rather than merely absent on a different connection.
type singleConnRunner struct{ conn *pgxpool.Conn }

func (r singleConnRunner) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := r.conn.Exec(ctx, sql, args...)
	return err
}

func (r singleConnRunner) QueryRow(ctx context.Context, sql string, args ...any) db.Row {
	return r.conn.QueryRow(ctx, sql, args...)
}

func (r singleConnRunner) QueryScalar(ctx context.Context, sql string, args ...any) (string, error) {
	var v string
	if err := r.conn.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return v, nil
}

func (r singleConnRunner) BeginTx(ctx context.Context) (db.TxRunner, error) {
	tx, err := r.conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &db.PgxTxRunner{Tx: tx}, nil
}

var authorityGUCs = []string{
	"striatum.daemon_auth",
	"striatum.rpc_id",
	"striatum.principal_id",
	"app.session_id",
}

func currentGUC(t *testing.T, ctx context.Context, runner singleConnRunner, name string) string {
	t.Helper()
	v, err := runner.QueryScalar(ctx, "SELECT COALESCE(current_setting($1, true), '')", name)
	if err != nil {
		t.Fatalf("read GUC %s: %v", name, err)
	}
	return v
}

// TestAuthorityPreludeResetsAcrossOutcomes is T-ATTR-RESET (RFC 0110 §4.7,
// C-ATTR-RESET-FAIL / OPS-12): after commit, rollback, cancel, timeout, and
// panic, a connection returned to use carries none of the authority secret or
// attribution labels. The prelude sets them transaction-local, so they vanish
// at every transaction boundary; this test pins that they are set mid-tx and
// gone afterwards on the same backend.
func TestAuthorityPreludeResetsAcrossOutcomes(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()

	conn, err := pool.RawPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire conn: %v", err)
	}
	defer conn.Release()
	runner := singleConnRunner{conn: conn}

	attr := db.AuthorityContext{
		Secret:      "secret-value",
		RPCID:       "rpc-123",
		PrincipalID: "principal-abc",
		SessionID:   "session-xyz",
	}

	cases := []struct {
		name string
		end  func(t *testing.T, tx db.TxRunner)
	}{
		{"commit", func(t *testing.T, tx db.TxRunner) {
			if err := tx.Commit(ctx); err != nil {
				t.Fatalf("commit: %v", err)
			}
		}},
		{"rollback", func(t *testing.T, tx db.TxRunner) {
			if err := tx.Rollback(ctx); err != nil {
				t.Fatalf("rollback: %v", err)
			}
		}},
		{"cancel", func(t *testing.T, tx db.TxRunner) {
			cctx, cancel := context.WithCancel(ctx)
			cancel() // handler context dies mid-transaction
			_ = tx.Exec(cctx, "SELECT 1")
			// withTx's deferred rollback uses a live (background) context.
			_ = tx.Rollback(context.Background())
		}},
		{"timeout", func(t *testing.T, tx db.TxRunner) {
			tctx, cancel := context.WithTimeout(ctx, 0)
			defer cancel()
			_ = tx.Exec(tctx, "SELECT 1")
			_ = tx.Rollback(context.Background())
		}},
		{"panic", func(t *testing.T, tx db.TxRunner) {
			func() {
				defer func() {
					_ = recover()
					_ = tx.Rollback(context.Background())
				}()
				panic("handler boom")
			}()
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.BeginAuthorizedMutation(ctx, runner, attr)
			if err != nil {
				t.Fatalf("begin authorized mutation: %v", err)
			}

			// Teeth: the prelude must actually set the GUCs inside the tx, or
			// the post-tx "is empty" assertion would be vacuous.
			if got := mustScalar(t, ctx, tx, "SELECT COALESCE(current_setting('striatum.rpc_id', true), '')"); got != "rpc-123" {
				t.Fatalf("rpc_id not set inside tx (prelude did not run?): %q", got)
			}
			if got := mustScalar(t, ctx, tx, "SELECT COALESCE(current_setting('striatum.daemon_auth', true), '')"); got != "secret-value" {
				t.Fatalf("daemon_auth not set inside tx: %q", got)
			}

			tc.end(t, tx)

			for _, guc := range authorityGUCs {
				if got := currentGUC(t, ctx, runner, guc); got != "" {
					t.Errorf("after %s: GUC %s leaked to the next checkout: %q", tc.name, guc, got)
				}
			}
		})
	}
}

func mustScalar(t *testing.T, ctx context.Context, tx db.TxRunner, sql string) string {
	t.Helper()
	v, err := tx.QueryScalar(ctx, sql)
	if err != nil {
		t.Fatalf("scalar %q: %v", sql, err)
	}
	return v
}
