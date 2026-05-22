---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/operator/plans/active-runway-1-5.md", "docs/operator/BRIEF.md", "docs/rfcs/0068-go-production-daemon-port.md", "docs/rfcs/0069-pg-only-daemon-global-surfaces.md", "docs/rfcs/0070-daemon-client-service-boundary.md", "docs/rfcs/0071-operator-diagnostics-and-cutover-evidence.md", "tests/architecture/test_legacy_sqlite_quarantine.py"]
---

# TODO 61 Legacy Fixture Cleanup Plan
author: cleanup-planner-codex-001

## Scope

Plan the next bounded TODO 61 cleanup batch only. This does not reopen TODO
55/56/59/60 policy and does not change the Go-daemon production boundary:
daemon-owned PostgreSQL remains live authority, Python remains a client layer,
and SQLite survives only as explicit historical fixture material.

The next batch should target residual test debt, not production source. Current
guardrails already assert that `src/striatum/legacy_sqlite/` and
`src/striatum/daemon.py` are gone, production sources do not import the legacy
SQLite package, and the first Track 2 test batch is clean. The remaining debt is
mostly module-level skipped tests that still import `striatum.legacy_sqlite` or
`sqlite3` as a fixture substrate.

## Next Bounded Batch

Batch name: `todo61-track3-skipped-fixture-prune`

Goal: remove one coherent cluster of broad module-level legacy skips and shrink
`RESIDUAL_LEGACY_SQLITE_TEST_IMPORTS` without touching production code.

Primary targets:

- `tests/exit_codes/test_rfc0043_split_brain.py`
- `tests/test_artifact_view_provenance_trail.py`
- `tests/test_view_file_breadcrumb_heuristic.py`
- `tests/test_job_detail_expected_artifacts.py`
- `tests/test_run_detail_recovery_panel.py`

Recommended handling:

- Convert `tests/exit_codes/test_rfc0043_split_brain.py` into active
  daemon-required refusal tests that create only the on-disk legacy signal
  files needed by the preflight check. Do not import `connect` or `db_path`,
  and do not open SQLite.
- Delete `tests/test_view_file_breadcrumb_heuristic.py` if its live assertion
  is already covered by `tests/test_view_file.py`; otherwise port only the
  breadcrumb assertion to the active view-file fixture.
- Delete or port the three web detail/provenance tests only when their asserted
  behavior has a current daemon-backed equivalent. If the behavior is obsolete
  because the old page shape was replaced, delete the file and cite the active
  coverage that supersedes it.

This is intentionally small. It removes five quarantine entries at most and
keeps the batch away from larger workflow lifecycle families such as recovery,
supervision, skills install, and dogfood composite history.

## Preserve As Historical

Keep these out of the cleanup batch:

- `tests/fixtures/v1_repo_local_sqlite/state.sqlite3`
- `tests/fixtures/v1_repo_local_sqlite/build_fixture.py`
- `tests/fixtures/v1_repo_local_sqlite/README.md`
- `tests/daemon_pg/handlers/recovery_evidence/conftest.py`

Those files are explicitly historical or fixture-support material for one-way
import/export evidence. They may be revisited only in a later migration-fixture
batch that decides whether the repository-local SQLite corpus is still needed.
Do not delete them as incidental cleanup while pruning skipped UI/refusal
tests.

## Disjoint Implementation Work Scope

Use three independent implementation lanes if the operator wants parallelism:

| Lane | Allowed write scope | Task |
|---|---|---|
| refusal-fixture | `tests/exit_codes/test_rfc0043_split_brain.py` | Remove legacy imports and re-enable split-brain refusal coverage without SQLite opens. |
| view-fixture | `tests/test_view_file_breadcrumb_heuristic.py`, `tests/test_artifact_view_provenance_trail.py` | Delete obsolete skipped tests or port the surviving assertions to active daemon/web fixtures. |
| detail-panel-fixture | `tests/test_job_detail_expected_artifacts.py`, `tests/test_run_detail_recovery_panel.py` | Delete obsolete skipped detail-panel tests or port surviving assertions to active daemon-backed web fixtures. |

After those lanes land, run one short integration cleanup pass over only:

- `tests/architecture/test_legacy_sqlite_quarantine.py`

That pass removes the converted/deleted files from
`RESIDUAL_LEGACY_SQLITE_TEST_IMPORTS` and, if applicable,
`RESIDUAL_SQLITE_REFERENCE_TESTS`.

## Guardrail Tightening

After the batch, tighten guardrails in this order:

1. Add a second batch constant next to `TRACK2_FIRST_BATCH_TESTS`, for example
   `TRACK3_SKIPPED_FIXTURE_PRUNE_TESTS`, containing the five target paths.
2. Add an assertion that none of those paths has a module-level
   `pytest.skip(..., allow_module_level=True)`.
3. Add an assertion that none of those paths imports `striatum.legacy_sqlite`,
   `striatum.db`, `striatum.migrations`, or raw `sqlite3`.
4. Keep the existing residual-import ledger for future batches, but make this
   batch disappear from it.

Verification for the implementation batch:

- `pytest tests/exit_codes/test_rfc0043_split_brain.py tests/test_view_file.py`
- Run any converted web detail tests directly if they remain.
- `pytest tests/architecture/test_legacy_sqlite_quarantine.py`

## Exit Criteria

The batch is complete when:

- no target file remains as a broad module-level legacy SQLite skip;
- no target file imports the legacy SQLite package or raw `sqlite3`;
- the historical `tests/fixtures/v1_repo_local_sqlite/` fixture remains
  untouched unless a later batch explicitly scopes it;
- `tests/architecture/test_legacy_sqlite_quarantine.py` names the reduced
  residual ledger and prevents the cleaned files from regressing.
