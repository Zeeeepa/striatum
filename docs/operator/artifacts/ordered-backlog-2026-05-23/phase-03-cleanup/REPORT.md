---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["tests/test_skills_install.py", "tests/architecture/test_legacy_sqlite_quarantine.py", "docs/TODO.md"]
---

# Phase 3 Legacy Cleanup
author: operator [self-declared: codex-driver]

## Result

Pruned one current skipped test fixture module from the residual legacy SQLite
surface.

Changed:

- Removed the broad `pytest.skip(..., allow_module_level=True)` from
  `tests/test_skills_install.py`.
- Replaced five legacy SQLite doctor checks with direct skill manifest and
  `skill_files_present` assertions.
- Removed `tests/test_skills_install.py` from
  `RESIDUAL_LEGACY_SQLITE_TEST_IMPORTS`.

This keeps historical fixtures untouched and does not restore
`striatum.legacy_sqlite`, `striatum.db`, repo-local SQLite, or the retired
daemon registry.

## Validation

```bash
.venv/bin/python -m pytest -q tests/test_skills_install.py tests/architecture/test_legacy_sqlite_quarantine.py
.venv/bin/python -m ruff check tests/test_skills_install.py tests/architecture/test_legacy_sqlite_quarantine.py
rg -n "from striatum\\.legacy_sqlite|import striatum\\.legacy_sqlite|allow_module_level=True" tests/test_skills_install.py
```

Pytest and ruff passed. The `rg` check returned no matches.

## Remaining Cleanup

The next safe cleanup batch is still test-fixture scoped: convert another
single skipped module or a small cluster with pure validation tests first.
The higher-risk TODO 63 direct-PG bootstrap/client boundary remains a separate
contract decision, not part of this slice.
