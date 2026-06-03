package mutations

import (
	"context"
	"errors"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// auditTestCtx threads the dispatch/envelope/auth a mutating RPC carries, so
// withTx's mutation-coupled audit append (RFC 0110 §4.4) is active.
func auditTestCtx() (context.Context, *rpc.AuditDispatch) {
	dispatch := &rpc.AuditDispatch{DaemonVersion: "test", Transport: "rpc"}
	ctx := rpc.WithAuditDispatch(
		rpc.WithEnvelope(
			rpc.WithAuthContext(
				context.Background(),
				rpc.AuthContext{Decision: "allowed", ClientID: "client-1", RepositoryID: "repo-1"},
			),
			rpc.Envelope{Method: "test.mutate", RequestID: "req-1", Params: map[string]any{"k": "v"}},
		),
		dispatch,
	)
	return ctx, dispatch
}

func createProbeTable(t *testing.T, runner db.Runner) {
	t.Helper()
	if err := runner.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS striatumd.mutation_probe(id text)"); err != nil {
		t.Fatalf("create probe table: %v", err)
	}
}

func probeCount(t *testing.T, runner db.Runner) string {
	t.Helper()
	n, err := runner.QueryScalar(context.Background(), "SELECT COUNT(*)::text FROM striatumd.mutation_probe")
	if err != nil {
		t.Fatalf("count probe rows: %v", err)
	}
	return n
}

// TestAuditFailClosed is T-AUDIT-FAILCLOSED (RFC 0110 §4.4, C-AUDIT-AUTH-PRELUDE).
func TestAuditFailClosed(t *testing.T) {
	t.Run("happy_path_couples_audit_to_mutation", func(t *testing.T) {
		pool := pgtest.Pool(t)
		createProbeTable(t, pool.Runner)
		ctx, dispatch := auditTestCtx()

		_, err := withTx(ctx, pool.Runner, func(tx db.TxRunner) (map[string]any, error) {
			return nil, tx.Exec(ctx, "INSERT INTO striatumd.mutation_probe(id) VALUES ($1)", "row-1")
		})
		if err != nil {
			t.Fatalf("withTx: %v", err)
		}
		if !dispatch.Appended || dispatch.AuditID == "" {
			t.Fatalf("mutation did not self-audit: appended=%v id=%q", dispatch.Appended, dispatch.AuditID)
		}
		if got := probeCount(t, pool.Runner); got != "1" {
			t.Fatalf("mutation row not committed: probe count=%s", got)
		}
		auditID, err := pool.Runner.QueryScalar(context.Background(),
			"SELECT audit_id::text FROM striatumd.audit_log WHERE request_id = $1", "req-1")
		if err != nil {
			t.Fatalf("read audit row: %v", err)
		}
		if auditID == "" {
			t.Fatal("no audit row written for the mutation inside its transaction")
		}
		if auditID != dispatch.AuditID {
			t.Fatalf("dispatch audit id %q != persisted audit id %q", dispatch.AuditID, auditID)
		}
	})

	t.Run("in_tx_append_failure_rolls_back_mutation", func(t *testing.T) {
		pool := pgtest.Pool(t)
		createProbeTable(t, pool.Runner)
		// Force every audit append to fail by removing the chain head.
		if err := pool.Runner.Exec(context.Background(), "DROP TABLE striatumd.audit_chain_head"); err != nil {
			t.Fatalf("break audit chain: %v", err)
		}
		ctx, dispatch := auditTestCtx()

		_, err := withTx(ctx, pool.Runner, func(tx db.TxRunner) (map[string]any, error) {
			return nil, tx.Exec(ctx, "INSERT INTO striatumd.mutation_probe(id) VALUES ($1)", "row-2")
		})
		if err == nil {
			t.Fatal("mutation succeeded despite a failed audit append (not fail-closed)")
		}
		var rpcErr *rpc.Error
		if !errors.As(err, &rpcErr) || rpcErr.Code != "audit_append_failed" {
			t.Fatalf("expected audit_append_failed, got %v", err)
		}
		if dispatch.Appended {
			t.Fatal("dispatch marked appended despite a failed append")
		}
		if got := probeCount(t, pool.Runner); got != "0" {
			t.Fatalf("mutation row persisted despite the failed audit append (no rollback): probe count=%s", got)
		}
	})

	t.Run("standalone_append_failure_errors", func(t *testing.T) {
		pool := pgtest.Pool(t)
		if err := pool.Runner.Exec(context.Background(), "DROP TABLE striatumd.audit_chain_head"); err != nil {
			t.Fatalf("break audit chain: %v", err)
		}
		recorder := db.AuditRecorder{Runner: pool.Runner, DaemonVersion: "test"}
		_, err := recorder.RecordRPC(
			context.Background(),
			rpc.Envelope{Method: "list.runs", RequestID: "req-read", Params: map[string]any{}},
			rpc.AuthContext{Decision: "allowed"},
			rpc.Response{OK: true},
		)
		if err == nil {
			t.Fatal("standalone audit append succeeded despite a broken chain (not fail-closed)")
		}
	})
}
