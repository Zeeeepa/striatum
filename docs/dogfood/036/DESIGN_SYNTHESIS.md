---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/036/design/codex/DESIGN.md", "docs/dogfood/036/design/claude_code/DESIGN.md", "docs/dogfood/036/design/gemini/DESIGN.md"]
---

author: designer-codex-gpt-5.5-001

# RFC 0034 Workflow Generator Implementation Plan

Status: design synthesis
Date: 2026-05-12

## Accepted Implementation Scope

The dogfood-036 V1 slice ships generator core, package-data catalog, CLI surface, local service API endpoints, custom-plan compiler, tests, and the `workflow init --style` rewire. The web chooser UI and chat-assisted scaffolding tool are explicitly deferred.

| RFC 0034 acceptance criterion | Concrete plan | Owner module | Test owner |
|---|---|---|---|
| `generate_workflow` exists as a library API and is covered by unit tests. | Add `src/striatum/workflow_generator/` with `generate_workflow(spec) -> GeneratedWorkflow`, dataclass JSON adapters, structured `GeneratorError`, and pure no-I/O compilation. | `src/striatum/workflow_generator/__init__.py`, `spec.py`, `envelope.py`, `errors.py`, `pipeline.py` | `tests/test_workflow_generator_envelope.py` |
| Every built-in shape with every compatible base lane set validates successfully. | Compile six named shapes plus `custom` into normal `striatum.workflow.v1` JSON and call `validate_workflow` before returning. | `src/striatum/workflow_generator/shapes/*.py`, `lane_sets/*.py`, `validate.py` | `tests/test_workflow_generator_shapes.py` |
| Incompatible shape/lane/modifier combinations return field-specific errors. | Implement a closed compatibility matrix in `modifiers.py`; every refusal raises `GeneratorError(field_path=...)`. | `src/striatum/workflow_generator/modifiers.py` | `tests/test_workflow_generator_modifiers.py`, `tests/test_workflow_generator_errors.py` |
| `shape: "custom"` compiles only from the closed block vocabulary and refuses unsafe plans. | Require `plan` for `custom`, validate block ids, edges, cycles, postures, lane bindings, and paths before normal workflow validation. | `src/striatum/workflow_generator/shapes/custom.py` | `tests/test_workflow_generator_custom.py` |
| `workflow templates list/show --json` exposes local catalog metadata. | Add package-data catalog loader and CLI routing under `workflow templates`. | `src/striatum/workflow_generator/catalog.py`, `src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py`, new `src/striatum/cli/workflow.py` | `tests/test_workflow_templates_catalog.py`, `tests/test_workflow_generator_cli.py` |
| Local service read endpoints expose the same catalog metadata. | Add `GET /workflow-templates` and `GET /workflow-templates/<id>` using the same loader. | `src/striatum/service.py` | `tests/test_service_workflow_generator.py` |
| `workflow generate --dry-run --json` returns envelope and writes nothing. | CLI builds a `WorkflowGenerationSpec`, calls `generate_workflow`, returns `{"ok": true, "data": envelope}`, and performs no filesystem writes. | `src/striatum/cli/workflow.py` | `tests/test_workflow_generator_cli.py` |
| `workflow generate <path>` writes `workflow.json`, roles, and prompts, then validates the written workflow. | Refuse existing targets, write generated files through temporary sibling files, re-read `workflow.json`, and run `load_workflow`. | `src/striatum/workflow_generator/write.py`, `src/striatum/cli/workflow.py` | `tests/test_workflow_generator_cli.py` |
| `POST /workflows/generate/preview` returns the same dry-run envelope and writes nothing. | Parse JSON spec, call pure generator, return the identical envelope wrapper; no mutation gate required. | `src/striatum/service.py` | `tests/test_service_workflow_generator.py` |
| `POST /workflows/generate` is mutation-gated, requires `confirm_write: true`, writes the tree, and returns structured field errors. | Reuse `allow_mutations` guard, require `confirm_write`, refuse overwrites and unsafe paths, then write via the same helper as CLI. | `src/striatum/service.py`, `src/striatum/workflow_generator/write.py` | `tests/test_service_workflow_generator.py` |
| `workflow init --style minimal|review|code-change` remains backwards compatible and delegates to the generator. | Rework `workflow_init()` as a shim that builds a local-lane spec and preserves returned keys: `status`, `path`, `workflow_path`, `style`, `files`. Snapshot old JSON before rewire and require byte-equivalent generated workflow JSON unless a deliberate decision changes it. | `src/striatum/cli/workflow_init.py` | `tests/test_workflow_init_backcompat.py`, existing CLI tests |
| Docs update `WORKFLOW_TYPES.md`, `WRITING_WORKFLOWS.md`, `CLI_REFERENCE.md`, `SPEC.md`, and `UBIQUITOUS_LANGUAGE.md`. | Document generator concepts, command flow, API surfaces, no schema migration, deferred UI/chat coverage, and catalog vocabulary. | `docs/*`, `CHANGELOG.md`, `docs/rfcs/0034-workflow-generator-and-template-catalog.md` | `tests/test_doc_links.py` |

