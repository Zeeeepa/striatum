package db_test

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestDeploySkew is T-DEPLOY-SKEW (RFC 0110 §8.2): the startup capability-parity
// check is inert with no bundle, passes when the binary supports the stamped
// capability, and fails closed when the schema stamps a capability the binary
// does not support (old binary vs authority-bearing schema). It also asserts the
// owner bundle applies atomically — a mid-bundle failure leaves no marker and no
// partial objects.
func TestDeploySkew(t *testing.T) {
	ctx := context.Background()

	t.Run("inert_without_bundle", func(t *testing.T) {
		pool := pgtest.Pool(t)
		if err := db.VerifyCapabilityParity(ctx, pool.Runner,
			db.RequiredAuthorityCapabilities(), db.SupportedAuthorityCapabilities()); err != nil {
			t.Fatalf("no-bundle parity must be inert pass, got: %v", err)
		}
	})

	t.Run("parity_pass_when_supported", func(t *testing.T) {
		pool := pgtest.Pool(t)
		if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
			t.Fatalf("apply bundle: %v", err)
		}
		if err := db.VerifyCapabilityParity(ctx, pool.Runner,
			db.RequiredAuthorityCapabilities(), db.SupportedAuthorityCapabilities()); err != nil {
			t.Fatalf("supported binary vs stamped schema must pass, got: %v", err)
		}
	})

	t.Run("refuse_when_binary_lacks_support", func(t *testing.T) {
		pool := pgtest.Pool(t)
		if _, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test"); err != nil {
			t.Fatalf("apply bundle: %v", err)
		}
		// Simulate an old binary: it supports no authority capabilities, but the
		// schema stamps audit_sd_append (requires_daemon_auth). Boot must refuse.
		if err := db.VerifyCapabilityParity(ctx, pool.Runner, nil, nil); err == nil {
			t.Fatal("old binary against authority-bearing schema must fail closed")
		}
	})

	t.Run("bundle_apply_is_atomic_on_failure", func(t *testing.T) {
		pool := pgtest.Pool(t)
		// Remove a table the bundle's REVOKE depends on, so applying the bundle
		// fails partway through its single transaction.
		if err := pool.Runner.Exec(ctx, "DROP TABLE striatumd.audit_log CASCADE"); err != nil {
			t.Fatalf("drop audit_log: %v", err)
		}
		_, _, err := db.ApplyOwnerBundles(ctx, pool.Runner, "test")
		if err == nil {
			t.Fatal("expected the bundle to fail with audit_log missing")
		}
		// No marker stamped and no partial objects: the version is still 0 and an
		// object created early in the bundle (the registry) is absent.
		if v, verr := db.OwnerBundleVersion(ctx, pool.Runner); verr != nil || v != 0 {
			t.Fatalf("after a failed apply, version=%d err=%v; want 0 (no partial marker)", v, verr)
		}
		present, perr := pool.Runner.QueryScalar(ctx,
			"SELECT (to_regclass('striatumd.daemon_auth_registry') IS NOT NULL)::text")
		if perr != nil {
			t.Fatalf("check registry: %v", perr)
		}
		if present == "true" {
			t.Fatal("a failed bundle left a partial object (daemon_auth_registry); not atomic")
		}
	})
}
