# Design-Run Seed (design-v2 REVISION) — RFC 0143 Lane credential survival across a daemon boot-epoch rotation

> **This is the design-v2 REVISION run.** A design-v1 falsification gate already
> ran on this RFC. Its adjudicator returned **`needs_revision`** with **seven
> findings (F1–F7, mostly high severity)** — the spec did not clear. This run is
> the *proper revision*: the Holder REVISES the v1 spec to resolve every finding,
> and the falsifiers re-attack the revised spec. The design-v1 artifacts are
> **required context docs** for this run:
> `docs/operator/artifacts/rfc-0143-design/dialogue/holder/HOLDER.md` (the v1
> spec to revise) and
> `docs/operator/artifacts/rfc-0143-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
> (the verdict + the seven findings with prescribed fixes). The
> **`## Binding revision constraints`** section below distills those seven
> findings — every one is **binding** and must be resolved.
>
> This document is the **required input** for the RFC 0143 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed
> at `docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md`
> (status `proposed`) — read it in full as your primary source; this SEED carries
> the charter, restates the four Open Questions, pins an operator
> anchor-verification table you must build on, and lists the binding revision
> constraints. Read this whole file, the RFC, and the two design-v1 context docs
> before producing any artifact.

## Charter — what this run must produce

This is a **design run**, not an implementation run. The deliverable is a
**falsifiable implementation spec** for RFC 0143: a concrete, buildable
specification that the `rfc-0143-build` run can execute contract-first (TDD),
produced by hardening the RFC against adversarial falsification.

This is a **security/authz-hot** decision (the RFC's `security_or_authz` blast
dimension). The committed `PROPOSAL.md` MUST:

1. **Resolve all four Open Questions** (below) with a concrete, defensible
   decision each — *which option / which mechanism / why*. A design run that
   leaves an Open Question unresolved has not cleared the gate.
2. **Hold the security invariant as the spine:** preserve the RFC 0096/#135/#296
   session-bound trust model. No lane ever reads the daemon's full-authority
   bootstrap admin client-token; no new lane-readable credential carries any of
   `{admin, apply, recovery, surgical_recovery}`.
3. **Name the exact surfaces** (files + functions + the new credential's
   ownership/mode/TTL) from the anchor table below.
4. **State every load-bearing claim as a falsifiable assertion** paired with the
   named test / game-day step that would prove it false.
5. **Flag maintainer ratification:** the cleared spec is a RECOMMENDATION the
   maintainer ratifies before any build slice touches credential code.
6. **Resolve every binding revision constraint (F1–F7)** in the section below.
   This is a revision run; a spec that leaves any v1 finding unresolved has not
   cleared the gate.

## Binding revision constraints (design-v1 gate findings — MUST resolve)

> The design-v1 falsification gate returned **`needs_revision`**. **All seven
> challenges were ruled material and all stood unrebutted.** Every constraint
> below is **BINDING**: the revised `HOLDER.md` (and the committed `PROPOSAL.md`)
> MUST resolve each finding with its **prescribed fix** — a concrete mechanism, a
> named code site, and the named falsifying test / game-day — exactly as the
> cycle-1 collaboration ledger directed. The full reasoning lives in the context
> doc
> `docs/operator/artifacts/rfc-0143-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`;
> the fixes are copied verbatim from that ledger's `findings:` block. **Resolving
> all seven, while holding the security invariant (no admin-token widening, no
> replay, no split-brain), is a precondition for a clearing verdict.** Do **not**
> regress the v1 strengths the ledger credited (the categorical R1 widening
> refusal; the F1 reachability-not-reminting insight; the F4 epoch/token
> decoupling; rejecting Option-3-as-primary; the ratification gate; the per-claim
> falsifiable-assertion discipline) — *build on* them.

**This is a REVISION.** The Holder starts from the design-v1 `HOLDER.md` (a
required context doc), resolves every finding below, and must hold the security
invariant throughout. The falsifiers FIRST verify each finding is genuinely
resolved, then hunt for any NEW material gap.

