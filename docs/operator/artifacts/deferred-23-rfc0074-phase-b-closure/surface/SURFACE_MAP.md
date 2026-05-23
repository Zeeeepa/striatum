---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md", "docs/operator/plans/rfc-0074-workflow-shape-catalog-phase-a.md", "docs/operator/artifacts/rfc-0074-phase-a-catalog/CLOSURE.md", "examples/implementation-panel-flow/workflow.json", "src/striatum/workflow_generator/core.py", "go/pkg/workflowgenerate/generate.go", "src/striatum/service_routes.py", "tests/test_workflow_generator.py", "tests/test_workflow_generation_web.py"]
---

# RFC 0074 Phase B Surface Map
author: deferred23-surface-codex-gpt-5-001

## Phase A Baseline

Phase A landed read-only catalog metadata and one runnable example. The
`implementation_panel` catalog entry is visible with
`generation_status: "example_only"`, an example workflow path, one role pack,
and two adversary packs. The Phase A closure explicitly deferred generator
shape emission, role/adversary generation options, chooser pack selection,
`proposal_count`, `score_dimensions`, cost/artifact-volume warnings, and RFC
0052 debate/panel semantics.

The hand-authored `examples/implementation-panel-flow/workflow.json` validates
as ordinary `striatum.workflow.v1`. It uses existing job types and artifact
kinds: `handoff`, `finding`, `findings_ledger`, `synthesis`, and `decision`.
It does not require RFC 0052 typed debate artifacts or panel daemon methods.

## Generator Surface

Python generator support is still pre-Phase-B:

- `src/striatum/workflow_generator/core.py` keeps `SHAPES` limited to
  `minimal`, `review`, `code_change`, `human_checkpoint`, `evidence_backed`,
  `multi_review_synthesis`, `multi_phase`, and `custom`.
- The generator `OPTION_KEYS` set does not include `role_pack`,
  `adversary_pack`, `proposal_count`, or `score_dimensions`.
- `generate_workflow()` compiles shape and lane set directly to normal jobs,
  edges, cycles, roles, prompts, and workflow files. There is no runtime state
  or database dependency in the pure Python path.

Go generator support mirrors Python:

- `go/pkg/workflowgenerate/generate.go` has the same shape and option-key
  allowlists.
- Go daemon handlers for `workflow.generate.preview` and `workflow.generate`
  call the Go generator, so Phase B must update Python and Go in the same
  bounded patch.

CLI behavior is also pre-Phase-B:

- `src/striatum/cli/parser.py` has `workflow generate --shape`,
  `--lane-set`, `--artifact-root`, repeated `--option`, and lane flags, but no
  first-class `--role-pack` or `--adversary-pack` options.
- `src/striatum/cli/dispatch.py` parses `--option` values into JSON but then
  the generator rejects unknown option keys.

Focused probes confirmed this current behavior:

- `workflow templates show implementation_panel --json` returns the
  metadata-only catalog entry.
- `workflow templates list --kind role_pack --json` returns
  `implementation_panel_roles`, `incident_response`, and
  `release_readiness`.
- `workflow templates list --kind adversary_pack --json` returns
  `maintainer_cost`, `operator_ergonomics`, and `security_privacy`.
- `workflow generate ... --shape implementation_panel --dry-run --json`
  exits 8 with `field_path: "spec.shape"`.
- `workflow generate ... --option role_pack='"implementation_panel_roles"'`
  exits 8 with `field_path: "spec.options.role_pack"`.
- `workflow generate ... --option adversary_pack='"maintainer_cost"'`
  exits 8 with `field_path: "spec.options.adversary_pack"`.
- `workflow generate ... --option score_dimensions='[...]'` exits 8 with
  `field_path: "spec.options.score_dimensions"`.
- `workflow generate ... --option proposal_count=3` exits 8 with
  `field_path: "spec.options.proposal_count"`.

## Service And UI Surface

The local service already has read endpoints that can surface packs:

- `GET /workflow-templates?kind=role_pack`
- `GET /workflow-templates?kind=adversary_pack`
- `GET /workflow-templates/<template_id>`

`tests/test_workflow_generation_web.py` covers role-pack response listing.
`tests/test_service.py` covers `/workflow-templates` plus generate preview and
write endpoint gating.

The current source does not contain an active chooser UI to extend. Route
dispatch has `/workflows` and `/workflows/<path>` for existing workflow files,
plus `/workflows/generate/preview` and `/workflows/generate` POST endpoints,
but no `/workflows/new` GET route. Source search found no
`WorkflowChooser` island and no `workflow_new.html` template. The current
pack-display surface is therefore API/service discovery, not a full browser
wizard.

## Test Surface

Current focused tests cover the Phase A and generator/API boundaries:

- `tests/test_workflow_generator.py` covers catalog list/show, role/adversary
  pack filters, Markdown rendering, built-in generator shapes, CLI
  generation, and CLI template listing.
- `tests/test_workflow_generation_web.py` covers service helper responses,
  daemon-routed preview, and mutation-gated local write behavior.
- `tests/test_service.py` covers HTTP endpoint behavior for workflow template
  listing and generation gates.
- `go/pkg/workflowtemplates/catalog_test.go` covers Go catalog role-pack
  loading/filtering and embedded catalog parity.
- `go/pkg/workflowgenerate/generate_test.go` covers Go generator preview,
  write safety, and generated workflow shape behavior.

## Implementation Surface For Phase B

A bounded Phase B generator patch would need to:

- add `implementation_panel` to Python and Go generator shape allowlists;
- add a small closed option surface for `role_pack`, `adversary_pack`,
  `proposal_count`, and `score_dimensions`;
- compile only to existing workflow v1/v1.1 constructs and existing artifact
  kinds;
- update CLI flags or document the JSON `--option` shape;
- keep generated metadata honest about selected packs;
- extend Python, Go, service, and example-focused tests;
- avoid RFC 0052 typed debate artifacts, committee phases, panel methods, or
  runtime validation semantics.

The browser chooser and cost/artifact-volume warnings are a separate UI/UX
surface because the current source has no active chooser route or island to
modify.
