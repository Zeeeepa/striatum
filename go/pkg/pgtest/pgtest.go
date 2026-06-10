package pgtest

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const EnvPGTestURL = "STRIATUM_PG_TEST_URL"
const staleDatabaseMaxAge = 6 * time.Hour

var pgtestDatabaseNameRE = regexp.MustCompile(`^striatum_pgtest_([0-9]+)_([0-9]+)$`)

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
	p, _ := Pools(t)
	return p
}

// Pools returns both privileged and unprivileged connection pools.
func Pools(t *testing.T) (*db.Pool, *db.Pool) {
	t.Helper()
	baseURL := os.Getenv(EnvPGTestURL)
	if strings.TrimSpace(baseURL) == "" {
		t.Skip(EnvPGTestURL + " not set; skipping live PostgreSQL test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	testURL, drop := createDatabase(t, ctx, baseURL)
	t.Cleanup(drop)

	// RFC 0110 §10 (C-PGTEST-NO-DML-GRANT): the canonical runtime role
	// striatumd_rw must exist BEFORE migrate so migration 0005 provisions its
	// production grant surface (broad table DML, then REVOKE UPDATE/DELETE on the
	// append-only events/artifacts) on this test database — the same surface the
	// 42501 negative-path gate must run against, instead of a hand-built one.
	ensureRuntimeRole(t, ctx, baseURL)

	pool, version, err := db.ConnectAndMigrate(ctx, testURL, "pgtest")
	if err != nil {
		t.Fatalf("connect/migrate pgtest database: %v", err)
	}
	if version != db.LatestDaemonDBVersion {
		t.Fatalf("schema version = %d, want %d", version, db.LatestDaemonDBVersion)
	}
	t.Cleanup(pool.Close)

	currentUser, err := pool.Runner.QueryScalar(ctx, "SELECT current_user")
	if err != nil {
		t.Fatalf("failed to query current user: %v", err)
	}

	parsed, err := url.Parse(testURL)
	if err != nil {
		t.Fatalf("parse test URL: %v", err)
	}
	dbName := strings.TrimPrefix(parsed.Path, "/")
	roleName := "striatumd_rw_" + dbName

	// The per-test LOGIN role is only a login shell over the migration-defined
	// striatumd_rw privileges (membership); pgtest issues NO GRANT/REVOKE naming a
	// protected table (G-PGTEST-GRANTS asserts this against roleSetupStatements).
	for _, stmt := range roleSetupStatements(dbName, roleName, currentUser) {
		if _, err := pool.RawPool.Exec(ctx, stmt); err != nil {
			t.Fatalf("setup unprivileged role (%q): %v", stmt, err)
		}
	}

	t.Cleanup(func() {
		adminPool, err := pgxpool.New(context.Background(), baseURL)
		if err == nil {
			_, _ = adminPool.Exec(context.Background(), fmt.Sprintf("DROP ROLE IF EXISTS %s", quoteIdent(roleName)))
			adminPool.Close()
		}
	})

	unprivilegedCfg, err := pgxpool.ParseConfig(testURL)
	if err != nil {
		t.Fatalf("parse unprivileged pgtest url: %v", err)
	}
	unprivilegedCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, fmt.Sprintf("SET ROLE %s", quoteIdent(roleName)))
		return err
	}

	unprivilegedPgPool, err := pgxpool.NewWithConfig(ctx, unprivilegedCfg)
	if err != nil {
		t.Fatalf("create unprivileged pgtest pool: %v", err)
	}
	t.Cleanup(unprivilegedPgPool.Close)

	unprivilegedPool := &db.Pool{
		URL:     testURL,
		Runner:  db.PgxRunner{Pool: unprivilegedPgPool},
		RawPool: unprivilegedPgPool,
		Close:   unprivilegedPgPool.Close,
	}

	return pool, unprivilegedPool
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
	sweepStaleDatabases(t, ctx, adminPool, time.Now())
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

func sweepStaleDatabases(t *testing.T, ctx context.Context, adminPool *pgxpool.Pool, now time.Time) {
	t.Helper()
	rows, err := adminPool.Query(ctx, `
		SELECT datname
		  FROM pg_database
		 WHERE datname LIKE 'striatum_pgtest_%'`)
	if err != nil {
		t.Logf("pgtest stale database sweep skipped: %v", err)
		return
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Logf("pgtest stale database sweep row skipped: %v", err)
			continue
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Logf("pgtest stale database sweep rows failed: %v", err)
		return
	}
	for _, name := range names {
		if !shouldSweepPgtestDatabase(name, now, staleDatabaseMaxAge) {
			continue
		}
		_, _ = adminPool.Exec(ctx, `
			SELECT pg_terminate_backend(pid)
			  FROM pg_stat_activity
			 WHERE datname = $1 AND pid <> pg_backend_pid()`, name)
		if _, err := adminPool.Exec(ctx, "DROP DATABASE IF EXISTS "+quoteIdent(name)); err != nil {
			t.Logf("pgtest stale database sweep could not drop %s: %v", name, err)
		}
	}
}

func shouldSweepPgtestDatabase(name string, now time.Time, maxAge time.Duration) bool {
	createdAt, pid, ok := parsePgtestDatabaseName(name)
	if !ok {
		return false
	}
	if maxAge > 0 && now.Sub(createdAt) > maxAge {
		return true
	}
	return !pidAlive(pid)
}

func parsePgtestDatabaseName(name string) (time.Time, int, bool) {
	match := pgtestDatabaseNameRE.FindStringSubmatch(name)
	if match == nil {
		return time.Time{}, 0, false
	}
	nanos, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil || nanos <= 0 {
		return time.Time{}, 0, false
	}
	pid64, err := strconv.ParseInt(match[2], 10, 64)
	if err != nil || pid64 <= 0 || pid64 > int64(^uint(0)>>1) {
		return time.Time{}, 0, false
	}
	return time.Unix(0, nanos), int(pid64), true
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	return false
}

// ensureRuntimeRole creates the canonical, cluster-wide striatumd_rw role if it
// is absent, so migration 0005 provisions its grant surface during migrate (RFC
// 0110 §10). It is idempotent and tolerates a concurrent creator (parallel test
// packages share the one cluster role); it never drops striatumd_rw.
func ensureRuntimeRole(t *testing.T, ctx context.Context, baseURL string) {
	t.Helper()
	adminPool, err := pgxpool.New(ctx, baseURL)
	if err != nil {
		t.Fatalf("connect to ensure runtime role: %v", err)
	}
	defer adminPool.Close()
	// A concurrent creator is benign, but it surfaces two different SQLSTATEs and
	// we must swallow both. CREATE ROLE on an already-present role raises
	// duplicate_object (42710); but the IF NOT EXISTS check is TOCTOU, so when two
	// packages both pass the pg_roles probe and both issue CREATE ROLE, the loser's
	// pg_authid insert collides on pg_authid_rolname_index and raises
	// unique_violation (23505) instead. Catching only 42710 let 23505 escape and
	// fail parallel packages intermittently. It never drops striatumd_rw.
	if _, err := adminPool.Exec(ctx, `
		DO $$
		BEGIN
		  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
		    CREATE ROLE striatumd_rw;
		  END IF;
		EXCEPTION WHEN duplicate_object OR unique_violation THEN
		  NULL;
		END
		$$;`); err != nil {
		t.Fatalf("ensure striatumd_rw role: %v", err)
	}
}

// roleSetupStatements returns the per-test login-role setup, kept as a pure
// statement list so the G-PGTEST-GRANTS guard (TestRoleSetupIssuesNoProtectedDML)
// can assert it issues NO GRANT/REVOKE naming a protected append-only table — the
// per-test role derives its DML surface from striatumd_rw membership, and that
// surface comes from migration 0005, never from imperative pgtest grants (RFC
// 0110 §10, C-PGTEST-NO-DML-GRANT). The schema/sequence USAGE grants to
// striatumd_rw mirror what `daemon doctor --provision-rw-role` adds beyond the
// migration's table DML; they name no protected table.
func roleSetupStatements(dbName, roleName, currentUser string) []string {
	return []string{
		fmt.Sprintf("DROP ROLE IF EXISTS %s", quoteIdent(roleName)),
		fmt.Sprintf("CREATE ROLE %s", quoteIdent(roleName)),
		fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", quoteIdent(dbName), quoteIdent(roleName)),
		"GRANT USAGE ON SCHEMA striatumd TO striatumd_rw",
		"GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA striatumd TO striatumd_rw",
		fmt.Sprintf("GRANT striatumd_rw TO %s", quoteIdent(roleName)),
		fmt.Sprintf("GRANT %s TO %s", quoteIdent(roleName), quoteIdent(currentUser)),
	}
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
