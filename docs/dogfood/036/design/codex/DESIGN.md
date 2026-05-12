# RFC 0034 Workflow Generator Design

Status: design
Date: 2026-05-12
author: designer-codex-gpt-5.5-001

## Summary

RFC 0034 should land as a generator service layered on the existing workflow
contract, not as a second workflow model. The durable runtime artifact remains a
normal JSON `workflow.json`; `striatum workflow validate` remains authoritative;
`run prepare` still snapshots only that workflow JSON. The new V1 slice adds a
single compiler path used by Python callers, CLI commands, and local service
endpoints.

The implementation should add:

- `src/striatum/workflow_generator.py` or a small
  `src/striatum/workflow_generator/` package containing value objects,
  catalog loading, shape compilers, lane-set compilers, custom-plan compilation,
  write helpers, and structured generation errors.
- `src/striatum/workflow_templates/` package data with a local built-in
  catalog. No hosted marketplace, remote fetch, telemetry, target-repo catalog
  extension, or repository inspection belongs in V1.
- CLI additions in `src/striatum/cli/parser.py`,
  `src/striatum/cli/dispatch.py`, and either a new
  `src/striatum/cli/workflow.py` module or the existing
  `workflow_init.py` split.
- Local HTTP service routes in `src/striatum/service.py`, reusing the same
  path-safety and mutation-gate posture already used by RFC 0024 workflow edit
  and run-now endpoints.

## Public API

Expose:

```python
def generate_workflow(spec: WorkflowGenerationSpec) -> GeneratedWorkflow:
    ...
```

`WorkflowGenerationSpec` should be a dataclass or typed value object that accepts
only JSON-serializable input and keeps the external schema stable:

```json
{
  "schema_version": "striatum.workflow_generator.v1",
  "shape": "code_change",
  "lane_set": "author_reviewer",
  "lane_modifiers": ["constrained"],
  "workflow_id": "my-change",
  "name": "My change",
  "workflow_version": "2026-05-12",
  "branch": {
    "mode": "confirm",
    "suggested_name": "striatum/my-change",
    "allow_dirty": false
  },
  "scaffold_root": "workflows/my-change",
  "artifact_root": "striatum/my-change",
  "lanes": {},
  "options": {}
}
```

Validation should happen in two phases. First, normalize and validate the
generation spec with field-specific `WorkflowGenerationError(field_path=...)`.
Second, compile to normal workflow JSON and call `validate_workflow(workflow,
repo_root=repo_root_if_known)` before returning success. A generator bug must
surface as a generation failure, not as an invalid starter file.

`GeneratedWorkflow` should be a plain envelope:

```json
{
  "workflow": {},
  "files": [
    {"path": "workflows/my-change/workflow.json", "content": "..."}
  ],
  "metadata": {
    "shape": "code_change",
    "lane_set": "author_reviewer",
    "lane_modifiers": ["constrained"],
    "graph": {"nodes": [], "edges": [], "cycles": []}
  },
  "warnings": [],
  "validation": {"ok": true}
}
```

`files[].path` is repo-relative. `workflow.json` content should be generated via
`json.dumps(workflow, indent=2) + "\n"`. Role and prompt files are Markdown
stubs under the requested scaffold root.

## Compiler Shape

Keep shape and lane-set compilation separate:

1. `compile_shape(spec) -> ShapePlan`: returns roles needed, job blocks, logical
   edges, cycles, artifact logical names, and prompt stub names.
2. `compile_lane_set(spec, shape_plan) -> LanePlan`: returns lane definitions,
   harness profiles if requested, role-to-lane or job-to-lane bindings, and
   warnings.
3. `assemble_workflow(spec, shape_plan, lane_plan) -> JsonObject`: produces the
   exact `striatum.workflow.v1` object.
4. `render_scaffold_files(...) -> list[GeneratedFile]`: emits workflow JSON,
   `roles/*.md`, `prompts/*.md`, and optional `RUNBOOK.md` / `SOURCES.md` only
   when explicitly requested later.

This keeps `workflow init --style` straightforward: build a spec with
`lane_set="local"`, the existing placeholder process command, and a scaffold
root equal to the current target path.

## Built-In Shapes

The V1 shape compilers should produce conservative, validating graphs:

| Shape | Jobs |
|---|---|
| `minimal` | `draft` producing one `handoff` artifact. |
| `review` | `draft -> review -> apply`, with review artifact kind `finding` and final artifact kind `synthesis`. |
| `code_change` | Same as `review`, plus a bounded `needs_revision` cycle from `review` to `draft`, default `max_revision_cycles=1`. |
| `human_checkpoint` | `analysis -> checkpoint -> apply`, where the checkpoint job is explicit and downstream work depends on operator resolution. |
| `evidence_backed` | `draft -> support_ledger -> evidence_audit -> final_review`, using the existing `support_ledger` and `finding` artifact kinds. |
| `multi_review_synthesis` | parallel review jobs in one parallel group -> `synthesis` -> `final_review`. |
| `custom` | compiled from `striatum.workflow_plan.v1`; see below. |

