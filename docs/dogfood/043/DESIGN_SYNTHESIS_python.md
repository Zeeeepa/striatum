---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/043/design/python/codex/DESIGN.md", "docs/dogfood/043/design/python/claude_code/DESIGN.md", "docs/dogfood/043/design/python/gemini/DESIGN.md"]
---
author: designer-unknown-model-003

# RFC 0045 Python Core Synthesis

## Implementation Position

RFC 0045 V1 adds phases as validation and graph materialization over the existing workflow DAG. It does not add a phase scheduler, a phase table, or a required `jobs.phase_id` column. The workflow snapshot remains the source for phase metadata, and existing `jobs` plus `job_dependencies` remain the runtime execution substrate.

When the input designs disagree, this plan chooses the lowest schema and runtime surface that satisfies RFC 0045: explicit phase metadata in workflow JSON, materialized dependency rows for synthesis gates, and derived phase status from the workflow snapshot plus job rows. The column/migration design is deferred because current status and dashboard paths already load the snapshot, and V1 does not need SQL phase queries on unbounded run sets.

## Workflow Schema

The validator accepts exactly two workflow schema strings:

```text
striatum.workflow.v1
striatum.workflow.v1.1
```

Add constants in `src/striatum/workflow.py`:

```python
WORKFLOW_SCHEMA_VERSION_V1 = "striatum.workflow.v1"
WORKFLOW_SCHEMA_VERSION_V1_1 = "striatum.workflow.v1.1"
ACCEPTED_WORKFLOW_SCHEMA_VERSIONS = frozenset({
    WORKFLOW_SCHEMA_VERSION_V1,
    WORKFLOW_SCHEMA_VERSION_V1_1,
})
VERDICT_JOB_TYPES = frozenset({"review", "phase_synthesis"})
```

For `striatum.workflow.v1`, top-level `phases` and job-level `phase_id` are forbidden. Existing v1 workflows must validate, plan, prepare, start, claim, complete, and report status unchanged.

For `striatum.workflow.v1.1`, top-level `phases` is optional. If it is missing or empty, no job may declare `phase_id` and the workflow behaves like v1 except for the accepted schema string. If `phases` is non-empty, every job must declare `phase_id`.

The accepted phase object shape is:

```json
{
  "id": "phase_1_design",
  "name": "Design",
  "color": "#6b7280",
  "description": "Parallel design tracks and synthesis"
}
```

`id` and `name` are required non-empty strings. `color` and `description` are optional strings. The core does not validate CSS color syntax.

The accepted job delta is:

```json
{
  "id": "synthesize_design",
  "type": "phase_synthesis",
  "phase_id": "phase_1_design"
}
```

Do not support RFC 0045's draft `phase` alias or `phases[].title` / `phases[].synthesis_job_id` fields in V1. The prompt chooses `phase_id`, and deriving synthesis from the unique `phase_synthesis` job in each phase avoids duplicating the source of truth. A compatibility alias can be a later migration if real authored workflows need it.

## Validator Plan

Modify `validate_workflow()` in `src/striatum/workflow.py` at the current schema gate and the post-job-validation section. The order is:

1. Check required top-level fields.
2. Accept schema version by membership in `ACCEPTED_WORKFLOW_SCHEMA_VERSIONS`.
3. Validate repository, provenance, branch, recovery, lanes, profiles, and roles as today.
4. Validate jobs as today, with review-only fields still review-only.
5. Build `phase_index = _validate_phases(workflow, job_map=job_map)`.
6. Validate explicit edges with `edge_dependency_pairs(workflow, include_phase_materialized=False)`.
7. Validate cross-phase edge rules with `_validate_phase_edges(explicit_edges, job_map=job_map, phase_index=phase_index)`.
8. Run `validate_needs_match_edges()` against explicit edges only.
9. Run cycle, parallelism, revision-policy, and warning checks against materialized edges where graph reachability matters.

Add a private immutable-ish structure:

```python
PhaseIndex = dict[str, Any]
```

