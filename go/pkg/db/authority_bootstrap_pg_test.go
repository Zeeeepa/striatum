package db_test

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

func dbNameFromURL(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse pool URL: %v", err)
	}
	return strings.TrimPrefix(u.Path, "/")
}

// TestBootstrapInertWithoutSchema: no owner bundle applied ⇒ BootstrapAuthority
// is inert (schema_absent, no secret) so a daemon without the bundle is
// unaffected.
func TestBootstrapInertWithoutSchema(t *testing.T) {
	pool := pgtest.Pool(t)
	res, err := db.BootstrapAuthority(context.Background(), db.BootstrapConfig{
		RuntimeURL: pool.URL, RuntimeRole: "halbritt", InstanceID: "i1", DaemonVersion: "test",
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if res.Posture != db.PostureSchemaAbsent || res.Secret != "" || res.Registered {
		t.Fatalf("inert bootstrap = %+v; want schema_absent/no secret", res)
	}
}

// TestBootstrapSingleRoleRegistersSecret: with the bundle applied and no owner
// URL, rotation is skipped (single-role posture) but the authority secret is
// generated and registered.
func TestBootstrapSingleRoleRegistersSecret(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply bundle: %v", err)
	}
	res, err := db.BootstrapAuthority(ctx, db.BootstrapConfig{
		RuntimeURL: pool.URL, RuntimeRole: "halbritt", InstanceID: "i1", DaemonVersion: "test",
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if res.Posture != db.PostureSingleRoleSkip || res.Secret == "" || !res.Registered || res.NewRuntimeURL != "" {
		t.Fatalf("single-role bootstrap = %+v; want rotation_skipped + secret + no new URL", res)
	}
	n := scalar(t, ctx, pool.Runner, "SELECT COUNT(*)::text FROM striatumd.daemon_auth_registry WHERE instance_id = 'i1'")
	if n != "1" {
		t.Fatalf("registry rows for i1 = %s; want 1", n)
	}
}

// TestBootstrapOwnerFailClosed: an unreachable owner DSN fails closed (§9.2).
func TestBootstrapOwnerFailClosed(t *testing.T) {
	pool := pgtest.Pool(t)
	_, err := db.BootstrapAuthority(context.Background(), db.BootstrapConfig{
		RuntimeURL:  pool.URL,
		OwnerURL:    "postgres://nobody:nopw@127.0.0.1:5599/does_not_exist?connect_timeout=2",
		RuntimeRole: "striatumd_rw", InstanceID: "i1", DaemonVersion: "test",
	})
	if err == nil {
		t.Fatal("unreachable owner DSN must fail closed")
	}
	if !strings.Contains(err.Error(), "daemon_pg_owner_bootstrap_failed") {
		t.Fatalf("error must be owner-attributable, got: %v", err)
	}
}

// TestBootstrapRotatesPasswordTwoRole: two-role bootstrap rotates the runtime
// password over the owner connection and returns a runtime URL that
// authenticates with the new password while the old one no longer works.
func TestBootstrapRotatesPasswordTwoRole(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply bundle: %v", err)
	}
	dbName := dbNameFromURL(t, pool.URL)
	role := "rot_" + dbName
	if len(role) > 63 {
		role = role[:63]
	}
	if err := pool.Runner.Exec(ctx, fmt.Sprintf("DROP ROLE IF EXISTS %s", quoteIdentForTest(role))); err != nil {
		t.Fatalf("drop role: %v", err)
	}
	if err := pool.Runner.Exec(ctx, fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD 'oldpw'", quoteIdentForTest(role))); err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := pool.Runner.Exec(ctx, fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", quoteIdentForTest(dbName), quoteIdentForTest(role))); err != nil {
		t.Fatalf("grant connect: %v", err)
	}
	t.Cleanup(func() {
		_ = pool.Runner.Exec(context.Background(), fmt.Sprintf("DROP ROLE IF EXISTS %s", quoteIdentForTest(role)))
	})

	oldURL := fmt.Sprintf("postgres://%s:oldpw@localhost:5432/%s", role, dbName)
	// Sanity: the old password authenticates over TCP before rotation.
	if p, err := db.Connect(ctx, oldURL, "test"); err != nil {
		t.Fatalf("precondition: old password should connect: %v", err)
	} else {
		p.Close()
	}

	res, err := db.BootstrapAuthority(ctx, db.BootstrapConfig{
		RuntimeURL:  oldURL,
		OwnerURL:    pool.URL, // owner over the PEER socket
		RuntimeRole: role, InstanceID: "i-two-role", DaemonVersion: "test",
	})
	if err != nil {
		t.Fatalf("two-role bootstrap: %v", err)
	}
	if res.Posture != db.PostureRotated || res.NewRuntimeURL == "" || res.Secret == "" {
		t.Fatalf("two-role bootstrap = %+v; want rotated + new URL + secret", res)
	}

	// The rotated URL authenticates; the old password no longer does.
	if p, err := db.Connect(ctx, res.NewRuntimeURL, "test"); err != nil {
		t.Fatalf("rotated password should connect: %v", err)
	} else {
		p.Close()
	}
	if p, err := db.Connect(ctx, oldURL, "test"); err == nil {
		p.Close()
		t.Fatal("old password still authenticates after rotation")
	}
}

// TestBootstrapRotatorScope is T-ROTATOR-SCOPE (§9.4): a second instance sharing
// the same role trips the probe; a distinct (per-instance) role does not.
func TestBootstrapRotatorScope(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply bundle: %v", err)
	}
	// Instance A registers role "shared".
	if _, err := db.BootstrapAuthority(ctx, db.BootstrapConfig{
		RuntimeURL: pool.URL, RuntimeRole: "shared", InstanceID: "A", DaemonVersion: "test",
	}); err != nil {
		t.Fatalf("bootstrap A: %v", err)
	}
	// Instance B, same role ⇒ collision.
	resB, err := db.BootstrapAuthority(ctx, db.BootstrapConfig{
		RuntimeURL: pool.URL, RuntimeRole: "shared", InstanceID: "B", DaemonVersion: "test",
	})
	if err != nil {
		t.Fatalf("bootstrap B: %v", err)
	}
	if !resB.RotatorCollision {
		t.Fatal("same role + different instance must trip the rotator probe")
	}
	// Instance C, per-instance role ⇒ no collision.
	resC, err := db.BootstrapAuthority(ctx, db.BootstrapConfig{
		RuntimeURL: pool.URL, RuntimeRole: "striatumd_rw_C", InstanceID: "C", DaemonVersion: "test",
	})
	if err != nil {
		t.Fatalf("bootstrap C: %v", err)
	}
	if resC.RotatorCollision {
		t.Fatal("a per-instance role must not trip the rotator probe")
	}
}

