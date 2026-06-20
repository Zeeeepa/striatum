package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/metrics"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// publishTwoRepoSnapshot publishes a snapshot with two consented repos mapped to
// explicit buckets so the capability-scoped handler has a deterministic
// repo_id -> bucket map to filter against.
func publishTwoRepoSnapshot(t *testing.T) {
	t.Helper()
	snap := metrics.Build(metrics.SnapshotInput{
		StrandedSupervisors: 7,
		RepoMetrics: []metrics.RepoMetric{
			{RepoID: "repo_alpha", Bucket: "3", Consented: true, RunStates: map[string]int{"running": 2}},
			{RepoID: "repo_beta", Bucket: "9", Consented: true, RunStates: map[string]int{"running": 4}},
		},
	})
	metrics.Publish(snap)
}

// readScopedToken builds a MemoryAuthorizer that grants the `read` capability,
// scoped to repoID (empty repoID = daemon-global read), to the returned token.
func readScopedToken(repoID string) (rpc.Authorizer, string) {
	authz := rpc.NewMemoryAuthorizer()
	token := "tok_" + strings.ReplaceAll(repoID, "_", "") + ".secret-" + repoID
	authz.AddToken(token, "client_"+repoID, map[rpc.Capability]rpc.CapabilityGrant{
		rpc.CapabilityRead: {RepositoryID: repoID},
	}, time.Now().Add(time.Hour))
	return authz, token
}

// TestScopedMetricsLoopbackServesFull asserts a loopback scrape is served in full
// (default-open, Phase A) regardless of any bearer.
func TestScopedMetricsLoopbackServesFull(t *testing.T) {
	publishTwoRepoSnapshot(t)
	authz, _ := readScopedToken("repo_alpha")
	h := newScopedMetricsHandler(metrics.NewCollector(nil).Handler(), authz)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("loopback status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `bucket="3"`) || !strings.Contains(body, `bucket="9"`) {
		t.Errorf("loopback full scrape missing a bucket; body:\n%s", body)
	}
}

// TestScopedMetricsRemoteFiltersByCapability is the deliverable #1 proof: a
// non-loopback scrape with a repo-alpha token sees only repo-alpha's bucket, never
// repo-beta's, while the aggregate Operational families stay visible.
func TestScopedMetricsRemoteFiltersByCapability(t *testing.T) {
	publishTwoRepoSnapshot(t)
	authz, token := readScopedToken("repo_alpha")
	h := newScopedMetricsHandler(metrics.NewCollector(nil).Handler(), authz)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "100.85.100.81:40000" // tailnet, non-loopback
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("scoped status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `striatum_metrics_repo_consent{bucket="3"}`) {
		t.Errorf("repo-alpha (bucket 3) consent series missing from scoped body:\n%s", body)
	}
	if !strings.Contains(body, `striatum_repo_runs{bucket="3",state="running"} 2`) {
		t.Errorf("repo-alpha (bucket 3) repo_runs series missing from scoped body:\n%s", body)
	}
	if strings.Contains(body, `bucket="9"`) {
		t.Errorf("repo-beta (bucket 9) leaked into a repo-alpha-scoped scrape:\n%s", body)
	}
	if !strings.Contains(body, "striatum_stranded_supervisors 7") {
		t.Errorf("aggregate Operational family hidden from an authorized remote scrape:\n%s", body)
	}
}

// TestScopedMetricsRemoteRequiresBearer asserts a non-loopback scrape without a
// bearer is rejected (401), and a bearer that authorizes none of the served repos
// is rejected (403) — fail closed beyond loopback.
func TestScopedMetricsRemoteRequiresBearer(t *testing.T) {
	publishTwoRepoSnapshot(t)
	authz, _ := readScopedToken("repo_alpha")
	h := newScopedMetricsHandler(metrics.NewCollector(nil).Handler(), authz)

	// No bearer.
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "100.85.100.81:40001"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("no-bearer remote scrape status = %d, want 401", rr.Code)
	}

	// Valid token, but it authorizes a repo this daemon does not serve.
	authzZ, tokenZ := readScopedToken("repo_unserved")
	hZ := newScopedMetricsHandler(metrics.NewCollector(nil).Handler(), authzZ)
	reqZ := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	reqZ.RemoteAddr = "100.85.100.81:40002"
	reqZ.Header.Set("Authorization", "Bearer "+tokenZ)
	rrZ := httptest.NewRecorder()
	hZ.ServeHTTP(rrZ, reqZ)
	if rrZ.Code != http.StatusForbidden {
		t.Errorf("unauthorized-repo remote scrape status = %d, want 403", rrZ.Code)
	}
}

// TestRequestIsLoopback covers the peer-address classifier's fail-closed behavior.
func TestRequestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:1234":      true,
		"[::1]:1234":          true,
		"100.85.100.81:1234":  false,
		"192.168.1.92:1234":   false,
		"":                    false,
		"garbage":             false,
	}
	for addr, want := range cases {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.RemoteAddr = addr
		if got := requestIsLoopback(req); got != want {
			t.Errorf("requestIsLoopback(%q) = %v, want %v", addr, got, want)
		}
	}
}
