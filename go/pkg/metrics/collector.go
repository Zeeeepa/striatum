package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

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
func (c *Collector) Refresh(ctx context.Context, now time.Time) error {
	if c.runner == nil {
		return fmt.Errorf("metrics fold requires a daemon runner")
	}
	rawCounts, err := c.runStateCounts(ctx)
	if err != nil {
		return fmt.Errorf("fold run-state counts: %w", err)
	}
	stranded, err := c.strandedSupervisorCount(ctx)
	if err != nil {
		return fmt.Errorf("fold stranded-supervisor count: %w", err)
	}
	Publish(newSnapshot(now.UTC(), rawCounts, stranded))
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