with JSON-safe keys:

```json
{
  "declared": true,
  "phase_order": ["phase_1_design", "phase_2_build"],
  "phase_by_id": {"phase_1_design": {"id": "phase_1_design", "name": "Design"}},
  "phase_position": {"phase_1_design": 0, "phase_2_build": 1},
  "job_phase": {"design_python": "phase_1_design"},
  "synthesis_by_phase": {"phase_1_design": "synthesize_design"}
}
```

`_validate_phases()` enforces these rules:

- v1 with `phases` is rejected.
- v1 with any job `phase_id` is rejected.
- v1.1 with no phases rejects any job `phase_id` and rejects any `phase_synthesis` job.
- Non-empty `phases` must be a list of objects with unique non-empty `id` and non-empty `name`.
- When phases are declared, every job must have a known `phase_id`.
- Every phase has exactly one job where `type == "phase_synthesis"` and `phase_id` matches that phase.
- Every `phase_synthesis` has at least one other job in its phase.
- `phase_synthesis` must not declare `reviewer_access_scope`, `reviewer_context_policy`, `review_posture`, or `required_review_postures`; those remain review-job concepts even though synthesis records a verdict.

Cross-phase edge rule:

- Same-phase edges are unrestricted.
- Edges from an earlier phase to a later phase are allowed only when the source job is the source phase's `phase_synthesis` job.
- Edges from a later phase to an earlier phase are rejected.
- Phase skips are rejected: an edge from phase index `N` to phase index `M` is valid only when `M == N + 1`.
- Ordinary job to ordinary job across phases is rejected.
- Edges into a later phase's `phase_synthesis` from an earlier phase are rejected because the later synthesis must summarize its own phase, not serve as a phase entry.

This chooses the stricter source-synthesis-only edge contract from RFC 0045 because it prevents the dogfood-042 cascade ambiguity while keeping phase entry points explicit.

## Graph Materialization

Refactor `edge_dependency_pairs()` in `src/striatum/workflow.py` to accept an optional keyword:

```python
def edge_dependency_pairs(
    workflow: JsonObject,
    *,
    include_phase_materialized: bool = True,
) -> list[tuple[str, str, JsonObject]]:
```

The default includes phase-materialized edges so existing callers like `plan_workflow()`, `workflow_graph_data()`, Mermaid/DOT output, and `create_run()` see the executable graph. Validation paths that compare author-declared `needs` to author-declared `edges` pass `include_phase_materialized=False`.

Materialized phase edges are:

- For each phase, every non-synthesis job in that phase gets an edge to the phase's `phase_synthesis` job if absent.
- No implicit fan-out is invented. Authors and the generator must declare edges from phase `N` synthesis to phase `N+1` entry jobs. This keeps graph intent reviewable and avoids hidden root changes in the next phase.

Use de-duplication by `(from_id, to_id)` so explicit fan-in edges are not duplicated.

## Runtime Changes

`create_run()` in `src/striatum/workflow.py` is the runtime materialization site. It already snapshots the workflow JSON, inserts jobs, and inserts dependency rows from `edge_dependency_pairs(workflow)`. Keep that shape:

- Do not add a phase table.
- Do not add a `jobs.phase_id` column in V1.
- Insert all jobs exactly as today, including `job_type == "phase_synthesis"`.
- Insert explicit and materialized synthesis fan-in edges through the existing `job_dependencies` table.
- Gate downstream dependencies from verdict-capable jobs with:

```python
if upstream_job.get("type") in VERDICT_JOB_TYPES:
    gate_json["requires_verdict"] = ["accept", "accept_with_findings"]
```

Apply the same `VERDICT_JOB_TYPES` change to `_planned_edge()`.

Broaden verdict internals in `src/striatum/db.py`:

- Rename `record_review_verdict()` internally only if useful, but at minimum allow `job_type in {"review", "phase_synthesis"}`.
- Keep the event type `verdict.recorded`.
- Use `_resolve_review_posture()` only for review jobs; phase synthesis verdicts should store posture `"neutral"`.
- For `needs_revision`, reuse the existing cycle path. A `phase_synthesis` may participate in an explicit cycle just like review jobs.
- For `reject`, reuse the existing fail path.

Broaden `prevalidate_submit_review()` in `src/striatum/cli/mutations.py` to accept both verdict job types. Keep the CLI verb `submit-review` for V1 compatibility, but update help/error text to "verdict-capable jobs" where possible. The packet already exposes a separate `verdict` command shape, so no new agent command is required for RFC 0045 V1.

Do not add `phase.entered` or `phase.exited` events in V1. They are useful, but they are not required for the acceptance criteria and would create derived state that can drift from the dependency graph. Status can derive current phase directly.

## Phase Status Shape

Add a helper in `src/striatum/cli/introspect.py`:

```python
def phase_progress_for_run(conn: sqlite3.Connection, *, run_id: str) -> JsonObject | None:
    ...
```

It loads `runs.workflow_snapshot_id`, parses `workflow_snapshots.workflow_json`, builds the same phase index from `workflow_phase_index(workflow)`, then reads current highest-attempt job rows for the run. Since V1 does not add `jobs.phase_id`, map each row's `workflow_job_id` back to `phase_index["job_phase"]`.

`striatum status --json` adds this field only when the selected run has declared phases:

```json
{
  "phases": [
    {
      "id": "phase_1_design",
      "name": "Design",
      "description": "Parallel design tracks and synthesis",
      "color": "#6b7280",
      "index": 0,
      "state": "active",
      "jobs_total": 5,
      "jobs_completed": 3,
      "jobs_by_state": {"completed": 3, "running": 1, "blocked": 1},
      "synthesis_job_id": "synthesize_design",
      "synthesis_state": "blocked",
      "synthesis_verdict": null
    }
  ],
  "current_phase_id": "phase_1_design"
}
```

Phase state is derived:

- `completed` when every job in the phase is `completed`.
- `active` when any job is `queued`, `claimed`, `running`, `waiting_human`, or `blocked`.
- `failed` when any job is `failed`.
- `canceled` when every incomplete job is `canceled` or `skipped`.
- `pending` otherwise.

`current_phase_id` is the first phase in order whose state is not `completed`; if all phases are completed, it is the last phase id.

## Dashboard And Service Sites

`src/striatum/dashboard.py`:

- `gather_payload()` already calls `status_command(conn, run_id=run_id)`, so no new query is needed.
- `render_frame()` should read `status_payload["phases"]` and render a compact phase line after the jobs/verdicts panels and before claimable/next actions:

```text
Phases: Design 3/5 active | Build 0/4 pending
```

- Add `_render_phases(phases, width)` as a small sibling to `_render_claimable()` and `_render_events()`.
- Graph renderers may later group by phase, but V1 Python core only needs the status panel. React Flow phase bands are Track B.

`src/striatum/service.py`:

- `_render_run_detail_page()` already computes `status_payload`; pass `phase_progress=status_payload.get("phases") or []` and `current_phase_id=status_payload.get("current_phase_id")` into `run_detail.html`.
- JSON status endpoints that call `status()` surface phases automatically.
- Do not duplicate phase math in service route code or templates.

## Generator Catalog Shape

Change `src/striatum/workflow_generator/core.py`:

- Add `"multi_phase"` to `SHAPES`.
- Add `"phases"` to `OPTION_KEYS`.
- Change `generate_workflow()` so `_compile_shape()` can return optional `phases`. The least invasive signature is:

```python
def _compile_shape(spec: WorkflowGenerationSpec) -> tuple[
    list[JsonObject],
    list[JsonObject],
    list[JsonObject],
    list[JsonObject],
]:
```

Existing shapes return `([], phases)` as the fourth value. `generate_workflow()` emits `schema_version: "striatum.workflow.v1.1"` and `workflow["phases"] = phases` only when `spec.shape == "multi_phase"`.

