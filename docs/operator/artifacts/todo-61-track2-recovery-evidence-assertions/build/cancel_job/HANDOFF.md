# Recovery Cancel Job Assertion Handoff
author: implementer-codex-002

## Summary

Updated
`tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py` so the
`test_cancelable_states` assertion matches the current daemon/PostgreSQL
`recovery.cancel_job` contract.

The test now expects the active cancelable job-state set:

- `blocked`
- `queued`
- `claimed`
- `running`
- `stale_lease`
- `waiting_human`

I also changed the module docstring from SQLite parity language to current
daemon/PostgreSQL contract language. No production handler code was changed.

## Verification

- `pytest tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py -q`
  -> 4 passed.
- `ruff check tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py`
  -> passed.
- `pytest tests/daemon_pg/handlers/recovery_evidence -q`
  -> 41 passed, 1 skipped, 1 failed.

The remaining shard failure is outside this packet's write scope:

- `tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py::test_process_adapter_blocker_kinds`
  still asserts the stale process-adapter blocker kind set.