// TestBootstrapTwoRoleWithoutOwnerURLActionable: a two-role deployment (runtime
// role is not the owner) with no STRIATUM_OWNER_DB_URL fails closed when it
// cannot write the owner-only registry, with an actionable error naming the
// owner DSN (not a bare permission-denied).
func TestBootstrapTwoRoleWithoutOwnerURLActionable(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply bundle: %v", err)
	}
	dbName := dbNameFromURL(t, pool.URL)
	role := "norol_" + dbName
	if len(role) > 63 {
		role = role[:63]
	}
	if err := pool.Runner.Exec(ctx, fmt.Sprintf("DROP ROLE IF EXISTS %s", quoteIdentForTest(role))); err != nil {
		t.Fatalf("drop role: %v", err)
	}
	if err := pool.Runner.Exec(ctx, fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD 'pw'", quoteIdentForTest(role))); err != nil {
		t.Fatalf("create role: %v", err)
	}
	// Grant CONNECT + schema USAGE (so the registry is visible) but not registry
	// INSERT — the owner-only posture a non-owner runtime role has.
	for _, stmt := range []string{
		fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", quoteIdentForTest(dbName), quoteIdentForTest(role)),
		fmt.Sprintf("GRANT USAGE ON SCHEMA striatumd TO %s", quoteIdentForTest(role)),
	} {
		if err := pool.Runner.Exec(ctx, stmt); err != nil {
			t.Fatalf("grant: %v", err)
		}
	}
	t.Cleanup(func() {
		_ = pool.Runner.Exec(context.Background(), fmt.Sprintf("DROP ROLE IF EXISTS %s", quoteIdentForTest(role)))
	})

	runtimeURL := fmt.Sprintf("postgres://%s:pw@localhost:5432/%s", role, dbName)
	_, err := db.BootstrapAuthority(ctx, db.BootstrapConfig{
		RuntimeURL: runtimeURL, OwnerURL: "", RuntimeRole: role, InstanceID: "i-2role", DaemonVersion: "test",
	})
	if err == nil {
		t.Fatal("two-role bootstrap without owner URL must fail closed")
	}
	if !strings.Contains(err.Error(), "STRIATUM_OWNER_DB_URL") {
		t.Fatalf("error must name STRIATUM_OWNER_DB_URL, got: %v", err)
	}
}

