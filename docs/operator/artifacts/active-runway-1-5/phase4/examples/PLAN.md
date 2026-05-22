---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["rfc_0074", "rfc_0076", "workflow_types_guide", "examples_readme", "rfc_0076_audit_workflow", "rfc_0076_catalog_followup_plan"]
---

# RFC 0074 Phase A Runnable Examples Plan
author: example-planner-claude-code-001
status: open
date: 2026-05-22

## Summary

RFC 0074 Phase A is metadata-plus-one-example: docs and a small starter set
of runnable example workflows that demonstrate the catalog vocabulary
(graph shape + role pack + adversary pack) on top of current primitives.
This plan picks the first runnable example to add, names every artifact
the example must produce, and lists the validation tests that pin both
the workflow contract and the on-disk files it references. It also
defines how the hand-authored RFC 0076 code/docs audit workflow should be
generalized into a `code_doc_audit` catalog entry later, without changing
its graph in Phase A.

This plan does not modify the bundled catalog metadata, the workflow
generator, the workflow chooser, or any schema registry. It does not
add a `striatum.audit_finding.v1` schema. It does not introduce a
findings issue queue in the operator UI. Phase A keeps every change
either in `examples/` or under tests that gate the example. Catalog
metadata wiring (CAT-001 and CAT-002 from the RFC 0076 follow-up plan)
remains a separate Phase A change tracked under that follow-up.

## Goals

- Land the first new runnable example named in RFC 0074 §2 so the
  catalog vocabulary has at least one concrete reference workflow that
  validates today.
- Make the example useful as a hand-authored fixture without RFC 0052
  debate artifacts, RFC 0034 generator support, or new schemas.
- Cover the example with tests at the same level the existing
  `examples/three-lane-design-build-review/` fixture is covered, so the
  catalog vocabulary cannot silently regress.
- Define the eventual `code_doc_audit` generalization shape so the
  Phase A catalog metadata pass can pick it up without re-deriving the
  intent from RFC 0076.

## Non-Goals

- Adding catalog metadata entries for `implementation_panel`,
  `strategy_review`, `premortem`, or `code_doc_audit`. That is the
  separate RFC 0074 Phase A catalog metadata pass; this plan only
  scaffolds the runnable example side of Phase A.
- Implementing RFC 0052's typed debate artifacts. The
  implementation-panel example uses existing `handoff`, `finding`,
  `findings_ledger`, `synthesis`, and `decision` kinds only.
- Generalizing `code_doc_audit` in source. This plan only names the
  generalization shape so a future change can act on it.
- Promoting any RFC 0076 Open Questions to first-class product surfaces.
  Specifically, no `striatum.audit_finding.v1` schema and no operator-UI
  finding queue.
- Expanding the example set beyond one fixture. RFC 0074 Phase A names
  five candidate examples; this plan ships the first and leaves the
  remaining four (`strategy-review-flow/`, `premortem-flow/`,
  `release-readiness-flow/`, `incident-response-flow/`) to follow-up
  Phase A work once `implementation-panel-flow/` is stable.

## First Example To Add

### Choice

Add `examples/implementation-panel-flow/` as the first RFC 0074 Phase A
runnable example.

### Justification

1. RFC 0074 §2 lists five candidate first examples and §3 calls out
   `implementation_panel` as the lightweight panel shape that can run on
   current primitives before RFC 0052 lands. The shape is the most
   load-bearing for the catalog vocabulary because it pairs a graph
   shape (`implementation_panel`), a role pack
   (`implementation_panel`: proposer_a/b/c, scorekeeper, arbitrator,
   dissent_reviewer), and an adversary pack (`maintainer_cost`) in one
   workflow.
2. It exercises the multi-review fan-in shape already covered by
   `examples/three-lane-design-build-review/`, but adds explicit
   scorecards, an arbitrator decision, and a dissent gate. That gives
   the catalog a usable reference for "decide between options" without
   requiring committee debate artifacts.
