package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type fakeRow struct {
	value string
	err   error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) == 0 {
		return nil
	}
	switch target := dest[0].(type) {
	case *string:
		*target = r.value
	case **string:
		if r.value == "" {
			*target = nil
		} else {
			v := r.value
			*target = &v
		}
	}
	return nil
}

type fakeRunner struct {
	scalars map[string]string
	execs   []string
}

func (f *fakeRunner) Exec(_ context.Context, sql string, _ ...any) error {
	f.execs = append(f.execs, sql)
	return nil
}

func (f *fakeRunner) QueryRow(_ context.Context, sql string, _ ...any) Row {
	for key, value := range f.scalars {
		if strings.Contains(sql, key) {
			return fakeRow{value: value}
		}
	}
	return fakeRow{err: pgx.ErrNoRows}
}

func (f *fakeRunner) QueryScalar(_ context.Context, sql string, _ ...any) (string, error) {
	for key, value := range f.scalars {
		if strings.Contains(sql, key) {
			return value, nil
		}
	}
	return "", nil
}

func (f *fakeRunner) BeginTx(_ context.Context) (TxRunner, error) {
	return nil, errors.New("fakeRunner does not support transactions")
}

func TestMigrationsAreOrdered(t *testing.T) {
	migrations, err := Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migrations) != LatestDaemonDBVersion {
		t.Fatalf("migration count = %d, want %d", len(migrations), LatestDaemonDBVersion)
	}
	for index, migration := range migrations {
		if migration.Version != index+1 {
			t.Fatalf("migration %d has version %d", index, migration.Version)
		}
		if migration.SHA256() == "" {
			t.Fatalf("migration %d missing sha", migration.Version)
		}
	}
}

