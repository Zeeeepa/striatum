package metrics

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// erroringQuerier fails every query, so the Phase A load-bearing fold in Refresh
// errors and exercises the publish-on-errored-tick path.
type erroringQuerier struct{}

func (erroringQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, fmt.Errorf("simulated fold query failure")
}

// TestSurrogateDeterministicBounded proves the per-repo surrogate is stable for a
// fixed (secret, repo_id), is bounded to [0, K), never echoes the input
// repository_id, and is salt-sensitive (a different daemon secret remaps the
// buckets) — the properties that make `bucket` a safe, unlinkable per-repo label.
func TestSurrogateDeterministicBounded(t *testing.T) {
	s := NewSurrogate("daemon-authority-secret")
	const repo = "repo_a89ecd1664764f039a127c62ab7da3f3"

	b1 := s.Bucket(repo)
	b2 := s.Bucket(repo)
	if b1 != b2 {
		t.Fatalf("surrogate is not deterministic: %q vs %q", b1, b2)
	}
	if strings.Contains(b1, repo) || strings.Contains(b1, "repo_") {
		t.Fatalf("surrogate bucket %q leaked the raw repository_id", b1)
	}
	// Bounded to [0, K).
	for _, id := range []string{repo, "", "repo_other", "x", strings.Repeat("z", 200)} {
		bucket := s.Bucket(id)
		var n int
		for _, r := range bucket {
			if r < '0' || r > '9' {
				t.Fatalf("bucket %q for %q is not a small integer", bucket, id)
			}
			n = n*10 + int(r-'0')
		}
		if n < 0 || n >= surrogateBuckets {
			t.Fatalf("bucket %d for %q is outside [0,%d)", n, id, surrogateBuckets)
		}
	}
	// Salt-sensitive: a different daemon secret produces a different mapping for at
	// least one of a small sample (the salt is what makes buckets non-portable
	// off-box). A nil surrogate degrades to a fixed bucket without panicking.
	other := NewSurrogate("a-different-daemon-secret")
	diff := false
	for _, id := range []string{"r1", "r2", "r3", "r4", "r5"} {
		if s.Bucket(id) != other.Bucket(id) {
			diff = true
			break
		}
	}
	if !diff {
		t.Fatalf("changing the daemon secret did not remap any bucket; surrogate is not salted")
	}
	var nilSurrogate *Surrogate
	if got := nilSurrogate.Bucket("anything"); got != "0" {
		t.Fatalf("nil surrogate Bucket = %q, want \"0\"", got)
	}
}

// TestConsentGatesProvenanceFamily is the deliverable #2 proof: a repo without
// consent emits no Provenance (repo_runs) series but DOES emit
// metrics_repo_consent{...}=0; a repo with consent emits both. The consent gate is
// enforced at fold time.
func TestConsentGatesProvenanceFamily(t *testing.T) {
	snap := Build(SnapshotInput{
		BuiltAt: sentinelBuiltAt,
		RepoMetrics: []RepoMetric{
			{RepoID: "repo_consented", Bucket: "3", Consented: true, RunStates: map[string]int{"running": 2}},
			{RepoID: "repo_dark", Bucket: "9", Consented: false, RunStates: map[string]int{"running": 5, "blocked": 1}},
		},
	})

	if got := snap.repoConsent["3"]; got != 1 {
		t.Errorf("consented bucket 3: consent gauge = %d, want 1", got)
	}
	if got := snap.repoConsent["9"]; got != 0 {
		t.Errorf("un-consented bucket 9: consent gauge = %d, want 0", got)
	}
	if got := snap.repoRuns[bucketState{bucket: "3", state: "running"}]; got != 2 {
		t.Errorf("consented bucket 3 repo_runs running = %d, want 2", got)
	}
	for k := range snap.repoRuns {
		if k.bucket == "9" {
			t.Errorf("un-consented bucket 9 leaked a repo_runs series %v; consent gate failed", k)
		}
	}
}

