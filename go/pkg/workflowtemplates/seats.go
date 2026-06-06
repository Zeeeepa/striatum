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
//     fixture. This is the honest default for claude today: it holds a seat, but
//     nothing catches the day a CLI version bump / trust-prompt change /
//     config-format shift breaks it (the precise re-rot RFC 0109 P3 closes).
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
//
// #190 (D174): agy is demoted `supported → degraded` again. The installed agy CLI
// (Antigravity 1.0.6) is OAuth-only — it has no headless/--login/API-key path — so
// the RFC 0109 P3 Installed CLI Gate stalls on an interactive login picker at the
// conformance step and the green fixture can no longer be produced. A seat returns
// to `supported` only via the RFC 0109 graduation gate once a headless auth path
// returns.
var degradedSeats = map[string]string{
	"agy": "agy CLI 1.0.6 (Antigravity) is OAuth-only with no headless/--login/API-key path; the RFC 0109 P3 installed-CLI gate stalls on an interactive login picker, so the supported seat fixture cannot be produced (#190)",
}

// supportedSeats are adapters with a green RFC 0109 P3 installed-CLI conformance
// fixture (adapterconformance.InstalledCLISeatFixtures). The graduation guard
// reconciles this set against that registry in both directions, so an adapter
// cannot be added here without its fixture, and a fixture cannot exist without
// graduating the seat — "the tier cannot lie."
//
//   - codex: green RFC 0109 P3 installed-CLI fixture
//     (TestInstalledCLISeatCodexTwoTurn: two-turn claim→publish→claim under one
//     attested session against the hermetic MCP harness).
//
// #190 (D174): agy was REMOVED from this set — its installed-CLI fixture can no
// longer go green because agy CLI 1.0.6 is OAuth-only and the gate stalls on a
// login picker. It is now `degraded` (degradedSeats); re-promotion is the RFC 0109
// graduation gate once a headless auth path returns.
var supportedSeats = map[string]struct{}{
	"codex": {},
}

// RegisterDegradedSeatForTest registers a degraded seat + reason for the lifetime
// of a test and returns a cleanup that restores the prior state. It is the seam
// that keeps the degraded_seat_lane lint's warning path under test now that the
// only production-degraded seat (agy) has graduated. Test-only; never call from
// production code.
func RegisterDegradedSeatForTest(adapter, reason string) func() {
	name := normalizeAdapterName(adapter)
	prev, had := degradedSeats[name]
	degradedSeats[name] = reason
	return func() {
		if had {
			degradedSeats[name] = prev
		} else {
			delete(degradedSeats, name)
		}
	}
}

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
