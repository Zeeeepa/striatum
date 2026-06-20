package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/halbritt/striatum/go/pkg/sessionliveness"
	"github.com/jackc/pgx/v5"
)

// Querier is the minimal multi-row DB surface the sweep-tick fold needs. It is
// satisfied by *db.PgxRunner — the same concrete type the recovery sweep
// type-asserts to (go/pkg/recovery/sweep.go). It is used ONLY by
// Collector.Refresh, never by the scrape path, which the zero-DB-query test
// enforces by handing the collector a Querier that panics on use and asserting a
// scrape never trips it.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Collector folds and publishes the metrics snapshot from the recovery-sweep
// tick and serves it on /metrics. It holds the daemon runner solely for the
// tick-time fold (Refresh); the scrape path (Handler) reads the published atomic
// pointer and never touches the runner.
type Collector struct {
	runner Querier
}

// NewCollector builds a collector over the daemon runner. A nil runner yields a
// collector whose Refresh is a no-op error and whose Handler serves an empty but
// valid surface — used by call sites (and tests) that only exercise the scrape
// path.
func NewCollector(runner Querier) *Collector {
	return &Collector{runner: runner}
}

// Refresh folds a fresh snapshot from the daemon DB and publishes it. It runs at
// the recovery-sweep cadence (default 60s), NOT on the scrape path: it issues a
// small fixed number of aggregate queries regardless of run/job count, so it can
// never become the per-scrape self-DoS the RFC warns against. The caller folds
// once per tick and treats any error as non-fatal (metrics are observational —
// the last-good snapshot keeps serving).
//
// The Phase A folds (run-state counts, stranded supervisors) are load-bearing
// and abort the refresh on error. The Phase B folds (the failure-mode families)
// are BEST-EFFORT: a Phase B query error degrades that one family to empty for
// this snapshot rather than blocking the whole surface, so a taxonomy fold bug
// can never take down the Phase A operational gauges.
func (c *Collector) Refresh(ctx context.Context, now time.Time) error {
	if c.runner == nil {
		return fmt.Errorf("metrics fold requires a daemon runner")
	}
	at := now.UTC()
	rawCounts, err := c.runStateCounts(ctx)
	if err != nil {
		return fmt.Errorf("fold run-state counts: %w", err)
	}
	stranded, err := c.strandedSupervisorCount(ctx)
	if err != nil {
		return fmt.Errorf("fold stranded-supervisor count: %w", err)
	}

	in := SnapshotInput{
		BuiltAt:             at,
		RawRunStateCounts:   rawCounts,
		StrandedSupervisors: stranded,
	}
	// Phase B: best-effort. Errors leave the corresponding family empty.
	in.EventCounts, _ = c.lifecycleEventCounts(ctx)
	in.LeaseTransitionCounts, _ = c.leaseTransitionCounts(ctx)
	in.WedgeAges, _ = c.runWedgeAges(ctx, at)
	in.LivenessMargins, _ = c.livenessMargins(ctx, at)

	Publish(Build(in))
	return nil
}

