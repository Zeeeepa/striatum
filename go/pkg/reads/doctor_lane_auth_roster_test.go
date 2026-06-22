package reads

import (
	"testing"

	"github.com/halbritt/striatum/go/pkg/laneproviderauth"
)

// TestReconcileLaneAuthRoster proves the three reconciliation classes and that
// every emitted record carries a STATIC snake_case check code (the only field the
// doctor_problems fold reads as the wire class).
func TestReconcileLaneAuthRoster(t *testing.T) {
	roster := laneproviderauth.Roster{Entries: []laneproviderauth.RosterEntry{
		{Lane: "codex", Provider: "codex", Kind: laneproviderauth.KindOAuth},
		{Lane: "claude", Provider: "claude", Kind: laneproviderauth.KindOAuth},
		{Lane: "agy", Provider: "agy", Kind: laneproviderauth.KindAPIKey},
	}}
	observed := LaneAuthObservedState{
		// codex is live; claude/agy emitted nothing (missing sample); an unrostered
		// "ghost" lane is live; claude is failing closed.
		LiveLanes:     map[string]bool{"codex": true, "ghost": true},
		MismatchLanes: map[string]bool{"claude": true},
	}

	records := ReconcileLaneAuthRoster(roster, observed)

	got := map[string]map[string]bool{}
	for _, rec := range records {
		check, _ := rec["check"].(string)
		lane, _ := rec["lane"].(string)
		if check == "" {
			t.Fatalf("record missing static check code: %#v", rec)
		}
		if got[check] == nil {
			got[check] = map[string]bool{}
		}
		got[check][lane] = true
	}

	// (b) claude + agy have no recent sample.
	if !got[checkLaneAuthMissingSample]["claude"] || !got[checkLaneAuthMissingSample]["agy"] {
		t.Fatalf("missing-sample not flagged for claude/agy: %#v", got[checkLaneAuthMissingSample])
	}
	if got[checkLaneAuthMissingSample]["codex"] {
		t.Fatalf("live codex lane wrongly flagged missing-sample")
	}
	// (a) ghost is live but unrostered.
	if !got[checkLaneAuthUnrosteredLiveLane]["ghost"] {
		t.Fatalf("unrostered live lane ghost not flagged: %#v", got[checkLaneAuthUnrosteredLiveLane])
	}
	if got[checkLaneAuthUnrosteredLiveLane]["codex"] {
		t.Fatalf("rostered codex lane wrongly flagged unrostered")
	}
	// (c) claude is failing closed.
	if !got[checkLaneAuthResolverMismatch]["claude"] {
		t.Fatalf("resolver mismatch not flagged for claude: %#v", got[checkLaneAuthResolverMismatch])
	}
}

// TestReconcileLaneAuthRosterCleanState yields no records when every roster lane
// is live and nothing is failing closed.
func TestReconcileLaneAuthRosterCleanState(t *testing.T) {
	roster := laneproviderauth.Roster{Entries: []laneproviderauth.RosterEntry{
		{Lane: "codex", Provider: "codex", Kind: laneproviderauth.KindOAuth},
	}}
	observed := LaneAuthObservedState{LiveLanes: map[string]bool{"codex": true}}
	if records := ReconcileLaneAuthRoster(roster, observed); len(records) != 0 {
		t.Fatalf("clean state produced %d records: %#v", len(records), records)
	}
}
