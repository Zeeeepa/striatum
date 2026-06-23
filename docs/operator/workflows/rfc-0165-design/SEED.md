# Design-Run Seed - RFC 0165 Claude provider credential freshness

This document is the required input for the RFC 0165 design run. It is
operator-supplied scaffolding, not the accepted design. The canonical proposal
is `docs/rfcs/0165-claude-provider-credential-freshness.md` and the active
tracker item is GitHub issue #583.

## Charter

Produce a falsifiable implementation spec for fixing #583: Claude lanes must
not wedge because their lane-local OAuth credential copy is stale after the
operator credential rotates.

The committed `PROPOSAL.md` must be buildable by a later code-change run. It
must name the exact source modules, durable state surface, launch/recovery
state transitions, tests, and privacy boundaries needed to make spawn-time
Claude credential freshness a real invariant.

This is a design run, not an implementation run. Do not change source code in
this run. Publish only the expected artifacts declared in the work packet.

## Incident Frame

The failure behind #583 was a Claude lane whose copied credential under the lane
OS user's home went stale after the operator's Claude OAuth credential rotated.
New Claude lanes then failed during MCP discovery as
`agent_mcp_discovery_stall`; recovery retried the doomed launch until budget was
exhausted. The operator had to manually copy the fresh operator credential into
the lane home, fix ownership and mode, resolve escalation, and re-drive.

The fix must prevent this before a real Claude process starts. Detection-only
work is insufficient.

## Required Design Decisions

Resolve these decisions concretely:

1. Hydration point: the exact launch and recovery path where Claude credential
   hydration and verification happens before the provider CLI starts.
2. Source and destination resolution: how the operator-side and lane-side
   Claude credential paths are resolved without accepting workflow-authored or
   arbitrary paths.
3. Rotation race: how the copy observes source generation before and after
   hydration and refuses or retries if the operator credential changes during
   the copy.
4. Custody storage: whether receipts are structured events, a table, a
   provider-auth dependency table, or a hybrid. State the retention model.
5. Redaction: exactly what is persisted and what is forbidden, including raw
   OAuth bytes, access tokens, refresh tokens, id tokens, full private operator
   paths, provider stdout/stderr, and daemon/control-plane tokens.
6. `provider_auth_gate=off`: whether it can bypass Claude hydration. If an
   emergency bypass exists, name it separately and explain the risk.
7. Recovery circuit breaker: how stale or unverifiable provider auth becomes
   `reseed_required` or equivalent readiness debt instead of another generic
   MCP discovery stall retry.
8. Freshness lead time: the minimum acceptable Claude credential expiry window
   before launch, and whether it is config, a constant, or derived.
9. Operator remediation: the exact doctor/dashboard/operator-facing message
   that tells a human what to refresh without leaking secrets.

## Falsification Targets

Falsifiers should attack these risks directly:

- Copy/verify race during operator Claude OAuth rotation.
- Hydrator privilege bridge or path escape from source/destination resolution.
- Custody receipts as events vs a table, especially queryability for doctor and
  recovery.
- `provider_auth_gate=off` bypassing the correctness boundary.
- Distinguishing stale Claude provider auth from ordinary MCP discovery failure
  without trusting lane-authored health claims.
- Owner/mode and symlink behavior for the destination credential path.
- Multi-repo and same-user collapse behavior when the lane OS user is the
  operator user.
- Whether the proposed tests prove no raw credential material or private path
  material reaches repo artifacts, metrics, events, doctor output, dashboards,
  or GitHub comments.

## Current Source Anchors

Treat these anchors as design input and verify them before citing them as
current in the artifact:

- `go/pkg/mutations/supervision_launch.go` contains the `supervise.start`
  launch path that must refuse before process start.
- `go/pkg/mutations/supervision_provider_auth.go` runs the current provider-auth
  preflight gate before supervisor rows and provider launch.
- `go/pkg/laneproviderauth/lane_provider_auth.go`, `resolver.go`, and
  `expiry.go` contain the existing provider auth/resolver/expiry surfaces to
  reuse or deliberately extend.
- `go/pkg/sessionliveness/liveness.go` and recovery code classify
  `agent_mcp_discovery_stall`; RFC 0165 must prevent stale Claude provider auth
  from being treated as ordinary retryable lane flakiness.
- `go/pkg/reads/doctor_lane_provider_auth.go` is the read-side precedent for
  provider-auth operator visibility.
- `docs/reference/command-authority-matrix.md` records the authority boundary:
  `supervise.start` is the daemon-backed launch authority.

## Acceptance For This Design Run

The design clears only if the final proposal contains:

- A source-anchored implementation order suitable for a TDD build run.
- Named tests for happy-path hydration, stale lane generation refusal, source
  rotation during hydration, wrong owner/mode refusal, symlink/path escape,
  unparseable credential refusal, redaction, and recovery circuit breaker
  behavior.
- A durable state schema or event contract for redacted custody receipts.
- Explicit separation between provider OAuth credentials and Striatum
  control-plane credentials.
- A minimal closure scope for #583, with optional host timer/proximal pre-warm
  clearly deferred unless required for the synchronous launch invariant.

Do not close #583 from this run. The issue closes only after code lands on
`origin/main` and verifier evidence exists.
