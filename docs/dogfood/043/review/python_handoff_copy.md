author: implementer-unknown-model-001

# RFC 0045 Python Core Handoff

## Shipped Scope

Implemented the Python-side RFC 0045 core for multi-phase workflows:

- Workflow validation accepts `striatum.workflow.v1.1`, validates declared `phases`, requires `phase_id` on phased jobs, enforces one `phase_synthesis` job per phase, and rejects invalid cross-phase edges.
- Workflow graph materialization adds phase synthesis fan-in edges while keeping `needs` comparison on authored edges only.
- Run preparation materializes `phase_synthesis` jobs onto the existing review lifecycle so the current SQLite `jobs.job_type` constraint does not need an out-of-scope migration.
- `status --json` derives `phases` and `current_phase_id` from the workflow snapshot plus latest job attempts.
- Dashboard and service run-detail surfaces receive phase progress from the status payload.
- The workflow generator supports `shape: "multi_phase"` and emits v1.1 workflows with phased track jobs and synthesis gates.
- `striatum workflow upgrade --add-phases` previews by default and writes with `--apply`.

## Files Touched

- `src/striatum/workflow.py`
- `src/striatum/cli/introspect.py`
- `src/striatum/cli/mutations.py`
- `src/striatum/cli/parser.py`
- `src/striatum/cli/dispatch.py`
- `src/striatum/cli/workflow.py`
- `src/striatum/dashboard.py`
- `src/striatum/service.py`
- `src/striatum/workflow_generator/core.py`
- `src/striatum/workflow_generator/catalog.py`
- `tests/fixtures/multi_phase_workflow.json`
- `tests/test_workflow_phases.py`
- `tests/test_cli_mvp.py`
- `tests/test_dashboard.py`
- `tests/test_service.py`
- `tests/test_workflow_generator.py`
- `tests/test_workflow_upgrade.py`

## Verification

- `make lint` passed.
- `make typecheck` passed.
- `make smoke` passed.
- Focused phase suite passed: `pytest tests/test_workflow_phases.py tests/test_workflow_generator.py tests/test_workflow_upgrade.py tests/test_cli_mvp.py::test_v1_workflow_fixtures_validate_without_phase_progress tests/test_cli_mvp.py::test_multi_phase_workflow_lifecycle_and_phase_progress -q`.
- `make test` ran 734 tests: 702 passed, 31 skipped, 1 failed. The failure is `tests/test_doc_links.py::test_decision_log_rows_under_word_budget` because `docs/DECISION_LOG.md` row `D094` is over the word budget; that doc is outside this packet's write scope.

## Deviations

- Did not add phase runtime tables, `jobs.phase_id`, or migrations. Phase progress is derived from snapshots as selected by the synthesis.
- Did not edit `src/striatum/db.py` or migrations because they are outside write scope. `phase_synthesis` runtime rows are stored as review-lifecycle rows while the workflow snapshot retains the authored `phase_synthesis` type.
- Did not implement frontend React Flow phase bands; frontend work is outside Track A's Python scope.
