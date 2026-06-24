# HOLDER — RFC 0166 P0 falsifiable SPEC: the sealed-progress silence budget

author: holder-author-001

> This is the leading proposal the falsifiers re-attack. It hardens the RFC 0166
> proposed direction (the four-part sealed-progress silence budget) into
> build-bearing, falsifiable assertions, ratifies the AND no-false-kill decision,
> and proves the hard core. It does **not** invent a different mechanism. Every
> claim is anchored to named source the falsifiers can open and the build run can
> execute against.

---

## 0. The exact gap (so the predicate's job is unambiguous)

A lane reaches `recoverStuckJobs`
(`go/pkg/mutations/recovery_decision_tree.go:704`) and is classified by the
**pure** `sessionliveness.Classify` (`go/pkg/sessionliveness/liveness.go:475`).
I traced the live #576 lane through `Classify` to pin which existing rung *should*
have caught it and why none does:

- The supervisor renews the work-lease from **PTY output**
  (`lease.heartbeat`, source `supervisor_pty_progress`), so
  `active_lease_last_heartbeat_at` (`ActiveLeaseHeartbeatAt`) stays fresh. That
  column is in **both** the lease-fresh base (`liveness.go:574`) **and**
  `protocolActivityFresh` (`liveness.go:845`). So the lane reads
  `working_protocol` right after each heartbeat and `quiet` between them — never
  `StallLeaseHeartbeat` (lease warm), never `ProtocolStalled`.
- The #324 wedge (`StallToolProgress`, `liveness.go:70`) is gated on
  `working.Protocol == ProtocolWorkingLocal` (`liveness.go:603,610`).
  `working_local` requires PTY fresh **and** `!protocolActivityFresh`
  (`workingResult`, `liveness.go:711-723`). A lane with **any** fresh
  protocol/lease-heartbeat signal reads `working_protocol`, so the
  `working_local`-gated #324 rung **cannot fire for it**. This is the structural
  reason #324 misses #576.
- `recoverStuckJobs` therefore sees `protocol ∈ {working_protocol, quiet}`, hits
  the `default` branch, finds `!confirmedDead()` (the agent is alive), and
  `continue`s (`recovery_decision_tree.go:1118-1120`). No action, forever.

So the target is precisely: **a lease holder kept non-stalled only by
forgeable/raw liveness (PTY-driven lease heartbeats, raw PTY frames), whose
forgery-resistant sealed timeline AND tool-call timeline are both stale.** That
is the lane the new rung must catch, and the single-final-seal genuinely-working
lane (real tool calls toward one terminal `work.complete`) is the lane it must
never touch.

---

## 1. PART 1 — the clock (derived detector)

**Claim 1.1 (derived, not stored).** The deadline is recomputed every sweep as
`now − floor`, where
`floor = GREATEST(jobSealedProgressAt(job), currentActiveLeaseAcquiredAt, jobs.started_at)`.
No `deadline_at` / `max_duration` column is added. Inputs are all durable
PostgreSQL timestamps: `jobSealedProgressAt` =
`GREATEST(max(artifacts.created_at), max(verdicts.created_at))`
(`recovery_decision_tree.go:446-465`), `leases.acquired_at`
(`0005_repo_local_workflow_state.sql`, lease DDL), `jobs.started_at`
(`0005_repo_local_workflow_state.sql:75` jobs DDL, nullable → `created_at` as the
final fallback inside the GREATEST/COALESCE).
- *Evidence that supports it:* `jobSealedProgressAt` already exists and is already
  called inside `recoverStuckJobs`' tx; no new read path is needed.
- *Refutation test:* a unit test that sets all four timestamps, computes the clock,
  then **restarts the daemon process** (drops all in-memory state) and recomputes —
  if the value differs, Claim 1.1 is false. Because nothing is stored, restart
  must reproduce the identical clock; a stored-deadline design would not.