// TestBootstrapAndConnectRecoversFromStaleRuntimePassword is the boot-#2
// regression gate for the RFC 0110 §9.1 restart-safe order. After a two-role
// rotation the live runtime password no longer matches the password captured in
// daemon.toml (rotation is RAM-only and is never written back; §9.1: "A DSN
// captured before a restart fails after it"). A naive boot that connects the
// runtime pool from daemon.toml *before* rotating would therefore authenticate
// with a now-stale password and fail on every restart after the first — a
// crash-loop brick under systemd Restart=on-failure. BootstrapAndConnect rotates
// over the owner connection FIRST, so it recovers even when cfg.RuntimeURL still
// carries the stale password.
func TestBootstrapAndConnectRecoversFromStaleRuntimePassword(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()
	if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
		t.Fatalf("apply bundle: %v", err)
	}
	dbName := dbNameFromURL(t, pool.URL)
	role := "boot2_" + dbName
	if len(role) > 63 {
		role = role[:63]
	}
	if err := pool.Runner.Exec(ctx, fmt.Sprintf("DROP ROLE IF EXISTS %s", quoteIdentForTest(role))); err != nil {
		t.Fatalf("drop role: %v", err)
	}
	if err := pool.Runner.Exec(ctx, fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD 'oldpw'", quoteIdentForTest(role))); err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := pool.Runner.Exec(ctx, fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", quoteIdentForTest(dbName), quoteIdentForTest(role))); err != nil {
		t.Fatalf("grant connect: %v", err)
	}
	t.Cleanup(func() {
		_ = pool.Runner.Exec(context.Background(), fmt.Sprintf("DROP ROLE IF EXISTS %s", quoteIdentForTest(role)))
	})

	// cfg mirrors the daemon's startup config: RuntimeURL carries the password
	// from daemon.toml ("oldpw"); OwnerURL is the owner PEER connection. The same
	// cfg is reused across boots — daemon.toml is never rewritten.
	cfg := db.BootstrapConfig{
		RuntimeURL:    fmt.Sprintf("postgres://%s:oldpw@localhost:5432/%s", role, dbName),
		OwnerURL:      pool.URL,
		RuntimeRole:   role,
		InstanceID:    "i-boot2",
		DaemonVersion: "test",
	}

	// Boot #1: rotates "oldpw" away and connects with the fresh password.
	boot1, err := db.BootstrapAndConnect(ctx, cfg, false)
	if err != nil {
		t.Fatalf("boot #1: %v", err)
	}
	if boot1.Authority.Posture != db.PostureRotated {
		t.Fatalf("boot #1 posture = %q; want rotated", boot1.Authority.Posture)
	}
	if got, err := boot1.Pool.Runner.QueryScalar(ctx, "SELECT 1::text"); err != nil || got != "1" {
		t.Fatalf("boot #1 pool unusable: got=%q err=%v", got, err)
	}
	boot1.Pool.Close()

	// Precondition that makes this regression meaningful: the daemon.toml
	// password is now stale, so a *direct* runtime connect (the naive
	// connect-then-rotate order) fails — that is exactly the brick the fix avoids.
	if p, err := db.Connect(ctx, cfg.RuntimeURL, "test"); err == nil {
		p.Close()
		t.Fatal("precondition: the stale daemon.toml password should no longer authenticate")
	}

	// Boot #2 (the regression): BootstrapAndConnect must still come up, because it
	// rotates over the owner connection before touching the runtime pool.
	boot2, err := db.BootstrapAndConnect(ctx, cfg, false)
	if err != nil {
		t.Fatalf("boot #2 must recover from the stale runtime password: %v", err)
	}
	if boot2.Authority.Posture != db.PostureRotated {
		t.Fatalf("boot #2 posture = %q; want rotated", boot2.Authority.Posture)
	}
	if got, err := boot2.Pool.Runner.QueryScalar(ctx, "SELECT 1::text"); err != nil || got != "1" {
		t.Fatalf("boot #2 pool unusable: got=%q err=%v", got, err)
	}
	boot2.Pool.Close()
}

func quoteIdentForTest(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
