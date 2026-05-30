# RFC 0098 Design - Codex Lane

author: designer-codex-gpt-5.5-xhigh-001
date: 2026-05-30
status: draft

## Problem Framing

RFC 0093 made live collaboration refusals honest: an adjudicator can publish a
`collaboration_ledger` with `verdict: needs_revision`, and the downstream gate
does not clear. That is still too weak for RFC-writing and specification work.
The refusal blocks a bad output, but it does not preserve the useful part of the
disagreement as obligations that the next revision must satisfy.

The missing property is productive refusal. If an adjudicator says
`needs_revision`, the ledger must compile the load-bearing objections into
typed, sourced constraints. The revision author should receive those
constraints as the work to discharge, and final review should verify discharge
instead of re-running the whole forum.

The smallest safe slice is therefore contract-first: extend the existing
`collaboration_ledger` front-matter schema additively, and put the
`needs_revision => constraints present` rule in the existing publish-time
validation path. That uses the current daemon boundary and does not require a
new method, table, or state aggregate.

## Proposed Approach

### Slice 1: `collaboration_ledger.v1.1` and Productive-Refusal Gate

Implement the schema in `go/pkg/artifactcontracts/contracts.go`, inside the
existing `Schemas["collaboration_ledger"]` entry and
`validateCollaborationLedger`.

The schema should remain one artifact kind, not a parallel `constraint_ledger`
kind:

- Accept `schema_version: "striatum.collaboration_ledger.v1"` and
  `schema_version: "striatum.collaboration_ledger.v1.1"`.
- Keep every current V1 required field valid as-is: `shape`, `topic`,
  `participants`, `entries`, `verdict`, and `rationale`.
- Add optional V1.1 fields: `cycle`, `constraints`, and `branches`.
- Add `adjudicated_constraint_extraction` to the accepted `shape` values.
- Keep raw-output guard behavior by retaining strict unknown-field rejection.

The productive-refusal rule should be version-aware:

- V1 ledgers continue to validate, including existing `needs_revision` ledgers
  with only `entries`.
- V1.1 ledgers with `verdict: needs_revision` must contain at least one
  productive row in `constraints`.
- A productive row is either `binding: true` or `kind: unresolved_question`.

Recommended `constraints` row shape:

```yaml
constraints:
  - id: C2-IMPL-1
    source_finding: IMPL-1
    source_refs: ["dialogue:2"]
    posture: implementation
    severity: high
    kind: invariant
    binding: true
    text: "claims.evidence_ids may reference raw evidence only."
    verification:
      expected_stage: "final_review"
      gate: "mixed raw+derived provenance fails closed"
    final_review_required: true
```

Recommended enum boundaries:

- `severity`: `low`, `medium`, `high`, `critical`
- `kind`: `invariant`, `gate`, `schema`, `policy`, `non_goal`,
  `accepted_risk`, `unresolved_question`
- `posture`: non-empty string, not a closed global enum, because RFC 0098 allows
  workflow-authored posture sets

Recommended `branches` shape should accept the natural front matter people will
write:

```yaml
branches:
  implementation: cleared_with_constraints
  privacy: cleared_with_constraints
  eval: cleared
```

That is a mapping from posture name to disposition. Accept an array form only if
the synthesis wants stronger machine readability:

```yaml
branches:
  - posture: implementation
    disposition: cleared_with_constraints
    constraint_ids: [C2-IMPL-1]
```

The accepted branch dispositions should be `cleared`,
`cleared_with_constraints`, `blocked`, `blocked_pending_answer`, and
`defer_with_successor`. Do not accept ambiguous bare `clear`.

The "advertised verdict" cleanup should be handled carefully. I would keep the
runtime verdict vocabulary unchanged for slice 1: `accept`,
`accept_with_findings`, `needs_revision`, and `reject`. The two refined states
from RFC 0098, `blocked_pending_answer` and `defer_with_successor`, should live
as branch dispositions first. Widening `recordVerdict` to new top-level verdicts
is not "pure contract/validation work"; it changes gate semantics and should
only land if the synthesis explicitly accepts that blast radius.

Natural front matter should be accepted through the existing YAML parser:
multiline lists, nested maps, unquoted scalar values, and ordinary YAML mapping
syntax. Add regression tests for this rather than introducing a second parser.

The hook path is already present:

- `artifact.publish` calls `validateArtifactFrontMatter`, which delegates to
  `artifactcontracts.ValidateFrontMatter`.
- `review.submit` pre-validates a submitted `collaboration_ledger`, then calls
  `publishArtifact`.
- `review.verdict` / `recordVerdict` calls `enforceCollaborationLedgerVerdict`,
  which parses and validates the required ledger before allowing the primitive
  publish-then-verdict path to clear the gate.

Putting the new rule in `validateCollaborationLedger` gives the same behavior on
all three paths without adding a daemon method. Invalid front matter remains an
`artifact_error` surfaced as publisher exit 6 through the CLI fallback.

Tests for slice 1 should land before code:

- V1 ledger without `constraints` still validates.
- V1.1 clearing ledger with no `constraints` validates if it has the existing
  claim/challenge/rebuttal substance rows.
