# Codex Design Prompt

Produce `docs/dogfood/036/design/codex/DESIGN.md`.

Design an implementation plan for RFC 0034: workflow generator + template catalog. Sit on top of the existing `workflow init --style`, `workflow validate`, scaffold templates under `src/striatum/scaffold/templates`, and the RFC 0024 visual builder. Do not redesign those.

Your plan must cover:

**Generator core:**

- `WorkflowGenerationSpec` value object (JSON-serializable) with the fields shown in RFC 0034 §2 (`shape`, `lane_set`, `lane_modifiers`, `workflow_id`, `name`, `workflow_version`, `branch`, `scaffold_root`, `artifact_root`, `lanes`, `options`)
- `GeneratedWorkflow` envelope shape (`workflow` object, `files` array, `metadata`, `warnings`)
- public Python API: `generate_workflow(spec) -> GeneratedWorkflow`
- generator MUST call `workflow validate` on the compiled `workflow.json` before returning success; bug in the generator must not become an invalid starter file
- structured error envelope with `field_path` for invalid specs

**Shape and lane-set axes:**

- built-in shapes: `minimal`, `review`, `code_change`, `human_checkpoint`, `evidence_backed`, `multi_review_synthesis`, `custom`
- built-in lane sets: `local`, `single_agent`, `author_reviewer`, `multi_review`, `custom`
- lane modifiers: `supervised`, `worktree_isolated`, `constrained`, `harness_profiled` — design the per-modifier compatibility matrix so incompatible combinations return field-specific errors

**Catalog metadata:**

- package-data layout (`src/striatum/workflow_templates/catalog.json` or directory tree)
- shape entry: `template_id`, `kind="shape"`, `display_name`, `summary`, `recommended_for`, `default_lane_sets`, `required_options`, `graph_preview`
- lane-set entry: `template_id`, `kind="lane_set"`, `display_name`, `summary`, `recommended_for`, `required_options`

**CLI surface:**

- `striatum workflow templates list [--kind shape|lane_set] [--json]`
- `striatum workflow templates show <template_id> [--json]`
- `striatum workflow generate <path> --shape <s> --lane-set <l> --artifact-root <p> [--lane-modifier <m>]... [--plan <p>] [--dry-run] [--json]`
- refuse-to-overwrite posture; `--dry-run` returns the `GeneratedWorkflow` envelope and writes nothing
- `workflow init --style minimal|review|code-change` stays, but dispatches through `generate_workflow` internally

**Local API surface for AI/operator-surrogate clients:**

- read endpoints: `GET /workflow-templates`, `GET /workflow-templates/<template_id>`
- mutation-gated generation: `POST /workflows/generate/preview` (non-mutating, safe to call freely) and `POST /workflows/generate` (requires `confirm_write: true`, behind `--allow-mutations`)
- structured field errors compatible with the existing service error envelope

**Custom-plan compiler (`shape: "custom"`):**

- constrained graph-plan document (`striatum.workflow_plan.v1`) with `blocks`, `edges`, `cycles`, `job_lane_bindings`
- closed block vocabulary: `draft | review | synthesis | implementation | test | human_checkpoint | support_ledger | evidence_audit | final_review`
- safety rules: repo-relative paths only, bounded cycles only, review policies on review jobs only, expected artifacts inside write scope, no `.striatum/` writes, immediate validation of compiled `workflow.json`

**Concrete touch points in `src/striatum/`:**

- new module: `src/striatum/workflow_generator.py` (or `src/striatum/workflow_generator/` package if helpful)
- new package data: `src/striatum/workflow_templates/`
- CLI: `src/striatum/cli/workflow.py` or `src/striatum/cli/dispatch.py` route additions
- local API: existing local service surface module (identify path during design)

**Explicitly deferred to a follow-up dogfood**, document but do NOT design:

- web `/workflows/new` chooser UI (RFC 0034 §9)
- chat-assisted scaffolding tool (RFC 0034 §10)
- target-repo local catalog extensions (RFC 0034 §6 V1.5)
- automatic repository inspection for suggested shapes

Cover the unit-test coverage strategy: per-shape compiler tests, per-lane-set tests, lane-modifier compatibility matrix tests, catalog-loader tests, custom-plan compiler tests (including refusal cases for unknown block kinds, unbounded cycles, invalid lane bindings, unsafe paths), and `workflow init --style` backwards-compat tests.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim into the artifact title block.

- Plain Markdown line, NO bold (`**`), NO italics, NO heading prefix (`#`), NO quotes around the value.
- Lowercase `author:` exactly.
- Correct: `author: designer-codex-gpt-5.5-001`
- Wrong: `**Author:** ...`, `Author: ...`, `# author: ...`, `author: "..."`.

The `handoff` artifact kind does not require YAML front matter. Synthesis and finding artifacts later in this dogfood will, with the JSON-encoded `key: <value>` block.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.
