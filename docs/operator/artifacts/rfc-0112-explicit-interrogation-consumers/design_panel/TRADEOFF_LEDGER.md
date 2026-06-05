---
schema_version: striatum.findings_ledger.v1
artifact_kind: findings_ledger
summary_count: 9
title: RFC 0112 Tradeoff Ledger
run_id: run_c57e270528b569e2c53c2befec8c3b82
date: 2026-06-05
tags:
  - rfc-0112
  - ace
  - interrogation-consumers
  - tradeoff-ledger
---

# RFC 0112 Tradeoff Ledger — Explicit Interrogation Consumers

author: tradeoff-ledger-claude-opus-4.8-001

Inputs normalized: PROBLEM_BRIEF, PROPOSAL_A (minimal), PROPOSAL_B
(lifecycle-first), PROPOSAL_C (fixture-first), SCORECARD_A
(accept_with_findings, F1–F7), SCORECARD_B (accept_with_findings, 2 minor
findings), SCORECARD_C (accept_with_findings, 5 blocking gaps). Every row
cites its supporting and opposing evidence; nothing below is new
recommendation — only normalization of what the panel produced.

Verdict landscape: all three scorecards returned `accept_with_findings`. No
proposal was rejected; the panel converged on one architecture with **one
genuine fork** (L3, multi-target cardinality) and **one definitional split**
(L6, `not_ready` semantics). Everything else is union-compatible.

## Ledger entries

### L1 — Field name and JSON shape (panel Q1)