// runStateCounts aggregates runs by lifecycle state across the daemon-owned DB.
// It selects only the closed-enum state column — never a repo path, branch, sha,
// prompt, or byline — so there is nothing sensitive to leak into a label.
func (c *Collector) runStateCounts(ctx context.Context) (map[string]int, error) {
	rows, err := c.runner.Query(ctx, `
		SELECT state, COUNT(*)::bigint
		  FROM striatumd.runs
		 GROUP BY state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var state string
		var n int64
		if err := rows.Scan(&state, &n); err != nil {
			return nil, err
		}
		counts[state] = int(n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

// strandedSupervisorCount counts process_supervisors still 'attached' to a
// terminal run — the RFC 0137 #417 phantom-supervisor signal, the exact shape
// the status read path LEFT-JOINs and then probes (see
// go/pkg/db/sql/0033_reap_terminal_run_supervisors.sql).
func (c *Collector) strandedSupervisorCount(ctx context.Context) (int, error) {
	rows, err := c.runner.Query(ctx, `
		SELECT COUNT(*)::bigint
		  FROM striatumd.process_supervisors ps
		  JOIN striatumd.runs r
		    ON r.repository_id = ps.repository_id
		   AND r.run_id = ps.run_id
		 WHERE ps.state = 'attached'
		   AND r.state IN ('completed', 'failed', 'canceled')`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var count int64
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, err
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return int(count), nil
}

// lifecycleEventCounts folds the apoptosis / necrosis / liveness counters from
// the DURABLE striatumd.events ledger. It selects ONLY the closed-enum
// classification fields (event_type and the stall_class / blocker_kind /
// lifecycle_metric payload tags) — never a run/job/session id, lane, role, path,
// or any free text — and GROUP BYs so it transfers one row per distinct
// classification, not one per event. Folding from the immutable append-only
// ledger is what makes the counters tx-safe (a rolled-back lifecycle transaction
// never wrote the event) and restart-consistent (the counter is re-derived from
// durable history, not reset to zero) — RFC 0137 §"Design guidance".
func (c *Collector) lifecycleEventCounts(ctx context.Context) ([]EventCount, error) {
	rows, err := c.runner.Query(ctx, `
		SELECT event_type,
		       COALESCE(payload_json->>'stall_class','')      AS stall_class,
		       COALESCE(payload_json->>'blocker_kind','')     AS blocker_kind,
		       COALESCE(payload_json->>'lifecycle_metric','') AS lifecycle_metric,
		       COUNT(*)::bigint AS n
		  FROM striatumd.events
		 WHERE event_type IN (
		       'run.completed', 'job.completed', 'session.closed',
		       'session.liveness_deadline_missed', 'session.liveness_recovered',
		       'run.escalated', 'recovery.job_quarantined')
		 GROUP BY 1, 2, 3, 4`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EventCount{}
	for rows.Next() {
		var eventType, stallClass, blockerKind, lifecycleTag string
		var n int64
		if err := rows.Scan(&eventType, &stallClass, &blockerKind, &lifecycleTag, &n); err != nil {
			return nil, err
		}
		out = append(out, EventCount{
			Event: LifecycleEvent{
				EventType:    eventType,
				StallClass:   stallClass,
				BlockerKind:  blockerKind,
				LifecycleTag: lifecycleTag,
			},
			Count: int(n),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// leaseTransitionCounts folds the lease_transitions counter from the durable
// lease.released / lease.expired events. Leases transition out of 'active', so
// from is fixed to active and to is derived from the event type; the raw reason
// is bucketed to the closed category enum by the fold. Only the bucketed reason
// reaches the wire.
func (c *Collector) leaseTransitionCounts(ctx context.Context) ([]LeaseTransitionCount, error) {
	rows, err := c.runner.Query(ctx, `
		SELECT event_type,
		       COALESCE(payload_json->>'reason','') AS reason,
		       COUNT(*)::bigint AS n
		  FROM striatumd.events
		 WHERE event_type IN ('lease.released', 'lease.expired')
		 GROUP BY 1, 2`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LeaseTransitionCount{}
	for rows.Next() {
		var eventType, reason string
		var n int64
		if err := rows.Scan(&eventType, &reason, &n); err != nil {
			return nil, err
		}
		to := "released"
		if eventType == "lease.expired" {
			to = "expired"
		}
		out = append(out, LeaseTransitionCount{
			Transition: LeaseTransition{From: "active", To: to, Reason: reason},
			Count:      int(n),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// nonTerminalRunStates is the set of run states that can be "wedged" — a run that
// has not advanced a job state in a while. Terminal runs (completed/failed/
// canceled/compromised) cannot wedge, so they are excluded.
var nonTerminalRunStates = []any{"running", "ready", "blocked", "needs_operator", "needs_branch_confirmation"}

// runWedgeAges observes, per non-terminal run, the seconds since its most recent
// durable event (its last state advance) — the wedge-age signal. Origin is
// daemon-core (runs are the daemon's own aggregate). It is bounded by the number
// of non-terminal runs, never the event history.
func (c *Collector) runWedgeAges(ctx context.Context, now time.Time) ([]WedgeObservation, error) {
	rows, err := c.runner.Query(ctx, `
		SELECT EXTRACT(EPOCH FROM ($1::timestamptz - MAX(e.created_at)))::float8 AS wedge_seconds
		  FROM striatumd.runs r
		  JOIN striatumd.events e
		    ON e.repository_id = r.repository_id AND e.run_id = r.run_id
		 WHERE r.state = ANY($2::text[])
		 GROUP BY r.repository_id, r.run_id`, now, nonTerminalRunStates)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WedgeObservation{}
	for rows.Next() {
		var ageSeconds float64
		if err := rows.Scan(&ageSeconds); err != nil {
			return nil, err
		}
		if ageSeconds < 0 {
			ageSeconds = 0
		}
		out = append(out, WedgeObservation{Origin: OriginDaemonCore, AgeSeconds: ageSeconds})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// livenessMargins observes, per active lane session holding an active lease, the
// seconds of margin to the lease-heartbeat deadline — the operationally central
// liveness deadline for a working lane. Margin = (LeaseHeartbeatSeconds +
// LeaseHeartbeatSlack) - elapsed-since-last-heartbeat; it goes negative once the
// deadline has elapsed (a reversible pre-death state, F-A6). The deadline is read
// from sessionliveness.DefaultPolicy so the margin stays anchored to the real
// liveness policy rather than a hardcoded constant. Origin is lane.
func (c *Collector) livenessMargins(ctx context.Context, now time.Time) ([]MarginObservation, error) {
	policy := sessionliveness.DefaultPolicy()
	deadline := float64(policy.LeaseHeartbeatSeconds + policy.LeaseHeartbeatSlack)
	rows, err := c.runner.Query(ctx, `
		SELECT EXTRACT(EPOCH FROM ($1::timestamptz - GREATEST(
		         s.last_work_heartbeat_at, al.last_heartbeat_at, al.acquired_at)))::float8 AS elapsed
		  FROM striatumd.sessions s
		  JOIN LATERAL (
		    SELECT al.last_heartbeat_at, al.acquired_at
		      FROM striatumd.leases al
		     WHERE al.repository_id = s.repository_id
		       AND al.owner_session_id = s.session_id
		       AND al.state = 'active'
		     ORDER BY al.acquired_at DESC
		     LIMIT 1
		  ) al ON true
		 WHERE s.state = 'active'
		   AND GREATEST(s.last_work_heartbeat_at, al.last_heartbeat_at, al.acquired_at) IS NOT NULL`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MarginObservation{}
	for rows.Next() {
		var elapsed float64
		if err := rows.Scan(&elapsed); err != nil {
			return nil, err
		}
		out = append(out, MarginObservation{Origin: OriginLane, MarginSeconds: deadline - elapsed})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Handler returns the /metrics scrape handler. It does exactly Load -> render ->
// write: no PG round-trip, no shared mutex. It is a method on Collector so the
// daemon mounts the exporter from the very collector that owns the runner and
// folds the snapshot — yet the body provably reads only the published atomic
// pointer and never c.runner. The zero-DB-query test pins that boundary by
// building this handler from a collector whose runner panics on use and
// asserting a scrape never trips it. The surface is therefore served from the
// http.Server's own goroutines, lock-domain-disjoint from the
// reconcile/recovery/status mutators.
func (c *Collector) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snap := Load()
		if snap == nil {
			// Before the first fold, serve a valid empty surface (age 0).
			snap = &Snapshot{}
		}
		w.Header().Set("Content-Type", scrapeContentType)
		_ = snap.WriteText(w, time.Now().UTC())
	})
}
