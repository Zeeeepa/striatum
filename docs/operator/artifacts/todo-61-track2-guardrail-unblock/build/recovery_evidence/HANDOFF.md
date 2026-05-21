# Recovery Evidence Skip Handoff
author: implementer-codex-002

## Summary

Narrowed the legacy SQLite skip in
`tests/daemon_pg/handlers/recovery_evidence/conftest.py`.

The file no longer performs a module-level
`pytest.skip(..., allow_module_level=True)`, so PostgreSQL-only recovery
evidence tests collect and run. The historical repo-local SQLite dependency is
now quarantined only inside the `sqlite_conn` fixture; tests must explicitly
request that fixture to receive the skip.

I also updated the conftest docstring so it no longer claims the whole
recovery-evidence parity rig is live SQLite/PG parity.

## Verification

- `pytest tests/daemon_pg/handlers/recovery_evidence/test_process_reconcile.py -q`
  -> 4 passed.
- `ruff check tests/daemon_pg/handlers/recovery_evidence/conftest.py`
  -> passed.
- `pytest tests/architecture/test_legacy_sqlite_quarantine.py -q`
  -> 14 passed.
- `pytest tests/daemon_pg/handlers/recovery_evidence -q`
  -> 40 passed, 1 skipped, 2 failed.
- `pytest tests/daemon_pg/handlers/recovery_evidence -q -k 'not cancelable_states and not process_adapter_blocker_kinds'`
  -> 40 passed, 1 skipped, 2 deselected.

The full recovery-evidence shard now runs far enough to expose two existing
stale assertion tests outside this packet's write scope:

- `tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py::test_cancelable_states`
  expects only `pending`, `claimed`, and `blocked`, but the current handler
  also includes `queued`, `running`, `stale_lease`, and `waiting_human`.
- `tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py::test_process_adapter_blocker_kinds`
  expects the older process blocker kind set, but the current handler exposes
  the expanded post-cutover set including nonzero exit, timeout, missing
  outputs, missing verdict, and lost-process variants.
