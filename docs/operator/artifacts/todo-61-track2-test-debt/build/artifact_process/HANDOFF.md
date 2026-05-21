# Artifact And Process Test Debt Handoff
author: implementer-codex-001

## Summary

Converted the artifact/process batch away from module-level legacy SQLite skips.
Both target files now import without `striatum.legacy_sqlite`, and current
schema/helper coverage runs normally.

In `tests/test_artifact_schemas.py`, current front-matter parser and workflow
validator assertions now run directly without repo-local state. Historical
publish-flow assertions that still depend on the retired repo-local SQLite
fixture are quarantined per test instead of hiding the whole module.

In `tests/test_process_adapter.py`, the current fail-closed direct adapter-run
assertion, workflow validation timeout checks, diagnostic-envelope check, and
lane environment expansion checks now run. Historical SQLite adapter-run and
migration fixtures are quarantined per test.

## Verification

- `pytest tests/test_artifact_schemas.py tests/test_process_adapter.py` ->
  22 passed, 35 skipped.
- `ruff check tests/test_artifact_schemas.py tests/test_process_adapter.py` ->
  passed.

## Residual Work

The skipped tests are intentionally not restored here because they model the
retired repo-local SQLite runner path. A later batch can replace the remaining
publish-flow and process-reconcile integration cases with daemon/PostgreSQL
harness fixtures if that coverage is still needed.
