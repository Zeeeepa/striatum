# Service Test SQLite Fixture Cleanup Handoff
author: implementer-codex-002

## Summary

Converted `tests/test_service.py` from a broad module-level legacy SQLite
quarantine to active daemon-routed unit coverage plus explicit per-fixture
quarantine skips.

The file no longer imports `striatum.legacy_sqlite`, `striatum.db`, or
`striatum.migrations`. Tests that only needed a SQLite tripwire now patch
stdlib `sqlite3.connect` directly, so daemon-routed service handlers still
fail if they attempt any SQLite open without depending on the retired package.

Historical subprocess service cases that still require the retired
repo-local SQLite fixture are now skipped with a shared reason:
`historical repo-local SQLite service fixture quarantined after Go/PG cutover`.
This keeps those cases named without hiding the daemon-routed service tests.

## Verification

- `pytest tests/test_service.py -q` -> 25 passed, 19 skipped.
- `ruff check tests/test_service.py` -> passed.
- `git diff --check` -> passed.

## Notes

No production service fallback policy, architecture guardrail, or other test
file was changed in this slice.
