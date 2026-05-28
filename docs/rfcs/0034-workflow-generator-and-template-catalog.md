# RFC 0034: Workflow Generator And Template Catalog

Status: accepted (V1)
Date: 2026-05-11
Context:
[`docs/WORKFLOW_TYPES.md`](../reference/workflow-types.md),
[`docs/WRITING_WORKFLOWS.md`](../how-to/writing-workflows.md),
[`docs/SPEC.md`](../reference/spec.md) § "Workflow Config",
[`RFC 0024`](0024-workflow-browser-and-builder.md),
[`RFC 0010`](0010-tool-harness-profiles.md),
[`RFC 0018`](0018-focused-adversarial-review-postures.md),
[`GitHub Actions workflow templates`](https://docs.github.com/en/actions/get-started/quickstart#using-workflow-templates),
[`actions/starter-workflows`](https://github.com/actions/starter-workflows),
[`Backstage Software Templates`](https://backstage.io/docs/features/software-templates/),
[`Argo WorkflowTemplates`](https://argo-workflows.readthedocs.io/en/latest/workflow-templates/),
[`n8n workflow templates`](https://docs.n8n.io/workflows/templates/)

## Problem

`docs/WORKFLOW_TYPES.md` names the workflow shapes and lane-set choices
operators should consider, but current product behavior still leaves too
much JSON authorship in the operator's hands. `striatum workflow init`
ships three starter styles (`minimal`, `review`, `code-change`) and the
web UI can browse/edit/run workflow files, but the operator still has to
translate an intent like "small code change, separate reviewer, local-only
constraint" into a correct `workflow.json` tree.

That is the wrong abstraction level for first-contact usage. Operators
should choose intent, lane topology, artifact root, and policy options.
Striatum should generate the workflow contract, validate it, and then let
the operator inspect or edit it.

The same gap blocks the future UI chooser. Without a first-class generator
API, the CLI, web UI, and eventual chat-assisted scaffolding would each
invent their own JSON assembly path. That would duplicate validator edge
cases and make "custom" workflows mean "freehand raw JSON" instead of
"compose from known safe building blocks."

## Goals

- Add a first-class workflow-generation function in Striatum.
- Promote documented workflow types and lane-set options into a small local
  template catalog.
- Generate complete workflow trees: `workflow.json`, role stubs, prompt
  stubs, and optional source/runbook files.
- Keep every generated workflow JSON-only and immediately valid under
  `workflow validate`.
- Provide a `custom` path that composes known graph blocks instead of
  requiring raw JSON authoring.
- Make CLI, web UI, local service API, and chat tooling use the same
  generator.
- Give human operators a visual chooser and AI/operator-surrogate clients a
  structured API endpoint with dry-run preview and explicit write semantics.
- Preserve local-first behavior: no hosted marketplace, no telemetry, no
  external template fetch.
- Keep `workflow init --style` as compatibility sugar over the generator.

## Non-Goals

- Replacing hand-authored `workflow.json`. Advanced operators may still edit
  workflow files directly.
- Running the generated workflow automatically. Generation writes or previews
  files; `run prepare` / UI run-now remains the lifecycle boundary.
- Template marketplace or remote import. A local target repo may eventually
  carry custom templates, but this RFC does not define hosted sharing.
- AI-authored workflows that bypass validation. Chat-assisted scaffolding
  and operator-surrogate clients may call the generator, but only through
  the same local API, mutation gate, dry-run preview, and validation path.
- Remote API access. The endpoint is served by Striatum's existing local
  service surface; non-loopback hosted control remains out of scope.
- Drag-and-drop graph editing. RFC 0024's visual builder remains the edit
  surface after generation.

## External Prior Art

This design borrows patterns from mature template systems while trimming
anything that violates Striatum's local-first boundary.

- **GitHub Actions** presents preconfigured workflow templates in the UI,
  suggests templates based on repository contents, and treats them as
  starting points the user may customize. The useful idea is "choose a
  relevant starter from context"; the non-goal for Striatum V1 is automatic
  repository analysis beyond explicit operator input.
- **Backstage Software Templates** separates template choice, parameter
  input, review, execution, task tracking, and re-run with prefilled
  parameters. The useful ideas are a review-before-run page and durable
  task-like generation records; Striatum should start smaller with dry-run
  JSON and no separate task runner.
- **Argo WorkflowTemplates** lets users submit a workflow from a reusable
  template and pass parameters; UI enum parameters can become dropdowns.
  The useful idea is that the reusable thing is parameterized and
  submit-able by reference; Striatum's equivalent is generating a concrete
  `workflow.json` snapshot from a catalog entry.
- **n8n** offers empty-start vs template-start, searchable/browsable
  template metadata, categories, collections, and a split between metadata
  preview and workflow import data. The useful idea is separating template
  metadata from workflow body; Striatum should implement this locally, not
  through an external API.

## Proposal

V1 implementation note: the generator core, built-in package-data
catalog, CLI `workflow templates` / `workflow generate` surface, local
service catalog and generation endpoints, custom-plan compiler, and
`workflow init --style` compatibility rewire are accepted for this
slice. The web chooser UI later landed in RFC 0038, and the chat-assisted
scaffolding tool landed in RFC 0036 V1. Future target-repo catalog
extensions remain a separate decision.

### 1. Generator concepts

Introduce four new concepts:

- **workflow shape**: the graph family, e.g. `minimal`, `review`,
  `code_change`, `human_checkpoint`, `evidence_backed`, or
  `multi_review_synthesis`.
- **lane set**: the lane topology, e.g. `local`, `single_agent`,
  `author_reviewer`, `multi_review`, or `custom`.
- **lane modifiers**: optional lane policies layered onto a lane set, such
  as `supervised`, `worktree_isolated`, `constrained`, or
  `harness_profiled`.
- **generation spec**: a structured input object that combines shape,
  lane set, artifact root, scaffold root, branch name, lane commands, and
  policy options.

Shape and lane set are intentionally separate. A code-change workflow can
run on one lane or on separate author/reviewer lanes. A multi-review
workflow can use normal process lanes or supervised lanes. Keeping the axes
separate avoids a combinatorial template explosion.

### 2. Public Python API

Add a library function under a new module, tentatively
`striatum.workflow_generator`:

```python
def generate_workflow(spec: WorkflowGenerationSpec) -> GeneratedWorkflow:
    ...
```

`WorkflowGenerationSpec` is a JSON-serializable value object:

```json
{
  "schema_version": "striatum.workflow_generator.v1",
  "shape": "code_change",
  "lane_set": "author_reviewer",
  "lane_modifiers": ["constrained"],
  "workflow_id": "my-change",
  "name": "My change",
  "workflow_version": "2026-05-11",
  "branch": {
    "mode": "confirm",
    "suggested_name": "striatum/my-change",
    "allow_dirty": false
  },
  "scaffold_root": "striatum/workflows/my-change",
  "artifact_root": "striatum/my-change",
  "lanes": {
    "author": {
      "display_model": "Codex GPT-5.5",
      "command": ["codex", "exec"]
    },
    "reviewer": {
      "display_model": "Claude Opus",
      "command": ["claude", "--print"]
    }
  },
  "options": {
    "review_postures": ["devils_advocate"],
    "max_revision_cycles": 1,
    "include_support_ledger": false
  }
}
```

`GeneratedWorkflow` returns the material the CLI/UI can preview or write:

```json
{
  "workflow": {"...": "workflow.json object"},
  "files": [
    {"path": "striatum/workflows/my-change/workflow.json", "content": "..."},
    {"path": "striatum/workflows/my-change/roles/author.md", "content": "..."},
    {"path": "striatum/workflows/my-change/prompts/draft.md", "content": "..."}
  ],
  "metadata": {
    "shape": "code_change",
    "lane_set": "author_reviewer",
    "graph": {"nodes": [], "edges": []}
  },
  "warnings": []
}
```

The generator must call the existing workflow validator before returning a
success result. This keeps generation bugs from becoming invalid starter
files.

### 3. Built-in shapes

V1 generator ships these shapes:

| Shape | Graph |
|---|---|
| `minimal` | One bounded job produces one artifact. |
| `review` | Draft -> fresh review -> synthesis/apply. |
| `code_change` | Draft -> review -> apply, with bounded `needs_revision`. |
| `human_checkpoint` | Analysis/review -> human checkpoint -> continue/cancel path. |
| `evidence_backed` | Produce artifact -> support ledger -> evidence audit -> final review. |
| `multi_review_synthesis` | Several independent reviews -> ledger/synthesis -> final review. |
| `custom` | Operator-supplied graph plan compiled from known block types. |

Existing `workflow init --style minimal|review|code-change` becomes a thin
wrapper over `generate_workflow` using `lane_set: "local"` and the current
single-lane placeholder behavior.

### 4. Built-in lane sets and modifiers

Base lane sets:

| Lane set | Meaning |
|---|---|
| `local` | One `local` process lane, useful for fixtures and manual operation. |
| `single_agent` | One real agent lane with write/review/synthesis capabilities. |
| `author_reviewer` | Separate author and reviewer lanes; review jobs are fresh by default. |
| `multi_review` | One author/synthesis lane plus multiple reviewer lanes. |
| `custom` | Operator-supplied lane plan with explicit job bindings. |

Lane modifiers:

| Modifier | Effect |
|---|---|
| `supervised` | Emits lanes compatible with `striatum supervise`; requires commands or wrappers that read newline-delimited packets. |
| `worktree_isolated` | Adds `worktree_isolation: "per_job"` to repo-write lanes. |
| `constrained` | Adds adapter `constraints` and optional `required_enforcement`. |
| `harness_profiled` | Adds `harness_profiles` and lane `harness_profile_id` references. |

The generator should reject incompatible combinations with field-specific
errors. Example: `worktree_isolated` on a review-only-only shape is a warning
or no-op; `supervised` without a command is an error.

### 5. Custom shape without raw JSON freestyle

`shape: "custom"` accepts a constrained graph-plan document:

```json
{
  "schema_version": "striatum.workflow_plan.v1",
  "blocks": [
    {"id": "draft", "kind": "draft"},
    {"id": "review", "kind": "review", "posture": "security"},
    {"id": "checkpoint", "kind": "human_checkpoint"},
    {"id": "implement", "kind": "implementation"}
  ],
  "edges": [
    {"from": "draft", "to": "review", "on": "completed"},
    {"from": "review", "to": "checkpoint", "on": "needs_revision"},
    {"from": "checkpoint", "to": "implement", "on": "continue"}
  ],
  "cycles": [
    {"from": "review", "to": "draft", "on_verdict": "needs_revision", "max_iterations": 1}
  ],
  "job_lane_bindings": {
    "draft": "author",
    "review": "security_reviewer",
    "implement": "author"
  }
}
```

Allowed block kinds are a closed set in V1:

```text
draft | review | synthesis | implementation | test |
human_checkpoint | support_ledger | evidence_audit | final_review
```

The custom compiler owns the same safety rules as the normal generator:
repo-relative paths only, bounded cycles only, review policies on review
jobs only, expected artifacts inside write scope, no `.striatum/` writes,
and immediate validation of the compiled `workflow.json`.

This gives operators an escape hatch without making "custom" synonymous
with unstructured JSON.

### 6. Template catalog metadata

Add a small package-data catalog, for example
`src/striatum/workflow_templates/catalog.json`, with one entry per built-in
shape and lane set.

Shape metadata:

```json
{
  "template_id": "code_change",
  "kind": "shape",
  "display_name": "Code change with bounded revision",
  "summary": "Draft, review, revise once if needed, then apply.",
  "recommended_for": ["small implementation", "docs/code edits"],
  "default_lane_sets": ["author_reviewer", "single_agent"],
  "required_options": ["workflow_id", "artifact_root"],
  "graph_preview": {"nodes": [], "edges": []}
}
```

Lane-set metadata:

```json
{
  "template_id": "author_reviewer",
  "kind": "lane_set",
  "display_name": "Separate author and reviewer",
  "summary": "Authoring jobs and review jobs bind to separate lanes.",
  "recommended_for": ["independent review", "code changes"],
  "required_options": ["author.command", "reviewer.command"]
}
```

The catalog is local package data. A future RFC may allow target-repo local
catalog extensions, but V1 ships only built-ins.

### 7. CLI surface

Add:

```bash
striatum workflow templates list [--kind shape|lane_set] [--json]
striatum workflow templates show <template_id> [--json]
striatum workflow generate <path> \
  --shape <shape> \
  --lane-set <lane_set> \
  --artifact-root <repo-relative-path> \
  [--lane-modifier <modifier>]... \
  [--plan <path-for-custom>] \
  [--dry-run] \
  [--json]
```

Rules:

- Refuse to overwrite existing paths unless a future `--force` is added.
- `--dry-run` returns the `GeneratedWorkflow` envelope without writing.
- Non-dry-run writes every file atomically enough for local use, then
  revalidates the written `workflow.json`.
- `workflow init --style ...` stays, but dispatches through
  `workflow generate` internally.
- `workflow validate` remains authoritative; generator success is never a
  substitute for validation on run prepare.

### 8. Local API surface for AI operators

The generator must be callable through Striatum's local service API so an
AI operator, operator surrogate, plugin, or chat tool can request generation
without screen-driving the human UI.

Add read endpoints:

```text
GET /workflow-templates
GET /workflow-templates/<template_id>
```

Add mutation-gated generation endpoints:

```text
POST /workflows/generate/preview
POST /workflows/generate
```

`POST /workflows/generate/preview` accepts `WorkflowGenerationSpec` and
returns the full `GeneratedWorkflow` envelope without writing. It is safe
for AI operators to call freely because it does not mutate the target repo.

`POST /workflows/generate` accepts the same spec plus a required
`confirm_write: true` flag and writes files through the same path-safety,
overwrite-refusal, atomic-write, and validation logic as the CLI. It is
behind `--allow-mutations` and returns structured field errors suitable for
tool callers.

The API response shape is deliberately the same as the Python API envelope:

```json
{
  "ok": true,
  "data": {
    "workflow": {},
    "files": [],
    "metadata": {},
    "warnings": [],
    "validation": {"ok": true}
  }
}
```

Errors use the existing service error envelope and include `field_path`
where possible. AI operators should be able to repair a spec from the error
without scraping prose.

This API is not a second generator. CLI, web UI, chat tools, and plugins
all call the same library function and receive the same envelope.

### 9. Web UI surface

Extend RFC 0024 with `/workflows/new`:

1. Choose workflow shape.
2. Choose lane set and modifiers.
3. Fill required fields: workflow id, name, scaffold path, artifact root,
   branch suggestion, lane commands/display models.
4. Preview generated graph and generated file list.
5. Review generated JSON before save.
6. Save to disk and open the existing visual builder.

The review-before-save step is intentionally explicit. Generated workflows
can cause repo writes later; the operator should inspect the contract before
it becomes a runnable file.

The UI must call the same local API preview/write endpoints above. Human
operator UX and AI operator UX should differ only in presentation, not in
the underlying generation or validation semantics.

### 10. Chat-assisted scaffolding

RFC 0036 V1 implements chat-assisted scaffolding through the closed tools
`generate_workflow_preview` and `generate_workflow_write`. The default
chat/tool behavior calls preview, shows the generated file list, and
requires explicit operator confirmation before executing the write. A chat
model may suggest shape/lane options or fill fields, but the generator and
validator remain the authority.

## Acceptance Criteria

- `generate_workflow` exists as a library API and is covered by unit tests.
- Every built-in shape with every compatible base lane set validates
  successfully.
- Incompatible shape/lane/modifier combinations return field-specific errors.
- `shape: "custom"` compiles only from the closed block vocabulary and
  refuses unbounded cycles, unknown block kinds, invalid lane bindings, and
  unsafe paths.
- `workflow templates list/show --json` exposes local catalog metadata.
- Local service read endpoints expose the same catalog metadata.
- `workflow generate --dry-run --json` returns workflow/file/metadata/warning
  envelopes and writes nothing.
- `workflow generate <path>` writes `workflow.json`, roles, and prompts, then
  validates the written workflow.
- `POST /workflows/generate/preview` returns the same dry-run envelope and
  writes nothing.
- `POST /workflows/generate` is mutation-gated, requires `confirm_write:
  true`, writes the generated tree, and returns structured field errors on
  invalid specs.
- `workflow init --style minimal|review|code-change` remains backwards
  compatible and delegates to the generator.
- Web UI `/workflows/new` can create at least `minimal`, `review`, and
  `code_change` workflows by calling the local API and then open them in the
  existing editor.
- Docs update `WORKFLOW_TYPES.md`, `WRITING_WORKFLOWS.md`,
  `CLI_REFERENCE.md`, `SPEC.md`, and `UBIQUITOUS_LANGUAGE.md`.

## Implementation Plan

### Step 1. Generator Core

Add the value objects, catalog package data, shape compilers, lane-set
compilers, and validation-backed tests. No CLI or web UI yet.

### Step 2. CLI

Add `workflow templates list/show` and `workflow generate`. Rewire
`workflow init --style` to call the generator while preserving output shape
where practical.

### Step 3. Local API Endpoints

Add catalog read endpoints and generation preview/write endpoints. Keep the
preview endpoint non-mutating; gate the write endpoint behind
`--allow-mutations` and require `confirm_write: true`.

### Step 4. Custom Plan Compiler

Add `shape: custom` with the closed block vocabulary, plan-file input, and
graph safety checks.

### Step 5. Web Chooser

Add `/workflows/new` and wire it to dry-run previews, generated graph
preview, save, and handoff to the visual builder via the local API.

### Step 6. Chat Tool

Add a mutation-gated chat tool that can produce a dry-run generation preview
and, with explicit confirmation, write the generated workflow through the
local API endpoint.

## Open Questions

- Should `workflow generate` write a durable source plan file
  (`workflow.generator.json`) alongside `workflow.json` so users can
  regenerate later, or is generated `workflow.json` the only durable source?
- Should lane commands be required for `single_agent` / `author_reviewer`
  generation, or should the generator allow placeholder commands with loud
  warnings?
- Should the generator infer available lane profiles from installed skill or
  plugin bundles, or should all lane commands be explicit operator input?
- Should target repositories be allowed to define local template catalog
  entries in V1.5, and if so where do they live?
- Should the web chooser support repository inspection for suggested shapes
  later, following the GitHub Actions pattern, or should Striatum keep
  selection entirely explicit?

## Domain Modeling

This RFC adds value objects and services, not a new run-state aggregate.

- **workflow generation spec**: value object.
- **workflow shape**: value object / catalog entry.
- **lane set**: value object / catalog entry.
- **workflow generator**: domain service that compiles a generation spec to a
  normal workflow config plus scaffold files.
- **generated workflow**: transient result envelope; the durable contract is
  still `workflow.json`.

The existing `workflow config` remains the executable contract. A generated
workflow is not special at run time; after generation, validation, run
prepare, snapshots, leases, artifacts, and reviews behave exactly as they do
for hand-authored workflows.