- V1.1 `needs_revision` ledger with empty or missing `constraints` is rejected.
- V1.1 `needs_revision` ledger with one binding constraint is accepted.
- V1.1 `needs_revision` ledger with one `unresolved_question` row is accepted.
- Unknown top-level fields still fail, preserving the no-stdout guard.
- Natural multiline YAML front matter is accepted.
- `review.submit` and primitive `publish-artifact` plus `review.verdict` both
  reject an invalid productive-refusal ledger.

### Slice 2: Generator Shape and Fixture

Add `adjudicated_constraint_extraction` to the existing collaboration generator
surface in `go/pkg/workflowgenerate/generate.go`:

- `shapes`
- `isCollaborationShape`
- `compileCollaborationShape`
- catalog entry in `go/pkg/workflowtemplates/catalog.json`
- generator tests in `go/pkg/workflowgenerate/generate_test.go`
- catalog tests in `go/pkg/workflowtemplates/catalog_test.go`
- fixture under `examples/adjudicated-constraint-extraction-flow/`

The generated graph should use `striatum.workflow.v1.1` and eight phases:
`survey`, `convener_synthesis`, `cross_exam`, `adjudication`,
`revision_synthesis`, `constraint_discharge_review`, `spec_publication`, and
`final_review`.

There is a concrete phase-edge risk in `go/pkg/mutations/run.go`: cross-phase
edges may only target the immediate next phase and may not target a later
`phase_synthesis` job. To stay legal without relaxing `run.prepare`, the
`adjudication` phase synthesis should depend on a first non-synthesis intake job
inside the `revision_synthesis` phase, for example
`revision_constraints_intake`; that job then feeds the phase synthesis job
inside its own phase. If the intended shape requires a direct edge to a later
`phase_synthesis`, defer slice 2 until #66 is fixed.

Do not start slice 2 until #84's republish dependency is resolved for every
artifact that re-publishes across cycles. The current `${cycle}` resolver already
protects collaboration ledgers, but RFC 0098 also needs cycle-aware logical names
or attempt-scoped artifacts for revised convener/spec outputs.

### Slice 3: Constraint-Discharge Final Review

Keep this as stretch after slice 1 and the generator fixture are green. The
least surprising shape is not a new artifact kind. It should be a normal
`finding` or `findings_ledger` with an optional `constraint_discharge` front
matter block, validated by `go/pkg/artifactcontracts`.

A final-review gate can then enforce:

- every `binding: true` and `final_review_required: true` constraint from the
  latest clearing ledger appears in `constraint_discharge`;
- `status: missing` fails;
- `status: partial` fails unless paired with `accepted_risk` and an owner/stage;
- `status: discharged` passes with evidence;
- `status: accepted_risk` passes only with owner and stage.

This should not re-run cross-exam. The reviewer verifies the constraint table
against the published spec and records a verdict from that typecheck.

## Alternatives Considered

1. `v2` ledger schema: Too much blast radius. The requested behavior is
   additive, and current V1 ledgers must keep validating. A V2 schema would make
   RFC 0093 fixtures and examples look obsolete for no runtime benefit.

2. First-class `constraint.*` daemon objects now: This is the right eventual
   model if constraints become cross-run inputs, but V1 has one consumer: the
   next phase in the same run. Artifact validation is enough, cheaper, and
   consistent with RFC 0093's substrate.

3. Put the productive-refusal check only in `review.submit`: That misses the
   primitive path where an adjudicator can publish the ledger and then call
   `review.verdict`. RFC 0093 already closed that bypass for verdict mismatch;
   RFC 0098 should extend the same shared validation path.

## Risks and Unknowns

- The schema can accidentally stop being additive. Guard this with a V1 fixture
  test that validates an unmodified RFC 0093 ledger.
- The branch/verdict vocabulary can leak into daemon job states. Keep
  `blocked_pending_answer` and `defer_with_successor` as ledger dispositions
  unless the implementation intentionally widens `recordVerdict` with explicit
  state-machine tests.
- `branches` has two plausible shapes: map and array. Accepting both is a small
  validation cost and avoids re-opening #79 over natural YAML authoring.
- Slice 2 may fail `run.prepare` if the graph targets a later `phase_synthesis`.
  Use an intake job in the next phase, or defer until #66 is fixed.
- Slice 2 also depends on #84. `${cycle}` is present for collaboration ledgers,
  but revised synthesis/spec artifacts need the same protection.
- Final-review enforcement can become semantic theater if it only checks that
  strings are present. Keep V1 structural: each constraint id has a discharge
  row, status, and evidence pointer. The reviewer still owns quality judgment.

## Rollout

First, land slice 1 only: contract helpers, front-matter tests, mutation-path
tests, and docs for the new ledger fields. This is the smallest blast radius and
directly satisfies productive refusal without a new daemon method.

Second, add the generator shape and example fixture only after #84 and the
phase-edge rule are known-safe. Validate with both `workflow validate` and
`run.prepare`, not just generator unit tests.

Third, add constraint-discharge final review as a separate structural gate once
the shape can produce and carry constraints reliably.
