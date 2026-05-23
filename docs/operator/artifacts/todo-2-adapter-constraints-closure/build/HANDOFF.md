---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["tests/test_workflow_adapter_constraints.py"]
---

# TODO 2 Adapter Constraint Handoff
author: todo2-adapter-codex-001

## Summary

Closed the current-scope TODO 2 gap by preserving the existing honest
enforcement boundary and adding a focused guardrail test.

The implementation now has a direct test that:

- pins the `process` adapter enforcement matrix:
  `network=forbidden` -> `advisory_strict`,
  `repo_scope=local_only` -> `advisory_strict`,
  `transcripts=off` -> `enforced`;
- validates that workflows may require `advisory_strict` for network and repo
  scope plus `enforced` for transcript-off;
- proves `worktree_isolation=per_job` does not promote `repo_scope` to
  `enforced`.

No source behavior was changed. No sandbox adapter, network namespace,
filesystem namespace, hosted service, telemetry, transcript capture, provider
SDK, or external persistence was added.

## Changed Files

- `tests/test_workflow_adapter_constraints.py`
- `docs/operator/plans/todo-2-adapter-constraints-closure.md`
- `docs/operator/workflows/todo-2-adapter-constraints-closure/workflow.json`
- `docs/operator/workflows/todo-2-adapter-constraints-closure/prompts/*.md`
- `docs/operator/artifacts/todo-2-adapter-constraints-closure/`

## Boundary Decision

The remaining `advisory_strict` -> `enforced` promotion for network and
repository/filesystem isolation requires a future RFC. It is not honest to
claim that the current `process` adapter or per-job git worktrees enforce
those constraints.
