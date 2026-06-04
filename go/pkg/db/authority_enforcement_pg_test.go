package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/jackc/pgx/v5/pgconn"
)

// enforcementDB applies the production owner bundle to a migrated database and
// returns the owner pool + ctx. The runtime role striatumd_rw is cluster-wide
// and not administrable by the (non-superuser) test owner, so enforcement is
// asserted via has_table_privilege (PostgreSQL's own privilege oracle) and via
// the role-agnostic authority gate — not by SET ROLE.
func enforcementDB(t *testing.T) (*db.Pool, context.Context) {
	t.Helper()
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply owner bundle: %v", err)
	}
	return pool, ctx
}

func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func scalar(t *testing.T, ctx context.Context, runner db.Runner, sql string, args ...any) string {
	t.Helper()
	v, err := runner.QueryScalar(ctx, sql, args...)
	if err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	return v
}

func hasInsert(t *testing.T, ctx context.Context, runner db.Runner, table string) bool {
	t.Helper()
	return scalar(t, ctx, runner,
		"SELECT has_table_privilege('striatumd_rw', 'striatumd.'||$1, 'INSERT')::text", table) == "true"
}

// TestParityReadGrant is the runtime-role read gate for the capability-parity
// check (RFC 0110 §8.2, owner bundle 0002). db.VerifyCapabilityParity reads
// striatumd.schema_authority AS THE RUNTIME ROLE at startup, so the runtime role
// must hold SELECT on it once any bundle is applied; otherwise the daemon fails
// parity with 42501 and crash-loops under Restart=on-failure. Bundle 0001
// created schema_authority owner-only and omitted the read grant; the slice-1
// parity tests ran as the PEER owner and could not catch the runtime-role gap.
// The read grant must not leak write privilege (schema_authority stays
// write-owner-only, RFC 0110 §13).
func TestParityReadGrant(t *testing.T) {
	pool, ctx := enforcementDB(t)
	if scalar(t, ctx, pool.Runner,
		"SELECT has_table_privilege('striatumd_rw', 'striatumd.schema_authority', 'SELECT')::text") != "true" {
		t.Fatal("striatumd_rw cannot SELECT schema_authority; the startup capability-parity check would fail 42501 (owner bundle must GRANT SELECT)")
	}
	if scalar(t, ctx, pool.Runner,
		"SELECT has_table_privilege('striatumd_rw', 'striatumd.schema_authority', 'INSERT')::text") != "false" {
		t.Fatal("striatumd_rw must not hold INSERT on schema_authority; write stays owner-only")
	}
}

// TestPhase0DirectInsertDenied is T-42501-P0 + T-GRANT-DRIFT: after the runtime
// role is provisioned broad DML (as daemon doctor does), the owner bundle's
// REVOKE removes audit_log INSERT (only) — so a direct write is denied while
// unprotected surfaces keep their grant; and a stray re-grant is undone by
// ReassertWriteRevokes.
func TestPhase0DirectInsertDenied(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()

	// Provision broad DML first (mirrors daemon doctor --provision-rw-role), so
	// the post-bundle denial proves the bundle's REVOKE, not an absent grant.
	if err := pool.Runner.Exec(ctx, "GRANT USAGE ON SCHEMA striatumd TO striatumd_rw"); err != nil {
		t.Fatalf("provision usage: %v", err)
	}
	if err := pool.Runner.Exec(ctx, "GRANT INSERT ON ALL TABLES IN SCHEMA striatumd TO striatumd_rw"); err != nil {
		t.Fatalf("provision insert: %v", err)
	}
	if !hasInsert(t, ctx, pool.Runner, "audit_log") {
		t.Fatal("precondition: striatumd_rw should hold audit_log INSERT before the bundle")
	}

	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply owner bundle: %v", err)
	}

	if hasInsert(t, ctx, pool.Runner, "audit_log") {
		t.Fatal("T-42501-P0: bundle did not revoke striatumd_rw audit_log INSERT")
	}
	if !hasInsert(t, ctx, pool.Runner, "events") {
		t.Fatal("events INSERT must stay untouched until P2 (false-green guard)")
	}

	// T-GRANT-DRIFT: a stray re-grant reopens the hole; reassert closes it.
	if err := pool.Runner.Exec(ctx, "GRANT INSERT ON striatumd.audit_log TO striatumd_rw"); err != nil {
		t.Fatalf("simulate grant drift: %v", err)
	}
	if !hasInsert(t, ctx, pool.Runner, "audit_log") {
		t.Fatal("drift setup failed: re-grant did not take")
	}
	if err := db.ReassertWriteRevokes(ctx, pool.Runner); err != nil {
		t.Fatalf("reassert revokes: %v", err)
	}
	if hasInsert(t, ctx, pool.Runner, "audit_log") {
		t.Fatal("T-GRANT-DRIFT: ReassertWriteRevokes did not re-close audit_log INSERT")
	}
}

