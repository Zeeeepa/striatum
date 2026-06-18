# RFC 0098: Adjudicated Constraint-Extraction Loop (Refusal-to-Constraint)

Status: implemented — the ACE shape (`adjudicated_constraint_extraction`) is graduated to `supported` (catalog `supportedShapes` + `adapterconformance/ace_interrogation_test.go`, tier-guarded); slice-4 first-class `constraint.*` objects (§6) are deferred until a second cross-run constraint consumer appears, and the §7 coverage metrics are optional-future (D222, closes #399)
Date: 2026-05-30
author: proposer-claude-opus-4-8-001
Source: GitHub issue #89 — surfaced by the Engram entity-relationship forum run
`run_2535842d92d1468ae978022417528812`, where three models did **not** converge
by agreement; they converged because an adjudicator refused publication once,
then forced the convener to turn objections into testable constraints. This RFC
promotes that observed behavior into a first-class Striatum workflow shape.

Context:
- [RFC 0093](0093-structured-live-collaboration-workflow-shapes.md) — the parent.
  This RFC is a **successor**: it reuses 0093's `adjudicator` role, the
  `phase_synthesis`-class **substance-gate**, and the
  `striatum.collaboration_ledger.v1` artifact, and extends them. 0093's gate
  answers *"did the exchange do its epistemic work?"* with a verdict; 0098's gate
  must additionally answer *"what binding constraints does a refusal produce, and
  were they discharged?"*. Where 0093 made refusal **honest**, 0098 makes it
  **productive**.
- [RFC 0082](0082-interrogation-sessions.md) / [RFC 0086](0086-multiparty-conversation.md)
  — the interrogation and conversation primitives the cross-exam phase composes.
- [RFC 0083](0083-iterated-panel-review-with-interrogation.md) — the revision
  cycle (`needs_revision` → re-open) this loop bounds with `max_cycles`.
- [RFC 0045](0045-multi-phase-workflow-editor-and-schema.md) /
  [RFC 0074](0074-workflow-shape-and-adversary-pack-catalog.md) — the multi-phase
  gating rules and pack catalog the new shape plugs into.
- [RFC 0095](0095-revision-safe-workflow-lifecycle.md) — the revision-safe
  lifecycle. Constraint-extraction is a multi-cycle revision loop, so it depends
  on 0095's attempt-aware re-open and on **#84** (republish a revised artifact
  under the same `logical_name`) being resolved for the convener/spec artifacts.
- [`docs/decisions/decision-log.md`](../decisions/decision-log.md) — D028
  (curated-text-only), D155 (RFC 0093 V1 accepted).
- Related friction from the same run: #84 (cycle-aware logical names), #79 / #88
  (collaboration-ledger contract rejecting natural / cleared front matter), #77
  (adjudicator should absorb `needs_revision` instead of spawning a checkpoint),
  #74 (synthesis contract opacity). 0098 should not ship on top of those bugs; it
  surfaces why they matter.

## Problem

RFC 0093 gives multi-model workflows an honest gate: a hollow exchange scores
`needs_revision` and the commit stays withheld. But a bare `needs_revision`
verdict is **too weak** in two ways the Engram run exposed:

1. **Refusal without extraction loses the work.** When the adjudicator blocks,
   the load-bearing objections live only as prose inside review artifacts. The
   next revision cycle re-reads the *prior artifact* and the reviewers' comments
   and tries to "address feedback" — averaging criticism back into a blander
   synthesis. The objections are not carried forward as discrete, trackable
   obligations. High-severity challenges silently evaporate between cycles.

2. **Closure is not verifiable.** "Final review" today re-litigates the whole
   design from scratch. Nothing checks that *each* objection the adjudicator
   found load-bearing actually landed in the final spec as testable text or a
   gate. A challenge can be answered rhetorically in cycle 1 and absent from the
   shipped RFC, with no signal.

The Engram forum worked because a human-shaped discipline filled both gaps by
hand: the adjudicator extracted a numbered constraint table from the objections,
the convener's next synthesis discharged each row explicitly, and the final
reviewer checked discharge instead of relitigating. That produced materially
better RFC content (raw-evidence-only provenance, an owner-self policy matrix, a
runtime redaction lint, review-burden gates). It should be a workflow shape, not
a discipline an operator remembers to enforce.

## Goals

1. Define `adjudicated_constraint_extraction` as a reusable RFC 0093-family shape
   in the catalog, generator-emitted like `falsification_gate`.
