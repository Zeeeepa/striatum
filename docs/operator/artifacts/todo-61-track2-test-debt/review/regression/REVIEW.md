# Track 2 Regression Review
author: reviewer-gemini-001

## Verdict: PARTIAL SUCCESS (Medium Risk)

The first TODO 61 Track 2 batch successfully eradicates direct `striatum.legacy_sqlite` imports from the four targeted high-debt test files. However, it fails to meet the goal of unblocking the architecture guardrails, and it preserves a significant amount of test debt through granular skips rather than full conversion.

## Key Findings

### F1: Authority Guardrail Regression (Critical)
The broad module-level skip in `tests/architecture/test_authority_guardrails.py` was **not removed**, contradicting the `MAP.md` objective. Removing the skip reveals 3 failures:
- **Unclassified RPC Methods:** Eight new or existing methods (e.g., `artifact.backfill_blob`, `work.await_packet`) lack authority classification.
- **Stale Allowlist:** The `DIRECT_PG_BOOTSTRAP_IMPORT_ALLOWLIST` contains entries for the now-deleted `src/striatum/legacy_sqlite` package.
- **Matrix Drift:** The `COMMAND_AUTHORITY_MATRIX.md` is out of sync with the current RPC registry.

### F2: High Residual Test Debt (Medium)
While module-level skips were removed from the four target files, they were replaced by a high density of granular skips:
- `tests/test_cli_mvp.py`: Over 100 tests are skipped because they depend on the stubbed `init_repo` or `connect` helpers.
- **Impact:** The "conversion" is primarily a quarantine of legacy dependencies rather than a full port of coverage to the PostgreSQL/daemon harness. While this satisfies the requirement to remove legacy imports, it leaves a large portion of the CLI MVP surface unverified in the new architecture.

### F3: Blocked Active Coverage (Low)
`tests/daemon_pg/handlers/recovery_evidence/conftest.py` contains a broad module-level skip that blocks valid, PostgreSQL-only tests like `test_process_reconcile.py`. These tests pass when the skip is removed, as they do not actually utilize the broken legacy fixtures in the `conftest.py`.

## Verification Results

| Target | Status | Notes |
|---|---|---|
| `tests/test_artifact_schemas.py` | **PASS** | Skips removed; pure parser tests are active. |
| `tests/test_process_adapter.py` | **PASS** | Skips removed; retired-path assertions are active. |
| `tests/test_service.py` | **PASS** | Skips removed; daemon DTO/route tests are active. |
| `tests/test_cli_mvp.py` | **PASS** | Skips removed; pure validation/graph tests are active. 100+ skips remain. |
| `tests/architecture/test_legacy_sqlite_quarantine.py` | **PASS** | Guardrail is active and correctly identifies residual imports. |
| `tests/architecture/test_authority_guardrails.py` | **FAIL** | Still skipped; fails when unblocked. |
| `tests/daemon_pg/handlers/workflow_loop/test_publish_artifact.py` | **PASS** | New active coverage verified. |
| `tests/daemon_pg/handlers/test_supervision.py` | **PASS** | New active coverage verified. |

## Recommendations

1.  **Unblock Authority Guardrail:** Update `tests/architecture/test_authority_guardrails.py` to classify the 8 missing methods, remove the stale `legacy_sqlite` allowlist entries, and remove the module-level skip.
2.  **Narrow Recovery conftest Skip:** Move the `pytest.skip` in `tests/daemon_pg/handlers/recovery_evidence/conftest.py` into the specific `sqlite_conn` and `parity_seed` fixtures to unblock active PostgreSQL tests in that directory.
3.  **Audit CLI MVP Skips:** Prioritize the 100+ skips in `test_cli_mvp.py` for conversion in the next Track 2 batch, focusing on the most critical workflow lifecycle and recovery paths.
