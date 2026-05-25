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
