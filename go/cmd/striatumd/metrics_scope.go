package main

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/metrics"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// newScopedMetricsHandler wraps the default-open /metrics handler with RFC 0137
// Phase D capability-scoped filtering (deliverable #1). A loopback scrape is
// served in FULL exactly as Phase A (the wrapped handler does a pure Load ->
// render -> write with zero DB queries). A scrape from beyond loopback must
// present a bearer whose RFC 0043 per-repo `read` capability authorizes at least
// one served repository; the response is then filtered to the salted surrogate
// buckets of the repositories that token authorizes, so a tailnet scraper holding
// only repo-A's token never sees repo-B's buckets. The repo-aggregate Operational
// families are always rendered.
//
// This REUSES the same rpc.Authorizer that gates RPC — it never invents a parallel
// ACL. The per-repo -> bucket mapping is read from the already-published snapshot
// (Snapshot.RepoBuckets), so the filtered path performs only RFC 0043
// authorization lookups (the same cost as one RPC auth), never a metrics re-fold.
func newScopedMetricsHandler(full http.Handler, authorizer rpc.Authorizer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default-open on loopback (Phase A). The peer address (RemoteAddr), not the
		// attacker-controllable Host header, is the trust signal: a loopback peer is
		// the local operator's Prometheus.
		if requestIsLoopback(r) {
			full.ServeHTTP(w, r)
			return
		}
		// Beyond loopback: require a bearer carrying the read capability.
		token, ok := bearerFromRequest(r)
		if !ok {
			http.Error(w, "metrics: Authorization bearer token required beyond loopback", http.StatusUnauthorized)
			return
		}
		snap := metrics.Load()
		if snap == nil {
			snap = &metrics.Snapshot{}
		}
		allowed := authorizedBuckets(authorizer, token, snap.RepoBuckets())
		if len(allowed) == 0 {
			// A valid bearer authorizes at least one served repository; none here
			// means the token carries no read capability for any repo this daemon
			// serves. Fail closed rather than exposing even the aggregate surface.
			http.Error(w, "metrics: token authorizes no served repository", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", metrics.ScrapeContentType())
		_ = snap.WriteTextScoped(w, time.Now().UTC(), allowed)
	})
}

// authorizedBuckets resolves the set of surrogate buckets a token may see by
// asking the SAME authorizer that gates RPC whether the token holds the `read`
// capability for each repository the snapshot folded. A repo the token is
// authorized for contributes its bucket to the allowed set. A daemon-global read
// grant authorizes every repo (so it sees all buckets); a repo-scoped token
// authorizes only its own.
func authorizedBuckets(authorizer rpc.Authorizer, token string, repoBuckets map[string]string) map[string]bool {
	allowed := map[string]bool{}
	if authorizer == nil {
		return allowed
	}
	readCap := rpc.CapabilityRead
	for repoID, bucket := range repoBuckets {
		ac := authorizer.Authorize(&readCap, repoID, token)
		if ac.Decision == "allowed" {
			allowed[bucket] = true
		}
	}
	return allowed
}

// bearerFromRequest extracts the bearer token from the Authorization header,
// matching the daemon's existing "Bearer <token>" syntax (go/pkg/mcp/http.go).
func bearerFromRequest(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", false
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

// requestIsLoopback reports whether the request's peer is a loopback address. It
// fails CLOSED: an empty or unparseable RemoteAddr is treated as NON-loopback, so
// an unidentifiable peer must authenticate rather than receive the default-open
// surface.
func requestIsLoopback(r *http.Request) bool {
	if r == nil {
		return false
	}
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
