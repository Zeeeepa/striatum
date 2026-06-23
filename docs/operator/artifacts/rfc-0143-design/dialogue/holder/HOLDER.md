# HOLDER — RFC 0143 falsifiable implementation spec

author: holder-author-001

> This is the **leading proposal** for the RFC 0143 design run: a falsifiable,
> buildable spec for *lane credential survival across a daemon boot-epoch
> rotation*. It is the published claim the falsifiers will attack and the
> adjudicator will gate. It resolves all four Open Questions, re-anchors every
> load-bearing claim to the operator-verified anchors against current `main`,
> holds the RFC 0096/#135/#296 session-bound trust model as the spine, and pairs
> each load-bearing security claim with the named test or game-day step that
> would refute it. Read `SEED.md` and
> `docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md` first;
> this spec supersedes the RFC's option sketch where the two disagree (the RFC is
> `proposed`; this spec resolves its open decision).

## Root reframe (held)

**A boot-epoch rotation must never force a lane to choose between reading the
daemon's full-authority bootstrap admin client-token and exiting silently
unsealed.** A `striatum-lane` lane authenticates as *its own* narrow,
session-scoped credential and **never** as the shared operator admin override.
The fix either gives the lane a narrow, session-scoped, lane-readable credential
that survives the rotation, or makes the failure **loud and routed** — never
silent, never via the admin token.

## The load-bearing architectural facts (re-anchored to current `main`)

Three facts the RFC under-states drive the whole design. Each is verifiable and
each is stated as a falsifiable claim below.

### F1 — The session-bound token stays *valid* across a restart; only its *reachability* breaks.

`mintSessionBoundToken` (`go/pkg/mutations/session_token.go:60`) inserts the
client row and per-capability grants into the daemon-owned PostgreSQL
(`striatumd.clients` / `striatumd.client_capabilities`), bound to `session_id`,
24h TTL (`:21`, `:46`, `:77-89`). **PostgreSQL is a separate process that
survives a `striatumd` restart** (D094 / RFC 0043). So after a boot-epoch
rotation the token's client row, its grants, the 24h TTL window, and the session
row are all *still present and still valid*; the production authorizer would
accept the same bearer unchanged. The token is *correct-by-design but
unreachable* — it lives only as the `STRIATUM_MCP_TOKEN` env literal (step 1 of
the resolution chain), and the post-rotation re-readers deliberately skip step 1
(F2). **This means the fix is fundamentally about *reachability*, not about
re-minting authority.**

### F2 — The post-rotation re-readers deliberately ignore the env literal and fall to step 3.

`ResolveTokenMaterialFresh` (`go/pkg/agentloop/endpoint.go:117-138`) — the #323
rotation-recovery reader — reads **only** step 2 (`STRIATUM_MCP_TOKEN_FILE`,
`:119-124`) then step 3 (runtime `client-token`, `:125-136`). It **never** reads
step 1 (`STRIATUM_MCP_TOKEN` env literal); bypassing the cached launch literal is
its entire point. Because today the session-bound token exists *only* as that env
literal and is *never written to any file*, `ResolveTokenMaterialFresh` finds
**no lane-readable session credential on disk** and falls to step 3 — the runtime
`client-token`. `ResolveTokenMaterial` (`go/pkg/agentloop/token.go:18-53`) reaches
the same step-3 branch (`:31-42`) whenever step 1/step 2 are absent (e.g. a CLI
fallback invoked in an env that does not carry `STRIATUM_MCP_TOKEN`). **The named
bug is precisely this step-3 fall-through.**

### F3 — The step-3 target is the full-authority admin token, and the entire runtime dir is owner-only — invisible to a lane.

