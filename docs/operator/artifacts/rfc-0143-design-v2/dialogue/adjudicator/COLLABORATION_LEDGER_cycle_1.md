---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0143 lane credential survival across a daemon boot-epoch rotation — falsifiable implementation spec (design-v2 REVISION)"
participants:
  - "holder-author-002"
  - "falsifier-reviewer-001"
  - "falsifier-reviewer-002"
  - "adjudicator-author-001"
cycle: 1
entries:
  - kind: claim
    by: "holder-author-002"
    refs: ["dialogue:1"]
    text: "Revised spec claims it resolves all seven v1 findings F1-F7 while holding the RFC 0096/#135/#296 trust model and building on the credited v1 strengths. Land Option 4 as the mandatory zero-trust-change floor (a typed agent-loop exit code plus a structured helper line recorded as a durable session_unrecoverable_across_rotation blocker via the daemon-owned helper, not work.block/session.report). Land a new rpc.CapabilityReseal authorizing ONLY work.complete, artifact.publish, interrogation.answer for the session's own in-flight job, carried NOT as a lane-readable bearer file but over the daemon-owned supervisor/PTY session-tied channel (maintainer pin retires the v1 0600 file, dissolving F2). Add a route-specific MethodEntry.ResealAlternate so the auth prelude admits reseal without granting plain write and records AuthContext.Capability == reseal (F4). Bound survival to the active lease window plus a bounded daemon-side reseal grace (F5). Name an in-flight predicate resealInFlightJob reusing activeLeaseFor with a no-recovery-generation-change guard (F3). Seal Codex over the receiver path with an honest no-in-place-MCP-survival matrix (F6). Move endpoint/epoch off lane-writable scratch to a daemon-owned lane-read-only file and reject missing-epoch supervised requests (F7). Asserts the security invariant holds structurally and is stronger than v1 because there is no bearer to steal."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "PTY output is not an authenticated reseal channel. The revision routes the F1 no-token floor AND the Slice-B reseal request over a daemon-owned supervisor/PTY channel, claiming the helper parses a structured line into a control event and that the helper reads lane stdout regardless of adapter (the F6 Codex receiver). But current source limits the helper-to-daemon control stream to lifecycle metadata and byte counts only, with agent output bytes kept OUT of the control channel (helper_protocol.go:41-44); RunHelper deliberately does not inspect workflow state, publish artifacts, complete jobs, or ack work (helper.go:120-127); pumpPTYProgress watches output VOLUME not content per D028 (helper.go:357-415); and the product boundary (how-to-agent.md, AGENTS.md) holds that terminal output is not authoritative workflow state and a lane must not advance state by printing phrases. So the structured line is either (a) ordinary terminal output: a product-boundary breach, and spoofable, because any process whose bytes reach the lane PTY (the provider CLI, a shell command during local verification, or prompt-injected model text) can print the same sentinel to drive a publish/complete or record a blocker on the in-flight job, i.e. false provenance; or (b) a NEW private wrapper-to-helper control envelope the revision never specifies: no named pipe, reserved exit-code value, message schema, framing, replay protection, or parser boundary. Helper process identity proves the helper belongs to the session; it does not authenticate the PTY bytes. F1 and F6, plus the channel half of F2/F5/F7, remain open as one security/authz gap; the typed exit code alone is a safe floor only if the helper records it WITHOUT parsing output bytes, which the spec does not commit to."
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "The reseal lease/grace/recovery-generation repair is not a race-free, concrete contract, so F3 and F5 are not genuinely closed. 'No recovery-generation change' names no storage: jobs has current_lease_id but no generation, leases has no generation column, job_recovery_state holds requeue/transfer/respawn counters (a recovery budget, not a lease-issued generation), and review_generation is the verdict-coherence epoch, not a job/lease epoch. 'Bounded reseal grace' has no number, source, or maximum, so it is not a falsifiable lifecycle contract. resealInFlightJob is said to reuse activeLeaseFor, which still returns a raw lease_error on an expired lease and performs no generation check (mutations.go:803-820). The common post-rotation case is therefore an unresolved race: the lane cannot heartbeat, leases.expires_at passes while the lane is still within the unspecified grace, and the recovery sweep drains helper events then expires/requeues in the same pass (recovery.go). Reuse yields the forbidden raw lease_error (violates F5), or a bearer/grace bypass revives a lease after recovery requeued/retired the job (split-brain, violates F3). The spec gestures at one transaction but states no serialization point or lock order relative to publish/complete (lockRunForJob) and the recovery sweep, so GD-1b cannot distinguish a safe same-lease renew-and-seal from whichever side of a race won."
verdict: "needs_revision"
rationale: "This adjudicates the design-v2 REVISION (the second falsification pass on RFC 0143). The revision is materially stronger than v1: it categorically holds the no-admin-token-widening invariant, RETIRES the v1 0600 lane-readable reseal bearer file (closing the same-uid FILE replay surface, F2), and resolves the OR-capability auth-prelude finding with a route-specific ResealAlternate that records AuthContext.Capability == reseal and never grants plain write (F4). Both falsifiers independently credit F2 (file) and F4 as genuinely resolved. But the gate does NOT clear: two new material challenges stand unrebutted, and several v1 findings are only nominally closed. falsifier-reviewer-001 shows the daemon-owned supervisor channel the revision leans on for F1 and F6 (and the channel half of F2/F5/F7) is, in current source, either parsed provider PTY output (a product-boundary breach the rules forbid, and a spoofable false-provenance surface that re-opens replay risk) or an unspecified private wrapper-to-helper control envelope; the helper carries only lifecycle/byte-count metadata and explicitly does not parse child output into control events. falsifier-reviewer-002 shows the F3/F5 lifecycle repair is conceptual, not concrete: no named recovery/lease generation column or increment protocol, no numeric reseal-grace bound, and no lock order, so the common post-rotation lease-expiry race resolves to either the forbidden raw lease_error (F5) or a lease revived after recovery requeued the job (split-brain, F3). I verified both falsifiers' load-bearing source citations against current main (helper_protocol.go:41-44 control-stream contract, helper.go:120-127 RunHelper non-authority contract, helper.go:357-415 D028 volume-only meter, activeLeaseFor mutations.go:803-820, and the absence of any lease/recovery generation column in 0005_repo_local_workflow_state.sql / 0020_job_recovery_state.sql); all citations are accurate. Clearing requires ALL THREE of: all seven F1-F7 resolved with a concrete mechanism (FAILS - F1, F3, F5, F6 are not genuinely closed, F7 channel-dependent; only F2 and F4 clear); no new material challenge standing unrebutted (FAILS - both C1 challenges land unrebutted); and the security invariant held structurally (admin-token non-widening HOLDS and is strengthened, but no-replay and no-split-brain do NOT hold structurally given the spoofable channel and the unresolved reseal/recovery race). No path widens admin-token exposure and no minted credential carries any of {admin,apply,recovery,surgical_recovery}; both falsifiers say the shape is salvageable and each supplies a concrete repair; so this is needs_revision, not reject. A clearing future revision must, in one place: (1) name a private authenticated agentloop/wrapper-to-helper (or to-daemon) control channel that provider stdout/stderr cannot write to, OR narrow F1 to a reserved agentloop process exit code the helper records WITHOUT parsing output bytes, and specify how the daemon receives publish/complete intent and artifact identity without trusting provider terminal output (deriving the expected-artifact set from daemon state and refusing unexpected paths); and (2) pin a concrete monotonic recovery/lease generation column with its increment points and stamped value, a numeric resealGrace with its source and maximum, and the exact lock order for resealInFlightJob relative to artifact.publish, work.complete (lockRunForJob) and the recovery sweep. Preserve the credited strengths: AF1 reachability-not-reminting, AF4 epoch/token decoupling, the categorical no-widening refusal, the F4 capability wiring, the F2 bearer-file retirement, the maintainer-ratification gate, and the per-claim falsifiable-assertion discipline. Even when this clears on a future revision, the chosen direction (a new rpc.CapabilityReseal class, a supervisor-mediated reseal path, and endpoint/epoch republish plumbing) is a security/authz trust-model change requiring maintainer ratification before any build slice touches credential code; adjudicator clearance gates the spec's soundness, not the maintainer's product call."
findings:
  - id: F1
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:2"]
    affected_invariants:
      - "R4 silent-failure regression (option 4 must be loud and routed)"
      - "R2 replay / false-provenance"
      - "product-boundary: terminal output is not authoritative workflow state"
    challenge: "Option-4 floor routing is only NOMINALLY closed. The typed agent-loop EXIT CODE is a sound trusted-wrapper signal, but the durable blocker and the reseal request also depend on a 'structured line on the PTY/helper bridge' the helper forwards. Current source: the helper control stream carries lifecycle/byte-count metadata only with agent output kept out of it (helper_protocol.go:41-44); RunHelper does not parse child output or touch workflow state (helper.go:120-127); pumpPTYProgress watches volume not content per D028 (helper.go:357-415). The revision does not name a private authenticated wrapper-to-helper control envelope (reserved exit-code value, message schema, framing, replay protection, parser boundary) distinct from provider stdout, so the floor is either a product-boundary breach or unspecified. Fix: name that non-PTY authenticated control channel, or narrow F1 to a reserved agentloop exit code the helper records WITHOUT parsing output bytes; add TestPTYOutputCannotEmitSupervisorControlEvent and TestAgentLoopReservedExitCodeRoutesUnrecoverableWithoutPTYParsing alongside GD-1."
  - id: F2
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:2"]
    challenge: "RESOLVED on the same-uid FILE replay: the v1 0600 lane-readable reseal bearer is retired (maintainer pin), so there is no on-disk bearer for a sibling striatum-lane process to read and replay AS A. Both falsifiers credit this as the right structural fix and strictly safer than v1. RESIDUAL: the replay / false-provenance question MIGRATES onto the supervisor channel - removing the file does not by itself authenticate the request bytes. That residual is tracked under F1 (channel authentication), not re-opened here; the bearer-file finding itself is accepted as resolved."
  - id: F3
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:3"]
    affected_invariants:
      - "R3 split-brain across rotation"
    challenge: "Split-brain predicate is not genuinely closed: the revised resealInFlightJob lists the right ingredients but 'no recovery-generation change' names no storage. jobs has current_lease_id but no generation; leases has no generation column; job_recovery_state holds requeue/transfer/respawn counters, not a lease-issued generation; review_generation is verdict-coherence, not a job/lease epoch. Fix: name a concrete monotonic generation column (with migration / owner-bundle location) or prove an existing field monotonic for every claim/requeue/release path; state the increment points and the value stamped into the lease/work-packet for reseal-time comparison; keep TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue and add TestResealPredicateUsesStampedRecoveryGeneration."
  - id: F4
    severity: high
    posture: design
    status: accepted
    source_refs: ["dialogue:2"]
    challenge: "RESOLVED at design level: the auth-prelude OR-capability finding is answered concretely. MethodEntry.RequiredCapability stays write; a route-specific ResealAlternate admits CapabilityReseal on only interrogation.answer / work.complete / artifact.publish; both authorizers project the alternate and record AuthContext.Capability == reseal (never write); the command-authority-matrix gains a reseal-alternate column and the guardrail test asserts CapabilityReseal reaches only those three routes. Both falsifiers credit this as a concrete answer that does not collapse to plain write."
  - id: F5
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:3"]
    affected_invariants:
      - "R3 split-brain across rotation"
      - "R4 legible-failure (typed class must fire, never a raw lease_error)"
    challenge: "Lease-clock repair is not genuinely closed: 'active lease window plus bounded daemon-side reseal grace' defines no grace number, source, or maximum, and resealInFlightJob is said to reuse activeLeaseFor, which returns a raw lease_error on expiry with no generation check (mutations.go:803-820). No lock/serialization protocol prevents the recovery sweep from requeueing the job while reseal revives the old lease. Fix: define resealGrace numerically with its source and maximum; specify the exact lock order for resealInFlightJob vs artifact.publish, work.complete (lockRunForJob) and the recovery sweep; ensure the expired-beyond-grace case routes the typed session_unrecoverable_across_rotation class, never a raw lease_error. Add TestResealBeyondGraceRoutesTypedNotLeaseError, TestResealGraceCannotReviveRequeuedLease, TestRecoveryRequeueWinsOverExpiredLeaseReseal; keep GD-1b."
  - id: F6
    severity: medium
    posture: design
    status: open
    source_refs: ["dialogue:2"]
    challenge: "Codex matrix is honestly de-scoped (no in-place MCP survival claimed) and that honesty is credited, but the substitute 'daemon-side receiver' is specified as the helper reading lane stdout regardless of adapter - the same parsed-PTY-output path challenged in F1. No adapter-independent receiver exists in named source (current Codex rotation only logs and injects a relaunch prompt, loop.go:625-646). Fix: specify the receiver as the F1 authenticated control channel (not provider stdout), and how it derives the expected-artifact set from daemon state and refuses unexpected paths; add TestCodexResealUsesReceiverNotProviderStdout; keep GD-Codex-Reseal-Rotation."
  - id: F7
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:2"]
    affected_invariants:
      - "#316 boot-epoch recycled-port defense (must not be weakened)"
    challenge: "Two halves. The FILE/mirror-integrity half is concretely resolved and credited: endpoint/epoch moves off lane-writable .striatum/scratch to a daemon-owned, lane-read-only 0644 file with O_NOFOLLOW symlink defense and atomic temp-and-rename, and a supervised request with a MISSING boot-epoch header is rejected (closing the permissive header-absent #316 path on the supervised path). The CHANNEL-integrity half inherits F1: a 'daemon-owned channel' that is in fact parsed terminal output is not integrity-protected. Fix: resolve the channel half via the F1 authenticated control channel; keep TestResealEpochMirrorRejectsTamperOrMissingEpoch."
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0143 design-v2 REVISION (cycle 1)