**Claim 1.2 (floor uses the CURRENT lease, not the job's first start).** The floor
is anchored on the **current active lease's** `acquired_at`, not the job's
original `started_at`, for any lane that has been requeued. A fresh requeued
session must start with a full budget; flooring on the original `started_at` would
let a never-sealing single-final-seal lane cross any finite budget across requeues
while a *brand-new* session is doing real work — the #145 CASE-2 mid-work kill
(`liveness.go:581-589`).
- *Evidence:* `requeueJobSameAttempt` issues a fresh claim → a new `active` lease
  with a new `acquired_at`; `GREATEST(...)` makes that new acquire dominate, so the
  per-session clock resets on requeue while the cross-session telomere (Part 4)
  bounds total fresh sessions.
- *Refutation test:* requeue a job (no seal), let the fresh session run < budget,
  and assert the rung does **not** fire on the fresh session purely because the
  *original* `started_at` is now > budget old. If it fires, Claim 1.2 is false.

**Claim 1.3 (new stall class, evaluated where the data lives).** A new constant
`StallSealedProgress = "wedged_no_sealed_progress"` is added next to
`StallToolProgress` (`liveness.go:70`). It is **not** decidable by the pure
`Classify` (which never SELECTs per-job `artifacts`/`verdicts`); it is evaluated
**only** in `recoverStuckJobs`, which already holds the tx, already SELECTs the
job/lease/session rows, and already calls `jobSealedProgressAt`. Like `StallDiscovery`
it is a `Stall*` enum value persisted to `liveness_stall_class`; it must be added to
the migration-0012 CHECK constraint set so the persisted class is legal.
- *Refutation test:* a test that drives the pure `Classify` with activity columns
  that would breach the sealed clock and asserts `Classify` **never** returns
  `wedged_no_sealed_progress` (it has no artifacts/verdicts input). If `Classify`
  can emit it, the layering claim is false.

---

## 2. PART 2 — the correctness constraint (AND, not OR): the central decision

**Claim 2.1 (the rung fires only when BOTH axes are stale).** The new rung fires
iff:

```
sealedSilenceBreached := SealedSilenceSeconds > 0
                         && missed(floor, SealedSilenceSeconds, now)
toolWedged             := sessionliveness.ToolProgressWedged(activity, policy, now)  // the EXACT #324 predicate
fire                   := sealedSilenceBreached && toolWedged
```

`ToolProgressWedged` is the existing `toolProgressWedged`
(`liveness.go:793-805`) promoted to an **exported** predicate so the AND's
tool half is bit-identical to #324 and cannot drift. That predicate already
encodes three load-bearing exclusions the AND inherits for free:
1. **Tool-call history precondition** (`base == nil → false`): a lane that has
   issued *zero* tool calls is not wedged by it.
2. **In-tool exclusion** (`if inTool { return false }`): a lane *inside* a single
   long tool call (a 20-minute test suite) is `working_tool`, never wedged — this
   is exactly the #145 long-foreground-command reprieve, inherited unchanged.
3. **Disabled when `ToolProgressSeconds <= 0`.**

**Claim 2.2 (no false-kill of the single-final-seal lane — the hard core, half 1).**
A legitimately-healthy lane whose only deliverable is one terminal `work.complete`
(zero intermediate seals → `jobSealedProgressAt` null → floor governs → sealed
clock crosses any finite budget) is **not** trapped *as long as it is making
real tool calls* (Read/Edit/Bash/Grep), because those keep the tool-call timeline
fresh → `toolWedged == false` → AND false. The deadline can only fire once the
lane has *also* gone tool-silent past `ToolProgressSeconds` (default 600 s, chosen
to clear genuine slow turns, `liveness.go:219-228`).
- *Evidence:* this is the same calibration #324 already ships; the AND strictly
  *narrows* #324's firing set (it adds a second required condition), so it cannot
  kill anything #324 already spares.
- *Refutation test (Falsifier 1's target):* construct a single-final-seal lane that
  (a) never seals and (b) is **inside** a long tool call OR emits a fresh tool call
  every < `ToolProgressSeconds`; assert the rung never fires. Any firing here is a
  hard fail (it is the #145 CASE-2 artifact-loss regression).

**Claim 2.3 (ratify AND over OR / sealed-only).** OR (`sealed-stale OR tool-stale`)
and sealed-only both reduce to "kill a lane that has not sealed for N seconds",
which is the banned plain wall-clock cap — it false-kills the single-final-seal
lane mid-work. The AND is the minimal predicate that closes the §0 gap **without**
that regression. **I ratify the AND as the no-false-kill core.**

**Claim 2.4 (HONEST coverage boundary — the efficacy edge Falsifier 2 must weigh).**
The AND deliberately does **not** catch a lane that games the tool axis by
**polling** (`work.await_packet`/`heartbeat` are `tools/call`s that stamp the
tool-call timeline). Such a lane keeps `toolWedged == false` and evades the rung.
This is an *accepted P0 residual*, not an oversight: at the tool-recency grain the
polling-gamer is indistinguishable from the genuinely-working single-final-seal
lane, and the objective makes no-false-kill load-bearing, so P0 chooses to spare
both. The P0 rung's guaranteed catch is the lane kept alive **only** by forgeable
PTY/lease-heartbeat liveness with a stale tool timeline (the §0 lane). The
polling-gamer is named as a P1 seam in §6 (tool-call **novelty**, the Part 3
principle applied to the tool axis). Pretending the AND closes the polling case
would be the dishonest claim; it does not, and that boundary is stated so the gate
ratifies what is actually built.

---

## 3. PART 3 — anti-gaming: reset consumes novelty, not events

**Claim 3.1 (a raw sealed clock is gameable; novelty is the fix).** `jobSealedProgressAt`
is `max(created_at)` over artifact/verdict rows — **any** new row resets it. A
flailing lane can mint rows. So the *reset of the telomere counter* (Part 4) is
gated not on "an event happened" but on a **strict increase** in a daemon-derived
monotonic progress position recorded on `striatumd.job_recovery_state`:

```
pos := ( count(distinct content_sha256 of the job's MILESTONE-bearing artifacts),
         count(sealed verdicts for the job),
         highest satisfied REQUIRED expected_artifacts milestone index )
```

Reset fires iff `pos` is strictly greater (lexicographically, any dimension) than
the stored cursor.

**Claim 3.2 (grain = `content_sha256`, scoped to declared/milestone artifacts).**
De-dup grain is `content_sha256` (`artifacts` DDL,
`0005_repo_local_workflow_state.sql:220`). Re-publishing identical bytes does not
grow `count(distinct content_sha256)`, so the cursor does not advance — the
identical-replay vector is closed.
- *Refutation test:* re-publish an identical-`content_sha256` artifact and assert
  the cursor (and the telomere counter) do **not** advance. If they do, 3.2 is false.

**Claim 3.3 (junk-blob hardening — the non-obvious hole).** I verified
`expected_artifacts are presence assertions, not an allowlist`
(`artifact_source_publish.go:115`): a lane **can** source-publish *undeclared*
in-scope files, each a distinct `content_sha256` artifact row
(`artifact_source_publish_test.go:133-164`). A naive `count(distinct content_sha256)`
over **all** the job's artifacts is therefore gameable by writing junk files into
`allowed_paths`. **Hardening:** the `content_sha256` dimension of `pos` counts
distinct hashes only over artifacts that satisfy a **declared** `expected_artifacts`
logical_name (milestone-bearing), **not** arbitrary undeclared source-published
blobs. Undeclared in-scope blobs remain durable provenance but do not advance the
novelty cursor. This keeps all three RFC dimensions while making each
un-gameable; it is a *hardening of the grain*, not a new mechanism.
- *Evidence the residual is bounded even un-hardened:* any cursor-advancing bytes
  must be genuinely-distinct, durable, on the run branch, and reviewable — finite
  and auditable, never silent.
- *Refutation test (Falsifier 2's target):* a lane writes N distinct junk files
  into `allowed_paths` each sweep and source-publishes them; assert the novelty
  cursor does **not** advance and the telomere still shortens. If junk advances the
  cursor, 3.3 is false.

**Claim 3.4 (verdict/milestone dimensions are intrinsically un-gameable).** `verdicts`
rows are written only by the daemon's verdict / `review.submit` handler for
verdict-capable jobs; for a non-reviewer job the count is structurally 0 and cannot
be minted. For a reviewer job, producing its required verdict **is** completing the
job (it seals and terminates), so a verdict cannot be used to keep a job alive. The
milestone index is monotone over **required** artifacts and cannot decrease.
Within one attempt the `(logical_name, attempt)` row is immutable
(`0018_artifact_attempt_scope.sql`) and append-only (`0005` triggers), so a single
logical_name cannot be re-minted with new bytes to inflate the count.

---

## 4. PART 4 — the action: self-heal ladder, telomere-bounded

**Claim 4.1 (first breach auto-requeues one fresh session, no page).** The new rung
reaches the **existing** budgeted action path: it sets
`action = "transfer_requeue"` (still-active stalled owner; `closeStalledOwner =
true`, `forceExpire = true`) and lands in the same
`requeueJobSameAttempt` + `closeStalledOwningSession` block
(`recovery_decision_tree.go:1331-1427`) CASE 2 already uses. No new requeue
mechanism is introduced; convergence is **inherited**, not re-derived.
- *Refutation test:* trip the rung once; assert exactly one fresh session is
  requeued, the stalled owner is closed, and **no** escalation/page is raised on the
  first breach.

**Claim 4.2 (the budget resets only on genuine sealed progress = telomere).** The
requeue is counted on `job_recovery_state.requeue_count` (or, for the unsealed
shape, the `agent_exited_unsealed` smaller bound, `recovery_decision_tree.go:1152-1161`).
The counter resets to 0 **only** when Part 3's novelty cursor strictly increases.
A lane burning fresh sessions without sealing never resets → `requeue_count`
monotonically climbs toward `policy.maxRequeues` (default 2,
`recovery_decision_tree.go:76`) / `unsealedRequeueBudget` → the telomere floor.
- *Evidence:* this is exactly the `current >= limit` branch
  (`recovery_decision_tree.go:1168`) I read; the new stall class flows through it
  with no change to the convergence logic.

**Claim 4.3 (escalation is bounded, storm-free, with a copy-pasteable verb).** At the
floor, `markRecoveryEscalation` flags `escalation_pending` once (`escalated_at`
set once, idempotent, `recovery_decision_tree.go:418-434`) → Phase 4 flips the run
`needs_operator`. The escalation carries a single recovery verb:
`recovery complete-stalled` when a durable artifact exists (reusing the #308 /
D200 `tryFinalizeUnsealedFromDurableArtifact` durability gates,
`recovery_decision_tree.go:1751-1815`), else `recovery requeue-stale --override`.
- *Convergence proof:* `requeue_count` is monotone non-decreasing absent a novelty
  reset, bounded by `maxRequeues`; so a deterministically-flailing lane escalates in
  at most `maxRequeues + 1` breaches. `escalation_pending`/`escalated_at` idempotency
  forbids an escalation storm. No auto-requeue loop: each breach either advances the
  counter (toward the floor) or — if it sealed novel work — resets and is by
  definition making progress.
- *Refutation test (Falsifier 2's target):* a lane that seals **nothing** across
  sweeps; assert `requeue_count` climbs to the floor and then escalates exactly once,
  in bounded sweeps, with the correct verb (`complete-stalled` iff a durable required
  artifact reconstructs, else `requeue-stale --override`). A loop, a storm, or the
  wrong verb refutes 4.3.

---

## 5. Hard-core proof obligations (the three the gate gates on)

1. **No false-kill (alive-and-legitimately-working ⇒ never killed).** Proven by
   Claim 2.1's AND + Claim 2.2: a lane making real tool calls or inside a tool call
   is spared (tool half false); a lane inside a long single tool call is `working_tool`
   and excluded by `toolProgressWedged`'s in-tool guard; the per-session clock resets
   on requeue (Claim 1.2). The strongest residual (long fully-silent think with no
   PTY, no tool call, no seal for > both budgets) is the genuinely-not-producing case
   the operator-grant seam (§6) and `SealedSilenceSeconds` calibration cover.
2. **Un-gameable (fake progress ⇒ no reset, no evasion of the telomere).** Proven by
   Claim 3.2 (identical replay), Claim 3.3 (junk-blob grain hardening), Claim 3.4
   (verdict/milestone intrinsics). The acknowledged residual (tool-axis polling,
   Claim 2.4) evades the *deadline* but cannot reset the *telomere* (it seals no
   novelty), so it cannot keep a job un-escalatable indefinitely once the deadline is
   reached by any means — and it is named as a P1 seam, not silently dropped.
3. **Restart + requeue consistent.** Proven by Claim 1.1 (derived, all-durable
   inputs ⇒ identical recomputation across restart) and Claim 1.2 (current-lease
   floor ⇒ correct per-session reset on requeue). The novelty cursor is persisted on
   `job_recovery_state` (Part 3 migration), so a restart mid-ladder resumes the same
   counter, not a fresh one.
   - *Refutation test:* advance the cursor + counter, restart the daemon, and assert
     the next sweep observes the persisted cursor (no spurious reset, no carry-over of
     a phantom advance).

---

## 6. P0 slice boundary & named later seams

**P0 ships (minimum that closes #576 safely):**
- The derived clock (Part 1) + `wedged_no_sealed_progress` rung in `recoverStuckJobs`
  (Claim 1.3) + the migration-0012 CHECK-constraint addition for the new enum.
- The AND gate reusing the exported `ToolProgressWedged` (Part 2).
- The novelty cursor on `job_recovery_state` via a **migration in the 0035 style**
  (`ADD COLUMN IF NOT EXISTS`, runtime-owned, degrade-safe: a deployment behind on it
  reads zeroed cursor and the rung simply does not arm — matching the 131-C /
  bundle-0012 degrade pattern): `sealed_progress_distinct_artifacts int`,
  `sealed_progress_verdicts int`, `sealed_progress_milestone int` (Part 3).
- First-breach auto-requeue-one + telomere escalation through the existing budgeted
  path (Part 4).
- **Shadow-first arming:** `SealedSilenceSeconds` is a new
  `sessionliveness.Policy` field defaulting to **0 = disabled** (the
  `ToolProgressSeconds` convention, `liveness.go:308-322`). P0 lands the rung as an
  **advisory** `doctor`/`dashboard` signal first; the escalating action arms only on
  an explicit opt-in threshold. A false-positive deadline must never convert a
  healthy run into an outage just by landing to `main`.

**Named later seams (out of P0):**
- **Threshold calibration** — per-`job_type` vs global `SealedSilenceSeconds`, chosen
  empirically from a shadow-bake corpus (there is no historical sealed-silence
  distribution to set it a priori).
- **Tool-axis novelty** — the Claim 2.4 polling-gamer: require tool-call *novelty*
  (distinct tool-call signatures / forward tool progress) rather than recency, so a
  lane polling `await_packet` to stay warm is also caught without false-killing the
  genuine single-final-seal lane.
- **Operator known-slow grant** (RFC Part 5) — `recovery grant-silence <run> <job>
  --seconds N` writing a forgery-resistant daemon-recorded floor
  (`floor = GREATEST(sealedProgressAt, currentLeaseAcquiredAt, max(grant))`); a lane
  cannot self-grant.
- **Run-scoped sibling-fairness preemption** (RFC appendix / #579 cousin).

**Local-first boundary preserved.** Every input is the daemon-owned PostgreSQL
state (`artifacts`, `verdicts`, `leases`, `jobs`, `job_recovery_state`); no hosted
service, telemetry, or external persistence is introduced. The clock is derived
from durable rows the daemon already owns.

---

## 7. Single-sentence claim for the falsifiers

*Adding a `wedged_no_sealed_progress` rung to `recoverStuckJobs` — a derived
per-session clock (`now − GREATEST(jobSealedProgressAt, currentLeaseAcquiredAt,
started_at)`) gated by an AND with the exact #324 `ToolProgressWedged` predicate,
whose telomere requeue counter resets only on a strict increase of a
content_sha256/verdict/milestone novelty cursor scoped to declared artifacts —
closes the alive-but-never-completing gap in bounded, restart-consistent sweeps
without re-introducing the #145 CASE-2 mid-work kill and without being resettable by
forged or junk progress.*