## Deferred Scope

| Deferred item | Why deferred | Lands in |
|---|---|---|
| Web `/workflows/new` chooser UI | The V1 slice needs the generator contract first; UI would otherwise invent decisions before the Python/CLI/API envelope is stable. | Follow-up dogfood implementing RFC 0034 §9 over the V1 local API. |
| Chat-assisted scaffolding tool | Chat mutation needs confirmation UX and should wrap the settled preview/write endpoints rather than co-design them. | Follow-up dogfood implementing RFC 0034 §10. |
| Target-repo local catalog extensions | V1 should prove built-in package data and avoid extension loading/path-safety complexity. | Future V1.5 RFC/dogfood after built-ins ship. |
| Automatic repository inspection for suggested shapes | It risks hidden heuristics and drifts from explicit local-first operator choice. | Deferred indefinitely until a separate decision accepts repository-analysis behavior. |

## WorkflowGenerationSpec Schema

`WorkflowGenerationSpec` is a JSON-serializable value object with `schema_version: "striatum.workflow_generator.v1"`. Unknown keys are errors. Paths are repo-relative POSIX-like strings, never absolute, never containing `..` segments, null bytes, `.git/`, or `.striatum/`.

Required fields:

| Field | Type | Notes | Error field_path |
|---|---|---|---|
| `schema_version` | string | Must equal `striatum.workflow_generator.v1`. | `spec.schema_version` |
| `shape` | string | `minimal`, `review`, `code_change`, `human_checkpoint`, `evidence_backed`, `multi_review_synthesis`, or `custom`. | `spec.shape` |
| `lane_set` | string | `local`, `single_agent`, `author_reviewer`, `multi_review`, or `custom`. | `spec.lane_set` |
| `workflow_id` | string | CLI may default from target path tail; API requires either explicit value or service-side normalization that reports the default in the returned spec metadata. | `spec.workflow_id` |
| `name` | string | CLI may default from `workflow_id`. | `spec.name` |
| `workflow_version` | string | CLI may default to UTC date. | `spec.workflow_version` |
| `branch` | object | `mode: "confirm"`, `suggested_name`, `allow_dirty: false` by default. | `spec.branch.*` |
| `scaffold_root` | string | Destination root for `workflow.json`, `roles/`, and `prompts/`; CLI positional `<path>` maps here. | `spec.scaffold_root` |
| `artifact_root` | string | Root used by generated job write scopes and artifact paths. | `spec.artifact_root` |
| `lanes` | object | Lane input keyed by lane id. Required command coverage depends on lane set/modifiers. | `spec.lanes.<lane_id>.*` |
| `options` | object | Shape/modifier options. May be empty. | `spec.options.<key>` |

