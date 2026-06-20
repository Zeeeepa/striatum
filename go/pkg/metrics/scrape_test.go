package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
)

// panicQuerier is a Querier whose every DB entrypoint panics. If the scrape path
// ever reaches PG, ServeHTTP unwinds with this panic and the test fails; its
// call counter lets the test prove the scrape path issued exactly zero queries.
type panicQuerier struct {
	calls atomic.Int64
}

func (p *panicQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	p.calls.Add(1)
	panic("scrape path touched the database")
}

// TestScrapeIssuesZeroQueries drives the scrape handler with a runner whose
// Query panics and asserts a scrape succeeds and issues zero queries — proving
// the scrape path never touches PG (RFC 0137 Acceptance Criteria: panic-on-query
// test). The Refresh sanity check proves the querier is a live DB entrypoint, so
// the zero-query assertion is not vacuous: a regression that made the handler
// re-fold on scrape would trip the panic.
func TestScrapeIssuesZeroQueries(t *testing.T) {
	pq := &panicQuerier{}
	c := NewCollector(pq)

	// Publish a snapshot without touching the DB.
	Publish(BuildSnapshot(sentinelBuiltAt, sentinelRuns(), 3))

	h := c.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	for i := 0; i < 256; i++ {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("scrape %d: status = %d, want 200", i, rr.Code)
		}
		if rr.Body.Len() == 0 {
			t.Fatalf("scrape %d: empty body", i)
		}
	}
	if got := pq.calls.Load(); got != 0 {
		t.Fatalf("scrape path issued %d DB queries; want 0", got)
	}

	// Sanity: Refresh genuinely reaches the querier (it panics), so the zero
	// above means the scrape path is decoupled, not that the querier is inert.
	func() {
		defer func() { _ = recover() }()
		_ = c.Refresh(context.Background(), sentinelNow)
		t.Fatal("Refresh did not reach the panicking querier; zero-query guard is vacuous")
	}()
	if pq.calls.Load() == 0 {
		t.Fatal("Refresh did not reach the querier; zero-query guard is vacuous")
	}
}

// TestConcurrentScrapesSeeIdenticalSnapshot fires ~1000 concurrent reads against
// a fixed published snapshot and asserts they observe the identical snapshot
// pointer — the atomic.Pointer is read, never recomputed per scrape (RFC 0137
// Acceptance Criteria: concurrent-scrape identity test).
func TestConcurrentScrapesSeeIdenticalSnapshot(t *testing.T) {
	snap := BuildSnapshot(sentinelBuiltAt, sentinelRuns(), 3)
	Publish(snap)

	const n = 1000
	observed := make([]*Snapshot, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			observed[i] = Load()
		}(i)
	}
	wg.Wait()

	for i, got := range observed {
		if got != snap {
			t.Fatalf("scrape %d observed a different snapshot pointer (%p) than the published one (%p)", i, got, snap)
		}
	}
}
