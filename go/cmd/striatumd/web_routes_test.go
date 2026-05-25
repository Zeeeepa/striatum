package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/webtest"
)

func TestWebServiceAdapterServesHealth(t *testing.T) {
	fixture := webtest.NewFixture()
	handler := newWebServiceHandler(fixture.Server, webServiceOptions{
		RepositoryID:    webtest.RepositoryID,
		CapabilityToken: "unused",
		AllowMutations:  false,
		WebEnabled:      true,
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWebServiceAdapterEnforcesServiceBearer(t *testing.T) {
	fixture := webtest.NewFixture()
	handler := newWebServiceHandler(fixture.Server, webServiceOptions{
		RepositoryID:    webtest.RepositoryID,
		CapabilityToken: "unused",
		ServiceToken:    "service-token",
		AllowMutations:  false,
		WebEnabled:      true,
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer service-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authorized status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWebServiceAdapterSSEUsesDaemonRunEvents(t *testing.T) {
	fixture := webtest.NewFixture()
	handler := newWebServiceHandler(fixture.Server, webServiceOptions{
		RepositoryID:    webtest.RepositoryID,
		CapabilityToken: "unused",
		AllowMutations:  false,
		WebEnabled:      true,
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/runs/run_1/events?since=0", nil)
	req.Host = "127.0.0.1"
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "event: striatum.event") {
		t.Fatalf("missing event frame: %s", rec.Body.String())
	}
	if len(fixture.Calls) == 0 || fixture.Calls[0].Method != "run.events" {
		t.Fatalf("calls = %#v", fixture.Calls)
	}
}
