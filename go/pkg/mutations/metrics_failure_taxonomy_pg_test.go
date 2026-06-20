package mutations

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/metrics"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// driveRunLiveness runs the REAL liveness refresh (the path that emits
// session.liveness_deadline_missed / session.liveness_recovered) over a run, in
// its own committed transaction, exactly as the recovery sweep does.
func driveRunLiveness(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID string) {
	t.Helper()
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return refreshRunLiveness(ctx, tx, repoID, runID, false)
	}); err != nil {
		t.Fatalf("refreshRunLiveness: %v", err)
	}
}

// foldAndRenderMetrics folds the REAL metrics collector over the live DB and
// returns the rendered Prometheus exposition — the operator-visible surface.
func foldAndRenderMetrics(t *testing.T, ctx context.Context, runner db.Runner, now time.Time) string {
	t.Helper()
	// The collector folds through metrics.Querier — the same concrete runner the
	// daemon type-asserts to at startup (cmd/striatumd/main.go).
	q, ok := runner.(metrics.Querier)
	if !ok {
		t.Fatalf("runner %T does not implement metrics.Querier", runner)
	}
	if err := metrics.NewCollector(q).Refresh(ctx, now); err != nil {
		t.Fatalf("metrics refresh: %v", err)
	}
	snap := metrics.Load()
	if snap == nil {
		t.Fatal("no metrics snapshot was published")
	}
	var buf bytes.Buffer
	if err := snap.WriteText(&buf, now); err != nil {
		t.Fatalf("render metrics: %v", err)
	}
	return buf.String()
}

func requireMetricLine(t *testing.T, body, line string) {
	t.Helper()
	if !strings.Contains(body, line) {
		t.Errorf("expected metric line %q in:\n%s", line, body)
	}
}

// TestLivenessMissCanRecoverWithoutNecrosis is the RFC 0137 F-A6 behavioral
// contract (prior-review F3 required this to drive the REAL path, not fold
// hardcoded event literals). It seeds an active-but-stalled session, drives the
// real refreshRunLiveness through active -> liveness_deadline_missed ->
// liveness_recovered, then folds the real metrics collector and asserts on the
// wire surface that the reversible pair moved striatum_liveness_deadline_events_total
// while striatum_necrosis_total never moved. A recoverable stall is a pre-death
// observation, not death, so it stays out of the apoptosis/necrosis conservation
// law entirely.
func TestLivenessMissCanRecoverWithoutNecrosis(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_fa6_liveness_metrics"

	runID, _, _, _, sessionID := seedStalledSessionActiveLane(t, ctx, runner, repoID)

	// active -> deadline_missed: the seeded session's activity is well past the
	// protocol deadlines, so the real refresh classifies a stall and emits
	// session.liveness_deadline_missed (recovery.go).
	driveRunLiveness(t, ctx, runner, repoID, runID)
	if got := eventCount(t, ctx, runner, repoID, runID, "session.liveness_deadline_missed"); got != 1 {
		t.Fatalf("session.liveness_deadline_missed events = %d, want 1 (real refresh did not classify a stall)", got)
	}

	// -> recovered: make the session's protocol activity fresh so the stall
	// clears, then refresh again; the stored stall clears and the real refresh
	// emits session.liveness_recovered. Only the genuine progress signals are
	// touched — deliberately NOT last_session_question_at / last_session_escalate_at,
	// which a fresh value would read as a NEW pending-attention stall.
	fresh := time.Now().UTC()
	if err := runner.Exec(ctx, `
		UPDATE striatumd.sessions
		   SET last_mcp_request_at = $3, last_work_heartbeat_at = $3,
		       last_session_heartbeat_at = $3
		 WHERE repository_id = $1 AND session_id = $2`, repoID, sessionID, fresh); err != nil {
		t.Fatalf("refresh session activity: %v", err)
	}
	driveRunLiveness(t, ctx, runner, repoID, runID)
	if got := eventCount(t, ctx, runner, repoID, runID, "session.liveness_recovered"); got != 1 {
		t.Fatalf("session.liveness_recovered events = %d, want 1 (real refresh did not clear the stall)", got)
	}

	// Fold the real collector and assert on the operator-visible wire surface.
	body := foldAndRenderMetrics(t, ctx, runner, time.Now().UTC())
	requireMetricLine(t, body, `striatum_liveness_deadline_events_total{reason="deadline_missed"} 1`)
	requireMetricLine(t, body, `striatum_liveness_deadline_events_total{reason="recovered"} 1`)
	// necrosis renders only OBSERVED (nonzero) tuples, so a recoverable stall must
	// leave the family with NO series at all — the load-bearing F-A6 assertion.
	if strings.Contains(body, "striatum_necrosis_total{") {
		t.Errorf("F-A6 violated: a necrosis series is present after a recoverable liveness miss:\n%s", body)
	}
}

