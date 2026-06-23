# Design-Run Seed (v2 / REVISION) - RFC 0165 Claude provider credential freshness

> **THIS IS THE SECOND REVISION.** The first RFC 0165 design run
> (`rfc-0165-design`, run `run_471c0965378cefa54c31015a74ad3162`) returned
> `needs_revision`. The holder's launch-time hydrator was useful, but the v1
> collaboration ledger left two material blockers open:
>
> - runtime credential expiry after launch still reached generic
>   `agent_mcp_discovery_stall` recovery before current provider-auth freshness
>   was classified;
> - the local-file copy model gave lanes raw Claude refresh-token custody, so
>   OAuth refresh-token rotation could invalidate or desynchronize the operator
>   source credential and future lanes.
>
> This v2 run is a real revision. The holder starts from the v1 `HOLDER.md` and
> the v1 `COLLABORATION_LEDGER_cycle_1.md`, revises the implementation spec to
> resolve all binding constraints below, and carries the useful v1 work forward
> unregressed. The falsifiers re-attack the revised spec. Do not treat the v1
> checkpoint as cleared.

This document is the required input for the RFC 0165 design run. It is
operator-supplied scaffolding, not the accepted design. The canonical proposal
is `docs/rfcs/0165-claude-provider-credential-freshness.md` and the active
tracker item is GitHub issue #583.

## Charter

Produce a falsifiable implementation spec for fixing #583: Claude lanes must not
wedge because lane-local provider credentials are stale, expired, or unsafe after
the operator Claude OAuth credential rotates.

The committed `PROPOSAL.md` must be buildable by a later code-change run. It
must name exact source modules, durable state surfaces, launch/recovery state
transitions, tests, and privacy boundaries. This is a design run, not an
implementation run. Do not change source code in this run. Do not close #583.

## V1 Finding Summary

The v1 ledger verdict is `needs_revision`. It records these binding constraints
for v2:

1. **C1-NO-LANE-RAW-REFRESH-TOKEN-CUSTODY.** Lanes must not receive or
   independently refresh raw Claude OAuth refresh tokens. A daemon-managed
   broker/access-token IPC design is the expected shape unless the revision
   proves an equally concrete OAuth-safe file model. If a broker is chosen, name
   the IPC/MCP surface, token lifetime, caller identity checks, and how the
   Claude CLI/SDK is configured to use it.
2. **C2-RUNTIME-FRESHNESS-RECOVERY-CLASSIFICATION.** Recovery must perform a
   current provider-auth freshness check before consuming generic requeue or
   transfer budget. Runtime-expired or near-expiry Claude auth becomes
   provider-auth debt, not ordinary lane flakiness.
3. **C3-HEARTBEAT-OR-BROKER-TELEMETRY-FOR-DECAY.** The design must name the
   daemon-owned signal that detects credential decay while a lane is running:
   broker-owned token state, trusted supervisor heartbeat telemetry, or an
   equivalent channel that does not trust lane-authored claims or provider
   stdout/stderr.
4. **C4-TEST-MATRIX-COVERS-LONG-RUN-AND-RTR.** The test plan must cover
   long-running lanes crossing expiry, concurrent refresh-token rotation,
   subsequent lane launch after one lane refreshes, operator-source validity, and
   redaction of OAuth material, private paths, provider output, and Striatum
   control-plane tokens.

## Preserve From V1

The revision should carry forward the v1 holder work where it remains valid:

- launch refusal before supervisor rows, scratch setup, capability-token mint,
  helper start, or a real Claude process;
- trusted operator/lane path resolution with workflow-authored path escape
  refusal;
- source-generation race checks for any remaining file or state hydration step;
- redacted custody receipts or broker-state receipts queryable by doctor and
  recovery;
- private-safe doctor/dashboard/operator messages;
- explicit separation between provider OAuth credentials and Striatum
  control-plane credentials.

## Required Design Decisions

Resolve these decisions concretely:

1. Credential custody model: daemon broker/access-token IPC, constrained file
   hydration, or another OAuth-safe design. State why it cannot corrupt the
   operator source credential during refresh-token rotation.