- **F1 — high — Option-4 floor cannot self-escalate with no reachable
  credential (falsifier-reviewer-001 C1).** The Option-4 "loud failure" floor —
  the mandatory, lands-first half of the proposal — refuses the runtime
  client-token and returns `ErrSessionUnrecoverableAcrossRotation`, but its only
  named escalation routes require auth the failing lane lacks: `session.report`
  needs `CapabilityClaim` and `work.block` needs `CapabilityWrite`. With no
  reachable token the lane can publish neither, so "loud failure" is a local
  process error, not a routed Striatum state transition.
  **Prescribed fix:** specify the NON-MCP route the no-token floor uses (typed
  agent-loop exit code, structured supervisor/PTY-bridge line, or an
  already-authenticated supervisor channel) and the exact DURABLE daemon
  event/blocker it produces (the `session_unrecoverable_across_rotation` recovery
  class as a recorded state transition the recovery sweep routes), and add **GD-1**
  asserting that a no-`STRIATUM_MCP_TOKEN_FILE` restart yields a durable
  `session_unrecoverable_across_rotation` event/requeue — not a local process
  error.

- **F2 — high — Durable `0600` reseal bearer file is a same-uid replay surface,
  not session-isolated (falsifier-reviewer-001 C2; risk R2).** Binding to
  `session_id` + `0600` does not stop a sibling process on the shared
  `striatum-lane` uid from reading and replaying A's bearer AS A (the
  session-binding predicate *permits* act-as-A; the holder itself concedes only a
  per-lane uid closes the read).
  **Prescribed fix:** pick a mechanism that defeats same-uid replay — per-session
  OS users, a non-bearer reseal channel tied to the supervised PTY/session,
  daemon-side proof the caller is the original supervisor process, or an ACL that
  sibling lanes cannot read — OR scope the design to a per-lane-uid sandbox and
  state that dependency explicitly. `0600` + session-binding alone does not
  provide it. Add **TestBorrowedResealBearerCannotSealVictimSession**.

- **F3 — high — "Current in-flight job" is not a concrete split-brain predicate
  (falsifier-reviewer-001 C3; risk R3).**
  **Prescribed fix:** name the exact database predicate (session live; same job
  still leased/acked by this session; lease active or explicitly resealable; no
  recovery-generation change; artifact path still in `expected_artifacts`; boot
  epoch accepted) and the named generation/lease check that drives a typed
  refusal. Add **TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue**.

- **F4 — high — `CapabilityReseal` cannot pass the single-required-capability
  auth prelude; the OR-capability mechanism is unspecified and may collapse to
  general `write` (falsifier-reviewer-002 C1; risk R1-adjacent).**
  **Prescribed fix:** state exactly whether `MethodEntry.RequiredCapability`
  becomes a set / route-specific alternate, how `PostgresAuthorizer` projects the
  alternate grant, how `AuthContext.Capability` records the selected grant, and
  which generated contracts / `command-authority-matrix` / guardrail tests change
  — **without granting plain `CapabilityWrite`**. Add
  **TestResealTokenCanReachOnlyResealRoutesWithoutWrite**.

- **F5 — high — The reseal path races the active lease clock and has no heartbeat
  authority, so OQ4's typed escalation does not fire on the common lease-expiry
  case (falsifier-reviewer-002 C2).** `artifact.publish` / `work.complete` both
  reject expired leases via `activeLeaseFor`; only `work.heartbeat` (which needs
  `claim`, denied to `CapabilityReseal`) extends the lease — so a restart longer
  than the lease window yields an ordinary `lease_error`, not the typed
  `session_unrecoverable_across_rotation`.
  **Prescribed fix:** decide and state EITHER that Slice B only survives within
  the **active lease window** (not the 24h token TTL) AND that Option 4 explicitly
  routes expired-lease-after-rotation as `session_unrecoverable_across_rotation`,
  OR grant `CapabilityReseal` a **scoped `work.heartbeat` for the current lease
  only**. Add **GD-1b** (restart, prevent reconnect past `leases.expires_at`,
  then reseal): acceptable outcomes are a scoped heartbeat renewing only that
  lease before publish/complete, or the typed self-escalating class — never a raw
  `lease_error` or stale-lease limbo.