// TestExecAuthorityGate is T-EXEC-AUTH: append_audit_row with no/wrong daemon
// authority secret raises SQLSTATE 28000 and mutates zero rows; with the correct
// registered secret it succeeds and writes a v3 row. The gate is role-agnostic
// (the secret is authority), so it is exercised as the owner here.
func TestExecAuthorityGate(t *testing.T) {
	pool, ctx := enforcementDB(t)
	const secret = "s3cr3t-authority"
	const salt = "per-instance-salt"
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_auth_registry(instance_id, role_name, digest, salt)
		VALUES ('inst-test', 'striatumd_rw',
		        encode(striatumd.digest(convert_to($1 || $2, 'UTF8'), 'sha256'), 'hex'), $2)`,
		secret, salt); err != nil {
		t.Fatalf("register secret: %v", err)
	}

	conn, err := pool.RawPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()

	before := scalar(t, ctx, pool.Runner, "SELECT COUNT(*)::text FROM striatumd.audit_log")

	const call = `SELECT striatumd.append_audit_row('v','c','r','m','allowed',NULL,'rpc','q',true,'p')`

	// No secret -> 28000, zero rows.
	_, err = conn.Exec(ctx, call)
	if code := pgErrCode(err); code != "28000" {
		t.Fatalf("append without secret: got %v (code %q); want 28000", err, code)
	}
	if got := scalar(t, ctx, pool.Runner, "SELECT COUNT(*)::text FROM striatumd.audit_log"); got != before {
		t.Fatalf("failed authority append mutated rows: before=%s after=%s", before, got)
	}

	// Wrong secret -> 28000.
	if _, err := conn.Exec(ctx, "SELECT set_config('striatum.daemon_auth', 'wrong', false)"); err != nil {
		t.Fatalf("set wrong secret: %v", err)
	}
	if code := pgErrCode(mustExecErr(conn, ctx, call)); code != "28000" {
		t.Fatalf("append with wrong secret: want 28000")
	}

	// Correct secret -> succeeds, writes a v3 row.
	if _, err := conn.Exec(ctx, "SELECT set_config('striatum.daemon_auth', $1, false)", secret); err != nil {
		t.Fatalf("set correct secret: %v", err)
	}
	if _, err := conn.Exec(ctx, call); err != nil {
		t.Fatalf("authorized append failed: %v", err)
	}
	format := scalar(t, ctx, pool.Runner,
		"SELECT hash_format_version::text FROM striatumd.audit_log ORDER BY audit_id DESC LIMIT 1")
	if format != "3" {
		t.Fatalf("authorized append wrote hash_format_version=%s; want 3", format)
	}
}

func mustExecErr(conn interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, ctx context.Context, sql string) error {
	_, err := conn.Exec(ctx, sql)
	return err
}

// TestPhase1ArtifactInsertDenied is T-42501-P1 + T-GRANT-DRIFT for the artifacts
// surface: owner bundle 0003 revokes striatumd_rw's direct artifacts INSERT (only),
// grants EXECUTE on the SD append function, and a stray re-grant is undone by
// ReassertWriteRevokes — while events (closed only at P2) keeps its INSERT.
func TestPhase1ArtifactInsertDenied(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()

	if err := pool.Runner.Exec(ctx, "GRANT USAGE ON SCHEMA striatumd TO striatumd_rw"); err != nil {
		t.Fatalf("provision usage: %v", err)
	}
	if err := pool.Runner.Exec(ctx, "GRANT INSERT ON ALL TABLES IN SCHEMA striatumd TO striatumd_rw"); err != nil {
		t.Fatalf("provision insert: %v", err)
	}
	if !hasInsert(t, ctx, pool.Runner, "artifacts") {
		t.Fatal("precondition: striatumd_rw should hold artifacts INSERT before the bundle")
	}

	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply owner bundle: %v", err)
	}

	if hasInsert(t, ctx, pool.Runner, "artifacts") {
		t.Fatal("T-42501-P1: bundle 0003 did not revoke striatumd_rw artifacts INSERT")
	}
	if !hasInsert(t, ctx, pool.Runner, "events") {
		t.Fatal("events INSERT must stay untouched until P2 (false-green guard)")
	}
	// The runtime role must hold EXECUTE on the sanctioned SD write path.
	if scalar(t, ctx, pool.Runner,
		"SELECT has_function_privilege('striatumd_rw', p.oid, 'EXECUTE')::text FROM pg_proc p WHERE proname='append_artifact_row' AND pronamespace='striatumd'::regnamespace ORDER BY oid DESC LIMIT 1") != "true" {
		t.Fatal("striatumd_rw lacks EXECUTE on append_artifact_row; P1 publishes would fail")
	}

	// T-GRANT-DRIFT: a stray re-grant reopens the hole; reassert closes it.
	if err := pool.Runner.Exec(ctx, "GRANT INSERT ON striatumd.artifacts TO striatumd_rw"); err != nil {
		t.Fatalf("simulate grant drift: %v", err)
	}
	if !hasInsert(t, ctx, pool.Runner, "artifacts") {
		t.Fatal("drift setup failed: re-grant did not take")
	}
	if err := db.ReassertWriteRevokes(ctx, pool.Runner); err != nil {
		t.Fatalf("reassert revokes: %v", err)
	}
	if hasInsert(t, ctx, pool.Runner, "artifacts") {
		t.Fatal("T-GRANT-DRIFT: ReassertWriteRevokes did not re-close artifacts INSERT")
	}
	// ReassertWriteRevokes must not over-reach: events (no stamp) keeps its grant.
	if !hasInsert(t, ctx, pool.Runner, "events") {
		t.Fatal("ReassertWriteRevokes over-revoked events (only stamped surfaces should close)")
	}
}

// TestArtifactExecAuthorityGate is T-EXEC-AUTH for the artifacts surface:
// append_artifact_row with no/wrong daemon-authority secret raises SQLSTATE 28000
// before the INSERT and mutates zero rows; with the correct secret the call gets
// PAST the authority gate to the INSERT (which then trips a foreign-key violation,
// 23503, because no parent repository/run is seeded — proving authority passed,
// not that the write was blocked).
func TestArtifactExecAuthorityGate(t *testing.T) {
	pool, ctx := enforcementDB(t)
	const secret = "s3cr3t-artifact-authority"
	const salt = "per-instance-salt"
	if err := pool.Runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_auth_registry(instance_id, role_name, digest, salt)
		VALUES ('inst-test', 'striatumd_rw',
		        encode(striatumd.digest(convert_to($1 || $2, 'UTF8'), 'sha256'), 'hex'), $2)`,
		secret, salt); err != nil {
		t.Fatalf("register secret: %v", err)
	}

	conn, err := pool.RawPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()

	before := scalar(t, ctx, pool.Runner, "SELECT COUNT(*)::text FROM striatumd.artifacts")
	const call = `SELECT striatumd.append_artifact_row('repo','art','run',NULL,NULL,'log','finding','p/f.md','sha',10,'create',now(),NULL,NULL,NULL,NULL,1)`

	// No secret -> 28000, zero rows.
	if code := pgErrCode(mustExecErr(conn, ctx, call)); code != "28000" {
		t.Fatalf("append without secret: want 28000, got %q", code)
	}
	if got := scalar(t, ctx, pool.Runner, "SELECT COUNT(*)::text FROM striatumd.artifacts"); got != before {
		t.Fatalf("failed authority append mutated rows: before=%s after=%s", before, got)
	}

	// Wrong secret -> 28000.
	if _, err := conn.Exec(ctx, "SELECT set_config('striatum.daemon_auth', 'wrong', false)"); err != nil {
		t.Fatalf("set wrong secret: %v", err)
	}
	if code := pgErrCode(mustExecErr(conn, ctx, call)); code != "28000" {
		t.Fatalf("append with wrong secret: want 28000, got %q", code)
	}

	// Correct secret -> authority passes, INSERT reached: FK violation (23503),
	// not an authority denial. Zero artifact rows still land.
	if _, err := conn.Exec(ctx, "SELECT set_config('striatum.daemon_auth', $1, false)", secret); err != nil {
		t.Fatalf("set correct secret: %v", err)
	}
	if code := pgErrCode(mustExecErr(conn, ctx, call)); code != "23503" {
		t.Fatalf("authorized append: want 23503 (FK violation past the authority gate), got %q", code)
	}
	if got := scalar(t, ctx, pool.Runner, "SELECT COUNT(*)::text FROM striatumd.artifacts"); got != before {
		t.Fatalf("authority gate let a partial row land: before=%s after=%s", before, got)
	}
}

