---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md", "docs/rfcs/0076-three-lane-code-and-doc-audit-workflow.md", "docs/operator/artifacts/rfc-0076-audit-remediation/catalog-followup/PLAN.md", "src/striatum/workflow_templates/catalog.json", "src/striatum/workflow_generator/catalog.py", "docs/operator/plans/active-runway-1-5.md"]
---

# RFC 0074 Phase A Catalog Metadata Plan
author: catalog-planner-codex-001
status: open
date: 2026-05-22

## Summary

RFC 0074 Phase A should be a metadata-only catalog expansion. Add graph-shape,
role-pack, and adversary-pack entries to the bundled local catalog and update
catalog read/render tests, but do not make these packs runtime validation
concepts yet.

The smallest useful slice is:

1. promote a starter set of expanded graph-shape entries;
2. define catalog metadata for reusable role packs and adversary packs;
3. include accepted RFC 0076 `code_doc_audit` as a follow-on entry, not as the
   first broad-shape example;
4. extend catalog loading and list/render surfaces conservatively;
5. pin the metadata with tests that prove workflow validation behavior is
   unchanged.

## First Graph-Shape Entries

Add these Phase A graph-shape catalog entries first:

| Entry | Phase A status | Reason |
|---|---|---|
| `implementation_panel` | first | RFC 0074's named lightweight panel shape. It exercises the new catalog vocabulary without requiring RFC 0052 typed committee artifacts. |
| `strategy_review` | first | Broadens the catalog beyond code and RFC review into strategy/product/operator reasoning. |
| `premortem` | first | Makes adversarial failure-mode review visible as a reusable operator intent. |
| `release_readiness` | next | Useful operational workflow; validates that catalog entries can describe non-code gates. |
| `incident_response` | next | Useful disciplined closure shape; keep it metadata-only until examples prove the prompt set. |
| `code_doc_audit` | accepted follow-on | RFC 0076 is accepted and has run, but generator/catalog integration was explicitly deferred to RFC 0074 Phase A. Add it after the initial RFC 0074 breadth entries so the catalog does not become Striatum-internal first. |

Each shape entry should carry the same baseline fields current shape entries
use: `template_id`, `kind: "shape"`, `display_name`, `summary`,
`recommended_for`, `default_lane_sets`, `required_options`, and
`graph_preview`.

Use graph previews only. Do not add generator implementations for these shapes
in Phase A unless a separate packet explicitly owns generation.

## Role-Pack Metadata Shape

Add a top-level `role_packs` array to `catalog.json`. Each entry should be
catalog metadata, not runtime authority:

```json
{
  "pack_id": "implementation_panel",
  "kind": "role_pack",
  "display_name": "Implementation panel",
  "summary": "Several proposal authors, a scorekeeper, an arbitrator, and a dissent reviewer.",
  "recommended_for": ["implementation_panel"],
  "roles": [
    {
      "role_id": "proposer_a",
      "display_name": "Proposal author A",
      "summary": "Drafts one credible implementation option.",
      "default_job_types": ["implementation"],
      "default_artifact_kind": "handoff"
    }
  ],
  "default_graph_shapes": ["implementation_panel"]
}
```

Required fields for Phase A:

- `pack_id`: stable identifier, unique inside `role_packs`;
- `kind`: exactly `role_pack`;
- `display_name`, `summary`, `recommended_for`: same operator-facing pattern
  as existing template entries;
- `roles`: non-empty list with `role_id`, `display_name`, and `summary`;
- `default_graph_shapes`: optional list of shape ids that the loader checks
  only when present.

Initial role packs:

| Pack | Roles |
|---|---|
| `implementation_panel` | `proposer_a`, `proposer_b`, `proposer_c`, `scorekeeper`, `arbitrator`, `dissent_reviewer` |
| `strategy_panel` | `strategist`, `customer_persona`, `operator_persona`, `risk_reviewer`, `synthesizer`, `principal_decider` |
| `premortem` | `proposal_author`, `failure_mode_reviewer`, `mitigation_planner`, `rollback_reviewer`, `final_reviewer` |
| `release_readiness` | `release_manager`, `docs_reviewer`, `migration_reviewer`, `smoke_verifier`, `rollback_reviewer` |
| `incident_response` | `reproducer`, `root_cause_analyst`, `fix_planner`, `verifier`, `retrospective_author` |
| `authority_docs_operator_audit` | `authority_runtime_auditor`, `docs_decision_drift_auditor`, `operator_adoption_auditor`, `audit_synthesizer`, `remediation_planner` |

## Adversary-Pack Metadata Shape

Add a top-level `adversary_packs` array. Entries should describe pressure
patterns and, where possible, map to existing RFC 0018 review postures without
creating new posture validation.

```json
{
  "pack_id": "maintainer_cost",
  "kind": "adversary_pack",
  "display_name": "Maintainer cost",
  "summary": "Pressure from future-maintenance, simplicity, and testability angles.",
  "recommended_for": ["implementation_panel", "strategy_review"],
  "postures": [
    {
      "id": "future_maintainer",
      "display_name": "Future maintainer",
      "review_posture": "ergonomics_dx",
      "summary": "Looks for long-term maintenance and debugging burden."
    }
  ]
}
```

Required fields for Phase A:

- `pack_id`: stable identifier, unique inside `adversary_packs`;
- `kind`: exactly `adversary_pack`;
- `display_name`, `summary`, `recommended_for`;
- `postures`: non-empty list with `id`, `display_name`, and `summary`;
- `review_posture`: optional, and when present it must be one of the current
  first-class review postures or a valid `custom:<name>` string.

Initial adversary packs:

