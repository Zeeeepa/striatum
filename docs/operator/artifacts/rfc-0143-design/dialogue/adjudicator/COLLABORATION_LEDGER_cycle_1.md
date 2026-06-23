---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0143 lane credential survival across a daemon boot-epoch rotation — falsifiable implementation spec"
participants:
  - "holder-author-001"
  - "falsifier-reviewer-001"
  - "falsifier-reviewer-002"
  - "adjudicator-author-001"
cycle: 1
entries:
  - kind: claim
    by: "holder-author-001"
    refs: ["dialogue:1"]
    text: "Land Option 4 (legible-failure floor, zero trust-model change) first; recommend Option 2 (a new lane-owned 0600 CapabilityReseal token scoped to work.complete/artifact.publish/interrogation.answer for the in-flight job) as the ratification-gated survival mechanism; fold a minimal Option-3 per-session endpoint+epoch republish into Option 2 to close the F4 epoch gap without weakening #316. The spec claims this resolves all four Open Questions while holding the RFC 0096/#135/#296 trust model structurally (no widening, no replay, no split-brain)."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "The Option-4 floor cannot self-escalate in the exact no-credential case it exists to handle: it refuses the runtime client-token and returns ErrSessionUnrecoverableAcrossRotation, but the two named in-band routes require auth the failing lane lacks — session.report needs CapabilityClaim and work.block needs CapabilityWrite. With no reachable token the lane cannot publish either, so 'loud failure' is a local process error, not a routed Striatum state transition. No non-MCP durable route, no named daemon event, and no test that a terminal line became routed workflow state is specified."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "A striatum-lane-owned 0600 reseal file is a same-uid replay surface, not session-isolated. The holder's containment argument only covers an attacker acting as session B; a bearer thief acts as session A. Since all lanes share the striatum-lane uid, a sibling lane process can read session A's 0600 file and present A's bearer as A — sealing A's in-flight job with hostile artifact content, a false interrogation answer, or false-provenance completion. The session-binding predicate permits act-as-A; 0600 + session_id does not prove the presenter is the original lane. The holder concedes a per-lane uid would be needed to close the read."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "'Current in-flight job' is asserted as the split-brain guard but never defined as a concrete database predicate. The spec does not name the query (session live, same job still leased/acked by this session, lease active or explicitly resealable, no recovery-generation change, artifact path still in expected_artifacts, boot epoch accepted). Without it Slice B either cannot reseal after ordinary lease expiry or opens the R3 split-brain write where the old lane publishes/completes after the daemon requeued or retired the job."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "CapabilityReseal cannot reach the named methods under the current single-required-capability auth prelude. server.go authorizes RequiredCapability before any handler scoping, and the registry requires write for interrogation.answer / work.complete / artifact.publish; a token carrying only CapabilityReseal fails at the prelude with capability_missing. The spec says 'new capability + handler scoping' but scoping runs too late. It does not specify whether MethodEntry.RequiredCapability becomes a set / OR-capability, how PostgresAuthorizer projects the alternate grant, how AuthContext records the selected grant, or which generated contracts/authority-matrix/tests change. If instead the token is granted general write to pass the prelude, the OQ2 narrowness claim collapses and A1 (no-widening) is unbuildable as stated."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "The survival token races the active lease clock and cannot heartbeat. artifact.publish and work.complete both call activeLeaseFor and reject expired leases; the only verb that extends the lease is work.heartbeat, which requires claim — denied to CapabilityReseal. A restart longer than the lease window (or a reconnect wedge that keeps the lane alive but unable to heartbeat) leaves the token valid within its 24h TTL while the lease is expired, so reseal fails with an ordinary lease_error, NOT the typed session_unrecoverable_across_rotation OQ4 promises. The spec neither scopes survival to the lease window (vs token TTL) nor grants a scoped heartbeat, and no falsifiable assertion covers the gap."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "The Codex adapter cannot adopt a rotated endpoint/epoch in place, so Slice B overclaims survival for it. Current source bakes the MCP URL into launch-time Codex -c args, does not reload them while running, and applyMCPEndpointRotation can only log and inject a 'must be relaunched' prompt; the boot-epoch header is rendered from launch env, not pulled live from a mirror by the running Codex tool process. The proposed scratch mirror may help a component that can be taught to re-read it, but it does not let a live Codex lane's MCP channel publish/complete post-rotation. The spec does not name the adapter-specific survival matrix (Claude config rewrite vs Codex relaunch/rebridge vs a daemon-side receiver path)."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "The endpoint/epoch mirror needs an integrity story, not just readability, and A6's #316 claim is unproven. .striatum/scratch is lane-WRITABLE by default (scratchACLTargets grants u:<lane>:rwx plus a default ACL), and the #316 check returns nil when the boot-epoch header is ABSENT (only a non-empty mismatch is rejected). A stale lane could delete/replace the mirror and fall back to the permissive header-absent path, or carry untrusted epoch material into the reconnect. The spec names the trust property ('daemon-written, owner-trusted') but not the mechanism: no path/owner/mode/ACL, no symlink defense, no atomic-replace semantics, and no rule requiring an epoch header on the supervised reseal path."
    correspondence: landed_unrebutted