2. Claude CLI/SDK integration: how a supervised lane uses the credential without
   receiving a raw refresh token, or the exact proof that any remaining local
   credential file cannot rotate/desynchronize the operator token family.
3. Launch gate: the exact `supervise.start`/provider-auth preflight path that
   refuses before any real Claude process starts when current auth is missing,
   expired, near expiry, unverifiable, or unsafe to delegate.
4. Runtime signal: the trusted broker or heartbeat state that tracks access-token
   expiry/near-expiry while the lane is running.
5. Recovery classifier: where `agent_mcp_discovery_stall` and adjacent recovery
   paths consult current provider-auth freshness before generic retry counters
   are touched.
6. Durable state: schema/event contract for redacted custody, dependency, broker,
   or freshness receipts. State retention and query paths for doctor/dashboard.
7. Redaction: exactly what is persisted and what is forbidden, including raw
   OAuth bytes, access tokens, refresh tokens, id tokens, full private operator
   paths, provider stdout/stderr, and daemon/control-plane tokens.
8. `provider_auth_gate=off`: whether it can bypass the Claude safety boundary.
   If any emergency bypass exists, name it separately, make it operator-explicit,
   and explain the risk.
9. Freshness lead time: the minimum acceptable Claude credential expiry window
   before launch/runtime continuation, and whether it is config, a constant, or
   derived.
10. Operator remediation: the exact doctor/dashboard/operator-facing message that
    tells a human what to refresh without leaking secrets.

## Falsification Targets

Falsifiers should attack these risks directly:

- any lane custody of raw refresh tokens or lane-side refresh-token rotation;
- concurrent lanes sharing an initial operator credential generation;
- subsequent lane launch after one lane has refreshed or failed to refresh;
- long-running lane expiry after successful launch but before first model/MCP
  action;
- broker/heartbeat trust boundaries, spoofing, replay, and same-user collapse;
- fallback to generic MCP-discovery recovery without current freshness evidence;
- source/destination path escape if any local credential material remains;
- `provider_auth_gate=off` bypassing the correctness boundary;
- whether tests prove no raw credential material or private path material reaches
  repo artifacts, metrics, events, doctor output, dashboards, or GitHub comments.

## Current Source Anchors

Treat these anchors as design input and verify them before citing them as
current in the artifact:

- `go/pkg/mutations/supervision_launch.go` contains the `supervise.start` launch
  path that must refuse before process start.
- `go/pkg/mutations/supervision_provider_auth.go` runs the current
  provider-auth preflight gate before supervisor rows and provider launch.
- `go/pkg/laneproviderauth/lane_provider_auth.go`, `resolver.go`, and
  `expiry.go` contain the existing provider auth/resolver/expiry surfaces to
  reuse or deliberately extend.
- `go/pkg/mutations/recovery_liveness_oracle.go` and
  `go/pkg/sessionliveness/liveness.go` classify liveness failures including
  MCP-discovery stalls.
- `go/pkg/reads/doctor_lane_provider_auth.go` is the read-side precedent for
  provider-auth operator visibility.
- `docs/reference/command-authority-matrix.md` records the authority boundary:
  `supervise.start` is the daemon-backed launch authority.

## Acceptance For This Design Run

The design clears only if the final proposal contains:

- a source-anchored implementation order suitable for a TDD build run;
- an explicit answer to C1 through C4 from the v1 ledger;
- named tests for no lane raw refresh-token custody, concurrent RTR safety,
  subsequent launch after lane refresh, operator-source validity, long-run
  post-launch expiry, recovery-time freshness classification, redaction, and
  any remaining owner/mode/symlink/path checks;
- a durable state schema or event contract for redacted custody/broker/freshness
  receipts;
- explicit separation between provider OAuth credentials and Striatum
  control-plane credentials;
- a minimal closure scope for #583, with host timers/proximal pre-warm clearly
  deferred unless required for the synchronous launch/runtime invariant.

Do not close #583 from this run. The issue closes only after code lands on
`origin/main` and verifier evidence exists.
