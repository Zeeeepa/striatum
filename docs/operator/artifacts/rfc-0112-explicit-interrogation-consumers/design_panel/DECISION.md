---
schema_version: striatum.decision.v1
decision_id: "dec_84e8f185604900a12982e453246fdfd1"
run_id: "run_c57e270528b569e2c53c2befec8c3b82"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "RFC 0112 design panel recommendation: adopt the arbitrated lifecycle-first plan with dissent amendments"
created_at: "2026-06-05T15:45:57Z"
---

# RFC 0112 design panel recommendation: adopt the arbitrated lifecycle-first plan with dissent amendments

author: principal-decider-claude-opus-4.8-001
date: 2026-06-05

Decision ID: `dec_84e8f185604900a12982e453246fdfd1`
Run ID: `run_c57e270528b569e2c53c2befec8c3b82`
Outcome: `accepted_with_follow_up`

> **Scope of this artifact.** This records the design panel's recommendation
> and the required follow-up work. It is **not** an RFC acceptance decision.
> It does not change RFC 0112 status, `docs/decisions/decision-log.md`,
> `docs/rfcs/README.md`, `docs/reference/spec.md`,
> `docs/reference/ubiquitous-language.md`, source code, or VERSION. A later
> operator decision must make any formal acceptance change.

## 1. The recommendation

Adopt the arbitrated plan in `ARBITRATOR_SYNTHESIS.md` — Proposal B's
lifecycle-first skeleton, amended by Proposal A's attempt-aware packet
projection and Proposal C's fixture-rigor requirements — **with three additive
amendments from `DISSENT_REVIEW.md`** (Findings 2, 3, 4; Finding 1 is resolved
against the dissent, see §2.1). The plan fixes both halves of the failure
named in `PROBLEM_BRIEF.md` §1:

- **No early close.** The pending-consumer predicate becomes the union of
  direct dependents and snapshot-declared explicit consumers
  (`interrogation_targets`), so the convener window survives the
  `convener_synthesis` phase gate into the cross-examiner fan-out.
- **No leak.** Every terminal consumer transition is routed through one
  lifecycle choke point (`markJobTerminal` in `go/pkg/mutations`) that invokes
  the generalized `releaseInterrogationTargetsForTerminalConsumer`, with an
  AST/static guard so future terminalizing paths cannot silently bypass it.

Load-bearing properties the panel converged on unanimously, which this
recommendation freezes:

- The consumer relation is **snapshot-derived, never stored** — a pure
  function of the run's frozen `workflow_snapshots.workflow_json` plus live
  `jobs` rows, isolated behind one resolver helper. No new table, no
  migration, no new RPC family, no fake dependency edges, no transcript
  capture, D028 intact (brief C-082-1/-2, §3).
- Zero behavior change for workflows that declare no `interrogation_targets`;
  old frozen snapshots are bit-identical in behavior (AC 5).
- Revision-reopen correctness needs **zero new reopen code**: it falls out of
  existing RFC 0095 machinery plus the reachable-downstream validation rule
  (ledger L9).
- ACE support-tier graduation stays **out of scope** (RFC 0112 non-goal,
  C-106-1); this plan only makes the graduation fixture possible.

**Readiness verdict:** the plan is implementation-ready and is **ready for a
later operator acceptance decision once the RFC edits in §3 land**. All three
scorecards and the dissent review returned `accept_with_findings`; no panel
artifact rejects the architecture. The honest rejection surface the ledger
identified (the choke point and the union predicate) was endorsed by every
scorecard.

## 2. Resolution of the dissent findings

### 2.1 Finding 1 (cap V1 at one target) — **rejected; uphold N ≥ 1**

