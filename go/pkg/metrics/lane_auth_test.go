package metrics

import (
	"fmt"
	"strings"
	"testing"
)

// TestThresholdFromRosterNotObserved is FA-4 at the fold layer: the exported
// threshold gauges carry the OPERATOR-DECLARED roster value, invariant to the
// observed/sampled credential lifetime — a degrading credential cannot shrink its
// own goalposts.
func TestThresholdFromRosterNotObserved(t *testing.T) {
	snap := Build(SnapshotInput{
		BuiltAt: sentinelBuiltAt,
		LaneAuthRoster: []LaneRosterObservation{
			{Lane: "codex", Provider: "codex", Kind: "oauth", StalenessThresholdSeconds: 9000, ExpiryLeadSeconds: 3600},
		},
		// A sample whose credential is about to expire (tiny seconds_to_expiry)
		// must NOT pull the exported threshold down toward the observed value.
		LaneCredSamples: []LaneCredSampleObservation{
			{Lane: "codex", Kind: "oauth", HasExpiry: true, SecondsToExpiry: 5, AgeSeconds: 999999},
		},
	})
	body := string(mustRender(t, snap))

	if !strings.Contains(body, metricLaneAuthStalenessThresh+`{lane="codex"} 9000`) {
		t.Fatalf("staleness threshold not the roster value 9000:\n%s", body)
	}
	if !strings.Contains(body, metricLaneCredExpiryLead+`{lane="codex"} 3600`) {
		t.Fatalf("expiry lead not the roster value 3600:\n%s", body)
	}
}

// TestLaneAuthSeriesMissingNamesMissingLane is FA-F1: the census expected vector
// is per-lane (no aggregate-only rule), so a rostered lane with no observed
// sample is still named by its `lane` label — the absence rule
// `expected unless on(lane) (sample_present == 1)` can fire WITH the lane label.
func TestLaneAuthSeriesMissingNamesMissingLane(t *testing.T) {
	snap := Build(SnapshotInput{
		BuiltAt: sentinelBuiltAt,
		LaneAuthRoster: []LaneRosterObservation{
			{Lane: "codex", Provider: "codex", Kind: "oauth"},
			{Lane: "claude", Provider: "claude", Kind: "oauth"},
		},
		// Only codex was observed this sweep; claude's credential vanished.
		LaneCredSamples: []LaneCredSampleObservation{
			{Lane: "codex", Kind: "oauth", HasExpiry: true, SecondsToExpiry: 3600, AgeSeconds: 60},
		},
	})
	body := string(mustRender(t, snap))

	// Both lanes are in the expected vector, each carrying its own lane label.
	if !strings.Contains(body, metricLaneAuthExpected+`{kind="oauth",lane="claude",provider="claude"} 1`) {
		t.Fatalf("missing lane claude is not named in the expected vector:\n%s", body)
	}
	if !strings.Contains(body, metricLaneAuthExpected+`{kind="oauth",lane="codex",provider="codex"} 1`) {
		t.Fatalf("expected vector missing codex:\n%s", body)
	}
	// claude has NO sample_present, so the absence rule names it; codex has one.
	if strings.Contains(body, metricLaneCredSamplePresent+`{kind="oauth",lane="claude"}`) {
		t.Fatalf("claude must have no sample_present (it is the missing lane):\n%s", body)
	}
	if !strings.Contains(body, metricLaneCredSamplePresent+`{kind="oauth",lane="codex"} 1`) {
		t.Fatalf("codex must emit sample_present:\n%s", body)
	}
}

// TestScalarCountCannotMaskRosterMismatch is FA-F1: a healthy non-codex api_key
// lane is both IN the expected set and emits sample_present (so it is covered, not
// silently dropped) while carrying NO expiry series — and a per-lane vector, not a
// scalar count, is what makes a single missing lane visible.
func TestScalarCountCannotMaskRosterMismatch(t *testing.T) {
	snap := Build(SnapshotInput{
		BuiltAt: sentinelBuiltAt,
		LaneAuthRoster: []LaneRosterObservation{
			{Lane: "codex", Provider: "codex", Kind: "oauth"},
			{Lane: "agy", Provider: "agy", Kind: "api_key"},
		},
		LaneCredSamples: []LaneCredSampleObservation{
			{Lane: "codex", Kind: "oauth", HasExpiry: true, SecondsToExpiry: 3600, AgeSeconds: 60},
			{Lane: "agy", Kind: "api_key", HasExpiry: false},
		},
	})
	body := string(mustRender(t, snap))

	// The api_key lane is census-covered: present in expected AND sample_present.
	if !strings.Contains(body, metricLaneAuthExpected+`{kind="api_key",lane="agy",provider="agy"} 1`) {
		t.Fatalf("healthy api_key lane dropped from expected vector:\n%s", body)
	}
	if !strings.Contains(body, metricLaneCredSamplePresent+`{kind="api_key",lane="agy"} 1`) {
		t.Fatalf("healthy api_key lane emits no sample_present (would silently drop):\n%s", body)
	}
	// The expected vector has two distinct lane series — a scalar count alone could
	// not name which lane is missing; the per-lane vector can.
	if got := countSeries(body, metricLaneAuthExpected); got != 2 {
		t.Fatalf("expected vector series = %d; want 2 distinct per-lane series:\n%s", got, body)
	}
}

