# Recovery Evidence Assertion Regression Review
author: reviewer-gemini-001

## Verdict: accept

## Summary

Verified that the two stale recovery-evidence assertion failures reported in
`todo-61-track2-guardrail-unblock` are resolved. The recovery-evidence test
shard now passes (42 passed, 1 skipped) and is no longer hidden by legacy
SQLite-parity skips.

## Findings

### 1. Recovery Cancel Job Assertions Resolved
The `test_cancelable_states` test in
`tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py` now correctly
asserts the expanded PostgreSQL cancelable-state set:
- `blocked`, `queued`, `claimed`, `running`, `stale_lease`, `waiting_human`.

This matches the production constant in
`src/striatum/daemon_pg/handlers/recovery_evidence/cancel_job.py`.

### 2. Recovery Resume Blocker Assertions Resolved
The `test_process_adapter_blocker_kinds` test in
`tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py` now asserts
the current PostgreSQL process-adapter blocker-kind contract:
- `process_outputs_missing`, `process_review_verdict_missing`,
  `process_exit_nonzero`, `process_timeout_exceeded`,
  `process_lost_with_outputs_missing`.

This matches the production constant in
`src/striatum/daemon_pg/handlers/recovery_evidence/resume_blocker.py`.

### 3. Active PostgreSQL Coverage Unblocked
Verified that `tests/daemon_pg/handlers/recovery_evidence/conftest.py` no
longer performs a module-level skip. The shard now collects and runs
PostgreSQL-backed integration tests (e.g., in `test_auto_finalize.py` and
`test_sweep.py`).

The single remaining skip (`test_evidence_export.py`) is appropriate as it covers
superseded legacy logic.

## Verification Results

- `pytest tests/daemon_pg/handlers/recovery_evidence -q`
  -> 42 passed, 1 skipped.
- Manual inspection of production and test constants confirmed exact parity.
