package workflowtemplates

import (
	"sort"
	"strings"
	"testing"
)

func TestSeatTierForAdapterAgyDegraded(t *testing.T) {
	// #190 (D174): agy is demoted supported → degraded. agy CLI 1.0.6 (Antigravity)
	// is OAuth-only with no headless/--login/API-key path, so the RFC 0109 P3
	// installed-CLI gate stalls on an interactive login picker and the green seat
	// fixture can no longer be produced. Re-promotion is the RFC 0109 graduation
	// gate once a headless auth path returns.
	if got := SeatTierForAdapter("agy"); got != SeatTierDegraded {
		t.Fatalf("SeatTierForAdapter(agy) = %q, want %q (#190)", got, SeatTierDegraded)
	}
	reason := SeatDegradationReason("agy")
	if reason == "" {
		t.Fatal("SeatDegradationReason(agy) must be non-empty now that agy is degraded (#190)")
	}
	if !strings.Contains(reason, "OAuth") && !strings.Contains(reason, "1.0.6") {
		t.Fatalf("SeatDegradationReason(agy) must cite the OAuth-only 1.0.6 cause: %q", reason)
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

func TestSupportedSeatsAfterAgyDemotion(t *testing.T) {
	// #190 (D174): codex remains the only `supported` seat, backed by its
	// installed-CLI fixture. agy was demoted to degraded (OAuth-only 1.0.6). The
	// adapterconformance graduation guard (TestSupportedSeatsHaveInstalledCLIFixture)
	// enforces that the supported set and InstalledCLISeatFixtures stay in lockstep.
	got := SupportedSeatAdapters()
	want := []string{"codex"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("SupportedSeatAdapters() = %v, want %v after agy was demoted (#190)", got, want)
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