// TestNoExpiryCredentialDoesNotSatisfyExpiryCensus is FA-F1: an api_key lane emits
// sample_present (census coverage) but NEVER a seconds_to_expiry series — it
// cannot be forced to produce expiry telemetry it does not have, nor be dropped
// for lacking it.
func TestNoExpiryCredentialDoesNotSatisfyExpiryCensus(t *testing.T) {
	snap := Build(SnapshotInput{
		BuiltAt: sentinelBuiltAt,
		LaneAuthRoster: []LaneRosterObservation{
			{Lane: "agy", Provider: "agy", Kind: "api_key"},
		},
		LaneCredSamples: []LaneCredSampleObservation{
			{Lane: "agy", Kind: "api_key", HasExpiry: false},
		},
	})
	body := string(mustRender(t, snap))

	if !strings.Contains(body, metricLaneCredSamplePresent+`{kind="api_key",lane="agy"} 1`) {
		t.Fatalf("api_key lane must emit sample_present:\n%s", body)
	}
	if strings.Contains(body, metricLaneCredSecondsToExpiry+`{kind="api_key",lane="agy"}`) {
		t.Fatalf("api_key lane must NOT emit a seconds_to_expiry series:\n%s", body)
	}
	if strings.Contains(body, metricLaneCredAgeSeconds+`{kind="api_key",lane="agy"}`) {
		t.Fatalf("api_key lane must NOT emit a cred_age series:\n%s", body)
	}
}

// TestLaneCredSeriesBudget is FA-3 extended to the lane-auth families: feeding 100
// synthetic lanes bounds every lane-keyed family at the budget+1 (the lane="other"
// sentinel) and surfaces the clip via striatum_metrics_cardinality_clipped_total.
func TestLaneCredSeriesBudget(t *testing.T) {
	roster := make([]LaneRosterObservation, 0, 100)
	samples := make([]LaneCredSampleObservation, 0, 100)
	mismatches := make([]LaneResolverMismatchObservation, 0, 100)
	for i := 0; i < 100; i++ {
		lane := fmt.Sprintf("lane_%03d", i)
		roster = append(roster, LaneRosterObservation{Lane: lane, Provider: "codex", Kind: "oauth"})
		samples = append(samples, LaneCredSampleObservation{Lane: lane, Kind: "oauth", HasExpiry: true, SecondsToExpiry: float64(i), AgeSeconds: float64(i)})
		mismatches = append(mismatches, LaneResolverMismatchObservation{Lane: lane, Kind: "oauth"})
	}
	snap := Build(SnapshotInput{BuiltAt: sentinelBuiltAt, LaneAuthRoster: roster, LaneCredSamples: samples, LaneResolverMismatches: mismatches})
	body := string(mustRender(t, snap))

	want := laneAuthSeriesBudget + 1 // kept + the lane="other" sentinel
	for _, fam := range []string{metricLaneAuthExpected, metricLaneCredSamplePresent, metricLaneCredResolverMismatch, metricLaneCredSecondsToExpiry, metricLaneCredAgeSeconds} {
		if got := countSeries(body, fam); got != want {
			t.Fatalf("%s series = %d; want %d (budget+other)", fam, got, want)
		}
	}
	if !strings.Contains(body, metricLaneAuthExpected+`{kind="other",lane="other",provider="other"} 1`) {
		t.Fatalf("expected vector overflow did not collapse to lane=\"other\":\n%s", body)
	}
	clipped := 100 - laneAuthSeriesBudget
	if !strings.Contains(body, metricCardinalityClipped+`{family="`+clipFamilyLaneAuthExpected+`"} `+fmt.Sprintf("%d", clipped)) {
		t.Fatalf("cardinality clip counter did not report %d clipped lanes for expected:\n%s", clipped, body)
	}
	if !strings.Contains(body, metricCardinalityClipped+`{family="`+clipFamilyLaneCredResolverMismatch+`"} `+fmt.Sprintf("%d", clipped)) {
		t.Fatalf("cardinality clip counter did not report resolver_mismatch clips:\n%s", body)
	}
}
