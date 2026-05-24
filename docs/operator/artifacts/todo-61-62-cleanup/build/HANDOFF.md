# TODO 61-62 Cleanup Handoff
author: implementer-codex-001

## Summary

Implemented the bounded TODO 62 cleanup from the plan and added a production
guardrail for TODO 61:

- `daemon doctor` now checks the registered repository's `.striatum/`
  operational scratch directory instead of the historical
  `.striatum/retired-local-state` filename.
- The doctor warning is now `daemon_repo_scratch_missing`, with `state_dir`
  in context and a message that names the missing `.striatum/` operational
  scratch directory.
- MCP daemon repository projections now expose `state_dir` and normalize
  stale `state_db_path` values that still point at `retired-local-state` to the
  `.striatum/` scratch directory.
- Python repository registration now uses `repo_policy.db_path(repo)` for the
  retired SQLite migration-refusal and symlink probes instead of duplicating
  `.striatum/retired-local-state` literals.
- Architecture guardrails now assert that the production
  `src/striatum/legacy_sqlite` package remains deleted and production sources
  do not import it.

## Files Changed

- `src/striatum/daemon_pg/client_admin.py`
- `src/striatum/daemon_pg/mcp_resources.py`
- `src/striatum/daemon_pg/repositories.py`
- `tests/test_daemon_pg_doctor.py`
- `tests/test_mcp_capability_scope_e2e.py`
- `tests/architecture/test_legacy_sqlite_quarantine.py`

## Verification

- `pytest tests/test_daemon_pg_doctor.py tests/test_mcp_capability_scope_e2e.py tests/daemon_pg/test_repo_registration.py` -> 25 passed
- `pytest tests/architecture/test_legacy_sqlite_quarantine.py tests/architecture/test_authority_guardrails.py` -> 12 passed, 1 skipped
- `pytest tests/test_daemon_pg_sweep.py tests/cli/test_dispatch_daemon_doctor.py tests/cli/test_daemon_doctor_without_daemon.py` -> 21 passed
- `pytest tests/cli/test_daemon_sqlite_import_retired.py tests/cli/test_daemon_core.py` -> 20 passed
- `ruff check src/striatum/daemon_pg/client_admin.py src/striatum/daemon_pg/repositories.py src/striatum/daemon_pg/mcp_resources.py tests/test_daemon_pg_doctor.py tests/test_mcp_capability_scope_e2e.py` -> passed
- `ruff check tests/architecture/test_legacy_sqlite_quarantine.py` -> passed
- `git diff --check` -> passed

## Remaining Follow-Ups

The broader TODO 61 legacy test conversion remains open. The repo still has
many tests importing `striatum.legacy_sqlite`; converting those requires PG
harness rewrites across service, workflow, recovery, and web fixtures. I did
not restore the deleted legacy package, reopen SQLite import windows, or decide
the blocked TODO 55, 56, 59, or 60 product questions.
