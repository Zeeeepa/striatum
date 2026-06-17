# RFC 0132: Gating/advisory reviews and quorum with dissent protection

Status: accepted (D212)
Date: 2026-06-17
author: proposer-claude-opus-4-8-001
Context:
- GitHub [#311](https://github.com/halbritt/striatum/issues/311) — the incident
  and its closing disposition, which captured **P2** (gating/advisory reviews +
  quorum) as a deferred graduation. This RFC writes that design up.
- Decision [D209](../decisions/decision-log.md) — #311 **P0** (per-job quarantine
  for a *single flaked* job). P2 is the panel-aware sibling: a *committee*'s right
  behavior when one gating reviewer is unrecoverable is to finalize on a quorum of
  the others — not to quarantine a reviewer whose verdict the gate still needs.
- Decision [D210](../decisions/decision-log.md) — Wave-2 closeout naming this
  graduation in its "Revisit Trigger".
- [RFC 0118](0118-gate-run-completion-on-attested-provenance.md) — the
  run-completion provenance gate. The attestation invariant ("no verdict/artifact
  is sealed that the agent did not produce") is the **hard constraint** P2 must
  not violate.
- [RFC 0126](0126-multi-reviewer-revision-coherence.md) / RFC 0095 — multi-reviewer
  revision coherence and revision-safe lifecycle. P2's dissent-protection must
  compose with revision rounds without re-creating the stale-verdict reopen
  incoherence those RFCs address.
- [RFC 0064](0064-review-diversity-enforcement.md) — review-diversity (gating vs
  advisory is the *authority* analog to diversity's *independence*).
- [RFC 0110](0110-daemon-postgres-authentication-and-database-enforced-write-boundary.md)
  — DB-enforced write boundary; the dissent invariant below is enforced at the DB
  layer, not by Go convention.
- Prior art in source: `go/pkg/mutations/mutations.go` (`dependenciesSatisfied`,
  `maybeCompleteRun`), `go/pkg/mutations/review.go` (`recordVerdict`,
  `applyVerdict`, `completeReviewJob`), `go/pkg/mutations/recovery_escalation.go`,
  `go/pkg/mutations/recovery_complete_stalled.go`,
  `go/pkg/workflowauthoring/lint.go` (the `degraded_seat_lane` lint precedent),
  `go/pkg/db/sql/0005_repo_local_workflow_state.sql` (the `verdicts` table and the
  `jobs UNIQUE (repository_id, run_id, workflow_job_id, attempt)` constraint),
  `go/pkg/db/sql/0007_decision_propagation.sql` (`superseded_by_decision_id`),
  `go/pkg/db/sql/0024_verdict_provenance_stamp.sql` (the RFC 0118 verdict HMAC
  stamp), `go/pkg/db/sql/0011_escalation_inbox.sql`.

## Problem

A review **panel** is N reviewer jobs, each producing a verdict
(`accept` / `needs_revision` / `reject`), feeding a downstream synthesis or gate
job. Today the downstream gate is satisfied edge-by-edge: `dependenciesSatisfied`
(`mutations.go`) walks each `job_dependencies` row and checks
`gate_json.requires_verdict`. **There is no aggregate quorum node and no notion
of a declared-seat denominator.** Consequently a single reviewer lane that cannot
complete — the #311 `agy`/Gemini case — wedges the entire run, because its edge
never satisfies.

The naive fix (let the gate finalize on whoever did vote) is worse: it silently
collapses a missing voice into agreement. The hard requirement is to finalize on
a quorum **without dropping a real disagreement into false unanimity**, and
**without deadlocking forever** when a voice is genuinely, permanently absent.

The disposition on #311 sketched a backward-looking rule: refuse the abstention
if a *prior attempt* of the missing lane recorded `needs_revision`/`reject`
(query `verdicts` at `attempt < current`). Grounding that against the schema
exposed a flaw worth fixing in this RFC: recovery (`recovery_invalidate_job.go`,
stalled-transfer) **reassigns `job_id` and `session_id`**, so a real reject can
fall off the current job lineage and a backward query keyed on those churning
identifiers reads it as *absent*. The dissent disappears exactly when recovery
moved it — the attack the rule was meant to stop.

## Goals

1. Let a panel **finalize on a quorum of gating reviewers** when a gating
   reviewer is unrecoverable, instead of wedging the whole run.
2. **Never finalize a false unanimity.** A disagreement that was ever expressed,
   or a dissent that recovery moved, must block a clean finalize until explicitly
   and attributably resolved.
3. **Never fabricate a vote.** Honor the RFC 0118 attestation invariant: the
   daemon may record that a seat is *unfilled*, never *how it would have voted*.
4. **Never deadlock silently.** When present reviewers genuinely disagree, fail
   into a *named, attributable* state an operator can act on — not an infinite
   wait and not a silent skip.
5. Keep advisory reviewers **non-blocking but never silent**.

## Non-Goals

- Semantic understanding of review content (confidence scores, concern-axis
  coverage, semantic-distance weighting). Those require an artifact model
  Striatum does not have and multiply the attack surface; explicitly out of
  scope (see Traps).
- Replacing the revision lifecycle (RFC 0095/0126). P2 composes with it.
- Changing P0's single-job quarantine. A *provenance-required reviewer* the gate
  still needs is precisely what P0 must **not** quarantine; P2 is the path for it.

## Proposed design

Three composing layers plus one axiom that must be decided first.

### The attestation axiom (decide first — it shapes everything)

The daemon must **never** synthesize a verdict value on a missing lane's behalf.
It may assert only the provable fact **"this seat is unfilled."** A missing
gating reviewer therefore does not become an `accept` or a `reject`; it leaves
its seat **unresolved**, which *raises the denominator* (see Layer A). With this
axiom, "false unanimity" becomes structurally hard to construct, because
unanimity is defined over *filled* seats and an unfilled seat is visible, not
silently counted.

### Layer A — Freeze the denominator (declared-seat quorum)

Reframe quorum as a **ceiling on tolerated silence, not a floor on accepts.**

- Each reviewer job carries `panel_role: gating | advisory` (default `gating`),
  validated in `workflowauthoring/lint.go` beside the existing
  `degraded_seat_lane` lint. (`panel_role` does not exist today.)
- The downstream gate job carries a `quorum_json` block frozen into the run
  snapshot at authoring time: `{ declared_gating_seats: [seat_key…],
  max_gating_abstentions: int }`, derived from the gating reviewers. Lint
  requires `max_gating_abstentions` to be a non-negative integer ≤ seat count,
  **default 0**, and any value > 0 to be set by an explicit author flag so the
  relaxation is a visible, fixture-diffable decision rather than an emergent
  property of who showed up.
- A new predicate `panelQuorumSatisfied` (branching off `dependenciesSatisfied`)
  classifies each **declared seat's** latest active verdict — resolved by a
  **stable seat key**, see Layer B — into `accept` / `blocking` / `abstain-or-
  missing`, and passes only when:
  `blocking == 0 AND (abstain + missing) ≤ max_gating_abstentions AND the
  remaining verdicts satisfy requires_verdict`.

Because a missing seat counts against a **fixed N**, shrinking the live set can
never manufacture a majority — the "lose two of five reviewers and the lone
accept clears" failure is impossible.

### Layer B — Forward-write dissent (replace the fragile backward query)

The stable identity to key dissent on is **`workflow_job_id`** — stable across
recovery because of `jobs UNIQUE (repository_id, run_id, workflow_job_id,
attempt)` (the `job_id`/`session_id` churn, the `attempt` increments, but the
`workflow_job_id` *seat* is constant).

- In `recordVerdict`, **inside the same transaction** that inserts a
  `needs_revision` / `reject` verdict, INSERT an append-only `dissent_ledger`
  row keyed on `(repository_id, run_id, workflow_job_id)`.
- Quorum reads the ledger, not a backward `verdicts` scan: any **live**
  (non-superseded) dissent row for any declared seat blocks a clean finalize,
  wherever recovery later moved that seat's lineage.
- Supersession is explicit and attributable: only a real `decisions` row (an
  operator override, or an absorbing-adjudicator accept) may stamp
  `superseded_by_decision_id` on a ledger row. A NULL/dangling clear is rejected.
- **DB-enforced completeness**: a doctor integrity check (and, ideally, a
  constraint trigger per the RFC 0110 boundary) flags any non-accepting,
  non-superseded verdict that lacks a live ledger row — so a future code path
  that records dissent *without* burning the token is caught, not silently
  re-opening the hole one layer down.

When a gating seat is unrecoverable, the recovery sweep authors a daemon-signed
**abstention stub** (an artifact, not a fabricated verdict) recording, per the
axiom, "seat unfilled" plus a typed reachability class:

- `structurally_unrecoverable` — the process is *provably* dead (the #147-class
  dead-PID probe), and a stub may be quorum-skipped (it counts against the
  abstention budget).
- `content_withheld` — the lane is alive but silent. **A `content_withheld` seat
  is refused even under unanimous gating accept**: silence from a *live* lane is
  not consent.

The stub is HMAC-signed with the daemon runtime key (reusing the
`0024_verdict_provenance_stamp` mechanism) and bound to the attempt ordinal and
the daemon boot-epoch (#316), so a stale or replayed stub fails verification.

### Layer C — Advisory: loud reject, silent accept, mandatory minority report

Advisory reviewers do not count toward gating quorum, but are never silent.
Implemented as guards in `applyVerdict` over two `panel_role`-keyed tallies in
one transaction:

1. A current-attempt **gating abstention** co-occurring with **any** current-
   attempt advisory `needs_revision`/`reject` → reclassify the abstention to
   `must_escalate` (reason `advisory_corroborated_abstention`).
2. **Unanimous advisory reject** (≥ 1 submitted advisory seat, all reject) →
   block finalize even on full gating accept (`unanimous_advisory_reject`).
3. Every finalize authors a required `advisory_minority_report.v1` front-matter
   artifact (per-seat verdict + rationale excerpt + which guard fired), pinned as
   an `expected_artifact` on the synthesis packet — the publisher refuses with
   exit code 6 if it is omitted.

An **advisory-only panel** (zero gating seats) blocks unconditionally
(`advisory_only_panel_ungrounded`) — an ungrounded signal must not auto-finalize.
All three block reasons are operator-resolvable blockers (the `checkpoint
resolve` shape), never auto-flips: advisory can stop the line, never silently
wedge it and never auto-`needs_revision` the implementer.

### The single load-bearing risk (named by every design branch)

The denominator (Layer A) and the prior-dissent evidence (Layer B) must be
**read-consistent and immutably bound in one transaction** holding the existing
per-run advisory lock (`lockRun` / `pg_advisory_xact_lock(hashtext(run_id))`).
Otherwise a concurrent recovery clears a row between the read and the finalize
(TOCTOU), or the finalizer re-derives the denominator from currently-live lanes,
and the protection evaporates *while lint shows green*. The forward-write of
Layer B is co-transactional with the verdict insert for exactly this reason.

## Alternatives considered (and why they are traps)

- **Confidence / volume-weighted quorum** (a `0.6` verdict, market-style price
  clearing). Unfalsifiable; lets a synthesizer launder a barely-cleared panel as
  consensus. Violates attestation legibility.
- **Ghost-vote / sampled-distribution abstention** (synthesize a probable verdict
  for the missing lane from its history). Directly violates the attestation
  invariant — the hardest trap. The axiom forbids it.
- **Using `recovery.complete-stalled` as the override path.** It is the
  laundering vector itself; it already *refuses* verdict-capable jobs (RFC 0118)
  and must continue to.
- **Crypto-on-the-stub as a guarantee of truth.** The signature certifies what
  the daemon *believed*, not ground truth; a `content_withheld → structurally_
  unrecoverable` misclassification (the #147 race) yields an audit-clean stub that
  legitimately drops a real dissent. Mitigation: a **conservative classifier** —
  default to `content_withheld` on any ambiguity; require *two* independent
  dead-signals before `structurally_unrecoverable`. Do not treat the signature
  as truth.
- **Affinity maturation** (raise the bar each failed attempt) as a default. Turns
  flaky-lane runs into deadlock machines. Opt-in only.
- **A `dissent_quarantine` run state used as a "merge anyway" button.** If most
  panels emit `needs_revision`, operators learn to reflexively override and the
  silent false-accept returns, laundered through an attributable-*looking*
  override nobody reads. If a run-state form is adopted, the lift must be costly
  and legible (the override artifact restates the minority verdict; the override
  *rate* is itself a doctor warning), and `needs_revision` must route to the
  revision cycle, with hard quarantine reserved for `reject`.

## Risks

- **Permanent fail-closed deadlock** if the seat-key mapping is wrong (a recovered
  /revised job not mapped back to its stable seat). This replaces false unanimity
  with a worse failure for autonomy. The predicate must resolve to `seat_key`/
  `workflow_job_id`, never `job_id`. Covered by the four-end-state fixture below.
- **Sticky dissent surviving an honest fix.** A phantom dissent about a *prior*
  revision could keep binding against a now-corrected artifact (the RFC 0095/0101
  reopen incoherence, made durable). Mitigation: bind a ledger row to the artifact
  sha it objected to and auto-stale it when the synthesis content-hash advances
  past it (reuse the #300 superseded machinery).
- **Backfill/cutover.** Switching the gate from the backward `verdicts` query to
  the ledger needs a transitional doctor check that diffs ledger-derived vs
  verdicts-derived quorum on every run to *prove* agreement before the old query
  is removed.

## Migration and rollout

- `panel_role` and `quorum_json` live in `workflow_json` — **no DDL**, validated
  in lint.
- `dissent_ledger` is a new **runtime** table (next free runtime migration). The
  abstention stub is an artifact (registered in `artifactcontracts`, no DDL).
- If the optional `dissent_quarantine` *run state* (the legible-deadlock form) is
  adopted, the `runs.state` CHECK is owner-held, so it needs **owner bundle 0013**
  (mirroring `0012_job_quarantine_state.sql`). The artifact-plus-`escalation_inbox`
  blocker form needs no owner bundle and is the recommended v1.
- Ship behind an off-by-default policy flag; default behavior is unchanged
  (every gating seat required, today's wedge) until a workflow opts into a
  quorum.

## Doctor and legibility

- `quorum_seat_unresolvable`, `quorum_denominator_mismatch`, and
  `finalize_ignored_advisory_dissent` doctor checks.
- `dissent_ledger` live rows surface as "unresolved dissent blocking finalize" in
  `striatum status` / dashboard.
- The finalize decision (gating tally, advisory tally, which guard fired) is
  rendered in `dashboard --run-id` / `run.summary`, so an advisory park or a
  quorum hold is self-explaining before `checkpoint resolve`.

## Test plan

- A four-end-state regression fixture locking the core invariant:
  (1) all-accept → finalize; (2) one **missing** gating seat within budget →
  finalize; (3) one **present** gating `needs_revision` → never silently finalize
  (routes to revision/escalation); (4) one **present** gating `reject` → blocked,
  attributable.
- A recovery race: a gating seat that records `reject`, is then stalled-
  transferred (job_id reassigned) → the ledger row still blocks finalize.
- An attestation test: a `content_withheld` seat is refused under unanimous
  gating accept; a `structurally_unrecoverable` seat (two dead-signals) is
  quorum-skipped within budget.
- A stub-replay test: a stub bound to a stale attempt ordinal / prior boot-epoch
  fails verification.
- `make -C go vet lint check-tests` **uncontended**.

## Open questions

1. **Adopt the axiom strictly?** If "the daemon may never cast a vote, only hold
   a seat open" is the load-bearing axiom, Layer B's stub never *votes* — it only
   refuses to vacate a seat, and the abstention budget (Layer A) becomes the
   *only* thing that can clear an unfilled seat. This is the attestation-safest
   shape and should be ratified before implementation, because it determines
   whether the stub carries a verdict value at all.
2. **The sharper rule (provocation):** the threat is usually framed as "the
   missing lane." Invert it — a hostile scheduler need not *kill* the dissenter,
   only make it *slow* while fast yes-men reach quorum. Should a panel be
   forbidden from finalizing-by-quorum while any gating seat is **live and
   working** (as opposed to provably dead)? That collapses Layer B's
   `content_withheld` rule into a stronger, simpler one — **quorum may skip a
   DEAD seat, never a slow one** — turning "detect dropped dissent" into "never
   race a live reviewer." Recommended for adoption; recorded as an open question
   because it tightens the quorum semantics and interacts with the liveness
   confidence model of [RFC 0131](0131-transport-aware-liveness-confidence.md).
3. Should the quorum shape be generalized to `all_seats | budget:N | k_of_n`,
   desugared in lint?

## Appendix — design provenance

The denominator/dissent/advisory three-layer decomposition, the
"quorum = ceiling on tolerated silence" reframe, the attestation axiom, the
forward-written-ledger fix for the backward-query flaw, and the trap catalog were
produced by a parallel-frame divergent-ideation run (regulator, markets, ant-
colony, inversion, and hostile-competitor frames over isolated branches, then
convergence) grounded against the real `mutations.go` / `review.go` source and the
`jobs` UNIQUE constraint. Four independent frames converged on the same three
mechanisms — that cross-frame convergence is the reason this RFC presents them as
one stacked design rather than competing options.
