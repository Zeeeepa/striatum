# v1 repo-local SQLite fixture

`state.sqlite3` is a deterministic repo-local workflow-state fixture at the
current SQLite `PRAGMA user_version`.

Regenerate it from the repository root with:

```bash
PYTHONPATH=src python tests/fixtures/v1_repo_local_sqlite/build_fixture.py
```
