package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	LatestDaemonDBVersion = 8
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
		5: "repo-local workflow state substrate",
		6: "events chain anchors + repo_event_chain_heads",
		7: "decision propagation projections",
		8: "lane evidence publish guard",
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
	if err := runner.Exec(ctx, "SELECT pg_advisory_lock($1)", MigrationLockKey); err != nil {
		return 0, err
	}
	unlocked := false
	defer func() {
		if !unlocked {
			_ = runner.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", MigrationLockKey)
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
	if err := runner.Exec(ctx, "SELECT pg_advisory_unlock($1)", MigrationLockKey); err != nil {
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

// VerifyMigrationsSHASource compares the embedded migration SHAs against the
// SQL files on disk at the supplied path. The comparison is by-filename and
// guards against drift between the Go-embedded SQL and the Python source-of-
// truth tree. Returns a non-nil error if any file is missing or differs.
func VerifyMigrationsSHASource(path string) error {
	migrations, err := Migrations()
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		name := filepath.Base(migration.Path)
		body, err := os.ReadFile(filepath.Join(path, name))
		if err != nil {
			return fmt.Errorf("read source migration %s: %w", name, err)
		}
		sum := sha256.Sum256(body)
		actual := hex.EncodeToString(sum[:])
		if actual != migration.SHA256() {
			return fmt.Errorf("migration %s sha mismatch: embedded=%s source=%s", name, migration.SHA256(), actual)
		}
	}
	return nil
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
	value, err := runner.QueryScalar(ctx, "SELECT sha256 FROM striatumd.schema_migrations WHERE version = $1", migration.Version)
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
	if err := runner.Exec(
		ctx,
		`INSERT INTO striatumd.schema_migrations(version, label, sha256, daemon_version)
VALUES ($1, $2, $3, $4)
ON CONFLICT (version) DO NOTHING`,
		migration.Version,
		migration.Label,
		migration.SHA256(),
		daemonVersion,
	); err != nil {
		return err
	}
	return runner.Exec(
		ctx,
		`INSERT INTO striatumd.schema_meta(key, value)
VALUES ('substrate_version', $1)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		strconv.Itoa(migration.Version),
	)
}