verdict: "needs_revision"
rationale: "All seven challenges are material (each would change the spec or exposes a real security/lifecycle defect) and all stand unrebutted by the spec as written. The most damning maps directly onto a named needs_revision trigger in the adjudication rubric: the Option-4 'loud failure' floor — the MANDATORY, lands-first half of the proposal — is not actually wired into the run's recovery, because its only named escalation routes (work.block / session.report) require an authenticated token the lane does not have in the precise no-credential failure the floor must handle. Two further material security gaps stand: a durable lane-readable 0600 bearer file is a same-uid replay surface (R2) the holder itself concedes is only closable with a per-lane uid, and 'current in-flight job' (the R3 split-brain guard) is asserted without a concrete database predicate. The buildability spine is also unresolved: CapabilityReseal cannot pass the current single-required-capability auth prelude without an unspecified OR-capability change (and if forced to general write, the no-widening narrowness claim collapses), the reseal path races the lease clock with no heartbeat authority so OQ4's typed escalation does not fire on the most common real failure, the Codex adapter cannot reload endpoint/epoch in place so Slice B overclaims survival, and the epoch mirror has a readability claim but no integrity mechanism over lane-writable scratch with a permissive header-absent #316 path. No path widens admin-token exposure and no minted credential carries {admin,apply,recovery,surgical_recovery}, so this is needs_revision, not reject; one revision cycle is available and the shape is salvageable. The spec's genuine strengths (rejecting the admin-token widening, the F1 reachability-not-reminting insight, the F4 epoch/token decoupling, the ratification gate, per-claim falsifying tests) are credited and must be preserved through the revision."
findings:
  - id: F1
    severity: high
    posture: design
    status: open
    challenge: "Option-4 floor cannot self-escalate with no reachable credential (falsifier-reviewer-001 C1). Fix: specify the NON-MCP route the no-token floor uses (typed agent-loop exit code, structured supervisor/PTY-bridge line, or an already-authenticated supervisor channel) and the exact DURABLE daemon event/blocker it produces (the session_unrecoverable_across_rotation recovery class as a recorded state transition the recovery sweep routes), and add GD-1 asserting that a no-STRIATUM_MCP_TOKEN_FILE restart yields a durable session_unrecoverable_across_rotation event/requeue — not a local process error."
  - id: F2
    severity: high
    posture: design
    status: open
    challenge: "Durable 0600 reseal bearer file is a same-uid replay surface, not session-isolated (falsifier-reviewer-001 C2; risk R2). Binding to session_id + 0600 does not stop a sibling process on the shared striatum-lane uid from reading and replaying A's bearer AS A. Fix: pick a mechanism that defeats same-uid replay — per-session OS users, a non-bearer reseal channel tied to the supervised PTY/session, daemon-side proof the caller is the original supervisor process, or an ACL that sibling lanes cannot read — OR scope the design to a per-lane-uid sandbox and state that dependency explicitly. 0600 + session-binding alone does not provide it. Add TestBorrowedResealBearerCannotSealVictimSession."
  - id: F3
    severity: high
    posture: design
    status: open
    challenge: "'Current in-flight job' is not a concrete split-brain predicate (falsifier-reviewer-001 C3; risk R3). Fix: name the exact database predicate (session live; same job still leased/acked by this session; lease active or explicitly resealable; no recovery-generation change; artifact path still in expected_artifacts; boot epoch accepted) and the named generation/lease check that drives a typed refusal. Add TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue."
  - id: F4
    severity: high
    posture: design
    status: open
    challenge: "CapabilityReseal cannot pass the single-required-capability auth prelude; the OR-capability mechanism is unspecified and may collapse to general write (falsifier-reviewer-002 C1; risk R1-adjacent). Fix: state exactly whether MethodEntry.RequiredCapability becomes a set / route-specific alternate, how PostgresAuthorizer projects the alternate grant, how AuthContext.Capability records the selected grant, and which generated contracts / command-authority-matrix / guardrail tests change — without granting plain CapabilityWrite. Add TestResealTokenCanReachOnlyResealRoutesWithoutWrite."
  - id: F5
    severity: high
    posture: design
    status: open
    challenge: "The reseal path races the active lease clock and has no heartbeat authority, so OQ4's typed escalation does not fire on the common lease-expiry case (falsifier-reviewer-002 C2). Fix: decide and state EITHER that Slice B only survives within the active lease window (not the 24h token TTL) AND that Option 4 explicitly routes expired-lease-after-rotation as session_unrecoverable_across_rotation, OR grant CapabilityReseal a scoped work.heartbeat for the current lease only. Add GD-1b (restart, prevent reconnect past leases.expires_at, then reseal): acceptable outcomes are a scoped heartbeat renewing only that lease before publish/complete, or the typed self-escalating class — never a raw lease_error or stale-lease limbo."
  - id: F6
    severity: medium
    posture: design
    status: open
    challenge: "The Codex adapter cannot adopt a rotated endpoint/epoch in place, so Slice B overclaims survival for it (falsifier-reviewer-002 C3). Fix: name the per-adapter survival matrix — Claude ephemeral-config rewrite, Agy behavior, Codex relaunch/rebridge (operator-assisted is acceptable if stated), or a daemon-side receiver/CLI-fallback path that can publish/complete without the Codex MCP client reloading launch args. Add GD-Codex-Reseal-Rotation. Do not claim in-place survival for the adapter family whose current source says it cannot reload in place."
  - id: F7
    severity: high
    posture: design
    status: open
    challenge: "The endpoint/epoch mirror has a readability claim but no integrity mechanism, and A6's #316 non-weakening is unproven (falsifier-reviewer-002 C4; risk R4-adjacent). Fix: specify the daemon-owned, lane-read-only mirror concretely (path, owner, mode/ACL that avoids the inherited lane-writable scratch default, symlink defense, atomic replace) AND decide whether a supervised reseal request with a MISSING boot-epoch header is rejected (close the permissive header-absent #316 path). Add TestResealEpochMirrorRejectsTamperOrMissingEpoch."
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0143 design run (cycle 1)