author: adjudicator-author-001

> Adjudication of the design-v2 REVISION dialogue trajectory for RFC 0143
> (*lane credential survival across a daemon boot-epoch rotation*). This is the
> **second** falsification pass on the RFC: the design-v1 gate returned
> `needs_revision` with seven findings (F1–F7); the Holder revised the spec and
> the two falsifiers re-attacked it. Inputs read: the revised Holder spec
> (`dialogue/holder/HOLDER.md`), both falsifier re-attacks
> (`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`),
> the `SEED.md` charter (charter, four Open Questions, operator anchor table, the
> `## Binding revision constraints` F1–F7, and the binding maintainer pin), and
> the cycle-1 ledger
> (`docs/operator/artifacts/rfc-0143-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`).
> No raw terminal output was read. The falsifiers' load-bearing source citations
> were independently re-verified against current `main` (see *Citation
> verification* below) and are accurate.

## Verdict

**verdict: needs_revision**

The revision is **materially stronger than v1** and resolves two of the seven
binding findings cleanly, but it does **not** clear the gate. Two new material
challenges — one per falsifier — **land unrebutted**, and four of the remaining
findings are only *nominally* closed (a "fix" that still routes through parsed
PTY output, or that names a predicate with no concrete storage). A
security/authz-hot gate is held high; this is not yet a buildable spec.

