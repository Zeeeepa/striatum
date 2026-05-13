# RFC 0045 Python Core Design

author: designer-unknown-model-002

## Scope

This handoff designs the Python-core implementation for RFC 0045 V1: workflow schema `striatum.workflow.v1.1`, ordered phases, `phase_synthesis` jobs, validator rules, runtime materialization, status reporting, generator support, and the workflow upgrade path. It intentionally does not design the React Flow editor or any frontend rendering.

The current core is already graph-first. `src/striatum/workflow.py` validates JSON, normalizes dependency edges, snapshots the workflow into a run, inserts jobs and `job_dependencies`, and leaves execution to existing job/edge gates. RFC 0045 should preserve that shape: phases are metadata and validation constraints over the existing job graph, not a new executor.

## Schema Contract

Accept both `striatum.workflow.v1` and `striatum.workflow.v1.1` in `validate_workflow()` at `src/striatum/workflow.py:475`. The current hard check at `src/striatum/workflow.py:495` must become membership in a small constant such as `WORKFLOW_SCHEMA_VERSIONS = {"striatum.workflow.v1", "striatum.workflow.v1.1"}`. Existing v1 workflows continue to validate unchanged.

For v1.1, add an optional top-level `phases` array. Use the prompt's field names as the implementation contract: each phase is `{id, name, color?, description?}`. Do not implement the RFC draft's older `title` / `synthesis_job_id` phase shape; instead derive a phase's synthesis job from the unique `jobs[].type == "phase_synthesis"` job with `phase_id` equal to that phase id. Jobs gain optional `phase_id`. For v1 workflows, both top-level `phases` and job-level `phase_id` are forbidden so v1 behavior stays unambiguous.

Valid v1.1 phase rules:

- `phases` is optional. If omitted or empty, the workflow runs in implicit single-phase mode and no job may declare `phase_id`.
- If `phases` is present, every phase id is a non-empty unique string.
- `name` is a non-empty string. `color` and `description`, when present, are strings. The core should not validate CSS color semantics; that belongs to UI affordances.
- Every job declaring `phase_id` must reference a declared phase. If phases are declared, every job should declare `phase_id`; this keeps status math and cross-phase validation deterministic.
- Each declared phase must have exactly one `phase_synthesis` job in that same phase, except the last phase may omit one only if synthesis is not needed for a later phase. Prefer the stricter V1 rule from the prompt/RFC acceptance criteria: exactly one per phase, because it makes phase status and generator output simple.
- `phase_synthesis` requires `phase_id`, normal `role_id`, normal `lane_id`, normal `expected_artifacts`, and a verdict-capable lifecycle.

## Validator Changes

Add a helper in `src/striatum/workflow.py`, called from `validate_workflow()` after `job_map` is built and before `edge_dependency_pairs(workflow)` at `src/striatum/workflow.py:576`:

```python
def _validate_phases(workflow: JsonObject, *, job_map: dict[str, JsonValue]) -> PhaseIndex:
    ...
```

Return a small typed structure or JSON-safe dict containing `phase_order`, `phase_by_id`, `job_phase`, and `synthesis_by_phase`. Recomputeing this is fine, but returning it avoids reparsing inside validation helpers. Keep the helper private unless status/generator code needs it; if reused, expose a read-only `workflow_phase_index(workflow)`.

The helper extends existing validator patterns:

- It should use `_list()` and `_string()` style validation, matching the current top-level/job parsing in `validate_workflow()` at `src/striatum/workflow.py:492` through `src/striatum/workflow.py:574`.
- It should run after role/lane validation so phase errors do not mask invalid job references.
- It should not mutate workflow JSON. The RFC draft mentions validator-generated implicit dependencies for synthesis. Do not generate edges in the validator; instead require explicit edges or materialize extra dependency rows in `create_run()` as described below. Validation should stay a validator.