| Pack | Includes |
|---|---|
| `security_privacy` | security, privacy/redaction, supply-chain |
| `maintainer_cost` | future maintainer, cost/complexity, testability |
| `operator_ergonomics` | operator ergonomics, new-user/onboarding, observability/debuggability |
| `premortem` | failure mode, rollback, compatibility/migration |
| `product_strategy` | skeptical product, customer persona, time-to-ship |
| `provenance_integrity` | data integrity/provenance, evidence auditor, formal spec |
| `code_doc_audit_postures` | authority drift, docs drift, operator ergonomics |

Keep `operator_ergonomics` as the broad RFC 0074 pack and make
`code_doc_audit_postures` reference or include an operator-ergonomics posture
instead of redefining the same concept under a conflicting name.

## RFC 0076 Fit

RFC 0076 should land in Phase A as an accepted follow-on catalog entry:

- graph shape: `code_doc_audit`;
- role pack: `authority_docs_operator_audit`;
- default adversary pack: `code_doc_audit_postures`;
- lane set: `multi_review` preferred, with three fresh sessions acceptable
  when provider diversity is unavailable;
- generation: deferred unless the Phase A implementation packet explicitly
  chooses to implement generation for one expanded shape.

The RFC 0076 catalog-follow-up artifact already classified the generated
template and audit packs as deferred to RFC 0074 Phase A. The first operator
run proved the hand-authored shape works, so Phase A should preserve that
shape as metadata and avoid introducing a dedicated audit-finding schema or a
finding queue.

## Loader And Validation Changes

Extend `src/striatum/workflow_generator/catalog.py` in the narrowest way:

- keep `CATALOG_SCHEMA_VERSION` at `striatum.workflow_templates.v1` unless the
  implementation chooses to make older catalogs invalid;
- validate optional top-level arrays `role_packs` and `adversary_packs`;
- reject non-object entries, missing ids, duplicate ids within each pack
  array, and wrong `kind` values;
- validate `recommended_for` as a list of strings;
- validate role/posture child arrays are non-empty lists of objects with the
  required string fields;
- optionally check `default_graph_shapes` and `recommended_for` references
  against known shape ids, but do not require every entry to reference an
  implemented generator shape;
- add `list_role_packs()`, `get_role_pack()`, `list_adversary_packs()`, and
  `get_adversary_pack()` helpers.

Do not change `validate_workflow()`, generated workflow schema validation,
runtime job admission, review-posture enforcement, daemon state, or front
matter schemas for Phase A.

## CLI And Web Exposure

Expose packs in read-only catalog surfaces in Phase A, but do not add
generation flags yet.

Recommended CLI behavior:

- extend `workflow templates list --kind` to accept `role_pack` and
  `adversary_pack`;
- let unfiltered `workflow templates list` include shapes, lane sets, role
  packs, and adversary packs;
- extend `workflow templates show <id>` to search all catalog categories and
  fail on ambiguous ids if a pack and shape ever share an id;
- extend `workflow templates render-md` with separate sections for role packs
  and adversary packs.

Recommended web behavior:

- update `GET /workflow-templates` to include role/adversary packs in the same
  read-only response, or accept a `kind` filter for them;
- keep the workflow chooser's write path unchanged in Phase A;
- do not surface role/adversary pack selectors in the generation wizard until
  `workflow generate` supports them.

This keeps Phase A useful for discovery and docs while avoiding a UI that
appears to generate pack-aware workflows before the generator can honor the
selection.

## Tests

Add focused tests around catalog metadata only:

| Test area | Assertions |
|---|---|
| Catalog loader | Valid `role_packs` and `adversary_packs` load; duplicate pack ids, wrong kinds, missing child fields, and bad `review_posture` values fail with `GeneratorError`. |
| Template listing | `list_templates("role_pack")` and `list_templates("adversary_pack")`, or dedicated list helpers, return sorted metadata without mutating entries. |
| Markdown rendering | `render_catalog_markdown()` includes role-pack and adversary-pack sections and preserves graph Mermaid rendering for shape entries. |
| CLI list/show/render | `workflow templates list --kind role_pack`, `--kind adversary_pack`, `show implementation_panel`, and `render-md --stdout` include pack metadata. |
| Web catalog response | `workflow_templates_response()` includes the new metadata in read-only responses and filters by kind when requested. |
| Runtime semantics unchanged | Existing `test_builtin_shapes_validate` still covers the generated shapes only; add an explicit assertion that metadata-only shapes/packs do not become valid `workflow generate --shape` inputs unless implemented in generator core. |

Do not add workflow-validation tests that expect role/adversary packs to affect
job roles, postures, artifact kinds, lane binding, or daemon behavior. Phase A
is catalog metadata, not runtime semantics.

## Implementation Order

1. Add catalog JSON entries for the five RFC 0074 starter graph shapes plus
   `code_doc_audit`.
2. Add `role_packs` and `adversary_packs` arrays with the initial metadata
   above.
3. Extend the catalog loader and helper APIs with validation and copy-on-read
   behavior.
4. Extend markdown rendering, CLI template listing/showing, and read-only web
   catalog responses.
5. Add tests for metadata loading, exposure, rendering, and unchanged
   generation/runtime semantics.
6. Update `docs/WORKFLOW_TYPES.md` and `examples/README.md` in the
   implementation packet if that packet's write scope includes docs.

## Guardrails

- Do not add hosted template retrieval, target-repository catalog extensions,
  or external persistence.
- Do not treat role packs or adversary packs as provider/model identities.
- Do not make packs daemon state or workflow live state.
- Do not make metadata-only graph-shape entries accepted generator shapes until
  generator core implements them.
- Do not add new artifact front-matter schemas for RFC 0076 audit findings in
  this catalog phase.
- Do not change the RFC 0018 review-posture closed set except through a
  separate accepted decision or RFC.
