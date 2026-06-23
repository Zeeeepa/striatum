# FALSIFIER — RFC 0143 security/authz challenge

author: falsifier-reviewer-001

## Summary

The holder proposal improves on the categorically unsafe admin-token widening, but it does not yet clear the security/authz gate. The remaining gaps are not wording gaps: Slice A's "loud failure" still depends on authenticated MCP after the credential path has failed, and Slice B's durable `0600` lane-owned bearer file is replayable by any process running as the shared lane OS user unless the design adds a second binding beyond `session_id`.

## Challenge 1 — The Option 4 floor cannot self-escalate when no credential is reachable

**Claim attacked.** The holder says Option 4 can land first as a zero-trust-change floor: `ResolveTokenMaterial` / `ResolveTokenMaterialFresh` refuse the runtime `client-token`, return `ErrSessionUnrecoverableAcrossRotation`, and the agent-loop maps that sentinel to `session_unrecoverable_across_rotation` via `work.block` and/or `session.report`.

**Concrete refutation.** In the exact failure Slice A is supposed to handle, the lane has no usable token. `ResolveTokenMaterialFresh` intentionally ignores the env literal and reads only `STRIATUM_MCP_TOKEN_FILE` and then the runtime `client-token` (`go/pkg/agentloop/endpoint.go:117-137`). Slice A does not add a token file, and Option 4 then refuses the runtime `client-token`. At that point the lane cannot call the two named escalation surfaces: `session.report` requires `CapabilityClaim`, while `work.block` requires `CapabilityWrite` (`go/pkg/rpc/registry_rfc0043_test.go:7-15`). The proposal therefore names an authenticated in-band route for a no-auth condition.

**Test that should fail the current spec.** `GD-1` should restart `striatumd` mid-job with no `STRIATUM_MCP_TOKEN_FILE`, then assert that the daemon records a durable `session_unrecoverable_across_rotation` event or blocker. Under the holder's current mechanism, the agent-loop can detect the sentinel locally, but it has no credential with which to publish the `work.block` / `session.report`.

**Strongest rebuttal.** The agent-loop could exit with a typed code, write a structured supervisor line, or use an already-attached PTY bridge so the daemon classifies the terminal event without a new MCP call.

**Gap that remains.** The spec does not name that non-MCP route, the exact durable daemon event it creates, or the test that proves a terminal line became routed workflow state. Without that mechanism, "loud failure" is still a local process error, not a Striatum state transition.

## Challenge 2 — A `striatum-lane`-owned `0600` reseal token is not session-isolated on a shared lane uid

**Claim attacked.** The holder's OQ2 cross-lane note says the same-uid read surface is contained because the token is bound to `session_id`: "presenting session A's reseal token while acting as session B is refused."

**Concrete refutation.** That is true only for an attacker who tries to act as session B. A bearer-token thief acts as session A. The current canonical predicate allows a bound token to act as its own bound session (`go/pkg/rpc/principal_session.go:21-26`), and the existing token comments describe the same enforcement: handlers refuse "any act-as session other than this one" (`go/pkg/mutations/session_token.go:43-45`). Binding the token to session A does not prove the process presenting it is the original lane for session A.

The OS isolation described by the holder is also not enough. The lane sandbox launches supervised lanes as a dedicated lane OS user through `sudo -n -u <lane-user> -- env -i ...` (`docs/how-to/lane-sandbox.md:31-37`), and the RFC seed identifies that common user as `striatum-lane`. A file owned by `striatum-lane` with mode `0600` is readable by any process with that uid. The current env-only `STRIATUM_MCP_TOKEN` avoids creating a durable same-uid read target; the proposed file reintroduces one.

**Test that should fail the current spec.** `TestBorrowedResealBearerCannotSealVictimSession`: start sessions A and B under the same lane OS user; have B obtain A's reseal file path or bearer; call `artifact.publish` / `work.complete` using A's `session_id`, A's job id, and A's bearer. The session-binding predicate should allow the "act as A" request unless the implementation adds a second factor not present in the holder proposal.

**Strongest rebuttal.** The daemon can keep the path unguessable, remove the file on session close, and rely on scoped `CapabilityReseal` so a borrowed token can only seal the victim's current job rather than claim new work or use elevated caps.

**Gap that remains.** Sealing the victim's current job is still a security mutation: it can publish hostile artifact content, answer an interrogation, or complete work with false provenance. If same-uid lanes are in scope, the spec needs one of: per-session OS users; a non-bearer reseal channel tied to the supervised PTY/session; daemon-side proof that the caller is the original supervisor process; or a file ownership/ACL mechanism that is not readable by sibling lanes with the same uid. `0600` alone does not provide that.

## Challenge 3 — "Current in-flight job" is not a concrete split-brain predicate

**Claim attacked.** The reseal token authorizes only `work.complete`, `artifact.publish`, and `interrogation.answer` for "the session's current in-flight job," with TTL up to 24h, and the holder claims session-recognition plus the epoch check prevents split-brain.

**Concrete refutation.** The spec does not define the database predicate that makes "current in-flight job" true at reseal time. Existing lane lifecycle calls are lease-shaped; this packet's compatibility commands require a `lease_id`, and `work.block` normalization requires `session_id`, `job_id`, and `lease_id` (`go/pkg/mutations/lifecycle.go:1597-1624`). If the reseal path preserves normal lease enforcement, a lane whose lease expired during the restart cannot reseal even though the reseal token remains valid for the proposed 24h window. If the reseal path bypasses or reissues lease authority from the bearer alone, it can race operator recovery: the old lane may publish or complete after the daemon has requeued, retired, or replaced the job.

**Test that should fail the current spec.** `TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue`: mint the reseal token, let the active lease expire, run the supported operator recovery path, then present the old reseal bearer against the old job. The expected result must be a typed refusal, not a publish/complete, and the refusal must be driven by a named job/session/lease generation check.

**Strongest rebuttal.** "Current in-flight job" can be implemented as a strict query: session still live, same job still leased/acked by this session, lease still active or explicitly resealable, no recovery generation change, artifact path still in `expected_artifacts`, and boot epoch accepted.

**Gap that remains.** That query is the security mechanism, but the holder proposal does not name it. Without it, Slice B either fails to reseal after ordinary lease expiry or creates the split-brain write path the seed calls out as R3.

## Bottom line

The proposal should not clear as-is. A safe revision can keep the broad shape, but it needs two concrete additions before build handoff: a durable non-MCP reporting route for the no-token Option 4 floor, and a reseal design that treats the durable bearer file as a same-uid replay surface rather than as session-isolated merely because it is `0600` and session-bound.
