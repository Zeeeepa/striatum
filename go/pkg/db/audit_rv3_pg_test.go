package db_test

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// registerAuthoritySecret records a registry digest for the given secret so
// assert_daemon_authority accepts a transaction whose prelude presented it.
func registerAuthoritySecret(t *testing.T, ctx context.Context, runner db.Runner, secret, salt string) {
	t.Helper()
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_auth_registry(instance_id, role_name, digest, salt)
		VALUES ($1, 'striatumd_rw',
		        encode(striatumd.digest(convert_to($2 || $3, 'UTF8'), 'sha256'), 'hex'), $3)
		ON CONFLICT (instance_id, role_name) DO UPDATE
		   SET digest = EXCLUDED.digest, salt = EXCLUDED.salt, rotated_at = now()`,
		"inst-rv3", secret, salt); err != nil {
		t.Fatalf("register authority secret: %v", err)
	}
}

// writeAuthorizedAudit opens an authorized mutation (prelude presents the
// secret) and appends one audit row through AppendAuditInTx, honoring the active
// hash format.
func writeAuthorizedAudit(t *testing.T, ctx context.Context, runner db.Runner, secret, requestID string) {
	t.Helper()
	attr := db.AuthorityContext{Secret: secret, RPCID: requestID, PrincipalID: "p", SessionID: "s"}
	tx, err := db.BeginAuthorizedMutation(ctx, runner, attr)
	if err != nil {
		t.Fatalf("begin authorized mutation: %v", err)
	}
	meta := db.AuditMeta{
		DaemonVersion: "v", Method: "m", Decision: "allowed", Transport: "rpc",
		RequestID: requestID, ParamsSHA256: "p", OK: true,
	}
	if _, err := db.AppendAuditInTx(ctx, tx, meta); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("append audit (%s): %v", db.AuditHashFormat(), err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func latestAuditFormat(t *testing.T, ctx context.Context, runner db.Runner, requestID string) string {
	t.Helper()
	v, err := runner.QueryScalar(ctx,
		"SELECT hash_format_version::text FROM striatumd.audit_log WHERE request_id = $1", requestID)
	if err != nil {
		t.Fatalf("read audit format: %v", err)
	}
	return v
}

// TestRV3CutoverDispatch is T-ROLLBACK-POSTURE (RFC 0110 §5.2): the
// audit.hash_format flag selects the write path — v2 (default) writes a v2 row
// via the Go path and no v3 row is producible; v3 writes a v3 row via the SD
// function. The flag is the two-way door.
func TestRV3CutoverDispatch(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply owner bundle: %v", err)
	}
	const secret = "rv3-secret"
	registerAuthoritySecret(t, ctx, pool.Runner, secret, "rv3-salt")
	t.Cleanup(func() { db.SetAuthorityRuntime("", db.AuditHashFormatV2, "", false) })

	// Default v2: the Go path writes a v2 row; no v3 row is produced.
	db.SetAuthorityRuntime(secret, db.AuditHashFormatV2, "", false)
	if db.AuditHashFormat() != "v2" {
		t.Fatalf("default format = %q, want v2", db.AuditHashFormat())
	}
	writeAuthorizedAudit(t, ctx, pool.Runner, secret, "req-v2")
	if got := latestAuditFormat(t, ctx, pool.Runner, "req-v2"); got != "2" {
		t.Fatalf("v2 flag wrote hash_format_version=%s; want 2", got)
	}
	if n, _ := pool.Runner.QueryScalar(ctx, "SELECT COUNT(*)::text FROM striatumd.audit_log WHERE hash_format_version = 3"); n != "0" {
		t.Fatalf("v2 flag produced a v3 row (count=%s) — rollback posture violated", n)
	}

	// Flip to v3: the SD function writes a v3 row.
	db.SetAuthorityRuntime(secret, db.AuditHashFormatV3, "", false)
	writeAuthorizedAudit(t, ctx, pool.Runner, secret, "req-v3")
	if got := latestAuditFormat(t, ctx, pool.Runner, "req-v3"); got != "3" {
		t.Fatalf("v3 flag wrote hash_format_version=%s; want 3", got)
	}
}