2. Make a `needs_revision` adjudication **structurally required** to publish
   binding constraints (or explicit unresolved-question rows). A naked refusal is
   rejected by the artifact contract.
3. Carry constraints forward: a revision packet receives the prior cycle's
   constraints as **first-class inputs**, and the convener must discharge each
   one explicitly (answer / fold-in / reject-with-rationale / accept-as-risk /
   defer-with-successor).
4. Make final review a **typecheck**: it fails if any binding constraint is
   missing, partial-without-accepted-risk, or unverifiable — and it does *not*
   re-run the forum.
5. Preserve every objection's lifecycle on the record: a high-severity challenge
   may only disappear via an explicit recorded disposition.
6. Stay inside the RFC 0093 substrate where possible: extend the existing
   `collaboration_ledger.v1` rather than invent a parallel artifact, and gate via
   ordinary RFC 0045 phase rules. New daemon authority is a deferred slice, not a
   V1 requirement.

## Non-Goals

- **No new floor-control primitive, economy, or daemon authority over run state.**
  The daemon remains the sole writer. V1 adds an artifact-schema extension and a
  shape fixture, not a new RPC family. First-class constraint objects (§ Proposal
  6) are an explicitly deferred slice.
- **No semantic/LLM scoring of constraint quality.** The gate validates that
  constraints are *present, typed, sourced, and dispositioned* — structural
  anti-theater, exactly as RFC 0093 V1 chose. Judging whether a constraint is
  *good* remains the adjudicator role's prose responsibility.
- **No consensus optimization.** The shape optimizes for unresolved risk becoming
  explicit and objections becoming tests; model *disagreement* is an input that
  raises specification quality, not a failure to smooth over.
- **Not a replacement for `falsification_gate` / `cross_examination`.** This is a
  longer, revision-cycling sibling for design/spec authoring, where the output is
  an implementable RFC. For a single publish-gate, the lighter 0093 shapes remain
  the right tool.

## Proposal

### 1. The shape

`adjudicated_constraint_extraction` is an RFC 0045 phased workflow:

```json
{
  "shape": "adjudicated_constraint_extraction",
  "phases": [
    "survey",
    "convener_synthesis",
    "cross_exam",
    "adjudication",
    "revision_synthesis",
    "constraint_discharge_review",
    "spec_publication",
    "final_review"
  ]
}
```

The load-bearing edges, beyond ordinary RFC 0045 sequencing:

- **`adjudication → revision_synthesis`** carries the constraint table. A
  `needs_revision` verdict re-opens `convener_synthesis`/`revision_synthesis`
  (RFC 0083 cycle, bounded by `max_cycles`), and the revision packet's inputs
  include the prior `collaboration_ledger`'s `constraints[]`, not just the prior
  artifact.
- **`spec_publication`** consumes the *latest cleared* constraint ledger as
  binding input — the RFC begins from adjudicated constraints, not from the
  original proposal.
- **`final_review`** reads the binding constraint set and the published spec and
  emits a `constraint_discharge`-bearing finding. The phase **fails closed** if
  any `binding: true` constraint is not `discharged` (or explicitly
  `accepted_risk` with owner/stage).

### 2. Roles

Reuses RFC 0093's `adjudicator`; the rest are ordinary roles with shape-specific
prompts:

- `convener` — writes the candidate synthesis.
- `cross_examiner:<posture>` — challenges the synthesis from a named posture.
- `adjudicator` — accepts, rejects, or **converts** challenges into binding
  constraints (RFC 0093 §2 substance-gate, extended by §4 below).
- `revision_convener` — republishes the synthesis after constraints (may be the
  same lane as `convener`).
- `spec_author` — writes the RFC/spec using the latest clearing ledger as binding
  input.
- `final_reviewer` — verifies every binding constraint is discharged.

Default posture set (overridable per workflow, but the posture must be explicit
in the artifact and ledger): product/user-outcome, implementation/schema/
migration, privacy/security/provenance, evaluation/measurement, operations/
workflow/process.

### 3. Objection lifecycle

Every substantive objection has a recorded lifecycle. A high-severity challenge
may only leave `open` through one of the terminal dispositions:

```
raised → answered | unanswered
answered  → accepted | rejected | converted_to_constraint | deferred_with_owner
unanswered ∧ load_bearing → needs_revision
converted_to_constraint → bound_into_next_artifact → final_review_verified
```

"Unanswered interrogation is evidence": if a cross-examiner opens an
interrogation and no answer arrives before the convener's artifact publishes, the
ledger preserves that fact, and for load-bearing findings the default adjudication
is `needs_revision` or `accept_with_findings` + a binding constraint.

