package db

import (
	"context"
	"strings"
	"testing"
)

// recordingEventRow captures the SQL AppendEventRowSD issues and the args, so the
// SD-call shape can be asserted without a database. It returns a scannable row
// that yields a fixed event_id.
type recordingEventRow struct {
	sql  string
	args []any
}

type fixedScalarRow struct{ id int64 }

func (r fixedScalarRow) Scan(dest ...any) error {
	if len(dest) == 1 {
		if p, ok := dest[0].(*int64); ok {
			*p = r.id
		}
	}
	return nil
}

func (t *recordingEventRow) QueryRow(_ context.Context, sql string, args ...any) Row {
	t.sql = sql
	t.args = args
	return fixedScalarRow{id: 42}
}

// TestAppendEventRowSDCallShape asserts the P2 event-append SD path issues a
// single SELECT through the owner-owned append_event_row, passes the nine event
// columns in order with the payload as a jsonb arg, and returns the in-DB-assigned
// event_id. The chain hash, event_id, created_at, and previous_hash are NOT in the
// call — the SD function owns them (RFC 0110 §7 P2).
func TestAppendEventRowSDCallShape(t *testing.T) {
	rec := &recordingEventRow{}
	id, err := AppendEventRowSD(context.Background(), rec, EventRow{
		RepositoryID:   "repo",
		RunID:          "run",
		EventType:      "job.created",
		ActorSessionID: nil,
		JobID:          "job",
		Payload:        map[string]any{"state": "running"},
	})
	if err != nil {
		t.Fatalf("AppendEventRowSD: %v", err)
	}
	if id != 42 {
		t.Fatalf("event_id = %d, want 42 (the in-DB assigned id)", id)
	}
	if !strings.Contains(rec.sql, "striatumd.append_event_row") {
		t.Fatalf("expected SD routing through append_event_row, got %q", rec.sql)
	}
	if strings.Contains(rec.sql, "INSERT INTO striatumd.events") {
		t.Fatal("SD path must not direct-INSERT events")
	}
	if !strings.Contains(rec.sql, "::jsonb") {
		t.Fatalf("payload must be passed as jsonb, got %q", rec.sql)
	}
	if len(rec.args) != 9 {
		t.Fatalf("append_event_row takes 9 args (the SD fn owns event_id/created_at/hash); got %d", len(rec.args))
	}
	if rec.args[0] != "repo" || rec.args[2] != "job.created" {
		t.Fatalf("args out of order: %v", rec.args)
	}
}

// TestAppendEventRowSDRequiresQueryRow guards the runner contract: a runner that
// cannot QueryRow is rejected rather than silently dropping the append.
func TestAppendEventRowSDRequiresQueryRow(t *testing.T) {
	if _, err := AppendEventRowSD(context.Background(), struct{}{}, EventRow{RepositoryID: "r"}); err == nil {
		t.Fatal("expected error for a runner without QueryRow")
	}
}
