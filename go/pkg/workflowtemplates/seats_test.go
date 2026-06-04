package workflowtemplates

import (
	"sort"
	"strings"
	"testing"
)

func TestSeatTierForAdapterAgySupported(t *testing.T) {
	// agy graduated by the RFC 0109 P3 installed-CLI gate (#149): the real agy CLI
	// holds a two-turn claim→publish→claim under one attested session.
	if got := SeatTierForAdapter("agy"); got != SeatTierSupported {
		t.Fatalf("SeatTierForAdapter(agy) = %q, want %q", got, SeatTierSupported)
	}
	if reason := SeatDegradationReason("agy"); reason != "" {
		t.Fatalf("SeatDegradationReason(agy) must be empty now that agy is supported, got %q", reason)
	}
}

func TestSeatTierForAdapterCodexSupported(t *testing.T) {
	// codex graduated by the RFC 0109 P3 installed-CLI gate (#152): the real
	// codex CLI holds a two-turn claim→publish→claim under one attested session.
	if got := SeatTierForAdapter("codex"); got != SeatTierSupported {
		t.Fatalf("SeatTierForAdapter(codex) = %q, want %q", got, SeatTierSupported)
	}
	if reason := SeatDegradationReason("codex"); reason != "" {
		t.Fatalf("SeatDegradationReason(codex) must be empty now that codex is supported, got %q", reason)
	}
}

func TestSeatTierForAdapterDefaultsExperimental(t *testing.T) {
	// claude works in practice but is NOT yet backed by an installed-CLI fixture
	// (P3), so the honest tier is experimental — never silently `supported`.
	// This is the RFC 0109 thesis encoded as a default.
	for _, adapter := range []string{"claude", "unknown-future-cli"} {
		if got := SeatTierForAdapter(adapter); got != SeatTierExperimental {
			t.Errorf("SeatTierForAdapter(%q) = %q, want %q", adapter, got, SeatTierExperimental)
		}
		if SeatDegradationReason(adapter) != "" {
			t.Errorf("SeatDegradationReason(%q) must be empty for a non-degraded seat", adapter)
		}
	}
}

func TestSeatTierForAdapterNormalizesPath(t *testing.T) {
	if got := SeatTierForAdapter("/home/u/.local/bin/codex"); got != SeatTierSupported {
		t.Fatalf("SeatTierForAdapter(abs path) = %q, want %q (must basename argv0 like agentloop.LaneAdapterName)", got, SeatTierSupported)
	}
}

func TestSupportedSeatsAfterP3(t *testing.T) {
	// The graduation: agy and codex are `supported`, backed by installed-CLI
	// fixtures. The adapterconformance graduation guard
	// (TestSupportedSeatsHaveInstalledCLIFixture) enforces that backing.
	got := SupportedSeatAdapters()
	want := []string{"agy", "codex"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("SupportedSeatAdapters() = %v, want %v after the RFC 0109 P3 gates graduated agy and codex", got, want)
	}
}

func TestRegisterDegradedSeatForTest(t *testing.T) {
	// The test seam that keeps the degraded path covered now that no production
	// seat is degraded: register one, observe the tier + reason, restore.
	if SeatTierForAdapter("phantom-cli") != SeatTierExperimental {
		t.Fatalf("precondition: phantom-cli must be experimental")
	}
	cleanup := RegisterDegradedSeatForTest("phantom-cli", "phantom defect (#0000)")
	if got := SeatTierForAdapter("phantom-cli"); got != SeatTierDegraded {
		t.Fatalf("after register: SeatTierForAdapter(phantom-cli) = %q, want %q", got, SeatTierDegraded)
	}
	if SeatDegradationReason("phantom-cli") == "" {
		t.Fatal("after register: SeatDegradationReason(phantom-cli) must be non-empty")
	}
	// The degraded entry must not leak into the supported set.
	if adapters := SupportedSeatAdapters(); !sort.StringsAreSorted(adapters) {
		t.Fatalf("SupportedSeatAdapters() must stay sorted: %v", adapters)
	}
	cleanup()
	if got := SeatTierForAdapter("phantom-cli"); got != SeatTierExperimental {
		t.Fatalf("after cleanup: SeatTierForAdapter(phantom-cli) = %q, want %q", got, SeatTierExperimental)
	}
}
