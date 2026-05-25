package pgtest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

const EnvPGTestURL = "STRIATUM_PG_TEST_URL"

// DB returns a migrated Runner wrapped in a rollback transaction.
func DB(t *testing.T) db.Runner {
	t.Helper()
	pool := Pool(t)
	tx, err := pool.Runner.BeginTx(context.Background())
	if err != nil {
		t.Fatalf("begin pgtest tx: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = tx.Rollback(ctx)
	})
	return txRunner{TxRunner: tx}
}

// Pool returns a migrated, isolated database. It skips when STRIATUM_PG_TEST_URL
// is unset and drops the database during test cleanup.
func Pool(t *testing.T) *db.Pool {
	t.Helper()
	baseURL := os.Getenv(EnvPGTestURL)
	if strings.TrimSpace(baseURL) == "" {
		t.Skip(EnvPGTestURL + " not set; skipping live PostgreSQL test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	testURL, drop := createDatabase(t, ctx, baseURL)
	t.Cleanup(drop)

	pool, version, err := db.ConnectAndMigrate(ctx, testURL, "pgtest")
	if err != nil {
		t.Fatalf("connect/migrate pgtest database: %v", err)
	}
	if version != db.LatestDaemonDBVersion {
		t.Fatalf("schema version = %d, want %d", version, db.LatestDaemonDBVersion)
	}
	t.Cleanup(pool.Close)
	return pool
}

type txRunner struct {
	db.TxRunner
}

func (r txRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, fmt.Errorf("pgtest rollback runner cannot open nested transactions")
}

func createDatabase(t *testing.T, ctx context.Context, baseURL string) (string, func()) {
	t.Helper()
	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse %s: %v", EnvPGTestURL, err)
	}
	name := fmt.Sprintf("striatum_pgtest_%d_%d", time.Now().UnixNano(), os.Getpid())
	adminPool, err := pgxpool.New(ctx, baseURL)
	if err != nil {
		t.Fatalf("connect pgtest admin database: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+quoteIdent(name)); err != nil {
		adminPool.Close()
		t.Fatalf("create pgtest database: %v", err)
	}
	testURL := *parsed
	testURL.Path = "/" + name
	drop := func() {
		dropCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_, _ = adminPool.Exec(dropCtx, `
			SELECT pg_terminate_backend(pid)
			  FROM pg_stat_activity
			 WHERE datname = $1 AND pid <> pg_backend_pid()`, name)
		_, _ = adminPool.Exec(dropCtx, "DROP DATABASE IF EXISTS "+quoteIdent(name))
		adminPool.Close()
	}
	return testURL.String(), drop
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
