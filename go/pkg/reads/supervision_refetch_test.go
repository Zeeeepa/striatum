package reads

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

// refetchEmptyRunner serves the first supervisor LIMIT-1 status fetch with a
// real row, but returns an EMPTY result set on the post-drain re-fetch (mirroring
// the supervisor row being concurrently removed, or the daemon stopping and
// rotating the MCP port out from under the read — #180/#200). BeginTx returns a
// no-op tx so the DrainHelperEventsHook branch runs and the re-fetch executes.
type refetchEmptyRunner struct {
	superviseReadFakeRunner
	statusFetches int
}

func (r *refetchEmptyRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return &refetchNoopTx{}, nil
}

func (r *refetchEmptyRunner) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "FROM striatumd.process_supervisors") &&
		strings.Contains(sql, "ORDER BY ps.started_at DESC") &&
		strings.Contains(sql, "LIMIT 1") {
		r.statusFetches++
		if r.statusFetches >= 2 {
			// The post-drain re-fetch returns no rows.
			return dashboardAllRowsFromMaps([]map[string]any{}), nil
		}
	}
	return r.superviseReadFakeRunner.Query(ctx, sql, args...)
}

type refetchNoopTx struct{}

func (refetchNoopTx) Exec(context.Context, string, ...any) error { return nil }
func (refetchNoopTx) QueryRow(context.Context, string, ...any) db.Row {
	return superviseReadFakeRow{}
}
func (refetchNoopTx) QueryScalar(context.Context, string, ...any) (string, error) { return "", nil }
func (refetchNoopTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return dashboardAllRowsFromMaps([]map[string]any{}), nil
}
func (refetchNoopTx) Commit(context.Context) error   { return nil }
func (refetchNoopTx) Rollback(context.Context) error { return nil }

// TestHandleSuperviseStatusDoesNotPanicWhenRefetchReturnsNoRows is the #180/#200
// regression: HandleSuperviseStatus drains PTY helper events and then re-fetches
// the supervisor row so rows[0]["pointer_metadata_json"] reflects the drained
// pointer metadata. The re-fetch result was assigned unconditionally, so when the
// re-fetch returned no rows (a concurrently removed supervisor, or the daemon
// stopping and rotating the MCP port) it clobbered the previously good rows with
// an empty slice and the next rows[0] index panicked with
// "index out of range [0] with length 0" — crashing striatumd, including during
// daemon stop (#200). The handler must keep the original row instead.
func TestHandleSuperviseStatusDoesNotPanicWhenRefetchReturnsNoRows(t *testing.T) {
	// Install a draining hook so the re-fetch branch runs (it is gated on
	// DrainHelperEventsHook != nil). Restore the prior hook afterwards.
	prevHook := DrainHelperEventsHook
	DrainHelperEventsHook = func(context.Context, db.TxRunner, string, string) error { return nil }
	t.Cleanup(func() { DrainHelperEventsHook = prevHook })

	runner := &refetchEmptyRunner{}
	runner.statusRow = superviseBaseRow("sup_refetch", os.Getpid(), currentStartTokenForTest())

	// Before the fix this panics; after the fix it returns a sane projection
	// built from the original row.
	result, err := HandleSuperviseStatus(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1", "session_id": "sess_1"},
	})
	if err != nil {
		t.Fatalf("HandleSuperviseStatus: %v", err)
	}
	if runner.statusFetches < 2 {
		t.Fatalf("expected the post-drain re-fetch to run (got %d status fetches)", runner.statusFetches)
	}
	if result["supervisor_id"] != "sup_refetch" {
		t.Fatalf("status projection should retain the original supervisor row, got supervisor_id=%#v", result["supervisor_id"])
	}
}