3. The other four candidate examples (`strategy_review`, `premortem`,
   `release_readiness`, `incident_response`) all reuse pieces of the
   panel shape (fan-in reviews, an arbitrator/decision step). Landing
   the panel first lets later examples adapt its layout instead of
   each one introducing fresh prompt and artifact patterns.
4. The shape can be validated by `striatum workflow validate` against
   the existing v1 schema. No new artifact kind, no new lane shape, and
   no new cycle policy is required.

### Workflow Shape

The example matches RFC 0074 §3:

```
problem_brief
  -> proposal_a, proposal_b, proposal_c          (parallel)
  -> scorecard_a, scorecard_b, scorecard_c       (parallel, fresh reviewers)
  -> tradeoff_ledger
  -> arbitrator_synthesis
  -> dissent_review
  -> decision
```

| Job ID | Type | Role | Lane | Notes |
|---|---|---|---|---|
| `frame_problem` | synthesis | `problem_framer` | `local` | Publishes the problem brief that proposals consume. |
| `propose_option_a` | synthesis | `proposer_a` | `local` | Independent implementation proposal A. |
| `propose_option_b` | synthesis | `proposer_b` | `local` | Independent implementation proposal B. |
| `propose_option_c` | synthesis | `proposer_c` | `local` | Independent implementation proposal C. |
| `score_option_a` | review | `scorekeeper` | `local` | Fresh-session scorecard against fixed dimensions. |
| `score_option_b` | review | `scorekeeper` | `local` | Fresh-session scorecard against fixed dimensions. |
| `score_option_c` | review | `scorekeeper` | `local` | Fresh-session scorecard against fixed dimensions. |
| `compile_tradeoffs` | synthesis | `tradeoff_ledger` | `local` | Materializes the support ledger. |
| `arbitrate` | synthesis | `arbitrator` | `local` | Resolves by criteria/evidence, not by vote average. |
| `review_dissent` | review | `dissent_reviewer` | `local` | One bounded dissent review; verdicts can route back to `arbitrate` once. |
| `record_decision` | synthesis | `principal_decider` | `local` | Publishes the final decision artifact. |

Edges:

- `frame_problem -> propose_option_a | propose_option_b | propose_option_c`
- `propose_option_a -> score_option_a` (and analogous for b, c)
- `score_option_a | score_option_b | score_option_c -> compile_tradeoffs`
- `compile_tradeoffs -> arbitrate -> review_dissent -> record_decision`
- One bounded cycle: `review_dissent -> arbitrate` on
  `needs_revision`, `max_iterations: 1`, `allow_same_lane: true`.

Lane shape: single `local` process lane mirroring the existing
`examples/docs-review-flow/` and `examples/human-checkpoint-flow/`
starter pattern. The example does not bind specific model providers.
The catalog metadata addition (CAT-001/CAT-002 follow-up) will later
declare `role_pack: implementation_panel` and
`adversary_pack: maintainer_cost`; the example fixture itself stays
single-lane so the test runs without provider credentials.

### Required Prompts

All prompts live under `examples/implementation-panel-flow/prompts/`
and are referenced from each job's `task_prompt.path`:

- `prompts/frame_problem.md`
- `prompts/propose_option.md` (shared by `propose_option_a/b/c`)
- `prompts/score_option.md` (shared by `score_option_a/b/c`)
- `prompts/compile_tradeoffs.md`
- `prompts/arbitrate.md`
- `prompts/review_dissent.md`
- `prompts/record_decision.md`

Each prompt declares its scorecard dimensions inline so the example is
self-contained. The fixed dimension list mirrors RFC 0074 §3:
`correctness, simplicity, migration_risk, testability,
operator_ergonomics, cost, performance, reversibility,
security_privacy, maintainability`.

### Required Context Docs

The fixture lists a small, generic context bundle so it can be read on
any target repo. None of these are required by core; they are the
authoring hints the prompts reference:

