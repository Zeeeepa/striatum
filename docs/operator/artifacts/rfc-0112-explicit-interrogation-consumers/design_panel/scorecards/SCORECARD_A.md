---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept_with_findings
severity: medium
tags:
  - rfc-0112
  - ace
  - interrogation-consumers
  - scorecard
  - option-a
---

# Scorecard — Proposal A (Minimal Explicit Interrogation Consumers)

author: scorekeeper-claude-opus-4.8-001
date: 2026-06-05
run: run_c57e270528b569e2c53c2befec8c3b82
workflow_job: score_option_a
posture: custom:maintainability

## Verdict

**accept_with_findings.** Proposal A is the right architecture: snapshot-declared
consumers, predicate union, a generalized terminal-consumer hook, attempt-aware
packet projection, no fake edges, no new table, no new RPC family, no owner-table
DDL. Every source anchor it cites checks out against current code. It is one
revision pass away from implementation-ready: its terminalizer enumeration —
the dimension it itself names "the largest correctness risk" — misses one real
production path (`work.block` with `severity=human_checkpoint`), and its guard
against future missed paths is enumeration-bound, which is exactly the guard
style the problem brief's criterion 2 warns against. Both are bounded, concrete
fixes that do not disturb the design.

## Dimension scores

| Dimension | Score (1–5) | Summary |
|---|---|---|
| Maintainability | 4.5 | Smallest mechanism that fits: extends the existing predicate (`interrogationConsumersPending`, `go/pkg/mutations/interrogation.go:516`) and hook rather than parallel machinery. Snapshot-as-declaration matches the frozen-snapshot runtime model. Minor cost: snapshot JSON parsing in the close path, acknowledged and acceptable. |
| Migration risk | 5 | No owner-table DDL at all (avoids the daemon-migrates-as-runtime-role crash-loop class). Event-payload `attempt` is additive. In-flight pre-field runs are explicitly declared old-and-restartable rather than silently rewritten. |
| Reversibility | 4.5 | The relation lives in workflow JSON + code paths; removing the field reverts behavior to direct-dependent-only. The V1 array-capped-at-one shape buys V2 multi-target without churn. |
| Operator legibility | 4 | `available`/`unavailable`/`not_ready` + `reason` + `instruction` in the packet means a lane never burns a state-changing call to discover absence; #131 non-wedging signal preserved. Gap: whether a packet-surfaced `unavailable` leaves a durable record (brief Q2 / RFC 0112 OQ3) is answered only for the deferred hard-gate slice, not for V1 (F3). |
| Lifecycle correctness | 3.5 | Predicate union, reopen behavior (`closeInterrogationTargetForReopen` at `interrogation.go:566`, invoked from `revision_routing.go:291`; transitive downstream reset reaches the fan-out through ordinary edges), and attempt-aware projection are all correct and verified against source. Held back by F1 (missed terminalizer) and F2 (enumeration-bound guard); the concurrency serialization assumption is unstated (F4). |
| Fixture rigor | 4 | Three cells map one-to-one onto RFC 0112 AC 2–4, drive production handlers with the fake agent, include genuine ask/answer turns per cross-examiner, and the two-posture economy is a good budget call. Gaps: no waiting_human consumer-exit cell, no end-of-run no-leaked-window invariant assertion, negative-validation tests not named (F6). |

## The six panel questions

| Q | Answered? | Notes |
|---|---|---|
| 1. Field name/shape | Mostly | Shape, defaults, unknown-field lint, duplicate rejection, V1 cap: all answered with rationale. Missing: where the field lands in the workflow schema docs and whether `striatum.workflow.v1.x` bumps (F5). |
| 2. `required` semantics | Partially | Advisory-in-V1 with the wedge rationale (a hard gate needs a proof model that does not exist) is the right call, and the deferred hard-gate sketch is concrete. OQ3 — durable record when `unavailable` is *surfaced in a packet* — is unanswered for V1 (F3). |
| 3. Multiple targets in V1 | **Yes, completely** | Cap at one, array shape for V2, explicit refusal to invent partial-order semantics now. Capping makes the "several targets in different states" sub-question correctly moot. |
| 4. Terminal paths | Substantially, with a real gap | The enumeration covers 12 of the 13 terminal-state writers in `go/pkg/mutations` (verified by sweeping every `UPDATE striatumd.jobs` that sets a state in the terminal set). It misses `HandleBlockWork` → `waiting_human` (F1), and the proposed guard cannot catch a *future* unenumerated path (F2) — the brief asks for exactly that guard. |
| 5. RFC 0105 fixture | **Yes, completely** | Cells, fault injection, assertions, and location (`go/pkg/adapterconformance/ace_interrogation_consumers_test.go`) all named; graduation flip correctly kept out of scope (C-106-1). Minor hardening in F6. |
| 6. Packet namespace | Yes | `context.interrogation_targets[]` with resolved `target_session_id`, three-state enum, `reason`, `instruction`, and attempt-aware resolution. Minor: the `striatum.work-packet.v1` contract-doc impact is not called out (fold into F5). |

