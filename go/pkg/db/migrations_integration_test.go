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
		"generated_records",
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

func TestLiveMigrationsInstallGeneratedRecordsIndex(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Pool(t)

	columns := map[string]bool{}
	rows, err := pool.RawPool.Query(ctx, `
		SELECT column_name
		  FROM information_schema.columns
		 WHERE table_schema = 'striatumd'
		   AND table_name = 'generated_records'
		 ORDER BY ordinal_position`)
	if err != nil {
		t.Fatalf("list generated_records columns: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatalf("scan generated_records column: %v", err)
		}
		columns[column] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("generated_records columns rows err: %v", err)
	}
	for _, column := range []string{
		"repository_id",
		"record_id",
		"source_path",
		"source_commit",
		"record_class",
		"run_id",
		"job_id",
		"artifact_id",
		"content_sha256",
		"blob_key",
		"blob_sha256",
		"content_type",
		"size_bytes",
		"retention_class",
		"bundle_id",
		"import_batch_id",
		"created_at",
		"status",
	} {
		if !columns[column] {
			t.Fatalf("generated_records missing column %s", column)
		}
	}

	indexes := map[string]bool{}
	indexRows, err := pool.RawPool.Query(ctx, `
		SELECT indexname
		  FROM pg_indexes
		 WHERE schemaname = 'striatumd'
		   AND tablename = 'generated_records'
		 ORDER BY indexname`)
	if err != nil {
		t.Fatalf("list generated_records indexes: %v", err)
	}
	defer indexRows.Close()
	for indexRows.Next() {
		var index string
		if err := indexRows.Scan(&index); err != nil {
			t.Fatalf("scan generated_records index: %v", err)
		}
		indexes[index] = true
	}
	if err := indexRows.Err(); err != nil {
		t.Fatalf("generated_records indexes rows err: %v", err)
	}
	for _, index := range []string{
		"generated_records_pkey",
		"uq_generated_records_repo_blob_key",
		"uq_generated_records_source_commit_path",
		"uq_generated_records_artifact",
		"idx_generated_records_run_job",
		"idx_generated_records_source_path",
		"idx_generated_records_content_sha256",
		"idx_generated_records_status",
		"idx_generated_records_bundle",
		"idx_generated_records_import_batch",
	} {
		if !indexes[index] {
			t.Fatalf("generated_records missing index %s", index)
		}
	}

	for _, privilege := range []string{"SELECT", "INSERT", "UPDATE"} {
		var allowed string
		if err := pool.RawPool.QueryRow(ctx,
			"SELECT has_table_privilege('striatumd_rw', 'striatumd.generated_records', $1)::text", privilege).Scan(&allowed); err != nil {
			t.Fatalf("check generated_records %s privilege: %v", privilege, err)
		}
		if allowed != "true" {
			t.Fatalf("striatumd_rw missing %s on generated_records", privilege)
		}
	}
	var deleteAllowed string
	if err := pool.RawPool.QueryRow(ctx,
		"SELECT has_table_privilege('striatumd_rw', 'striatumd.generated_records', 'DELETE')::text").Scan(&deleteAllowed); err != nil {
		t.Fatalf("check generated_records DELETE privilege: %v", err)
	}
	if deleteAllowed != "false" {
		t.Fatalf("striatumd_rw has DELETE on generated_records; expected status updates instead of row deletion")
	}

	if class, ok := db.ClassifyTable("generated_records"); !ok || class != db.ClassRuntimeDML {
		t.Fatalf("write authority generated_records = (%q, %v), want (%q, true)", class, ok, db.ClassRuntimeDML)
	}
	if class, ok := db.ClassifyReadTable("generated_records"); !ok || class != db.ReadClassRuntimeSensitive {
		t.Fatalf("read authority generated_records = (%q, %v), want (%q, true)", class, ok, db.ReadClassRuntimeSensitive)
	}
}
