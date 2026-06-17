package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

const futureRuntimeMigrationOwnerDDLFloor = 27

var runtimeMigrationOwnerDDLPattern = regexp.MustCompile(`(?is)\b(?:ALTER|DROP)\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?striatumd\.[a-z_][a-z0-9_]*`)

func runtimeMigrationOwnerDDLViolations(migration Migration) []string {
	matches := runtimeMigrationOwnerDDLPattern.FindAllString(migration.SQL, -1)
	violations := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		cleaned := strings.Join(strings.Fields(match), " ")
		if seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		violations = append(violations, cleaned)
	}
	sort.Strings(violations)
	return violations
}

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

// TestMigrationTwentyThreePrincipalsIsOwnershipSafe is RFC 0107: the
// multi-principal migration creates NEW tables + grants the runtime role and
// does NOT ALTER any owner-held table nor declare a foreign key referencing one
// (regression guard for the RFC 0081 owner-table crash-loop). The only FK it
// declares is principal_clients -> principals, both NEW tables the runtime role
// owns; principal_clients.client_id is a bare text column with no FK to the
// owner-held clients table (audit attribution / referential integrity is
// enforced in Go, exactly like audit_log.client_id).
func TestMigrationTwentyThreePrincipalsIsOwnershipSafe(t *testing.T) {
	migrations, err := Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var migration *Migration
	for index := range migrations {
		if migrations[index].Version == 23 {
			migration = &migrations[index]
			break
		}
	}
	if migration == nil {
		t.Fatal("migration 23 is missing")
	}
	sql := migration.SQL
	for _, needle := range []string{
		"CREATE TABLE IF NOT EXISTS striatumd.principals",
		"CREATE TABLE IF NOT EXISTS striatumd.principal_clients",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.principals TO striatumd_rw",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.principal_clients TO striatumd_rw",
	} {
		if !strings.Contains(sql, needle) {
			t.Fatalf("migration 23 missing %q", needle)
		}
	}
	// Ownership-safety: no ALTER of any table, and no foreign key to any
	// owner-held table. striatumd_rw must be able to apply this migration.
	for _, forbidden := range []string{
		"ALTER TABLE",
		"REFERENCES striatumd.clients",
		"REFERENCES striatumd.client_capabilities",
		"REFERENCES striatumd.repositories",
		"REFERENCES striatumd.sessions",
		"REFERENCES striatumd.runs",
		"FOREIGN KEY",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration 23 must not contain %q (owner-table dependency); attribution is enforced in Go", forbidden)
		}
	}
}

