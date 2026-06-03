package workflowtemplates

import "testing"

func TestSeatTierForAdapterAgyDegraded(t *testing.T) {
	if got := SeatTierForAdapter("agy"); got != SeatTierDegraded {
		t.Fatalf("SeatTierForAdapter(agy) = %q, want %q", got, SeatTierDegraded)
	}
	if reason := SeatDegradationReason("agy"); reason == "" {
		t.Fatal("SeatDegradationReason(agy) must name the tracked defect for the operator warning")
	}
}

func TestSeatTierForAdapterDefaultsExperimental(t *testing.T) {
	// claude/codex work in practice but are NOT yet backed by an installed-CLI
	// fixture (P3, #149), so the honest tier is experimental — never silently
	// `supported`. This is the RFC 0109 thesis encoded as a default.
	for _, adapter := range []string{"claude", "codex", "unknown-future-cli"} {
		if got := SeatTierForAdapter(adapter); got != SeatTierExperimental {
			t.Errorf("SeatTierForAdapter(%q) = %q, want %q", adapter, got, SeatTierExperimental)
		}
		if SeatDegradationReason(adapter) != "" {
			t.Errorf("SeatDegradationReason(%q) must be empty for a non-degraded seat", adapter)
		}
	}
}

func TestSeatTierForAdapterNormalizesPath(t *testing.T) {
	if got := SeatTierForAdapter("/home/u/.local/bin/agy"); got != SeatTierDegraded {
		t.Fatalf("SeatTierForAdapter(abs path) = %q, want %q (must basename argv0 like agentloop.LaneAdapterName)", got, SeatTierDegraded)
	}
}

func TestNoSeatSupportedUntilP3(t *testing.T) {
	// The cannot-lie invariant: no seat may be `supported` until its installed-CLI
	// fixture lands (#149). If this changes, the adapterconformance graduation
	// guard must already enforce the fixture backing.
	if got := SupportedSeatAdapters(); len(got) != 0 {
		t.Fatalf("SupportedSeatAdapters() = %v, want empty until an installed-CLI fixture (RFC 0109 P3) graduates a seat", got)
	}
}