Optional fields:

| Field | Type | Notes |
|---|---|---|
| `lane_modifiers` | string array | Closed set: `supervised`, `worktree_isolated`, `constrained`, `harness_profiled`; default `[]`. |
| `plan` | object | Required iff `shape == "custom"`; may be supplied from `--plan`. |
| `context_docs` | array | Optional generated workflow `context_docs`; defaults to `[]`. |
| `parallelism` | object | Optional override for generated `parallelism`; default is declared, `max_active_jobs` derived from shape/lane set. |

`options` has closed known keys in V1: `review_postures`, `max_revision_cycles`, `include_support_ledger`, `constraints`, `required_enforcement`, `harness_profiles`, `reviewer_count`, and `custom_job_artifacts`. Unknown `options.*` keys are errors in V1 because generator specs must be repairable by `field_path`.

## GeneratedWorkflow Envelope

The Python API, CLI `--json`, and local API return the same data envelope:

```json
{
  "workflow": {},
  "files": [
    {"path": "workflows/my-change/workflow.json", "content": "{}\n"},
    {"path": "workflows/my-change/roles/author.md", "content": "..."},
    {"path": "workflows/my-change/prompts/draft.md", "content": "..."}
  ],
  "metadata": {
    "shape": "code_change",
    "lane_set": "author_reviewer",
    "lane_modifiers": ["constrained"],
    "graph": {"nodes": [], "edges": [], "cycles": []},
    "catalog_templates": ["code_change", "author_reviewer"]
  },
  "warnings": [],
  "validation": {"ok": true, "workflow_id": "my-change"}
}
```

CLI and service wrap that envelope as `{"ok": true, "data": <GeneratedWorkflow>}`. Errors use:

```json
{
  "ok": false,
  "error": {
    "code": 8,
    "message": "incompatible lane modifier",
    "field_path": "spec.lane_modifiers[0]",
    "hint": "supervised is forbidden for lane_set local",
    "ref": "striatum workflow templates show local"
  }
}
```

## Built-In Shapes

| Shape | Graph description | Catalog summary |
|---|---|---|
| `minimal` | `draft` produces one handoff artifact. | One bounded job for a small report or starter artifact. |
| `review` | `draft -> review -> apply`, where `apply` is a synthesis job. | Draft, fresh review, then final synthesis. |
| `code_change` | `draft -> review -> apply`, with one bounded `needs_revision` cycle from review to draft by default. | Code or docs change with an explicit review gate and one revision path. |
| `human_checkpoint` | `analysis -> checkpoint -> apply`, with downstream work held behind a human checkpoint. | Require owner judgment before continuing. |
| `evidence_backed` | `draft -> support_ledger -> evidence_audit -> final_review`. | Produce claims with a support ledger and audit review. |
| `multi_review_synthesis` | Parallel reviews in one group -> synthesis -> final review. | Collect several independent reviews before a final recommendation. |
| `custom` | Compile user plan blocks, edges, and bounded cycles into ordinary jobs. | Compose a workflow from known safe block kinds without raw workflow JSON. |

## Built-In Lane Sets

| Lane set | Lane topology | Catalog summary |
|---|---|---|
| `local` | One `local` process lane using `["sh", "-c", "cat >/dev/null"]`; all jobs bind to it. | Fixture/manual starter lane that validates without a real model command. |
| `single_agent` | One `agent` lane with write/review/synthesis capabilities. | One real agent session handles the whole workflow. |
| `author_reviewer` | `author` lane for draft/synthesis/build/test; fresh `reviewer` lane for review jobs. | Separate authoring and review lanes for independent review. |
| `multi_review` | One `author` lane plus N reviewer lanes derived from `options.review_postures` or `options.reviewer_count`. | Productive disagreement through multiple reviewer lanes. |
| `custom` | Explicit lane definitions and `job_lane_bindings`. | Advanced lane topology with every binding declared. |