Add `_compile_multi_phase(spec)` in `src/striatum/workflow_generator/core.py`. It consumes `spec.options["phases"]`:

```json
[
  {
    "id": "phase_1_design",
    "name": "Design",
    "description": "Parallel design",
    "color": "#6b7280",
    "tracks": [
      {"id": "python", "shape": "minimal", "lane_id": "author"},
      {"id": "docs", "shape": "review", "lane_id": "author"}
    ],
    "synthesis_lane_id": "reviewer"
  }
]
```

The generator must:

- Prefix generated track job ids as `{phase_id}__{track_id}__{base_id}`.
- Set `phase_id` on every generated job.
- Scope `parallel_group` as `{phase_id}:{track_id}`.
- Emit one `phase_synthesis` job per phase with id `{phase_id}__synthesis`, `role_id: "reviewer"`, `lane_id` from `synthesis_lane_id` or the first reviewer lane, and a required synthesis artifact at `{artifact_root}/{phase_id}/SYNTHESIS.md`.
- Emit explicit edges from each phase synthesis job to each next phase entry job.
- Rely on validator materialization for same-phase fan-in, though explicit fan-in is acceptable if already present.
- Validate the generated workflow before returning.

Update `src/striatum/workflow_templates/catalog.json` with a `multi_phase` shape entry. No new package directory is needed; the catalog loader already reads the JSON package data through `src/striatum/workflow_generator/catalog.py`.

## Upgrade CLI

Extend the existing `striatum workflow upgrade <path>` parser in `src/striatum/cli/parser.py`:

```text
striatum workflow upgrade <path> [--add-phases] [--phase-map <path>] [--force] [--dry-run] [--json]
```

Do not add `--apply`; the current command already writes unless `--dry-run`, and keeping that convention avoids a second write contract for the same verb.

Implementation in `src/striatum/cli/workflow.py`:

- Keep the current harness-profile upgrade as the default when `--add-phases` is absent.
- With `--add-phases`, load the workflow, refuse if it is not v1, refuse if it already has `phases` unless `--force`, and reuse `_running_runs_for_workflow()` so non-terminal runs refuse mutation unless `--dry-run`.
- If `--phase-map` is provided, parse it as JSON mapping:

```json
{
  "phases": [{"id": "phase_1_design", "name": "Design"}],
  "jobs": {"design_python": "phase_1_design"}
}
```

- Without `--phase-map`, infer phases from `parallel_group` prefixes split on the first `_`, `-`, or `:`. Preserve source job order for phase ordering.
- Relabel every existing job with `phase_id`.
- Reuse an existing terminal consolidate/synthesis job as `type: "phase_synthesis"` when all ordinary jobs in the inferred phase already feed it; otherwise insert a new `{phase_id}__synthesis` job with reviewer role and reviewer lane fallback.
- Rewrite cross-phase ordinary edges to originate from the source phase synthesis job.
- Add explicit phase-synthesis-to-next-phase-entry edges.
- Bump `schema_version` to `striatum.workflow.v1.1`.
- Re-run `validate_workflow()` on the rewritten object before writing.

Return envelope:

```json
{
  "workflow_path": "workflow.json",
  "status": "would_update",
  "mode": "add_phases",
  "phases_added": [{"id": "phase_1_design", "name": "Design"}],
  "jobs_relabelled": [{"job_id": "design_python", "phase_id": "phase_1_design"}],
  "jobs_inserted": [{"job_id": "phase_1_design__synthesis", "type": "phase_synthesis"}],
  "edges_rewritten": [{"from": "build", "to": "test", "rewritten_from": "phase_1__synthesis"}],
  "warnings": []
}
```

Use the existing `workflow_path.write_text(json.dumps(...))` pattern in this module for consistency.

## Backwards Compatibility Matrix

These existing behaviors must continue unchanged:

- Every current `striatum.workflow.v1` fixture under `examples/` and `tests/fixtures/` validates without edits.
- Existing generator shapes `minimal`, `review`, `code_change`, `human_checkpoint`, `evidence_backed`, `multi_review_synthesis`, and `custom` still emit v1.
- `plan_workflow()` and `workflow_graph_data()` output for v1 fixtures remains byte-identical except for deliberate test fixture updates caused by unrelated ordering.
- `create_run()` for v1 inserts the same jobs and dependency rows.
- `striatum status --json` for v1 omits `phases` and `current_phase_id`.
- `striatum dashboard --once` for v1 omits the phase panel.
- Existing review lifecycle tests for `submit-review`, verdict gates, review cycles, override verdicts, and process completion still pass.

## New Test Plan

Add focused validator tests, preferably in `tests/test_workflow_phases.py`:

- Accept v1.1 with two phases, all jobs assigned, one `phase_synthesis` per phase, and cross-phase edges from phase synthesis to next phase entries.
- Reject v1 with top-level `phases`.
- Reject v1 with job `phase_id`.
- Reject v1.1 with phases where a job lacks `phase_id`.
- Reject duplicate phase ids.
- Reject missing or empty phase names.
- Reject unknown job `phase_id`.
- Reject missing `phase_synthesis`.
- Reject duplicate `phase_synthesis` jobs in one phase.
- Reject empty phase containing only synthesis.
- Reject `phase_synthesis` without `phase_id`.
- Reject ordinary cross-phase edge.
- Reject backward phase edge.
- Reject phase skip.
- Accept intra-phase ordinary edges.
- Confirm materialized edge list includes same-phase fan-in to synthesis and de-duplicates explicit fan-in.

Add runtime tests:

- `create_run()` materializes explicit edges plus implicit synthesis fan-in into `job_dependencies`.
- `run start` enqueues only first-phase root jobs when second phase entry jobs depend on first phase synthesis.
- `submit-review` or the verdict command accepts a running `phase_synthesis` job.
- Accepting a `phase_synthesis` verdict completes the job and unblocks next-phase entries through existing `requires_verdict` gates.
- A `needs_revision` verdict on `phase_synthesis` follows an explicit cycle when declared.
- Status for a v1.1 run returns the phase payload and current phase id.
- Dashboard output includes the compact phase line only for v1.1 runs.
- Service run detail receives phase progress from `status()`.

Add generator and upgrade tests:

- `generate_workflow()` with `shape: "multi_phase"` emits v1.1, `phases`, phase-scoped job ids, one synthesis per phase, and a valid graph.
- Catalog list/show includes `multi_phase`.
- `workflow upgrade --add-phases --dry-run` reports a change set and does not write.
- `workflow upgrade --add-phases` rewrites a v1 fixture, validates the result, and refuses non-terminal runs through the existing running-run guard.
- `workflow upgrade --add-phases --phase-map <path>` honors explicit mapping and rejects unknown job ids or phases.

Add one end-to-end fixture `tests/fixtures/multi_phase_workflow.json` with two phases and two tracks per phase. The CI lifecycle test should prepare the run, confirm/start it, complete first-phase track jobs, run the phase synthesis verdict, assert second-phase jobs become claimable only after the accepting verdict, then complete the run.

## Implementation Order

1. Add schema constants, `VERDICT_JOB_TYPES`, phase index helpers, and validator tests.
2. Refactor edge dependency handling to separate explicit and materialized edges.
3. Materialize same-phase fan-in in `create_run()` through `edge_dependency_pairs()` and generalize verdict gates.
4. Broaden verdict recording/prevalidation for `phase_synthesis`.
5. Add status phase progress, dashboard phase line, and service pass-through.
6. Add generator `multi_phase` support and catalog entry.
7. Add `workflow upgrade --add-phases`.
8. Add the full multi-phase lifecycle fixture and e2e test.

The central invariant is that phase behavior is visible as ordinary jobs, ordinary dependency rows, and ordinary verdict gates. That keeps RFC 0045 inside the runner's current execution model while giving the validator and UI enough structure to prevent phase bypasses.
