package db_test

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// TestVerifyCapabilityParityInertAgainstLivePG exercises the actual startup
// query path (readStampedCapabilities) against a migrated database. In release N
// the schema_authority marker table is absent, so the checker must return nil
// (inert) without erroring — this is the daemon-boot gate, so a scan/type error
// here would crash startup.
func TestVerifyCapabilityParityInertAgainstLivePG(t *testing.T) {
	pool := pgtest.Pool(t)
	if err := db.VerifyCapabilityParity(context.Background(), pool.Runner, nil, nil); err != nil {
		t.Fatalf("inert parity check against a live migrated DB must pass, got: %v", err)
	}
	// Even with required caps, an absent authority schema is inert (the N->N+1
	// sequencing guarantee).
	if err := db.VerifyCapabilityParity(context.Background(), pool.Runner, []string{"cap.audit"}, nil); err != nil {
		t.Fatalf("inert parity check with required caps must pass when no authority schema exists, got: %v", err)
	}
}
