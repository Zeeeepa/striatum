# Regression-Risk Review: Go Repo Projection

- **Workflow:** `todo-61-62-go-repo-projection-revision`
- **Reviewer:** Gemini
- **Date:** 2026-05-21
- **Posture:** `custom:regression_risk`
- **Verdict:** `accepted`

author: reviewer-gemini-001

## Summary

The regression fix for repository state-path projections has been successfully verified across both Python and Go daemon implementations. The structural divergence identified in previous reviews (F1) is resolved: daemon-backed `repo.list`, `repo.resolve`, and `repo.add` (for existing registrations) now consistently normalize stale `.striatum/state.sqlite3` file paths to the `.striatum/` operational scratch directory. This normalization ensures a consistent registry view for all clients (CLI, RPC, and MCP) while preserving the underlying database integrity.

## Findings

### F1: Consistent Projection across Python and Go (Resolved)
The normalization logic is now correctly implemented in both primary daemon cores:
- **Python Fix:** Centralized in `_repository_projection` within `src/striatum/daemon_pg/repositories.py`. It is applied to `repo_list_pg`, `repo_resolve_pg`, and `repo_add_pg` (duplicate identity path).
- **Go Fix:** Implemented in `projectedStateDBPath` within `go/pkg/repositories/service.go`. It is applied via `publicRepository` in `Add`, `List`, and `Resolve` handlers.
- **MCP Resources:** Python's `src/striatum/daemon_pg/mcp_resources.py` also includes the normalization logic in its `_repository_rows` helper, ensuring `striatum://daemon/repos` and dashboard resources are consistent.

### F2: Robust Test Coverage (Verified)
Focused tests have been added and verified in both languages:
- **Python Tests:** `test_repo_list_and_resolve_normalize_stale_state_sqlite_projection` in `tests/daemon_pg/test_repo_registration.py` successfully verifies that stale SQLite paths are normalized and that the underlying PostgreSQL row remains unchanged.
- **Go Tests:** `TestListNormalizesStaleStateSQLiteProjection`, `TestResolveNormalizesStaleStateSQLiteProjection`, and `TestAddAlreadyRegisteredNormalizesStaleStateSQLiteProjection` in `go/pkg/repositories/service_test.go` provide equivalent coverage for the Go implementation.
- **Execution:** All focused tests pass, along with broader e2e tests (`test_mcp_capability_scope_e2e.py`).

### F3: Architectural Integrity
- **Immutability:** Both implementations correctly perform output-only normalization. The `striatumd.repositories.state_db_path` column in PostgreSQL is not modified, preserving historical registry data.
- **SQLite Tripwire:** Verification was performed with `STRIATUM_SQLITE_CONNECT_TRIPWIRE=1`, confirming that no unexpected SQLite connections are made during repository listing or resolution.

## Verification Results

### 1. Python Functional Correctness
- `pytest tests/daemon_pg/test_repo_registration.py`: **PASS** (7 passed)
- `pytest tests/test_mcp_capability_scope_e2e.py`: **PASS**

### 2. Go Functional Correctness
- `cd go && go test -v ./pkg/repositories/...`: **PASS** (9 passed)

## Recommendation

`accepted`. The fix is complete, correctly implemented in both language stacks, and well-tested. No further regressions were identified in the projection paths.