## Lane-Modifier Compatibility Matrix

Decision values: `required` means the lane set cannot be valid without the modifier-owned fields; `allowed` means apply normally; `forbidden` means raise `GeneratorError`; `warning` means no-op or partial application with a warning in the envelope.

| Modifier \\ Lane set | `local` | `single_agent` | `author_reviewer` | `multi_review` | `custom` |
|---|---|---|---|---|---|
| `supervised` | forbidden | required | required | required | allowed |
| `worktree_isolated` | warning | allowed | allowed | allowed | allowed |
| `constrained` | allowed | allowed | allowed | allowed | allowed |
| `harness_profiled` | forbidden | required | required | required | allowed |

Field-specific refusal shape for forbidden cells:

```json
{
  "code": 8,
  "message": "lane modifier is incompatible with lane set",
  "field_path": "spec.lane_modifiers[0]",
  "hint": "modifier 'supervised' is forbidden for lane_set 'local'"
}
```

Additional rules:

- `supervised` requires `lanes.<id>.command` for each non-local process lane and warns when a custom lane command is present but `options.supervision_compatible` is not explicitly true. This is advisory because the runner cannot prove an arbitrary CLI consumes newline-delimited packets.
- `worktree_isolated` sets `worktree_isolation: "per_job"` only on lanes that own repo-write jobs. If a shape has no repo-write jobs, return `warning` and leave lanes unchanged.
- `constrained` requires `options.constraints` values from the existing workflow constraint vocabulary and `options.required_enforcement` from existing adapter enforcement levels.
- `harness_profiled` requires `options.harness_profiles` with valid `tool_family` values from `workflow.HARNESS_PROFILE_TOOL_FAMILIES`; generated lane `harness_profile_id` values must reference generated top-level profiles.

## Catalog Metadata Layout

Use package data, not target-repo discovery:

```text
src/striatum/workflow_templates/
  __init__.py
  catalog.json
```

`catalog.json` contains a top-level object:

```json
{
  "schema_version": "striatum.workflow_templates.v1",
  "shapes": [],
  "lane_sets": []
}
```

Shape entries:

```json
{
  "template_id": "code_change",
  "kind": "shape",
  "display_name": "Code change with bounded revision",
  "summary": "Draft, review, revise at most once if needed, then apply.",
  "recommended_for": ["small code edit that needs an explicit review gate"],
  "default_lane_sets": ["author_reviewer", "single_agent"],
  "required_options": ["workflow_id", "artifact_root"],
  "graph_preview": {"nodes": [{"id": "draft", "label": "Draft"}], "edges": [], "cycles": []}
}
```

Lane-set entries:

```json
{
  "template_id": "author_reviewer",
  "kind": "lane_set",
  "display_name": "Separate author and reviewer",
  "summary": "Authoring jobs and review jobs bind to separate lanes.",
  "recommended_for": ["independent review for a code or docs change"],
  "required_options": ["lanes.author.command", "lanes.reviewer.command"]
}
```

Loader semantics: `catalog.py` uses `importlib.resources.files("striatum.workflow_templates")`, validates catalog shape on first load, caches the parsed object in-process, returns entries sorted by `kind` then `template_id`, and never fetches remote templates. Malformed package data raises a non-field `GeneratorError` with code 8 because it is a product bug, not user input.

## Custom-Plan Compiler

`shape: "custom"` accepts `spec.plan` with `schema_version: "striatum.workflow_plan.v1"`.

Closed block vocabulary:

```text
draft | review | synthesis | implementation | test |
human_checkpoint | support_ledger | evidence_audit | final_review
```

Refusal cases:

| Refusal | field_path |
|---|---|
| Missing custom plan | `spec.plan` |
| Unknown plan schema | `spec.plan.schema_version` |
| Duplicate block id | `spec.plan.blocks[i].id` |
| Unknown block kind | `spec.plan.blocks[i].kind` |
| Review-only fields on non-review block | `spec.plan.blocks[i].review_posture` |
| Invalid review posture | `spec.plan.blocks[i].posture` |
| Edge references missing block | `spec.plan.edges[i].from` or `.to` |
| Base `edges` graph contains a cycle | `spec.plan.edges` |
| Cycle lacks positive `max_iterations` | `spec.plan.cycles[i].max_iterations` |
| Cycle references missing block | `spec.plan.cycles[i].from` or `.to` |
| Cycle source is not a review block | `spec.plan.cycles[i].from` |
| Lane binding references missing block | `spec.plan.job_lane_bindings.<block_id>` |
| Lane binding references missing lane | `spec.plan.job_lane_bindings.<block_id>` |
| Derived artifact path escapes artifact root | `spec.plan.blocks[i].artifact_path` |

Safety rules: base graph must be acyclic; loops live only in `cycles`; every cycle is bounded; every block has a lane binding under `lane_set: custom`; every generated artifact path is inside the block write scope; every compiled workflow is immediately passed to `validate_workflow`.

## Public Python API

Public imports:

```python
from striatum.workflow_generator import (
    GeneratedWorkflow,
    GeneratorError,
    WorkflowGenerationSpec,
    generate_workflow,
)
```

`generate_workflow(spec)` is pure: no filesystem writes, no network, no SQLite mutation. It normalizes `spec`, compiles shape and lane-set plans, applies modifiers in this order: `supervised`, `worktree_isolated`, `constrained`, `harness_profiled`, renders scaffold files, calls `validate_workflow(workflow)`, and returns `GeneratedWorkflow`. Validation errors are wrapped as `GeneratorError(field_path="workflow")` when no narrower generation field can be mapped.

## CLI Surface

Add exact verbs:

```text
striatum workflow templates list [--kind shape|lane_set] [--json]
striatum workflow templates show <template_id> [--json]
striatum workflow generate <path>
  --shape <shape>
  --lane-set <lane_set>
  --artifact-root <repo-relative-path>
  [--lane-modifier <modifier>]...
  [--plan <path>]
  [--workflow-id <id>]
  [--name <name>]
  [--workflow-version <version>]
  [--branch-suggestion <branch>]
  [--lane-command <lane_id>=<json-array>]...
  [--lane-display-model <lane_id>=<display-name>]...
  [--option <dotted.key>=<json-value>]...
  [--dry-run]
  [--json]
```

`--shape`, `--lane-set`, and `--artifact-root` are required for `workflow generate`. `--plan` is required iff `--shape custom`. `<path>` maps to `spec.scaffold_root`. `--dry-run` writes nothing and returns the full envelope. Non-dry-run refuses to overwrite any existing generated file; V1 does not ship `--force`. Human help text must point first-time users to `workflow templates list`, include exactly one copy-paste dry-run example, and say that `workflow validate` remains authoritative.

## Local API Surface

Add endpoints to the existing local service:

```text
GET /workflow-templates
GET /workflow-templates/<template_id>
POST /workflows/generate/preview
POST /workflows/generate
```

`GET /workflow-templates` accepts optional `?kind=shape|lane_set`. `GET /workflow-templates/<id>` returns one catalog entry or 404. `POST /workflows/generate/preview` accepts `{"spec": <WorkflowGenerationSpec>}` and writes nothing. `POST /workflows/generate` accepts `{"spec": <WorkflowGenerationSpec>, "confirm_write": true}`, requires `--allow-mutations`, refuses overwrites, and writes through the same helper as the CLI. All generation errors return structured `field_path`; missing mutation permission uses `field_path: "server.allow_mutations"` and missing confirmation uses `field_path: "confirm_write"`.

## `workflow init --style` Backwards-Compatibility

