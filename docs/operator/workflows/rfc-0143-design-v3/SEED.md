# Design-Run Seed (v3 / REVISION) — RFC 0143 Lane credential survival across a daemon boot-epoch rotation

> **THIS IS THE THIRD REVISION (v3).** Two prior design runs ran the same
> falsification gate on this RFC. v1 (`rfc-0143-design`) returned `needs_revision`
> with seven findings F1–F7. v2 (`rfc-0143-design-v2`) **resolved F2 and F4
> cleanly** (both falsifiers conceded the bearer-file retirement and the
> route-specific capability wiring), but returned `needs_revision` **again**
> because two material challenges landed unrebutted and four findings were only
> *nominally* closed. This v3 run is a **proper revision**: the holder starts from
> the **v2** `HOLDER.md` (a required context doc), REVISES the spec to **resolve
> the five binding constraints BC1–BC5** distilled below, and **carries the
> v2-resolved set forward unregressed**; the falsifiers re-attack the revised spec.
> The v2 design record — `dialogue/holder/HOLDER.md`,
> `dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`, and
> `dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — lives under
> `docs/operator/artifacts/rfc-0143-design-v2/`; the **v2** `HOLDER.md` (the spec
> being revised) and the **v2** collaboration ledger (the verdict + the full
> BC1–BC5 analysis and prescribed fixes) are wired in as required `context_docs`.
>
> This document is the **required input** for the RFC 0143 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed at
> `docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md` (status
> `proposed`) — read it in full as your primary source; this SEED carries the
> charter, **pins the ratified design shape (do not relitigate)**, states **what
> already cleared by v2 (carry forward, do not reopen)**, and lists the **five
> binding constraints v3 MUST resolve**, each anchored to exact source sites. Read
> this whole file, the **v2** `HOLDER.md` + the **v2** collaboration ledger, and
> the RFC before producing any artifact.

## Framing — what this run must produce

This is a **design run**, not an implementation run. RFC 0143 is the security/authz
problem of **lane credential survival across a daemon boot-epoch rotation**: when a
lane loses its live RPC connection across a daemon restart, its
credential-resolution chain falls through to the full-authority bootstrap admin
`client-token` (which a `striatum-lane` lane cannot read), so a complete-on-disk
deliverable exits unsealed with a misleading permission error. The deliverable of
this run is a **falsifiable implementation spec** the `rfc-0143-build` run can
execute contract-first (TDD), produced by hardening the v2 spec against adversarial
falsification.

The v2 falsification gate found the v2 design **`needs_revision`**. The holder must
produce a proposal that **resolves the five binding constraints BC1–BC5 below**
while **carrying the v2-resolved set forward unregressed**. A revised spec that
leaves any BC open — or regresses any carried-forward item — has NOT cleared the
gate. This is the gate's single allowed revision cycle for v3, so a second
`needs_revision` ends the gate uncleared. All five constraints must land in **one
coherent proposal**: the **security cluster is BC1 + BC2 + BC3**, the **lifecycle
cluster is BC4 + BC5**.

## Ratified design shape (do NOT relitigate)

The maintainer has ratified the trust-model shape and the F2 replay defense; these
are binding and **override any softer framing**. The v2 gate did not contest them —
do not reopen them, build on them:

- **OQ1 — trust-model shape (ratified): Option 4 + ratification-gated Option 2 +
  minimal Option 3.**
  - **Slice A (mandatory, lands first, ZERO trust-model change):** Option 4 — a
    legible, self-escalating `session_unrecoverable_across_rotation` signal
    replacing the silent unsealed exit. This is the floor; it must be buildable and
    valuable on its own.
  - **Slice B (ratification-gated):** Option 2's *narrow* reseal authority — a
    session-scoped `CapabilityReseal` covering ONLY the in-flight job's seal
    (`work.complete` / `artifact.publish` / `interrogation.answer`), **never** any
    of `{admin, apply, recovery, surgical_recovery}` and **never plain `write`** —
    folding in a minimal Option 3 per-session endpoint+epoch republish so the lane
    never needs to read the admin `client-token`.
- **F2 — replay defense (DECIDED): non-bearer, daemon-owned, session-tied channel.
  NO readable reseal token file.** Because all lanes currently share the
  `striatum-lane` OS user, a `0600` reseal *file* is a same-uid replay surface
  readable by sibling lanes. The ratified resolution: deliver/verify the
  `CapabilityReseal` authority over the **daemon-owned supervisor/PTY session-tied
  channel** — there is NO lane-readable reseal token file at all. The daemon proves
  the calling session, not a bearer file. Do NOT reintroduce a readable bearer file
  as the reseal credential under any option.
- **Slice B requires maintainer ratification before any build slice touches
  credential code.** Adjudicator clearance gates the spec's *soundness*, not the
  maintainer's product call. Slice A is zero-trust-change and may land first under
  the normal review gate **once BC1 makes it routed over a real, non-PTY-output
  channel.**

## Already cleared by v2 — carry forward unregressed (do NOT reopen)

> The v2 collaboration ledger records the following as genuinely resolved / sound;
> **both v2 falsifiers conceded F2 and F4.** The v3 revision MUST preserve them —
> verbatim from the **v2** `HOLDER.md` where applicable — and the cycle-3
> adjudicator's clearing verdict requires them intact. Re-opening any of these is a
> regression that fails the gate.

- **F2 — RESOLVED (bearer-file retirement).** The v1 `0600` lane-readable reseal
  bearer file is retired (maintainer pin); the reseal authority is carried over a
  **non-bearer, daemon-owned, session-tied channel**, so there is no on-disk bearer
  for a sibling `striatum-lane` process to read and replay AS A. The *residual*
  replay/false-provenance question migrates onto the channel — that residual is
  **BC1**, not a reopening of F2.
- **F4 — RESOLVED (auth mechanism without plain `write`).** A route-specific
  `MethodEntry.ResealAlternate` admits `CapabilityReseal` on **only**
  `interrogation.answer` / `work.complete` / `artifact.publish`; both authorizers
  project the alternate and record `AuthContext.Capability == reseal` (**never
  `write`**); the `command-authority-matrix` gains a reseal-alternate column and the
  guardrail test pins "reseal reaches only those three routes." Preserve this
  wiring exactly.
- **F7 file-mirror half — RESOLVED.** Endpoint/epoch moves off lane-writable
  `.striatum/scratch` to a **daemon-owned, lane-read-only `0644` file** with
  `O_NOFOLLOW` symlink defense and atomic temp-and-rename, and a supervised request
  with a **MISSING** boot-epoch header is **rejected** — closing the permissive
  header-absent #316 path on the supervised path. Keep this and
  `TestResealEpochMirrorRejectsTamperOrMissingEpoch`. (The *channel*-integrity half
  of F7 inherits BC1.)
- **AF1 — reachability-not-reminting.** The session-bound token stays *valid*
  across a restart; only its *reachability* breaks. The fix is **routing**, not
  re-minting. Keep `TestTokenValidAcrossRestart` as a real falsifier.
- **AF4 — epoch/token decoupling.** Endpoint rotation and boot-epoch rotation are
  coupled; #316 deliberately retires a surviving lane's connection. The token does
  NOT rotate on a normal restart (only the endpoint does). Preserve this framing.
- **The categorical no-admin-token-widening invariant.** No lane ever reads the
  daemon's full-authority bootstrap admin `client-token`
  (`go/pkg/admin/bootstrap.go:18-27` grants
  `{admin,read,write,claim,review,apply,recovery,surgical_recovery}`); no minted
  credential carries any of `{admin, apply, recovery, surgical_recovery}`. The
  revision *strengthens* this by never materializing `CapabilityReseal` into any
  lane-readable file.
- **The per-claim falsifiable-assertion discipline.** Every load-bearing claim is
  paired with the named test / game-day that refutes it. Extend it to cover the
  channel and generation mechanisms; do not abandon it.

## The 5 binding constraints v3 MUST resolve (all in one place to clear)

> The v2 gate distilled the two standing material challenges + the four nominally
> closed findings into exactly two repairs the next revision must make "in one
> place" (v2 ledger §"What the next revision MUST fix"). This SEED expands those
> two repairs into five precise, falsifiable constraints. Each names the exact
> source sites; anchor every load-bearing claim in the revised spec to them.

### BC1 — Authenticated control channel, NOT parsed PTY output (closes F1 / F6 / the channel half of F2 / F7-channel)

**The gap.** The supervisor channel the v2 spec leans on for the Slice-A floor AND
the Slice-B reseal request currently routes through **parsed PTY output** — a
product-boundary breach and a spoofable false-provenance surface. In current
source the helper carries **lifecycle/byte-count metadata only**, with agent output
bytes kept OUT of the control channel (`go/pkg/supervisor/helper_protocol.go:41-44`);
`RunHelper` **moves bytes only** — it does not inspect workflow state, publish
artifacts, complete jobs, or ack work (`helper.go:120-127`); `pumpPTYProgress`
watches output **VOLUME not content** per D028 (`helper.go:357-415`); and the
accepted helper-event whitelist has **no reseal/unrecoverable event**
(`go/pkg/mutations/supervision.go:19-28`, `:217-234`, `:298-306`, `:424-425`). The
product boundary forbids advancing state by printing phrases
(`docs/how-to/how-to-agent.md:292-297`, `:366-378`; `AGENTS.md:43-56`). So a
"structured line on the PTY/helper bridge" is either (a) ordinary terminal output —
a breach, and spoofable (the provider CLI, a shell command during local
verification, or prompt-injected model text can print the same sentinel to drive a
publish/complete or record a blocker on the in-flight job, i.e. false provenance);
or (b) a NEW private wrapper-to-helper control envelope the v2 spec never specifies.

**v3 must EITHER:**
- **(a)** name a **private authenticated agentloop/wrapper → helper-or-daemon
  control channel** that provider `stdout`/`stderr` **CANNOT** write to — specifying
  the **descriptor/FIFO, ownership, message schema, framing, replay protection, and
  parser boundary** — so the helper records a reseal/unrecoverable control event
  that no PTY byte can forge; OR
- **(b)** narrow Slice-A's floor to a **reserved agentloop process EXIT CODE** the
  helper records **WITHOUT parsing output bytes** (the typed-exit-code half is
  already a sound trusted-wrapper signal; the spec must commit the helper to
  recording it without inspecting child output).

**New tests:** `TestPTYOutputCannotEmitSupervisorControlEvent`,
`TestProviderOutputCannotDriveResealOrBlocker`,
`TestAgentLoopReservedExitCodeRoutesUnrecoverableWithoutPTYParsing`.

### BC2 — Artifact identity from daemon state, not from output (security cluster)

**The gap.** Even with an authenticated control channel (BC1), the daemon must not
trust terminal output for **what artifact** is being sealed. For Slice-B reseal
intent, the daemon must **DERIVE the expected-artifact set from its own state** —
the job's `expected_artifacts` — and **REFUSE any unexpected path**; it must never
read artifact identity or content from provider terminal output.

**v3 must** specify that reseal publish/complete derives the expected-artifact set
from the job's `expected_artifacts` (daemon state) and refuses unexpected paths, and
reuse the existing handler payload contracts:
- `artifact.publish` needs `session_id` / `job_id` / `lease_id` / `kind` /
  `logical_name` / `path` (`go/pkg/mutations/artifact.go:52-60`, `:150-170`);
- `work.complete` needs `session_id` / `job_id` / `lease_id`
  (`lifecycle.go:1124-1129`);
- `interrogation.answer` needs `session_id` / `interrogation_id` / `body`
  (`interrogation.go:217-221`).

**New test:** `TestCodexResealUsesReceiverNotProviderStdout`.

### BC3 — Principal model: `CapabilityReseal` is a daemon-internal marker, not a public bearer capability (closes NEW cycle-2 finding C2)

**The gap.** `CapabilityReseal` is conflated as BOTH a public bearer-auth capability
AND a daemon-internal projection. The public prelude authorizes
`envelope.CapabilityToken` before dispatch (`server.go:107-111`), and
`PostgresAuthorizer` resolves only a **token-backed** decision
(`auth_pg.go:159-206`) — so with **no reseal bearer** (the F2 pin retired the file),
a public alternate route is **unreachable**. A capability with no bearer cannot pass
a prelude that demands a bearer token.

**v3 must** DECLARE `CapabilityReseal` a **daemon-internal capability marker**
projected by a **private `resealInFlightJob` mutation** that:
- maps `supervisor_id` → `session_id`, constructs an **internal** `AuthContext`,
  and calls the same lower-level publish/complete routines against the active
  worktree (so the authority is daemon-projected, never bearer-presented);
- keeps the **public route-alternate** (`registry_methods.go:8-10`) **for tests
  only** (the F4 guardrail that proves reseal reaches only the three routes);
- defines the **reseal payload schema** + the **validation/reuse path** for
  `publish` / `complete` / `answer`, including how an author-line / front-matter
  validation failure routes back to the Option-4 floor (so a malformed reseal
  surfaces as `session_unrecoverable_across_rotation`, never a silent drop).

### BC4 — Concrete generation column for the split-brain guard (closes F3)

**The gap.** The v2 predicate's "no recovery-generation change" guard **names no
storage**: `jobs` has `current_lease_id` but no generation
(`go/pkg/db/sql/0005_repo_local_workflow_state.sql:75-104`, `:166-179`);
`job_recovery_state` holds requeue/transfer/respawn counters — a recovery **budget**,
not a lease-issued generation (`0020_job_recovery_state.sql:13-28`);
`review_generation` is the **verdict epoch**, not a job/lease epoch
(`owner/0009_review_generation.sql`); and `activeLeaseFor` does **no generation
check** (`mutations.go:803-820`). An asserted generation guard with no column, no
increment point, and no stamped value is the "resolved-without-a-mechanism" pattern
the gate rejects.

**v3 must** name a **concrete monotonic recovery/lease generation column** WITH:
- its **migration / owner-bundle location** (where the column is added);
- the exact **increment points** (which claim / requeue / release / recovery paths
  advance it); and
- the **value stamped** into the lease / work-packet for reseal-time comparison
  (so `resealInFlightJob` compares the stamped generation against the current row and
  refuses on any change).

**New test:** `TestResealPredicateUsesStampedRecoveryGeneration` (keep
`TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue`).

### BC5 — Numeric grace + exact lock order vs the recovery race (closes F5)

**The gap.** The v2 "bounded reseal grace" has **no number, source, or maximum**,
and `activeLeaseFor` returns a **raw `lease_error`** on expiry with no generation
check (`mutations.go:803-820`). There is **no lock order** vs the recovery sweep,
which **drains helper events then expires/requeues in one pass**
(`go/pkg/mutations/recovery.go:575-587`, `:619-623`, `:866-890`), while the normal
seal paths take `lockRunForJob` + row locks (`artifact.go:75-85`,
`lifecycle.go:1135-1180`). The common post-rotation race therefore resolves to
either the forbidden raw `lease_error` (violates F5) or a lease revived after
recovery requeued the job (split-brain, violates F3).

**v3 must:**
- define **`resealGrace` numerically** — its **source** and its **maximum** (e.g. a
  short daemon constant or a function of the packet heartbeat window), with **one
  same-lease extension only** before a generation change forecloses it;
- specify the **EXACT lock order** for `resealInFlightJob` vs `artifact.publish` /
  `work.complete` (`lockRunForJob`) and the **recovery sweep** (the same per-run
  lock + `FOR UPDATE` on the job / lease / recovery-state rows, so reseal and the
  sweep cannot interleave);
- ensure **expired-beyond-grace ALWAYS routes the typed
  `session_unrecoverable_across_rotation` class** — NEVER a raw `lease_error` and
  NEVER a lease revived after the sweep requeued the job.

**New tests:** `TestResealBeyondGraceRoutesTypedNotLeaseError`,
`TestResealGraceCannotReviveRequeuedLease`,
`TestRecoveryRequeueWinsOverExpiredLeaseReseal` (keep `GD-1b`).

## Clearing condition for this revision

The adjudicator clears the gate only if **all five binding constraints BC1–BC5 are
genuinely resolved** with a concrete mechanism and named tests, **the v2-credited
resolved set is carried forward unregressed** (F2, F4, the F7 file-mirror half, AF1,
AF4, the no-admin-token-widening invariant, the falsifiable-assertion discipline),
and **no new material challenge** stands unrebutted. The security cluster is
BC1 + BC2 + BC3; the lifecycle cluster is BC4 + BC5 — all five must land in one
coherent proposal. The verdict is `reject` only if a path widens admin-token
exposure or mints a credential carrying any of
`{admin, apply, recovery, surgical_recovery}`; otherwise `needs_revision` if any BC
remains open. One revision cycle is available within this run; the falsifiers
re-attack the revised spec. **Slice B still requires maintainer ratification before
any build slice touches credential code** — adjudicator clearance gates the spec's
soundness, not the maintainer's product call.

---
<sub>Operator scaffold for the RFC 0143 falsification-gate design run (v3 / REVISION
of `rfc-0143-design-v2`; resolves the cycle-2 binding constraints BC1–BC5 — the
security cluster BC1/BC2/BC3 and the lifecycle cluster BC4/BC5 — and carries the
v2-resolved set forward). Lanes: author=claude (holder/adjudicator/committer),
reviewer=codex (falsifiers).</sub>
