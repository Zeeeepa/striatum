package db

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const EnvDaemonDBURL = "STRIATUM_DAEMON_DB_URL"

type Config struct {
	URL        string
	Source     string
	ConfigPath string
}

func ResolveConfig(explicitURL string) Config {
	configPath := DefaultConfigPath()
	if explicitURL != "" {
		return Config{URL: explicitURL, Source: "--postgres-url", ConfigPath: configPath}
	}
	if envURL := os.Getenv(EnvDaemonDBURL); envURL != "" {
		return Config{URL: envURL, Source: EnvDaemonDBURL, ConfigPath: configPath}
	}
	if fileURL := readConfigURL(configPath); fileURL != "" {
		return Config{URL: fileURL, Source: configPath, ConfigPath: configPath}
	}
	return Config{ConfigPath: configPath}
}

func DefaultConfigPath() string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "striatum", "daemon.toml")
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "striatum", "daemon.toml")
}

func RedactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.User != nil {
		username := parsed.User.Username()
		if _, ok := parsed.User.Password(); ok {
			parsed.User = url.UserPassword(username, "<redacted>")
		}
	}
	query := parsed.Query()
	for _, key := range []string{"password", "pass", "token", "sslpassword"} {
		if _, ok := query[key]; ok {
			query.Set(key, "<redacted>")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// Row mirrors pgx.Row for callers that do not want to depend on pgx directly.
// Concrete implementations returned by Runner / TxRunner are pgx.Row values.
type Row = pgx.Row

// Runner is the database access surface used by the daemon. Calls are
// parameterized; multi-statement DDL is allowed because the runtime is
// configured for the PostgreSQL simple protocol.
type Runner interface {
	Exec(ctx context.Context, sql string, args ...any) error
	QueryRow(ctx context.Context, sql string, args ...any) Row
	QueryScalar(ctx context.Context, sql string, args ...any) (string, error)
	BeginTx(ctx context.Context) (TxRunner, error)
}

// TxRunner is the transaction-scoped sibling of Runner. Callers must call
// Commit or Rollback before the surrounding context is cancelled.
type TxRunner interface {
	Exec(ctx context.Context, sql string, args ...any) error
	QueryRow(ctx context.Context, sql string, args ...any) Row
	QueryScalar(ctx context.Context, sql string, args ...any) (string, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Pool wraps a pgxpool.Pool with the Runner surface and a Close helper. The
// caller owns the pool lifetime and must invoke Close on shutdown.
//
// V1.7: RawPool exposes the underlying pgxpool.Pool so consumers that need
// pgx-typed access (e.g. SupervisorPointerStore for transactional reads
// with explicit row scanning) can construct against it without forcing
// the Runner abstraction to grow surface area.
type Pool struct {
	URL     string
	Runner  Runner
	RawPool *pgxpool.Pool
	Close   func()
}

// PgxRunner is a thin adapter from a pgx connection pool to the Runner
// interface used by the rest of the daemon.
type PgxRunner struct {
	Pool *pgxpool.Pool
}

func (r PgxRunner) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := r.Pool.Exec(ctx, sql, args...)
	return err
}

func (r PgxRunner) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return r.Pool.QueryRow(ctx, sql, args...)
}

func (r PgxRunner) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return r.Pool.Query(ctx, sql, args...)
}

func (r PgxRunner) QueryScalar(ctx context.Context, sql string, args ...any) (string, error) {
	var value string
	err := r.Pool.QueryRow(ctx, sql, args...).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func (r PgxRunner) BeginTx(ctx context.Context) (TxRunner, error) {
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	return &PgxTxRunner{Tx: tx}, nil
}

// PgxTxRunner is the transaction-scoped twin of PgxRunner.
type PgxTxRunner struct {
	Tx pgx.Tx
}

func (t *PgxTxRunner) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := t.Tx.Exec(ctx, sql, args...)
	return err
}

func (t *PgxTxRunner) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return t.Tx.QueryRow(ctx, sql, args...)
}

func (t *PgxTxRunner) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.Tx.Query(ctx, sql, args...)
}

func (t *PgxTxRunner) QueryScalar(ctx context.Context, sql string, args ...any) (string, error) {
	var value string
	err := t.Tx.QueryRow(ctx, sql, args...).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func (t *PgxTxRunner) Commit(ctx context.Context) error {
	return t.Tx.Commit(ctx)
}

func (t *PgxTxRunner) Rollback(ctx context.Context) error {
	return t.Tx.Rollback(ctx)
}

// Connect opens a pgx connection pool for the daemon. The pool is configured
// with an application_name tagged with the daemon version and the PostgreSQL
// simple protocol so that multi-statement migration files run unchanged.
func Connect(ctx context.Context, postgresURL string, daemonVersion string) (*Pool, error) {
	if postgresURL == "" {
		return nil, errors.New("daemon PostgreSQL URL is not configured")
	}
	cfg, err := pgxpool.ParseConfig(postgresURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres url: %w", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = "striatumd-go/" + daemonVersion
	if _, ok := cfg.ConnConfig.RuntimeParams["statement_timeout"]; !ok {
		cfg.ConnConfig.RuntimeParams["statement_timeout"] = "60000"
	}
	// Simple protocol keeps migration DDL with multiple statements working and
	// still binds parameters safely via client-side quoting; the daemon's hot
	// queries are low cardinality and do not need server-side prepared plans.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Pool{
		URL:     postgresURL,
		Runner:  PgxRunner{Pool: pool},
		RawPool: pool,
		Close:   pool.Close,
	}, nil
}

// ConnectAndMigrate opens the pool and applies pending migrations using the
// daemon version label for provenance.
func ConnectAndMigrate(ctx context.Context, postgresURL string, daemonVersion string) (*Pool, int, error) {
	config := ResolveConfig(postgresURL)
	if config.URL == "" {
		return nil, 0, errors.New("daemon PostgreSQL URL is not configured")
	}
	pool, err := Connect(ctx, config.URL, daemonVersion)
	if err != nil {
		return nil, 0, err
	}
	version, err := ApplyMigrations(ctx, pool.Runner, daemonVersion)
	if err != nil {
		pool.Close()
		return nil, 0, err
	}
	return pool, version, nil
}

func readConfigURL(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "postgres_url") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			}
		}
	}
	return ""
}
