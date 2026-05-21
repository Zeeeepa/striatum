# Guardrail Unblock Regression Review
author: reviewer-gemini-001

## Verdict: SUCCESS

The `todo-61-track2-guardrail-unblock` batch successfully unblocks the architecture guardrails and recovery-evidence active tests, fulfilling the primary objectives from the `MAP.md` and addressing the regressions identified in the previous Track 2 batch.

## Key Findings

### F1: Authority Guardrails Unblocked (Pass)
The module-level skip in `tests/architecture/test_authority_guardrails.py` has been removed. The following remediations were verified:
- **RPC Classification:** Eight Go-only daemon methods (e.g., `artifact.backfill_blob`, `work.await_packet`) are now explicitly classified in `GO_ONLY_DAEMON_METHODS` within the test file, closing the unclassified-method gap.
- **Allowlist Cleanup:** The `DIRECT_PG_BOOTSTRAP_IMPORT_ALLOWLIST` no longer contains the stale `legacy_sqlite` entry.
- **Matrix Alignment:** `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` has been updated with the new RPC methods and classifications, maintaining sync with the RPC registry.

### F2: Recovery Evidence Coverage Unblocked (Pass)
The broad module-level skip in `tests/daemon_pg/handlers/recovery_evidence/conftest.py` has been narrowed to the `sqlite_conn` fixture.
- **Active Coverage:** PostgreSQL-only tests in the recovery-evidence directory now run.
- **Focused Success:** `tests/daemon_pg/handlers/recovery_evidence/test_process_reconcile.py` passes (4 tests).
- **Failure Visibility:** Two legitimate failures are now correctly exposed in the shard (`test_cancel_job.py` and `test_resume_blocker.py`) due to stale assertions against current handler logic. These are reported as existing debt rather than regressions in this batch.

## Verification Results

| Target | Status | Notes |
|---|---|---|
| `tests/architecture/test_authority_guardrails.py` | **PASS** | 23 passed. |
| `tests/architecture/test_legacy_sqlite_quarantine.py` | **PASS** | 14 passed. |
| `tests/daemon_pg/handlers/recovery_evidence/test_process_reconcile.py` | **PASS** | 4 passed. |
| `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` | **PASS** | Updated with 8 new methods. |
| `tests/daemon_pg/handlers/recovery_evidence` | **FAIL** | 40 passed, 2 failed (exposed stale assertions). |

## Recommendations

1.  **Resolve Recovery Assertion Debt:** Prioritize fixing the exposed stale assertions in `test_cancel_job.py` and `test_resume_blocker.py` in a follow-up Track 2 or cleanup batch.
2.  **Maintain Matrix Sync:** Ensure that any future RPC additions continue to follow the new classification pattern in both the matrix and the guardrail.
