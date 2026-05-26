package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/webtest"
)

// TestParseTailscaleUsersNormalizesToEmpty asserts the four fail-closed
// normalizations (RFC 0085): unset, empty, whitespace-only, and
// empty-after-parse all resolve to an empty allowlist (deny all).
func TestParseTailscaleUsersNormalizesToEmpty(t *testing.T) {
	for name, raw := range map[string]string{
		"unset/empty":       "",
		"whitespace-only":   "   \t ",
		"empty-after-parse": ", ,,  ,",
		"newlines-only":     "\n,\n",
	} {
		if got := parseTailscaleUsers(raw); len(got) != 0 {
			t.Fatalf("%s: parseTailscaleUsers(%q) = %v; want empty set (deny all)", name, raw, got)
		}
	}
}

func TestParseTailscaleUsersParsesAndTrims(t *testing.T) {
	got := parseTailscaleUsers(" alice@example.com , bob@example.com ,, alice@example.com ")
	if len(got) != 2 {
		t.Fatalf("parsed set = %v; want 2 distinct logins", got)
	}
	for _, want := range []string{"alice@example.com", "bob@example.com"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("parsed set %v missing %q", got, want)
		}
	}
}

// TestResolveWebUIOptionsIsReadOnlyIdentity asserts the identity socket options
// are identity-auth, read-only, and carry the runtime token downstream.
func TestResolveWebUIOptionsIsReadOnlyIdentity(t *testing.T) {
	t.Setenv("STRIATUM_DAEMON_WEB_TAILSCALE_USERS", "alice@example.com")
	t.Setenv("STRIATUM_DAEMON_WEB_TAILSCALE_HOST", "node.example.ts.net")
	opts := resolveWebUIOptions("runtime-token")
	if !opts.IdentityAuth {
		t.Fatal("identity socket options must set IdentityAuth")
	}
	if opts.AllowMutations {
		t.Fatal("identity socket must be read-only (AllowMutations=false)")
	}
	if opts.ServiceToken != "" {
		t.Fatal("identity socket must not use a bearer ServiceToken")
	}
	if opts.CapabilityToken != "runtime-token" {
		t.Fatalf("CapabilityToken = %q; want the runtime token", opts.CapabilityToken)
	}
	if _, ok := opts.IdentityAllowlist["alice@example.com"]; !ok {
		t.Fatalf("allowlist = %v; want alice@example.com", opts.IdentityAllowlist)
	}
	if opts.AllowedHost != "node.example.ts.net" {
		t.Fatalf("AllowedHost = %q; want the MagicDNS host", opts.AllowedHost)
	}
}

// TestMainListenerBearerPathUnchanged asserts the loopback listener built by
// newWebServiceHandler keeps bearer auth and never trusts identity headers: a
// Tailscale-User-Login header is ignored, the bearer token still gates access.
func TestMainListenerBearerPathUnchanged(t *testing.T) {
	fixture := webtest.NewFixture()
	handler := newWebServiceHandler(fixture.Server, webServiceOptions{
		RepositoryID: webtest.RepositoryID,
		ServiceToken: "service-token",
		WebEnabled:   true,
	})

	// A spoofed identity header must NOT authenticate on the main listener.
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Tailscale-User-Login", "alice@example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("main listener honored an identity header = %d; want 401 (bearer only)", rec.Code)
	}

	// The bearer token still works unchanged.
	req = httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Host = "127.0.0.1"
	req.Header.Set("Authorization", "Bearer service-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bearer auth on main listener = %d body=%s; want 200", rec.Code, rec.Body.String())
	}
}

// TestIdentityHandlerDeniesMutationOverSocket is an end-to-end check on the
// handler the socket serves: an allowlisted identity GET to a permitted route
// succeeds, but POST /v1/invoke is refused regardless of method.
func TestIdentityHandlerDeniesMutationOverSocket(t *testing.T) {
	t.Setenv("STRIATUM_DAEMON_WEB_TAILSCALE_USERS", "alice@example.com")
	t.Setenv("STRIATUM_DAEMON_WEB_TAILSCALE_HOST", "node.example.ts.net")
	fixture := webtest.NewFixture()
	handler := newWebServiceHandler(fixture.Server, resolveWebUIOptions("runtime-token"))

	get := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	get.Host = "node.example.ts.net"
	get.Header.Set("Tailscale-User-Login", "alice@example.com")
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, get)
	if getRec.Code != http.StatusOK {
		t.Fatalf("identity GET /v1/health = %d body=%s; want 200", getRec.Code, getRec.Body.String())
	}

	post := httptest.NewRequest(http.MethodPost, "/v1/invoke", strings.NewReader(`{"method":"status"}`))
	post.Host = "node.example.ts.net"
	post.Header.Set("Content-Type", "application/json")
	post.Header.Set("Tailscale-User-Login", "alice@example.com")
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, post)
	if postRec.Code != http.StatusForbidden {
		t.Fatalf("identity POST /v1/invoke = %d body=%s; want 403", postRec.Code, postRec.Body.String())
	}
}
