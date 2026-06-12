package mutations

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/jackc/pgx/v5"
)

// RFC 0104 guard: assert that every per-run mutation transaction takes the
// per-run advisory lock (pg_advisory_xact_lock, via lockRun) BEFORE any blocking
// FOR UPDATE on a run-scoped table. The deadlock A1 (TestRunLockClaimVerdict-
// SweepDeadlock) proves the lock works at runtime; this guard proves the
// *discipline* — a handler that grows a new run-scoped FOR UPDATE without taking
// lockRun first re-opens the {sessions, runs} cycle, and this test fails.

// runScopedTables are the run-aggregate tables the RFC 0104 invariant covers. A
// blocking FOR UPDATE on any of them must be serialized behind lockRun.
var runScopedTables = []string{
	"striatumd.sessions", "striatumd.runs", "striatumd.jobs", "striatumd.leases",
	"striatumd.queue_messages", "striatumd.verdicts", "striatumd.blockers",
	"striatumd.interrogations",
}

// blockingRunScopedForUpdate reports whether sql is a blocking FOR UPDATE on a
// run-scoped table. SKIP LOCKED is exempt: per the RFC 0104 non-goals the claim
// queue's `FOR UPDATE OF qm SKIP LOCKED` is already deadlock-safe (it skips
// locked rows rather than blocking on them).
func blockingRunScopedForUpdate(sql string) bool {
	if !strings.Contains(sql, "FOR UPDATE") || strings.Contains(sql, "SKIP LOCKED") {
		return false
	}
	for _, tbl := range runScopedTables {
		if strings.Contains(sql, tbl) {
			return true
		}
	}
	return false
}

// checkLockRunFirst is the pure invariant check: for every recorded transaction,
// if it runs a blocking run-scoped FOR UPDATE then a pg_advisory_xact_lock must
// appear earlier in that same transaction. Returns a descriptive error on the
// first violation, nil when the invariant holds.
func checkLockRunFirst(txLogs [][]string) error {
	for txIdx, log := range txLogs {
		advisoryIdx := -1
		for i, sql := range log {
			if strings.Contains(sql, "pg_advisory_xact_lock") {
				advisoryIdx = i
				break
			}
		}
		for i, sql := range log {
			if !blockingRunScopedForUpdate(sql) {
				continue
			}
			if advisoryIdx == -1 {
				return fmt.Errorf("tx %d ran a blocking run-scoped FOR UPDATE without ever taking lockRun: %s", txIdx, squashSQL(sql))
			}
			if advisoryIdx > i {
				return fmt.Errorf("tx %d ran a blocking run-scoped FOR UPDATE before lockRun: %s", txIdx, squashSQL(sql))
			}
			break // the first FOR UPDATE in this tx is enough to validate ordering
		}
	}
	return nil
}

func squashSQL(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}

// --- SQL-order recording runner --------------------------------------------

// lockRecorder wraps a db.Runner and records the SQL executed inside each
// transaction it opens (one log per BeginTx), so checkLockRunFirst can assert
// lockRun is taken before any run-scoped FOR UPDATE. Reads/Execs outside a
// transaction are passed through untracked (per-run mutations always use a tx).
type lockRecorder struct {
	inner  db.Runner
	txLogs *[][]string
}

func newLockRecorder(inner db.Runner) *lockRecorder {
	logs := &[][]string{}
	return &lockRecorder{inner: inner, txLogs: logs}
}

func (r *lockRecorder) Exec(ctx context.Context, sql string, args ...any) error {
	return r.inner.Exec(ctx, sql, args...)
}
func (r *lockRecorder) QueryRow(ctx context.Context, sql string, args ...any) db.Row {
	return r.inner.QueryRow(ctx, sql, args...)
}
func (r *lockRecorder) QueryScalar(ctx context.Context, sql string, args ...any) (string, error) {
	return r.inner.QueryScalar(ctx, sql, args...)
}
func (r *lockRecorder) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return r.inner.(queryer).Query(ctx, sql, args...)
}

// EncodesJSONBAsText opts the recorder into db.JSONBArg's pgx text encoding,
// since it delegates to a real pgx runner.
func (r *lockRecorder) EncodesJSONBAsText() bool { return true }

func (r *lockRecorder) BeginTx(ctx context.Context) (db.TxRunner, error) {
	tx, err := r.inner.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	*r.txLogs = append(*r.txLogs, nil)
	return &recordingTx{inner: tx, txLogs: r.txLogs, idx: len(*r.txLogs) - 1}, nil
}

type recordingTx struct {
	inner  db.TxRunner
	txLogs *[][]string
	idx    int
}

