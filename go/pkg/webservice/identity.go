package webservice

import (
	"net/http"
	"path"
	"strings"
)

// IdentityReadRoute is one entry in the RFC 0085 tailnet-identity read-route
// allowlist — the only routes a tailnet-identity request (arriving over the
// dedicated web-ui.sock that `tailscale serve` targets) is permitted to reach.
// The set is read-only by construction: every entry is a GET against a read
// RPC. Read-only is enforced by membership in this allowlist, NOT by the HTTP
// verb — design interrogation intg_4b69c562… rejected "GET means safe", so a
// future read-but-mutating GET route would still be denied until explicitly
// added here.
type IdentityReadRoute struct {
	// Method is always GET in this slice; carried explicitly so the audit test
	// asserts read-only on the route record, not on an implicit convention.
	Method string
	// Pattern is the documentation shape (e.g. /v1/runs/{run_id}).
	Pattern string
	// Example is a concrete path matching Pattern, used by the route-audit test
	// to drive PermitIdentityRoute over a real value.
	Example string
}

// IdentityReadRoutes is the normative allowlist of routes permitted over the
// tailnet-identity UI socket (RFC 0085 D143). Everything not matched by an
// entry here is denied (403), including POST /v1/invoke (even when its method
// is a read), workflow generation, and any other/future route. Mutations stay
// on the bearer + loopback path.
func IdentityReadRoutes() []IdentityReadRoute {
	return []IdentityReadRoute{
		{http.MethodGet, "/v1/health", "/v1/health"},
		{http.MethodGet, "/v1/runs", "/v1/runs"},
		{http.MethodGet, "/v1/runs/{run_id}", "/v1/runs/run_1"},
		{http.MethodGet, "/v1/runs/{run_id}/interrogations", "/v1/runs/run_1/interrogations"},
		{http.MethodGet, "/v1/runs/{run_id}/interrogations/{interrogation_id}", "/v1/runs/run_1/interrogations/intg_1"},
		// Read-only HTML dashboard (server-rendered status page) + its static
		// assets, so the human web UI is reachable over `tailscale serve` and not
		// only the JSON API. All GET, no mutation; the page renders the same
		// `status` read already exposed at /v1/runs.
		{http.MethodGet, "/", "/"},
		{http.MethodGet, "/run", "/run"},
		{http.MethodGet, "/static/{asset}", "/static/app.js"},
	}
}

// PermitIdentityRoute reports whether (method, rawPath) is in the
// tailnet-identity read-route allowlist. It is the single authority the
// identity handler consults before dispatching, and the normative route-audit
// test (TestIdentityRouteAuditMatchesAllowlist) asserts it permits exactly the
// IdentityReadRoutes set and denies every other route — including all mutating
// ones — so the read-only invariant is auditable, not incidental.
func PermitIdentityRoute(method, rawPath string) bool {
	if method != http.MethodGet {
		return false
	}
	clean := path.Clean(rawPath)
	switch {
	case clean == "/v1/health":
		return true
	case clean == "/" || clean == "/run":
		// Read-only HTML dashboard (server-rendered status page).
		return true
	case strings.HasPrefix(clean, "/static/"):
		// Static CSS/JS for the dashboard. path.Clean above removes any
		// "../" so this cannot escape the static asset prefix.
		return true
	case clean == "/v1/runs":
		return true
	case strings.HasPrefix(clean, "/v1/runs/"):
		parts := strings.Split(strings.TrimPrefix(clean, "/v1/runs/"), "/")
		if parts[0] == "" {
			return false
		}
		switch len(parts) {
		case 1:
			// /v1/runs/{run_id}
			return true
		case 2:
			// /v1/runs/{run_id}/interrogations — sibling read routes
			// (why, dashboard, artifacts, events) are intentionally NOT here.
			return parts[1] == "interrogations"
		case 3:
			// /v1/runs/{run_id}/interrogations/{interrogation_id} (incl ?view=chat)
			return parts[1] == "interrogations" && parts[2] != ""
		}
		return false
	}
	return false
}

// serveIdentity is the request path for the dedicated tailnet-identity UI
// listener (web-ui.sock). It trusts the Tailscale-User-Login header — set only
// by `tailscale serve` on a 0600 owner-only socket and stripped of any
// client-spoofed copy — instead of a bearer token, and permits ONLY the
// read-route allowlist. The main loopback listener never enters this path and
// never trusts identity headers.
func (h *Handler) serveIdentity(w http.ResponseWriter, r *http.Request) {
	if !h.identityHostAllowed(r.Host) {
		writeJSON(w, http.StatusForbidden, errorPayload("host_refused", "Host not permitted on the tailnet-identity UI listener"))
		return
	}
	login := strings.TrimSpace(r.Header.Get("Tailscale-User-Login"))
	if login == "" {
		writeJSON(w, http.StatusUnauthorized, errorPayload("identity_missing", "missing Tailscale identity"))
		return
	}
	if _, ok := h.config.IdentityAllowlist[login]; !ok {
		writeJSON(w, http.StatusForbidden, errorPayload("identity_not_allowed", "tailnet login is not in the allowlist"))
		return
	}
	if !PermitIdentityRoute(r.Method, r.URL.Path) {
		writeJSON(w, http.StatusForbidden, errorPayload("route_forbidden", "route is not in the tailnet-identity read-route allowlist"))
		return
	}
	// Permitted read route → dispatch through the shared GET router. Only the
	// five allowlisted routes can reach here, so no mutating handler is reachable.
	h.routeGET(w, r)
}

// identityHostAllowed accepts loopback (defense-in-depth; identity is still
// required) and the single configured MagicDNS host that `tailscale serve`
// forwards. The bind stays loopback elsewhere; this is a Host allowlist, not a
// bind change.
func (h *Handler) identityHostAllowed(hostport string) bool {
	if isLoopbackHost(hostport) {
		return true
	}
	allowed := strings.TrimSpace(h.config.AllowedHost)
	if allowed == "" {
		return false
	}
	return strings.EqualFold(hostOnly(hostport), hostOnly(allowed))
}
