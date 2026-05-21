---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/workflows/todo-61-track2-test-debt/prompts/map_track2_imports.md", "docs/operator/artifacts/todo-61-62-cleanup/final/SUMMARY.md"]
---

# Track 2 Test Debt Import Map
author: triager-codex-001

## Scope

This maps the first TODO 61 Track 2 batch only:

- `tests/test_cli_mvp.py`
- `tests/test_service.py`
- `tests/test_artifact_schemas.py`
- `tests/test_process_adapter.py`
- architecture guardrail: `tests/architecture/test_legacy_sqlite_quarantine.py`

The four batch files are currently hidden by identical module-level skips:
`import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)`.
Those skips are masking imports of the deleted `striatum.legacy_sqlite`
package, not representing a legitimate optional dependency skip.

## Import Map

| File | Current legacy imports | `striatum.db` / `striatum.migrations` imports | Recommendation |
|---|---|---|---|
| `tests/test_cli_mvp.py` | `from striatum.legacy_sqlite.db import connect` at module scope; local imports of `init_repo`; local `connect, db_path` import in evidence-redaction test. | None found. | Split. Convert workflow lifecycle, artifact, verdict, recovery, summary, and adapter assertions to daemon/PostgreSQL handler or daemon-harness coverage. Quarantine only explicitly historical SQLite evidence/export assertions if they still prove one-way import compatibility. |
| `tests/test_service.py` | Local `connect`/`init_repo` imports in phase/SSE fixtures; local `import striatum.legacy_sqlite.db as db` in web mutation/error fixtures; many string monkeypatches against `striatum.legacy_sqlite.service.sqlite3.connect`. | None found. | Convert service DTO, web mutation, JSON read, startup, SSE, and daemon-route tests to daemon DTO mocks or PG harness. Quarantine subprocess service tests that intentionally exercise the retired legacy fixture mode, or delete if covered by daemon-backed service tests. Replace monkeypatch strings with production-module tripwires after legacy package deletion. |
| `tests/test_artifact_schemas.py` | `from striatum.legacy_sqlite.db import connect` at module scope; local `init_repo` import in the lifecycle helper. | None found. | Convert publish-artifact/front-matter integration tests to daemon/PostgreSQL publish coverage. Keep pure parser/schema tests in this file without any repo-state setup. Move `SCHEMA_SQL` legacy bootstrap assertion to historical fixture coverage or delete if SQLite schema retention is no longer accepted. |
| `tests/test_process_adapter.py` | `from striatum.legacy_sqlite.db import connect` at module scope; local `init_repo` imports in `_start_with` and migration-idempotence test. | None found. | Preserve `adapter run is retired outside legacy test harness` as active production coverage. Move old adapter-run behavior to explicit historical fixture coverage only if needed; otherwise replace with daemon/PG process-supervision and `process_reconcile` handler tests. |
| `tests/architecture/test_legacy_sqlite_quarantine.py` | No direct `striatum.legacy_sqlite` imports; it scans for them. | No direct facade imports except scanner string constants. | Keep active and unskipped. Tighten after batch conversion so test fixtures no longer get a broad `tests/` SQLite-reference quarantine. |

## Hidden Skip Pattern

The module-level skip appears at:

- `tests/test_cli_mvp.py:3`
- `tests/test_service.py:9`
- `tests/test_artifact_schemas.py:5`
- `tests/test_process_adapter.py:14`
- `tests/architecture/test_authority_guardrails.py:3`

The authority guardrail skip is outside the four requested conversion files but is part of the architecture guardrail surface. It currently disables the command-authority drift checks entirely, so it should be removed or narrowed before treating Track 2 as complete.

`tests/architecture/test_legacy_sqlite_quarantine.py` is not skipped. It already contains deleted-state assertions for `src/striatum/legacy_sqlite`, production import bans, and fixture facade-import checks. Its current weak point is `TEST_SQLITE_QUARANTINE_PREFIXES = {Path("tests"): ...}`, which classifies all test SQLite references as fixtures. After converting this batch, narrow that prefix to explicit historical fixture paths.

## Suggested Batches

1. `test_artifact_schemas` first: separate pure front-matter parser tests from publish-artifact integration tests. Convert publish integration to daemon PG workflow-loop coverage, and decide whether `test_legacy_sqlite_bootstrap_does_not_constrain_artifact_kind` belongs in a quarantined historical SQLite fixture.

2. `test_process_adapter` second: keep the retired-path assertion active, then convert or delete the legacy adapter-run behavior. Most active runtime behavior now belongs beside daemon PG supervision/recovery tests, especially `tests/daemon_pg/handlers/recovery_evidence/test_process_reconcile.py` and `tests/daemon_pg/handlers/test_supervision.py`.

3. `test_service` third: retain daemon DTO and route-policy tests by mocking `striatum.service_daemon.call_repo_method` and tripwiring production modules, not the deleted legacy package. Subprocess service tests should use a daemon-backed fixture or become explicit legacy service fixtures.

4. `test_cli_mvp` last: this is the broadest legacy fixture. Move remaining current behavior into focused daemon PG handler, CLI dispatch, workflow-validation, evidence, recovery, and branch tests. Quarantine only narrow historical assertions that still need legacy SQLite import material.

5. Tighten guardrails: remove module-level skip from `tests/architecture/test_authority_guardrails.py`, keep `tests/architecture/test_legacy_sqlite_quarantine.py` active, and replace the broad `tests/` SQLite quarantine with explicit paths such as `tests/_fixtures/legacy/` if any remain.

## Focused Verification

For each implementation batch, run the converted file plus its nearest active daemon/architecture coverage:

- Artifact batch: `pytest tests/test_artifact_schemas.py tests/daemon_pg/handlers/workflow_loop/test_publish_artifact.py`
- Process batch: `pytest tests/test_process_adapter.py tests/daemon_pg/handlers/recovery_evidence/test_process_reconcile.py tests/daemon_pg/handlers/test_supervision.py`
- Service batch: `pytest tests/test_service.py tests/test_service_runtime.py tests/test_service_state.py tests/test_service_sse.py`
- CLI MVP split: run the moved focused files plus `pytest tests/daemon_pg/handlers/workflow_loop tests/daemon_pg/handlers/run_lifecycle tests/daemon_pg/handlers/recovery_evidence`
- Guardrails: `pytest tests/architecture/test_legacy_sqlite_quarantine.py tests/architecture/test_authority_guardrails.py`

Final Track 2 gate should include `make test` or the nearest CI-equivalent shard after the skipped files are re-enabled.