// TestScopedRenderHidesForeignBuckets is the deliverable #1 proof: a
// capability-scoped render (a non-nil allowed set) shows only the authorized
// repo's per-repo buckets while every repo-aggregate Operational family stays
// fully visible — a tailnet scraper holding only repo-A's token never sees
// repo-B's surrogate buckets.
func TestScopedRenderHidesForeignBuckets(t *testing.T) {
	snap := Build(SnapshotInput{
		BuiltAt:             sentinelBuiltAt,
		StrandedSupervisors: 4,
		RepoMetrics: []RepoMetric{
			{RepoID: "repo_a", Bucket: "3", Consented: true, RunStates: map[string]int{"running": 2}},
			{RepoID: "repo_b", Bucket: "9", Consented: true, RunStates: map[string]int{"running": 7}},
		},
	})

	var buf bytes.Buffer
	if err := snap.WriteTextScoped(&buf, sentinelNow, map[string]bool{"3": true}); err != nil {
		t.Fatalf("WriteTextScoped: %v", err)
	}
	body := buf.String()

	if !strings.Contains(body, `striatum_metrics_repo_consent{bucket="3"}`) {
		t.Errorf("authorized bucket 3 consent series missing from scoped body")
	}
	if !strings.Contains(body, `striatum_repo_runs{bucket="3",state="running"} 2`) {
		t.Errorf("authorized bucket 3 repo_runs series missing from scoped body")
	}
	if strings.Contains(body, `bucket="9"`) {
		t.Errorf("foreign bucket 9 leaked into a capability-scoped body:\n%s", body)
	}
	// Operational repo-aggregate families are always rendered regardless of scope.
	if !strings.Contains(body, "striatum_stranded_supervisors 4") {
		t.Errorf("operational family hidden by capability scope; want always visible")
	}
	if !strings.Contains(body, `striatum_metrics_tick_status{status="ok"} 1`) {
		t.Errorf("tick_status hidden by capability scope; want always visible")
	}

	// The default-open (loopback) render sees every bucket.
	var full bytes.Buffer
	if err := snap.WriteText(&full, sentinelNow); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if !strings.Contains(full.String(), `bucket="9"`) {
		t.Errorf("loopback full render dropped bucket 9; default-open must see all repos")
	}
}

// TestLifecycleBalanceCountsUnaccountedTerminal proves the OQ2 lifecycle-balance
// gauge increments for a terminal transition that declared a death the fold could
// not classify (a necrosis-tagged session.closed with an out-of-domain
// stall_class) and stays zero for intentional skips (a recovery_transfer close).
func TestLifecycleBalanceCountsUnaccountedTerminal(t *testing.T) {
	clean := Build(SnapshotInput{Events: []LifecycleEvent{
		{EventType: "session.closed"}, // clean apoptosis
		{EventType: "session.closed", LifecycleTag: LifecycleTagRecoveryTransfer}, // intentional skip
		{EventType: "session.closed", LifecycleTag: LifecycleTagNecrosis, StallClass: string(NecrosisAgentPIDDead)}, // accounted necrosis
	}})
	if clean.unaccountedTerminal != 0 {
		t.Errorf("lifecycle_balance = %d for fully-accounted events; want 0", clean.unaccountedTerminal)
	}

	blind := Build(SnapshotInput{Events: []LifecycleEvent{
		{EventType: "session.closed", LifecycleTag: LifecycleTagNecrosis, StallClass: "totally_unknown_stall"},
		{EventType: "session.closed", LifecycleTag: LifecycleTagNecrosis, StallClass: "another_unknown"},
	}})
	if blind.unaccountedTerminal != 2 {
		t.Errorf("lifecycle_balance = %d for two unaccounted deaths; want 2", blind.unaccountedTerminal)
	}
	// And it must not have leaked into necrosis.
	if len(blind.necrosis) != 0 {
		t.Errorf("an unclassifiable death leaked into necrosis: %v", blind.necrosis)
	}
}