### 4. Artifact contract — `collaboration_ledger.v1.1`

Extend RFC 0093's `striatum.collaboration_ledger.v1` (not a new schema family) to
`v1.1`, additively. The existing `entries[]` and `verdict` stay; add two
optional-but-gated blocks the adjudicator populates:

```yaml
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: adjudicated_constraint_extraction
cycle: 2                              # cycle-aware; see #84
verdict: needs_revision | accept | accept_with_findings | reject
# --- new in 1.1 ---
constraints:                          # REQUIRED non-empty when ACE + needs_revision
  - id: C2-IMPL-1
    source_finding: IMPL-1            # refs a same-ledger findings[] row
    posture: implementation
    severity: high
    kind: invariant | gate | schema | policy | non_goal | accepted_risk | unresolved_question
    binding: true
    text: "claims.evidence_ids may reference raw evidence only ..."
    verification:
      expected_stage: "Stage 6"
      gate: "mixed raw+derived provenance fails closed"
    final_review_required: true
branches:                             # posture disposition matrix
  implementation: cleared_with_constraints
  privacy: cleared_with_constraints
  eval: cleared
```

Allowed adjudication verdicts stay the RFC 0093 daemon-routable vocabulary:
`accept`, `accept_with_findings`, `needs_revision`, `reject`. The two RFC 0098
refinements, `blocked_pending_answer` and `defer_with_successor`, are
`branches{}` dispositions, not verdicts. Per #88, prompts and docs must
advertise only terms the contract accepts; ambiguous bare `clear` stays
disallowed.

The cross-exam finding gains structured rows (carried in `findings_ledger` or the
ledger `entries[]`, not free prose):

```yaml
findings:
  - id: IMPL-1
    severity: high
    posture: implementation
    status: open
    challenge: "..."
    closest_acceptable_answer: "..."
    affected_invariants: [raw_evidence_only, append_only_projection]
    requested_constraint_shape: {kind: invariant}
    requires_convener_rebuttal: true
```

Final review emits a discharge table (a `finding`/`findings_ledger`, not a new
kind):

```yaml
constraint_discharge:
  - constraint_id: C2-IMPL-1
    status: discharged | missing | partial
    evidence: "RFC 0069 §2.1, §9 Stage 6"
```

### 5. Workflow semantics (the four invariants the gate enforces)

1. **Refusal must be productive.** `review.submit` of a `collaboration_ledger`
   with `verdict: needs_revision` is rejected (exit code 6) unless `constraints[]`
   is non-empty (binding constraints or explicit unresolved-question rows). This
   is the structural extension of RFC 0093's substance-gate.
2. **Revisions consume constraints, not just prior artifacts.** The revision
   packet's `task_prompt`/inputs present the prior cycle's `constraints[]` and ask
   the convener to discharge each explicitly. Re-opening reuses RFC 0095's
   attempt-aware revision lifecycle.
3. **Unanswered interrogations are evidence** (see §3).
4. **Final review is a typecheck.** It does not re-run the forum; it verifies each
   `binding: true && final_review_required: true` constraint appears in the spec
   in testable form, and fails closed on any `missing` / unaccepted `partial`.

### 6. Deferred slice — first-class constraint objects

Optional later surface, **not required for V1**: durable run-level constraint
objects analogous to decisions/findings —
`constraint.record` / `constraint.list` / `constraint.discharge` /
`constraint.verify` — so final review and downstream *implementation* workflows
consume constraints directly instead of re-parsing ledger front matter. V1
derives the same information from the `collaboration_ledger.v1.1` artifact; the
RPC family is justified only once a second workflow needs to read constraints
across runs.

### 7. Coverage metrics (observability, not a gate)

The run summary / dashboard can expose, derived from the ledgers: high-severity
findings raised, answered-by-rebuttal, converted-to-binding, rejected-with-
rationale, deferred, discharged-in-final, and cycles-to-clearance. These make
workflow quality measurable instead of vibe-based; they do not gate anything.

## Implementation Plan (slices, smallest blast radius first)

> **Status (2026-05-30): Slices 1–3 LANDED** (the V1 target). Slice 1
> `6a43c13`; slice 2 (generator shape + fixture) and slice 3
> (discharge-verifying final review + `constraint_discharge` shape) landed in the
> backlog sweep. Slice 4 (first-class constraint objects + coverage metrics)
> remains deferred.

