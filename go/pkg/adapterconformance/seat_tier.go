package adapterconformance

// InstalledCLISeatFixtures is the RFC 0109 P3 backing registry: the set of adapter
// seats (claude / codex / agy) with a GREEN installed-CLI conformance fixture — the
// real CLI driven through a two-turn `claim → publish → claim` asserting the same
// attested session across both turns (#95). It is the "cannot lie" truth source for
// the workflowtemplates seat support tier, exactly as ReliabilityFixtureShapes backs
// the RFC 0106 shape tier.
//
// The graduation guard (TestSupportedSeatsHaveInstalledCLIFixture) reconciles this
// registry against workflowtemplates.SupportedSeatAdapters() in both directions, so
// a seat cannot be marked supported without its fixture, and a fixture entry cannot
// be orphaned. When the installed-CLI runner goes green for a seat, add it here AND
// graduate it in workflowtemplates.supportedSeats — the guard fails if they drift.
//
//   - agy: TestInstalledCLISeatAgyTwoTurn is green (the real agy CLI drives a
//     two-turn claim→publish→claim under one attested session), corroborated live
//     by the 3-lane needs_revision panel.
//   - codex: TestInstalledCLISeatCodexTwoTurn is green against the hermetic
//     httptest MCP harness once codex's workspace-trust prompt is answered and
//     the PTY-side daemon receiver is left off so codex's own MCP receive loop
//     owns every work.await_packet call.
var InstalledCLISeatFixtures = map[string]bool{
	"agy":   true,
	"codex": true,
}
