package webservice_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/webtest"
)

// registerInterrogationReads wires interrogation.list + interrogation.show onto
// the fixture's RPC server. interrogation.show returns run_id "run_1" only for
// interrogation_id "intg_match"; any other id resolves under run_id
// "run_other" so the run-ownership 404 path can be exercised.
func registerInterrogationReads(fixture *webtest.Fixture, turns []any) {
	// list goes through the tracking helper so tests can assert on fixture.Calls.
	fixture.Register("interrogation.list", map[string]any{"count": 1, "run_id": "run_1", "items": []any{
		map[string]any{"interrogation_id": "intg_match", "state": "open"},
	}})
	// show needs a conditional run_id, so it is registered directly.
	fixture.Server.Register("interrogation.show", func(ctx context.Context, env rpc.Envelope) (map[string]any, error) {
		id, _ := env.Params["interrogation_id"].(string)
		runID := "run_other"
		if id == "intg_match" {
			runID = "run_1"
		}
		return map[string]any{
			"interrogation": map[string]any{
				"interrogation_id":        id,
				"run_id":                  runID,
				"topic":                   "threat model",
				"state":                   "closed",
				"interrogator_session_id": "sess_reviewer",
				"target_session_id":       "sess_target",
				"opened_at":               "2026-05-25T00:00:00Z",
			},
			"turns":      turns,
			"turn_count": len(turns),
		}, nil
	})
}

func TestInterrogationListRouteUsesDaemonRPC(t *testing.T) {
	fixture := webtest.NewFixture()
	registerInterrogationReads(fixture, nil)
	rec := webtest.Request(t, webtest.Handler(fixture, false), http.MethodGet, "/v1/runs/run_1/interrogations", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fixture.Calls) != 1 || fixture.Calls[0].Method != "interrogation.list" {
		t.Fatalf("calls = %#v", fixture.Calls)
	}
	if got := fixture.Calls[0].Params["run_id"]; got != "run_1" {
		t.Fatalf("run_id param = %#v", got)
	}
	if got := fixture.Calls[0].Params["repository_id"]; got != webtest.RepositoryID {
		t.Fatalf("repository_id = %#v", got)
	}
}

func TestInterrogationShowReturnsCuratedJSON(t *testing.T) {
	fixture := webtest.NewFixture()
	registerInterrogationReads(fixture, []any{
		map[string]any{"kind": "interrogation_question", "body": "q", "turn_index": 0},
		map[string]any{"kind": "interrogation_answer", "body": "a", "turn_index": 1},
	})
	rec := webtest.Request(t, webtest.Handler(fixture, false), http.MethodGet, "/v1/runs/run_1/interrogations/intg_match", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"turn_count\":2") {
		t.Fatalf("expected curated turns in body: %s", rec.Body.String())
	}
}

// TestInterrogationShowRunOwnership404 is the IDOR guard: an interrogation_id
// that resolves under a different run must 404 when fetched under run_1's path.
func TestInterrogationShowRunOwnership404(t *testing.T) {
	fixture := webtest.NewFixture()
	registerInterrogationReads(fixture, nil)
	rec := webtest.Request(t, webtest.Handler(fixture, false), http.MethodGet, "/v1/runs/run_1/interrogations/intg_other", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-run interrogation, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestInterrogationChatViewEscapesBodies pins that the server-rendered chat
// page (?view=chat) escapes attacker-controlled turn bodies via html/template.
func TestInterrogationChatViewEscapesBodies(t *testing.T) {
	fixture := webtest.NewFixture()
	registerInterrogationReads(fixture, []any{
		map[string]any{"kind": "interrogation_question", "body": "<script>alert(1)</script>", "turn_index": 0},
		map[string]any{"kind": "interrogation_answer", "body": "<img src=x onerror=alert(1)>", "turn_index": 1},
	})
	rec := webtest.Request(t, webtest.Handler(fixture, false), http.MethodGet, "/v1/runs/run_1/interrogations/intg_match?view=chat", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q", ct)
	}
	body := rec.Body.String()
	for _, raw := range []string{"<script>alert(1)</script>", "<img src=x onerror=alert(1)>"} {
		if strings.Contains(body, raw) {
			t.Fatalf("raw payload leaked into chat page: %q\n%s", raw, body)
		}
	}
	for _, escaped := range []string{"&lt;script&gt;alert(1)&lt;/script&gt;", "&lt;img src=x onerror=alert(1)&gt;"} {
		if !strings.Contains(body, escaped) {
			t.Fatalf("expected escaped form %q in chat page\n%s", escaped, body)
		}
	}
}