// TestRunWedgeAgeCountsOnlyJobStateAdvances is the RFC 0137 F1 behavioral
// regression (prior-review F1): a wedged run's age must be measured from its last
// real job-state advance, never reset by a later non-job event such as the
// per-tick daemon.recovery_sweep. It seeds a non-terminal run whose only job-state
// advance is 90 minutes old, then records a recovery-sweep tick 30 seconds ago,
// folds the real collector, and asserts the wedge age reflects the OLD job event
// (the low buckets stay empty) rather than the recent sweep.
func TestRunWedgeAgeCountsOnlyJobStateAdvances(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_wedge_age_job_advance"
	runID := "run_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"workflow_id": "wf"}) // state=running (non-terminal)

	now := time.Now().UTC()
	// The run's last real forward progress: a job-state advance 90 minutes ago.
	seedRunEventAt(t, ctx, runner, repoID, runID, "job.queued", now.Add(-90*time.Minute))
	// A run-scoped recovery-sweep tick 30 seconds ago — must NOT count as progress.
	seedRunEventAt(t, ctx, runner, repoID, runID, "daemon.recovery_sweep", now.Add(-30*time.Second))

	body := foldAndRenderMetrics(t, ctx, runner, now)

	// One observation (~5400s), measured from job.queued, not the 30s sweep: the
	// low buckets stay empty. Buggy (unfiltered) behavior would land it in le="60".
	requireMetricLine(t, body, `striatum_run_wedge_age_seconds_bucket{origin="daemon-core",le="60"} 0`)
	requireMetricLine(t, body, `striatum_run_wedge_age_seconds_bucket{origin="daemon-core",le="3600"} 0`)
	requireMetricLine(t, body, `striatum_run_wedge_age_seconds_count{origin="daemon-core"} 1`)
}

// TestApoptosisHandoffAndDrainFoldedFromDurableEvents proves the previously-hollow
// apoptosis reasons are now produced end-to-end by the real collector SQL from
// real durable events (prior-review F2): a lease.released carrying a transfer and
// a supervisor.stopped move lease_handoff / supervisor_drained, while a plain
// completion release does not, and necrosis stays untouched.
func TestApoptosisHandoffAndDrainFoldedFromDurableEvents(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_apoptosis_handoff_drain"
	runID := "run_" + repoID

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{"workflow_id": "wf"})

	now := time.Now().UTC()
	seedRunEventWithPayload(t, ctx, runner, repoID, runID, "lease.released",
		map[string]any{"reason": "operator_transfer", "transfer": true}, now)
	seedRunEventWithPayload(t, ctx, runner, repoID, runID, "lease.released",
		map[string]any{"reason": "completed"}, now) // not a handoff -> no apoptosis
	seedRunEventWithPayload(t, ctx, runner, repoID, runID, "supervisor.stopped",
		map[string]any{"reason": "operator stop"}, now)

	body := foldAndRenderMetrics(t, ctx, runner, now)
	requireMetricLine(t, body, `striatum_apoptosis_total{origin="lane",reason="lease_handoff"} 1`)
	requireMetricLine(t, body, `striatum_apoptosis_total{origin="supervisor",reason="supervisor_drained"} 1`)
	if strings.Contains(body, "striatum_necrosis_total{") {
		t.Errorf("a healthy handoff/drain produced a necrosis series:\n%s", body)
	}
}

// seedRunEventWithPayload inserts a durable event carrying a JSON payload at a
// chosen time — the payload-bearing companion to seedRunEventAt.
func seedRunEventWithPayload(t *testing.T, ctx context.Context, runner db.Runner, repoID, runID, eventType string, payload map[string]any, at time.Time) {
	t.Helper()
	payloadArg, err := db.JSONBArg(runner, payload)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.events (
		  repository_id, run_id, event_type, payload_json, created_at
		) VALUES ($1,$2,$3,$4::jsonb,$5)`, repoID, runID, eventType, payloadArg, at); err != nil {
		t.Fatalf("insert event %s: %v", eventType, err)
	}
}
