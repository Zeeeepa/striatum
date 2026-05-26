package webservice_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/webservice"
	"github.com/halbritt/striatum/go/pkg/webtest"
)

const identityHost = "node.example.ts.net"

// identityHandler builds a web handler in RFC 0085 tailnet-identity auth mode
// over the fixture's RPC server, with the given allowlisted logins.
func identityHandler(fixture *webtest.Fixture, users ...string) http.Handler {
	set := map[string]struct{}{}
	for _, u := range users {
		set[u] = struct{}{}
	}
	return webservice.New(webservice.Config{
		RPC:               fixture.Server,
		RepositoryID:      webtest.RepositoryID,
		CapabilityToken:   "unused",
		WebEnabled:        true,
		IdentityAuth:      true,
		IdentityAllowlist: set,
		AllowedHost:       identityHost,
	})
}

// identityRequest drives one request against an identity-mode handler. A
// non-empty login is sent as the Tailscale-User-Login header that `tailscale
// serve` would inject; the Host is the configured MagicDNS name.
func identityRequest(t *testing.T, handler http.Handler, method, target, login, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = identityHost
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if login != "" {
		req.Header.Set("Tailscale-User-Login", login)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestIdentityAllowedLoginPermittedGET(t *testing.T) {
	fixture := webtest.NewFixture()
	handler := identityHandler(fixture, "alice@example.com")
	// /v1/health needs no RPC; /v1/runs maps to the read "status" handler.
	for _, target := range []string{"/v1/health", "/v1/runs"} {
		rec := identityRequest(t, handler, http.MethodGet, target, "alice@example.com", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s with allowed identity = %d body=%s", target, rec.Code, rec.Body.String())
		}
	}
}

func TestIdentityLoginNotInAllowlistForbidden(t *testing.T) {
	fixture := webtest.NewFixture()
	handler := identityHandler(fixture, "alice@example.com")
	rec := identityRequest(t, handler, http.MethodGet, "/v1/runs", "mallory@example.com", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("not-in-allowlist identity = %d body=%s; want 403", rec.Code, rec.Body.String())
	}
}

func TestIdentityNoIdentityUnauthorized(t *testing.T) {
	fixture := webtest.NewFixture()
	handler := identityHandler(fixture, "alice@example.com")
	rec := identityRequest(t, handler, http.MethodGet, "/v1/runs", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no identity = %d body=%s; want 401", rec.Code, rec.Body.String())
	}
}

// TestIdentityPostInvokeForbidden asserts the read-only invariant is enforced by
// route allowlist, not HTTP verb: POST /v1/invoke is denied under identity even
// when its method parameter is a read (status) — not just when it mutates.
func TestIdentityPostInvokeForbidden(t *testing.T) {
	fixture := webtest.NewFixture()
	handler := identityHandler(fixture, "alice@example.com")
	for _, body := range []string{
		`{"method":"status"}`,     // read method — still denied
		`{"method":"run.cancel"}`, // mutating method — denied
	} {
		rec := identityRequest(t, handler, http.MethodPost, "/v1/invoke", "alice@example.com", body)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("POST /v1/invoke %s under identity = %d body=%s; want 403", body, rec.Code, rec.Body.String())
		}
	}
	// The mutating RPC must never have been dispatched.
	for _, call := range fixture.Calls {
		if call.Method == "run.cancel" || call.Method == "status" {
			t.Fatalf("identity POST /v1/invoke reached RPC method %q; must be denied before dispatch", call.Method)
		}
	}
}

func TestIdentityNonAllowlistedGETForbidden(t *testing.T) {
	fixture := webtest.NewFixture()
	handler := identityHandler(fixture, "alice@example.com")
	// /v1/runs/{id}/why is a GET read route but NOT in the allowlist.
	rec := identityRequest(t, handler, http.MethodGet, "/v1/runs/run_1/why?id=run_1", "alice@example.com", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-allowlisted GET under identity = %d body=%s; want 403", rec.Code, rec.Body.String())
	}
}

// TestIdentityEmptyAllowlistDeniesAll asserts fail-closed: an enabled identity
// listener with no allowlisted logins denies every identity request, even for a
// permitted route with a present login.
func TestIdentityEmptyAllowlistDeniesAll(t *testing.T) {
	fixture := webtest.NewFixture()
	handler := identityHandler(fixture) // empty allowlist
	rec := identityRequest(t, handler, http.MethodGet, "/v1/runs", "alice@example.com", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("empty allowlist permitted-route GET = %d body=%s; want 403 (fail closed)", rec.Code, rec.Body.String())
	}
}

// TestIdentityRouteAuditMatchesAllowlist is the normative route-audit test
// (RFC 0085). It enumerates the full web-handler route surface and asserts that
// PermitIdentityRoute permits EXACTLY the IdentityReadRoutes allowlist and
// denies everything else — every non-allowlisted read route and every mutating
// route — so the read-only invariant is auditable rather than incidental.
func TestIdentityRouteAuditMatchesAllowlist(t *testing.T) {
	type route struct {
		method string
		path   string
	}
	// The complete surface the web GET/POST routers expose (see routeGET /
	// routePOST in service.go). Keep this in sync with that surface; the audit
	// fails loudly if a new route slips in unaudited.
	allRoutes := []route{
		{http.MethodGet, "/v1/health"},
		{http.MethodGet, "/v1/runs"},
		{http.MethodGet, "/v1/runs/run_1"},
		{http.MethodGet, "/v1/runs/run_1/interrogations"},
		{http.MethodGet, "/v1/runs/run_1/interrogations/intg_1"},
		{http.MethodGet, "/v1/runs/run_1/interrogations/intg_1?view=chat"},
		// GET read routes deliberately excluded from the identity allowlist:
		{http.MethodGet, "/v1/runs/run_1/why?id=run_1"},
		{http.MethodGet, "/v1/runs/run_1/dashboard"},
		{http.MethodGet, "/v1/runs/run_1/artifacts"},
		{http.MethodGet, "/v1/runs/run_1/events"},
		{http.MethodGet, "/v1/artifacts/artifact_1/raw"},
		{http.MethodGet, "/workflow-templates"},
		{http.MethodGet, "/workflow-templates/minimal"},
		{http.MethodGet, "/"},
		{http.MethodGet, "/run"},
		{http.MethodGet, "/static/app.js"},
		// Mutating routes — must never be reachable over identity:
		{http.MethodPost, "/v1/invoke"},
		{http.MethodPost, "/workflows/generate/preview"},
		{http.MethodPost, "/workflows/generate"},
		// Mutating verbs on read paths — also denied (verb is not the gate):
		{http.MethodPost, "/v1/runs"},
		{http.MethodDelete, "/v1/runs/run_1"},
	}

	// Expected permitted set, taken straight from the normative allowlist.
	expectedPermitted := map[string]struct{}{}
	for _, r := range webservice.IdentityReadRoutes() {
		if r.Method != http.MethodGet {
			t.Fatalf("allowlist entry %q %q is not a GET; the identity set must be read-only", r.Method, r.Pattern)
		}
		if !webservice.PermitIdentityRoute(r.Method, r.Example) {
			t.Fatalf("allowlist example %q %q is not permitted by PermitIdentityRoute", r.Method, r.Example)
		}
		expectedPermitted[r.Method+" "+r.Example] = struct{}{}
	}

	// Walk the full surface; the set PermitIdentityRoute permits must equal the
	// allowlist example set exactly.
	gotPermitted := map[string]struct{}{}
	for _, rt := range allRoutes {
		// PermitIdentityRoute keys on path (it path.Clean's and ignores query),
		// so strip any query before recording the permitted key.
		cleanPath := rt.path
		if i := strings.IndexByte(cleanPath, '?'); i >= 0 {
			cleanPath = cleanPath[:i]
		}
		if webservice.PermitIdentityRoute(rt.method, rt.path) {
			gotPermitted[rt.method+" "+cleanPath] = struct{}{}
		}
	}

	if len(gotPermitted) != len(expectedPermitted) {
		t.Fatalf("permitted set size = %d, allowlist size = %d\n got=%v\nwant=%v", len(gotPermitted), len(expectedPermitted), gotPermitted, expectedPermitted)
	}
	for key := range expectedPermitted {
		if _, ok := gotPermitted[key]; !ok {
			t.Fatalf("allowlisted route %q not permitted by the route surface walk", key)
		}
	}
	for key := range gotPermitted {
		if _, ok := expectedPermitted[key]; !ok {
			t.Fatalf("route %q is permitted over identity but is NOT in the allowlist", key)
		}
	}
}
