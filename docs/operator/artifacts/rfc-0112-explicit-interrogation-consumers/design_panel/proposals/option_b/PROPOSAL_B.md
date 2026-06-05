# Proposal B — Lifecycle-First Explicit Interrogation Consumers

author: proposer-b-claude-opus-4.8-001
date: 2026-06-05
run: rfc-0112-explicit-interrogation-consumers-design-panel
role: proposer_b (independent option lane)

## 0. Thesis

The RFC 0112 failure is two-sided — the consumer set closes the window too
early, and the closure hook would close it too late or never — and both sides
are **lifecycle defects, not schema defects**. Option B therefore anchors the
design on the job lifecycle state machine:

1. **Window lifetime is a pure derivation of job lifecycle state.** The
   pending-consumer predicate reads the immutable workflow snapshot plus the
   live `jobs` table. Nothing is materialized, registered, or cached; there is
   no second copy of the consumer relation to drift.
2. **"Consumer became terminal" is a first-class lifecycle moment with one
   choke point.** Every production path that moves a potential consumer from a
   non-terminal to a terminal state funnels through a single shared helper
   that both writes the terminal state and runs the generalized release hook.
   A guard test makes a bypassing future path a compile-adjacent failure, not
   a latent leak.
3. **Validation rules are chosen to make the lifecycle correct, not just the
   graph tidy.** The reachable-downstream rule is what guarantees the RFC 0095
   revision cascade re-blocks every explicit consumer with zero new reopen
   code (§7). Validation and lifecycle are one design, not two.

Everything else (field shape, packet projection, `required` semantics)
follows from those three commitments.

## 1. Workflow field shape and validation (panel Q1)

Option B keeps the RFC 0112 §1 shape unchanged — the field rides on the
**consumer** job and is resolved only at lifecycle moments:

```json
{
  "id": "cross_examiner_1",
  "type": "build",
  "interrogation_targets": [
    { "workflow_job_id": "convener_draft", "required": true }
  ]
}
```

V1 entry fields, exactly two:

- `workflow_job_id` (string, required) — the upstream workflow job whose
  latest-attempt awaiting-interrogation session is the target.
- `required` (bool, optional, default `false`) — packet instruction strength
  plus durable-evidence trigger (§2). Not a completion gate in V1.

The field is declarative and frozen into the run's workflow snapshot at
`run.prepare`, like `interrogable` itself
(`go/pkg/mutations/lifecycle.go:1071` `jobIsInterrogable` reads the snapshot;
no `jobs`-table column is added, honoring the RFC 0079 §5 owner-table rule).

### Validation rules (hard errors at validate/prepare)

Enforced where `ValidatePhaseShapes` already runs
(`go/pkg/workflowauthoring/phases.go:43`, invoked from
`go/pkg/mutations/run.go:1081`), as a sibling `ValidateInterrogationTargets`:

| Rule | Severity | Reason |
|---|---|---|
| `workflow_job_id` exists in the same workflow | error | dangling target can never arm; silent-wedge class |
| target declares `interrogable: true` | error | a target that never opens a window silently never arms — fail loud at authoring time |
| target ≠ declaring job (no self-reference) | error | mirrors `interrogation.open`'s self-interrogation refusal (`interrogation.go:39`) |
| declaring job reachable downstream of the target through workflow edges | error | load-bearing for revision correctness (§7), not just hygiene |
| duplicate `workflow_job_id` entries on one job | error | ambiguous `required` merge; cheap to reject |
| `interrogation_targets` present on the target job itself | error | per RFC 0112 §1 |
| unknown entry fields | **lint warning** | forward compatibility for V2 fields (e.g. a future `gate` flag); `workflowauthoring.Lint` (`lint.go:27`) gains `lintInterrogationTargetUnknownFields` |
| `required: true` while the declaring job's lane lacks `interrogate` capability | lint warning | honest lint: the packet will instruct an interrogation the session cannot open |

Reachability is computed over the snapshot's declared edges (the same
`job_dependencies` source that `run.prepare` materializes), so direct
dependency remains sufficient but not necessary, per RFC 0112.

Documentation home: the workflow schema section of `docs/reference/spec.md`
plus a `docs/reference/ubiquitous-language.md` entry for **interrogation
consumer** (follow-up scope, §11).

## 2. `required: true` semantics in V1 (panel Q2)

