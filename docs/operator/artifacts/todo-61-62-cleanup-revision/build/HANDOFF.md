# State Path Projection Revision Handoff
author: implementer-codex-001

## Summary

Fixed regression-review F1 for Python repository projections:

- `repo_list_pg` and `repo_resolve_pg` now serialize repository rows through a
  shared projection helper in `src/striatum/daemon_pg/repositories.py`.
- The helper normalizes stale `state_db_path` values ending in
  `.striatum/state.sqlite3` to the `.striatum/` operational scratch directory.
- The stored PostgreSQL row is not rewritten; this is output normalization only,
  so migration history and older registry rows remain intact.
- The duplicate-registration return path also uses the same projection helper,
  preventing the same stale value from leaking through `repo_add_pg` when a
  repository is already registered.

No command authority matrix update was needed because no RPC method,
capability, fallback classification, or user-visible check id changed.

## Files Changed

- `src/striatum/daemon_pg/repositories.py`
- `tests/daemon_pg/test_repo_registration.py`

## Verification

- `pytest tests/daemon_pg/test_repo_registration.py` -> 7 passed
- `pytest tests/test_mcp_capability_scope_e2e.py tests/test_daemon_pg_doctor.py` -> 19 passed
- `ruff check src/striatum/daemon_pg/repositories.py tests/daemon_pg/test_repo_registration.py` -> passed
- `git diff --check` -> passed

## Notes

The repository worktree had pre-existing unrelated edits across docs, Go daemon
code, daemon MCP/runtime files, and tests. This revision only changed the files
listed above and left the broader TODO 61/62 Track 2 and Track 3 follow-ups
untouched.
