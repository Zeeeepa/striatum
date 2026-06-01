package adapterconformance

// Build state across RFC 0101 Phase 2 slices.
//
// Slice 1 (shipped) — the hermetic "Tier A" core: the closed failure taxonomy
// (taxonomy.go), the ordered C0..C12 contract (contract.go), and the two clauses
// asserted with no live CLI, no daemon, and no PostgreSQL — C0 (clause_c0.go,
// the #101 AdapterContract regression gate) and C2 (clause_c2.go, the #87
// LaneEnvHardened golden). Tier A rides inside `make -C go check`.
//
// Slice 2 (shipped — THIS code) — the fake-agent lifecycle conformance runner.
// It turns the daemon-lifecycle clauses C3..C10 (+ C7) into LIVE asserts driven
// by an in-process daemon over pgtest, and arms the taxonomy via broken-mode
// fixtures. These files carry it:
//
//   - harness.go — NewHarness: provisions an isolated, migrated PostgreSQL DB via
//     go/pkg/pgtest and assembles the SAME production stack striatumd boots —
//     rpc.NewServer() with reads.Register + mutations.Register +
//     repositories.Service.Register over the pgtest runner, wrapped in
//     mcp.NewHTTPHandler(mcp.Service{RPC, Authorizer, ActivityRecorder:
//     sessionliveness.DBRecorder}) and served on an httptest loopback with an
//     ephemeral read/claim/write capability token. NOT the live daemon or live PG.
//   - testagent/agent.go — one configurable fake MCP agent that speaks JSON-RPC
//     (initialize -> tools/list -> tools/call ...) to the harness MCP endpoint
//     with the bearer token. A happy-path mode drives the full single-job
//     lifecycle (register -> await_packet -> ack -> heartbeat -> answer one
//     interrogation -> publish -> complete -> await(no_work)); the broken modes
//     each ARM exactly one contract FailureClass (never_tools_list ->
//     BootstrapStall, await_never -> AwaitPacketStall, ack_never -> AckStall,
//     no_heartbeat -> HeartbeatMissed, ignore_interrogation -> InterrogationIgnored,
//     exit_before_complete -> CompleteMissing/TurnExitEarly).
//   - observer.go — DaemonObserver: reads daemon state to evaluate the clauses —
//     sessionliveness.ProjectionFromRow over the session row (last_tools_list_at,
//     last_await_packet_at, last_ack_at, last_work_heartbeat_at,
//     last_work_complete_at), the active lease row, job state, the artifacts row
//     (C8), and the interrogation answer turn (C7). Reads only; never PTY text.
//   - fixture.go — a validation-passing single-job document_only/draft workflow
//     seeded into a temp target repo, with a pending work message so the claim
//     returns immediately and a front-matter progress_note expected artifact.
//   - runner.go — RunLive: drives one fake-agent lifecycle and evaluates C3..C10
//     (+ C7 when scoped) from the DaemonObserver using the Slice-1 deadlines
//     (DefaultPolicy x STRIATUM_CONFORMANCE_DEADLINE_SCALE). Each unmet clause
//     records its FailureClass + structured routing fields.
//   - runner_test.go — Tier-A taxonomy-arming self-tests: the happy path (all
//     in-scope clauses Pass) and each broken mode (asserting exactly the expected
//     FailureClass). PG-gated; they skip cleanly without STRIATUM_PG_TEST_URL.
//
// DEFERRED to a follow-up slice (NOT built here):
//
//   - C1 real-CLI pre-flight (preflight.go) — resolve the configured binary on the
//     supervised PATH + --version + a cheap auth/liveness probe -> Ready /
//     AdapterUnavailable / AdapterVersionUnknown / AdapterUnauthenticated /
//     AdapterProviderUnavailable, the infra-outcome gate (§2.7). C1 stays Deferred.
//   - Launching the testagent through supervise.start -> go/pkg/supervisor
//     RunHelper (the real PTY/argv bootstrap path) for C11 (NoPrematureExit). This
//     slice drives the agent in-process for determinism; the PTY path is
//     Slice-2b. C11 stays Deferred.
//   - C12 work-tree credential scan (filesystem + git, post-teardown) for the #70
//     bearer/.gemini residue and lane-authored control-plane helpers. C12 stays
//     Deferred.
//   - skipledger.go + conformance_skips.json — the non-rotting agy skip ledger
//     (reject a skip missing promote_when; version-gated + issue-gated promotion;
//     UnexpectedKnownGapPass on a lucky pass).
//   - go/cmd/striatum-adapter-conformance/main.go — the driver binary (flags,
//     dist/adapter-conformance.json, human per-clause summary, mode-aware
//     outcome-distinguished exit code) + the Make/CI Tier-B wiring.
//
// LiveClauses() and StillDeferredClauses() (contract.go) enumerate the split.
