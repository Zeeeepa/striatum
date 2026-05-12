author: implementer-codex-gpt-5.5-001

# Build Handoff: RFC 0032 Cross-Repo + MCP Mutation V2 Slice

Status: implemented
Date: 2026-05-12

## Shipped

Implemented the accepted dogfood-035 V2 slice without adding the deferred
multi-repo daemon end-to-end harness.

- Added cross-repo workflow validation in `src/striatum/workflow.py`:
  `repositories`, required `primary_repository`, required per-job
  `repository`, repository-qualified artifact/write-scope collision checks,
  `cross_repo_artifact_augmented`, `cross_repo_cycle: true`, and
  `parallelism.per_repo_max_active_jobs`.
- Added repo-local SQLite migration v14 and fresh-schema support for
  nullable `runs.cross_repo_run_id`.
- Added daemon PostgreSQL migration v3 for `cross_repo_runs`,
  `cross_repo_run_repositories`, `cross_repo_cycle_counters`, and
  `audit_repositories`, plus method metadata support for
  `repository_scope_mode` and `recovery`.
- Added `src/striatum/cross_repo.py`, a mock-friendly daemon lifecycle
  helper for prepare/start/cancel/describe/list/reconcile.
- Extended daemon RPC registry/capability code with `recovery`,
  repository scope modes, cross-repo route declarations, recovery route
  declarations, and repo-scope mismatch denial.
- Extended daemon MCP behavior so PostgreSQL-backed daemon MCP can filter
  `tools/list`, re-authorize `tools/call`, and append metadata-only audit
  and request-log rows for allowed and denied calls.
- Added thin `cross-repo list|describe|why|cancel` parser/dispatch
  surfaces. `cancel` remains blocked behind the daemon lifecycle service
  rather than pretending the full harness exists.
- Updated README, SPEC, MCP, UBIQUITOUS_LANGUAGE, CLI_REFERENCE,
  HOW_TO_HUMAN, RFC 0032, and CHANGELOG.

## Tests

Added focused tests:

- `tests/test_workflow_cross_repo.py`
- `tests/test_cross_repo_lifecycle.py`
- `tests/test_mcp_mutation_capabilities.py`
- `tests/test_daemon_rpc_registry.py`

Updated daemon migration/RPC tests for daemon DB v3 and repo-local v14.

Targeted verification run:

```text
pytest tests/test_workflow_cross_repo.py tests/test_cross_repo_lifecycle.py tests/test_mcp_mutation_capabilities.py tests/test_daemon_rpc_registry.py tests/test_daemon_pg.py tests/test_daemon_rpc.py
```

Result: 34 passed.

## Deferred

Per the work packet and `docs/TODO.md` Open item 19, the following remain
deferred to the multi-repo test harness RFC:

- real two-or-more-repo daemon end-to-end tests;
- live cross-repo edge progression through the scheduler;
- daemon restart during actual multi-repo `preparing`;
- real artifact publication into separate participant worktrees;
- cross-repo cycle accounting through live daemon coordination;
- cross-platform path identity verification.

## Delegation

Used three native explorer sub-agents:

- workflow validator integration points and test plan;
- daemon RPC/MCP capability and audit integration points;
- daemon DB/lifecycle migration and mocked lifecycle test plan.
