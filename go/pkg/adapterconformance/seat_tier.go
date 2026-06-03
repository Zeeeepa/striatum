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
//     two-turn claim→publish→claim under one attested session, ×3), corroborated
//     live by the 3-lane needs_revision panel. codex is NOT here: it does not reach
//     work.claim against the in-process httptest harness (works live; its hermetic
//     MCP path is an RFC 0109 follow-up), so it stays `experimental`.
var InstalledCLISeatFixtures = map[string]bool{
	"agy": true,
}
