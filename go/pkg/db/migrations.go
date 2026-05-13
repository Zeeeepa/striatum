package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	LatestDaemonDBVersion = 4
	MigrationLockKey      = 332933
)

//go:embed sql/*.sql
var migrationFS embed.FS

type Migration struct {
	Version int
	Label   string
	Path    string
	SQL     string
}

func Migrations() ([]Migration, error) {
	labels := map[int]string{
		1: "baseline daemon postgres substrate",
		2: "daemon rpc supervision and apply receipts",
		3: "cross-repo workflows and MCP mutation scope",
		4: "dogfood surgical recovery capability",
	}
	entries, err := migrationFS.ReadDir("sql")
	if err != nil {
		return nil, err
	}
	migrations := []Migration{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, err := strconv.Atoi(strings.SplitN(entry.Name(), "_", 2)[0])
		if err != nil {
			return nil, err
		}
		path := "sql/" + entry.Name()
		body, err := migrationFS.ReadFile(path)
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, Migration{
			Version: version,
			Label:   labels[version],
			Path:    path,
			SQL:     string(body),
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

func ApplyMigrations(ctx context.Context, runner Runner, daemonVersion string) (int, error) {
	if err := runner.Exec(ctx, fmt.Sprintf("SELECT pg_advisory_lock(%d)", MigrationLockKey)); err != nil {
		return 0, err
	}
	unlocked := false
	defer func() {
		if !unlocked {
			_ = runner.Exec(context.Background(), fmt.Sprintf("SELECT pg_advisory_unlock(%d)", MigrationLockKey))
		}
	}()
	if err := ensureMetaTable(ctx, runner); err != nil {
		return 0, err
	}
	current, err := ReadSchemaVersion(ctx, runner)
	if err != nil {
		return 0, err
	}
	if current > LatestDaemonDBVersion {
		return 0, fmt.Errorf("daemon PostgreSQL schema version %d is newer than supported %d", current, LatestDaemonDBVersion)
	}
	migrations, err := Migrations()
	if err != nil {
		return 0, err
	}
	for _, migration := range migrations {
		if migration.Version <= current {
			if err := verifyRecordedHash(ctx, runner, migration); err != nil {
				return 0, err
			}
			continue
		}
		if err := applyOne(ctx, runner, migration, daemonVersion); err != nil {
			return 0, err
		}
		current = migration.Version
	}
	if err := runner.Exec(ctx, fmt.Sprintf("SELECT pg_advisory_unlock(%d)", MigrationLockKey)); err != nil {
		return current, err
	}
	unlocked = true
	return current, nil
}

func ReadSchemaVersion(ctx context.Context, runner Runner) (int, error) {
	value, err := runner.QueryScalar(ctx, "SELECT value FROM striatumd.schema_meta WHERE key = 'substrate_version'")
	if err != nil || value == "" {
		return 0, nil
	}
	version, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (m Migration) SHA256() string {
	sum := sha256.Sum256([]byte(m.SQL))
	return hex.EncodeToString(sum[:])
}

func ensureMetaTable(ctx context.Context, runner Runner) error {
	return runner.Exec(ctx, `
CREATE SCHEMA IF NOT EXISTS striatumd;
CREATE TABLE IF NOT EXISTS striatumd.schema_meta (
  key text PRIMARY KEY,
  value text NOT NULL
);`)
}

func verifyRecordedHash(ctx context.Context, runner Runner, migration Migration) error {
	sql := fmt.Sprintf("SELECT sha256 FROM striatumd.schema_migrations WHERE version = %d", migration.Version)
	value, err := runner.QueryScalar(ctx, sql)
	if err != nil || value == "" {
		return nil
	}
	if strings.TrimSpace(value) != migration.SHA256() {
		return fmt.Errorf("daemon PostgreSQL migration %d hash mismatch", migration.Version)
	}
	return nil
}

func applyOne(ctx context.Context, runner Runner, migration Migration, daemonVersion string) error {
	if err := runner.Exec(ctx, migration.SQL); err != nil {
		return err
	}
	record := fmt.Sprintf(`
INSERT INTO striatumd.schema_migrations(version, label, sha256, daemon_version)
VALUES (%d, %s, %s, %s)
ON CONFLICT (version) DO NOTHING;
INSERT INTO striatumd.schema_meta(key, value)
VALUES ('substrate_version', %s)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;`,
		migration.Version,
		quoteLiteral(migration.Label),
		quoteLiteral(migration.SHA256()),
		quoteLiteral(daemonVersion),
		quoteLiteral(strconv.Itoa(migration.Version)),
	)
	return runner.Exec(ctx, record)
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
