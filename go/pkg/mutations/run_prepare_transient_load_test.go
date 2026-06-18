package mutations

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestWithTxRetryOnTransientLoadRetriesThenSucceeds is the #355 victim-side
// mitigation regression. Under a multi-run supervise storm a plain control-plane
// append (run.prepare's `run.created`) is starved behind a lock-holding
// sweep/reconcile transaction and dies at statement_timeout as SQLSTATE 57014.
// That is transient back-pressure, not a real refusal, so the wrapper must retry
// it and let it succeed once the convoy clears — instead of hard-failing the
// operator with a raw SQL error.
func TestWithTxRetryOnTransientLoadRetriesThenSucceeds(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner

	timeout57014 := &pgconn.PgError{Code: statementTimeoutSQLState, Message: "canceling statement due to statement timeout"}
	var attempts int
	result, err := withTxRetryOnTransientLoad(ctx, runner, func(db.TxRunner) (map[string]any, error) {
		attempts++
		// Fail transiently on the first two attempts, then succeed — modeling the
		// convoy clearing between bounded retries.
		if attempts < 3 {
			return nil, timeout57014
		}
		return map[string]any{"ok": true}, nil
	})
	if err != nil {
		t.Fatalf("withTxRetryOnTransientLoad returned error after the transient back-pressure cleared: %v", err)
	}
	if result["ok"] != true {
		t.Fatalf("result = %#v, want {ok:true}", result)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3 (two transient failures then success)", attempts)
	}
}

// TestWithTxRetryOnTransientLoadSurfacesLegibleErrorAfterBudget asserts that
// sustained back-pressure (every bounded attempt times out) surfaces a legible
// `daemon_under_load` error carrying the SQLSTATE, NOT the raw 57014 — directly
// answering #355's "reads as a hard, unexplained failure" complaint.
func TestWithTxRetryOnTransientLoadSurfacesLegibleErrorAfterBudget(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner

	timeout57014 := &pgconn.PgError{Code: statementTimeoutSQLState, Message: "canceling statement due to statement timeout"}
	var attempts int
	start := time.Now()
	_, err := withTxRetryOnTransientLoad(ctx, runner, func(db.TxRunner) (map[string]any, error) {
		attempts++
		return nil, timeout57014
	})
	if err == nil {
		t.Fatal("expected an error after the retry budget was exhausted")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error is not an rpc.Error: %v", err)
	}
	if rpcErr.Code != "daemon_under_load" {
		t.Fatalf("error code = %q, want daemon_under_load (the raw 57014 must be translated)", rpcErr.Code)
	}
	if rpcErr.Details["sqlstate"] != statementTimeoutSQLState {
		t.Fatalf("error details sqlstate = %v, want %q", rpcErr.Details["sqlstate"], statementTimeoutSQLState)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want the wrapper to retry (bounded) before giving up", attempts)
	}
	// Sanity: the bounded backoff must not block for an unreasonable time.
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("retry budget took %v — backoff is unbounded", elapsed)
	}
}

