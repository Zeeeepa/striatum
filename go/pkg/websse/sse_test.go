package websse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type staticSource struct{}

func (staticSource) Fetch(ctx context.Context, since int, limit int) (Batch, error) {
	return Batch{
		Events: []Event{{ID: since + 1, Type: "striatum.event", Data: map[string]any{"run_id": "run_1"}}},
	}, nil
}

type terminalSource struct{}

func (terminalSource) Fetch(ctx context.Context, since int, limit int) (Batch, error) {
	return Batch{Terminal: true, RunState: "completed"}, nil
}

func TestSincePrefersLastEventID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/events?since=2", nil)
	req.Header.Set("Last-Event-ID", "7")
	if got := Since(req); got != 7 {
		t.Fatalf("Since = %d", got)
	}
}

func TestWriteEventFramesData(t *testing.T) {
	var builder strings.Builder
	if err := WriteEvent(&builder, "striatum.event", 4, map[string]any{"ok": true}); err != nil {
		t.Fatal(err)
	}
	got := builder.String()
	for _, want := range []string{"event: striatum.event", "id: 4", `data: {"ok":true}`} {
		if !strings.Contains(got, want) {
			t.Fatalf("frame missing %q: %s", want, got)
		}
	}
}

func TestStreamWritesTerminalEventAndReturns(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	Stream(req.Context(), rec, terminalSource{}, 0, Options{PollInterval: time.Millisecond})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, "event: striatum.run_terminal") {
		t.Fatalf("missing terminal event: %s", got)
	}
}
