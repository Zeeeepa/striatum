package webtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/halbritt/striatum/go/pkg/webservice"
)

const RepositoryID = "repo_test"

type Fixture struct {
	Server *rpc.Server
	Calls  []Call
}

type Call struct {
	Method string
	Params map[string]any
}

func NewFixture() *Fixture {
	server := rpc.NewServer()
	server.Authorizer = rpc.AllowAllAuthorizer{}
	fixture := &Fixture{Server: server}
	fixture.Register("status", map[string]any{"runs": []any{}, "next_actions": []any{}})
	fixture.Register("why", map[string]any{"target": "ok"})
	fixture.Register("dashboard", map[string]any{"frame": "dashboard"})
	fixture.Register("list.artifacts", map[string]any{"items": []any{}})
	fixture.Register("workflow.templates.list", map[string]any{"templates": []any{}})
	fixture.Register("workflow.templates.show", map[string]any{"template_id": "minimal"})
	fixture.Register("run.cancel", map[string]any{"run_id": "run_1", "state": "canceled"})
	fixture.Register("workflow.generate.preview", map[string]any{"ok": true, "preview": true})
	fixture.Register("workflow.generate", map[string]any{"ok": true, "written": true})
	fixture.Register("artifact.get_content", map[string]any{
		"artifact_id":  "artifact_1",
		"content_type": "text/plain; charset=utf-8",
		"body_base64":  "aGVsbG8K",
	})
	fixture.Register("run.events", map[string]any{
		"run":      map[string]any{"run_id": "run_1", "state": "running"},
		"terminal": false,
		"events": []any{
			map[string]any{"event_id": 1, "event_type": "run.started", "run_id": "run_1"},
		},
	})
	return fixture
}

func (f *Fixture) Register(method string, data map[string]any) {
	f.Server.Register(method, func(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
		f.Calls = append(f.Calls, Call{Method: envelope.Method, Params: envelope.Params})
		return data, nil
	})
}

func Handler(fixture *Fixture, allowMutations bool) http.Handler {
	return webservice.New(webservice.Config{
		RPC:             fixture.Server,
		RepositoryID:    RepositoryID,
		CapabilityToken: "unused",
		AllowMutations:  allowMutations,
		WebEnabled:      true,
	})
}

func Request(t *testing.T, handler http.Handler, method string, target string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "127.0.0.1"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
