package webservice_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/webtest"
)

func TestHealthAndSecurityHeaders(t *testing.T) {
	fixture := webtest.NewFixture()
	rec := webtest.Request(t, webtest.Handler(fixture, false), http.MethodGet, "/v1/health", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'self'") {
		t.Fatalf("missing CSP header: %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
}

func TestReadRouteUsesDaemonRPC(t *testing.T) {
	fixture := webtest.NewFixture()
	rec := webtest.Request(t, webtest.Handler(fixture, false), http.MethodGet, "/v1/runs/run_1/dashboard", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fixture.Calls) != 1 || fixture.Calls[0].Method != "dashboard" {
		t.Fatalf("calls = %#v", fixture.Calls)
	}
	if got := fixture.Calls[0].Params["repository_id"]; got != webtest.RepositoryID {
		t.Fatalf("repository_id = %#v", got)
	}
}

func TestMutationRefusedWhenDisabled(t *testing.T) {
	fixture := webtest.NewFixture()
	body := `{"method":"run.cancel","params":{"run_id":"run_1"}}`
	rec := webtest.Request(t, webtest.Handler(fixture, false), http.MethodPost, "/v1/invoke", body)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fixture.Calls) != 0 {
		t.Fatalf("mutation reached RPC: %#v", fixture.Calls)
	}
}

func TestMutationAllowedWhenEnabled(t *testing.T) {
	fixture := webtest.NewFixture()
	body := `{"method":"run.cancel","params":{"run_id":"run_1"}}`
	rec := webtest.Request(t, webtest.Handler(fixture, true), http.MethodPost, "/v1/invoke", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(fixture.Calls) != 1 || fixture.Calls[0].Method != "run.cancel" {
		t.Fatalf("calls = %#v", fixture.Calls)
	}
}

func TestNonLoopbackHostRefused(t *testing.T) {
	fixture := webtest.NewFixture()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Host = "striatum.example.test"
	rec := httptest.NewRecorder()
	webtest.Handler(fixture, false).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRetiredRoutesReturnGone(t *testing.T) {
	fixture := webtest.NewFixture()
	for _, route := range []string{"/dogfood", "/dogfood/042", "/chat", "/chat/session"} {
		rec := webtest.Request(t, webtest.Handler(fixture, false), http.MethodGet, route, "")
		if rec.Code != http.StatusGone {
			t.Fatalf("%s status = %d body=%s", route, rec.Code, rec.Body.String())
		}
	}
}

func TestStaticAssetServedFromGoEmbed(t *testing.T) {
	fixture := webtest.NewFixture()
	rec := webtest.Request(t, webtest.Handler(fixture, false), http.MethodGet, "/static/base.css", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "font-family") {
		t.Fatalf("unexpected asset body: %s", rec.Body.String())
	}
}

func TestArtifactRawUsesDaemonContent(t *testing.T) {
	fixture := webtest.NewFixture()
	rec := webtest.Request(t, webtest.Handler(fixture, false), http.MethodGet, "/v1/artifacts/artifact_1/raw", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "hello\n" {
		t.Fatalf("body = %q", got)
	}
}
