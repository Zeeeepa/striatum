# Regression-Risk Review: TODO 61-62 Cleanup Revision

- **Workflow:** `todo-61-62-cleanup-revision`
- **Reviewer:** Gemini
- **Date:** 2026-05-21
- **Posture:** `custom:regression_risk`
- **Verdict:** `accepted`

author: reviewer-gemini-001

## Summary

The revision for TODO 61-62 cleanup successfully addresses the structural projection divergence identified in the previous review (F1). The fix centralizes path normalization for repository state paths, ensuring consistent directory-based projection across Python CLI, RPC, and MCP clients without modifying the underlying database storage or reopening legacy SQLite production authority.

## Findings

### F1: Repo State-Path Projection Divergence Fixed (Resolved)
The divergence between how repository state paths were projected has been resolved.
- **Centralized Normalization:** Logic from `src/striatum/daemon_pg/mcp_resources.py` has been moved and centralized into a `_repository_projection` helper in `src/striatum/daemon_pg/repositories.py`.
- **Consistent Projection:** Any repository row with a `state_db_path` pointing to the legacy `state.sqlite3` file within the operational scratch directory is now projected as the scratch directory path itself.
- **Coverage:** This normalization is applied to `repo_list_pg`, `repo_resolve_pg`, and `repo_add_pg` (for existing identities), ensuring all Python-based clients see a consistent view.
- **Verification:** Verified by `test_repo_list_and_resolve_normalize_stale_state_sqlite_projection` in `tests/daemon_pg/test_repo_registration.py` and `test_pg_daemon_mcp_resource_reads_preserve_shapes_without_sqlite` in `tests/test_mcp_capability_scope_e2e.py`. Both tests confirm that the database remains unchanged while the projected output is normalized.

### F2: Architecture Boundaries Maintained
- **SQLite Authority:** The fix does not reopen SQLite production authority. `src/striatum/daemon_pg/client_admin.py` remains PostgreSQL-only, and `src/striatum/daemon_pg/repositories.py` continues to guard against existing SQLite state during registration.
- **Tripwire Validation:** Tests use `STRIATUM_SQLITE_CONNECT_TRIPWIRE=1` to ensure no unexpected SQLite connections are made during repository listing or MCP resource reading.
- **Service Policy:** `src/striatum/service_command_policy.py` has been further restricted, now requiring `STRIATUM_LEGACY_SERVICE_FIXTURE=1` for the legacy test-harness escape.

### F3: Scope Discipline
- **Track 2/3 and TODOs:** In accordance with the revision instructions, no implementation work was performed on Track 2 (Test Debt), Track 3 (Corpus/Templates), or blocked TODO items (55, 56, 59, 60). These remain as documented residuals for future work.

## Verification Results

### 1. Functional Correctness
- `repo_list_pg` / `repo_resolve_pg`: **PASS**. Normalization confirmed via unit tests.
- `daemon_mcp_resources_pg`: **PASS**. Resource URI and payload normalization confirmed via e2e tests.

### 2. Architecture Integrity
- `pytest tests/architecture/test_legacy_sqlite_quarantine.py`: **PASS**. Production sources remain free of unclassified SQLite imports.
- `pytest tests/daemon_pg/test_repo_registration.py`: **PASS**.
- `pytest tests/test_mcp_capability_scope_e2e.py`: **PASS**.

## Suggested Disposition

`accepted`. The specific regression risk identified in the previous round has been mitigated with a surgical, well-tested fix that respects existing architectural boundaries.