**Why not `reject`.** No path widens admin-token exposure, and no minted
credential carries any of `{admin, apply, recovery, surgical_recovery}`. The
revision in fact *strengthens* the no-widening invariant by retiring the v1
lane-readable bearer file entirely. Both falsifiers explicitly say the shape is
salvageable and each supplies a concrete repair. So this is `needs_revision`.

### The clearing condition, walked

A clearing verdict requires **all three** to hold; **each fails**:

1. **All seven F1–F7 resolved with a concrete mechanism — FAILS.** F2 and F4 are
   genuinely resolved. **F1, F3, F5, F6** are only nominally closed and **F7** is
   half-closed (file-mirror concrete; channel-integrity inherits F1).
2. **No new material challenge standing unrebutted — FAILS.** Both falsifier C1
   challenges are material and unrebutted (the cycle ends at adjudication; the
   Holder had no turn to rebut, and the spec as written does not pre-empt them).
3. **Security invariant held structurally — FAILS on two of three axes.**
   *No admin-token widening:* **HELD** (and strengthened). *No replay:* **not
   structural** — the supervisor channel as specified is spoofable PTY output, a
   false-provenance surface. *No split-brain:* **not structural** — the
   reseal/recovery race is unresolved.

## Finding-by-finding walk (all seven F1–F7)

