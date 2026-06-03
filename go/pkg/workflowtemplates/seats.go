package workflowtemplates

import (
	"path/filepath"
	"sort"
	"strings"
)

// RFC 0109 adapter SEAT support tiers — the seat analog of the RFC 0106 shape
// support tiers (SupportTierForShape above). A "seat" is a declared adapter
// (claude / codex / agy) holding a supervised, multi-turn lane. The tier tells
// the truth about whether that seat survives an unattended multi-turn run:
//
//   - `supported`    — has a green RFC 0109 P3 installed-CLI conformance fixture
//     (adapterconformance.InstalledCLISeatFixtures). The graduation guard
//     (adapterconformance.TestSupportedSeatsHaveInstalledCLIFixture) enforces
//     that a seat may carry this tier ONLY with such a fixture, so — exactly as
//     for shapes (RFC 0105 backing) — the seat tier cannot lie.
//   - `experimental` — works in practice but is NOT gated by an installed-CLI
//     fixture. This is the honest default for claude/codex today: they hold a
//     seat, but nothing catches the day a CLI version bump / trust-prompt change
//     / config-format shift breaks them (the precise re-rot RFC 0109 P3 closes).
//   - `degraded`     — a known, tracked defect prevents a reliable supervised
//     multi-turn seat. agy is here (#95/#85/#76/#139).
//   - `unsupported`  — declared but not viable at all.
//
// The deferral RFC 0109 §B names ("two seats suffice, defer the third") is what
// keeps a broken seat costless. This taxonomy makes the cost a recorded,
// surfaced classification (workflowauthoring.lintDegradedSeatLane) instead of a
// silent collapse (#139).
const (
	SeatTierSupported    = "supported"
	SeatTierExperimental = "experimental"
	SeatTierDegraded     = "degraded"
	SeatTierUnsupported  = "unsupported"
)

// degradedSeats maps an adapter name to the operator-facing reason its supervised
// seat is degraded. The reason is surfaced verbatim in the lint warning so an
// operator reading a `run prepare` knows exactly which gap they are accepting.
// Keep entries tied to a tracked issue so "agy is broken" is never tribal
// knowledge that resets every umbrella (RFC 0109 §B).
var degradedSeats = map[string]string{
	"agy": "agy collapses after one supervised turn into an unattested duplicate session (#95), and gates at launch on a folder-trust / telemetry prompt the supervised PTY cannot dismiss (#76/#139); it is not yet a viable multi-turn seat (RFC 0109 P1)",
}

// supportedSeats are adapters with a green RFC 0109 P3 installed-CLI conformance
// fixture. It is intentionally EMPTY until P3 lands (#149): no seat can honestly
// claim `supported` before an installed-CLI gate proves it. The graduation guard
// in adapterconformance reconciles this set against InstalledCLISeatFixtures in
// both directions, so an adapter cannot be added here without its fixture, and a
// fixture cannot exist without graduating the seat.
var supportedSeats = map[string]struct{}{}

// normalizeAdapterName maps a (possibly absolute) lane argv0 to its bare adapter
// name, matching agentloop.LaneAdapterName (basename, drop a .exe suffix) without
// importing that package — workflowtemplates is a low-level package and must not
// pull in the agent-loop runtime.
func normalizeAdapterName(adapter string) string {
	base := filepath.Base(strings.TrimSpace(adapter))
	return strings.TrimSuffix(base, ".exe")
}

// SeatTierForAdapter returns the RFC 0109 support tier for an adapter seat. A
// path is reduced to its bare adapter name first. Unknown adapters are
// `experimental` (the honest default: unproven by an installed-CLI gate until a
// fixture says otherwise — never silently `supported`).
func SeatTierForAdapter(adapter string) string {
	name := normalizeAdapterName(adapter)
	if _, ok := supportedSeats[name]; ok {
		return SeatTierSupported
	}
	if _, ok := degradedSeats[name]; ok {
		return SeatTierDegraded
	}
	return SeatTierExperimental
}

// SeatDegradationReason returns the operator-facing reason an adapter's seat is
// degraded, or "" when the seat is not classified degraded.
func SeatDegradationReason(adapter string) string {
	return degradedSeats[normalizeAdapterName(adapter)]
}

// SupportedSeatAdapters returns the sorted set of adapters currently classified
// `supported`. The RFC 0109 graduation guard iterates this to assert each has a
// green installed-CLI conformance fixture (the seat tier cannot lie).
func SupportedSeatAdapters() []string {
	out := make([]string, 0, len(supportedSeats))
	for name := range supportedSeats {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