`workflow init --style minimal|review|code-change` keeps its command shape, default `review` style, overwrite refusal, placeholder `local` lane, generated file list, and returned JSON keys. Internally, `workflow_init()` builds a `WorkflowGenerationSpec` with `lane_set: "local"`, `shape: "code_change"` for legacy `code-change`, and compatibility artifact paths matching the existing `_starter_workflow()` output under `docs/workflows/<slug>/`. Existing tests should pass unchanged; new snapshot coverage pins the old starter JSON.

## Schema Migration

No database migration is expected for V1. The catalog is package data, the generator is a pure compiler, and generated workflows remain ordinary `striatum.workflow.v1` JSON. Do not add SQLite tables, migrations, or state rows for generation.

## Test Strategy

Add focused tests:

- `tests/test_workflow_generator_envelope.py`: spec/envelope JSON round trips, unknown spec keys, required-field errors.
- `tests/test_workflow_generator_shapes.py`: shape x compatible lane-set matrix validates with `validate_workflow` and graph data renders.
- `tests/test_workflow_generator_modifiers.py`: full compatibility matrix, modifier effects, warning/no-op behavior, field-specific errors.
- `tests/test_workflow_generator_custom.py`: all custom-plan refusal cases plus one valid custom plan.
- `tests/test_workflow_templates_catalog.py`: package-data load, list/show, sorted order, malformed catalog guard, metadata quality.
- `tests/test_workflow_generator_cli.py`: templates commands, dry-run writes nothing, non-dry-run refuses overwrite, generated file tree validates.
- `tests/test_workflow_init_backcompat.py`: legacy `workflow init` JSON and return envelope compatibility.
- `tests/test_service_workflow_generator.py`: catalog GETs, preview without mutation gate, generate with `confirm_write` and `allow_mutations`, refusal envelopes.

Run `make lint`, `make typecheck`, and `make test` before handoff.

## Documentation Deltas

- `docs/SPEC.md`: add generator API contract, catalog package-data boundary, local endpoint contract, and explicit "no migration" note.
- `docs/WORKFLOW_TYPES.md`: replace chooser-roadmap-only wording with the generator flow and template catalog vocabulary.
- `docs/WRITING_WORKFLOWS.md`: add "generate first, edit later" guidance and custom-plan format.
- `docs/CLI_REFERENCE.md`: add `workflow templates list/show` and `workflow generate` forms.
- `docs/UBIQUITOUS_LANGUAGE.md`: add `workflow generation spec`, `generated workflow`, `workflow template catalog`, `workflow shape`, `lane set`, and `custom plan`.
- `docs/HOW_TO_HUMAN.md`: add first-time generator storyboard using `--dry-run`.
- `docs/rfcs/0034-workflow-generator-and-template-catalog.md`: update status after implementation acceptance and note V1 scope adjustment deferring UI/chat.
- `CHANGELOG.md`: add unreleased entry.

## Staging Plan

1. Generator package, catalog package data, and unit tests.
2. CLI `workflow templates` and `workflow generate`, with dry-run and overwrite tests.
3. `workflow init --style` rewire and backcompat tests.
4. Local API catalog/preview/write endpoints and service tests.
5. Custom-plan compiler and refusal matrix.
6. Docs and changelog.

This dogfood lands items 1-6. A future dogfood lands `/workflows/new`, chat-assisted scaffolding, and any later local catalog extension decision.

## Human-Decision Questions

- Should non-dry-run `workflow generate` ever gain `--force`, or should overwrite remain out of scope until operators hit repeated friction?
- Should CLI generation require real lane commands for `single_agent`, `author_reviewer`, and `multi_review`, or allow placeholder commands with warnings? This synthesis chooses required commands for real lane sets, placeholder only for `local`.
- Should a durable `workflow.generator.json` source spec be written beside generated `workflow.json`? This synthesis chooses no for V1; generated `workflow.json` is the durable contract.
- Should `workflow generate` default `workflow_version` to the current UTC date or require it explicitly in API calls? This synthesis chooses CLI default, API explicit-or-normalized with returned defaults.