// TestWithTxRetryOnTransientLoadReturnsNonTransientImmediately asserts a real
// transition refusal (a non-57-class error) is returned immediately, with no
// retry — the wrapper only swallows transient daemon-load back-pressure.
func TestWithTxRetryOnTransientLoadReturnsNonTransientImmediately(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner

	refusal := rpc.NewError("invalid_transition", "real refusal", nil)
	var attempts int
	_, err := withTxRetryOnTransientLoad(ctx, runner, func(db.TxRunner) (map[string]any, error) {
		attempts++
		return nil, refusal
	})
	if err == nil {
		t.Fatal("expected the non-transient refusal to be returned")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
		t.Fatalf("error = %v, want the original invalid_transition refusal unchanged", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (a non-transient error must NOT be retried)", attempts)
	}
}

// TestWithTxRetryOnTransientLoadRetriesLockTimeout is the #389 gap-1 / #383 item-3
// regression for the transient-load wrapper (run.prepare path): a transient
// lock_timeout (SQLSTATE 55P03) — a writer that lost a race for a contended lock
// under multi-run state-DB load — must be retried like a statement timeout, not
// surfaced raw. The convoy clears between bounded attempts and the verb succeeds.
func TestWithTxRetryOnTransientLoadRetriesLockTimeout(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner

	lockTimeout55P03 := &pgconn.PgError{Code: lockTimeoutSQLState, Message: "canceling statement due to lock timeout"}
	var attempts int
	result, err := withTxRetryOnTransientLoad(ctx, runner, func(db.TxRunner) (map[string]any, error) {
		attempts++
		if attempts < 3 {
			return nil, lockTimeout55P03
		}
		return map[string]any{"ok": true}, nil
	})
	if err != nil {
		t.Fatalf("withTxRetryOnTransientLoad returned error after the transient lock_timeout cleared: %v", err)
	}
	if result["ok"] != true {
		t.Fatalf("result = %#v, want {ok:true}", result)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3 (two transient lock_timeouts then success)", attempts)
	}
}

// TestWithTxRetryOnDeadlockRetriesLockTimeout asserts the per-run lifecycle
// wrapper (artifact.publish/seal, review.submit, run.retry_job, interrogation,
// supervision-control) also self-heals a transient lock_timeout (#389 gap 1 /
// #383 item 3): a 55P03 victim retries within the bounded budget and succeeds
// once the lock contention clears, instead of consuming the job's only attempt.
func TestWithTxRetryOnDeadlockRetriesLockTimeout(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner

	lockTimeout55P03 := &pgconn.PgError{Code: lockTimeoutSQLState, Message: "canceling statement due to lock timeout"}
	var attempts int
	result, err := withTxRetryOnDeadlock(ctx, runner, func(db.TxRunner) (map[string]any, error) {
		attempts++
		if attempts < 2 {
			return nil, lockTimeout55P03
		}
		return map[string]any{"ok": true}, nil
	})
	if err != nil {
		t.Fatalf("withTxRetryOnDeadlock returned error after the transient lock_timeout cleared: %v", err)
	}
	if result["ok"] != true {
		t.Fatalf("result = %#v, want {ok:true}", result)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2 (one transient lock_timeout then success)", attempts)
	}
}

// TestWithTxRetryOnDeadlockSurfacesDaemonUnderLoadOnLockTimeoutBudget asserts that
// when a lock_timeout (55P03) persists through the bounded budget, the per-run
// lifecycle wrapper surfaces the legible `daemon_under_load` error carrying the
// SQLSTATE — NOT the deadlock-specific invalid_transition and NOT the raw 55P03.
func TestWithTxRetryOnDeadlockSurfacesDaemonUnderLoadOnLockTimeoutBudget(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner

	lockTimeout55P03 := &pgconn.PgError{Code: lockTimeoutSQLState, Message: "canceling statement due to lock timeout"}
	var attempts int
	start := time.Now()
	_, err := withTxRetryOnDeadlock(ctx, runner, func(db.TxRunner) (map[string]any, error) {
		attempts++
		return nil, lockTimeout55P03
	})
	if err == nil {
		t.Fatal("expected an error after the lock_timeout retry budget was exhausted")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error is not an rpc.Error: %v", err)
	}
	if rpcErr.Code != "daemon_under_load" {
		t.Fatalf("error code = %q, want daemon_under_load (the raw 55P03 must be translated, not deadlock invalid_transition)", rpcErr.Code)
	}
	if rpcErr.Details["sqlstate"] != lockTimeoutSQLState {
		t.Fatalf("error details sqlstate = %v, want %q", rpcErr.Details["sqlstate"], lockTimeoutSQLState)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want the wrapper to retry (bounded) before giving up", attempts)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("retry budget took %v — backoff is unbounded", elapsed)
	}
}

// TestWithTxRetryOnDeadlockStillSurfacesDeadlockGuidance pins that folding the
// transient-load class into withTxRetryOnDeadlock did NOT change deadlock
// handling: a sustained 40P01 still exhausts to the serial-retry
// `invalid_transition` error, not daemon_under_load.
func TestWithTxRetryOnDeadlockStillSurfacesDeadlockGuidance(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner

	deadlock40P01 := &pgconn.PgError{Code: deadlockSQLState, Message: "deadlock detected"}
	var attempts int
	_, err := withTxRetryOnDeadlock(ctx, runner, func(db.TxRunner) (map[string]any, error) {
		attempts++
		return nil, deadlock40P01
	})
	if err == nil {
		t.Fatal("expected an error after the deadlock retry budget was exhausted")
	}
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error is not an rpc.Error: %v", err)
	}
	if rpcErr.Code != "invalid_transition" {
		t.Fatalf("error code = %q, want invalid_transition (deadlock guidance must be unchanged)", rpcErr.Code)
	}
	if rpcErr.Details["sqlstate"] != deadlockSQLState {
		t.Fatalf("error details sqlstate = %v, want %q", rpcErr.Details["sqlstate"], deadlockSQLState)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want the wrapper to retry (bounded) before giving up", attempts)
	}
}