**Advisory packet-instruction strength, with durable lifecycle evidence. Not a
hard gate.** The lifecycle argument, which is stronger than the compatibility
argument:

A hard completion gate would couple the consumer's ability to terminalize to
the *target's* liveness — inverting the dependency direction the entire
lifecycle is built on. The window can retire for legitimate reasons the
consumer does not control and cannot remediate:

- a `needs_revision` reopen retires the target mid-flight
  (`closeInterrogationTargetForReopen`, `interrogation.go:566`);
- the recovery sweep retires a leaked window
  (`closeLeakedInterrogationWindow`, `recovery_decision_tree.go:273`);
- run cancel/failure closes every session
  (`closeRemainingSessions`, `run.go:470`, `mutations.go:835`).

Gating `work.complete` on "interrogated at least once" turns each of those
into a fresh wedge class — precisely the #84/#65 family RFC 0095 spent three
phases retiring. The existing non-wedging `interrogation_unavailable` signal
(`interrogation.go:743`, reason `panel_window_closed`, #131) remains the
fallback contract.

### Durable records (RFC 0112 OQ 3)

Durable evidence lives on **lifecycle transitions, not on projections**:

1. `interrogation.open` returning the unavailable signal appends an
   `interrogation.unavailable_signaled` event (curated, D028-clean: target
   session id, interrogable job id, reason — no body text). Today the signal
   is response-only; making it an event gives the adjudicator and the RFC 0105
   fixtures an assertable record that "unanswered interrogation is evidence"
   (C-098-2).
2. When the terminal-consumer choke point (§5) terminalizes a job whose
   declaration says `required: true` and **no** interrogation row exists from
   that job's session(s) against the resolved target for the current attempt,
   it appends `interrogation.required_skipped` (job id, attempt, target
   workflow job id, resolved target session id or null, packet-surfaced
   state). Evidence, not enforcement.

Packet-build surfacing alone does **not** write an event: claims can repeat
(requeue, stale-lease re-claim) and a projection should not mutate history.
This split answers OQ 3: *surfaced* unavailability is recorded when the lane
acts (open) or when the consumer finishes without acting (terminal hook) —
the two moments with lifecycle meaning.

A later hard-gate V2 becomes a pure predicate flip over data V1 already
records: refuse `work.complete` unless an interrogation row or an
`interrogation.unavailable_signaled` / `required_skipped` event exists. That
is the concrete deferral plan for RFC 0112 OQ 1.

## 3. Multiple targets per consumer in V1 (panel Q3)

**Allow N ≥ 1. No artificial cap.** The lifecycle treats each
(target, consumer) pair independently; there is no shared state across pairs
to corrupt, so a cap would be schema policing with no lifecycle benefit:

- **Predicate:** each target's window is held open by *its own* declared
  consumer set. With consumers in different states, target A's window closes
  when all of A's consumers are terminal regardless of target B's state.
  The existing release hook is already plural over upstreams
  (`releaseInterrogationTargetForCompletedReview` loops `upstreams`,
  `interrogation.go:535-557`) — N declared targets reuse the same loop shape.
- **Packet:** `context.interrogation_targets[]` carries one entry per
  declared target, each with its own independently-resolved
  `state`/`target_session_id` (§6). Two targets in `available` and
  `unavailable` states coexist legibly; the lane consults each entry.
- **Hook:** the terminal choke point releases the union
  (direct upstreams ∪ declared targets); per-target
  `maybeCloseInterrogationTarget` (`interrogation.go:386`) is already
  idempotent and guarded, so order does not matter.
- **`required` with N targets:** evaluated per entry. A consumer with two
  `required: true` targets gets two instructions and (if skipped) two
  `required_skipped` events.

Foreseeable need is real, not hypothetical: a falsification-gate variant with
two interrogable drafts, or an ACE extension where cross-examiners also
interrogate the `revision_draft` on cycle ≥ 2. One lint warning (not a block)
when a single job declares more than three targets keeps authoring honest.

## 4. Window-ownership predicate: derived, never stored

`interrogationConsumersPending` (`interrogation.go:516`) becomes the disjunction
of two halves:

```
pending(target) :=
     EXISTS direct dependent job NOT IN terminal set        -- existing SQL, unchanged
  OR EXISTS job in run whose snapshot definition declares
       interrogation_targets[].workflow_job_id = target.workflow_job_id
       AND job.state NOT IN terminal set                    -- new half
```

with the terminal set unchanged:
`('completed','failed','canceled','skipped','waiting_human')`
(`interrogation.go:373`).

The new half is computed by fetching the run's workflow snapshot and mapping
declared consumer `workflow_job_id`s to live job rows — the identical
derivation pattern `jobIsInterrogable` (`lifecycle.go:1071`) and
`workflowJobInterrogable` (`claim.go:537`) already use. The snapshot is
immutable per run, so the derivation is always-correct by construction.

**No new table, no schema change, no migration.** This is a deliberate
Option B choice (decision criterion 8): a materialized
`interrogation_consumers` table is rejected in §10 because it creates a
parallel source of truth that `run.prepare`, revision cascades, and
re-preparations must keep coherent, and it drags in RFC 0079 §5
owner-migration friction for a relation that is a pure function of data we
already store.

Because all four close guards stay where they are
(`maybeCloseInterrogationTarget`, `interrogation.go:397-416`: active state,
no open interrogations, no active lease, no pending consumers), C-095-4 holds
untouched; the only change inside the close path is the widened predicate.

**Why this fixes the early close:** all of a run's jobs exist from
`run.prepare` in `blocked` state, and `blocked` is not in the terminal set.
The moment `convener_synthesis` records its accepting verdict, the
cross-examiners are `blocked` — and with the declaration in the snapshot they
are now visible to the predicate, so the window survives the phase gate with
no graph edit and no scheduling change (C-098-1 intact).

## 5. The terminal-consumer release hook and its choke point (panel Q4)

### 5.1 The generalized hook

`releaseInterrogationTargetForCompletedReview` (`interrogation.go:534`)
generalizes to:

```
releaseInterrogationTargetsForTerminalConsumer(ctx, runner, repositoryID, runID, jobID)
```

1. Candidate targets := direct `depends_on_job_id` upstreams (existing query)
   ∪ this job's declared `interrogation_targets` resolved through the
   snapshot to interrogable job ids.
2. For each candidate, resolve the target session
   (`interrogableTargetSessionForJob`, `interrogation.go:494`) and call the
   idempotent `maybeCloseInterrogationTarget`.
3. Emit the `interrogation.required_skipped` evidence event (§2) when
   applicable.

The old name remains as a one-line alias during review, then is deleted; its
three call sites move to the choke point below.

### 5.2 The choke point: `markJobTerminal`

The lifecycle-first centerpiece. A new shared helper in `mutations`:

```
markJobTerminal(ctx, tx, repositoryID, job, newState, opts)
  -- writes the jobs-row terminal UPDATE (state, completed_at, lease clear)
  -- appends the job.* event the caller specifies
  -- invokes releaseInterrogationTargetsForTerminalConsumer
```

Every production path that moves a job **from a non-terminal state into the
terminal set** swaps its inline `UPDATE striatumd.jobs` for the helper. The
complete inventory, from source:

| # | Path | Site | Transition | Hook today |
|---|---|---|---|---|
| 1 | `work.complete` (build/synthesis) | `lifecycle.go:1009` (`HandleWorkComplete`) | running → completed | **missing** — the ACE cross-examiner path; the leak half of the RFC failure |
| 2 | `work.block` escalation | `lifecycle.go:908` (`HandleWorkBlock`, `isEscalation` severities) | running → waiting_human | **missing** (waiting_human is in the terminal set) |
| 3 | `review.verdict` accept / accept_with_findings | `review.go:563` (`completeReviewJob`) | running → completed | present (`review.go:569`) |
| 4 | `review.verdict` needs_revision absorbed by adjudicator | `review.go:614` | running → completed | present (`review.go:617`) |
| 5 | `review.verdict` reject (no revision cycle) | `review.go:659` (`failReviewJob`, `review.go:912`) | running → failed | **missing** |
| 6 | `review.verdict` needs_revision → human checkpoint | `review.go:633` (`openHumanCheckpoint`, write at `review.go:959`) | running → waiting_human | **missing** |
| 7 | `override-verdict` completing a parked review | `review.go:326` (`HandleReviewOverride`) | waiting_human → completed | present — terminal→terminal, see invariant below |
| 8 | operator escalation resolve completing a review | `operator.go:306` | waiting_human → completed | terminal→terminal |
| 9 | checkpoint dismiss canceling the blocker job | `operator.go:426` | waiting_human → canceled | terminal→terminal |
| 10 | recovery auto-publish completing a job | `recovery.go:1073` (`completeAutoRecoveredJob`, write at `recovery.go:1100`) | stale/running → completed | **missing** |
| 11 | recovery validated-outputs complete | `recovery.go:1892` (`completeRecoveredJob`, write at `recovery.go:1923`) | → completed | **missing** |
| 12 | `recovery.cancel_job` | `recovery.go:2036` (`cancelSingleJob`, write at `recovery.go:2073`) | → canceled | **missing** |
| 13 | auto-finalize completing a straggler | `recovery_auto_finalize.go:1104` (`completeAutoFinalizedJob`, write at `recovery_auto_finalize.go:1131`) | → completed | **missing** |
| 14 | `run.cancel` bulk job cancel | `run.go:446` | bulk → canceled | structurally covered: `closeRemainingSessions` (`run.go:470`) closes every session including any target |
| 15 | run failure finalization | `mutations.go:828-835` | run failed | structurally covered, same closer |

Three invariants make this table complete rather than merely long:

- **Terminal→terminal transitions are predicate no-ops.** Rows 7–9 move jobs
  *within* the terminal set, so the pending-consumer count cannot increase
  and the existing hook call at row 7 was always a safety call, not a
  correctness need. `markJobTerminal` keeps firing the hook there anyway —
  it is idempotent — so the rule stays uniform: *every* terminal write goes
  through the helper.
- **Re-entry cannot resurrect.** A `waiting_human` consumer that resumes
  (blocker resolved → running, `recovery.go:332`) may find the window already
  closed. Per C-095-2 a closed session is never resurrected; the resumed
  consumer receives the non-wedging unavailable signal and the §2 evidence
  event records it. This is the one deliberately accepted one-way door, and
  it is existing panel behavior, not new.
- **Reopen paths are not terminalizations.** `reopenJobForAttempt`
  (`revision_routing.go:280`) re-blocks; its window obligation is the
  *reopen* closer (§7), already wired at `revision_routing.go:291`.

### 5.3 The guard that keeps the set closed

Two layers, both with repo precedent:

1. **AST guard test** (`go/pkg/mutations/terminal_release_guard_test.go`,
   modeled on `mutation_tx_guard_test.go` and the RFC 0111 both-direction
   catalog guard): parse the `mutations` package; any `UPDATE striatumd.jobs`
   statement that sets `state` to a literal in the terminal set must occur
   inside `markJobTerminal` or carry an explicit allowlist entry with a
   written rationale. The shipped allowlist has exactly two entries — rows 14
   and 15 — each annotated with the `closeRemainingSessions` structural
   argument. A new terminalizing path fails the guard until it either uses
   the choke point or argues its exemption in code review.
2. **The recovery sweep as convergence backstop.** `recoverStuckJobs`'
   leaked-window case (`recovery_decision_tree.go:268-287`) inherits the
   widened predicate automatically, so even a hypothetical missed path
   converges: the next sweep closes the orphaned window loudly
   (`interrogation_window_closed` action in the sweep report) instead of
   leaking it silently. C-105-2's complete-or-escalate-within-budget property
   is preserved by construction.

## 6. Work-packet projection (panel Q6)

The projection lives at `context.interrogation_targets[]`, assembled in
`buildPacket` (`claim.go:283`) alongside the existing optional context blocks
(`implementation_envelope`, `shared_resources`, `augmentation_references`,
`claim.go:341-349`) — same contract, same `content_mode: references`
namespace, read-only derivation, **no writes at claim time**.

Per declared entry:

```json
{
  "workflow_job_id": "convener_draft",
  "required": true,
  "state": "available",
  "target_session_id": "sess_…",
  "target_attempt": 2,
  "reason": null,
  "instruction": "Open interrogation against target_session_id and record at least one question before publishing findings; if open returns interrogation_unavailable, proceed on the published artifact and note it."
}
```

State definitions are tied to lifecycle facts, not heuristics:

| `state` | Lifecycle fact | `target_session_id` | `reason` |
|---|---|---|---|
| `available` | latest `session.awaiting_interrogation` event for the target job resolves to a session whose row is `active` (`interrogableTargetSessionForJob`, `interrogation.go:494`) | the live session | null |
| `unavailable` | an awaiting event exists but that session is no longer active | the retired session id (evidence linkage) | the session's `close_reason` verbatim — `interrogation_window_closed` or `revision_reopened` — so the lane sees *why* without a state-changing call |
| `not_ready` | no `session.awaiting_interrogation` event has ever been recorded for the target workflow job | null | `target_not_yet_completed` |

`not_ready` vs `unavailable` is therefore exact: *never armed* vs *armed and
since retired*. In a validated graph `not_ready` is rare (the consumer's
claimability implies the target path completed), but it is reachable — e.g. a
target that terminalized as `skipped` — and the packet must be legible rather
than wrong when it happens.

The instruction string is daemon-authored per (state, required) pair, with
the unavailable variant mirroring the `interrogation.open` signal's guidance
text (`interrogation.go:770`) so the packet and the RPC never disagree. The
projection consumes no new RPC and changes nothing for jobs that declare no
targets (AC 5, criterion 3): the block is simply absent, exactly like the
other optional context blocks.

## 7. Revision reopen and stale-target retirement (panel Q5-adjacent)

Option B adds **zero new reopen code**. The coherence argument:

1. `reopenJobForAttempt` already retires the superseded target session and
   any open interrogations against it *first*, while the old session is
   still resolvable (`revision_routing.go:286-293` →
   `closeInterrogationTargetForReopen`, `interrogation.go:566`), with
   `close_reason = revision_reopened`. C-095-2's fresh-attempt rule is
   untouched.
2. `resetDownstreamForRevision` (`revision_routing.go:402`) re-blocks every
   *transitive downstream* terminal job. The §1 validation rule that every
   explicit consumer is **reachable downstream of its target** is exactly
   what guarantees the cascade reaches every declared consumer — for ACE:
   `convener_draft → convener_synthesis → cross_examiner_* →
   cross_exam_synthesis → … → adjudicate`. Re-blocked consumers are
   non-terminal again, so the predicate (§4) holds the window open *for the
   fresh attempt's session* the moment the re-run `convener_draft` completes
   and emits a new `session.awaiting_interrogation` event
   (`lifecycle.go:1044-1051`). `interrogableTargetSessionForJob` orders by
   `event_id DESC` (`interrogation.go:495-501`), so resolution lands on the
   fresh session, never the retired one. No resurrection path exists.
3. Attempt scoping (C-095-3) is unaffected: the consumer relation is keyed by
   `workflow_job_id` against the stable job row whose `attempt` column bumps
   in place (`revision_routing.go:380-387`); leases, messages, artifacts, and
   verdicts keep their existing attempt validity rules.

A stale target session therefore retires through exactly **four doors**, all
existing or extended-existing code, each with a distinct `close_reason`:

| Door | Mechanism | Reason |
|---|---|---|
| consumer set drains | §5 choke point → `maybeCloseInterrogationTarget` | `interrogation_window_closed` |
| revision reopen | `closeInterrogationTargetForReopen` | `revision_reopened` |
| recovery sweep backstop | `closeLeakedInterrogationWindow` | `interrogation_window_closed` |
| run teardown (cancel/fail/complete) | `closeRemainingSessions` | `run_canceled` / `run_failed` / run-complete reason |

The fixture in §8 asserts the first two explicitly and the absence of any
fifth (no session left `active` at cell end).

## 8. The RFC 0105 fixture that proves ACE can graduate (panel Q5)

Home: `go/pkg/adapterconformance/ace_interrogation_test.go`, on the existing
`Harness` (in-process daemon, production handlers, fake agent —
`harness.go:98`), following the `lifecycle_revision_test.go` /
`falsification_gate_test.go` cell pattern. PG-gated under `make check` per
C-105-1/2.

**Genuineness rule (C-106-1, anti-isomorphism discipline):** the run is
seeded from the **production generator output** —
`compileAdjudicatedConstraintExtraction` (`generate.go:899`) with the
smallest legal parameters (2 cross-examiner postures, `max_cycles = 1` for
cell 1, `2` for cells 2–3) — not from a hand-built lookalike graph. The
generator change this RFC ships (each `cross_examiner_N` declaring
`interrogation_targets: [{"workflow_job_id": "convener_draft", "required": true}]`)
is therefore exercised by the same bytes production runs use.

### Cell 1 — `TestACEExplicitConsumersHappyPath` (RFC 0112 AC 2)

Drive through production handlers:

1. survey phase completes; `convener_draft` completes via `work.complete` →
   assert the session enters `awaiting_interrogation` (event present, session
   active).
2. `convener_synthesis` records an accepting verdict → **assert the target
   session is still active** (the early-close regression core; this single
   assertion fails on today's main).
3. Claim `cross_examiner_1` → assert
   `context.interrogation_targets[0]` is `state: available` with the correct
   live session id (AC 6).
4. Each examiner runs one **genuine** `open → ask → answer → close`
   round-trip against the convener session (≥ 1 interrogation per
   cross-examiner, the AC 2 floor).
5. `cross_examiner_1` completes → **assert window still open** (examiner 2
   pending — per-consumer independence).
6. `cross_examiner_2` completes → **assert target session closed with
   `close_reason = interrogation_window_closed`** (the leak-side regression:
   this is the `work.complete` hook call, which does not exist today).
7. Join, adjudication clears, run completes within budget; **assert no
   session remains active** (C-105-2 leak budget).

### Cell 2 — `TestACEExplicitConsumersRevisionReopenFreshWindow` (AC 3)

Drive to `adjudicate`, record `needs_revision` → assert: prior target session
closed `revision_reopened` with zero open interrogation rows against it;
cross-exam fan-out and join re-blocked; fresh `convener_draft` attempt
completes into a **new** awaiting session (different session id); a re-run
examiner's packet resolves the fresh id as `available`; a genuine
interrogation lands against the fresh session; cycle 2 clears and the run
completes. Asserts §7 end-to-end.

### Cell 3 — `TestACEExplicitConsumersDeadLaneDuringReCascade` (AC 4)

Within cell 2's re-cascade, hard-kill the lane of one reopened cross-examiner
(the `TestRevisionLifecycleLaneDeathSelfRecovers` fault pattern,
`lifecycle_revision_test.go:271`). Assert: the production sweep
(`HandleRecoveryAuto` → `recoverStuckJobs`) requeues that branch on the
**same attempt** while the join stays blocked and the convener window stays
open (dead examiner ≠ terminal examiner); a fresh lane claims, interrogates,
completes; the run finishes or escalates loudly within budget.

### Cell 4 — compatibility (AC 5)

The existing direct-dependent interrogating-panel suites
(`interrogation_test.go`, `interrogation_deadlock_test.go`, the panel cells
in `adapterconformance`) run **unmodified** — zero fixture edits is itself
the assertion. One new micro-cell, `TestNoTargetsPacketUnchanged`, asserts a
no-declaration packet contains no `context.interrogation_targets` key.

Unit-level companions in `workflowauthoring`: validation accepts the ACE
shape and rejects missing / self / non-interrogable / unreachable targets
(AC 1), plus the §1 lint warnings.

Graduation itself stays out of scope (C-106-1), but these are the cells an
RFC 0106 graduation later cites as genuine, non-isomorphic coverage.

## 9. Risks, migration, backward compatibility

- **Zero schema change.** The consumer relation derives from the immutable
  snapshot; no migration, no owner-applied DDL (RFC 0079 §5 friction class
  avoided entirely), nothing for v2→v3-style auth chains to track.
- **Old runs and old snapshots.** A snapshot with no declarations makes the
  new predicate half vacuously empty — bit-identical behavior. Existing
  tests pass untouched (AC 5).
- **In-flight ACE runs at deploy** keep their frozen snapshots (#115: a
  running run uses its snapshot, not live workflow JSON) and keep failing at
  the gate until **re-prepared** with the updated generator. The migration
  note in the release changelog must say "re-prepare ACE runs"; no daemon
  code can fix a frozen snapshot, and pretending otherwise would be
  dishonest.
- **Predicate cost.** Each terminal transition adds one snapshot fetch + JSON
  scan inside an already-open transaction — the identical cost profile
  `jobIsInterrogable` pays on every `work.complete` today. Runs have tens of
  jobs, not thousands; no index or cache is warranted until a measured need
  exists.
- **Choke-point refactor breadth.** Rows 1–13 of §5.2 each swap an inline
  UPDATE for one helper call — mechanical, reviewable per-site, and guarded
  three ways (AST guard, sweep backstop, fixture cells). The risk of *not*
  centralizing — the next terminalizing path silently bypassing the hook —
  is the documented second half of this RFC's failure and strictly worse.
- **`waiting_human` one-way door** (§5.2): a resumed consumer can find the
  window gone. Accepted, evidenced (§2 events), and identical to existing
  panel behavior under #131; the alternative (excluding `waiting_human` from
  the terminal set) would hold windows open indefinitely on human time,
  which is the leak class again.
- **Event-derived session resolution.** Target resolution rides
  `session.awaiting_interrogation` events ordered by `event_id DESC`; a
  reopen inserts the fresh event after the retire, so ordering is
  transactionally safe. This is the existing resolution mechanism
  (`interrogation.go:494`), not new machinery.

## 10. Rejected alternatives

1. **Fake `convener_draft → cross_examiner_*` edges.** Disallowed by the
   brief §3 and RFC 0112 non-goals; conflates scheduling truth with liveness
   concern; silently grows the revision reset set.
2. **Materialized `interrogation_consumers` table at prepare.** A second
   source of truth requiring coherence maintenance across re-prepare and
   revision cascades, plus owner-migration friction — for a relation that is
   a pure function of the snapshot. Derive, don't store (§4).
3. **Hard `required` gate in V1.** Creates a new wedge class on every
   legitimate window-retirement path (§2); deferred as a V2 predicate flip
   over V1's durable evidence.
4. **Per-call-site patching without a choke point.** Patches the two visible
   paths and re-arms the "too narrow hook" failure on the next terminalizing
   path; the §5.2 inventory shows seven missing sites *today*, which is the
   empirical case against ad-hoc coverage.
5. **Window keepalive via lane heartbeats or idle timeouts.** Makes lane
   behavior authoritative over window state, violating C-082-3; timeouts
   trade the early-close for a nondeterministic close.
6. **Runtime consumer registration RPC** (a lane declaring "I will
   interrogate X"). New RPC family — forbidden non-goal; also moves a static
   workflow fact into runtime mutable state.
7. **V1 cap of one target per consumer.** No lifecycle simplification gained
   (the machinery is set-based either way); forecloses real near-term shapes;
   replaced by an honest lint visibility line (§3).
8. **Widening the direct-dependent predicate to transitive downstream jobs
   implicitly.** No declaration needed, but every downstream job in the run
   would hold every upstream window open — windows would routinely survive to
   run end, the leak class generalized. Explicit declaration is the point.

## 11. Implementation envelope (honest scope accounting)

This panel's downstream jobs write only under
`docs/operator/artifacts/rfc-0112-explicit-interrogation-consumers/design_panel/`.
**The implementation cannot land inside that envelope.** The real change
set, to be scheduled as its own scoped work after the panel's decision:

| Area | Files | Change |
|---|---|---|
| Validation + lint | `go/pkg/workflowauthoring/{workflow,phases,lint}.go` | `ValidateInterrogationTargets`, lint warnings (§1) |
| Generator | `go/pkg/workflowgenerate/generate.go` | ACE cross-examiners declare targets (§8) |
| Predicate + hook | `go/pkg/mutations/interrogation.go` | §4 predicate union; §5.1 generalized hook; §2 events |
| Choke point + call sites | `go/pkg/mutations/{lifecycle,review,operator,recovery,recovery_auto_finalize,run}.go` | `markJobTerminal` + rows 1–13 swaps (§5.2) |
| Guard | `go/pkg/mutations/terminal_release_guard_test.go` | §5.3 AST guard |
| Packet | `go/pkg/mutations/claim.go` | §6 projection in `buildPacket` |
| Fixtures | `go/pkg/adapterconformance/ace_interrogation_test.go` (+ `workflowauthoring` unit tests) | §8 cells |
| Docs | `docs/reference/spec.md`, `docs/reference/ubiquitous-language.md` | explicit-consumer field, projection contract, "interrogation consumer" term (RFC 0112 AC 7) |

No new RPC methods, so `docs/reference/command-authority-matrix.md` and the
authority guardrail tests are untouched; the MCP tool surface is unchanged.
If any path outside this table proves necessary during implementation, that
is a scope change to surface explicitly — not to absorb silently.