For all built-ins, generated `write_scope.allowed_paths` should be rooted at
`artifact_root`, with review-only jobs writing to subdirectories such as
`<artifact_root>/review/` or `<artifact_root>/reviews/<posture>/`. Every
artifact path must sit under its job write scope and outside `.striatum/`.

## Lane Sets And Modifiers

Base lane sets:

| Lane set | Lanes | Default bindings |
|---|---|---|
| `local` | one `local` process lane with `["sh", "-c", "cat >/dev/null"]` | all jobs bind to `local` |
| `single_agent` | one `agent` lane | all jobs bind to `agent` |
| `author_reviewer` | `author` plus `reviewer` | authoring/synthesis/test jobs bind to `author`; review jobs bind to `reviewer`; review jobs are fresh by default |
| `multi_review` | `author` plus N reviewer lanes | review jobs bind one-to-one to reviewer lanes; convergence jobs bind to `author` |
| `custom` | explicit lanes and `job_lane_bindings` | compiler validates every referenced lane/job |

Modifier compatibility:

| Modifier | Applies To | Error / Warning Rules |
|---|---|---|
| `supervised` | all process lanes except `local` by default | error if a real agent lane lacks `command`; warning for `local`; generated lanes should not claim supervision support unless commands can read packet stdin |
| `worktree_isolated` | repo-write lanes in `single_agent`, `author_reviewer`, `multi_review`, `custom` | set `worktree_isolation: "per_job"` only on lanes that own repo-write jobs; warning if the shape has no repo-write jobs; no-op for review-only lanes |
| `constrained` | any lane set | add `constraints` and optional `required_enforcement` from spec options; error on unknown constraint names/values before workflow validation |
| `harness_profiled` | real agent lane sets and `custom` | add top-level `harness_profiles` and lane `harness_profile_id`; error if `tool_family` is missing or outside the existing closed set |

This avoids combinatorial templates: shape compilers know graphs, lane-set
compilers know topology, modifiers patch lane policy in a deterministic order:
`supervised`, `worktree_isolated`, `constrained`, then `harness_profiled`.

## Catalog

Use local package data:

```text
src/striatum/workflow_templates/
  __init__.py
  catalog.json
```

The catalog loader should use `importlib.resources.files(...)`, validate the
catalog at import/use time, and return sorted entries. V1 entries are metadata,
not source workflow bodies. Shape entries contain:

```json
{
  "template_id": "code_change",
  "kind": "shape",
  "display_name": "Code change with bounded revision",
  "summary": "Draft, review, revise once if needed, then apply.",
  "recommended_for": ["small implementation", "docs/code edits"],
  "default_lane_sets": ["author_reviewer", "single_agent"],
  "required_options": ["workflow_id", "artifact_root"],
  "graph_preview": {"nodes": [], "edges": [], "cycles": []}
}
```

Lane-set entries contain:

```json
{
  "template_id": "author_reviewer",
  "kind": "lane_set",
  "display_name": "Separate author and reviewer",
  "summary": "Authoring jobs and review jobs bind to separate lanes.",
  "recommended_for": ["independent review", "code changes"],
  "required_options": ["lanes.author.command", "lanes.reviewer.command"]
}
```

`workflow templates list/show` and service read endpoints should read this
catalog, not reimplement metadata.

## CLI

Add parser subcommands:

```text
striatum workflow templates list [--kind shape|lane_set] [--json]
striatum workflow templates show <template_id> [--json]
striatum workflow generate <path> --shape <s> --lane-set <l>
  --artifact-root <repo-relative-path>
  [--lane-modifier <m>]...
  [--workflow-id <id>]
  [--name <name>]
  [--workflow-version <v>]
  [--branch <branch>]
  [--lane-command <lane_id>=<json-array-or-shell-string>]...
  [--plan <path>]
  [--dry-run]
  [--json]
```

`workflow generate --dry-run --json` returns the full
`GeneratedWorkflow` envelope and writes nothing. Non-dry-run refuses when the
target path already exists; V1 should not add `--force`. Writes should create
the target tree, write files through temporary sibling files, then re-read and
validate `workflow.json`.

`workflow init --style minimal|review|code-change` should delegate internally
to `generate_workflow`. Preserve current behavior where practical: default
style `review`, placeholder `local` lane, refusal to overwrite, and returned
JSON fields `status`, `path`, `workflow_path`, `style`, `files`.

