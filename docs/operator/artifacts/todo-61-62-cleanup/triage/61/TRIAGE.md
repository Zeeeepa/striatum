---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["todo_61", "docs/rfcs/0068-go-production-daemon-port.md", "docs/operator/plans/rfc-0068-go-daemon-port.md", "docs/architecture/COMMAND_AUTHORITY_MATRIX.md"]
---

# TODO 61 Residual Cleanup Triage
author: triager-codex-001

## Summary

TODO 61 is no longer about Go parity for active daemon RPC methods. The live
product boundary has moved: `striatum daemon start` launches Go, the Python
daemon module is deleted, retired SQLite import commands refuse before opening
SQLite, and removed dogfood/reviewed-apply RPC names audit as
`method_unknown`.

The remaining cleanup is test and compatibility debt. The implementer should
avoid adding new product behavior unless a decision explicitly accepts it.
PostgreSQL-native operator composites are still an open product question, not
an implementation gap to fill opportunistically.

## Highest Priority Cleanup

1. Convert or delete direct legacy SQLite tests.

   Evidence: `rg -n "from striatum\\.legacy_sqlite|import striatum\\.legacy_sqlite" tests -g '!**/__pycache__/**'` currently finds 69 imports across 45 test files. The current source tree has no `src/striatum/legacy_sqlite/` package, and `python -c 'import importlib.util; print(importlib.util.find_spec("striatum.legacy_sqlite"))'` returns `None`.

   Start with the broad workflow/live-state fixture tests:
   `tests/test_cli_mvp.py`, `tests/test_service.py`, `tests/test_artifact_schemas.py`,
   `tests/test_process_adapter.py`, `tests/test_supervise.py`,
   `tests/test_session_close.py`, `tests/test_worktree_isolation.py`,
   `tests/test_recovery_extended.py`, `tests/test_recovery_resume.py`,
   `tests/test_pause_resume.py`, `tests/test_cli_run_cancel.py`,
   `tests/test_retry_job.py`, and the web detail/page tests that build
   SQLite-backed rows.

   Desired end state: tests that assert current production behavior should use
   daemon/PostgreSQL harnesses under `tests/_harness/pg.py`,
   `tests/_harness/daemon.py`, and `tests/_harness/repos.py`. Tests that only
   preserve historical importer behavior should move under a narrow fixture
   path and require an explicit legacy import escape.

2. Remove stale compatibility assumptions from architecture tests.

   Evidence: `tests/architecture/test_legacy_sqlite_quarantine.py` now has
   `PRODUCTION_SQLITE_QUARANTINE = {}` and asserts the Python daemon is
   deleted, but it still expects tests to import legacy SQLite modules
   directly and treats those as classified fixtures. That made sense while the
   compatibility package existed; it is now masking broken imports rather than
   documenting a supported fixture package.

   Update this test to enforce the next boundary: no production imports of
   `striatum.legacy_sqlite`, no root `striatum.db` / `striatum.migrations`
   facades, and either no test imports or only imports in a deliberately named
   retained fixture directory.

3. Finish the corpus/export fixture split.

   Evidence: `src/striatum/corpus/export.py` still accepts an opaque `conn`,
   reads `PRAGMA user_version`, and writes manifest authority as
   `legacy_sqlite_fixture`. That is acceptable only for a legacy direct
   exporter, not for current production corpus export. `docs/DECISION_LOG.md`
   D116 says PostgreSQL corpus export should read daemon/repository metadata
   directly and the legacy direct exporter remains quarantined until deleted or
   rerouted.

   Implementer path: keep production coverage on the daemon/PostgreSQL corpus
   handler, then either move this direct exporter into an explicitly historical
   fixture module or delete it with the tests that depend on it
   (`tests/test_cli_corpus_export.py`, `tests/test_corpus_export_integration.py`,
   daemon PG corpus/archive handler tests).

4. Retire service test-harness fallback after web/API parity tests move to
   daemon RPC.

   Evidence: `src/striatum/service.py` still documents legacy SQLite
   compatibility for explicit subprocess test-harness paths, and
   `src/striatum/service_command_policy.py` retains
   `STRIATUM_TEST_HARNESS=1`, `STRIATUM_DAEMON_REQUIRED=0`, and
   `STRIATUM_LEGACY_SERVICE_FIXTURE=1` as the only path that bypasses daemon
   routing. Production routing is correct; the cleanup is to eliminate tests
   that still need the bypass.

   Target files: `tests/test_service.py`, `tests/test_chat_tools.py`,
   `tests/test_web_ui.py`, `tests/test_workflow_generation_web.py`, and web
   action/detail tests that still create legacy state.

5. Keep operator composites blocked pending product decision.

   Evidence: D110 removed `dogfood.publish_on_behalf` and
   `dogfood.surgical_recovery` from the production daemon contract, and
   RFC 0068 records stale calls as `method_unknown`. The open question in
   `docs/operator/plans/rfc-0068-go-daemon-port.md` is whether new
   PostgreSQL-native composites should exist at all.

   Implementation rule: do not reintroduce these methods, CLI aliases, MCP
   tools, or a hidden service shortcut while cleaning tests. Existing coverage
   such as `tests/test_mcp_dogfood_e2e.py` and
   `tests/test_mcp_mutation_capabilities.py` should continue to assert absence
   and `method_unknown`.

## Lower Priority But Worth Doing

- Remove stale `.pyc` and deleted-module references from diagnostics if they
  appear in local worktrees. They are not tracked behavior, but they confuse
  `find`/manual inspection around deleted modules.
- Sweep comments that say "PG port of `striatum.db.*`" in
  `src/striatum/daemon_pg/handlers/recovery_evidence/_sql.py` once all
  legacy fixtures are gone. They are historical implementation notes, not
  production dependencies.
- Reconcile `src/striatum/skills/context.py` and skill templates that say
  "the CLI is the only client" with the current native daemon MCP direction.
  This is adjacent to TODO 61 because it can preserve stale operator
  assumptions, but the primary MCP wording cleanup belongs with the active
  CLI-retirement/native-MCP track.

## Suggested Verification Commands

Use these after each cleanup slice:

```bash
rg -n "from striatum\\.legacy_sqlite|import striatum\\.legacy_sqlite|striatum\\.db|striatum\\.migrations" src tests -g '!**/__pycache__/**'
rg -n "sqlite3|state\\.sqlite3|daemon-registry\\.sqlite3|striatumd\\.sqlite3" src/striatum tests -g '!**/__pycache__/**'
pytest tests/architecture/test_legacy_sqlite_quarantine.py
pytest tests/cli/test_daemon_sqlite_import_retired.py tests/cli/test_daemon_core.py
pytest tests/test_mcp_dogfood_e2e.py tests/test_mcp_mutation_capabilities.py
make daemon-go-conformance
make test
```

## Non-Goals For This Cleanup

- Do not restore `src/striatum/daemon.py`, `src/striatum/db.py`,
  `src/striatum/migrations.py`, or `src/striatum/legacy_sqlite/` just to make
  old tests pass.
- Do not reopen `striatum daemon migrate` or
  `striatum daemon migrate-repo-local`; `src/striatum/cli/daemon.py` correctly
  refuses them before importing SQLite code.
- Do not add PostgreSQL-native publish-on-behalf or surgical-recovery
  composites without a new accepted decision and contract update.