- **F6 — medium — The Codex adapter cannot adopt a rotated endpoint/epoch in
  place, so Slice B overclaims survival for it (falsifier-reviewer-002 C3).**
  Current source bakes the MCP URL into launch-time Codex `-c` args and cannot
  reload them while running; `applyMCPEndpointRotation` can only log + inject a
  "must be relaunched" prompt.
  **Prescribed fix:** name the per-adapter survival matrix — Claude
  ephemeral-config rewrite, Agy behavior, Codex relaunch/rebridge
  (operator-assisted is acceptable if stated), or a daemon-side
  receiver/CLI-fallback path that can publish/complete without the Codex MCP
  client reloading launch args. Add **GD-Codex-Reseal-Rotation**. Do **not** claim
  in-place survival for the adapter family whose current source says it cannot
  reload in place.

- **F7 — high — The endpoint/epoch mirror has a readability claim but no
  integrity mechanism, and A6's #316 non-weakening is unproven
  (falsifier-reviewer-002 C4; risk R4-adjacent).** `.striatum/scratch` is
  lane-WRITABLE by default (`scratchACLTargets` grants `u:<lane>:rwx` plus a
  default ACL), and the #316 check returns `nil` when the boot-epoch header is
  ABSENT (only a non-empty mismatch is rejected) — so a stale lane could
  delete/replace the mirror and fall back to the permissive header-absent path.
  **Prescribed fix:** specify the daemon-owned, lane-read-only mirror concretely
  (path, owner, mode/ACL that avoids the inherited lane-writable scratch default,
  symlink defense, atomic replace) AND decide whether a supervised reseal request
  with a MISSING boot-epoch header is rejected (close the permissive
  header-absent #316 path). Add **TestResealEpochMirrorRejectsTamperOrMissingEpoch**.

**Clearing condition for this revision.** The adjudicator clears the gate only if
**all seven findings above are resolved** with a concrete mechanism, **no new
material challenge** stands unrebutted, and the **security invariant holds**
structurally (no admin-token widening, no replay, no split-brain). One revision
cycle is available within this run; the falsifiers re-attack the revised spec.

## Root reframe (do not lose this)

**A boot-epoch rotation must never force a lane to choose between reading the
daemon's full-authority bootstrap admin token and exiting silently unsealed.**
Today, when a lane loses its live RPC connection across a daemon restart, its
credential-resolution chain falls through to the admin `client-token` (which a
`striatum-lane` lane cannot read), so a complete-on-disk deliverable exits
unsealed with a misleading permission error. Either give the lane a *narrow,
session-scoped, lane-readable* credential that survives the rotation, or make the
failure *loud and routed* — never silent, never via the admin token.

## The four options (from the RFC §"The unresolved decision")

1. **Status quo** — orphan the in-flight output as complete-but-unsealed; operator
   requeue (`supervise stop` → `session close` → `recovery auto`) is the supported
   recovery. Accept the interruption.
2. **Durable lane-owned reseal token** — mint a `striatum-lane`-owned `0600` token
   carrying only the caps needed to seal the in-flight job, invalidated on session
   close, bounded by the session TTL. Preserves the trust model; adds a new
   credential file + lifecycle. *(Handoff-recommended trust-preserving direction.)*
3. **Re-mint + re-inject on rotation** — the boot-epoch rotation handshake re-mints
   and re-injects the session-bound token into the live lane env/token file, so the
   chain stops at step 1/2 and the lane never needs the admin token.
4. **Narrow the fallback (legible failure)** — make the resolution chain refuse to
   reach the admin `client-token` for a non-owner lane and surface a typed,
   self-escalating "session unrecoverable across rotation" signal so the run routes
   it. Does not reseal, but removes the silent/misleading dead-end. Combinable
   with 1.

## Open Questions to resolve (design calls)

1. **OQ1 — Which option (the trust-model shape).** Pick the primary survival
   mechanism (1/2/3) AND whether to also land option 4 as the immediate safety
   net. This is THE security/authz call.
2. **OQ2 — Surviving authority + lifecycle.** If a new credential class: which
   exact caps survive a rotation (just `write` to seal? the lane's normal
   `{claim,write,read,review}`?), TTL/window, file ownership/mode, and the exact
   invalidation triggers (session close / TTL / new boot epoch).
3. **OQ3 — Where the mechanism lives.** Name the exact code site(s) for the mint,
   the injection, the rotation handshake (option 3), and the resolution-chain
   refusal (option 4).
4. **OQ4 — Legible-failure fallback.** Define the self-escalating failure when the
   lane genuinely cannot reseal, and how the run's recovery routes it.

## Load-bearing risks (attack these)

- **R1 trust-model widening:** any option that exposes the admin token, or mints a
  credential carrying elevated caps, is categorically out of bounds. Test: present
  the new credential for `admin`/`apply`/`recovery` → REFUSED.
- **R2 durable-file replay:** a lane-readable token file is a leak surface (the
  reason today's token is env-only/in-memory). Must be invalidated on session
  close, TTL-bound, epoch-bound, and not readable by a different lane user.
- **R3 split-brain across rotation:** a reseal must only seal the in-flight job the
  daemon still recognizes; it cannot write into a session the daemon retired.
- **R4 silent-failure regression:** option 4 must make the failure LOUD
  (self-escalating recovery class wired into routing), not swap one silent exit for
  another.

## Anchor verification against current `main` (operator pre-flight, 2026-06-22)

Verified against `~/git/striatum` @ `main` (all claims **ACCURATE**, no drift).
Treat as ground truth; re-anchor the spec to these file:line references.

| RFC claim / area | Status | Anchor (current source) |
| --- | --- | --- |
| Session-bound token grants `{claim,write,read,review}`, 24h TTL, bound to `session_id` | **ACCURATE** | `go/pkg/mutations/session_token.go:21` `sessionBoundTokenTTL = 24*time.Hour`; `:46` `sessionBoundCapabilities = {Claim,Write,Read,Review}`; grants bound to `sessionID` at `:77-89`. |
| Injected as `STRIATUM_MCP_TOKEN` into lane env at supervise start | **ACCURATE** | `go/pkg/mutations/supervision_env.go:342` `entries = append(entries, "STRIATUM_MCP_TOKEN="+boundToken)`; minted in `HandleSuperviseStart` tx (`supervision_control.go:158`). |
| Token does NOT rotate on a normal restart (only the endpoint does) | **ACCURATE (load-bearing)** | `go/pkg/agentloop/endpoint.go:111-112`: "The token does NOT rotate on a normal daemon restart (only the endpoint does)…". |
| 4-step credential-resolution order (env literal → env file → runtime client-token → repo `.striatum/capability_token`) | **ACCURATE (load-bearing)** | `go/pkg/agentloop/token.go:18-20` (1, `STRIATUM_MCP_TOKEN`), `:23-28` (2, `STRIATUM_MCP_TOKEN_FILE`), `:31-41` (3, runtime `client-token` via `admin.RuntimeTokenPath()` / `STRIATUM_DAEMON_RUNTIME_DIR`), `:44-51` (4, repo token). The bug fires at step 3. |
| Runtime `client-token` is the full-authority bootstrap admin token, `0600` owner-only | **ACCURATE (load-bearing)** | `go/pkg/admin/bootstrap.go:18-27` `bootstrapCapabilities = {admin,read,write,claim,review,apply,recovery,surgical_recovery}`; `writeRuntimeToken` `:139-142` mkdir `0700`, `:153/:164` chmod `0600`. |
| `ReadTokenFile` refuses any token file not owner-only | **ACCURATE (load-bearing)** | `go/pkg/agentloop/token.go:81-84` `if mode&0077 != 0 { return "", errors.New("daemon token file is not owner-only") }`. |
| Daemon publishes a boot-epoch file + rotates the mcp-http-endpoint on a boot-epoch change | **ACCURATE (load-bearing)** | `go/cmd/striatumd/main.go:700` `bootEpochFileName = "mcp-boot-epoch"`, `:748-749` writes atomically; `:465-486` computes `daemonBootEpoch()` and injects `BootEpoch` into lane env (`supervision_env.go:352-354`). |
| #323 endpoint-rotation recovery re-reads token via `ResolveTokenMaterialFresh` and reconnects | **ACCURATE (load-bearing)** | `go/pkg/agentloop/loop.go:600-605` re-reads token on rotation; `:619-658` `applyMCPEndpointRotation` rewrites ephemeral config + reconnects ("#323 rotation recovery"). |
| A durable, lane-readable session-token file does NOT exist today | **NOT-FOUND (ACCURATE)** | No `striatum-lane`-owned token-file write anywhere in `go/pkg/`; the survival credential is genuinely new. |
| Lanes run as OS user `striatum-lane` via `sudo -n -u striatum-lane` | **ACCURATE** | `docs/how-to/lane-sandbox.md:34,52,99`; daemon launches lane commands via `sudo -n -u <lane-user> -- env -i …`. |

**Net design implication.** The RFC's premise is fully current: the bug is the
step-3 fall-through to an unreadable admin token across an endpoint rotation,
while the session-bound token (env-only, never on disk) is correct-by-design but
unreachable once the live RPC connection drops. The single most important
decision is OQ1 — and the falsifiers must press hardest on the security invariant
(no widening, no replay, no split-brain). Lean: option 4 (legible failure) as the
immediate, zero-trust-change safety net, plus a narrow option-2/3 survival
mechanism — but the gate decides.

---
<sub>Operator scaffold for the RFC 0143 falsification-gate **design-v2 REVISION**
run. The Holder revises the design-v1 `HOLDER.md` to resolve the seven binding
findings (F1–F7) while holding the security invariant; the falsifiers re-attack
the revised spec. Lanes: author=claude (holder/adjudicator/committer),
reviewer=codex (falsifiers).</sub>

## Maintainer ratification (operator pin — BINDING, supersedes any softer framing above)

The maintainer has ratified the security/authz decision (OQ1) and the F2 replay
defense. These are binding and override any "recommended/optional" language:

- **OQ1 — trust-model shape (ratified): Option 4 + ratification-gated Option 2 + minimal Option 3.**
  - **Slice A (mandatory, lands first, ZERO trust-model change):** Option 4 — the
    legible, self-escalating `session_unrecoverable_across_rotation` signal
    replacing the silent unsealed exit. This is the floor; it must be buildable
    and valuable on its own.
  - **Slice B (ratification-gated):** Option 2's *narrow* reseal authority — a
    session-scoped `CapabilityReseal` covering ONLY the in-flight job's seal
    (`work.complete` / `artifact.publish` / `interrogation.answer`), never any of
    `{admin, apply, recovery, surgical_recovery}` and never plain `write` — folding
    in a minimal Option 3 per-session endpoint+epoch republish so the lane never
    needs to read the admin client-token.
- **F2 — replay defense (DECIDED): non-bearer, daemon-owned, session-tied channel. NO readable reseal token file.**
  Because all lanes currently share the `striatum-lane` OS user, a `0600` reseal
  *file* is a same-uid replay surface readable by sibling lanes. The ratified
  resolution is therefore: deliver/verify the `CapabilityReseal` authority over
  the **daemon-owned supervisor/PTY session-tied channel** — there is NO
  lane-readable reseal token file at all. The spec MUST specify how the reseal
  authority is carried and verified over that session-tied channel (the daemon
  proves the calling session, not a bearer file). This resolves F2 and reshapes
  **F7**: with no file, the epoch-integrity concern moves onto the daemon-owned
  channel (which the daemon controls), so the spec addresses epoch tamper/missing
  on that channel rather than a lane-writable mirror. Do NOT reintroduce a
  readable bearer file as the reseal credential under any option.