## Local Service API

Add read endpoints:

```text
GET /workflow-templates
GET /workflow-templates/<template_id>
```

Add generation endpoints:

```text
POST /workflows/generate/preview
POST /workflows/generate
```

`preview` is non-mutating and may be available without `--allow-mutations`.
It accepts a generation spec and returns the same envelope as
`generate_workflow`. `generate` requires `--allow-mutations` and
`confirm_write: true`, refuses overwrite, writes the generated files, and
returns the written paths plus the generation envelope. Both POST endpoints
should require `Content-Type: application/json`, enforce the existing 1 MB body
cap pattern, and return errors in the current service shape:

```json
{
  "ok": false,
  "error": {
    "code": 422,
    "message": "invalid lane modifier",
    "errors": [
      {"field_path": "lane_modifiers[0]", "message": "unknown modifier"}
    ]
  }
}
```

Path safety should mirror `_handle_workflow_edit_save`: reject absolute paths,
`..`, null bytes, symlink escapes, `.git/`, and `.striatum/`.

## Custom Plan Compiler

`shape: "custom"` requires a `plan` object or `--plan` file:

```json
{
  "schema_version": "striatum.workflow_plan.v1",
  "blocks": [
    {"id": "draft", "kind": "draft"},
    {"id": "review", "kind": "review", "posture": "security"}
  ],
  "edges": [{"from": "draft", "to": "review", "on": "completed"}],
  "cycles": [
    {"from": "review", "to": "draft", "on_verdict": "needs_revision", "max_iterations": 1}
  ],
  "job_lane_bindings": {"draft": "author", "review": "security_reviewer"}
}
```

Closed block vocabulary:

```text
draft | review | synthesis | implementation | test |
human_checkpoint | support_ledger | evidence_audit | final_review
```

The custom compiler must reject unknown block kinds, duplicate block ids,
edges or cycles pointing at missing blocks, cycles without positive
`max_iterations`, review policy fields on non-review blocks, invalid posture
values, lane bindings to missing lanes, artifact paths outside write scope,
and any path that writes to `.striatum/` or escapes the repo. After compiling,
it still runs `validate_workflow`; the custom plan is an input convenience, not
a validator bypass.

## Existing Touch Points

- `src/striatum/workflow.py`: reuse `validate_workflow`,
  `workflow_graph_data`, `ALLOWED_POSTURES`, and harness-profile validation via
  normal workflow validation. Do not move validator authority into the
  generator.
- `src/striatum/cli/workflow_init.py`: replace direct starter assembly with
  generator calls or keep compatibility wrappers that call shared shape
  compiler functions.
- `src/striatum/cli/parser.py` and `src/striatum/cli/dispatch.py`: add
  template and generation routing.
- `src/striatum/service.py`: add local endpoint branches alongside existing
  `/workflows/` routes.
- `src/striatum/web/workflows.py`: can remain RFC 0024 discovery/detail code;
  generator service code should not be hidden there.
- `pyproject.toml`: include `striatum.workflow_templates` package data if the
  current package-data settings do not already include JSON resources.

## Deferred Work

Defer these to a follow-up dogfood exactly as RFC 0034 scopes them:

- Web `/workflows/new` chooser UI.
- Chat-assisted scaffolding tool.
- Target-repo local catalog extensions.
- Automatic repository inspection for suggested shapes.

The V1 implementation should still shape service envelopes so those follow-ups
can call the same generator API without inventing a second path.

## Tests

Unit tests should cover:

- every built-in shape with compatible lane sets validates under
  `validate_workflow`;
- every lane modifier in the compatibility matrix, including field-specific
  errors for incompatible or underspecified combinations;
- catalog loading, sorting, `list/show`, unknown template ids, and malformed
  package data;
- custom plans, including refusal cases for unknown block kinds, duplicate ids,
  unbounded cycles, invalid lane bindings, invalid posture fields, unsafe paths,
  and `.striatum/` writes;
- CLI dry-run writes nothing, non-dry-run refuses overwrite, and generated
  `workflow.json` revalidates after writing;
- `workflow init --style minimal|review|code-change` remains backward
  compatible enough for existing tests and docs;
- service preview does not require mutations and writes nothing;
- service generate is mutation-gated, requires `confirm_write: true`, refuses
  unsafe paths and overwrites, and returns structured `field_path` errors.

The highest-value regression test is a parameterized matrix over
`shape x lane_set x compatible_modifiers` that calls `generate_workflow`, parses
the returned `workflow.json`, and runs `validate_workflow` plus
`workflow_graph_data`. That catches drift between the generator and the runtime
contract early.