// TestMigrationTwentyEightJobWorkspacesIsOwnershipSafe is RFC 0127 P0 (D195): the
// plain-dir workspace migration creates a NEW runtime-owned table + grants the
// runtime role, recording the staged base tree sha. Like migrations 16/23 it must
// NOT ALTER any table (the regular-migration owner-DDL guard forbids ALTER of
// striatumd.* for versions >= 27 outright) and must declare NO foreign key
// (referential integrity is enforced in Go in the workspace.create handler), so
// the runtime role striatumd_rw can apply it without the RFC 0081 owner-table
// crash-loop.
func TestMigrationTwentyEightJobWorkspacesIsOwnershipSafe(t *testing.T) {
	migrations, err := Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var migration *Migration
	for index := range migrations {
		if migrations[index].Version == 28 {
			migration = &migrations[index]
			break
		}
	}
	if migration == nil {
		t.Fatal("migration 28 is missing")
	}
	sql := migration.SQL
	for _, needle := range []string{
		"CREATE TABLE IF NOT EXISTS striatumd.job_workspaces",
		"PRIMARY KEY (repository_id, workspace_id)",
		"base_tree_sha text NOT NULL",
		"workspace_kind text NOT NULL CHECK (workspace_kind IN ('plain_dir'))",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.job_workspaces TO striatumd_rw",
	} {
		if !strings.Contains(sql, needle) {
			t.Fatalf("migration 28 missing %q", needle)
		}
	}
	// Ownership-safety: no ALTER/DROP of any table, and no foreign key to any
	// table. striatumd_rw must be able to apply this migration.
	for _, forbidden := range []string{
		"ALTER TABLE",
		"DROP TABLE",
		"REFERENCES striatumd.repositories",
		"REFERENCES striatumd.runs",
		"REFERENCES striatumd.jobs",
		"REFERENCES striatumd.leases",
		"FOREIGN KEY",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration 28 must not contain %q (owner-table dependency / future-runtime-DDL guard); integrity is enforced in Go", forbidden)
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

// TestMigrationNineteenAddsPTYAndToolLivenessColumns is RFC 0101 Phase 1: the
// migration adds last_pty_activity_at / last_tool_call_started_at /
// last_tool_call_finished_at to the owner-held sessions table so the classifier
// can fuse PTY + tool-call signals (#80/#83/#117). Like migration 12 it ALTERs
// an owner table, so it is owner-applied; it is idempotent (every ADD COLUMN is
// IF NOT EXISTS) so a re-run is a safe no-op.
func TestMigrationNineteenAddsPTYAndToolLivenessColumns(t *testing.T) {
	migrations, err := Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var migration *Migration
	for index := range migrations {
		if migrations[index].Version == 19 {
			migration = &migrations[index]
			break
		}
	}
	if migration == nil {
		t.Fatal("migration 19 is missing")
	}
	sql := migration.SQL
	for _, needle := range []string{
		"ALTER TABLE striatumd.sessions",
		"ADD COLUMN IF NOT EXISTS last_pty_activity_at timestamptz",
		"ADD COLUMN IF NOT EXISTS last_tool_call_started_at timestamptz",
		"ADD COLUMN IF NOT EXISTS last_tool_call_finished_at timestamptz",
	} {
		if !strings.Contains(sql, needle) {
			t.Fatalf("migration 19 missing %q", needle)
		}
	}
	if migration.Label == "" {
		t.Fatal("migration 19 has no label")
	}
}

// TestMigrationTwentyNineFaninBarrierIsOwnershipSafe is RFC 0135 P1 (D215/D216):
// the fan-in sealed-barrier migration creates the two NEW runtime-owned tables the
// live-seal JOIN barrier reads (the append-only freeze record + the
// attempt-addressed staging table) and grants the runtime role, with the
// freeze table granted SELECT, INSERT ONLY (append-only) backed by a refuse-trigger.
// Like migrations 16/23/28 it must NOT ALTER/DROP any owner table (the
// future-runtime-DDL guard forbids ALTER/DROP of striatumd.* for versions >= 27
// outright) and must declare NO foreign key to striatumd.jobs (the FK-to-owner-
// table trap, D215): the seat identity is carried as bare columns and referential
// integrity is enforced in Go (the live-seal JOIN at evaluation time).
func TestMigrationTwentyNineFaninBarrierIsOwnershipSafe(t *testing.T) {
	migrations, err := Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var migration *Migration
	for index := range migrations {
		if migrations[index].Version == 29 {
			migration = &migrations[index]
			break
		}
	}
	if migration == nil {
		t.Fatal("migration 29 is missing")
	}
	sql := migration.SQL
	for _, needle := range []string{
		"CREATE TABLE IF NOT EXISTS striatumd.fanin_freeze_points",
		"CREATE TABLE IF NOT EXISTS striatumd.barrier_staged_contributions",
		"PRIMARY KEY (repository_id, barrier_id)",
		"PRIMARY KEY (repository_id, barrier_id, workflow_job_id, attempt)",
		// The freeze record is append-only: SELECT, INSERT only + a refuse-trigger.
		"GRANT SELECT, INSERT ON striatumd.fanin_freeze_points TO striatumd_rw",
		"refuse_repo_append_only_change",
		"BEFORE UPDATE ON striatumd.fanin_freeze_points",
		"BEFORE DELETE ON striatumd.fanin_freeze_points",
		// The staging table carries full DML (a requeue tombstones a stale row).
		"GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.barrier_staged_contributions TO striatumd_rw",
	} {
		if !strings.Contains(sql, needle) {
			t.Fatalf("migration 29 missing %q", needle)
		}
	}
	// Ownership-safety: no ALTER/DROP of any table, and no foreign key to any
	// table. striatumd_rw must be able to apply this migration; the seat identity
	// (repository_id, run_id, workflow_job_id, attempt) is bare columns and the
	// referential integrity is enforced in Go.
	for _, forbidden := range []string{
		"ALTER TABLE",
		"DROP TABLE",
		"REFERENCES striatumd.repositories",
		"REFERENCES striatumd.runs",
		"REFERENCES striatumd.jobs",
		"FOREIGN KEY",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration 29 must not contain %q (owner-table dependency / future-runtime-DDL guard); integrity is enforced in Go", forbidden)
		}
	}
}

func TestFutureRuntimeMigrationsDoNotCarryOwnerDDL(t *testing.T) {
	migrations, err := Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	for _, migration := range migrations {
		if migration.Version < futureRuntimeMigrationOwnerDDLFloor {
			continue
		}
		if violations := runtimeMigrationOwnerDDLViolations(migration); len(violations) > 0 {
			t.Fatalf("migration %d carries owner-table DDL forbidden in regular runtime migrations: %v", migration.Version, violations)
		}
	}
}

func TestRuntimeMigrationOwnerDDLGuardDetectsForbiddenDDL(t *testing.T) {
	migration := Migration{
		Version: futureRuntimeMigrationOwnerDDLFloor,
		SQL: `
			ALTER TABLE striatumd.runs ADD COLUMN IF NOT EXISTS unsafe text;
			DROP TABLE IF EXISTS ONLY striatumd.sessions;
			CREATE TABLE IF NOT EXISTS striatumd.new_runtime_table (id text PRIMARY KEY);
		`,
	}

	violations := runtimeMigrationOwnerDDLViolations(migration)

	expected := []string{
		"ALTER TABLE striatumd.runs",
		"DROP TABLE IF EXISTS ONLY striatumd.sessions",
	}
	if !reflect.DeepEqual(violations, expected) {
		t.Fatalf("violations = %#v, want %#v", violations, expected)
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
