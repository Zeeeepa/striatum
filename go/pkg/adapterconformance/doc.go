package adapterconformance

// Slice-2 hand-off (DEFERRED — not built in this hermetic Tier-A slice).
//
// This package currently contains only the hermetic "Tier A" core (RFC 0101
// Phase 2, Slice 1): the closed failure taxonomy (taxonomy.go), the ordered
// C0..C12 contract (contract.go), and the two clauses that can be asserted with
// no live CLI, no daemon, and no PostgreSQL — C0 (clause_c0.go, the #101
// AdapterContract regression gate) and C2 (clause_c2.go, the #87 LaneEnvHardened
// golden). Tier A rides inside `make -C go check` via go test on this package.
//
// The following components are DEFERRED to Slice 2. They carry the C1/C3..C12
// live assertions and require a live/fake-agent runner; none of them are faked
// in Slice 1 (the deferred clauses return StatusDeferred):
//
//   - runner.go — Run(ctx, AdapterSpec, ContractProfile, Mode) Report: provisions
//     an isolated PostgreSQL DB via go/pkg/pgtest, boots a REAL in-process
//     striatumd (production RPC handlers + go/pkg/mcp HTTP server) on loopback
//     with an ephemeral 0600 token, runs C1 pre-flight, seeds a single-job fixture
//     workflow, launches the lane via the production supervise.start -> RunHelper
//     path, drives the scripted interrogation, evaluates each live clause against
//     the observer, applies the post-green bounded retry (§2.7), and tears down +
//     runs the C12 hygiene scan.
//   - observer.go — DaemonObserver: thin reads over the same daemon the production
//     stack uses (run.detail / supervise.status + sessionliveness.ProjectionFromRow
//     over the session row + artifacts/jobs reads + PTY-helper event metadata),
//     never PTY text.
//   - preflight.go — the C1 health/auth probe per adapter (resolve binary +
//     --version + a cheap auth/liveness check) -> Ready / AdapterUnavailable /
//     AdapterVersionUnknown / AdapterUnauthenticated / AdapterProviderUnavailable
//     without burning a full agent turn.
//   - fixture.go — minimal validation-passing single-job workflow/role/prompt/
//     expected-artifact builder (document_only needs inputs; review lane differs
//     in model family from author; branch/--print shapes honored).
//   - skipledger.go + conformance_skips.json — the non-rotting agy skip ledger
//     (reject a skip missing promote_when; version-gated + issue-gated promotion;
//     UnexpectedKnownGapPass on a lucky pass).
//   - testagent/ — one configurable hermetic fake agent with the broken modes
//     (never_tools_list, discovery_probe, exit_before_complete,
//     ignore_interrogation, no_heartbeat, leak_token, leak_helper, env_secret) +
//     a happy-path mode that drives the full fixture against the real daemon, so
//     each mode yields exactly its contract FailureClass (taxonomy is armed).
//   - go/cmd/striatum-adapter-conformance/main.go — the driver binary: flags
//     --adapters/--timeout/--json-report/--ci/--release/--promote-agy-multiturn,
//     the matrix run, dist/adapter-conformance.json, the human per-clause stdout
//     summary, and the mode-aware outcome-distinguished exit code (§1.6: exit 1
//     contract / 75 provider-transient / 0 pass).
//
// The C1 and C3..C12 live assertions belong in Slice 2; in Slice 1 those clauses
// are declared in Contract() with their ID/Deadline/Profile/OnFailure and a
// deferred Assert that returns StatusDeferred (never a faked result).