Cross-phase edge enforcement belongs next to normalized edge handling. The current `edge_dependency_pairs()` at `src/striatum/workflow.py:731` verifies edge object shape, `from`, `to`, job existence, and `on == "completed"`. Leave that function as the basic normalizer, then add `_validate_phase_edges(workflow, job_map=job_map, phase_index=phase_index)` after it. The rules:

- If no explicit phases exist, all edges are intra-phase and pass.
- Same-phase edges are unrestricted.
- Edges may not point backward from a later phase to an earlier phase.
- Edges crossing from phase N to phase M where `M > N` are valid only when either endpoint is a `phase_synthesis` job. This implements the packet's rule that cross-phase dependencies must originate from or terminate at synthesis.
- For the recommended stricter transition model, entry jobs in phase `N+1` should depend on phase `N`'s synthesis job, and no ordinary job in phase `N` may edge directly to an ordinary job in phase `N+1`.
- Cross-phase skips, such as phase 1 directly to phase 3, should be rejected in V1 unless the edge is from phase 1 synthesis to phase 2 synthesis through explicit phase 2 work. RFC 0045 says linear phases only.

`phase_synthesis` must be treated as verdict-capable wherever the current code special-cases `review`:

- `create_run()` currently adds `requires_verdict` to dependency gates only when `upstream_job.get("type") == "review"` at `src/striatum/workflow.py:696`. Extend this to `upstream_job.get("type") in VERDICT_JOB_TYPES`, where `VERDICT_JOB_TYPES = {"review", "phase_synthesis"}`.
- `_planned_edge()` mirrors that review-gate logic at `src/striatum/workflow.py:804`; extend it so graph/plan JSON reports phase synthesis gates correctly.
- `_planned_review_gates()` at `src/striatum/workflow.py:811` can either be generalized to `_planned_verdict_gates()` or left as review-only with a new sibling `_planned_phase_gates()`. Prefer generalization only if it stays small.
- Review-policy fields should remain review-only. Do not let `phase_synthesis` declare `reviewer_access_scope`, `reviewer_context_policy`, `review_posture`, or `required_review_postures`; those functions currently enforce review-only semantics around `src/striatum/workflow.py:1443` and later.

## Runtime Materialization

`create_run()` at `src/striatum/workflow.py:610` is the exact function that snapshots workflow JSON and materializes jobs and dependency rows. It currently:

- calls `load_workflow()` at `src/striatum/workflow.py:612`;
- inserts the workflow snapshot at `src/striatum/workflow.py:617`;
- inserts every workflow job into `jobs` at `src/striatum/workflow.py:652`;
- inserts dependency rows from `edge_dependency_pairs(workflow)` at `src/striatum/workflow.py:693`.

Do not add a phase table in V1. The workflow snapshot already stores phase metadata, and the executor can continue operating over `jobs` plus `job_dependencies`. Store no new SQLite columns unless implementers decide phase querying is too expensive; status can derive phase membership from the snapshot and job rows.

For synthesis fan-in, materialize extra dependency rows in `create_run()` rather than mutating the workflow:

- Build `phase_index = workflow_phase_index(workflow)`.
- For each `phase_synthesis` job, add dependency rows from every other job in the same phase to that synthesis job if the row does not already exist.
- Use the same `INSERT OR IGNORE INTO job_dependencies` pattern at `src/striatum/workflow.py:699`.
- Existing explicit author edges still materialize normally.

This gives the RFC's "implicit dependency on every other job in the same phase" without hiding extra edges in `workflow_graph_data()`. If graph/plan output should show those implicit dependencies, add them through a new `materialized_dependency_pairs(workflow)` helper and use that in `plan_workflow()` at `src/striatum/workflow.py:196`, `workflow_graph_data()` at `src/striatum/workflow.py:250`, and `create_run()`.

Run start needs no phase-specific scheduler. `run_start()` in `src/striatum/cli/mutations.py:187` enqueues root jobs by selecting jobs with no dependency rows (`src/striatum/cli/mutations.py:204`). Once phase synthesis fan-in and next-phase edges are materialized in `job_dependencies`, root detection and downstream enqueue stay correct. `dependencies_satisfied()` in `src/striatum/db.py:528` already blocks on completed upstream jobs and optional `requires_verdict` gates, so phase transitions work through existing edge gates.

