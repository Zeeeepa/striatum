# HOLDER — RFC 0143 falsifiable implementation spec (design-v2 REVISION)

author: holder-author-002

> This is the **revised leading proposal** for RFC 0143 (*lane credential
> survival across a daemon boot-epoch rotation*). The design-v1 falsification
> gate returned **`needs_revision`** with **seven findings F1–F7** — all material,
> all unrebutted. This revision resolves every one with a concrete mechanism, a
> named code site, and a named falsifying test/game-day, while holding the RFC
> 0096/#135/#296 session-bound trust model as the spine and *building on* (not
> regressing) the v1 strengths the cycle-1 ledger credited.
>
> **The single structural change from v1.** The maintainer ratification pin in
> `SEED.md` (binding, supersedes softer framing) decided F2: **there is NO
> lane-readable reseal token file at all.** Because every lane currently shares
> the `striatum-lane` OS uid, any `0600` bearer file is a same-uid replay surface.
> The reseal authority is therefore carried and verified over the **daemon-owned
> supervisor/PTY session-tied channel** — the daemon proves the calling session
> from its own supervision row + the helper's process identity, never from a
> bearer file a sibling lane could read. This one decision dissolves F2 at the
> root and reshapes F1, F5, F6, and F7 around the same daemon-owned channel.
>
> Read `SEED.md` (charter, four Open Questions, operator anchor table, the
> `## Binding revision constraints` F1–F7 + maintainer pin), the canonical RFC
> `docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md`, the
> v1 spec `docs/operator/artifacts/rfc-0143-design/dialogue/holder/HOLDER.md`,
> and the cycle-1 ledger
> `docs/operator/artifacts/rfc-0143-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
> first. This spec supersedes the RFC's option sketch where they disagree (the
> RFC is `proposed`; this spec resolves its open decision and is itself a
> RECOMMENDATION the maintainer ratifies — see the ratification gate).

## Root reframe (held)

**A boot-epoch rotation must never force a lane to choose between reading the
daemon's full-authority bootstrap admin client-token and exiting silently
unsealed.** A `striatum-lane` lane authenticates as *its own* narrow,
session-scoped credential and **never** as the shared operator admin override.
The fix either lets the lane's in-flight work be sealed over a narrow,
session-scoped, **daemon-owned channel** that survives the rotation, or makes the
failure **loud and routed** — never silent, never via the admin token.

## Naming note (avoid the v1 label collision)

The cycle-1 gate uses **F1–F7** for its seven *findings*. The v1 spec also used
"F1–F4" for its four *architectural facts*, which collides. This revision renames
the architectural facts to **AF1–AF4** and reserves **F1–F7** exclusively for the
gate findings. AF1–AF4 are the v1 strengths the ledger credited; they are
preserved verbatim in substance below.

## Architectural facts re-anchored to current `main` (AF1–AF4, preserved v1 strengths)

### AF1 — The session-bound token stays *valid* across a restart; only its *reachability* breaks. (credited v1 strength — kept)

`mintSessionBoundToken` (`go/pkg/mutations/session_token.go:60`) inserts the
client row + per-capability grants into daemon-owned PostgreSQL
(`striatumd.clients` / `striatumd.client_capabilities`), bound to `session_id`,
24h TTL (`:21`, `:46`, `:77-89`). **PostgreSQL survives a `striatumd` restart**
(D094 / RFC 0043). After a boot-epoch rotation the client row, grants, TTL
window, and session row are all still present and valid; the production
authorizer would accept the same bearer unchanged. The token is
*correct-by-design but unreachable* — it lives only as the `STRIATUM_MCP_TOKEN`
env literal (step 1), and the post-rotation re-readers deliberately skip step 1.
**The fix is fundamentally about reachability/routing, not re-minting authority.**

### AF2 — The post-rotation re-readers ignore the env literal and fall to step 3.

`ResolveTokenMaterialFresh` (`go/pkg/agentloop/endpoint.go:117-138`) — the #323
rotation-recovery reader — reads only step 2 (`STRIATUM_MCP_TOKEN_FILE`,
`:119-124`) then step 3 (runtime `client-token`, `:125-136`); it never reads step
1. `ResolveTokenMaterial` (`go/pkg/agentloop/token.go:18-53`) reaches the same
step-3 branch (`:31-41`) whenever step 1/2 are absent. Since the session-bound
token exists only as the env literal and is never written to disk, the fresh
re-read finds no lane-readable credential and falls to step 3 — **the named
bug.**

### AF3 — The step-3 target is the full-authority admin token in a `0700` dir invisible to a lane.

`admin.BootstrapRuntimeTokenIfNeeded` (`go/pkg/admin/bootstrap.go:18-27`) grants
the runtime `client-token` the full `bootstrapCapabilities` set
`{admin, read, write, claim, review, apply, recovery, surgical_recovery}`;
`writeRuntimeToken` (`:139-142`, `:153/:164`) writes it `0600` inside a `0700`
runtime dir. The boot-epoch file is written owner-only to the same dir
(`go/cmd/striatumd/main.go:739-753`). A lane running as `striatum-lane` **cannot
traverse the `0700` dir at all**, so it can read neither the rotated endpoint,
nor the rotated boot epoch, nor the `client-token`; `ReadTokenFile`
(`token.go:81-84`, `mode&0077 != 0`) would reject the token anyway. The OS-level
`0700` dir is the first wall, and it is load-bearing for the security invariant.

### AF4 — Endpoint rotation and boot-epoch rotation are coupled, and #316 intentionally retires a surviving lane's connection. (credited v1 strength — kept)

The dynamic MCP port and the boot epoch are both minted once per daemon process
(`daemonBootEpoch()` `sync.Once`, `main.go:720-725`; listener binds once). No
restart rotates the endpoint without rotating the epoch. After a restart the
surviving lane still carries the **old** epoch in `STRIATUM_MCP_BOOT_EPOCH`
(`supervision_env.go:352-354`), and #316 rejects any request whose presented
epoch differs from the live epoch (`main.go:697-719`) — a deliberate
recycled-port defense we must not silently weaken. #323 recovery
(`loop.go:589-613`) rewrites endpoint+token but **does not** refresh the epoch.
**Token reachability (AF1–AF3) and epoch reachability are two distinct gaps;
#316 makes the epoch gap a deliberate retirement.**

## Decision summary (resolves all four Open Questions)

| OQ | Decision |
| --- | --- |
| **OQ1** | **Ratified: Option 4 (mandatory floor, lands first, zero trust change) + ratification-gated Option 2 (narrow reseal authority over the daemon-owned supervisor channel) + minimal Option 3 (per-session endpoint+epoch republish over that same channel).** Option-1-alone rejected (keeps the silent dead-end); Option-3-as-primary rejected (cannot mutate a running process's env). The reseal authority is **never a lane-readable bearer file** — it travels over the daemon-owned supervisor/PTY session-tied channel (maintainer pin). |
| **OQ2** | A new **`rpc.CapabilityReseal`** capability authorizing **only** `work.complete` + `artifact.publish` + `interrogation.answer`, **only for the session's own in-flight job**, never any of `{admin, apply, recovery, surgical_recovery}` and **never plain `write`/`claim`/`read`/`review`**. It is **never minted into a lane-readable file**; it is projected by the daemon on the supervisor-proven reseal path, scoped by `session_id`. Lifecycle: bound to the session; valid only inside the **active lease window** (+ a bounded daemon-side reseal grace, F5); invalidated on `session close` (rows gone), on lease expiry/requeue (recovery-generation change), and on a new boot epoch the supervised path does not accept. |
| **OQ3** | Enum + route alternate: `go/pkg/rpc/registry.go:16-23` (new `CapabilityReseal`), `MethodEntry.ResealAlternate` on the three routes (`registry_methods.go:8-10`); prelude alternate projection in `server.go:107-111` + `MemoryAuthorizer.Authorize` (`capability.go:175-233`) + `PostgresAuthorizer`/`striatumd.authorize_capability` (`auth_pg.go:159-206`), recorded in `AuthContext.Capability` (`capability.go:28`). Daemon-internal reseal entrypoint over the supervisor channel: `go/pkg/supervisor/helper_protocol.go` (new control event) → daemon supervise handler → new `resealInFlightJob` in `go/pkg/mutations/` reusing `activeLeaseFor` (`mutations.go:803`). Floor refusal: `token.go:31-41` + `endpoint.go:125-136`. Epoch+endpoint republish: `main.go:459-486`. #323 wiring: `loop.go:589-613` / `applyMCPEndpointRotation:619`. |
| **OQ4** | Typed recovery class **`session_unrecoverable_across_rotation`** recorded as a **durable daemon blocker** (new `blocker_kind`) via the supervisor control channel — NOT a local process error. The resolution chain refuses the runtime `client-token` for a supervised lane (`ErrSessionUnrecoverableAcrossRotation`); the lane emits a typed exit code + structured helper line; the daemon-owned helper forwards it; the daemon records the blocker and the recovery sweep routes it to the operator requeue. Every Slice-B refusal (no predicate, expired/requeued lease, missing/stale epoch) falls to this same floor — never a raw `lease_error`, never a silent unsealed exit. |

## OQ1 — Which trust-model option (THE security/authz call)

**Decision (ratified in `SEED.md`): land Option 4 as the mandatory floor first;
land Option 2's narrow reseal authority — carried over the daemon-owned
supervisor/PTY channel, never a bearer file — as the ratification-gated survival
mechanism, with a minimal Option-3 endpoint+epoch republish over that same
channel. Reject Option-1-alone and Option-3-as-primary.**

- **Option 1 (status quo) — rejected as the whole answer, retained as the
  terminal behavior.** The operator requeue (`supervise stop` → `session close`
  → `recovery auto`) remains the supported recovery when a lane genuinely cannot
  reseal. Status-quo *alone* leaves #512 intact (silent unsealed exit behind a
  misleading "permission denied"); Option 4 keeps the requeue as destination but
  makes the path to it loud, typed, and durably recorded.
- **Option 2 (narrow reseal authority) — recommended primary survival mechanism,
  re-shaped to a non-bearer channel.** v1 wrote a `0600` lane-readable reseal
  *file*; the maintainer pin retires that (same-uid replay, F2). Instead the
  reseal authority is the **supervisor-proven, session-tied channel**: the lane
  emits a structured reseal request over the PTY/helper bridge (no credential),
  the daemon-owned helper forwards it, and the daemon performs the seal under an
  internally-projected `CapabilityReseal` scope after the F3 predicate holds.
  Structurally incapable of admin/apply/recovery widening (OQ2) and structurally
  incapable of same-uid replay (no bearer to steal, F2).
- **Option 3 (re-mint + re-inject on rotation) — rejected as primary (credited
  v1 strength, kept).** You cannot mutate the env of an already-running process.
  Its only realizable form is a token file the lane re-reads — which the pin
  forbids. We adopt only its *narrowest, non-credential* element: on boot, for
  each still-live supervised session, the daemon republishes the fresh
  **endpoint + epoch** (not any token) over the daemon-owned channel so a lane
  that wants to resume normal MCP work can present the fresh epoch the legitimate
  daemon delivered. This is endpoint/epoch reachability plumbing, not a credential
  re-mint, and it never weakens #316 (F7).
- **Option 4 (legible failure) — mandatory, lands first, zero trust change.** It
  only *removes* a fall-through (step 3) a lane could never legitimately use, and
  converts the silent/misleading dead-end into a typed, durably-recorded,
  routed escalation. It is the floor that catches every case Slice B does not (no
  reachable channel, retired session, expired/requeued lease, unverifiable epoch,
  in-flight job already gone). **Option 4 without Option 2 is a complete,
  shippable, conservative fix; Option 2 without Option 4 would still have silent
  dead-ends in its own failure modes.** Ship Option 4 first.

**Sequencing.** Slice A = Option 4 (no new capability; lands immediately under the
normal review gate once F1 makes it routed). Slice B = Option 2 (the
`CapabilityReseal` reseal-over-supervisor-channel) + the minimal Option-3
endpoint+epoch republish (**gated on maintainer ratification** — see the gate).

## OQ2 — Surviving authority + lifecycle of the new capability class

**`rpc.CapabilityReseal` is a new capability, deliberately narrower than the
session-bound token, and it is NEVER materialized into a lane-readable file.**

- **Capabilities (minimal).** `CapabilityReseal` authorizes **only**
  `work.complete`, `artifact.publish`, and `interrogation.answer`, and **only for
  the session's own current in-flight job**. It does **not** grant `claim`
  (cannot claim new work), `write` (cannot publish into another job), `read` /
  `review`, and categorically **none** of `{admin, apply, recovery,
  surgical_recovery}`. A *dedicated capability* (not "reuse `{claim,write}`")
  makes R1/R3 *structurally* true — the authority literally has no verb to
  escalate or to touch a foreign job — and is what lets the guardrail tests prove
  no-widening (F4 / A1).
- **How the authority is carried (F2 — non-bearer).** There is **no reseal token
  file** under any path. The authority is projected by the daemon on the
  supervisor-proven reseal path: the daemon-owned helper (daemon uid, distinct
  from `striatum-lane`) forwards the lane's structured reseal request over the
  helper→daemon control-event stream; the daemon matches the helper's
  `supervisor_id` to the `session_id` it supervises (the daemon-held supervision
  row) and constructs `AuthContext{Capability: CapabilityReseal, SessionID: s}`
  internally. A bearer never reaches the lane.
- **Lifecycle / window (F5).** Survival is scoped to the **active lease window of
  the in-flight job**, not the 24h token TTL. The daemon-internal reseal
  transaction admits the seal only if the lease is active *or* within a bounded
  daemon-side **reseal grace** it may extend the *same* lease row by (covering the
  seal round-trip), never a general lane-invokable heartbeat. Outside that window
  (lease already expired and not within grace, or the job requeued) the reseal is
  refused and falls to Option 4.
- **Invalidation triggers.** (a) `session close` deletes the session/lease state,
  so the supervisor-proven predicate fails. (b) Lease expiry beyond grace, or a
  recovery requeue that bumps the recovery/lease generation, fails the F3
  predicate. (c) A new boot epoch the supervised path does not accept (F7) fails
  the predicate. There is no durable artifact (no file) to invalidate.

## OQ3 — Where the mechanism lives (exact code sites)

| Element | Site | Change |
| --- | --- | --- |
| **Mint (existing)** | `go/pkg/mutations/session_token.go:60` `mintSessionBoundToken` | unchanged (still the live env token; AF1). |
| **New capability enum** | `go/pkg/rpc/registry.go:16-23` | add `CapabilityReseal Capability = "reseal"` to the enum and the `Capabilities` validity set. |
| **Route alternate** | `go/pkg/rpc/registry.go:47` (`MethodEntry`) + `registry_methods.go:8-10` | add `ResealAlternate bool` to `MethodEntry`; set it `true` ONLY for `interrogation.answer`, `work.complete`, `artifact.publish`. `RequiredCapability` stays `CapabilityWrite` (unchanged for normal callers). |
| **Prelude alternate projection** | `go/pkg/rpc/server.go:107-111` | after `Authorize(entry.RequiredCapability,…)` returns `capability_missing`, if `entry.ResealAlternate` re-authorize against `CapabilityReseal`; on success thread `AuthContext{Capability: CapabilityReseal}`. Never falls back to plain `write`. |
| **Authorizer projection** | `go/pkg/rpc/capability.go:175-233` (`MemoryAuthorizer.Authorize`) + `go/pkg/rpc/auth_pg.go:159-206` (`PostgresAuthorizer`) + `striatumd.authorize_capability` PG fn | project the reseal-alternate grant and return the *resolved* capability so `AuthContext.Capability` (`capability.go:28`, set at `:230` / `auth_pg.go:203`) records `reseal`, not `write`. |
| **Handler scoping** | `work.complete` / `artifact.publish` (`go/pkg/mutations/lifecycle.go`, `review.go`/publish path) + `interrogation.answer` | branch on `ctx.Capability == CapabilityReseal` (read via `AuthFromContext`, `capability.go:58`) to enforce the in-flight-job-only F3 predicate; deny if the bound `session_id`/job does not match. |
| **Daemon-internal reseal entrypoint (channel)** | `go/pkg/supervisor/helper_protocol.go` (new `HelperEventResealRequested` / typed `agent_exited` reason) → daemon supervise handler that owns supervise.* + the helper control stream | the helper (daemon uid) forwards the structured reseal request; the daemon maps `supervisor_id`→`session_id` and calls the seal under the projected `CapabilityReseal` AuthContext. No token round-trips to the lane. |
| **Split-brain predicate** | new `resealInFlightJob` in `go/pkg/mutations/` reusing `activeLeaseFor` (`go/pkg/mutations/mutations.go:803`) + recovery/lease-generation check | F3 (below). |
| **Resolution-chain refusal (Slice A floor)** | `go/pkg/agentloop/token.go:31-41` (step-3 branch) **and** `go/pkg/agentloop/endpoint.go:125-136` (`ResolveTokenMaterialFresh`) | a supervised lane never consumes the runtime `client-token`; return typed `ErrSessionUnrecoverableAcrossRotation`. |
| **Self-escalation route (Slice A)** | `go/pkg/agentloop/loop.go:589-613` + `applyMCPEndpointRotation:619` → `go/pkg/supervisor/helper.go` → daemon supervise handler | map the sentinel to a typed agent-loop exit code + structured helper line; the helper forwards it; the daemon records a durable `session_unrecoverable_across_rotation` blocker the recovery sweep routes. |
| **Endpoint+epoch republish (minimal Option 3)** | `go/cmd/striatumd/main.go:459-486` boot path (beside `daemonBootEpoch()`/`writeBootEpochFile`) | for each still-live supervised session, deliver the fresh endpoint+epoch over the daemon-owned helper channel (or a daemon-uid, lane-read-ONLY file outside the lane-writable scratch ACL — F7), never any token. |

The `STRIATUM_MCP_TOKEN` env literal (step 1) is unchanged and still wins in
normal operation; the supervisor reseal path is consulted **only** when the live
MCP connection is gone after a rotation, keeping the new surface to the reseal
capability + the rotation window.

## OQ4 — Legible-failure fallback (the loud, routed floor)

Define a typed recovery class **`session_unrecoverable_across_rotation`** recorded
as a **durable daemon blocker** (new `blocker_kind`, in the `blockers` table the
recovery sweep already reads — `go/pkg/reads/detail.go:391`, `escalation_resolve.go:328`):

1. **Refuse the admin token.** `ResolveTokenMaterial` (`token.go:31-41`) and
   `ResolveTokenMaterialFresh` (`endpoint.go:125-136`) must not return the runtime
   `client-token` for a supervised-lane context; return
   `ErrSessionUnrecoverableAcrossRotation`.
2. **Escalate over the daemon-owned channel (not an MCP verb the lane lacks
   auth for).** The agent-loop maps the sentinel to a typed exit code AND a
   structured line on the PTY/helper bridge. The **daemon-owned helper**
   (`go/pkg/supervisor/helper.go`, daemon uid) parses it into a control event
   and forwards it to the daemon core over the already-authenticated helper→daemon
   stream. The daemon records the durable `session_unrecoverable_across_rotation`
   blocker — **a recorded state transition, not a local process error.** This is
   the F1 fix: the floor does NOT use `work.block` (needs `CapabilityWrite`) or
   `session.report` (needs `CapabilityClaim`), which the no-token lane cannot
   reach; it uses the supervisor channel the daemon already trusts.
3. **No silent exit, no misleading error.** The lane must not exit `0` unsealed
   and must not surface a raw "permission denied" / `lease_error`. The terminal
   state is an explicit, durably-recorded, routed escalation; the recovery sweep
   routes it to the operator requeue.

This holds even when Slice B is enabled: every reseal refusal (no reachable
channel, retired session, expired/requeued lease, unverifiable epoch, job gone)
falls to this same floor instead of back to step 3 or to a raw `lease_error`.

## Resolution of the seven binding revision constraints (F1–F7)

### F1 — Make the Option-4 floor actually routed (was: floor cannot self-escalate with no credential)

**Chosen prescribed-fix option: an already-authenticated supervisor channel +
typed agent-loop exit code, producing a durable daemon blocker.** The no-token
lane never calls `work.block`/`session.report` (both need caps it lacks). It
emits `ErrSessionUnrecoverableAcrossRotation` as a **typed agent-loop exit code**
and a **structured line on the PTY/helper bridge**. The **daemon-owned helper**
(`go/pkg/supervisor/helper.go`, runs as the daemon uid, not `striatum-lane`)
forwards it over the helper→daemon control-event stream
(`HelperControlEvent`, `helper_protocol.go`) into the daemon core, which records a
durable `session_unrecoverable_across_rotation` blocker (new `blocker_kind`) on
the in-flight job — a recorded state transition the recovery sweep routes. No MCP
auth is required of the lane because the helper, not the lane, talks to the
daemon.
**Refuting test/game-day — GD-1:** restart `striatumd` mid-job so the lane loses
its connection with no reachable `STRIATUM_MCP_TOKEN_FILE`; assert the run shows
a **durable `session_unrecoverable_across_rotation` blocker/requeue event**, not a
local process error, a silent unsealed exit, or a raw permission error. *Refuted
if* the terminal state is any of those, or no durable daemon event is recorded.

### F2 — Defeat same-uid replay (was: durable `0600` bearer is a same-uid replay surface)

**Chosen prescribed-fix option (maintainer-ratified): a non-bearer reseal channel
tied to the supervised PTY/session, with daemon-side proof that the caller is the
original supervisor process. There is NO lane-readable reseal token file.** The
reseal authority is `CapabilityReseal` *projected by the daemon* on the
supervisor-proven path: the helper's process identity
(`go/pkg/supervisor/process_identity_linux.go`) + the daemon's supervision row
(`supervisor_id` ↔ `session_id`) prove the calling session; a sibling
`striatum-lane` process cannot forge the helper identity nor write the daemon-owned
helper→daemon channel (it holds no handle to it and runs under the helper's PTY,
not beside it). With no bearer on disk there is nothing for a sibling to read and
replay.
**Refuting test — `TestBorrowedResealBearerCannotSealVictimSession`:** assert (a)
no reseal-token file is written for any session under any path (the v1 file path
is absent), and (b) a process presenting itself as session B — or any
lane-reachable surface — cannot drive a seal of session A's in-flight job; the
only path that seals is the daemon-internal supervisor-proven reseal keyed to the
`supervisor_id`↔`session_id` row. *Refuted if* any on-disk reseal bearer exists,
or a non-supervisor caller seals a foreign session's job.

### F3 — Name the split-brain predicate (was: "current in-flight job" undefined)

**Chosen prescribed-fix option: a strict reseal-time database predicate driving a
typed refusal.** `resealInFlightJob` (new, `go/pkg/mutations/`, reusing
`activeLeaseFor` at `mutations.go:803`) seals only when ALL hold, in one
transaction:
1. **session live** — `sessions` row for `session_id` is `active` (not closed/retired);
2. **same job still owned by this session** — the job is still leased/acked by
   `session_id` (the `activeLeaseFor(repositoryID, leaseID, sessionID, jobID)`
   predicate: lease exists, owner = this session, job = this job);
3. **lease active or within reseal grace (F5)** — `leases.expires_at > now()`, or
   `now() - expires_at ≤ resealGrace` and the lease was not requeued;
4. **no recovery-generation change** — the job's recovery/lease generation column
   is unchanged since the lease was issued (the daemon has not requeued/retired or
   re-leased the job to another session);
5. **artifact path still expected** (for `artifact.publish`) — the published path
   is still an open entry in the job's `expected_artifacts`;
6. **boot epoch accepted (F7)** — the supervised path's epoch equals the live
   epoch (header required, not absent).
Any failure → typed `session_unrecoverable_across_rotation` refusal (routes to
Slice A), never a silent or raw `lease_error`.
**Refuting test — `TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue`:**
after the daemon requeues/retires the in-flight job (generation bump) or the lease
expires beyond grace, a reseal attempt is REFUSED with the typed class. *Refuted
if* an old lane publishes/completes into a requeued/retired job, or the refusal is
a raw `lease_error`.

### F4 — Specify the authority mechanism without granting plain `write` (was: `CapabilityReseal` cannot pass the single-cap prelude)

**Chosen prescribed-fix option: `MethodEntry.RequiredCapability` stays a single
`write`; a route-specific `ResealAlternate` is added; the prelude and both
authorizers project the alternate and record the selected grant — never plain
`write`.** Concretely:
- `MethodEntry` gains `ResealAlternate bool` (`registry.go:47`), set `true` only
  for `interrogation.answer` / `work.complete` / `artifact.publish`
  (`registry_methods.go:8-10`).
- The prelude (`server.go:107-111`) authorizes `RequiredCapability` (`write`)
  first; on `capability_missing` for a `ResealAlternate` route it re-authorizes
  against `CapabilityReseal`. A `write` token keeps working unchanged; a
  reseal-scoped principal is admitted *only* to those three routes.
- `MemoryAuthorizer.Authorize` (`capability.go:175-233`) and
  `PostgresAuthorizer`/`striatumd.authorize_capability` (`auth_pg.go:159-206`)
  project the reseal grant and return the *resolved* capability, so
  `AuthContext.Capability` (`capability.go:28`, set `:230`/`auth_pg.go:203`)
  records `reseal`. Handlers branch on `ctx.Capability == CapabilityReseal` to
  apply the F3 scope.
- **Generated contracts + guardrail:** `docs/reference/command-authority-matrix.md`
  gains a reseal-alternate column for exactly those three routes; the authority
  guardrail tests assert `CapabilityReseal` reaches *only* those three and
  **no** other route, and never resolves to `write`.
**Refuting test — `TestResealTokenCanReachOnlyResealRoutesWithoutWrite`:** a
reseal-scoped principal authorizes `work.complete`/`artifact.publish`/
`interrogation.answer` (in-flight job) and is REFUSED `work.claim_next`,
`repo.write`, `git.commit_apply`, recovery.*, and every non-reseal route; its
`AuthContext.Capability` is `reseal`, never `write`. *Refuted if* it reaches any
non-reseal route, or the grant resolves to `write`.

### F5 — Resolve the lease clock (was: reseal races the lease, no heartbeat authority)

**Chosen prescribed-fix option: Slice B survives only within the active lease
window (not the 24h token TTL), AND Option 4 explicitly routes
expired-lease-after-rotation as `session_unrecoverable_across_rotation`.** The
daemon-internal reseal transaction admits the seal only if the in-flight lease is
active, or within a bounded daemon-side **reseal grace** by which the daemon may
extend the *same* lease row to cover the seal round-trip (this is the daemon
acting on the supervisor-proven path, NOT a lane-invokable `work.heartbeat`, and
`CapabilityReseal` carries no heartbeat verb — consistent with the maintainer
pin's cap set). If the lease already expired beyond grace, or was requeued, the
reseal is refused with the typed class (F1 floor), never a raw `lease_error`.
**Refuting game-day — GD-1b:** restart `striatumd`, prevent reconnect until past
`leases.expires_at`, then drive a reseal. Acceptable outcomes: the daemon renews
**only that one lease** within grace and seals, **or** the typed
`session_unrecoverable_across_rotation` class fires and routes. *Refuted if* the
outcome is a raw `lease_error`, stale-lease limbo, or a silent unsealed exit.

### F6 — Name the per-adapter survival matrix (was: Codex cannot reload endpoint/epoch in place)

**Chosen prescribed-fix option: a daemon-side receiver path (the supervisor
channel) that seals without the adapter's MCP client reloading launch args, plus
an honest per-adapter matrix; no claim of in-place Codex MCP survival.** Because
the reseal travels over the **PTY/helper bridge** the daemon-owned helper reads
from the lane's stdout regardless of adapter, **no adapter needs to reload its MCP
launch args to seal the in-flight job.**

| Adapter | Reseal-in-flight-job (Slice B) | Resume normal MCP work after rotation |
| --- | --- | --- |
| **Claude** (ephemeral MCP config) | supervisor channel (no token reload) | #323 ephemeral-config rewrite + endpoint/epoch republish |
| **Agy** | supervisor channel | same as Claude where supported |
| **Codex** (MCP URL baked into launch `-c` args; `applyMCPEndpointRotation` can only log + inject a "must be relaunched" prompt) | supervisor channel (daemon-side receiver) — **no in-place MCP survival claimed** | operator-assisted relaunch / `supervise rebridge` only |

**Refuting game-day — GD-Codex-Reseal-Rotation:** restart `striatumd` mid-job for
a Codex lane; assert the in-flight job is sealed over the supervisor channel **or**
fails legibly to Option 4 — and that the spec does **not** claim the Codex MCP
client reconnected in place. *Refuted if* the spec relies on the Codex MCP client
reloading baked launch args, or the Codex lane silently exits unsealed.

### F7 — Give the epoch path an integrity mechanism (was: mirror readable but not integrity-protected; permissive header-absent #316)

**Chosen prescribed-fix option (re-shaped by the pin): the epoch lives on the
daemon-owned channel, not a lane-writable mirror; and a supervised reseal/reconnect
with a MISSING boot-epoch header is REJECTED.** Two parts:
1. **No lane-writable mirror.** The reseal path requires **no** lane-presented
   epoch — the daemon already knows the live epoch and the supervisor-proven
   session, so the #316 check for reseal is daemon-internal. The minimal Option-3
   endpoint+epoch republish (for a lane resuming *normal* MCP work) is delivered
   over the daemon-owned helper channel, or written by the helper (daemon uid) to
   a **lane-read-ONLY** file that is **owned by the daemon user, mode `0644`, in a
   daemon-owned directory outside the lane-writable `.striatum/scratch` ACL**
   (`scratchACLTargets` grants `u:<lane>:rwx` — explicitly NOT used here), with
   **`O_NOFOLLOW` symlink defense** and **atomic temp-file + rename** replace.
   Sibling lanes cannot tamper (no write to the daemon-owned dir).
2. **Close the permissive header-absent #316 path for the supervised path.** A
   supervised reconnect or reseal that presents a **MISSING** boot-epoch header is
   **REJECTED** (`main.go:697-719` currently returns `nil` when the header is
   absent — only a non-empty mismatch is rejected). The supervised path requires a
   present epoch header equal to the live epoch; the general non-supervised path
   may keep its current behavior to avoid breaking other clients.
**Refuting test — `TestResealEpochMirrorRejectsTamperOrMissingEpoch`:** assert
(a) any lane-readable epoch artifact is daemon-owned, not lane-writable, and a
symlink/replace attack fails; (b) a supervised reseal/reconnect with a missing or
mismatched epoch header is rejected, not silently accepted. *Refuted if* the lane
can write/replace the epoch source, or a missing-header supervised request is
accepted.

## Security invariant (the spine) — held explicitly

The runtime `client-token` carries the full `bootstrapCapabilities` set and is
`0600` in a `0700` dir (AF3; `bootstrap.go:18-27`, `:139-142`). **Any option that
lets a lane read that file, or that mints a lane-readable credential carrying ANY
of `{admin, apply, recovery, surgical_recovery}`, is categorically out of
bounds.** This spec makes that **structurally impossible**, and the revision makes
it *stronger* than v1:

- The lane is never granted OS read of the `0700` runtime dir (AF3); the Slice-A
  floor (OQ4) removes the only code path that would have read the `client-token`.
- The only new authority, `CapabilityReseal`, carries **no elevated verb to
  grant** and is **never materialized into any lane-readable file** (F2). There is
  no bearer for a lane to read, steal, or replay — strictly safer than v1's
  `0600` file.
- The reseal is projected by the daemon only on the supervisor-proven channel; a
  lane cannot present `CapabilityReseal` itself, let alone admin/apply/recovery.
- The epoch republish moves **endpoint + epoch only** (non-secret anti-confusion
  tags) over a daemon-owned, integrity-protected channel (F7); never the admin
  token, never any credential the lane did not already hold for its own session.

## Falsifiable assertions (each with the named test / game-day that refutes it)

> Every assertion is stated so a single named test/game-day, if it fails,
> *refutes the claim*. A1–A8 are the v1 assertions (kept, re-anchored); A9–A13 add
> coverage for the revision. The falsifiers should attack these directly.

- **A1 — No-widening (R1).** `CapabilityReseal` carries only the three reseal
  verbs. *Refuted if* `TestResealTokenCanReachOnlyResealRoutesWithoutWrite` (F4)
  shows it authorizing any of `admin`/`apply`/`recovery`/`surgical_recovery`,
  `work.claim_next`, or any non-reseal route, **or** resolving to `write`.
- **A2 — No admin-token fall-through (the named bug, R1).**
  `ResolveTokenMaterial` / `ResolveTokenMaterialFresh` never return the runtime
  `client-token` for a supervised lane. *Refuted if*
  `TestResolveRefusesRuntimeClientTokenForLane` returns the admin token instead of
  `ErrSessionUnrecoverableAcrossRotation`.
- **A3 — No-replay (R2), now structural.** There is no lane-readable reseal bearer
  to replay. *Refuted if* `TestBorrowedResealBearerCannotSealVictimSession` (F2)
  finds any on-disk reseal bearer or a sibling/foreign-session caller sealing
  session A's job.
- **A4 — No split-brain (R3).** A reseal seals only the in-flight job the daemon
  still recognizes. *Refuted if*
  `TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue` (F3) shows a reseal
  succeeding after requeue/retire, or job-X reseal publishing into job Y.
- **A5 — Loud, durable failure (R4).** The Option-4 path records a durable
  `session_unrecoverable_across_rotation` blocker the run routes. *Refuted if*
  game-day **GD-1** (F1) shows a silent unsealed exit, a raw "permission denied",
  a local-only process error, or no durable daemon event.
- **A6 — Epoch path does not weaken #316.** A lane adopts a fresh epoch only over
  the daemon-owned channel; a recycled-port daemon is still rejected, and a
  missing-header supervised request is rejected. *Refuted if* game-day **GD-2**
  (bind a *different* daemon to the recycled port after the lane launched) shows
  the lane resealing into the wrong daemon, **or**
  `TestResealEpochMirrorRejectsTamperOrMissingEpoch` (F7) shows a tamper/missing
  epoch accepted.
- **A7 — Lease-window bound (F5).** Slice B survives only within the active lease
  window (+ bounded grace). *Refuted if* **GD-1b** yields a raw `lease_error` or
  stale-lease limbo instead of a same-lease renew-and-seal or the typed class.
- **A8 — Token validity survives the restart (AF1).** A session-bound token minted
  before the rotation still authorizes against the restarted daemon (same PG, same
  session row, TTL unexpired). *Refuted if* `TestTokenValidAcrossRestart` shows
  the PG-resident token rejected purely because the process restarted.
- **A9 — Capability resolves to reseal, not write (F4).** A reseal-scoped seal
  records `AuthContext.Capability == reseal`. *Refuted if* the audit/AuthContext
  records `write` for a reseal-path seal (`TestResealTokenCanReachOnlyResealRoutesWithoutWrite`).
- **A10 — Floor routes via the supervisor channel, not an MCP verb (F1).** The
  no-token floor produces its durable event without the lane calling
  `work.block`/`session.report`. *Refuted if* GD-1 requires the lane to hold
  `CapabilityWrite`/`CapabilityClaim` to escalate.
- **A11 — Codex sealed over the receiver path, no in-place MCP reload (F6).**
  *Refuted if* GD-Codex-Reseal-Rotation shows the Codex MCP client reconnecting in
  place from reloaded launch args, or a silent unsealed Codex exit.
- **A12 — Reseal predicate is a concrete query (F3).** The reseal refusal is
  driven by the named session/lease/generation/expected-artifact/epoch predicate.
  *Refuted if* `TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue` shows a
  reseal admitted on bearer/identity alone without the predicate.
- **A13 — Epoch source is daemon-owned and integrity-protected (F7).** *Refuted
  if* `TestResealEpochMirrorRejectsTamperOrMissingEpoch` shows a lane-writable
  epoch source, a successful symlink/replace attack, or a missing-header
  supervised request accepted.

## Scope discipline (Non-Goals held)

- Does **not** re-classify the downstream `agent_exited_unsealed` recovery policy
  (RFC 0152 / D249) — that governs what happens *after* an unsealed exit; this
  governs whether the exit happens silently. The new
  `session_unrecoverable_across_rotation` blocker is a *distinct, earlier* class.
- Does **not** change the committee POSIX-ACL repo provisioning (#537 / #539) —
  target-repository filesystem ACL, not a daemon-private credential.
- Does **not** touch `run drive`'s transient-socket behavior (#513).
- Does **not** weaken the #316 boot-epoch recycled-port defense (A6/F7 strengthen
  it on the supervised path).
- Does **not** introduce any lane-readable credential file (the v1 `0600` reseal
  file is explicitly retired by the maintainer pin).
- Local-first, single-host, daemon-owned PostgreSQL as the single writer.

## Maintainer ratification gate (required)

**Slice B introduces a new capability class (`rpc.CapabilityReseal`), a new
auth-prelude route alternate, a daemon-internal supervisor-mediated reseal path,
and new endpoint/epoch republish plumbing — a security/authz trust-model change.**
This cleared spec is a **RECOMMENDATION the maintainer ratifies before any build
slice touches credential code.** Slice A (Option 4, the legible-failure floor) is
zero-trust-change and may land first under the normal review gate **once F1 makes
it actually routed**; **Slice B does not land until the maintainer ratifies the
new capability class.** Falsifier clearance of this spec is **not** maintainer
ratification — the adjudicator's collaboration ledger gates the dialogue; the
maintainer gates the credential code. (The maintainer has already ratified the
OQ1 shape and the F2 non-bearer decision in `SEED.md`; this gate governs the build
slice that writes the code.)

---
<sub>Holder revised proposal (design-v2) for the RFC 0143 falsification-gate
design run. The adjudicator's collaboration ledger — not falsifier completion —
decides whether this gate clears.</sub>
