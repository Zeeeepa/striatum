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
//   - agy: TestInstalledCLISeatAgyTwoTurn is green against the real agy CLI.
//   - codex: TestInstalledCLISeatCodexTwoTurn is green against the hermetic
//     httptest MCP harness once codex's workspace-trust prompt is answered and
//     the PTY-side daemon receiver is left off so codex's own MCP receive loop
//     owns every work.await_packet call.
var InstalledCLISeatFixtures = map[string]bool{
	"agy":   true,
	"codex": true,
}

// CIExecutedInstalledCLISeats names the seats whose installed-CLI fixture is
// ACTUALLY RUN by the scheduled CI gate (make installed-cli-check ->
// .github/workflows/ci-installed-cli.yml), not merely declared green by a bool in
// InstalledCLISeatFixtures. The original guard reconciled registry<->registry, so a
// seat could be marked supported with a fixture that no CI job ever executed —
// exactly the codex gap #358 found (TestInstalledCLISeatCodexTwoTurn existed but
// only the agy fixture was -run in the Makefile). The graduation guard now requires
// every supported seat to be in BOTH InstalledCLISeatFixtures AND this set, so a
// "supported" tier cannot rest on a never-CI-run fixture.
//
// When you wire a seat's fixture into make installed-cli-check (and the scheduled
// workflow), add it here. Keep this in lockstep with the Makefile's -run filter.
var CIExecutedInstalledCLISeats = map[string]bool{
	"agy":   true,
	"codex": true,
}