The main runtime change outside `workflow.py` is verdict admission. `record_verdict()` currently refuses non-review jobs at `src/striatum/db.py:1579`, `override_review_verdict()` refuses non-review jobs at `src/striatum/db.py:1658`, and `prevalidate_submit_review()` refuses non-review jobs at `src/striatum/cli/mutations.py:747`. Either broaden these to `VERDICT_JOB_TYPES` or add parallel phase-synthesis-specific command names. I recommend broadening the existing internals while keeping the CLI verb `submit-review` for now only if product accepts the naming mismatch. Cleaner V1 naming would add `striatum verdict` support for `phase_synthesis` and keep `submit-review` review-only.

## Status Reporting

Add a `phases` block to `striatum status --json` only for workflows whose snapshot has declared phases. The existing aggregation point is `status()` in `src/striatum/cli/introspect.py:161`; it already counts jobs by state at `src/striatum/cli/introspect.py:169` and returns the payload at `src/striatum/cli/introspect.py:196`. Extend it by loading the run's workflow snapshot, building a phase index, reading latest job rows for the run, and returning:

```json
{
  "phases": [
    {
      "id": "phase_1",
      "name": "Design",
      "state": "active",
      "jobs_total": 5,
      "jobs_done": 3,
      "jobs_by_state": {"completed": 3, "running": 1, "blocked": 1},
      "synthesis_job_id": "synthesize_phase_1",
      "synthesis_state": "blocked"
    }
  ],
  "current_phase_id": "phase_1"
}
```

Define `jobs_done` as terminal-success jobs (`completed`) only. `failed`, `canceled`, and `skipped` should be counted separately but not done. Define current phase as the first phase in order with any non-completed job; if all jobs are completed, use the last phase with state `completed`.

`dashboard.gather_payload()` at `src/striatum/dashboard.py:57` already calls `status_command(conn, run_id=run_id)` at `src/striatum/dashboard.py:75`, so the dashboard can consume the new `status["phases"]` without another query. `render_frame()` at `src/striatum/dashboard.py:128` currently renders job counts with `_render_left_column()` around `src/striatum/dashboard.py:209`. Add a compact phase panel between the job/verdict columns and claimable rows. Keep it text-only: `Phases: Design 3/5 active | Build 0/4 pending`.

For the local service, run-detail handling reads workflow and job rows around `src/striatum/service.py:1010` and passes them to HTML rendering. The service should either call the same phase-summary helper used by `status()` or call `status()` and pass `phases` through the page context. Avoid duplicating phase math in Jinja or service route code.

## Generator And Catalog

`src/striatum/workflow_generator/core.py` has a closed `SHAPES` choice consumed in `WorkflowGenerationSpec.from_json()` at `src/striatum/workflow_generator/core.py:119`, and `_compile_shape()` dispatches concrete shapes at `src/striatum/workflow_generator/core.py:352`. Add `multi_phase` to the shape set and the package-data catalog entry used by `workflow templates list/show`.

`generate_workflow()` currently hardcodes `"schema_version": "striatum.workflow.v1"` at `src/striatum/workflow_generator/core.py:224`. For `spec.shape == "multi_phase"`, emit `striatum.workflow.v1.1` plus `phases`. Other shapes remain v1.

Compiler design:

- Accept phase definitions through `spec.options["phases"]` for V1 rather than extending the generator schema. Each phase option should include `id`, `name`, optional `description`, optional `color`, and `tracks`.
- Generate per-track jobs with `phase_id` and `parallel_group` names scoped by phase id.
- Generate one `phase_synthesis` job per phase with `phase_id`, a synthesis artifact, and lane selected from `synthesis_lane` or the author lane.
- Emit explicit edges from each track terminal job to that phase's synthesis job, and from each phase synthesis job to the next phase's entry jobs. This makes generated workflows readable even though runtime also materializes implicit fan-in.
- Call `validate_workflow(workflow)` exactly as `generate_workflow()` already does at `src/striatum/workflow_generator/core.py:241`.