- **Slice 1 — artifact + gate (no daemon method).** `collaboration_ledger.v1.1`
  additive schema; the shape-scoped `adjudicated_constraint_extraction` +
  `needs_revision ⇒ non-empty constraints[]` publish gate; contract accepts the
  advertised clearing dispositions (#88) and natural ledger front matter (#79).
  Pure contract/validation work.
- **Slice 2 — shape fixture + generator.** `adjudicated_constraint_extraction`
  registered in the collaboration shape pack; `workflow generate --shape …`
  emits the 8-phase graph; starter fixture under
  `examples/adjudicated-constraint-extraction-flow/`. Requires cycle-aware
  logical names (#84) so `forum_synthesis_${cycle}` republish does not collide.
- **Slice 3 — discharge-verifying final review.** Final-review job type/prompt +
  the `constraint_discharge` finding shape; `final_review` fails closed on
  undischarged binding constraints.
- **Slice 4 (deferred).** First-class constraint objects (§6) + coverage metrics
  (§7).

Slices 1–3 are the V1 target; slice 4 is explicitly deferred, mirroring how
RFC 0093 shipped `falsification_gate`/`cross_examination` first and deferred
`fog_of_war_review`/`synaptic_prune`.

## Acceptance Criteria

1. `workflow generate --shape adjudicated_constraint_extraction` produces a
   `striatum.workflow.v1.1` graph that passes `workflow validate` **and**
   `run.prepare` (same phase rules — see #66), with exactly one `phase_synthesis`
   job per declared phase.
2. `review.submit` / `publish-artifact` rejects an
   `adjudicated_constraint_extraction` `collaboration_ledger.v1.1` whose
   `verdict: needs_revision` carries an empty `constraints[]` (exit code 6), and
   accepts one with ≥1 binding constraint or unresolved-question row.
3. The contract accepts the clearing verbs it advertises and natural ledger front
   matter (closes #88 / #79 for this shape).
4. A revision-synthesis packet presents the prior cycle's `constraints[]` as
   first-class inputs; republishing the synthesis under the same logical name
   across cycles succeeds (depends on #84).
5. `final_review` fails closed when any `binding: true` constraint is `missing` or
   unaccepted-`partial`, and passes when all are `discharged` or `accepted_risk`
   with owner/stage — without re-running prior phases.
6. The run dashboard can render the posture-disposition matrix and constraint-
   coverage counts (§7).
7. `collaboration_ledger.v1.1` is additive: every valid `v1` ledger still
   validates.

## Open Questions

1. **`v1.1` vs `v2`.** Is the constraint table additive enough to stay `v1.1`, or
   does `binding`/`final_review_required` semantics warrant `v2`? Default: `v1.1`,
   additive, since `v1` ledgers stay valid (AC 7).
2. **Adjudicator absorbing the cycle (#77).** Should the `needs_revision` verdict
   route directly back to `revision_synthesis` (adjudicator absorbs it) rather
   than spawning a checkpoint? This RFC assumes #77's resolution; the shape wants
   absorption.
3. **Cross-phase edge to a later `phase_synthesis` (#66).** `run.prepare`
   currently rejects edges targeting a later phase's synthesis job. The
   `adjudication → revision_synthesis` constraint edge must be expressed within
   that rule (or the rule relaxed for same-cycle revision). Resolve before
   slice 2.
4. **First-class constraint objects threshold (§6).** What is the second consumer
   that justifies the RPC family? Likely a downstream implementation workflow that
   reads a design run's constraints. Defer until it exists.
5. **Posture set governance.** Ship the default five postures as a pack, or let
   each workflow declare its own with no default? Default: a pack with override.

## Domain Modeling

New/extended ubiquitous-language terms (to land in
`docs/reference/ubiquitous-language.md` with the V1 slice):

- **Adjudicated constraint-extraction loop** — a collaboration workflow shape in
  which adjudicator refusal compiles objections into binding constraints that the
  next revision must discharge and final review must verify.
- **Binding constraint** — a typed, sourced obligation extracted from a
  load-bearing objection, carried forward across revision cycles, that final
  review checks for discharge.
- **Constraint discharge** — the verified landing of a binding constraint in the
  final artifact as testable text or a gate (`discharged` / `partial` /
  `missing` / `accepted_risk`).
- **Productive refusal** — a `needs_revision` adjudication that is required to
  emit constraints; the structural successor to RFC 0093's "honest refusal".
- **Posture-disposition matrix** — the per-posture branch status table
  (`cleared` / `cleared_with_constraints` / `blocked`) the adjudicator maintains
  across cycles.