| Aspect | Evidence |
|---|---|
| Convergence | **Unanimous.** `interrogation_targets` array of `{workflow_job_id, required}` on the consumer job, exactly the RFC 0112 §1 shape. A (Workflow Field Shape), B (§1), C (Q1) all confirm; no scorecard opposes. |
| Validation rules | Converged set: target exists in same workflow; target declares `interrogable: true`; no self-reference; consumer reachable downstream of target via ordinary edges; duplicates rejected (A, B). B adds two beyond A/C: reject `interrogation_targets` on the target job itself (per RFC 0112 §1), and lint-warn when the declaring lane lacks `interrogate` capability. A adds the redundant-direct-dependency lint. Unknown entry fields = lint warning in V1 (all three; C's JSON schema uses `additionalProperties: true`). |
| Support | SCORECARD_A Q1 "mostly answered"; SCORECARD_B Q1 "complete"; SCORECARD_C Q1 "complete". |
| Opposition / gaps | SCORECARD_A F5 (low): doc home and `striatum.workflow.v1.x` version-bump question unstated in A; and the self/chained rule must be made precise — may an `interrogable: true` job itself declare `interrogation_targets`? Only B names doc homes (`docs/reference/spec.md` workflow schema section + `docs/reference/ubiquitous-language.md` "interrogation consumer" entry, B §1/§11). |
| Risk | Low. Shape is settled; residual risk is documentation drift (F5 class), not design. |
| Implementation consequence | `ValidateInterrogationTargets` as a sibling of `ValidatePhaseShapes` in `go/pkg/workflowauthoring` (B §1 names the hook points: `phases.go:43`, invoked from `mutations/run.go:1081`); lint additions in `workflowauthoring/lint.go`; spec + ubiquitous-language doc updates (RFC 0112 AC 7). All outside this panel's frozen write scope — implementation envelope honesty per brief criterion 9 (stated by A, B §11, and C). |

### L2 — Advisory vs hard-gated `required: true` (panel Q2, RFC 0112 OQ 1/OQ 3)

| Aspect | Evidence |
|---|---|
| Convergence | **Unanimous on the headline:** advisory packet-instruction strength in V1, never a hard completion gate; the non-wedging `interrogation_unavailable` fallback (#131) remains the runtime contract. A (Required Semantics), B (§2), C (Q2). |
| Strongest argument | B §2's lifecycle inversion argument: a hard gate couples the consumer's ability to terminalize to the *target's* liveness, and the window retires for legitimate reasons the consumer cannot remediate (revision reopen, recovery sweep, run cancel) — re-creating the #84/#65 wedge family RFC 0095 retired. A concurs (hard gate needs a durable proof model that does not exist). SCORECARD_B Q2 endorses ("deadlock risks under revision reopen cascades"). |
| The OQ 3 fork (durable records) and its resolution | A answers durable records only inside its deferred hard-gate sketch — SCORECARD_A F3 (medium) flags V1 OQ 3 as unanswered. B answers it fully: `interrogation.unavailable_signaled` event on the actual `interrogation.open` refusal + `interrogation.required_skipped` event at the terminal choke point when a `required: true` target was never interrogated; packet-build surfacing alone writes **nothing** (claims can repeat; projections must not mutate history). C independently agrees on the half it covers (durable event only on explicit `open`, named `interrogation.unavailable_observed`; packet surfacing writes nothing). The panel's resolved answer is therefore B's two-event scheme, which subsumes C's one event and discharges A-F3. |
| Deferred V2 path | Converged: hard gate becomes a pure predicate flip over the V1 evidence rows/events at `work.complete`/verdict time (A's sketch; B §2 makes it concrete as "interrogation row OR unavailable_signaled OR required_skipped"). |
| Risk | Medium if events are dropped from scope: V2 hard-gating becomes undesignable without V1 evidence (B §2), and "unanswered interrogation is evidence" (C-098-2) loses its assertable record. A poorly behaved lane can skip advisory interrogation in V1 (A Risks; accepted by all three — the RFC 0105 fixture proves the intended path instead). |
| Implementation consequence | Two curated, D028-clean events appended in `go/pkg/mutations/interrogation.go` (open-refusal path) and at the terminal choke point (L4); no packet-time writes; no enforcement code in V1. |

### L3 — One target vs multiple targets in V1 (panel Q3, RFC 0112 OQ 2) — **the genuine fork**

| Aspect | Evidence |
|---|---|
| Position: cap at one | A: keep the array shape but reject `len > 1` in V1 — ACE needs exactly one; capping avoids defining multi-target ordering, partial availability, and hard-gate interaction before a real workflow needs them. SCORECARD_A scores this "Yes, completely" answered and F7 calls it "the best available answer to OQ2" (cap makes the mixed-states sub-question correctly moot). |
| Position: allow N ≥ 1 | B §3: no cap — each (target, consumer) pair is lifecycle-independent, the hook loop is already plural over upstreams (`interrogation.go:535-557`), per-entry `required` evaluation, lint (not block) above three targets; names near-term shapes (two-draft falsification variant; cycle-≥2 ACE interrogating `revision_draft`). B §10.7 explicitly rejects the cap as "schema policing with no lifecycle benefit." C (Q3) also allows N. SCORECARD_B endorses ("plurality correctly supported with set-based hooks"). |
| Opposition to N | SCORECARD_C on C's version: "directionally complete" but must define consumer behavior when several required targets have mixed states — a gap **only B fills** (per-entry independent state/instruction/event). SCORECARD_A's praise of the cap is the counter-evidence to N. |
| Tally | Proposals: 2 (B, C) for N, 1 (A) for cap. Scorecards: B's endorses N; A's endorses cap; C's accepts N conditionally on mixed-state semantics (supplied by B §3). |
| Risk | Cap: a later validation change when a real shape needs several targets (A acknowledges; B argues the near-term shapes are real, not hypothetical). N: mixed-state packet/required semantics must ship day one — already written in B §3, so the marginal cost is review, not design. Either way the machinery is set-based; cardinality is a validation-rule dial, not an architecture choice. |
| Implementation consequence | This is the **one dial the arbitrator must set**. If N: adopt B §3 verbatim (per-entry states, per-entry `required_skipped` events, >3-target lint). If cap: adopt A's `len > 1` rejection and delete nothing else — the rest of the plan is identical under both settings. |

### L4 — Terminal paths that must call the generalized release hook (panel Q4)

| Aspect | Evidence |
|---|---|
| Convergence | Unanimous on the hook itself: `releaseInterrogationTargetForCompletedReview` generalizes to `releaseInterrogationTargetsForTerminalConsumer(ctx, runner, repositoryID, runID, jobID)`, releasing the union of direct upstreams ∪ declared targets through the idempotent, already-guarded `maybeCloseInterrogationTarget` (C-095-4 untouched). A, B §5.1, C Q4. |
| The completeness fork and its resolution | A enumerates ~11 call sites and proposes a **table-driven behavioral test** as guard. SCORECARD_A lands two findings against exactly that: **F1 (high)** — A misses `HandleBlockWork` `severity=human_checkpoint` → `waiting_human` (`lifecycle.go:908`); if the *last* pending consumer blocks there, nothing triggers close evaluation and the omission propagates into A's own guard. **F2 (medium)** — an enumeration-bound guard cannot catch a *future* terminalizing path, and the recovery backstop provably cannot reach the F1 leak (`recoverStuckJobs` CASE 3 scans only unfinished jobs; in the leak scenario every consumer is terminal and the join is `blocked`). SCORECARD_A F2 names the acceptable fix shapes: a choke-point helper + static guard, or an invariant assertion. B §5.2 ships precisely that: `markJobTerminal` choke point with a 15-row from-source inventory — **including the `work.block` row A missed (row 2)** — plus a §5.3 AST guard test (every `UPDATE striatumd.jobs` writing a terminal state must be inside the helper or carry a two-entry rationale'd allowlist: `run.cancel` and run-failure, structurally covered by `closeRemainingSessions`). C's choke point ("`db.WithJobStateTransition` or equivalent") is judged "aspirational, not concrete" by SCORECARD_C (Q4 incomplete; blocking gap 2). SCORECARD_B scores B's version 10/10 on lifecycle correctness. The panel's resolved answer is B's choke point + AST guard. |
| Residual corrections to B | (a) SCORECARD_B minor 1: the AST guard must not false-positive on unrelated UPDATE statements and is a maintenance surface — mitigated by B's modeled-on-precedent claim (`mutation_tx_guard_test.go`, RFC 0111 catalog guard). (b) B §5.3 claims the recovery sweep is a convergence backstop for missed paths; SCORECARD_A F2's CASE 3 analysis shows the sweep **cannot** reach the all-consumers-terminal leak — so the honest backstop for that class is the end-of-run no-active-session fixture invariant (L5), and the plan should say so rather than over-credit the sweep. (c) SCORECARD_A F4 (medium): concurrent terminalization of two consumers must not double-miss the close (neither sees the other terminal) — the serialization assumption (RFC 0104 per-run mutation lock) is unstated in all three proposals and must be stated and pinned with a fixture assertion. |
| Risk | This is the highest-stakes row: A's own risk section names "missed terminalizer" the largest correctness risk, and A shipped with one (SCORECARD_A F1). Per-site patching empirically leaves seven missing sites today (B §10.4). A future unguarded path is the silent-leak class C-105-2 exists to refuse. |
| Implementation consequence | `markJobTerminal` in `go/pkg/mutations` writing terminal state + caller-specified event + hook invocation; mechanical swaps at B §5.2 rows 1–13 across `{lifecycle,review,operator,recovery,recovery_auto_finalize,run}.go`; AST guard test `terminal_release_guard_test.go` with the two-row allowlist; stated RFC 0104 lock assumption; terminal→terminal transitions remain uniform no-op-safe hook calls (B's invariant). |

### L5 — Exact RFC 0105 fixture cells needed before ACE can graduate (panel Q5)

| Aspect | Evidence |
|---|---|
| Convergence | Unanimous: three production-handler cells in `go/pkg/adapterconformance` on the existing in-process harness + fake agent, mapping one-to-one onto RFC 0112 AC 2–4 — happy path, revision-reopen fresh window, dead-lane during re-cascade. A (three cells, two-posture economy), B §8 (named tests: `TestACEExplicitConsumersHappyPath`, `…RevisionReopenFreshWindow`, `…DeadLaneDuringReCascade`), C (Q5 fixture table). PG-gated under `make check` per C-105-1/2. Graduation flip itself stays out of scope in all three (C-106-1). |
| Genuineness discipline | B §8's rule is the differentiator: seed the run from **production generator output** (`compileAdjudicatedConstraintExtraction`, `generate.go:899`, smallest legal parameters) so the generator change ships in the same bytes production uses — the anti-isomorphism discipline RFC 0106/D169 demands. SCORECARD_B explicitly endorses. A uses a two-posture generator spec (compatible); C is generator-silent. |
| Required hardening (from scorecards, all additive) | (1) SCORECARD_C blocking gap 1: **preserved-context assertions** — seed a convener-only fact, ask for it through interrogation, fail if the answer is derivable from artifacts alone (mirrors the RFC 0082 intention test); without it "open/ask/answer" proves plumbing, not preserved-context cross-examination. (2) SCORECARD_A F6a: a `waiting_human` consumer-exit cell/assertion (`work.block` path — ties to F1) proving the window state across block→resolve. (3) SCORECARD_A F6b: the end-of-run invariant "no session remains in `awaiting_interrogation`" asserted directly (B cell 1 step 7 already has it; A lacked it). (4) SCORECARD_A F6c: named negative-validation tests for AC 1 (missing/self/non-interrogable/unreachable) as `workflowauthoring` unit tests. (5) B's cell 4 compatibility floor: existing direct-dependent suites run **unmodified** + `TestNoTargetsPacketUnchanged` (AC 5). (6) SCORECARD_C: assert same-attempt requeue after dead-lane and mid-fan-out window-still-open (B cells already carry both: cell 1 steps 2/5, cell 3). |
| Risk | Shallow-evidence risk, not wedge risk: without (1)–(3) the cells prove happy/revision/fault paths but not the closure invariant or genuine context preservation — RFC 0106 graduation evidence would be genuine but narrower than the predicate's state space (SCORECARD_A "Gaps" section; SCORECARD_C fixture rigor 3/5). |
| Implementation consequence | `go/pkg/adapterconformance/ace_interrogation_test.go` (B's name; A's `ace_interrogation_consumers_test.go` is the same artifact) with cells 1–4 + the six hardening items above; `workflowauthoring` unit companions. These are the cells a later RFC 0106 ACE graduation cites as genuine, non-isomorphic coverage. |

### L6 — Work-packet namespace and target-session projection (panel Q6)

| Aspect | Evidence |
|---|---|
| Convergence | Unanimous namespace: `context.interrogation_targets[]`, per-entry `workflow_job_id`, `required`, `state ∈ {available, unavailable, not_ready}`, `target_session_id`, `instruction`. A and B both add `target_attempt` and `reason`; B anchors assembly in `buildPacket` (`claim.go:283`) alongside the existing optional context blocks, read-only, **no writes at claim time** — consistent with the existing packet `context` contract. Absent block when nothing is declared = AC 5 compatibility (A, B). |
| Nullability and reason rules | SCORECARD_C gap 5 demands explicit rules; B supplies them: `available` → live session id, reason null; `unavailable` → the **retired** session id kept for evidence linkage, reason = the session's `close_reason` verbatim (`interrogation_window_closed` / `revision_reopened`) so the lane sees *why* without burning a state-changing call; `not_ready` → null session id, reason `target_not_yet_completed`. A: session id present for available/unavailable, absent for not_ready; adds machine-readable reasons. Union is coherent; B's close_reason-verbatim is the stricter, evidence-linked variant. |
| The definitional split: `not_ready` | A (attempt-aware): no awaiting-interrogation session **for the target's current attempt** — so a mid-reopen rerun projects `not_ready`. B (event-historical): never armed (no awaiting event ever); a retired prior-attempt session projects `unavailable` with reason `revision_reopened`. The split only matters in the rare mid-revision claim window, but the packet must be legible rather than wrong there (B §6). SCORECARD_A F7 praises A's attempt-aware resolution as catching "a genuine revision-cycle bug class — a reopened consumer degrading to a false `unavailable` against a retired session"; SCORECARD_C gap 3 independently demands projection "bound only to the fresh attempt." The evidence therefore leans attempt-aware state resolution (A) carrying B's reason vocabulary — `not_ready` with reason `revision_reopened` is strictly more legible than either alone: the lane learns both "do not use a session" and "why". |
| Mechanism | A: add `attempt` to future `session.awaiting_interrogation` event payloads (additive; attempt-1 default for legacy events). B: resolve via latest event `event_id DESC` (`interrogableTargetSessionForJob`, `interrogation.go:494`) — existing mechanism, transactionally safe across reopen. These compose: event-DESC resolution + attempt check. |
| Risk | Without attempt-awareness, a reopened consumer sees a prior attempt's closed session and degrades to false `unavailable` instead of waiting for/using the fresh target (A Risks; SCORECARD_A F7; SCORECARD_C gap 3). The daemon-authored `instruction` string must mirror the `interrogation.open` signal text so packet and RPC never disagree (B §6). |
| Implementation consequence | Projection in `go/pkg/mutations/claim.go` (`buildPacket`); `attempt` added to awaiting-event payloads; `striatum.work-packet.v1` contract-doc impact recorded (SCORECARD_A F5 fold-in). No new RPC. |

### L7 — Consumer-relation derivation: snapshot-derived, never stored (cross-cutting)

| Aspect | Evidence |
|---|---|
| Convergence | **Unanimous and load-bearing:** the consumer relation is a pure function of the run's frozen `workflow_snapshots.workflow_json` plus live `jobs` rows. No new table, no owner-table DDL, no migration. A rejects an `interrogation_consumers` table; B §4/§10.2 rejects materialization ("derive, don't store" — a second source of truth would need coherence across re-prepare and revision cascades, plus RFC 0079 §5 owner-migration friction); C's JSONB lateral-join SQL is the same decision expressed as a query. Old snapshots make the new predicate half vacuously empty — bit-identical behavior, AC 5 (all three). |
| Scorecard pressure | SCORECARD_C gap 4: decide whether the raw JSONB query is the durable implementation or a temporary shape — either way isolate it **behind one resolver helper** and test absent arrays, malformed entries, multiple targets, and direct-plus-explicit duplicate consumers. SCORECARD_B minor 2: profile the per-terminal-transition snapshot lookup for deep revision histories. B's cost answer: identical profile to what `jobIsInterrogable` already pays on every `work.complete`; runs have tens of jobs — no cache until measured need. SCORECARD_A: snapshot parsing in the close path "acknowledged and acceptable." |
| Risk | Low and bounded: query-shape maintainability (SCORECARD_C maintainability 3/5 for raw lateral JSONB) — resolved by the single-resolver rule; perf is speculative pending profile. |
| Implementation consequence | One resolver helper (the derivation pattern `jobIsInterrogable`/`workflowJobInterrogable` already use) consumed by both the predicate (L8-adjacent: `interrogationConsumersPending` union, `interrogation.go:516`) and the release hook; malformed-shape unit tests; in-flight pre-field ACE runs keep frozen snapshots and must be **re-prepared** (B §9 — the honest #115 migration note; A says restart/operator-recover, same substance). |

### L8 — Concurrency serialization of consumer terminalization (cross-cutting)

| Aspect | Evidence |
|---|---|
| Source | SCORECARD_A F4 (medium): two cross-examiners terminalizing concurrently must not each observe the other as still-pending (neither closes → leak) nor double-close. No proposal states the serialization assumption; all presumably inherit the RFC 0104 per-run mutation lock. |
| Opposition | None — no panel artifact disputes the finding; B's idempotent `maybeCloseInterrogationTarget` covers double-close but not the mutual-miss case, which only serialization covers. |
| Risk | A silent leak indistinguishable from the F2 class if run-level locking is ever relaxed without revisiting this predicate. |
| Implementation consequence | State the RFC 0104 `lockRun` assumption explicitly in the predicate/hook code docs and **pin it with a fixture assertion** (two consumers completing back-to-back; window closes exactly once) so a future locking relaxation fails loudly. |

### L9 — Revision reopen, attempt scoping, and stale-target retirement (cross-cutting)

| Aspect | Evidence |
|---|---|
| Convergence | Unanimous: **zero new reopen code.** `reopenJobForAttempt` already retires the superseded target session + open interrogations first (`closeInterrogationTargetForReopen`, `revision_routing.go:291` → `interrogation.go:566`, `close_reason = revision_reopened`); the transitive downstream reset reaches the ACE fan-out through ordinary edges; the fresh attempt emits a fresh awaiting event; C-095-2/-3 hold. A (Revision Reopen Behavior), B §7, C (Q5 revision fixture). SCORECARD_A verified the reopen anchors against source and found "no gaps." |
| The load-bearing link | B §7's observation ties L1 to L9: the **reachable-downstream validation rule is what guarantees** the RFC 0095 reset cascade re-blocks every explicit consumer — validation and lifecycle are one design. This is why reachability is a hard error, not lint (B §1 table). |
| Retirement doors | B §7 enumerates exactly four (consumer-drain choke point, revision reopen, recovery-sweep leaked-window case, run teardown), each with a distinct `close_reason`; the fixture asserts the first two and the absence of a fifth. Subject to the L4(b) correction: the sweep door cannot reach the all-consumers-terminal case. |
| Risk | Covered by L6's attempt-awareness; a stale packet surviving the boundary is refused by the existing live-target check (A). The `waiting_human` one-way door (resumed consumer may find the window gone) is accepted, evidenced via L2 events, and is existing panel behavior under #131 (B §5.2 invariant 2) — not a new regression. |
| Implementation consequence | None beyond L1's reachability rule, L4's choke point, and L6's attempt-aware projection — that is the point: reopen correctness falls out of existing RFC 0095 machinery plus validation. |

## Smallest coherent plan

The panel converged on one architecture. The plan below is B's lifecycle-first
skeleton (the only proposal whose terminal-path answer survived scorecard
audit intact) amended by the scorecard findings and carrying A's
attempt-aware projection and C's fixture-rigor demands. Every element traces
to a cited row above. The arbitrator can accept or reject it as a unit, with
exactly one dial to set.

1. **Field + validation (L1).** `interrogation_targets: [{workflow_job_id,
   required}]` on consumer jobs; hard errors: missing target, not
   `interrogable: true`, self-reference, not reachable-downstream, duplicates,
   declared on the target job itself; lint warnings: unknown entry fields,
   lane lacks `interrogate`, redundant direct-dependency. New
   `ValidateInterrogationTargets` beside `ValidatePhaseShapes`; document in
   `docs/reference/spec.md` + ubiquitous-language ("interrogation consumer"),
   resolving the precise self/chained rule (SCORECARD_A F5).
2. **THE DIAL — V1 target cardinality (L3).** Default per the evidence
   majority: **allow N ≥ 1** with B §3's per-entry semantics and the >3-target
   lint. Flipping to A's cap-at-one is a single validation rule and disturbs
   nothing else in this plan; the arbitrator sets this dial explicitly either
   way.
3. **Predicate (L7).** Widen `interrogationConsumersPending` to the union
   (direct dependents ∪ snapshot-declared consumers), derivation isolated
   behind one resolver helper with malformed-shape tests; terminal set
   unchanged; no schema change; old snapshots bit-identical (AC 5).
4. **Choke point + hook (L4).** `markJobTerminal` helper invoking the
   generalized `releaseInterrogationTargetsForTerminalConsumer`; swap B §5.2
   rows 1–13 (including `work.block` → `waiting_human`, the SCORECARD_A F1
   path); AST guard test with the two-row structural allowlist
   (`run.cancel`, run-failure); state and fixture-pin the RFC 0104 per-run
   lock assumption (L8); credit the end-of-run fixture invariant — not the
   recovery sweep — as the backstop for the all-consumers-terminal leak class
   (L4 correction (b)).
5. **`required` semantics + durable evidence (L2).** Advisory in V1;
   `interrogation.unavailable_signaled` on actual open-refusal and
   `interrogation.required_skipped` at the choke point; packet projection
   writes nothing; V2 hard gate pre-declared as a predicate flip over this
   evidence.
6. **Packet projection (L6).** `context.interrogation_targets[]` in
   `buildPacket` with `workflow_job_id`, `required`, `state`,
   `target_session_id` (B's nullability rules), `target_attempt`, `reason`
   (close_reason verbatim), daemon-authored `instruction` mirroring the
   `interrogation.open` signal text. **Attempt-aware state resolution** (A +
   SCORECARD_C gap 3): `not_ready` = no awaiting session for the current
   attempt, with B's reason vocabulary; `attempt` added additively to
   awaiting-event payloads, legacy events default attempt 1.
7. **ACE generator (L1/L5).** Each generated `cross_examiner_N` declares
   `{workflow_job_id: convener_draft, required: true}`; graph unchanged, no
   fake edges (brief §3, RFC 0112 non-goal).
8. **Fixtures (L5).** `go/pkg/adapterconformance/ace_interrogation_test.go`,
   seeded from production generator output: cell 1 happy path (window
   survives the gate; survives first consumer; closes on last with
   `interrogation_window_closed`; end-of-run no-active-session invariant),
   cell 2 revision-reopen fresh window, cell 3 dead-lane same-attempt requeue
   during re-cascade, cell 4 no-targets compatibility
   (`TestNoTargetsPacketUnchanged` + existing suites unmodified). Hardening:
   preserved-context assertion (seed a convener-only fact — SCORECARD_C gap
   1), `waiting_human` block→resolve assertion (SCORECARD_A F6a),
   concurrency pin (L8), named negative-validation unit tests (F6c).
9. **Scope honesty (brief criterion 9).** Implementation touches
   `go/pkg/{workflowauthoring,workflowgenerate,mutations,adapterconformance}`
   and `docs/reference/*` — outside this panel's frozen envelope; lands as
   separately scoped work. No new RPC family, no tier flip, no schema
   migration, no transcript capture. In-flight pre-field ACE runs are
   re-prepared, not silently rewritten (#115).

Rejecting the plan means rejecting B's choke point or the union predicate —
the two elements every scorecard either demanded or scored highest — so the
honest rejection surface is narrow: the arbitrator's real decisions are the
L3 dial and whether the L6 merged `not_ready` definition (attempt-aware state
+ B's reason vocabulary) is the packet contract to freeze.
