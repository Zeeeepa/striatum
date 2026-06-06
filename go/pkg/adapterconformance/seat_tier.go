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
//   - codex: TestInstalledCLISeatCodexTwoTurn is green against the hermetic
//     httptest MCP harness once codex's workspace-trust prompt is answered and
//     the PTY-side daemon receiver is left off so codex's own MCP receive loop
//     owns every work.await_packet call.
//
// #190 (D174): agy was REMOVED. Its fixture (TestInstalledCLISeatAgyTwoTurn) can no
// longer go green because agy CLI 1.0.6 (Antigravity) is OAuth-only and the gate
// stalls on an interactive login picker at the conformance step; the fixture now
// detects that picker and skips-with-reason. agy's seat is reclassified `degraded`
// in workflowtemplates; the reconcile guard (TestSupportedSeatsHaveInstalledCLIFixture)
// requires both this registry and supportedSeats to drop agy together.
var InstalledCLISeatFixtures = map[string]bool{
	"codex": true,
}