func (t *recordingTx) record(sql string) {
	(*t.txLogs)[t.idx] = append((*t.txLogs)[t.idx], sql)
}
func (t *recordingTx) Exec(ctx context.Context, sql string, args ...any) error {
	t.record(sql)
	return t.inner.Exec(ctx, sql, args...)
}
func (t *recordingTx) QueryRow(ctx context.Context, sql string, args ...any) db.Row {
	t.record(sql)
	return t.inner.QueryRow(ctx, sql, args...)
}
func (t *recordingTx) QueryScalar(ctx context.Context, sql string, args ...any) (string, error) {
	t.record(sql)
	return t.inner.QueryScalar(ctx, sql, args...)
}
func (t *recordingTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.record(sql)
	return t.inner.(queryer).Query(ctx, sql, args...)
}
func (t *recordingTx) Commit(ctx context.Context) error   { return t.inner.Commit(ctx) }
func (t *recordingTx) Rollback(ctx context.Context) error { return t.inner.Rollback(ctx) }
func (t *recordingTx) EncodesJSONBAsText() bool           { return true }

// --- the teeth: checkLockRunFirst detects violations (hermetic) -------------

func TestRunLockGuardLogicDetectsViolations(t *testing.T) {
	good := [][]string{{
		`SELECT run_id FROM striatumd.sessions WHERE session_id = $1`,
		`SELECT pg_advisory_xact_lock(hashtext($1))`,
		`SELECT * FROM striatumd.sessions WHERE session_id = $2 FOR UPDATE`,
		`SELECT * FROM striatumd.runs WHERE run_id = $2 FOR UPDATE`,
	}}
	if err := checkLockRunFirst(good); err != nil {
		t.Fatalf("good tx flagged as a violation: %v", err)
	}

	missingLock := [][]string{{
		`SELECT * FROM striatumd.runs WHERE run_id = $1 FOR UPDATE`,
	}}
	if err := checkLockRunFirst(missingLock); err == nil {
		t.Fatal("a run-scoped FOR UPDATE with NO lockRun was not flagged (guard has no teeth)")
	}

	lockTooLate := [][]string{{
		`SELECT * FROM striatumd.jobs WHERE job_id = $1 FOR UPDATE`,
		`SELECT pg_advisory_xact_lock(hashtext($1))`,
	}}
	if err := checkLockRunFirst(lockTooLate); err == nil {
		t.Fatal("a lockRun taken AFTER a run-scoped FOR UPDATE was not flagged")
	}

	// SKIP LOCKED queue claims are exempt and must NOT be flagged even with no lock.
	skipLocked := [][]string{{
		`SELECT qm.* FROM striatumd.queue_messages qm ... FOR UPDATE OF qm SKIP LOCKED`,
	}}
	if err := checkLockRunFirst(skipLocked); err != nil {
		t.Fatalf("a SKIP LOCKED queue claim was wrongly flagged: %v", err)
	}
}

// --- per-run handlers take lockRun first (PG-gated) -------------------------

// TestPerRunHandlersTakeLockRunFirst drives representative per-run mutation
// handlers through the SQL-recording runner and asserts each takes lockRun
// before any blocking run-scoped FOR UPDATE — the RFC 0104 acceptance guard.
func TestPerRunHandlersTakeLockRunFirst(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_runlock_guard"
	fx := seedRunLockRaceFixture(t, ctx, runner, repoID)

	cases := []struct {
		name  string
		drive func(rec *lockRecorder)
	}{
		{"work.claim_next", func(rec *lockRecorder) {
			_, _ = HandleClaimNext(ctx, rec, intgEnv(repoID, map[string]any{"session_id": fx.claimantSession}))
		}},
		{"review.verdict", func(rec *lockRecorder) {
			if _, err := HandleRecordVerdict(ctx, rec, intgEnv(repoID, map[string]any{
				"session_id": fx.reviewerSession, "job_id": fx.reviewJob, "lease_id": fx.reviewLease, "verdict": "accept",
			})); err != nil {
				t.Fatalf("review.verdict: %v", err)
			}
		}},
		{"run.cancel", func(rec *lockRecorder) {
			if _, err := HandleRunCancel(ctx, rec, intgEnv(repoID, map[string]any{"run_id": fx.runID})); err != nil {
				t.Fatalf("run.cancel: %v", err)
			}
		}},
		{"session.register", func(rec *lockRecorder) {
			_, _ = HandleRegisterSession(ctx, rec, intgEnv(repoID, map[string]any{
				"run_id": fx.runID,
				"role":   "builder",
				"lane":   "claude",
			}))
		}},
		{"recovery.sweep", func(rec *lockRecorder) {
			if _, err := SweepRun(ctx, rec, repoID, fx.runID, "guard-test"); err != nil {
				t.Fatalf("recovery.sweep: %v", err)
			}
		}},
	}

	for _, tc := range cases {
		resetRunLockRace(t, ctx, runner, fx)
		rec := newLockRecorder(runner)
		tc.drive(rec)
		if err := checkLockRunFirst(*rec.txLogs); err != nil {
			t.Fatalf("%s violates the RFC 0104 per-run lock invariant: %v", tc.name, err)
		}
	}
}
