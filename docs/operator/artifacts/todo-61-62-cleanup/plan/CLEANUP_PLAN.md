---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/todo-61-62-cleanup/triage/61/TRIAGE.md", "docs/operator/artifacts/todo-61-62-cleanup/triage/62/TRIAGE.md"]
---

# TODO 61-62 Residual Cleanup Plan

## Objective

Reconcile the TODO 61 (Go production daemon / RFC 0068) and TODO 62 (PostgreSQL-only daemon-global surfaces / RFC 0069) triage artifacts into a bounded implementation plan. This plan eliminates technical debt from the retired SQLite live-state architecture while preserving current RFC 0050/0068/0069 progress.

## Implementation Tracks

### Track 1: TODO 62 Core Logic Fixes (High Priority)

Resolve the misleading daemon-doctor warnings and hardcoded SQLite literals in production paths.

1.  **Daemon Doctor Normalization (Option A):**
    *   Update `src/striatum/daemon_pg/client_admin.py` to make the operational scratch probe directory-aware.
    *   If `state_db_path` points to a file that does not exist, check if its parent directory exists and is named `.striatum`.
    *   Rename check ID to `daemon_repo_scratch_missing` and message to "registered repository `.striatum/` operational scratch is missing".
    *   Update `src/striatum/daemon_pg/mcp_resources.py` to ensure MCP projections also reflect the directory-based scratch path.

2.  **Repo Registration Cleanup:**
    *   Update `src/striatum/daemon_pg/repositories.py` to use `striatum.repo_policy.db_path(repo)` instead of hardcoded `.striatum/retired-local-state` literals for the migration-refusal probe.
    *   Ensure the symlink probe also uses the policy helper.

### Track 2: TODO 61 Test Debt Reduction (High Priority)

Eliminate the dependency on `striatum.legacy_sqlite` and retired SQLite modules in the test suite.

1.  **Batch Test Conversion:**
    *   Identify and convert tests using `from striatum.legacy_sqlite import ...`.
    *   Primary targets: `tests/test_cli_mvp.py`, `tests/test_service.py`, `tests/test_artifact_schemas.py`, `tests/test_process_adapter.py`.
    *   Convert to use `tests/_harness/pg.py`, `tests/_harness/daemon.py`, or move to a quarantined `tests/_fixtures/legacy/` path for historical assertion.

2.  **Architecture Guardrail Update:**
    *   Update `tests/architecture/test_legacy_sqlite_quarantine.py` to enforce the "deleted" state of `striatum.legacy_sqlite`.
    *   Assert no production imports of `striatum.legacy_sqlite`, `striatum.db`, or `striatum.migrations`.

3.  **Service Fallback Retirement:**
    *   Remove `STRIATUM_TEST_HARNESS=1` and related bypasses from `src/striatum/service_command_policy.py` once tests are migrated.
    *   Eliminate the `STRIATUM_DAEMON_REQUIRED=0` escape for paths that are now fully daemon-routed.

### Track 3: Corpus & Skill Templates (Medium Priority)

1.  **Corpus Export Split:**
    *   Refactor `src/striatum/corpus/export.py` to remove `PRAGMA user_version` and legacy `state_authority` hardcoding for production exports.
    *   Ensure production corpus export reads from the daemon/PG metadata.

2.  **Skill/Plugin Template Audit:**
    *   Ensure skill templates and `src/striatum/skills/context.py` reflect that the native daemon/MCP is the primary interface, not just the CLI.

## Blocked Product Decisions (Out of Scope)

The following items remain blocked and **must not** be implemented as part of this cleanup:

*   **PostgreSQL-Native Operator Composites:** Do not reintroduce `dogfood.publish_on_behalf` or `dogfood.surgical_recovery` as PG-native methods. They remain `method_unknown`.
*   **TODO 55, 56, 59, 60:** Any product decisions regarding these TODOs are outside the scope of this architecture-cleanup workstream.
*   **Daemon Contract Generation:** Automated generation of registry-probe or global diagnostic paths from the daemon method contract remains a separate product decision.

## Verification Strategy

1.  **Architecture Integrity:**
    *   `pytest tests/architecture/test_legacy_sqlite_quarantine.py`
    *   `pytest tests/architecture/test_authority_guardrails.py`
2.  **Functional Correctness:**
    *   `striatum daemon doctor --authority` (confirm no false `daemon_repo_state_missing`)
    *   `make daemon-go-conformance`
3.  **Regression Suite:**
    *   `make test` (Ensure no regressions in MCP, CLI, or repository lifecycles)

## Sequencing

1.  **Phase 1:** Land Track 1 (Doctor & Registration fixes). These are the most visible "bugs" reported in the triage.
2.  **Phase 2:** Land Track 2 (Architecture test updates and initial batch of test conversions).
3.  **Phase 3:** Complete Track 2 (Service fallback removal) and Track 3 (Corpus/Templates).