author: adjudicator-author-001

> Adjudication of the cycle-1 dialogue trajectory for the RFC 0143 design run
> (*lane credential survival across a daemon boot-epoch rotation*). Inputs read:
> the Holder spec (`dialogue/holder/HOLDER.md`), both falsifier challenges
> (`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`), the
> `SEED.md` charter + operator anchor table, and the canonical RFC
> (`docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md`). No
> raw terminal output was read. The falsifiers' source citations are credited
> because they agree with the SEED's operator-verified anchor table (env-only
> session token, single-cap auth prelude, lane-writable scratch ACL, permissive
> header-absent #316 path).

## Verdict

**verdict: needs_revision**

Seven challenges were raised across the two falsifiers. **Every one is material**
(each would change the spec or exposes a real security/lifecycle defect) and
**every one stands unrebutted by the spec as written**. This is a
security/authz-hot gate and the bar is held high: the proposal improves
decisively on the categorically unsafe admin-token widening (the RFC's option 1
literal), but it does not yet clear.

The single most decisive failure maps onto a named `needs_revision` trigger in
the adjudication rubric — *"an option-4 'loud failure' not actually wired into
the run's recovery."* Option 4 is the holder's **mandatory, lands-first** floor,
the conservative fix that is supposed to hold even when the reseal token is
absent. But its only named escalation routes (`work.block`, `session.report`)
require an authenticated capability the lane does **not** have in the exact
no-credential case the floor exists to handle (F1). A floor that cannot escalate
is not a floor.