func TestVerifyMigrationsSHASourceRejectsExtraSourceMigration(t *testing.T) {
	// Post-RFC-0078 the embedded migrations are the canonical source of truth
	// (the Python source tree is gone). Reconstruct a source dir from the embed,
	// add a spurious newer file, and assert VerifyMigrationsSHASource rejects it.
	tmp := t.TempDir()
	entries, err := migrationFS.ReadDir("sql")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		body, err := migrationFS.ReadFile(filepath.Join("sql", entry.Name()))
		if err != nil {
			t.Fatalf("read embedded migration: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmp, entry.Name()), body, 0o644); err != nil {
			t.Fatalf("write copied migration: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "9999_future.sql"), []byte("SELECT 1;\n"), 0o644); err != nil {
		t.Fatalf("write future migration: %v", err)
	}

	err = VerifyMigrationsSHASource(tmp)

	if err == nil || !strings.Contains(err.Error(), "newer than the embedded Go daemon migrations") {
		t.Fatalf("expected extra migration refusal, got %v", err)
	}
}

func TestMigrationFiveCarriesRepoLocalWorkflowTables(t *testing.T) {
	migrations, err := Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var migrationFive *Migration
	for index := range migrations {
		if migrations[index].Version == 5 {
			migrationFive = &migrations[index]
			break
		}
	}
	if migrationFive == nil {
		t.Fatal("migration 5 is missing")
	}
	for _, table := range []string{
		"workflow_snapshots",
		"runs",
		"sessions",
		"jobs",
		"queue_messages",
		"leases",
		"work_packets",
		"events",
	} {
		if !strings.Contains(migrationFive.SQL, "striatumd."+table) {
			t.Fatalf("migration 5 missing repo-local table %s", table)
		}
	}
}

func TestMigrationFourteenCarriesAutoFinalizeCircuitBreakers(t *testing.T) {
	migrations, err := Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var migrationFourteen *Migration
	for index := range migrations {
		if migrations[index].Version == 14 {
			migrationFourteen = &migrations[index]
			break
		}
	}
	if migrationFourteen == nil {
		t.Fatal("migration 14 is missing")
	}
	for _, needle := range []string{
		"striatumd.auto_finalize_circuit_breakers",
		"PRIMARY KEY (repository_id, run_id, workflow_job_id, cause)",
		"auto_finalize_circuit_breakers_open_idx",
		"GRANT SELECT, INSERT, UPDATE, DELETE",
	} {
		if !strings.Contains(migrationFourteen.SQL, needle) {
			t.Fatalf("migration 14 missing %q", needle)
		}
	}
}

// TestMigrationSixteenInterrogationsIsOwnershipSafe is RFC 0082 Required Test 9:
// the interrogations migration creates a new table + grants the runtime role
// and does NOT ALTER owner tables nor declare foreign keys referencing
// owner-held tables (regression guard for the RFC 0081 incident).
func TestMigrationSixteenInterrogationsIsOwnershipSafe(t *testing.T) {
	migrations, err := Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var migration *Migration
	for index := range migrations {
		if migrations[index].Version == 16 {
			migration = &migrations[index]
			break
		}
	}
	if migration == nil {
		t.Fatal("migration 16 is missing")
	}
	sql := migration.SQL
	for _, needle := range []string{
		"CREATE TABLE IF NOT EXISTS striatumd.interrogations",
		"PRIMARY KEY (repository_id, interrogation_id)",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.interrogations TO striatumd_rw",
	} {
		if !strings.Contains(sql, needle) {
			t.Fatalf("migration 16 missing %q", needle)
		}
	}
	// Ownership-safety: no ALTER of any table, and no foreign keys to the
	// owner-held tables. striatumd_rw must be able to apply this migration.
	for _, forbidden := range []string{
		"ALTER TABLE",
		"REFERENCES striatumd.repositories",
		"REFERENCES striatumd.runs",
		"REFERENCES striatumd.sessions",
		"FOREIGN KEY",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration 16 must not contain %q (owner-table dependency); referential integrity is enforced in Go", forbidden)
		}
	}
}

// TestMigrationEighteenWidensArtifactAttemptScope is RFC 0095 §1 / GH #84:
// migration 18 records the producing attempt on each artifact and widens BOTH
// artifact unique keys with `attempt`, so a re-opened attempt may republish the
// same logical_name/path under its own attempt. It is owner-applied (it ALTERs
// the owner-held artifacts table), so unlike migrations >=16 it is ALLOWED to
// ALTER an owner table and there is no ownership-safety guard here.
func TestMigrationEighteenWidensArtifactAttemptScope(t *testing.T) {
	migrations, err := Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var migration *Migration
	for index := range migrations {
		if migrations[index].Version == 18 {
			migration = &migrations[index]
			break
		}
	}
	if migration == nil {
		t.Fatal("migration 18 is missing")
	}
	sql := migration.SQL
	for _, needle := range []string{
		"ALTER TABLE striatumd.artifacts",
		"ADD COLUMN IF NOT EXISTS attempt integer NOT NULL DEFAULT 1",
		"DROP CONSTRAINT IF EXISTS artifacts_repository_id_run_id_job_id_logical_name_key",
		"UNIQUE (repository_id, run_id, job_id, logical_name, attempt)",
		"DROP CONSTRAINT IF EXISTS artifacts_repository_id_run_id_repo_path_content_sha256_key",
		"UNIQUE (repository_id, run_id, repo_path, content_sha256, attempt)",
	} {
		if !strings.Contains(sql, needle) {
			t.Fatalf("migration 18 missing %q", needle)
		}
	}
}

func TestApplyMigrationsRecordsVersion(t *testing.T) {
	runner := &fakeRunner{scalars: map[string]string{}}
	version, err := ApplyMigrations(context.Background(), runner, "test")
	if err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if version != LatestDaemonDBVersion {
		t.Fatalf("version = %d, want %d", version, LatestDaemonDBVersion)
	}
	joined := strings.Join(runner.execs, "\n")
	if !strings.Contains(joined, "substrate_version") {
		t.Fatalf("substrate version was not recorded")
	}
}

func TestDeriveMigrationLockKey(t *testing.T) {
	ctx := context.Background()
	runner := &fakeRunner{scalars: map[string]string{
		"current_database": "test_db",
	}}
	key, err := deriveMigrationLockKey(ctx, runner)
	if err != nil {
		t.Fatalf("deriveMigrationLockKey error: %v", err)
	}
	if key == 0 {
		t.Fatal("expected non-zero migration lock key")
	}
}

