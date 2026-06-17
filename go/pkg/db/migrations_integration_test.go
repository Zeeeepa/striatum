package db_test

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

func TestLiveMigrationsInstallCurrentSchemaInvariants(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)
	version, err := db.ReadSchemaVersion(ctx, pool.Runner)
	if err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != db.LatestDaemonDBVersion {
		t.Fatalf("schema version = %d, want %d", version, db.LatestDaemonDBVersion)
	}
	for _, table := range []string{
		"repositories",
		"workflow_snapshots",
		"runs",
		"jobs",
		"work_packets",
		"events",
		"repo_event_chain_heads",
		"auto_finalize_circuit_breakers",
		"job_recovery_state",
		"fanin_freeze_points",
		"barrier_staged_contributions",
	} {
		var exists bool
		if err := pool.RawPool.QueryRow(ctx, `
			SELECT EXISTS (
			  SELECT 1 FROM information_schema.tables
			   WHERE table_schema = 'striatumd' AND table_name = $1
			)`, table).Scan(&exists); err != nil {
			t.Fatalf("inspect table %s: %v", table, err)
		}
		if !exists {
			t.Fatalf("expected migrated table striatumd.%s", table)
		}
	}
}