Two further material **security** gaps stand: the durable lane-readable `0600`
bearer file is a same-uid replay surface the holder itself concedes is only
closable with a per-lane uid (F2, risk R2), and "current in-flight job" — the R3
split-brain guard — is asserted without a concrete database predicate (F3). The
**buildability spine** is also unresolved: `CapabilityReseal` cannot pass the
current single-required-capability auth prelude without an unspecified
OR-capability change, and if forced to general `write` the no-widening
narrowness claim collapses (F4); the reseal path races the lease clock with no
heartbeat authority, so OQ4's typed escalation does not fire on the most common
real failure (F5); the Codex adapter cannot reload endpoint/epoch in place, so
Slice B overclaims survival (F6); and the epoch mirror has a readability claim
but no integrity mechanism over lane-writable scratch (F7).

No path widens admin-token exposure and no minted credential carries any of
`{admin, apply, recovery, surgical_recovery}` — so this is **needs_revision, not
reject**. The shape is salvageable and both falsifiers explicitly say a safe
revision can keep it. One revision cycle is available: the Holder revises
`HOLDER.md`, then the falsifiers re-attack the revised spec.

## Challenge ledger

### F1 — `falsifier-reviewer-001` C1: the Option-4 floor cannot self-escalate when no credential is reachable

- **Claim challenged.** Option 4 can land first as a zero-trust-change floor:
  `ResolveTokenMaterial` / `ResolveTokenMaterialFresh` refuse the runtime
  `client-token`, return `ErrSessionUnrecoverableAcrossRotation`, and the
  agent-loop maps that sentinel to a `session_unrecoverable_across_rotation`
  signal via `work.block` and/or `session.report` (HOLDER OQ4).
