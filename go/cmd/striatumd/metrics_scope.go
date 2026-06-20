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
// one served repository; the response is then filtered to the per-repo series of
// the repositories that token authorizes, identified by their REAL repository_id,
// so a tailnet scraper holding only repo-A's token never sees repo-B's series —
// even when repo-A and repo-B happen to collide into the same salted surrogate
// bucket (the bucket is intentionally lossy; filtering on it would leak a colliding
// repo). The repo-aggregate Operational families are always rendered.
//
// This REUSES the same rpc.Authorizer that gates RPC — it never invents a parallel
// ACL. The authorized repository_ids are resolved from the already-published
// snapshot (Snapshot.RepoIDs), so the filtered path performs only RFC 0043
// authorization lookups (the same cost as one RPC auth per served repo, and a token
// authorizes few), never a metrics re-fold.
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
		allowed := authorizedRepos(authorizer, token, snap.RepoIDs())
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

// authorizedRepos resolves the set of repositories a token may see by asking the
// SAME authorizer that gates RPC whether the token holds the `read` capability for
// each repository the snapshot folded. Filtering is keyed by REAL repository_id —
// never the lossy salted bucket — so two repositories that collide into one
// surrogate bucket stay isolated: a repo-A token authorizes repo-A only, and the
// render emits repo-A's series under the shared bucket WITHOUT a colliding repo-B's
// data (RFC 0137 §4). A daemon-global read grant authorizes every repo; a
// repo-scoped token authorizes only its own. A token authorizes few repos, so
// resolving by id keeps cardinality fine.
func authorizedRepos(authorizer rpc.Authorizer, token string, repoIDs []string) map[string]bool {
	allowed := map[string]bool{}
	if authorizer == nil {
		return allowed
	}
	readCap := rpc.CapabilityRead
	for _, repoID := range repoIDs {
		ac := authorizer.Authorize(&readCap, repoID, token)
		if ac.Decision == "allowed" {
			allowed[repoID] = true
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