- `README.md`
- `docs/INDEX.md`
- `docs/UBIQUITOUS_LANGUAGE.md`
- `docs/WORKFLOW_TYPES.md`
- `docs/HOW_TO_AGENT.md`
- `docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md`

The example does not reference RFC 0052 in its context docs to avoid
implying that this fixture implements committee deliberation.

### Expected Artifacts

All artifact paths live under
`examples/implementation-panel-flow/artifacts/` so the example is
self-contained and does not write into `docs/`:

| Job | Logical name | Kind | Path |
|---|---|---|---|
| `frame_problem` | `problem_brief` | `handoff` | `artifacts/PROBLEM_BRIEF.md` |
| `propose_option_a` | `proposal_a` | `handoff` | `artifacts/proposals/PROPOSAL_A.md` |
| `propose_option_b` | `proposal_b` | `handoff` | `artifacts/proposals/PROPOSAL_B.md` |
| `propose_option_c` | `proposal_c` | `handoff` | `artifacts/proposals/PROPOSAL_C.md` |
| `score_option_a` | `scorecard_a` | `finding` | `artifacts/scorecards/SCORECARD_A.md` |
| `score_option_b` | `scorecard_b` | `finding` | `artifacts/scorecards/SCORECARD_B.md` |
| `score_option_c` | `scorecard_c` | `finding` | `artifacts/scorecards/SCORECARD_C.md` |
| `compile_tradeoffs` | `tradeoff_ledger` | `findings_ledger` | `artifacts/TRADEOFF_LEDGER.md` |
| `arbitrate` | `arbitrator_synthesis` | `synthesis` | `artifacts/ARBITRATOR_SYNTHESIS.md` |
| `review_dissent` | `dissent_review` | `finding` | `artifacts/DISSENT_REVIEW.md` |
| `record_decision` | `decision` | `decision` | `artifacts/DECISION.md` |

Each front-matter–carrying artifact uses its V1 schema unchanged. The
`findings_ledger`, `synthesis`, and `decision` schemas are the same
ones used elsewhere in the repo; the example does not introduce any
new schema or schema field.

### Write Scopes

Per-job write scopes keep proposers, scorers, ledgerers, the
arbitrator, the dissent reviewer, and the decider in disjoint
directories so `require_disjoint_write_scopes` parallelism can run:

| Job | `allowed_paths` (relative to repo root) |
|---|---|
| `frame_problem` | `examples/implementation-panel-flow/artifacts/PROBLEM_BRIEF.md` |
| `propose_option_a` | `examples/implementation-panel-flow/artifacts/proposals/PROPOSAL_A.md` |
| `propose_option_b` | `examples/implementation-panel-flow/artifacts/proposals/PROPOSAL_B.md` |
| `propose_option_c` | `examples/implementation-panel-flow/artifacts/proposals/PROPOSAL_C.md` |
| `score_option_a` | `examples/implementation-panel-flow/artifacts/scorecards/SCORECARD_A.md` |
| `score_option_b` | `examples/implementation-panel-flow/artifacts/scorecards/SCORECARD_B.md` |
| `score_option_c` | `examples/implementation-panel-flow/artifacts/scorecards/SCORECARD_C.md` |
| `compile_tradeoffs` | `examples/implementation-panel-flow/artifacts/TRADEOFF_LEDGER.md` |
| `arbitrate` | `examples/implementation-panel-flow/artifacts/ARBITRATOR_SYNTHESIS.md` |
| `review_dissent` | `examples/implementation-panel-flow/artifacts/DISSENT_REVIEW.md` |
| `record_decision` | `examples/implementation-panel-flow/artifacts/DECISION.md` |

All jobs declare `forbidden_paths: [".striatum/"]`. The workflow does
not write outside `examples/implementation-panel-flow/`.

### Validation Commands

The example must pass:

```bash
PYTHONPATH=src python3 -m striatum.cli workflow validate \
  examples/implementation-panel-flow/workflow.json

PYTHONPATH=src python3 -m striatum.cli workflow templates list

make lint
make typecheck
make test PYTEST_ARGS="-k implementation_panel"
```

The `workflow validate` invocation pins the shape against the current
`striatum.workflow.v1` schema. `workflow templates list` is included
because the catalog metadata follow-up will later add an entry that
references this fixture; this plan does not invoke generation, only the
list verb to confirm the catalog still loads.

## Generalizing `code_doc_audit` Later

RFC 0076 §7 already names a generated `code_doc_audit` shape. The RFC
0076 catalog follow-up plan (CAT-001 + CAT-002) defers that to the RFC
0074 Phase A catalog metadata pass. This plan defines the
generalization contract so the metadata pass can be a metadata-only
change.

### Source Of Truth

The hand-authored
`docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json` stays
the source of truth for the graph. No edits to that workflow are part
of this plan. When the catalog metadata pass runs, it must reproduce
the same graph and the same expected artifacts.

### Graph Shape Entry

Add a new shape `code_doc_audit` with this contract:

- **Nodes**: `audit_authority_runtime`, `audit_docs_decision_drift`,
  `audit_operator_adoption` (parallel) → `synthesize_audit` →
  `write_remediation_plan`.
- **Cycles**: none. The audit shape is non-cyclic; remediation closure
  is a separate workflow.
- **Required options at generate time**: `workflow_id`, `artifact_root`,
  `lane_set` (typically a three-lane fan-out), and the three
  per-lane prompt paths.
- **Default expected artifact kinds**: `finding` per audit lane,
  `synthesis` for the synthesis job, `synthesis` for the remediation
  plan.
- **Parallelism defaults**: `mode: declared`, `max_active_jobs: 3`,
  `require_disjoint_write_scopes: true`.

### Role Pack `authority_docs_operator_audit`

The catalog metadata pass adds a role pack matching the existing
hand-authored roles in
`docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json`:

- `coordinator`
- `authority_auditor`
- `docs_auditor`
- `operator_auditor`
- `synthesizer`
- `planner`

The pack ships role descriptions only; no new role definition files are
generated. Prompts remain repository-supplied.

### Adversary Pack Mapping

Per the RFC 0076 catalog follow-up plan, either:

1. Add a dedicated `code_doc_audit_postures` adversary pack containing
   `authority_drift`, `docs_drift`, and `operator_ergonomics`; or
2. Merge `operator_ergonomics` with the RFC 0074 adversary pack of the
   same name and treat `authority_drift` and `docs_drift` as
   audit-specific postures inside a `code_doc_audit_postures` pack.

The metadata pass picks the merged option (2) to avoid duplicating
`operator_ergonomics`. This is the recommendation; the metadata pass
is free to revisit if naming pressure changes.

### What Phase A Must Not Add For `code_doc_audit`

- No `striatum.audit_finding.v1` schema. RFC 0076 Open Question 1 is
  resolved as no-action by the RFC 0076 catalog follow-up plan (CAT-003).
  This plan does not reverse that resolution. The existing `finding` /
  `findings_ledger` schemas remain the audit shape's contract.
- No operator-UI findings queue, finding browser, or per-finding RPC.
  RFC 0076 Open Question 5 is resolved as no-action by the same
  follow-up (CAT-004). Existing artifact storage and `docs/issues/<N>/`
  already cover finding promotion when an audit run produces work that
  deserves issue tracking.
- No changes to the hand-authored workflow at
  `docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json`.
  Generation must be additive; it produces a new tree under a chosen
  `artifact_root`, not edits to the existing operator workflow.

## Tests

The example workflow must be covered by tests at the same level as
`examples/three-lane-design-build-review/` (see
`tests/test_example_workflows.py`). Add a new test module
`tests/test_implementation_panel_flow.py` with these named tests:

| Test | What it asserts |
|---|---|
| `test_implementation_panel_flow_workflow_validates` | `load_workflow(...)` plus `validate_workflow(...)` succeed against the v1 schema. |
| `test_implementation_panel_flow_jobs_and_edges` | Job ids match the eleven jobs listed above; edges and the bounded `review_dissent -> arbitrate` cycle match the spec; cycle has `max_iterations: 1` and `on_verdict: needs_revision`. |
| `test_implementation_panel_flow_referenced_files_exist` | Every `task_prompt.path` and every `context_docs[*].path` resolves to an existing file on disk (mirrors the existing three-lane fixture test). |
| `test_implementation_panel_flow_artifact_paths_disjoint` | Every job's `expected_artifacts[*].path` is under `examples/implementation-panel-flow/artifacts/`, the write scopes are disjoint, and `.striatum/` is in `forbidden_paths` for each job. |
| `test_implementation_panel_flow_no_new_artifact_kinds` | Every expected artifact `kind` is a known v1 kind (`handoff`, `finding`, `findings_ledger`, `synthesis`, `decision`); no novel kind is introduced. |

Add one update to the existing example workflow test surface:

| Test (existing or extended) | What it asserts |
|---|---|
| `tests/test_example_workflows.py::test_all_example_workflow_files_validate` | A parametrized variant (or a new sibling test in the same file) that walks `examples/*/workflow.json`, loads each, and validates each. This change ensures any future RFC 0074 Phase A example automatically inherits validation coverage. If a strict-equivalent test already exists under a slightly different name, extend it instead of duplicating; do not add a second walker. |

The `tests/test_workflow_generator.py` module is not part of this
change. The catalog metadata follow-up may add tests there; this plan
does not.

## Phasing And Sequencing

1. **Now (this plan)**: scope the work, name the example, name the
   tests, name the `code_doc_audit` generalization contract. No source
   or catalog change.
2. **Next Phase A unit (out of scope for this artifact, but the
   immediate successor):** scaffold
   `examples/implementation-panel-flow/` with the workflow JSON,
   prompts, and tests above. Land it as a single bounded change.
3. **Following Phase A unit:** the RFC 0074 catalog metadata pass
   (CAT-001 + CAT-002 from the RFC 0076 follow-up) wires the
   `implementation_panel` shape, the `implementation_panel` role pack,
   the `maintainer_cost` adversary pack, and the `code_doc_audit`
   shape/role/adversary entries into the bundled template catalog and
   updates `docs/WORKFLOW_TYPES.md`. The example fixture from step 2
   continues to validate without change.
4. **Later Phase A units:** add the remaining four RFC 0074 examples
   (`strategy-review-flow/`, `premortem-flow/`,
   `release-readiness-flow/`, `incident-response-flow/`), each as its
   own bounded change with its own test module modeled on
   `tests/test_implementation_panel_flow.py`.

## References

- `docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md` — Phase
  A scope, §2 candidate shapes, §3 implementation panel graph, §4-5
  role/adversary packs.
- `docs/rfcs/0076-three-lane-code-and-doc-audit-workflow.md` — `code_doc_audit`
  shape source of truth (referenced for generalization, not edited).
- `docs/operator/artifacts/rfc-0076-audit-remediation/catalog-followup/PLAN.md`
  — CAT-001, CAT-002 deferred to RFC 0074 Phase A; CAT-003, CAT-004
  resolved as no-action.
- `docs/WORKFLOW_TYPES.md` — current operator-facing guide that names
  the existing workflow types.
- `examples/README.md` — the index this plan's example will join.
- `tests/test_example_workflows.py` — coverage shape this plan's tests
  mirror.
- `docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json` —
  current hand-authored audit workflow; unchanged by this plan.
- `docs/operator/plans/active-runway-1-5.md` — the operator plan that
  schedules Phase 4 (RFC 0074 Phase A) work.