The arbitrator set the L3 cardinality dial to allow `N >= 1` targets with a
lint warning above three. The dissent recommends capping at `len = 1`. The
dial stays at **N ≥ 1**, for the reasons the evidence majority already
carries (2 of 3 proposals, plus SCORECARD_B's endorsement):

- The machinery is set-based end to end — predicate, hook loop, packet state,
  and evidence events are all per `(consumer, target)` entry. A cap removes
  no lifecycle code; it is a validation rule that polices schema without a
  lifecycle benefit (Proposal B §10.7).
- The dissent's premise that "no near-term shape needs more than one target"
  is contradicted by named, concrete candidates: the two-draft falsification
  variant and cycle-≥2 ACE interrogating `revision_draft` (ledger L3).
- The mixed-state semantics the dissent worries about are not future design
  work — they are already fully specified (Proposal B §3, per-entry
  state/instruction/event) and are cheap review, not new design.

**Binding condition that discharges the dissent's underlying concern:** the
multi-target mixed-state surface ships **tested in V1, day one** — the focused
validation/mutation tests must include multiple targets in different states,
per-entry `required_skipped` events, and the >3-target lint (already in the
synthesis test sequence). If those tests are descoped, the cap question must
be reopened before acceptance.

### 2.2 Finding 2 (toothless advisory `required`) — **accepted as an RFC edit**

`required: true` remains advisory in V1 (unanimous panel position; a hard gate
re-creates the #84/#65 wedge family under revision reopen). The dissent's ask
— a concrete, pre-declared V2 graduation path — is accepted and becomes a
required RFC edit (§3, item 2): document the V2 hard gate as a pure predicate
flip over V1 evidence, refusing `work.complete` only when a required target's
window is still live (`active`/`awaiting_interrogation`) for the consumer's
attempt and the consumer holds neither an interrogation row nor an
`interrogation.unavailable_signaled` event. No V1 enforcement code.

### 2.3 Finding 3 (skipped targets project as `not_ready` forever) — **accepted**

The projection helper must handle terminal target-job states explicitly: a
target job in `skipped` state projects `state: unavailable` with
`reason: target_skipped` (never an indefinite, misleading `not_ready`). This
folds into the L6 projection contract (§4, slice 3) and the RFC packet
contract section (§3, item 4).

### 2.4 Finding 4 (fixtures don't assert evidence events) — **accepted**

The fixture cells must assert, in the database, that
`interrogation.unavailable_signaled` is appended on an actual
`interrogation.open` refusal and `interrogation.required_skipped` is appended
when a consumer terminalizes having skipped a required target. Without these
assertions the advisory-evidence mechanism (and the V2 hard-gate path that
depends on it) is unverified. Folds into the fixture contract (§5).

## 3. Required edits to RFC 0112 before acceptance

RFC 0112 as proposed is directionally right but under-specified relative to
the panel's resolved answers. The following edits are **required before a
later operator acceptance decision** (RFC text only; no status change in this
panel):

1. **Resolve OQ 2 (cardinality):** multiple targets allowed (`N >= 1`),
   per-entry lifecycle/state/event semantics per Proposal B §3, lint warning
   above three targets.
2. **Resolve OQ 1 + OQ 3 (`required` + durable records):** advisory in V1
   with the two-event evidence scheme — `interrogation.unavailable_signaled`
   on actual `interrogation.open` refusal, `interrogation.required_skipped`
   at the terminal choke point; packet projection writes **nothing**; the V2
   hard gate pre-declared as the predicate flip in §2.2 above.
3. **Terminal choke point, exact paths, and guard:** name `markJobTerminal`,
   the full terminal-path inventory (work.complete; `work.block` →
   `waiting_human`; all review.verdict/submit-review outcomes including
   absorbed needs-revision, reject, and human-checkpoint; override-verdict;
   checkpoint/escalation resolve; recovery auto-publish, validated-output
   completion, auto-finalize; recovery.cancel_job including cascades), the
   AST guard with the two-row structural allowlist (`run.cancel`,
   run-failure finalization, both covered by `closeRemainingSessions`), and
   the stated RFC 0104 per-run lock serialization assumption. Credit the
   end-of-run fixture invariant — **not** the recovery sweep — as the
   backstop for the all-consumers-terminal leak class.
4. **Attempt-aware packet projection:** `context.interrogation_targets[]`
   fields `workflow_job_id`, `required`, `state`, `target_session_id`,
   `target_attempt`, `reason`, daemon-authored `instruction` mirroring the
   `interrogation.open` signal text. State resolution is attempt-aware:
   `available` = live awaiting session for the target's current attempt
   (reason null); `unavailable` = an awaiting session for the relevant
   attempt existed but is retired (retired session id kept for evidence
   linkage, reason = session close_reason verbatim) **or the target job is
   `skipped`** (`reason: target_skipped`, per §2.3); `not_ready` = no
   awaiting session for the current attempt (after reopen,
   `reason: revision_reopened`, never a falsely-exposed stale session).
   `attempt` added additively to `session.awaiting_interrogation` event
   payloads; legacy events default to attempt 1.
5. **Precise self/chained rule (SCORECARD_A F5):** self-reference is a hard
   error, and V1 hard-errors `interrogation_targets` on any job that itself
   declares `interrogable: true` — no chained interrogable consumers in V1;
   relax only when a real shape needs chaining (deferred, §6).
6. **Fixture hardening requirements** (so AC 2–4 reflect the contract in §5):
   preserved-context assertions, `waiting_human` block/resolve cell, evidence-
   event assertions (§2.4), end-of-run no-awaiting-session invariant,
   concurrency pin, named negative-validation tests, and seeding from
   production generator output.
7. **Documentation homes:** record that implementation updates
   `docs/reference/spec.md` (workflow field + packet contract,
   `striatum.work-packet.v1` impact) and
   `docs/reference/ubiquitous-language.md` ("interrogation consumer") — after
   implementation, not in this panel.
8. **Migration note (#115 class):** in-flight ACE runs prepared before the
   field exists keep their frozen snapshots; they must be re-prepared after
   implementation, never silently rewritten.

## 4. Implementation slices and verification gates

Implementation lands as **separately scoped follow-up work** — it touches
`go/pkg/{workflowauthoring,workflowgenerate,mutations,adapterconformance}`
and `docs/reference/*`, all outside this panel's frozen write scope (brief
criterion 9). Recommended slices, each independently verifiable:

| # | Slice | Content | Gate |
|---|-------|---------|------|
| 1 | Validation + lint | `ValidateInterrogationTargets` beside `ValidatePhaseShapes` (hard errors: missing target, not `interrogable: true`, self-reference, interrogable declarer, not reachable-downstream, duplicates); lint: unknown entry fields, lane lacks `interrogate`, redundant direct dependency, >3 targets | Named negative-validation unit tests in `workflowauthoring` (missing/self/non-interrogable/unreachable/duplicate/interrogable-declarer); absent-array and malformed-entry tests |
| 2 | Resolver + predicate + choke point | One snapshot-derived resolver helper (consumed by predicate, hook, and packet builder; never materialized); widen `interrogationConsumersPending` to direct ∪ declared; `markJobTerminal` + generalized `releaseInterrogationTargetsForTerminalConsumer`; mechanical swaps across the full terminal-path inventory; the two V1 evidence events | AST guard test (`terminal_release_guard_test.go` shape) with the two-row allowlist; focused mutation tests incl. direct-plus-explicit dedupe, multiple targets mixed states, required-skip and unavailable events; terminal→terminal transitions no-op-safe |
| 3 | Packet projection | Attempt-aware `context.interrogation_targets[]` in `buildPacket`, nullability + reason rules per §3 item 4 incl. `target_skipped`; `attempt` in awaiting-event payloads | Projection-state unit tests (available/unavailable/not_ready/skipped, mid-reopen); `TestNoTargetsPacketUnchanged` |
| 4 | ACE generator | Every generated `cross_examiner_N` declares `{workflow_job_id: convener_draft, required: true}`; graph unchanged, no fake edges | Generator output snapshot test; phase validation still passes |
| 5 | Conformance fixtures | The §5 fixture contract in `go/pkg/adapterconformance/ace_interrogation_test.go` | All §5 cells green through production handlers; existing direct-dependent suites pass **unmodified** |

Cross-slice gates before any acceptance/graduation discussion: `make test`,
`make typecheck`, `make lint`, and the PG-gated `make check`/`make smoke`
path. Slices 1–4 may land together or separately; slice 5 must land before
any RFC 0106 tier discussion cites it.

## 5. RFC 0105 fixture acceptance contract (for later ACE graduation)

The fixture is `go/pkg/adapterconformance/ace_interrogation_test.go`, seeded
from **production generator output** (`compileAdjudicatedConstraintExtraction`,
smallest legal parameters) — never a hand-built graph — driving production
handlers with the fake agent, PG-gated under `make check` (C-105-1/-2,
anti-isomorphism per RFC 0106/D169). Required cells and assertions:

1. **Happy path** (`TestACEExplicitConsumersHappyPath`): window survives the
   `convener_synthesis` gate; survives the first consumer's completion;
   closes exactly once on the last consumer with
   `interrogation_window_closed`; each cross-examiner opens ≥1 genuine
   interrogation; **preserved-context assertion** — seed a convener-only fact
   (never written to any artifact), interrogate for it, fail if the answer is
   derivable from artifacts alone (mirrors the RFC 0082 intention test).
2. **Revision reopen** (`…RevisionReopenFreshWindow`): `adjudicate`
   needs_revision reopens `convener_draft`; prior target session and open
   interrogations retire (`revision_reopened`); fan-out and join re-block;
   the **fresh attempt's** session is interrogated on the next cycle with
   preserved-context assertion repeated; packet projection during the reopen
   window shows `not_ready`/`revision_reopened`, never a stale retired
   session.
3. **Dead lane during re-cascade** (`…DeadLaneDuringReCascade`): hard-dead
   lane injected into a reopened cross-examiner; recovery sweep requeues that
   branch **same-attempt**; join remains blocked; target window stays open
   throughout; a fresh lane completes; the run completes or escalates loudly
   within budget.
4. **Compatibility floor**: `TestNoTargetsPacketUnchanged` plus all existing
   direct-dependent interrogation suites passing unmodified (AC 5).

Mandatory cross-cell assertions:

- **Evidence events recorded in the DB** (dissent F4):
  `interrogation.unavailable_signaled` on an actual open-refusal;
  `interrogation.required_skipped` on terminalizing past a required target.
- **`waiting_human` door** (SCORECARD_A F6a): a consumer exits via
  `work.block` → `waiting_human` and the window state is correct across
  block → resolve.
- **End-of-run invariant** (the honest leak backstop): no session remains in
  `awaiting_interrogation` at any cell's end.
- **Concurrency pin** (ledger L8): two consumers terminalizing back-to-back
  close the window exactly once; pins the RFC 0104 per-run lock assumption so
  a future locking relaxation fails loudly.

This contract is what a later RFC 0106 graduation decision may cite as
genuine, non-isomorphic coverage. **Graduation itself is a separate, later
decision and is not pre-approved here.**

## 6. Deferred questions (must remain out of V1)

1. **Hard-gating `required: true`** (OQ 1): V2-only, as the pre-declared
   predicate flip in §2.2. No V1 enforcement code.
2. **Conversation/floor-control consumers** (OQ 4): explicitly out; the
   explicit-consumer mechanism stays interrogation-scoped until a
   conversation shape presents a real need.
3. **Chained interrogable consumers** (§3 item 5): an `interrogable: true`
   job declaring its own `interrogation_targets` stays a hard error until a
   real shape needs chaining.
4. **Snapshot-query caching/profiling**: no cache in V1; the per-terminal
   lookup matches the cost profile `jobIsInterrogable` already pays. Profile
   before optimizing (SCORECARD_B minor 2).
5. **Cardinality revisit clause**: if V1's multi-target mixed-state tests are
   descoped for any reason, the §2.1 cap question reopens before acceptance.

## 7. Follow-up checklist (the `follow_up_required` items)

1. Land the §3 RFC 0112 edits (RFC text only).
2. Operator acceptance decision on the revised RFC 0112 (decision log + RFC
   status — outside this panel).
3. Implementation slices 1–5 (§4) as separately scoped work with their gates.
4. Post-implementation doc updates: `docs/reference/spec.md`,
   `docs/reference/ubiquitous-language.md`.
5. Re-prepare (never rewrite) any in-flight pre-field ACE runs (#115 class).
6. Only after slice 5 is green: a separate RFC 0106 ACE graduation
   discussion citing §5 as its evidence.

## 8. Inputs

- `PROBLEM_BRIEF.md` (problem framing, constraints, decision criteria)
- `proposals/option_a/PROPOSAL_A.md`, `option_b/PROPOSAL_B.md`,
  `option_c/PROPOSAL_C.md`
- `scorecards/SCORECARD_A.md`, `SCORECARD_B.md`, `SCORECARD_C.md`
- `TRADEOFF_LEDGER.md` (normalized evidence, L1–L9)
- `ARBITRATOR_SYNTHESIS.md` (selected plan)
- `DISSENT_REVIEW.md` (`accept_with_findings`, Findings 1–4 resolved in §2)
- `docs/rfcs/0112-explicit-interrogation-consumers.md` (proposed text the §3
  edits apply to)