| Finding | Sev | Disposition | One-line reason |
| --- | --- | --- | --- |
| **F1** | high | **open** (nominal) | Floor depends on a structured PTY/helper line; helper parses no child output (D028) and no private control envelope is named. |
| **F2** | high | **resolved** | v1 `0600` bearer file retired (maintainer pin) — no on-disk bearer to replay. Residual replay risk migrates to F1. |
| **F3** | high | **open** (nominal) | "No recovery-generation change" names no column/increment; current schema has none. |
| **F4** | high | **resolved** | `ResealAlternate` admits `CapabilityReseal` on the three routes, records `reseal` not `write`; matrix + guardrail named. |
| **F5** | high | **open** (nominal) | `resealGrace` has no bound/source; reused `activeLeaseFor` still raises raw `lease_error`; no lock order vs recovery. |
| **F6** | medium | **open** | Honest no-in-place-Codex matrix, but the "receiver" is the same parsed-stdout path as F1. |
| **F7** | high | **open** (half) | File-mirror integrity concrete and credited; channel-integrity half inherits F1. |

### F1 — `falsifier-reviewer-001`: the no-token floor still routes through PTY output (open)

The v1 defect was that the floor's escalation routes (`work.block` /
`session.report`) need a capability the no-token lane lacks. The revision
correctly drops those and routes via the daemon-owned helper — **but** the
"structured line on the PTY/helper bridge" the helper is said to forward does not
exist as a content channel in current source. `HelperControlEvent` carries
"lifecycle metadata and byte counts only; agent output bytes stay out of the
control channel" (`helper_protocol.go:41-44`); `RunHelper` "deliberately does not
… inspect workflow state, publish artifacts, complete jobs, or acknowledge work …
only moves process bytes" (`helper.go:120-127`); `pumpPTYProgress` "watches output
VOLUME (not content, per D028)" (`helper.go:357-415`). So the structured line is
either ordinary terminal output — a product-boundary breach (`AGENTS.md`,
`how-to-agent.md`: terminal output is not workflow state; a lane must not advance
state by printing phrases) and **spoofable** (any process whose bytes reach the
lane PTY, including prompt-injected model text, can print the sentinel to drive a
publish/complete or a blocker on the in-flight job) — or a **new private
wrapper-to-helper envelope the revision never specifies** (no reserved exit-code
value, schema, framing, replay protection, or parser boundary). The **typed exit
code** half is a sound trusted-wrapper signal; the spec just does not commit to
the helper recording it *without parsing output bytes*. **Material, unrebutted →
needs_revision.**

