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

func quoteIdentForTest(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