`admin.BootstrapRuntimeTokenIfNeeded` (`go/pkg/admin/bootstrap.go:18-27`) grants
the runtime `client-token` the full `bootstrapCapabilities` set —
`{admin, read, write, claim, review, apply, recovery, surgical_recovery}` — and
`writeRuntimeToken` (`:134-168`) writes it `0600` inside a `0700` runtime dir.
The boot-epoch file is *also* written owner-only `0600` to that same `0700`
runtime dir (`go/cmd/striatumd/main.go:743-753` `writeBootEpochFile`
→ `writeOwnerOnlyTextFile`). A lane running as `striatum-lane` (via
`sudo -n -u striatum-lane`, `docs/how-to/lane-sandbox.md`) **cannot traverse the
`0700` runtime dir at all**, so it can read *neither* the rotated endpoint, *nor*
the rotated boot epoch, *nor* the `client-token`. `ReadTokenFile`
(`go/pkg/agentloop/token.go:75-92`) would reject the `client-token` anyway
(`mode&0077 != 0` guard, `:82-84`), but the OS-level `0700` dir is the first wall.

### F4 — Endpoint rotation and boot-epoch rotation are *coupled*, and #316 intentionally retires a surviving lane's connection.

Both the dynamic MCP port and the boot epoch are minted **once per daemon
process** (`daemonBootEpoch()` `sync.Once`, `main.go:720-725`; the listener binds
once at `:455`). There is **no** restart that rotates the endpoint without
rotating the epoch. After a restart the surviving lane still carries the **old**
epoch in `STRIATUM_MCP_BOOT_EPOCH` (`supervision_env.go:352-354`), and the new
daemon **rejects any request whose presented epoch differs from its live epoch**
(#316 recycled-port defense — "a mismatch is a recycled-port hit even when the
same installation merely restarted", `main.go:707-719`). So even with a valid,
reachable token, a stale-epoch reconnect is **rejected by design**. #323 recovery
(`go/pkg/agentloop/loop.go:589-613`) rewrites the endpoint + token but **does not
refresh the epoch**, so a claude lane that "reconnects" still presents the stale
epoch and is rejected. **This is the fact the RFC misses: token reachability
(F1–F3) and epoch reachability are two distinct gaps, and #316 makes the epoch
gap a deliberate retirement we must not silently weaken.**

## Decision summary (resolves all four Open Questions)

| OQ | Decision |
| --- | --- |
| **OQ1** | **Option 4 (legible-failure floor) is mandatory and lands first** (zero trust-model change). **Option 2 (narrow, lane-owned, session-scoped *reseal* token) is the recommended survival mechanism, ratification-gated, landing second.** Option 1-alone is rejected (preserves the silent dead-end); Option 3-as-primary is rejected (re-injecting env into a *running* process is impossible — it degenerates to Option 2's file plus heavier rotation plumbing). A minimal Option-3-style **per-session republish of endpoint+epoch** is folded into Option 2 to close the F4 epoch gap *without weakening #316*. |
| **OQ2** | A **new `CapabilityReseal` credential class**: authorizes only `work.complete` + `artifact.publish` + `interrogation.answer` **scoped to the session's current in-flight job**; carries **none** of `{admin, apply, recovery, surgical_recovery}` and **not** general `claim`/`write`. Bound to `session_id`, file `striatum-lane`-owned `0600`, TTL = `min(session TTL, reseal window)`. Invalidated on `session close` (row + grants deleted, file removed), on TTL, and gated-on-use by session-recognition + the #316 epoch check. |
| **OQ3** | Mint: new `mintSessionResealToken` beside `mintSessionBoundToken` (`session_token.go`). Capability: new `rpc.CapabilityReseal` + handler scoping. Injection: `supervisedEnvEntries` (`supervision_env.go:320`) writes the file + sets `STRIATUM_MCP_TOKEN_FILE`; minted in the `HandleSuperviseStart` tx (`supervision_control.go:158`). Rotation republish: `striatumd` boot (`main.go:465-466`). Refusal/floor: `ResolveTokenMaterial` (`token.go:31-42`) + `ResolveTokenMaterialFresh` (`endpoint.go:125-136`). #323 wiring: `loop.go:589-613` / `applyMCPEndpointRotation` (`:619`). |
| **OQ4** | A typed `session_unrecoverable_across_rotation` recovery class. The resolution chain refuses to consume the runtime `client-token` for a supervised lane and returns `ErrSessionUnrecoverableAcrossRotation`; the agent-loop surfaces it as a self-escalating signal routed to the supported operator requeue (`supervise stop` → `session close` → `recovery auto`) — never a silent unsealed exit, never a misleading "permission denied". |

## OQ1 — Which trust-model option (THE security/authz call)

**Decision: land Option 4 as the mandatory floor first; recommend Option 2 as the
ratification-gated survival mechanism, with a minimal Option-3 republish folded in
to close the epoch gap. Reject Option 1-alone and Option 3-as-primary.**

Rationale, option by option:

- **Option 1 (status quo) — rejected as the *whole* answer, retained as the
  *terminal* behavior.** The operator requeue (`supervise stop` → `session close`
  → `recovery auto`) remains the supported recovery when a lane genuinely cannot
  reseal. But status-quo *alone* leaves the #512 defect intact: the lane exits
  *silently unsealed* behind a *misleading* "permission denied". Option 4 keeps
  the requeue as the destination but makes the path to it loud and typed.

- **Option 2 (durable lane-owned reseal token) — recommended primary survival
  mechanism.** It is the *minimal* change that fixes F2: writing a lane-readable
  session credential to disk (step 2, `STRIATUM_MCP_TOKEN_FILE`) makes
  `ResolveTokenMaterialFresh` find it *instead of* falling to step 3. By F1 the
  credential does not need re-minting on rotation — only *reachability* — so a
  file written **once** at supervise start suffices for the token. Scoped to a new
  `CapabilityReseal` (OQ2), it is structurally incapable of the admin/apply/
  recovery widening R1 forbids.

- **Option 3 (re-mint + re-inject on rotation) — rejected as primary.** You
  **cannot mutate the environment of an already-running lane process**; "re-inject
  into the live lane env" is not implementable. Its only realizable form is
  "write a token file the lane re-reads," which *is* Option 2's mechanism — plus
  the daemon having to enumerate live lanes on boot and re-mint per rotation
  (more moving parts, more attack surface, and it fights F1, which shows
  re-minting is unnecessary). **However**, F4 shows the token alone cannot
  reconnect across the #316 epoch guard, so we adopt the *narrowest* Option-3
  element: on boot, for each still-live supervised session, the daemon republishes
  the fresh **endpoint + epoch** (not the token) into that session's
  **lane-readable** scratch mirror, so the lane can present the fresh epoch *the
  legitimate daemon wrote*. This is endpoint/epoch *reachability* plumbing, not a
  credential re-mint.

- **Option 4 (narrow the fallback / legible failure) — mandatory, lands first.**
  It is zero-trust-change: it only *removes* a fall-through (step 3) a lane could
  never legitimately use, and converts the silent/misleading dead-end into a
  typed, routed escalation. It is also the **floor** that catches every case
  Option 2 does not (no reseal token; session retired; epoch unverifiable;
  in-flight job already gone). **Option 2 without Option 4 would still have a
  silent dead-end in its own failure modes; Option 4 without Option 2 is a
  complete, shippable, conservative fix.** Hence: ship Option 4 first.

**Sequencing.** Slice A = Option 4 (no credential mint; lands immediately).
Slice B = Option 2 + the minimal Option-3 republish (new credential class; **gated
on maintainer ratification**, see the ratification gate below).

## OQ2 — Surviving authority + lifecycle of the new credential class

**The reseal token is a new `rpc.CapabilityReseal` credential, deliberately
narrower than the session-bound token.**

- **Capabilities that survive (minimal).** `CapabilityReseal` authorizes **only**:
  `work.complete`, `artifact.publish`, and `interrogation.answer` — and **only for
  the session's own current in-flight job**. It does **not** grant general `claim`
  (cannot claim *new* work across the rotation), nor general `write` (cannot
  publish into another job), nor `read`/`review`, and categorically **none** of
  `{admin, apply, recovery, surgical_recovery}`. Choosing a *dedicated capability*
  over "reuse `{claim, write}`" is deliberate: it makes R1/R3 *structurally* true
  (the token literally has no verb to escalate or to touch a foreign job), not
  merely enforced by a handler check.
- **Lifecycle / TTL.** Minted in the `HandleSuperviseStart` transaction beside the
  session-bound token, bound to the same `session_id`. `expires_at =
  min(sessionBoundTokenTTL, resealWindow)`; the recommended `resealWindow` is the
  session TTL (24h) for v1 simplicity, with a tighter window (e.g. the supervisor
  lease horizon) called out as a defensible hardening for the falsifiers to weigh.
- **File ownership / mode.** Written `striatum-lane`-owned `0600` to the lane's own
  operational scratch (e.g. `.striatum/scratch/<session>/reseal-token`, never
  committed), pointed to by `STRIATUM_MCP_TOKEN_FILE`. `0600` passes
  `ReadTokenFile`'s `mode&0077 != 0` guard (`token.go:82-84`) and is unreadable by
  *other* OS users.
- **Invalidation triggers (all three).** (a) **`session close`** deletes the
  client row + grants *and* removes the file (so a post-close file cannot seal).
  (b) **TTL** expiry via the PG `expires_at` on the grants — same enforcement path
  as the session-bound token, no new code. (c) A **new boot epoch** does *not* by
  itself invalidate the PG-resident token (F1), but its *use* is gated by the
  daemon's session-recognition check **and** the #316 epoch check, so a token for a
  session the new daemon has retired cannot seal (R3).
- **Cross-lane file-read note (R2, addressed head-on).** If all lanes share the
  `striatum-lane` uid, `0600` does not isolate lane A's file from lane B's process.
  This is *contained*, not escalated, because the token's authority is bound to
  `session_id`: presenting session A's reseal token while acting as session B is
  refused by the bound-enforcement (`IsSessionBound`, session_token.go:43-46). The
  file is also removed on session close to shrink the window. (A per-lane uid, if
  the sandbox later adopts one, closes even the read.)

## OQ3 — Where the mechanism lives (exact code sites)

| Element | Site | Change |
| --- | --- | --- |
| **Mint (existing)** | `go/pkg/mutations/session_token.go:60` `mintSessionBoundToken` | unchanged (still the live env token). |
| **Mint (new)** | `go/pkg/mutations/session_token.go` — new `mintSessionResealToken` | inserts a `CapabilityReseal`-only client bound to `session_id`; returns bearer for the file write. |
| **New capability** | `go/pkg/rpc` capability enum + the authorizers | add `rpc.CapabilityReseal`; scope `work.complete` / `artifact.publish` / `interrogation.answer` handlers to accept it **only** for the session's in-flight job. |
| **Injection** | `go/pkg/mutations/supervision_env.go:320` `supervisedEnvEntries` | write the `0600` reseal file to lane scratch and append `STRIATUM_MCP_TOKEN_FILE=<path>` (today *not* injected — new wiring); mint in the `HandleSuperviseStart` tx (`supervision_control.go:158`). |
| **Rotation republish (Option-3 element)** | `go/cmd/striatumd/main.go:465-466` boot path (beside `daemonBootEpoch()` / `writeBootEpochFile`) | for each still-live supervised session, write the fresh **endpoint + epoch** into that session's **lane-readable** scratch mirror (daemon-written, lane-read-only). |
| **Resolution-chain refusal (floor)** | `go/pkg/agentloop/token.go:31-42` (step-3 branch) **and** `go/pkg/agentloop/endpoint.go:125-136` (`ResolveTokenMaterialFresh`) | a supervised lane never consumes the runtime `client-token`; return typed `ErrSessionUnrecoverableAcrossRotation`. |
| **#323 recovery** | `go/pkg/agentloop/loop.go:589-613` + `applyMCPEndpointRotation:619` | read endpoint+epoch+reseal-token from the lane scratch mirror; reconnect with the fresh epoch + reseal token, or emit `session_unrecoverable_across_rotation`. |

The `STRIATUM_MCP_TOKEN` env literal (step 1) is unchanged and still wins in
normal operation, so the durable reseal file is consulted **only on the fresh
re-read path** — i.e. only during a rotation reseal — keeping the durable file's
blast radius to the reseal capability and to the rotation window.

## OQ4 — Legible-failure fallback (the loud floor)

Define a typed recovery class **`session_unrecoverable_across_rotation`**:

1. **Refuse the admin token.** `ResolveTokenMaterial` (`token.go:31-42`) and
   `ResolveTokenMaterialFresh` (`endpoint.go:125-136`) must **not** return the
   runtime `client-token` for a supervised-lane context. Replace the silent
   step-3 read with a typed `ErrSessionUnrecoverableAcrossRotation` sentinel.
2. **Self-escalate.** The agent-loop maps that sentinel to a
   `session_unrecoverable_across_rotation` signal emitted in-band (e.g. a
   `work.block` with the typed reason, and/or `session.report
   report_kind=escalate`) so `run drive` / the recovery sweep routes it to the
   operator requeue. The codex-specific wedge prompt (`loop.go:669` family)
   already demonstrates the in-band-surface pattern; this generalizes it to a
   typed recovery class.
3. **No silent exit, no misleading error.** The lane must not exit `0` unsealed
   and must not surface a raw "permission denied". The terminal state is an
   *explicit, routed* escalation.

This holds even when Slice B (the reseal token) lands: any path where the lane
*genuinely cannot* reseal (no token, retired session, unverifiable epoch) falls
to this same loud floor instead of back to step 3.

## Security invariant (the spine) — held explicitly

Per the anchor table: the runtime `client-token` carries the **full**
`bootstrapCapabilities` set and is `0600` owner-only by construction
(`bootstrap.go:18-27`, `:134-168`). **Any option that lets a lane read that file,
or that mints a lane-readable token carrying ANY of
`{admin, apply, recovery, surgical_recovery}`, is categorically out of bounds.**
This spec makes that **structurally impossible**, not merely policy-enforced:

- The lane is never granted OS read of the `0700` runtime dir (F3); the floor
  (OQ4) removes the only code path that would have read the `client-token`.
- The only new lane-readable credential is the `CapabilityReseal` token, whose
  capability set *contains no elevated verb to grant* — there is no code path by
  which it could present admin/apply/recovery authority.
- The epoch republish moves **endpoint + epoch only** (non-secret anti-confusion
  tags), never the admin token, never any credential the lane did not already
  hold for its own session.

## Falsifiable assertions (each with the named test / game-day that refutes it)

> Every assertion is stated so that a single named test or game-day step, if it
> fails, *refutes the claim*. The falsifiers should attack these directly.

- **A1 — No-widening (R1).** The reseal token carries only `CapabilityReseal`.
  *Refuted if:* `TestResealTokenRefusesElevatedCaps` shows the token authorizing
  any of `admin`/`apply`/`recovery`/`surgical_recovery`, **or** `work.claim_next`
  (claiming *new* work) succeeds with it. *Expected:* `capability_missing` /
  refused for all of these.

- **A2 — No admin-token fall-through (the named bug, R1).**
  `ResolveTokenMaterial` and `ResolveTokenMaterialFresh` never return the runtime
  `client-token` for a supervised lane. *Refuted if:*
  `TestResolveRefusesRuntimeClientTokenForLane` (lane context, runtime dir holds a
  `client-token`, no step-1/step-2 material) returns the admin token instead of
  `ErrSessionUnrecoverableAcrossRotation`.

- **A3 — No-replay past session close (R2).** A reseal-token file persisting after
  `session close` cannot seal. *Refuted if:* `TestResealTokenRejectedAfterSessionClose`
  shows a still-on-disk token sealing after the session is closed, **or** the file
  is not removed on close. *Expected:* PG row gone → refused, file absent.

- **A4 — No split-brain (R3).** A reseal seals only the in-flight job the daemon
  still recognizes; it cannot write into a retired session or a foreign job.
  *Refuted if:* `TestResealRefusedForRetiredSession` shows a reseal succeeding
  after the session is retired, **or** a reseal scoped to job X publishing into
  job Y.

- **A5 — Loud failure (R4).** The Option-4 path emits
  `session_unrecoverable_across_rotation` and the run routes it. *Refuted if:*
  game-day **GD-1** (restart `striatumd` mid-job, no reseal token) shows a silent
  unsealed exit, a raw "permission denied", **or** the run failing to requeue.
  *Expected:* a typed, routed escalation; operator requeue completes the seal.

- **A6 — Epoch republish does not weaken #316.** A lane adopts a fresh epoch only
  from the daemon-written, owner-trusted per-session scratch mirror; a different
  daemon on a recycled port is still rejected. *Refuted if:* game-day **GD-2**
  (bind a *different* daemon to the recycled port after the lane launched) shows
  the surviving lane resealing into the wrong daemon, **or** a lane adopting an
  epoch from any source the legitimate daemon did not write.

- **A7 — Owner-only reseal file.** The reseal file is `0600`; a group/other
  -readable reseal file fails closed. *Refuted if:* `TestResealTokenFileOwnerOnly`
  shows `ReadTokenFile` accepting a `0640`/`0644` reseal file (the existing
  `mode&0077` guard, `token.go:82-84`, must reject it).

- **A8 — Token validity survives the restart (F1).** A session-bound (or reseal)
  token minted before the rotation still authorizes against the restarted daemon
  (same PG, same session row, TTL unexpired). *Refuted if:*
  `TestTokenValidAcrossRestart` shows the PG-resident token rejected purely
  because the daemon process restarted (as opposed to epoch/session-retirement
  gating).

## Scope discipline (Non-Goals held)

- Does **not** re-classify the downstream `agent_exited_unsealed` recovery policy
  (RFC 0152 / D249) — that governs what happens *after* an unsealed exit; this
  governs whether the exit happens silently.
- Does **not** change the committee POSIX-ACL repo provisioning (#537 / #539) —
  that is target-repository filesystem ACL, not a daemon-private credential.
- Does **not** touch `run drive`'s transient-socket behavior (#513).
- Does **not** weaken the #316 boot-epoch recycled-port defense (A6 is the guard).
- Local-first, single-host, daemon-owned PostgreSQL as the single writer.

## Maintainer ratification gate (required)

**Slice B (Option 2 + the minimal Option-3 republish) introduces a new
capability class (`rpc.CapabilityReseal`), a new durable lane-readable credential
file, and new rotation plumbing — a security/authz trust-model change.** This
cleared spec is a **RECOMMENDATION the maintainer ratifies before any build slice
touches credential code.** Slice A (Option 4, the legible-failure floor) is
zero-trust-change and may land first under the normal review gate; **Slice B does
not land until the maintainer ratifies the new credential class.** Falsifier
clearance of this spec is *not* maintainer ratification — the adjudicator's
collaboration ledger gates the dialogue; the maintainer gates the credential
code.

---
<sub>Holder leading proposal for the RFC 0143 falsification-gate design run. The
adjudicator's collaboration ledger — not falsifier completion — decides whether
this gate clears.</sub>
