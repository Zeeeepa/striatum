# CLI MVP Legacy SQLite Import Handoff
author: implementer-codex-003

## Scope

Converted `tests/test_cli_mvp.py` away from direct imports of deleted legacy
SQLite modules while staying inside the packet write scope.

## Changes

- Removed the module-level `pytest.skip("legacy sqlite eradicated", ...)` so
  current non-SQLite tests in the file execute again.
- Removed direct imports of `striatum.legacy_sqlite.db`.
- Added narrow helper-level quarantine for tests that still call retired
  repo-local SQLite setup or introspection (`init_repo`, `connect`, `db_path`).
  Those tests now skip at the point they request retired state instead of
  hiding the whole module.
- Left current CLI/workflow authoring tests visible. Focused run result:
  `17 passed, 75 skipped`.

## Verification

- `pytest tests/test_cli_mvp.py -q`
- `ruff check tests/test_cli_mvp.py`
- `rg -n "from striatum\\.legacy_sqlite|import striatum\\.legacy_sqlite|from striatum\\.db|import striatum\\.db|from striatum\\.migrations|import striatum\\.migrations" tests/test_cli_mvp.py`
  returned no matches.

## Notes

The working tree already contained unrelated edits before this packet,
including removal of the retired local MCP wrapper module and corresponding
tests from `tests/test_cli_mvp.py`. I did not revert or expand those changes.

The remaining skipped tests are historical broad CLI lifecycle assertions that
still depend on repo-local SQLite internals. They should be converted in later
focused batches to daemon/PostgreSQL handler or CLI-dispatch coverage instead
of restoring production SQLite authority.
