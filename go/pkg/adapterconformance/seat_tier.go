package adapterconformance

// InstalledCLISeatFixtures is the RFC 0109 P3 backing registry: the set of adapter
// seats (claude / codex / agy) with a GREEN installed-CLI conformance fixture — the
// real CLI driven through a two-turn `claim → publish → claim` asserting the same
// attested session across both turns (#95). It is the "cannot lie" truth source for
// the workflowtemplates seat support tier, exactly as ReliabilityFixtureShapes backs
// the RFC 0106 shape tier.
//
// It is intentionally EMPTY until the P3 installed-CLI runner lands (#149): no seat
// can honestly claim support_tier=`supported` before an installed-CLI gate proves
// it. The graduation guard (TestSupportedSeatsHaveInstalledCLIFixture) reconciles
// this registry against workflowtemplates.SupportedSeatAdapters() in both
// directions, so a seat cannot be marked supported without its fixture, and a
// fixture entry cannot be orphaned. When B1 (the installed-CLI runner) goes green
// for a seat, add it here AND graduate it in workflowtemplates.supportedSeats — the
// guard fails if they drift.
var InstalledCLISeatFixtures = map[string]bool{}