- **Material?** **Yes — decisively.** It attacks the SEED's R4 (option 4 must
  make failure *loud and routed*, not swap one silent exit for another) and is a
  named `needs_revision` trigger ("an option-4 'loud failure' not actually wired
  into the run's recovery"). Option 4 is the *mandatory* floor and the holder's
  whole conservative-fix story rests on it.
- **Rebutted, or stands?** **Stands unrebutted.** In the exact failure the floor
  must handle the lane has no usable token: `ResolveTokenMaterialFresh`
  deliberately ignores the env literal and reads only `STRIATUM_MCP_TOKEN_FILE`
  then the runtime `client-token`; Slice A adds no token file and then refuses
  the `client-token`. At that point the two named routes are both authenticated —
  `session.report` requires `CapabilityClaim`, `work.block` requires
  `CapabilityWrite` — so neither can be published. The spec names an
  *authenticated in-band route for a no-auth condition*. It gestures at the codex
  wedge-prompt pattern but does not name a concrete non-MCP durable route, the
  exact daemon event created, or a test proving a terminal line became routed
  workflow state. The falsifier even supplies the rebuttal-on-the-holder's-behalf
  (typed exit code / supervisor line / PTY bridge) and correctly notes the spec
  does not commit to it.
- **Disposition.** Material defect, unrebutted → drives `needs_revision`.

### F2 — `falsifier-reviewer-001` C2: the `0600` reseal token is a same-uid replay surface, not session-isolated (risk R2)

- **Claim challenged.** The same-uid read surface is *contained* because the
  token is bound to `session_id`: "presenting session A's reseal token while
  acting as session B is refused" (HOLDER OQ2 cross-lane note).
- **Material?** **Yes.** R2 (durable-file replay) is a load-bearing SEED risk and
  the precise reason today's session token is env-only/in-memory. The challenge
  goes to whether a *new* durable credential file is a leak surface.
- **Rebutted, or stands?** **Stands unrebutted** (and is partly conceded). The
  containment argument only covers an attacker acting as session *B*. A bearer
  thief acts as session *A*: the session-binding predicate *permits* the act-as-A
  request, and `0600` does not stop a sibling process sharing the
  `striatum-lane` uid from reading A's file and replaying A's bearer. Sealing A's
  in-flight job is still a security mutation — hostile artifact content, a false
  interrogation answer, false-provenance completion. The holder's own note
  concedes the read is only closed by a per-lane uid ("if the sandbox later
  adopts one, closes even the read"). The mitigations offered (unguessable path,
  removed on close, scoped caps) shrink the window but do not remove the
  same-uid replay vector. Credit: the holder *did* anticipate the same-uid issue
  and bound blast radius to the in-flight job; that is real and should be kept —
  but it is not isolation.
- **Disposition.** Material security defect, unrebutted → drives `needs_revision`.

### F3 — `falsifier-reviewer-001` C3: "current in-flight job" is not a concrete split-brain predicate (risk R3)

- **Claim challenged.** The reseal authorizes the three verbs only for "the
  session's current in-flight job," and session-recognition + the #316 epoch
  check prevent split-brain (HOLDER OQ2/A4).
- **Material?** **Yes.** R3 (split-brain across rotation) is load-bearing, and
  the SEED requires Open Questions resolved *with a concrete mechanism* (named
  code site, capability test, invalidation trigger). An asserted guard with no
  predicate is exactly the "resolved without a concrete mechanism" pattern.
- **Rebutted, or stands?** **Stands unrebutted.** The spec never defines the
  database predicate that makes "current in-flight job" true at reseal time.
  Existing lifecycle is lease-shaped (`activeLeaseFor` rejects non-active,
  wrong-owner, wrong-job, expired). If reseal preserves lease enforcement, a lane
  whose lease expired during the restart cannot reseal though the token is valid;
  if it bypasses lease authority from the bearer alone, the old lane can
  publish/complete after the daemon requeued or retired the job — the R3 write the
  SEED names. The assertion A4 exists, but the *mechanism* (a strict
  job/session/lease-generation query) does not.
- **Disposition.** Material defect, unrebutted → drives `needs_revision`.

### F4 — `falsifier-reviewer-002` C1: `CapabilityReseal` cannot reach the named methods under the current auth prelude (risk R1-adjacent)

- **Claim challenged.** The new credential carries only `CapabilityReseal` (not
  general `claim`/`write`) yet authorizes `work.complete`, `artifact.publish`,
  `interrogation.answer` for the in-flight job — "structurally narrower" than the
  session token (HOLDER OQ2/OQ3, A1).
- **Material?** **Yes.** It is load-bearing for both buildability and the
  no-widening invariant (A1/R1). If unresolved, A1 is unbuildable; if forced to
  general `write`, the narrowness claim collapses.
- **Rebutted, or stands?** **Stands unrebutted.** The RPC server authorizes a
  single `RequiredCapability` *before* any handler scoping, and the registry
  requires `write` for those three methods; a `CapabilityReseal`-only token fails
  the prelude with `capability_missing` and never reaches the scoping. "New
  capability + handler scoping" does not say whether
  `MethodEntry.RequiredCapability` becomes a set / route-specific alternate, how
  `PostgresAuthorizer` projects the alternate grant, how `AuthContext.Capability`
  records the selected grant, or which generated contracts / authority matrix /
  guardrail tests change. This is a load-bearing authority-model change left
  unspecified.
- **Disposition.** Material defect, unrebutted → drives `needs_revision`.

### F5 — `falsifier-reviewer-002` C2: the survival token races the lease clock and cannot heartbeat

- **Claim challenged.** Bounding the token by `min(session TTL, reseal window)`
  and authorizing publish/complete for the in-flight job is sufficient to reseal
  after rotation (HOLDER OQ2).
- **Material?** **Yes.** It undermines both the survival claim (token TTL is the
  wrong clock) and OQ4 (the typed escalation does not fire where it matters most).
- **Rebutted, or stands?** **Stands unrebutted.** `artifact.publish` and
  `work.complete` both require an active, unexpired lease (`activeLeaseFor`); the
  only verb that extends it is `work.heartbeat`, which requires `claim` — denied
  to `CapabilityReseal`. A restart longer than the lease window leaves the token
  valid within its 24h TTL but the lease expired, so the terminal error is the
  ordinary `lease_error`, not OQ4's `session_unrecoverable_across_rotation`. The
  spec neither scopes survival to the lease window nor grants a scoped heartbeat,
  and no falsifiable assertion covers it.
- **Disposition.** Material defect, unrebutted → drives `needs_revision`.

### F6 — `falsifier-reviewer-002` C3: the Codex adapter cannot adopt a rotated endpoint/epoch in place

- **Claim challenged.** #323 recovery reads endpoint+epoch+reseal-token from the
  lane scratch mirror and reconnects; the minimal Option-3 republish closes the
  epoch gap (HOLDER OQ1/OQ3).
- **Material?** **Yes** (buildability/scope). It bears on whether Slice B is
  buildable for the adapter family the run itself uses, and whether the spec
  overclaims survival.
- **Rebutted, or stands?** **Stands unrebutted.** Current source bakes the MCP
  URL into launch-time Codex `-c` args, does not reload them while running, and
  `applyMCPEndpointRotation` can only log + inject a "must be relaunched" prompt;
  the boot-epoch header is launch-arg/launch-env rendered, not live-pulled from a
  mirror by the running Codex tool process. The mirror may help a component that
  can be taught to re-read it, but it does not let a live Codex MCP channel
  publish/complete post-rotation. The spec does not name the adapter-specific
  survival matrix. (Operator-assisted relaunch for Codex is an acceptable
  resolution if the spec *states* it, rather than claiming in-place survival.)
- **Disposition.** Material defect, unrebutted → drives `needs_revision`.

### F7 — `falsifier-reviewer-002` C4: the endpoint/epoch mirror needs an integrity story, not just readability (risk R4-adjacent)

- **Claim challenged.** A6: epoch republish does not weaken #316 because the lane
  adopts a fresh epoch only from a daemon-written, owner-trusted per-session
  scratch mirror.
- **Material?** **Yes.** #316 is an anti-confusion security guard; A6 is a named
  load-bearing assertion. A property asserted without a mechanism, over a surface
  whose current defaults contradict it, does not hold "structurally."
- **Rebutted, or stands?** **Stands unrebutted.** `.striatum/scratch` is
  lane-*writable* by default (`scratchACLTargets` grants `u:<lane>:rwx` plus a
  default ACL), and the #316 check returns nil when the boot-epoch header is
  *absent* (only a non-empty mismatch is rejected). So a stale lane could delete
  or replace the mirror and fall back to the permissive header-absent path, or
  carry untrusted epoch material into the reconnect. The spec names the trust
  property but not the mechanism: no path/owner/mode/ACL that escapes the
  inherited write default, no symlink defense, no atomic-replace semantics, and
  no rule requiring an epoch header on the supervised reseal path. "Non-secret"
  does not mean "integrity-free."
- **Disposition.** Material defect, unrebutted → drives `needs_revision`.

## Credited strengths (preserve these through the revision)

The revision must **build on** these, not regress them:

- **The categorical R1 widening is correctly refused.** The spec rejects
  group-reading the runtime `client-token` and structures `CapabilityReseal` to
  carry *none* of `{admin, apply, recovery, surgical_recovery}`. This is the
  hottest blast-radius dimension and the holder held it. (F4 is about *how* the
  narrow capability is wired, not a widening.)
- **The F1 reframe is a genuine, correct insight the RFC understated:** the
  PG-resident session token stays *valid* across a restart; only its
  *reachability* breaks — so the fix is about reachability, not re-minting
  authority. A8 / `TestTokenValidAcrossRestart` is a real falsifying test.
- **The F4 epoch/token decoupling is correct and important:** token reachability
  and epoch reachability are two distinct gaps, and #316 makes the epoch gap a
  deliberate retirement that must not be silently weakened. (The integrity gap in
  F7 is the unfinished half of this same insight, not a refutation of it.)
- **Option 3-as-primary is correctly rejected** (you cannot mutate a running
  process's env), and the reasoning that it degenerates into Option 2's file is
  sound.
- **The maintainer-ratification gate is correctly flagged** and the per-claim
  falsifiable-assertion discipline (A1–A8 with named tests/game-days) is the
  right shape — the revision should extend it to cover F1–F7, not abandon it.

## What the revision MUST fix to clear on re-attack

Close the security gaps (F1–F4, F7) and resolve the lifecycle/buildability gaps
(F5, F6), or honestly narrow the claims. Concretely:

1. **F1 — make the Option-4 floor actually routed.** Name the NON-MCP escalation
   route for the no-token case (typed agent-loop exit code, structured
   supervisor/PTY-bridge line, or an already-authenticated supervisor channel),
   the exact DURABLE daemon event/blocker it produces, and the test (GD-1) that a
   no-`STRIATUM_MCP_TOKEN_FILE` restart yields a recorded
   `session_unrecoverable_across_rotation` event/requeue rather than a local
   process error.
2. **F2 — defeat same-uid replay, or state the dependency.** Choose per-session
   OS users, a non-bearer reseal channel tied to the supervised PTY/session,
   daemon-side proof the caller is the original supervisor process, or an ACL
   sibling lanes cannot read — OR scope the design to a per-lane-uid sandbox and
   state that dependency. `0600` + session-binding alone is insufficient.
3. **F3 — name the split-brain predicate.** A strict reseal-time query (session
   live; same job still leased/acked by this session; lease active or explicitly
   resealable; no recovery-generation change; artifact path still in
   `expected_artifacts`; boot epoch accepted) with a named generation/lease check
   driving a typed refusal.
4. **F4 — specify the authority mechanism.** Whether
   `MethodEntry.RequiredCapability` becomes a set / route-specific alternate, how
   `PostgresAuthorizer` projects the alternate grant, how `AuthContext.Capability`
   records the selected grant, and which generated contracts /
   command-authority-matrix / guardrail tests change — without granting plain
   `CapabilityWrite`.
5. **F5 — resolve the lease clock.** Either state that Slice B only survives
   inside the active lease window (not the token TTL) AND have Option 4 route
   expired-lease-after-rotation as the typed class, OR grant a scoped
   `work.heartbeat` for the current lease only. Cover it with GD-1b.
6. **F6 — name the per-adapter survival matrix.** Claude config rewrite, Agy
   behavior, Codex relaunch/rebridge (operator-assisted is fine if stated), or a
   daemon-side receiver/CLI-fallback path; do not claim in-place Codex survival.
   Cover it with GD-Codex-Reseal-Rotation.
7. **F7 — give the mirror an integrity mechanism.** Daemon-owned, lane-read-only
   mirror (path, owner, mode/ACL escaping the lane-writable scratch default,
   symlink defense, atomic replace) AND a decision to reject a supervised reseal
   request with a MISSING boot-epoch header (close the permissive header-absent
   #316 path).

## Note on maintainer ratification (carries forward regardless of verdict)

Even when this spec clears on re-attack, the chosen direction (a new
`rpc.CapabilityReseal` class, a new durable lane-readable credential file, and
new rotation/epoch plumbing) is a **security/authz trust-model change** that
requires **maintainer ratification** before any build slice touches credential
code. The holder correctly states this gate; it is recorded here so it is not
lost. Adjudicator clearance gates the *spec's soundness*; it is **not** the
maintainer's product call on the trust-model change. Slice A (the Option-4 floor)
is zero-trust-change and may land first under the normal review gate **once F1
makes it actually routed** — but Slice B does not land until the maintainer
ratifies the new credential class.

---
<sub>Adjudicator collaboration ledger for the RFC 0143 falsification-gate design
run, cycle 1. The ledger verdict — not falsifier completion — gates the phase:
`needs_revision` returns the spec to the Holder for one revision cycle, after
which the falsifiers re-attack.</sub>