### F2 — `falsifier-reviewer-001`: same-uid bearer-file replay (resolved)

The maintainer-pinned decision to carry the reseal authority over a non-bearer,
daemon-owned channel — **no lane-readable reseal token file at all** — dissolves
the v1 same-uid *file* replay at the root. Both falsifiers credit this as correct
and strictly safer than the v1 `0600` file. **Accepted as resolved.** The
*replay* concern does not vanish, though: it **migrates** onto the channel
authentication gap, which is now F1's problem, not a file-mode problem.

### F3 — `falsifier-reviewer-002`: split-brain predicate has no concrete storage (open)

The revised `resealInFlightJob` lists the right ingredients (live session, same
leased/acked job, active-or-grace lease, no recovery-generation change, expected
artifact path, accepted epoch). But the **"no recovery-generation change" guard
names no storage**: `jobs.current_lease_id` exists but there is no generation
column; `leases` has none; `job_recovery_state` holds `requeue_count` /
`transfer_count` / `respawn_count` (a recovery *budget*, not a lease-issued
generation); `review_generation` is the verdict-coherence epoch, a different
concept. An asserted generation guard with no column, no increment point, and no
stamped value is the same "resolved-without-a-concrete-mechanism" pattern the v1
gate rejected. **Material, unrebutted → needs_revision.**

### F4 — `falsifier-reviewer-002`: OR-capability auth prelude (resolved)

`MethodEntry.RequiredCapability` stays `write` for ordinary callers; a
route-specific `ResealAlternate` re-authorizes against `CapabilityReseal` only on
`interrogation.answer` / `work.complete` / `artifact.publish`; both authorizers
project the alternate and record `AuthContext.Capability == reseal` (never
`write`); the authority matrix gains a reseal column and the guardrail test
pins "reseal reaches only those three routes." Both falsifiers credit this as a
concrete answer that does **not** collapse to plain `write`. **Accepted as
resolved.**

### F5 — `falsifier-reviewer-002`: reseal races the lease clock with no race-free contract (open)

