# RFC 0074 Phase A Catalog Build Handoff
author: operator [self-declared: codex-operator]
artifact_kind: handoff
status: complete
date: 2026-05-22

## Summary

Commit `89858a0` landed the narrow RFC 0074 Phase A catalog discovery slice.
The implementation keeps role packs, adversary packs, and the
`implementation_panel` shape as read-only authoring metadata over ordinary
workflow templates. It does not add runtime semantics, daemon state, model or
lane identity, workflow validation requirements, or generator behavior for the
new packs.

## What Landed

- `src/striatum/workflow_templates/catalog.json` and
  `go/pkg/workflowtemplates/catalog.json` now carry `role_packs` and
  `adversary_packs` collections beside shapes and lane sets.
- The `implementation_panel` shape is catalog-visible with
  `generation_status: "example_only"`,
  `example_workflow_path: "examples/implementation-panel-flow/workflow.json"`,
  `role_packs: ["implementation_panel_roles"]`, and adversary packs
  `maintainer_cost` plus `operator_ergonomics`.
- The initial pack entries are:
  `implementation_panel_roles`, `release_readiness`, `incident_response`,
  `maintainer_cost`, `operator_ergonomics`, and `security_privacy`.
- Python catalog loading/listing now validates the new collections and accepts
  `kind` filters for `role_pack` and `adversary_pack`; catalog Markdown renders
  dedicated Role Packs and Adversary Packs sections.
- Go catalog loading/listing mirrors the Python reader and the Go read handler
  accepts the expanded kind vocabulary.
- CLI discovery allows
  `workflow templates list --kind {shape,lane_set,role_pack,adversary_pack}`.
  Existing service/web template-list responses inherit the new filters through
  the shared catalog list path.
- The commit also added the RFC 0074 Phase A operator plan and workflow
  scaffold under `docs/operator/plans/` and `docs/operator/workflows/`.

## Validation Evidence

Validation evidence in `89858a0` is test-backed rather than a new command run
from this handoff-only packet:

- `tests/test_workflow_generator.py` checks that `implementation_panel` is
  listed, remains `example_only`, role/adversary pack filters return the new
  entries, Markdown render includes the new sections, and the CLI JSON list
  path handles `--kind role_pack`.
- `tests/test_workflow_generation_web.py` checks that the service template
  response lists role packs.
- `go/pkg/workflowtemplates/catalog_test.go` checks the Go catalog loader sees
  `implementation_panel` and `implementation_panel_roles`, preserves sorted
  listing behavior, and filters role packs by kind.
- `go/pkg/reads/workflow_templates_test.go` checks the Go read handler kind
  filter with `role_pack`.
- Existing implementation-panel fixture coverage in `tests/test_example_workflows.py`
  validates `examples/implementation-panel-flow/workflow.json`, referenced
  files, disjoint artifact paths, and use of existing artifact kinds.

I did not rerun tests in this packet because the requested scope was to write
only this handoff artifact after the implementation had already landed.

## Phase B Deferrals

All generation and selection behavior remains deferred: expanded
`workflow generate --shape` support, `--role-pack`, `--adversary-pack`,
`proposal_count`, `score_dimensions`, web chooser pack selectors, and
cost/artifact-volume estimation.

RFC 0052 debate or panel artifact schemas remain out of this slice. Packs must
also stay out of workflow validation inputs, daemon state, lane/model identity,
artifact schemas, and runtime gates until a later accepted implementation owns
those semantics.

The broader RFC 0074 candidate catalog from
`discovery/PACK_DISCOVERY.md` remains future catalog work. The landed commit
exposes the `implementation_panel` example and starter packs only; additional
metadata-only shapes such as `strategy_review`, `premortem`,
`release_readiness`, `incident_response`, and `code_doc_audit` still need a
separate follow-up if they are promoted beyond discovery.