## Workflow Upgrade CLI

The existing `striatum workflow upgrade <path>` parser lives at `src/striatum/cli/parser.py:220`, and the implementation is `workflow_upgrade()` in `src/striatum/cli/workflow.py:35`. Extend this command with `--add-phases`, `--phase-map <json-or-path>`, and an apply/dry-run behavior consistent with the current `--dry-run` and running-run refusal.

Recommended behavior:

- Without `--add-phases`, preserve existing RFC 0040 harness-profile upgrade behavior.
- With `--add-phases`, load and validate the existing v1 workflow, refuse if it is already v1.1 unless `--force`, then write v1.1 plus phase metadata.
- Use `parallel_group` clusters as the heuristic. Split group names on the first underscore or hyphen and group shared prefixes into phase candidates, preserving topological order from `edge_dependency_pairs()`.
- If `--phase-map` is supplied, use it as authoritative mapping from job id to phase id and phase metadata. Validate every job id is known and every phase id exists.
- Emit a JSON change set with added phases, changed jobs, inserted synthesis jobs, and inserted edges. Write only when not `--dry-run`.

Implementation note: `src/striatum/cli/workflow.py` currently uses `workflow_path.write_text()` directly at `src/striatum/cli/workflow.py:187`; that is acceptable for this module's existing pattern, but the upgraded workflow must be revalidated after rewriting and before writing.

## Backwards Compatibility And Tests

The compatibility invariant is strict: all existing `striatum.workflow.v1` workflows validate, prepare, start, claim, complete, and report status exactly as before unless they opt into v1.1.

Test matrix:

- Validator accepts an existing v1 fixture with no `phases`.
- Validator rejects v1 with top-level `phases`.
- Validator rejects v1 job with `phase_id`.
- Validator accepts v1.1 with phases, job `phase_id`, one `phase_synthesis` per phase, and cross-phase edges through synthesis.
- Validator rejects duplicate phase ids, missing phase names, unknown job phase ids, missing synthesis job, duplicate synthesis jobs, and `phase_synthesis` without `phase_id`.
- Validator rejects ordinary cross-phase edge from phase 1 job to phase 2 job.
- Validator accepts intra-phase edges between ordinary jobs.
- Validator rejects backward phase edges and phase skips.
- `create_run()` materializes all jobs with existing columns, inserts explicit edges, and inserts implicit same-phase fan-in rows for synthesis.
- `run_start()` enqueues only phase 1 root jobs when later phase jobs depend on phase 1 synthesis.
- Accepting verdict on a `phase_synthesis` job unblocks next-phase entry jobs through existing `requires_verdict` gates.
- Non-accepting verdict on `phase_synthesis` follows the chosen review-like failure/revision behavior.
- `status --json` omits `phases` for v1 and includes accurate phase counts for v1.1.
- Dashboard render includes a compact phase line only when status includes phases.
- Generator `multi_phase` emits valid v1.1.
- `workflow upgrade --add-phases --dry-run` reports changes without writing.
- `workflow upgrade --add-phases` refuses workflows referenced by non-terminal runs, matching the guard in `src/striatum/cli/workflow.py:80`.

## Implementation Order

1. Add schema-version constants, phase parsing/indexing helpers, and validator tests.
2. Generalize verdict-capable job constants and gate generation for `review` plus `phase_synthesis`.
3. Materialize phase synthesis fan-in in `create_run()` and add lifecycle tests.
4. Add status phase summaries and dashboard/service pass-through.
5. Add generator `multi_phase` and catalog metadata.
6. Extend `workflow upgrade --add-phases` and add dry-run/apply tests.

The central design constraint is that phases stay a layer over the existing graph. The executor already knows how to advance blocked jobs when dependency rows and verdict gates are satisfied; RFC 0045 should feed that executor better graph metadata rather than creating a second scheduler.
