---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/todo-2-adapter-constraints-closure/map/MAP.md", "docs/operator/artifacts/todo-2-adapter-constraints-closure/build/HANDOFF.md", "docs/operator/artifacts/todo-2-adapter-constraints-closure/review/REVIEW.md"]
---

# TODO 2 Adapter Constraints Closure Summary
author: todo2-adapter-codex-001

## Result

TODO 2 is closeable for current process-adapter scope.

The source already implements the accepted enforcement model:
`required_enforcement` is validated, work packets expose requested and actual
adapter enforcement, and the `process` adapter honestly reports
`network=forbidden` and `repo_scope=local_only` as `advisory_strict` while
reporting `transcripts=off` as `enforced`.

I did not promote network or repository/filesystem isolation to `enforced`.
That would require a new sandbox/worktree adapter RFC with explicit OS
containment, network namespacing, filesystem namespace behavior, portability,
recovery, and operator UX.

## Changed Files

- `tests/test_workflow_adapter_constraints.py`
- `docs/operator/plans/todo-2-adapter-constraints-closure.md`
- `docs/operator/workflows/todo-2-adapter-constraints-closure/workflow.json`
- `docs/operator/workflows/todo-2-adapter-constraints-closure/prompts/map_current_adapter_constraints.md`
- `docs/operator/workflows/todo-2-adapter-constraints-closure/prompts/close_or_escalate_adapter_constraints.md`
- `docs/operator/workflows/todo-2-adapter-constraints-closure/prompts/review_adapter_boundary.md`
- `docs/operator/workflows/todo-2-adapter-constraints-closure/prompts/finalize_todo2_closure.md`
- `docs/operator/artifacts/todo-2-adapter-constraints-closure/map/MAP.md`
- `docs/operator/artifacts/todo-2-adapter-constraints-closure/build/HANDOFF.md`
- `docs/operator/artifacts/todo-2-adapter-constraints-closure/review/REVIEW.md`
- `docs/operator/artifacts/todo-2-adapter-constraints-closure/final/SUMMARY.md`

## Validation

- `PYTHONPATH=src python3 -m striatum.cli workflow validate docs/operator/workflows/todo-2-adapter-constraints-closure/workflow.json --json` -> valid.
- `PYTHONPATH=src .venv/bin/python -m striatum.cli workflow validate docs/operator/workflows/todo-2-adapter-constraints-closure/workflow.json --json` -> valid.
- `.venv/bin/python -m pytest -q tests/test_workflow_adapter_constraints.py` -> 2 passed.
- `.venv/bin/python -m pytest -q tests/test_process_adapter.py` -> 6 passed, 12 skipped.
- `.venv/bin/python -m pytest -q tests/test_cli_mvp.py::test_workflow_lane_constraints_validate_and_appear_in_packets tests/test_cli_mvp.py::test_process_adapter_scrubs_proxy_env_when_network_forbidden tests/test_cli_mvp.py::test_adapter_unavailable_flow_rejects_at_validation` -> 3 skipped by current fixture quarantine.
- `.venv/bin/python -m pytest -q tests/test_workflow_lint.py tests/test_workflow_generator.py` -> 33 passed.
- `.venv/bin/ruff check tests/test_workflow_adapter_constraints.py` -> passed.
- `git diff --check` -> passed.

System `python3 -m pytest` was not available because the system interpreter
does not have `pytest`; the repository virtualenv was used for test execution.

## Shared-Doc Updates Requested

Do not make these from this packet unless the operator explicitly opens that
scope:

- Update `docs/TODO.md` item 2 from "most done" to "done for current
  process-adapter scope".
- Refresh the TODO item 2 source pointer from the retired `src/striatum/db.py`
  location to `src/striatum/repo_policy.py`.
- Track enforced network/filesystem isolation as a separate future RFC/TODO
  item, not as remaining TODO 2 implementation debt.