## Findings

### F1 (high) — Missed terminalizer: `work.block` with `severity=human_checkpoint`

`HandleBlockWork` (`go/pkg/mutations/lifecycle.go:908`) sets a job to
`waiting_human` when a lane blocks with `severity=human_checkpoint`. That state
is in `terminalInterrogationConsumerStates` (`interrogation.go:373`), so the
blocking consumer leaves the pending set — but the proposal's hook list covers
only the *review-side* `waiting_human` (`openHumanCheckpoint`,
`review.go:959`) and the two `checkpoint.resolve` outcomes
(`operator.go:306`/`426`), not the `work.block` entry path. If the **last**
pending cross-examiner blocks on a human checkpoint, nothing triggers the close
evaluation: the convener's preserved-context session sits in
`awaiting_interrogation` even though the predicate would report zero pending
consumers.

Bounded consequence, but real: an `escalation_inbox` row exists (loud, not a
silent wedge), and the window heals when the operator resolves the checkpoint
*because the proposal does hook both `checkpoint.resolve` branches*. Still, the
window state machine is violated for the whole block-to-resolve interval, a
live preserved-context lane outlives its contract, and — decisively — the
proposal's own table-driven guard "drives each terminalizer above," so the
omission propagates into the guard itself. The proposal's risk section says
"the largest correctness risk is a missed terminalizer"; it shipped with one.

### F2 (medium) — The missed-path guard is enumeration-bound

Brief criterion 2 and question 4 ask "what guard keeps a future terminalizing
path from bypassing it." A table-driven mutation test over an enumerated list
catches regressions in covered paths only; a *new* terminal transition added
next quarter bypasses both hook and guard silently. Two acceptable shapes,
either of which resolves this:

- **Choke point:** route every jobs-table terminal-state write through one
  shared `terminalizeJob(...)` helper that fires the release hook, plus a
  static guard test asserting no `UPDATE striatumd.jobs` outside that helper
  sets a terminal state (the same grep-guard discipline the authority-matrix
  tests use).
- **Invariant guard:** an end-of-cell assertion in the RFC 0105 fixture (and
  ideally in the recovery sweep) that no active session sits in
  `awaiting_interrogation` with zero pending consumers.

Note the existing recovery backstop does **not** cover the gap: CASE 3 of
`recoverStuckJobs` (`recovery_decision_tree.go:271`) only scans *unfinished*
jobs (`claimed`/`running`/`stale_lease`). In the F1 leak scenario every
consumer is terminal and the join is `blocked`, so no scanned row reaches the
leaked window. The plan should state this rather than implicitly relying on a
backstop that cannot fire.

### F3 (medium) — OQ3 unanswered for V1

Does projecting `state: "unavailable"` into a packet write a durable event, or
is a durable record produced only when the lane actually calls
`interrogation.open`? The proposal answers durable-record questions only inside
the deferred hard-gate sketch. The cheapest honest V1 answer (project-only in
packets; durable event only on the actual `interrogation.open` refusal; durable
unavailable/waiver records deferred to the hard-gate RFC) is likely right — but
it must be stated, because the hard-gate design depends on it.

### F4 (medium) — Concurrency serialization assumption unstated

Two cross-examiners terminalizing concurrently must not each observe the other
as still-pending (neither closes → leak) or double-close. The plan presumably
inherits serialization from the RFC 0104 per-run lock on mutation paths, but it
never says so. State the assumption explicitly and pin it with a fixture
assertion, so a future relaxation of run-level locking cannot silently break
close ordering.

### F5 (low) — Schema/doc homes not named

Where `interrogation_targets` is documented in the workflow authoring schema,
whether `striatum.workflow.v1.x` bumps, and the `striatum.work-packet.v1`
contract-doc impact of the new `context` block are all unstated (brief Q1/Q6
ask directly). Also make RFC 0112's "rejected on the target job itself" rule
precise: the proposal rejects self-reference; explicitly state whether an
`interrogable: true` job may itself declare `interrogation_targets` (chained
windows) in V1.