// TestLivenessMissCanRecoverWithoutNecrosis (RFC 0137 Acceptance Criteria, F-A6):
// drives active -> liveness_deadline_missed -> liveness_recovered and asserts the
// reversible liveness-events counter moved, striatum_necrosis_total did NOT
// increment, and the OQ2 lifecycle-balance gauge stayed zero.
func TestLivenessMissCanRecoverWithoutNecrosis(t *testing.T) {
	snap := Build(SnapshotInput{Events: []LifecycleEvent{
		{EventType: "session.liveness_deadline_missed"},
		{EventType: "session.liveness_recovered"},
	}})

	if got := snap.livenessEvents[string(LivenessDeadlineMissed)]; got != 1 {
		t.Errorf("liveness_deadline_events_total{reason=deadline_missed} = %d, want 1", got)
	}
	if got := snap.livenessEvents[string(LivenessDeadlineRecovered)]; got != 1 {
		t.Errorf("liveness_deadline_events_total{reason=recovered} = %d, want 1", got)
	}
	if len(snap.necrosis) != 0 {
		t.Errorf("a reversible liveness miss incremented necrosis_total: %v", snap.necrosis)
	}
	if snap.unaccountedTerminal != 0 {
		t.Errorf("lifecycle_balance moved on a reversible liveness miss: %d, want 0", snap.unaccountedTerminal)
	}
}

// TestErroredTickSnapshotPreservesDataAndStampsError proves publish-on-errored-
// tick (deliverable #3 / OQ1): the carried-forward snapshot keeps the previous
// fold's data and builtAt — so the staleness SLI keeps climbing — while flipping
// tick_status to error. A nil previous snapshot yields an empty error-stamped
// surface.
func TestErroredTickSnapshotPreservesDataAndStampsError(t *testing.T) {
	prev := Build(SnapshotInput{
		BuiltAt:             sentinelBuiltAt,
		StrandedSupervisors: 5,
		TickStatus:          TickOK,
	})
	errored := erroredTickSnapshot(prev)

	if errored.TickStatus() != TickError {
		t.Errorf("errored tick status = %q, want error", errored.TickStatus())
	}
	if errored.strandedSupervisors != 5 {
		t.Errorf("errored tick dropped last-good data: stranded = %d, want 5", errored.strandedSupervisors)
	}
	if !errored.builtAt.Equal(sentinelBuiltAt) {
		t.Errorf("errored tick reset builtAt to %v; the staleness SLI must keep climbing from %v", errored.builtAt, sentinelBuiltAt)
	}
	// Age keeps growing relative to the preserved builtAt.
	if age := errored.ageSeconds(sentinelBuiltAt.Add(90 * time.Second)); age != 90 {
		t.Errorf("errored tick age = %v, want 90 (builtAt preserved)", age)
	}

	empty := erroredTickSnapshot(nil)
	if empty.TickStatus() != TickError {
		t.Errorf("nil-prev errored tick status = %q, want error", empty.TickStatus())
	}
}

// TestRefreshErroredTickPublishesErrorStatus drives Refresh against a runner whose
// load-bearing Phase A fold fails and asserts it (1) returns the error, (2)
// republishes the last-good data rather than dropping it, and (3) stamps
// tick_status=error so the failed tick is visible on /metrics — the end-to-end
// publish-on-errored-tick contract (deliverable #3).
func TestRefreshErroredTickPublishesErrorStatus(t *testing.T) {
	// Seed a healthy last-good snapshot.
	Publish(Build(SnapshotInput{BuiltAt: sentinelBuiltAt, StrandedSupervisors: 5, TickStatus: TickOK}))

	c := NewCollector(erroringQuerier{})
	if err := c.Refresh(context.Background(), sentinelNow); err == nil {
		t.Fatalf("Refresh against an erroring runner returned nil; want error")
	}

	snap := Load()
	if snap.TickStatus() != TickError {
		t.Errorf("after an errored tick, tick_status = %q, want error", snap.TickStatus())
	}
	if snap.strandedSupervisors != 5 {
		t.Errorf("errored tick dropped last-good data: stranded = %d, want 5", snap.strandedSupervisors)
	}

	var buf bytes.Buffer
	if err := snap.WriteText(&buf, sentinelNow); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if !strings.Contains(buf.String(), `striatum_metrics_tick_status{status="error"} 1`) {
		t.Errorf("errored-tick body missing tick_status=error:\n%s", buf.String())
	}
}
