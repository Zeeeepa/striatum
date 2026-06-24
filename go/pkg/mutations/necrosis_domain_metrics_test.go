package mutations

import (
	"testing"

	"github.com/halbritt/striatum/go/pkg/metrics"
)

// TestNecrosisDomainMatchesConfirmedDeadConstants is the RFC 0137 Phase B union
// guardrail. It anchors the metrics necrosis-reason enum to EXACTLY the
// confirmed-dead constants that live in THIS package, built from the real source
// constants (not string literals) so renaming a constant's value also breaks the
// test. It lives in package mutations because that is where the unexported
// stallClass* / recoveryExhaustedBlockerKind constants are visible; it imports
// metrics (the sanctioned mutations -> metrics direction; metrics never imports
// mutations, so there is no cycle).
//
// Effect: a NEW stall class cannot silently enter the necrosis label (it would
// have to be added to the metrics domain, which this test would then reject
// unless it is also a confirmed-dead constant here), and a recoverable class
// (liveness_deadline_missed, F-A6) can never re-enter it.
func TestNecrosisDomainMatchesConfirmedDeadConstants(t *testing.T) {
	want := map[string]bool{
		stallClassAgentPIDDead:                       true, // recovery_decision_tree.go
		stallClassAgentExitedUnsealed:                true, // recovery_decision_tree.go
		stallClassSessionUnrecoverableAcrossRotation: true, // recovery_decision_tree.go (RFC 0143 Slice A, #512)
		recoveryExhaustedBlockerKind:                 true, // recovery_escalation.go
	}

	got := map[string]bool{}
	for _, r := range metrics.NecrosisReasons() {
		got[string(r)] = true
	}

	for r := range want {
		if !got[r] {
			t.Errorf("metrics necrosis domain is MISSING confirmed-dead reason %q", r)
		}
	}
	for r := range got {
		if !want[r] {
			t.Errorf("metrics necrosis domain CONTAINS %q, which is not a confirmed-dead source constant (a new stall class silently entered necrosis)", r)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("necrosis domain size = %d, want exactly %d", len(got), len(want))
	}

	// F-A6 made explicit: the reversible liveness miss is NOT a necrosis reason.
	if got["liveness_deadline_missed"] {
		t.Error("F-A6 violated: liveness_deadline_missed is in the necrosis domain")
	}

	// The site-tag helper agrees with the metrics domain for the stall-class half
	// (recovery_exhausted is a blocker kind, not a stall class, so it is excluded
	// from this stall-class check by design).
	if !isNecrosisStallClass(stallClassAgentPIDDead) || !isNecrosisStallClass(stallClassAgentExitedUnsealed) {
		t.Error("isNecrosisStallClass must accept both confirmed-dead stall classes")
	}
	if isNecrosisStallClass("agent_protocol_idle_stall") {
		t.Error("isNecrosisStallClass must reject a reversible protocol stall class")
	}
}
