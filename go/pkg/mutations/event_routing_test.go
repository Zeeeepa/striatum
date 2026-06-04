package mutations

import (
	"context"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
)

// eventRoutingRunner records the SQL appendEvent issues and serves the reads the
// direct path needs (chain head + nextval) so both phases run without a database.
type eventRoutingRunner struct {
	queryRowSQL []string
	execSQL     []string
}

type eventRoutingScalarRow struct{ id int64 }

func (r eventRoutingScalarRow) Scan(dest ...any) error {
	if len(dest) == 1 {
		if p, ok := dest[0].(*int64); ok {
			*p = r.id
		}
	}
	return nil
}

type eventRoutingNullStringRow struct{}

func (eventRoutingNullStringRow) Scan(dest ...any) error { return nil }

func (r *eventRoutingRunner) QueryRow(_ context.Context, sql string, _ ...any) db.Row {
	r.queryRowSQL = append(r.queryRowSQL, sql)
	switch {
	case strings.Contains(sql, "append_event_row"):
		return eventRoutingScalarRow{id: 7}
	case strings.Contains(sql, "nextval"):
		return eventRoutingScalarRow{id: 7}
	default: // last_hash FROM repo_event_chain_heads -> NULL
		return eventRoutingNullStringRow{}
	}
}

func (r *eventRoutingRunner) Exec(_ context.Context, sql string, _ ...any) error {
	r.execSQL = append(r.execSQL, sql)
	return nil
}

func (r *eventRoutingRunner) joinedQueryRow() string { return strings.Join(r.queryRowSQL, "\n") }
func (r *eventRoutingRunner) joinedExec() string     { return strings.Join(r.execSQL, "\n") }

// TestAppendEventRoutesByPhase is the RFC 0110 §7 P2 call-site routing gate:
// below full the event append is the historical direct INSERT (+ chain-head
// upsert); at full it routes through the owner-owned SECURITY DEFINER
// append_event_row and issues no direct events INSERT.
func TestAppendEventRoutesByPhase(t *testing.T) {
	t.Cleanup(func() { db.SetActiveWriteBoundary(db.PhaseNone) })

	for _, tc := range []struct {
		phase  db.WriteBoundaryPhase
		wantSD bool
	}{
		{db.PhaseNone, false},
		{db.PhaseAuditOnly, false},
		{db.PhaseAuditArtifacts, false},
		{db.PhaseFull, true},
	} {
		db.SetActiveWriteBoundary(tc.phase)
		r := &eventRoutingRunner{}
		id, err := appendEvent(context.Background(), r, "repo", "run", "job.created", nil, "job", nil, nil, nil,
			map[string]any{"state": "running"})
		if err != nil {
			t.Fatalf("phase %s: %v", tc.phase, err)
		}
		if id == 0 {
			t.Fatalf("phase %s: expected a non-zero event id", tc.phase)
		}
		if tc.wantSD {
			if !strings.Contains(r.joinedQueryRow(), "striatumd.append_event_row") {
				t.Fatalf("phase %s: expected SD routing, queryRow=%q", tc.phase, r.joinedQueryRow())
			}
			if strings.Contains(r.joinedExec(), "INSERT INTO striatumd.events") {
				t.Fatalf("phase %s: SD routing must not direct-INSERT events (exec=%q)", tc.phase, r.joinedExec())
			}
		} else {
			if !strings.Contains(r.joinedExec(), "INSERT INTO striatumd.events") {
				t.Fatalf("phase %s: expected direct INSERT, exec=%q", tc.phase, r.joinedExec())
			}
			if strings.Contains(r.joinedQueryRow(), "striatumd.append_event_row") {
				t.Fatalf("phase %s: pre-P2 must not call the SD function", tc.phase)
			}
		}
	}
}
