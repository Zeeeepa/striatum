# Track 2 Guardrail Handoff
author: implementer-codex-004

## Summary

Updated `tests/architecture/test_legacy_sqlite_quarantine.py` so the completed
first Track 2 test-debt batch stays enforced.

The guardrail now names the converted batch files:

- `tests/test_artifact_schemas.py`
- `tests/test_cli_mvp.py`
- `tests/test_process_adapter.py`
- `tests/test_service.py`

Those files now fail the architecture test if they reintroduce imports of
`striatum.legacy_sqlite`, `striatum.db`, or `striatum.migrations`, or if they
restore a broad module-level `pytest.skip(..., allow_module_level=True)` to
hide deleted legacy SQLite dependencies.

Residual legacy SQLite fixture imports outside this batch are recorded in an
explicit future-batch allowlist instead of being silently covered by a broad
`tests/` quarantine. The stdlib `sqlite3` reference guardrail was narrowed the
same way.

## Verification

- `pytest tests/architecture/test_legacy_sqlite_quarantine.py -q` -> 14 passed.
- `ruff check tests/architecture/test_legacy_sqlite_quarantine.py` -> passed.

## Residual Work

The residual allowlist documents future Track 2 batches. It still includes
legacy SQLite imports in test areas such as recovery evidence fixtures,
RFC 0043 split-brain exit-code coverage, corpus/export tests, dashboard/web
tests, skill install tests, supervision tests, and worktree isolation tests.
Those remain visible as cleanup work rather than blanket-approved test debt.
