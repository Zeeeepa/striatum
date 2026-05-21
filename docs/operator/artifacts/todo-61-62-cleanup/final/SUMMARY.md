---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/todo-61-62-cleanup/build/HANDOFF.md", "docs/operator/artifacts/todo-61-62-cleanup/review/architecture/REVIEW.md", "docs/operator/artifacts/todo-61-62-cleanup/review/regression/REVIEW.md", "docs/operator/artifacts/todo-61-62-cleanup-revision/build/HANDOFF.md", "docs/operator/artifacts/todo-61-62-cleanup-revision/review/REVIEW.md"]
---

# TODO 61-62 Cleanup Final Summary
author: synthesizer-codex-002

## Accepted Cleanup

The accepted cleanup is the bounded TODO 61/62 slice plus the follow-up
state-path projection revision.

The original implementation fixed the visible TODO 62 operational-scratch
regression: `daemon doctor` now checks the registered repository's
`.striatum/` operational scratch directory instead of treating the retired
`.striatum/state.sqlite3` filename as live state. The doctor warning is now
`daemon_repo_scratch_missing`, with `state_dir` context and wording that names
the missing operational scratch directory.

The implementation also moved MCP repository projections toward the current
scratch-directory model, replaced hardcoded `.striatum/state.sqlite3` probes in
repository registration with `repo_policy.db_path(repo)`, and added production
guardrails asserting that production sources do not import the retired
`striatum.legacy_sqlite` package.

The regression review found one high-risk issue: Python repository list/resolve
projections could still leak stale `state_db_path` values that pointed at
`.striatum/state.sqlite3`. The revision fixed that by centralizing repository
projection in `src/striatum/daemon_pg/repositories.py`; `repo_list_pg`,
`repo_resolve_pg`, and duplicate `repo_add_pg` returns now normalize stale
SQLite-file paths to the `.striatum/` scratch directory without rewriting the
stored PostgreSQL row.

Final review disposition: accepted. The architecture-boundary review was
`accept_with_findings`; the initial regression review's high-risk F1 was
resolved by the revision, and the revision review accepted the result.

## Tests Run

From the original implementation:

- `pytest tests/test_daemon_pg_doctor.py tests/test_mcp_capability_scope_e2e.py tests/daemon_pg/test_repo_registration.py` -> 25 passed.
- `pytest tests/architecture/test_legacy_sqlite_quarantine.py tests/architecture/test_authority_guardrails.py` -> 12 passed, 1 skipped.
- `pytest tests/test_daemon_pg_sweep.py tests/cli/test_dispatch_daemon_doctor.py tests/cli/test_daemon_doctor_without_daemon.py` -> 21 passed.
- `pytest tests/cli/test_daemon_sqlite_import_retired.py tests/cli/test_daemon_core.py` -> 20 passed.
- Focused `ruff check` on the touched daemon PG and architecture test files -> passed.
- `git diff --check` -> passed.

From the accepted revision:

- `pytest tests/daemon_pg/test_repo_registration.py` -> 7 passed.
- `pytest tests/test_mcp_capability_scope_e2e.py tests/test_daemon_pg_doctor.py` -> 19 passed.
- `ruff check src/striatum/daemon_pg/repositories.py tests/daemon_pg/test_repo_registration.py` -> passed.
- `git diff --check` -> passed.

## Unresolved Blockers

No live workflow blocker remains for this cleanup slice. The run status has no
open blockers, and the only high-risk review finding was resolved and accepted
in the revision workflow.

There are residual product/backlog items, but they are not blockers for this
accepted cleanup:

- TODO 61 remains active for legacy SQLite fixture/import cleanup and
  compatibility-module deletion or conversion.
- TODO 62 remains mostly done; residual daemon-global gaps are future registry
  probes that guardrails may discover.
- TODO 63 remains mostly done around daemon client/service boundary completion,
  with PostgreSQL-native operator-composite policy still undecided.
- TODO 55, 56, 59, and 60 remain blocked on product decisions and were
  explicitly out of scope.

## Next Workflow To Queue

Queue the next bounded TODO 61 workflow for Track 2 test-debt reduction:
convert or quarantine the remaining `striatum.legacy_sqlite` test imports and
tighten the architecture guardrail once the converted coverage no longer needs
the fixture escape. The highest-value starting batch is the broad fixture-heavy
tests named in the cleanup plan: `tests/test_cli_mvp.py`,
`tests/test_service.py`, `tests/test_artifact_schemas.py`, and
`tests/test_process_adapter.py`.