### F6 (low) — Fixture hardening

(a) Add a cell (or assertion) where a consumer exits via `waiting_human`
(`work.block` path — ties to F1) and the window closes correctly once the
checkpoint resolves. (b) Add the end-of-run invariant "no session remains in
`awaiting_interrogation`" — RFC 0105 C-105-2 counts a leaked window as a gate
failure, so assert it directly rather than only via run completion. (c) Name
the negative-validation tests for AC 1 (missing / self-referential /
not-interrogable / not-reachable targets), even if they live as
`workflowauthoring` unit tests rather than conformance cells.

### F7 (info) — Strengths the tradeoff ledger should carry

- **Attempt-aware packet resolution** (attempt in future
  `session.awaiting_interrogation` payloads, resolved against the target job's
  current attempt) catches a genuine revision-cycle bug class — a reopened
  consumer degrading to a false `unavailable` against a retired session — that
  a naive projection would ship. This is C-095-2 made concrete.
- **Cap-V1-at-one-target inside an array** is the best available answer to
  OQ2: ACE needs exactly one, and it defers multi-target ordering/partial
  availability semantics without API churn later.
- **Blast radius matches criterion 8 exactly:** no new RPC family, no new
  aggregate, no schema change beyond what consumer resolution needs, D028
  intact, direct-dependent workflows behaviorally unchanged.
- **Honest envelope accounting (criterion 9):** the proposal states up front
  that implementation touches `go/pkg/workflowauthoring`, `workflowgenerate`,
  `mutations`, `adapterconformance`, and `docs/reference/*` — outside this
  panel's frozen scope — instead of pretending the panel scope suffices.
- **Required-stays-advisory in V1** is correctly argued: a hard gate without a
  durable proof model would wedge replacement lanes that legitimately receive
  `panel_window_closed`, mixing the liveness fix with new adjudication
  semantics.

## Gaps that would wedge ACE or shallow graduation evidence

- **Wedge risk:** none outright — F1's leak escalates loudly
  (`escalation_inbox`) and heals at checkpoint resolution, so it is a
  state-machine violation and a lingering live lane, not a silent wedge. The
  unfixed-F2 scenario (a future unenumerated terminalizer) is the one that
  could produce a genuinely silent leak, and the recovery backstop provably
  cannot reach it (see F2).
- **Shallow-evidence risk:** without F6's no-leaked-window invariant and the
  `waiting_human` cell, the fixture proves the happy/revision/dead-lane paths
  but not the closure invariant itself — RFC 0106 graduation evidence would be
  genuine but narrower than the predicate's actual state space.
- Revision-cycle behavior, packet projection, and validation rules contain no
  gaps I could find: reopen semantics were verified against
  `revision_routing.go:291` + `interrogation.go:566`, the transitive reset
  reaches cross-examiners through ordinary edges, and the validation list
  satisfies brief criterion 4 including the redundant-direct-dependency lint.

## Concrete changes required for implementation-ready

1. Add `HandleBlockWork` (`severity=human_checkpoint` → `waiting_human`,
   `lifecycle.go:908`) to the terminal-path list, and re-derive the list by
   sweeping every `UPDATE striatumd.jobs` that writes a state in
   `terminalInterrogationConsumerStates` (current full set: `HandleCompleteWork`,
   `completeReviewJob`, `failReviewJob`, `openHumanCheckpoint`,
   `HandleOverrideVerdict`, `HandleCheckpointResolve` ×2, `cancelSingleJob`
   + cascade, `completeAutoRecoveredJob`, `completeRecoveredJob`,
   `completeAutoFinalizedJob`, `HandleRunCancel`, `HandleBlockWork`).
2. Replace or augment the enumeration-bound guard with a choke-point helper +
   static guard test, or a no-leaked-window invariant in the fixture and sweep
   (F2); explicitly note that `recoverStuckJobs` CASE 3 cannot reach this leak.
3. State the V1 answer to OQ3 (durable record on packet-surfaced
   `unavailable`: yes/no and why) (F3).
4. State the concurrency serialization assumption for concurrent consumer
   terminalization and pin it in the fixture (F4).
5. Name the workflow-schema and work-packet contract documentation homes and
   any schema-version bump; make the self/chained-declaration rule precise (F5).
6. Harden the fixture per F6 (waiting_human cell, end-of-run window invariant,
   named negative-validation tests).

None of these alter the proposal's architecture; all are additive to the plan
text and the fixture matrix.
