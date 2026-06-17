package adapterconformance

import (
	"testing"

	"github.com/halbritt/striatum/go/pkg/workflowtemplates"
)

// TestSupportedSeatsHaveInstalledCLIFixture is the RFC 0109 P3 graduation guard —
// the seat analog of the RFC 0106 TestSupportedShapesHaveReliabilityFixture. An
// adapter seat may carry support_tier=`supported` in workflowtemplates ONLY if it
// has a green installed-CLI conformance fixture registered in
// InstalledCLISeatFixtures (this package). It also asserts the reverse (every
// fixture-backed seat is marked supported), so the seat classification and the
// fixture registry can never silently drift apart — the seat tier cannot lie.
//
// Hermetic (no PG): it only compares two in-process registries, so it runs in the
// default `make -C go test` tier. A supported seat must stay backed by a green
// installed-CLI fixture, and a fixture-backed seat must be graduated in the
// workflow template catalog; either drift fails CI, which is the whole point of
// RFC 0109 P3.
func TestSupportedSeatsHaveInstalledCLIFixture(t *testing.T) {
	for _, adapter := range workflowtemplates.SupportedSeatAdapters() {
		if !InstalledCLISeatFixtures[adapter] {
			t.Errorf("adapter seat %q is marked support_tier=supported but has NO green installed-CLI conformance fixture (InstalledCLISeatFixtures) — the seat tier cannot lie; land the RFC 0109 P3 fixture (#149) or mark it degraded/experimental", adapter)
		}
		// #358: a declared-green bool is not enough — the fixture must be ACTUALLY
		// EXECUTED by the scheduled CI gate. Otherwise a "supported" tier could rest
		// on a fixture no job ever runs (the codex gap: its fixture existed but only
		// agy was -run in make installed-cli-check). Require CI execution too.
		if !CIExecutedInstalledCLISeats[adapter] {
			t.Errorf("adapter seat %q is marked support_tier=supported and has a fixture bool, but that fixture is NOT executed by the scheduled CI gate (CIExecutedInstalledCLISeats / make installed-cli-check) — a supported tier may not rest on a never-run fixture; wire it into the workflow's -run filter or mark the seat experimental", adapter)
		}
	}

	supported := map[string]bool{}
	for _, adapter := range workflowtemplates.SupportedSeatAdapters() {
		supported[adapter] = true
	}
	for adapter, green := range InstalledCLISeatFixtures {
		if !green {
			continue
		}
		if !supported[adapter] {
			t.Errorf("adapter seat %q has a green installed-CLI fixture but is NOT marked support_tier=supported — graduate it (workflowtemplates.supportedSeats) or remove the fixture entry", adapter)
		}
	}

	// Every CI-executed seat must have a fixture bool and be supported, so the two
	// registries cannot drift from the workflow's actual -run set.
	for adapter := range CIExecutedInstalledCLISeats {
		if !InstalledCLISeatFixtures[adapter] {
			t.Errorf("seat %q is in CIExecutedInstalledCLISeats but has no InstalledCLISeatFixtures entry — a CI-run fixture must be registered green", adapter)
		}
		if !supported[adapter] {
			t.Errorf("seat %q is CI-executed but not marked supported — graduate it or drop it from CIExecutedInstalledCLISeats", adapter)
		}
	}
}
