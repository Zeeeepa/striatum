# Authority Guardrail Unblock Handoff
author: implementer-codex-001

## Summary

Removed the module-level legacy-SQLite skip from
`tests/architecture/test_authority_guardrails.py`, so the command authority
guardrails now run.

The unblocked guardrail exposed the expected drift:

- eight active Go-only daemon methods were missing authority classification:
  `artifact.backfill_blob`, `artifact.get_content`, `artifact.list_for_run`,
  `corpus.fetch_historical_dogfood_file`,
  `corpus.list_historical_dogfood_files`,
  `corpus.list_historical_dogfoods`,
  `corpus.migrate_historical_dogfood_file`, and `work.await_packet`;
- the direct-PostgreSQL bootstrap allowlist still contained the deleted
  `src/striatum/legacy_sqlite/repo_local_migration.py` entry;
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` lacked rows for those
  active registry methods.

I added a narrow `GO_ONLY_DAEMON_METHODS` classification for the current
Go-only contract methods, removed the stale deleted-package allowlist entry,
and added matrix rows for the blob/historical-dogfood RPCs plus
`work.await_packet`. No TODO 55, 56, 59, or 60 decision was made.

## Verification

- `pytest tests/architecture/test_authority_guardrails.py -q` -> 23 passed.
- `pytest tests/architecture/test_legacy_sqlite_quarantine.py tests/architecture/test_authority_guardrails.py -q` -> 37 passed.
- `ruff check tests/architecture/test_authority_guardrails.py` -> passed.

## Residual Work

The authority guardrail is active again. The added Go-only classification is
intentionally narrow; future daemon methods still need an explicit matrix row
and classification when they are added.
