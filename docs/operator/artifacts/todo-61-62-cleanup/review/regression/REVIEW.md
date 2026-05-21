# Regression-Risk Review: TODO 61-62 Cleanup

- **Workflow:** `todo-61-62-cleanup`
- **Reviewer:** Gemini
- **Date:** 2026-05-21
- **Posture:** `custom:regression_risk`
- **Verdict:** `accepted_with_follow_up`

## Summary

The cleanup for TODO 61 (RFC 0068) and TODO 62 (RFC 0069) is **partially complete**. Track 1 (Core Logic Fixes) has landed successfully, resolving the most visible "false-positive" warnings in the daemon doctor and safeguarding new repository registrations. However, Track 2 (Test Debt) and Track 3 (Corpus/Templates) have significant remaining work, and a structural inconsistency has been introduced in how repository state paths are projected.

## Findings

### F1: Structural Projection Divergence (High Risk)
There is a divergence in how `state_db_path` is projected to clients, which may break tools that rely on a consistent registry view.
- **Go Daemon & Python MCP:** Correctly project the operational scratch *directory* (`.striatum/`).
- **Python CLI/RPC (Internal):** Projects the raw `striatumd.repositories.state_db_path` column, which for older repositories still contains the *file* path (`.striatum/state.sqlite3`).
- **Daemon Doctor:** Derives the scratch path from `repo_root` and ignores the column entirely.
- **Impact:** Tools registered before the cutover see a file path; newly registered tools see a directory. This inconsistency should be resolved by normalizing the path in `src/striatum/daemon_pg/repositories.py` (similar to the logic already in `mcp_resources.py`).

### F2: Track 2 (Test Debt) Stalled (Medium Risk)
The implementation of Track 2 is largely missing from the current state.
- **Legacy Imports:** 69 imports of `striatum.legacy_sqlite` remain across 45 test files.
- **Architecture Guardrails:** `tests/architecture/test_legacy_sqlite_quarantine.py` has been updated with classification helpers but does not yet enforce the removal of `striatum.legacy_sqlite` imports in the `tests/` directory.
- **Service Fallback:** `src/striatum/service_command_policy.py` still contains the `STRIATUM_TEST_HARNESS=1` + `STRIATUM_DAEMON_REQUIRED=0` bypass, allowing tests to continue dodging daemon routing.
- **Impact:** The test suite remains coupled to retired SQLite modules, masking potential daemon-routing regressions in production-like paths.

### F3: Track 3 (Corpus/Templates) Stalled (Medium Risk)
- **Corpus Export:** `src/striatum/corpus/export.py` still hardcodes `legacy_sqlite_fixture` as the state authority and uses `PRAGMA user_version`. It has not been refactored to use daemon/PG metadata.
- **Impact:** Production corpus exports from PG-backed repositories will contain misleading or incorrect metadata.

### F4: Stale Documentation (Low Risk)
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` has not been updated to reflect the rename of `daemon_repo_state_missing` to `daemon_repo_scratch_missing`.

## Verification Results

### 1. Architecture Integrity
- `pytest tests/architecture/test_legacy_sqlite_quarantine.py`: **PASS** (but coverage is insufficient for `tests/` directory).
- `pytest tests/architecture/test_authority_guardrails.py`: **PASS**.

### 2. Functional Correctness
- `striatum daemon doctor --authority`: **PASS**. Verified on a repository with a "stale" `state_db_path` column; it correctly reports `ok: true` and finds no problems.
- `striatum repo list --json`: **DIVERGENT**. Returns a mix of file and directory paths.

### 3. Regression Suite
- `tests/test_daemon_pg_doctor.py`: **PASS**. New coverage for the doctor fix is robust.

## Recommendations

1.  **Immediate:** Move the normalization logic from `src/striatum/daemon_pg/mcp_resources.py` into `src/striatum/daemon_pg/repositories.py`'s `repo_list_pg` and `repo_resolve_pg` so all Python clients see a consistent directory-based path.
2.  **Follow-up:** Execute the Track 2 batch conversion of tests and enable the architecture guardrail for `tests/` imports.
3.  **Follow-up:** Refactor `src/striatum/corpus/export.py` to support PostgreSQL-native metadata.
4.  **Documentation:** Update the `COMMAND_AUTHORITY_MATRIX.md` with the new doctor check IDs.
