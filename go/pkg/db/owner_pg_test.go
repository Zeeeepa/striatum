package db_test

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestOwnerBundleAppliesAndIsIdempotent applies the production owner bundle SQL
// against a migrated database and asserts: version goes 0 -> 1, the objects
// exist, re-apply is a no-op, and the capability stamp the parity checker reads
// is present (RFC 0110 §8.1).
func TestOwnerBundleAppliesAndIsIdempotent(t *testing.T) {
	pool := pgtest.Pool(t)
	ctx := context.Background()

	if v, err := db.OwnerBundleVersion(ctx, pool.Runner); err != nil || v != 0 {
		t.Fatalf("pre-apply version = %d, err = %v; want 0", v, err)
	}

	applied, version, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test")
	if err != nil {
		t.Fatalf("apply owner bundles: %v", err)
	}
	if len(applied) != 1 || applied[0] != 1 || version != 1 {
		t.Fatalf("apply result applied=%v version=%d; want [1], 1", applied, version)
	}

	// Re-apply is idempotent: nothing applied, version unchanged.
	applied2, version2, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test")
	if err != nil {
		t.Fatalf("re-apply owner bundles: %v", err)
	}
	if len(applied2) != 0 || version2 != 1 {
		t.Fatalf("re-apply applied=%v version=%d; want [], 1", applied2, version2)
	}

	// Objects exist.
	checks := map[string]string{
		"daemon_auth_registry table": "SELECT (to_regclass('striatumd.daemon_auth_registry') IS NOT NULL)::text",
		"daemon_auth_log table":      "SELECT (to_regclass('striatumd.daemon_auth_log') IS NOT NULL)::text",
		"schema_authority table":     "SELECT (to_regclass('striatumd.schema_authority') IS NOT NULL)::text",
		"assert_daemon_authority fn": "SELECT (to_regprocedure('striatumd.assert_daemon_authority()') IS NOT NULL)::text",
		"audit_v3_row_hash fn":       "SELECT (to_regproc('striatumd.audit_v3_row_hash') IS NOT NULL)::text",
		"append_audit_row fn":        "SELECT (to_regproc('striatumd.append_audit_row') IS NOT NULL)::text",
	}
	for name, sql := range checks {
		got, err := pool.Runner.QueryScalar(ctx, sql)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != "true" {
			t.Errorf("%s missing after apply", name)
		}
	}

	stamp, err := pool.Runner.QueryScalar(ctx,
		"SELECT requires_daemon_auth::text FROM striatumd.schema_authority WHERE capability = 'audit_sd_append'")
	if err != nil {
		t.Fatalf("read capability stamp: %v", err)
	}
	if stamp != "true" {
		t.Fatalf("audit_sd_append stamp requires_daemon_auth = %q; want true", stamp)
	}
}
