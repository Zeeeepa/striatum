author: implementer-codex-gpt-5.5-001

# Build Handoff: RFC 0035 Multi-Repo Test Harness V1

Status: implemented
Date: 2026-05-12

## Shipped

Implemented the RFC 0035 V1 test-infrastructure slice:

- Added `tests/_harness/` with `MultiRepoHarness`, daemon process helpers,
  ephemeral Postgres setup/reset helpers, repo registration/local-run helpers,
  MCP client helpers, token helpers, audit-chain assertions, and scope helpers.
- Added `tests/conftest.py` with `postgres_url`, class-scoped
  `multi_repo_harness`, and function-scoped `clean_daemon_db` fixtures.
- Added the harness smoke test and the five requested e2e modules:
  `test_cross_repo_prepare_e2e.py`,
  `test_cross_repo_lifecycle_e2e.py`,
  `test_cross_repo_crash_recovery_e2e.py`,
  `test_mcp_capability_scope_e2e.py`, and
  `test_per_repo_write_scope_e2e.py`.
- Added pytest marker registration for `multi_repo` and a
  `make test-multi-repo` target.
- Updated `docs/TODO.md`, `docs/SPEC.md`, and `CHANGELOG.md` to reflect
  the landed harness as developer test infrastructure, not product API.

The harness uses the existing production seams: daemon PG migrations,
`CrossRepoLocalRunner` lifecycle helpers, workflow validation,
`DaemonRpcServer` MCP authorization/audit path, token capability vocabulary,
and daemon audit hash helpers. It does not introduce a public operator-facing
API or a parallel product implementation.

## Verification

Passed:

- `make install` (run as part of `make lint` / `make typecheck`)
- `make lint`
- `make typecheck`
- `make test` — 630 passed, 31 skipped
- `make test-multi-repo` — 31 skipped
- `make smoke`

The 31 skipped tests are the new `multi_repo` marker set. This environment
does not have a reachable configured system Postgres/daemon-pg setup, so the
skip path was exercised and reported clearly. The modules still import under
the full test run, and lint/typecheck cover the new harness code.

## Delegation

Used three explorer sub-agents:

- Cross-repo lifecycle and workflow-test seams.
- Daemon PG / capability / MCP / audit seams.
- Existing test fixture, Makefile, and pytest configuration patterns.

The parent session implemented and integrated all file changes, ran
verification, and authored this handoff.

## Deferred

- Go-client testing surface remains deferred to a future Go-core/client RFC.
- A two-repos-with-worktree-isolated-lanes example workflow remains deferred
  to an examples/onboarding follow-up.
- Docker-based or bundled ephemeral Postgres remains a separate hardening RFC.
- Windows daemon harness support remains out of scope for RFC 0035.
- Cross-machine, multi-tenant, and performance/load testing remain out of
  scope unless the product boundary changes.

## Notes

The current daemon foreground process can bind its runtime socket without the
Postgres URL; the harness separately owns the ephemeral Postgres database used
for lifecycle, MCP, and audit assertions. This matches the current source
reality where the full daemon socket accept loop still lags the PG-backed
Python seams.
