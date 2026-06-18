package mutations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestAwaitPacketSwallowsStatementTimeoutAsTransient guards #197: under
// concurrent multi-run claim contention a claim query can be canceled for
// exceeding statement_timeout (SQLSTATE 57014). Before the fix HandleAwaitPacket
// propagated that raw, so the lane saw
// `ERROR: canceling statement due to statement timeout (SQLSTATE 57014)`, read it
// as a hard failure, and gave up its receive loop — orphaning the job. The await
// loop must instead treat it as transient daemon load: poll again, and at the
// deadline return a legible no_work envelope with reason=transient_daemon_load,
// never a raw SQL error.
func TestAwaitPacketSwallowsStatementTimeoutAsTransient(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_await_57014"
	runID := "run_await_57014"
	sessionID := "sess_await_57014"

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		"roles":       map[string]any{"worker": map[string]any{}},
		"lanes":       map[string]any{"claude": map[string]any{"display_model": "Claude"}},
	})
	intgSeedSession(t, ctx, runner, repoID, runID, sessionID, "worker", "claude", nil, "active")
	intgAttest(t, ctx, runner, repoID, runID, sessionID, "claude")

	// Inject a Postgres statement-timeout error from the claim step on every
	// attempt, and shrink the loop so the deadline path runs fast.
	timeout57014 := &pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout"}
	var calls int
	restoreClaim := awaitClaimNext
	restoreTimeout := awaitPacketTimeout
	restorePoll := awaitPacketPollInterval
	awaitClaimNext = func(context.Context, db.Runner, rpc.Envelope) (map[string]any, error) {
		calls++
		return nil, timeout57014
	}
	awaitPacketTimeout = 40 * time.Millisecond
	awaitPacketPollInterval = 5 * time.Millisecond
	t.Cleanup(func() {
		awaitClaimNext = restoreClaim
		awaitPacketTimeout = restoreTimeout
		awaitPacketPollInterval = restorePoll
	})

	res, err := HandleAwaitPacket(ctx, runner, intgEnv(repoID, map[string]any{"session_id": sessionID}))
	if err != nil {
		t.Fatalf("await_packet propagated an error for a transient 57014 (must be swallowed): %v", err)
	}
	if got := res["status"]; got != "no_work" {
		t.Fatalf("status = %v, want no_work", got)
	}
	if got := res["reason"]; got != "transient_daemon_load" {
		t.Fatalf("reason = %v, want transient_daemon_load", got)
	}
	if calls < 1 {
		t.Fatalf("claim step was never called (%d) — the loop did not poll", calls)
	}
}

// TestIsTransientDaemonLoadError pins the SQLSTATE classification: 57014 and the
// class-57 connection-teardown codes are transient; everything else (deadlock,
// unique violation, a plain error) is not.
func TestIsTransientDaemonLoadError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"statement_timeout_57014", &pgconn.PgError{Code: "57014"}, true},
		{"lock_timeout_55P03", &pgconn.PgError{Code: "55P03"}, true},
		{"lock_timeout_55P03_wrapped", fmt.Errorf("append_event_row (sd): %w", &pgconn.PgError{Code: "55P03"}), true},
		{"admin_shutdown_57P01", &pgconn.PgError{Code: "57P01"}, true},
		{"crash_shutdown_57P02", &pgconn.PgError{Code: "57P02"}, true},
		{"cannot_connect_now_57P03", &pgconn.PgError{Code: "57P03"}, true},
		{"deadlock_40P01_not_transient_load", &pgconn.PgError{Code: "40P01"}, false},
		{"unique_violation_23505", &pgconn.PgError{Code: "23505"}, false},
		{"plain_error", context.DeadlineExceeded, false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTransientDaemonLoadError(tc.err); got != tc.want {
				t.Fatalf("isTransientDaemonLoadError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