Choosing "active lease window + bounded daemon-side reseal grace" is the right
*shape*, but the grace has **no number, source, or maximum**, and
`resealInFlightJob` is said to **reuse `activeLeaseFor`**, which returns a raw
`lease_error` on an expired lease and performs no generation check
(`mutations.go:803-820`). In the common post-rotation race (lane cannot
heartbeat, lease expires within the unspecified grace, recovery sweep drains
helper events then expires/requeues in the same pass) the spec resolves to either
the **forbidden raw `lease_error`** (violates F5) or a **lease revived after
recovery requeued the job** (split-brain, violates F3). The spec gestures at "one
transaction" but names no serialization point or lock order relative to
`lockRunForJob` and the recovery sweep, so **GD-1b cannot distinguish a safe
same-lease renew-and-seal from whichever side of the race won.** **Material,
unrebutted → needs_revision.**

### F6 — `falsifier-reviewer-002`: Codex receiver is the same parsed-stdout path (open)

The adapter matrix **honestly** stops claiming in-place Codex MCP survival — a
real improvement, credited. But its substitute, "the daemon-owned helper reads
from the lane's stdout regardless of adapter," is the **same parsed-PTY-output
path** F1 challenges. No adapter-independent receiver exists in named source;
current Codex rotation only logs and injects a relaunch prompt
(`loop.go:625-646`). The honesty is kept; the mechanism must be re-grounded on
F1's authenticated channel. **Open.**

### F7 — `falsifier-reviewer-002`: epoch integrity (half-closed)

The **file-mirror** half is concretely resolved and credited: endpoint/epoch
moves off lane-writable `.striatum/scratch` to a daemon-owned, lane-read-only
`0644` file with `O_NOFOLLOW` symlink defense and atomic temp-and-rename, and a
supervised request with a **missing** boot-epoch header is rejected (closing the
permissive header-absent #316 path on the supervised path). The **channel**
half inherits F1: a "daemon-owned channel" that is in fact parsed terminal output
is not integrity-protected. **Open pending F1.**

## The two standing material challenges (consolidated)

Both re-attacks reduce to **one defect each**, and together they are a single
sentence: *the revision named the right shapes but not the load-bearing
mechanisms.*

- **C1-security (falsifier-reviewer-001), spans F1/F6 and the channel half of
  F2/F5/F7.** The "daemon-owned supervisor/PTY channel" is, in current source,
  either parsed provider stdout (forbidden, spoofable) or an unspecified private
  envelope. Until a real, authenticated, non-PTY control path is named (or F1 is
  narrowed to a reserved exit code the helper records without parsing output),
  the floor, the reseal request, the Codex receiver, and the epoch-integrity
  claim are not structurally sound.
- **C1-lifecycle (falsifier-reviewer-002), spans F3/F5.** The reseal predicate
  has no concrete recovery/lease generation column, the reseal grace has no
  numeric bound, and there is no lock order serializing `resealInFlightJob` with
  the normal seal paths and the recovery sweep — so the common post-rotation
  expiry race yields either a raw `lease_error` or a split-brain revive.

## Citation verification (adjudicator, against current `main`)

The adjudicator credits a falsifier citation only where it agrees with verified
source. The decisive citations were re-checked directly:

- `helper_protocol.go:41-44` — `HelperControlEvent` payloads "carry lifecycle
  metadata and byte counts only; agent output bytes stay out of the control
  channel." **Accurate.**
- `helper.go:120-127` — `RunHelper` "deliberately does not … inspect workflow
  state, publish artifacts, complete jobs, or acknowledge work … only moves
  process bytes and reports control events." **Accurate.**
- `helper.go:357-415` — `pumpPTYProgress` "watches output VOLUME (not content,
  per D028)," emitting `{bytes,total_bytes,meaningful}`. **Accurate.**
- `mutations.go:803-820` — `activeLeaseFor` requires `state==active`,
  owner/session/job equality, rejects expired leases with `lease_error`, and
  performs **no** generation check. **Accurate.**
- Schema — no `generation` column on `leases`/`jobs`
  (`0005_repo_local_workflow_state.sql`); `job_recovery_state` holds
  `requeue_count`/`transfer_count`/`respawn_count` (`0020_job_recovery_state.sql`);
  `review_generation` is the review epoch (`owner/0009_review_generation.sql`).
  **Accurate.**

All load-bearing citations agree with current `main`; the challenges are
grounded, not speculative.

