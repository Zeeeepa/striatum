package db_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// tryWriteAuthorizedAudit runs an authorized mutation + audit append, returning
// any error (the non-fatal sibling of writeAuthorizedAudit).
func tryWriteAuthorizedAudit(ctx context.Context, runner db.Runner, secret, requestID string) error {
	attr := db.AuthorityContext{Secret: secret, RPCID: requestID, PrincipalID: "p", SessionID: "s"}
	tx, err := db.BeginAuthorizedMutation(ctx, runner, attr)
	if err != nil {
		return err
	}
	meta := db.AuditMeta{
		DaemonVersion: "v", Method: "m", Decision: "allowed", Transport: "rpc",
		RequestID: requestID, ParamsSHA256: "p", OK: true,
	}
	if _, err := db.AppendAuditInTx(ctx, tx, meta); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// TestAuthLiveness is T-AUTH-LIVENESS (RFC 0110 §4.5): authority is
// lifetime-of-instance — a registered secret keeps working with no TTL even as
// rotated_at ages; a deleted/superseded registry row makes the next authorized
// write fail as daemon_auth_lost with zero rows landed.
func TestAuthLiveness(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply bundle: %v", err)
	}
	const secret = "liveness-secret"
	registerAuthoritySecret(t, ctx, pool.Runner, secret, "liveness-salt")
	db.SetAuthorityRuntime(secret, db.AuditHashFormatV3, "", false)
	t.Cleanup(func() { db.SetAuthorityRuntime("", db.AuditHashFormatV2, "", false) })

	// Authorized v3 write succeeds.
	if err := tryWriteAuthorizedAudit(ctx, pool.Runner, secret, "live-1"); err != nil {
		t.Fatalf("authorized v3 write should succeed: %v", err)
	}

	// Aging rotated_at far into the past must not break authority (no TTL).
	if err := pool.Runner.Exec(ctx,
		"UPDATE striatumd.daemon_auth_registry SET rotated_at = now() - interval '400 days'"); err != nil {
		t.Fatalf("age rotated_at: %v", err)
	}
	if err := tryWriteAuthorizedAudit(ctx, pool.Runner, secret, "live-2"); err != nil {
		t.Fatalf("aged authority must still write (no TTL): %v", err)
	}

	before := scalar(t, ctx, pool.Runner, "SELECT COUNT(*)::text FROM striatumd.audit_log")

	// Delete the registry row: the next authorized write fails as daemon_auth_lost.
	if err := pool.Runner.Exec(ctx, "DELETE FROM striatumd.daemon_auth_registry"); err != nil {
		t.Fatalf("delete registry: %v", err)
	}
	err := tryWriteAuthorizedAudit(ctx, pool.Runner, secret, "live-3")
	if err == nil {
		t.Fatal("write after registry deletion must fail (daemon_auth_lost)")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "daemon_auth_lost" {
		t.Fatalf("expected daemon_auth_lost, got %v", err)
	}
	if after := scalar(t, ctx, pool.Runner, "SELECT COUNT(*)::text FROM striatumd.audit_log"); after != before {
		t.Fatalf("a lost-authority write mutated rows: before=%s after=%s", before, after)
	}
}

// TestPreludeObserverHidesSecret is T-PRELUDE-OBSERVER (RFC 0110 §4.2): while an
// authorized transaction holds the daemon-authority secret, a second session
// reading pg_stat_activity never sees the secret value — the prelude rides the
// extended protocol so the secret travels as a bind parameter, shown as $1.
func TestPreludeObserverHidesSecret(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	const secret = "observer-secret-must-not-leak-9f3a"

	conn, err := pool.RawPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire holder conn: %v", err)
	}
	defer conn.Release()

	var pid int
	if err := conn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
		t.Fatalf("backend pid: %v", err)
	}

	// The prelude is the holder backend's last statement (extended protocol).
	tx, err := db.BeginAuthorizedMutation(ctx, singleConnRunner{conn: conn},
		db.AuthorityContext{Secret: secret, RPCID: "r", PrincipalID: "p", SessionID: "s"})
	if err != nil {
		t.Fatalf("begin authorized mutation: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// A second session observes the holder backend's query text.
	observed, err := pool.Runner.QueryScalar(ctx,
		"SELECT COALESCE(query, '') FROM pg_stat_activity WHERE pid = $1", pid)
	if err != nil {
		t.Fatalf("observe pg_stat_activity: %v", err)
	}
	if strings.Contains(observed, secret) {
		t.Fatalf("daemon authority secret leaked into pg_stat_activity.query: %q", observed)
	}
	if !strings.Contains(observed, "set_config('striatum.daemon_auth'") {
		t.Fatalf("did not capture the prelude in pg_stat_activity (got %q); test would be vacuous", observed)
	}
}
