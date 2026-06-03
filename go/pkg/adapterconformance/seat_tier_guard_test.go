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
// default `make -C go test` tier. Today both registries are empty (no seat has an
// installed-CLI gate yet, #149), so the guard passes vacuously — and the moment
// someone marks agy `supported` without landing its fixture, this fails CI, which
// is the whole point of RFC 0109 P3.
func TestSupportedSeatsHaveInstalledCLIFixture(t *testing.T) {
	for _, adapter := range workflowtemplates.SupportedSeatAdapters() {
		if !InstalledCLISeatFixtures[adapter] {
			t.Errorf("adapter seat %q is marked support_tier=supported but has NO green installed-CLI conformance fixture (InstalledCLISeatFixtures) — the seat tier cannot lie; land the RFC 0109 P3 fixture (#149) or mark it degraded/experimental", adapter)
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
}