// TestRegistryACL is T-REGISTRY-ACL: the runtime role holds no SELECT on the
// owner-only authority registry.
func TestRegistryACL(t *testing.T) {
	pool, ctx := enforcementDB(t)
	if scalar(t, ctx, pool.Runner,
		"SELECT has_table_privilege('striatumd_rw', 'striatumd.daemon_auth_registry', 'SELECT')::text") != "false" {
		t.Fatal("striatumd_rw can SELECT daemon_auth_registry; must be owner-only")
	}
}

// TestSDHardening is T-SD-HARDEN: the owner-owned write functions are SECURITY
// DEFINER with a pinned search_path and no PUBLIC execute.
func TestSDHardening(t *testing.T) {
	pool, ctx := enforcementDB(t)
	for _, fn := range []string{"assert_daemon_authority", "append_audit_row", "append_artifact_row"} {
		if scalar(t, ctx, pool.Runner,
			"SELECT prosecdef::text FROM pg_proc WHERE proname=$1 AND pronamespace='striatumd'::regnamespace ORDER BY oid DESC LIMIT 1", fn) != "true" {
			t.Errorf("%s is not SECURITY DEFINER", fn)
		}
		cfg := scalar(t, ctx, pool.Runner,
			"SELECT COALESCE(array_to_string(proconfig, ','), '') FROM pg_proc WHERE proname=$1 AND pronamespace='striatumd'::regnamespace ORDER BY oid DESC LIMIT 1", fn)
		if !containsAll(cfg, "search_path=", "striatumd", "pg_temp") {
			t.Errorf("%s search_path not pinned (proconfig=%q)", fn, cfg)
		}
		if scalar(t, ctx, pool.Runner,
			"SELECT has_function_privilege('public', oid, 'EXECUTE')::text FROM pg_proc WHERE proname=$1 AND pronamespace='striatumd'::regnamespace ORDER BY oid DESC LIMIT 1", fn) != "false" {
			t.Errorf("%s is executable by PUBLIC", fn)
		}
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