## Credited strengths (preserve these through the next revision)

Build on these — do **not** regress them:

- **The categorical no-widening refusal holds**, and the revision *strengthens*
  it: `CapabilityReseal` carries no elevated verb and is **never materialized
  into any lane-readable file** — strictly safer than v1's `0600` file.
- **F2 is genuinely resolved** by retiring the bearer file (maintainer pin).
- **F4 is genuinely resolved**: the `ResealAlternate` wiring admits the narrow
  capability without granting `write` and records `reseal` in `AuthContext`.
- **AF1 (reachability-not-reminting)** and **AF4 (epoch/token decoupling)** are
  preserved correctly; `TestTokenValidAcrossRestart` remains a real falsifier.
- **F7's file-mirror half** (daemon-owned, lane-read-only, `O_NOFOLLOW`, atomic
  replace, reject-missing-epoch) is concrete and correct — keep it.
- **The maintainer-ratification gate** and the **per-claim falsifiable-assertion
  discipline** (A1–A13 with named tests/game-days) are the right shape; extend
  them to cover the channel and generation mechanisms, don't abandon them.

## What the next revision MUST fix to clear on re-attack

Exactly two mechanisms, named concretely:

1. **Name the authenticated control channel (closes F1, F6, and the channel
   halves of F2/F5/F7).** Specify a private agentloop/wrapper-to-helper (or
   wrapper-to-daemon) control path that provider `stdout`/`stderr` cannot write
   to — OR narrow Slice A's floor to a **reserved `agentloop` process exit code**
   the helper records **without parsing output bytes**. For Slice B's reseal,
   state how the daemon receives publish/complete intent and artifact identity
   without trusting provider terminal output: derive the expected-artifact set
   from daemon state and refuse unexpected paths. Add
   `TestPTYOutputCannotEmitSupervisorControlEvent`,
   `TestProviderOutputCannotDriveResealOrBlocker`,
   `TestAgentLoopReservedExitCodeRoutesUnrecoverableWithoutPTYParsing`, and
   `TestCodexResealUsesReceiverNotProviderStdout`.
2. **Pin the lease/grace/generation contract (closes F3 and F5).** Name a
   concrete monotonic recovery/lease generation column (with its migration /
   owner-bundle location) — or prove an existing field monotonic across every
   claim/requeue/release — and state where it is incremented and what value is
   stamped into the lease/work-packet for reseal-time comparison. Define
   `resealGrace` numerically with its source and maximum. Specify the exact lock
   order for `resealInFlightJob` relative to `artifact.publish`, `work.complete`
   (`lockRunForJob`), and the recovery sweep. Add
   `TestResealGraceCannotReviveRequeuedLease`,
   `TestRecoveryRequeueWinsOverExpiredLeaseReseal`,
   `TestResealBeyondGraceRoutesTypedNotLeaseError`, and
   `TestResealPredicateUsesStampedRecoveryGeneration`.

Everything else in the revision (OQ1 shape, F2, F4, the F7 file half, the
assertion discipline) is sound and should be carried forward unchanged.

## Note on maintainer ratification (carries forward regardless of verdict)

The chosen direction — a new `rpc.CapabilityReseal` class, a daemon-internal
supervisor-mediated reseal path, and endpoint/epoch republish plumbing — is a
**security/authz trust-model change** that requires **maintainer ratification**
before any build slice touches credential code. The maintainer has already
ratified the OQ1 *shape* and the F2 non-bearer decision in `SEED.md`; that pin
governs the design direction, not the soundness of this spec. Adjudicator
clearance gates the **spec's soundness**; it is **not** the maintainer's product
call on the credential code. Slice A (the Option-4 floor) is zero-trust-change
and may land first under the normal review gate — **once F1 makes it routed over
a real, non-PTY-output channel.** Slice B does not land until the maintainer
ratifies the new capability class **and** this spec clears.

---
<sub>Adjudicator collaboration ledger for the RFC 0143 falsification-gate
design-v2 REVISION run (cycle 1). The ledger verdict — not falsifier completion —
gates the phase: `needs_revision` returns the spec to the Holder. Two material
challenges stand unrebutted; F1/F3/F5/F6 are nominally closed and F7 is
half-closed, while F2 and F4 are genuinely resolved.</sub>
