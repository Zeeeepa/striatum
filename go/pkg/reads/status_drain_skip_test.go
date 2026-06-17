package reads

import (
	"context"
	"errors"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/jackc/pgx/v5"
)

// drainSkipTx is a no-op transaction whose Exec (the SET LOCAL lock_timeout)
// always succeeds. Commit/Rollback are recorded so the test can assert the
// skip path rolls back rather than commits.
type drainSkipTx struct {
	committed  bool
	rolledBack bool
}

func (t *drainSkipTx) Exec(context.Context, string, ...any) error { return nil }
func (t *drainSkipTx) QueryRow(context.Context, string, ...any) db.Row {
	return nil
}
func (t *drainSkipTx) QueryScalar(context.Context, string, ...any) (string, error) { return "", nil }
func (t *drainSkipTx) Commit(context.Context) error {
	t.committed = true
	return nil
}
func (t *drainSkipTx) Rollback(context.Context) error {
	t.rolledBack = true
	return nil
}

type drainSkipRunner struct{ tx *drainSkipTx }

func (r *drainSkipRunner) Exec(context.Context, string, ...any) error { return nil }
func (r *drainSkipRunner) QueryRow(context.Context, string, ...any) db.Row {
	return nil
}
func (r *drainSkipRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}
func (r *drainSkipRunner) BeginTx(context.Context) (db.TxRunner, error) { return r.tx, nil }
func (r *drainSkipRunner) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return dashboardAllRowsFromMaps([]map[string]any{}), nil
}

type authorityPreludeRecordingTx struct {
	drainSkipTx
	operations []string
	secret     any
}

func (t *authorityPreludeRecordingTx) ExecBound(_ context.Context, _ string, args ...any) error {
	t.operations = append(t.operations, "authority_prelude")
	if len(args) > 0 {
		t.secret = args[0]
	}
	return nil
}

func (t *authorityPreludeRecordingTx) Exec(ctx context.Context, query string, args ...any) error {
	t.operations = append(t.operations, "exec")
	return t.drainSkipTx.Exec(ctx, query, args...)
}

func (t *authorityPreludeRecordingTx) Commit(ctx context.Context) error {
	t.operations = append(t.operations, "commit")
	return t.drainSkipTx.Commit(ctx)
}

type authorityPreludeRecordingRunner struct{ tx *authorityPreludeRecordingTx }

func (r *authorityPreludeRecordingRunner) Exec(context.Context, string, ...any) error { return nil }
func (r *authorityPreludeRecordingRunner) QueryRow(context.Context, string, ...any) db.Row {
	return nil
}
func (r *authorityPreludeRecordingRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", nil
}
func (r *authorityPreludeRecordingRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return r.tx, nil
}
func (r *authorityPreludeRecordingRunner) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return dashboardAllRowsFromMaps([]map[string]any{}), nil
}

// TestDrainStatusHelperEventsSkipsOnContention is the #193 latency-half
// regression: when the helper-event drain on the status READ path errors (the
// modeled lock_timeout / 55P03 under recovery contention), the status read must
// degrade gracefully — note helper_drain_skipped on the session row, roll the
// drain back, and NOT fail the read — instead of blocking for the statement
// timeout (the >30s spike in #193).
func TestDrainStatusHelperEventsSkipsOnContention(t *testing.T) {
	prevHook := DrainHelperEventsHook
	DrainHelperEventsHook = func(context.Context, db.TxRunner, string, string) error {
		return errors.New("canceling statement due to lock timeout")
	}
	t.Cleanup(func() { DrainHelperEventsHook = prevHook })

	tx := &drainSkipTx{}
	runner := &drainSkipRunner{tx: tx}
	row := map[string]any{"session_id": "sess_1"}

	drainStatusHelperEvents(context.Background(), runner, "repo_1", "sup_1", row)

	if row["helper_drain_skipped"] != true {
		t.Fatalf("expected helper_drain_skipped=true on contention; row=%#v", row)
	}
	if !tx.rolledBack {
		t.Fatalf("contended drain must roll the sub-transaction back")
	}
	if tx.committed {
		t.Fatalf("contended drain must NOT commit the sub-transaction")
	}
}

// TestDrainStatusHelperEventsCommitsOnSuccess is the companion happy-path: a
// successful drain commits and does not flag helper_drain_skipped.
func TestDrainStatusHelperEventsCommitsOnSuccess(t *testing.T) {
	prevHook := DrainHelperEventsHook
	DrainHelperEventsHook = func(context.Context, db.TxRunner, string, string) error { return nil }
	t.Cleanup(func() { DrainHelperEventsHook = prevHook })

	tx := &drainSkipTx{}
	runner := &drainSkipRunner{tx: tx}
	row := map[string]any{"session_id": "sess_1"}

	drainStatusHelperEvents(context.Background(), runner, "repo_1", "sup_1", row)

	if _, skipped := row["helper_drain_skipped"]; skipped {
		t.Fatalf("successful drain must not flag helper_drain_skipped; row=%#v", row)
	}
	if !tx.committed {
		t.Fatalf("successful drain must commit the sub-transaction")
	}
}

// TestDrainStatusHelperEventsInstallsAuthorityPrelude is the #329 regression:
// read-triggered helper-event drains call the same event append SD path as
// mutation handlers, so they must open an authorized mutation transaction before
// DrainHelperEventsHook writes supervisor.progress events.
func TestDrainStatusHelperEventsInstallsAuthorityPrelude(t *testing.T) {
	db.SetAuthorityRuntime("test-daemon-secret", db.AuditHashFormatV3, "", false)
	t.Cleanup(func() { db.SetAuthorityRuntime("", db.AuditHashFormatV2, "", false) })

	prevHook := DrainHelperEventsHook
	DrainHelperEventsHook = func(_ context.Context, runner db.TxRunner, _, _ string) error {
		if recordingTx, ok := runner.(*authorityPreludeRecordingTx); ok {
			recordingTx.operations = append(recordingTx.operations, "drain")
		}
		return nil
	}
	t.Cleanup(func() { DrainHelperEventsHook = prevHook })

	tx := &authorityPreludeRecordingTx{}
	runner := &authorityPreludeRecordingRunner{tx: tx}
	row := map[string]any{"session_id": "sess_1"}

	drainStatusHelperEvents(context.Background(), runner, "repo_1", "sup_1", row)

	if tx.secret != "test-daemon-secret" {
		t.Fatalf("authority prelude secret = %v, want process daemon secret", tx.secret)
	}
	if len(tx.operations) < 4 {
		t.Fatalf("operations = %#v, want authority prelude before drain setup, hook, and commit", tx.operations)
	}
	wantPrefix := []string{"authority_prelude", "exec", "drain", "commit"}
	for i, want := range wantPrefix {
		if tx.operations[i] != want {
			t.Fatalf("operations = %#v, want prefix %#v", tx.operations, wantPrefix)
		}
	}
}
