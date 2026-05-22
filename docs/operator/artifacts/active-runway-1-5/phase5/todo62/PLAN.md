---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/rfcs/0069-pg-only-daemon-global-surfaces.md", "docs/operator/artifacts/todo-61-62-cleanup/triage/62/TRIAGE.md", "docs/operator/artifacts/todo-61-62-cleanup/final/SUMMARY.md", "docs/operator/plans/active-runway-1-5.md", "src/striatum/repo_policy.py", "src/striatum/day_zero.py", "src/striatum/daemon_pg/repositories.py", "src/striatum/daemon_pg/repo_cutover_report.py", "src/striatum/cli/daemon_required.py", "src/striatum/cli/daemon.py", "src/striatum/cli/parser.py", "tests/architecture/test_legacy_sqlite_quarantine.py"]
---

# TODO 62 PG-Only Daemon-Global Guardrail Cleanup Plan
author: cleanup-planner-claude-code-001
status: ready
date: 2026-05-22

## Outcome

Define the smallest bounded production-source cleanup that finishes RFC 0069's
"PostgreSQL-only daemon-global surfaces" residual list without disturbing
SQLite migration-refusal diagnostics, operator-facing refusal wording, or
TODO 61 test-quarantine state. The remediation already accepted under
[`todo-61-62-cleanup`](../../../todo-61-62-cleanup/final/SUMMARY.md) covers
the visible daemon-doctor symptom and the stale repository projection; this
plan handles only the literal-encoding residuals the prior cleanup
deliberately deferred.

## Authority Reading

The RFC 0069 substantive surfaces have all landed:

- Production `striatum.daemon.connect_registry()` is deleted; the Python
  daemon module is gone; production sources do not import the retired
  `striatum.legacy_sqlite` package
  (`tests/architecture/test_legacy_sqlite_quarantine.py` asserts both).
- `dashboard.all`, MCP `resources/list`/`read`, daemon `status`, `stop`,
  `health`, `audit`, and repo-scoped `read_doctor` read from PostgreSQL.
- Repo registrar refusal for legacy SQLite is PostgreSQL-only;
  `repo_add_pg` calls `repo_policy.db_path(repo)` and refuses without
  opening SQLite (`src/striatum/daemon_pg/repositories.py:39-44`).
- Doctor probe in `daemon_doctor_records_pg` reports
  `daemon_repo_scratch_missing` against `state_dir(repo_root)`, not the
  legacy filename (`src/striatum/daemon_pg/client_admin.py:518-528`).
- `repo_list_pg`/`repo_resolve_pg` and the MCP resource projection
  normalize stale `state_db_path` rows that still end in `state.sqlite3`
  back to the `.striatum/` scratch directory without rewriting the
  column (`src/striatum/daemon_pg/repositories.py:270-279`;
  `src/striatum/daemon_pg/mcp_resources.py:399-422`).
- `PRODUCTION_SQLITE_QUARANTINE` and `DAEMON_CONNECT_REGISTRY_CALLERS`
  guardrail allowlists are empty.

The only durable PostgreSQL column name still spelled `state_db_path`
is `striatumd.repositories.state_db_path` in
`src/striatum/daemon_pg/sql/0001_baseline.sql:26`. The column is the
operator-visible projection key; do not rename it.

## Two Surfaces, One Vocabulary

RFC 0069's vocabulary distinguishes:

1. **Legacy SQLite file refusal / diagnostics.** Code that refuses to
   open `.striatum/state.sqlite3`, reports migration-required envelopes,
   or projects a tombstone/migrated sentinel. These paths intentionally
   spell the legacy filename in user-visible refusal text.
2. **Live SQLite authority.** Code that would treat
   `.striatum/state.sqlite3` as a production datastore. Production no
   longer contains any. RFC 0069 §Acceptance Criteria forbid this.

This plan only touches surface (1) to remove ad-hoc literals where
`repo_policy.db_path(repo)` is mechanically equivalent. It does not
rewrite operator-facing refusal wording, skill/plugin templates, or
tombstone-inspection diagnostics.

## Stale Literal-Encoding Residuals (production sources)

These are the remaining `repo / ".striatum" / "state.sqlite3"`-shaped
literals in `src/striatum/**` that still encode the legacy filename
directly instead of routing through `repo_policy.db_path` /
`repo_policy.state_dir`:

| Location | Role | Cleanup |
|---|---|---|
| `src/striatum/day_zero.py:148` | `state_db_path` value in `sqlite_migration_required` envelope | Replace with `str(db_path(repo))`. |
| `src/striatum/day_zero.py:211` | `_inspect_repo` `state_db` probe | Replace with `state_db = db_path(repo)`. |
| `src/striatum/day_zero.py:217` | `state_tombstone_exists` probe | Derive from `db_path(repo).with_name(db_path(repo).name + ".tombstone")`. |
| `src/striatum/daemon_pg/repo_cutover_report.py:49` | `sentinel_path` for migration sentinel | Derive from `db_path(repo).with_name(db_path(repo).name + ".migrated")`. |
| `src/striatum/cli/daemon_required.py:193-194` | `is_repo_migrated` helper | Replace `state_db` / `tombstone` paths with `db_path(repo_path)` and `db_path(repo_path).with_name(db_path(repo_path).name + ".tombstone")`. The wording-bearing refusal messages around it (`render_repo_not_migrated_message` / `_hint`) stay as-is. |

The cutover report at `src/striatum/daemon_pg/repo_cutover_report.py`
already routes `source_path = db_path(repo)`, `state_db = db_path(repo)`,
and `state_stat = db_path(repo).stat()` through `repo_policy.db_path`.
Only the sentinel literal at line 49 is new work.

## Stale References To Leave Alone

These are correct migration-refusal/diagnostic surfaces and must keep
the literal `.striatum/state.sqlite3` spelling because the wording or
output shape is operator-facing or load-bearing:

- Refusal messages and CLI help text:
  `src/striatum/cli/daemon.py:21`,
  `src/striatum/cli/daemon_required.py:129, 139`,
  `src/striatum/cli/parser.py:347`.
- Operator skill / plugin guidance templates under
  `src/striatum/plugins/templates/**/skills/*.tmpl` and
  `src/striatum/skills/templates/**/*.tmpl` plus
  `src/striatum/skills/context.py:62`. The directive
  "do not rely on `.striatum/state.sqlite3`" is the intended guidance.
- Cutover-report `_sqlite_exception_notes` and `sqlite_finalization`
  diagnosis strings in
  `src/striatum/daemon_pg/repo_cutover_report.py:300-314, 594-614`.
  These are the designated diagnostic surface for the retired file.
- The durable PostgreSQL `striatumd.repositories.state_db_path` column.

Docs that describe the cutover (`docs/POSTGRES_TRANSITION.md`,
`docs/HOW_TO_AGENT.md`, `docs/SPEC.md`, `docs/CLI_REFERENCE.md`,
`docs/HOW_TO_HUMAN.md`) are operator-facing references to the one-time
migration window and stay as-is.

## Tests That Stay As Migration-Refusal Coverage

These tests already encode the correct post-cutover semantics and must
remain unchanged by this cleanup:

- `tests/daemon_pg/test_repo_registration.py` —
  `test_repo_list_and_resolve_normalize_stale_state_sqlite_projection`
  (lines 68-103) and
  `test_repo_add_refuses_existing_sqlite_source_without_opening_it`
  (lines 106-120). Stale-projection normalization and SQLite refusal
  coverage.
- `tests/test_daemon_pg_doctor.py`,
  `tests/cli/test_dispatch_daemon_doctor.py`,
  `tests/cli/test_daemon_doctor_without_daemon.py`. Doctor warning
  rename and behavior coverage.
- `tests/test_mcp_capability_scope_e2e.py:140-185`. Stale state_db_path
  projection normalization in MCP resources.
- `tests/exit_codes/test_rfc0043_refusals.py`,
  `tests/exit_codes/test_rfc0043_split_brain.py`. Daemon-required
  refusal envelopes when SQLite files are present.
- `tests/test_corpus_redaction.py:24`. Redaction of legacy state paths
  in evidence exports.
- `tests/test_view_file.py:74`, `tests/test_web_view.py:108`. The view
  file endpoint must 404 on `.striatum/state.sqlite3`.
- `tests/test_ui_packaging.py:43`. Fresh-clone smoke wraps the legacy
  file path.
- `tests/test_doc_links.py:119-190`. Doc-link integrity tests that pin
  the wording "is not authoritative live state."
- `tests/test_multi_repo_harness.py:28`, `tests/test_harness_v2_fixes.py:232`,
  `tests/test_process_adapter.py:62`. Negative assertions that
  `.striatum/state.sqlite3` does not exist after harness setup.
- All `tests/daemon_pg/**` fixture rows that insert
  `state_db_path = .../.striatum/state.sqlite3` to exercise the legacy
  shape against current projections.
- `tests/architecture/test_legacy_sqlite_quarantine.py`. The full
  quarantine assertion set; do not relax it.
- `tests/test_day_zero.py`. The day-zero `sqlite_migration_required`
  envelope test must keep asserting the envelope shape; if the
  envelope's `state_db_path` becomes `str(db_path(repo))` after this
  cleanup, the test expectation updates to the same `db_path(repo)`
  computation — it does not change the user-visible literal.

`RESIDUAL_LEGACY_SQLITE_TEST_IMPORTS` and `RESIDUAL_SQLITE_REFERENCE_TESTS`
in `tests/architecture/test_legacy_sqlite_quarantine.py` are owned by
the TODO 61 Track-2 test-debt retirement workflow. Do not touch them
from this cleanup batch.

## Smallest Cleanup Batch

One bounded batch with a single disjoint write scope. All five edits
are mechanical literal-to-helper substitutions inside the
`legacy_state = repo / ".striatum" / "state.sqlite3"`-shape; none
change semantics or operator-visible output strings.

Write scope (single batch, single repo_write):

```
allowed_paths:
  - src/striatum/day_zero.py
  - src/striatum/daemon_pg/repo_cutover_report.py
  - src/striatum/cli/daemon_required.py
  - tests/test_day_zero.py
forbidden_paths:
  - .striatum/
```

Edits:

1. `src/striatum/day_zero.py`
   - Import: add `from striatum.repo_policy import db_path` near the
     top of the module.
   - `state_db_path` value at line 148: replace the inline literal
     with `str(db_path(repo))`.
   - `_inspect_repo` at line 211: replace the inline `state_db`
     assignment with `state_db = db_path(repo)`.
   - `_inspect_repo` at line 217: replace the tombstone probe with
     `db_path(repo).with_name(db_path(repo).name + ".tombstone").exists()`
     (or cache `db_path(repo)` to a local and derive once).
2. `src/striatum/daemon_pg/repo_cutover_report.py`
   - Replace the `sentinel_path = repo / ".striatum" / "state.sqlite3.migrated"`
     literal at line 49 with
     `sentinel_path = source_path.with_name(source_path.name + ".migrated")`
     (where `source_path = db_path(repo)` is already computed at line
     47). Note: `source_path.with_name(source_path.name + ".tombstone")`
     is already the pattern used at line 48.
3. `src/striatum/cli/daemon_required.py`
   - Import `db_path` from `striatum.repo_policy`.
   - `is_repo_migrated` body at lines 193-194: replace the two inline
     literals with `state_db = db_path(repo_path)` and
     `tombstone = state_db.with_name(state_db.name + ".tombstone")`.
   - Do not change `render_repo_not_migrated_message` /
     `render_repo_not_migrated_hint`; their `.striatum/state.sqlite3`
     spelling is the user-facing refusal copy.
4. `tests/test_day_zero.py`
   - Update only the `sqlite_migration_required` envelope expectation
     (and any `_inspect_repo` assertion that compares `state_db_path`
     verbatim) to compute `db_path(repo)` the same way the source does.
     Keep the literal `.striatum/state.sqlite3` in any doc-string or
     hint-text assertion that pins user-facing copy.

This batch is intentionally narrow:

- It does not delete `repo_policy.DB_NAME` or `db_path`. The TRIAGE
  rule was to retire them only after the registrar/cutover-report
  rewrites land; cutover-report still relies on `db_path` after this
  batch, so `DB_NAME` retirement remains a separate one-line follow-up.
- It does not touch tests under
  `tests/daemon_pg/**`, `tests/exit_codes/**`,
  `tests/architecture/test_legacy_sqlite_quarantine.py`, or any
  test owned by TODO 61 Track-2.
- It does not modify operator-facing wording, plugin/skill templates,
  schema columns, or daemon RPC envelopes.

## Verification

After the batch lands, run:

```bash
ruff check src/striatum/day_zero.py \
          src/striatum/daemon_pg/repo_cutover_report.py \
          src/striatum/cli/daemon_required.py \
          tests/test_day_zero.py
pytest tests/test_day_zero.py \
       tests/exit_codes/test_rfc0043_refusals.py \
       tests/cli/test_daemon_doctor_without_daemon.py \
       tests/architecture/test_legacy_sqlite_quarantine.py \
       tests/architecture/test_authority_guardrails.py
pytest tests/test_daemon_pg_doctor.py \
       tests/daemon_pg/test_repo_registration.py
rg -n 'repo[^"]*"\.striatum"[^"]*"state\.sqlite3"' src/striatum
```

The final `rg` should return only the operator-facing refusal/wording
sites enumerated above (cli/daemon.py:21, cli/daemon_required.py
render_* messages, cli/parser.py:347, repo_cutover_report.py diagnosis
strings, plugin/skill templates, `skills/context.py`) plus the
`DB_NAME = "state.sqlite3"` definition in `repo_policy.py`.

## Non-Goals

- Do not reintroduce `connect_registry()` or any SQLite-backed daemon
  registry helper.
- Do not delete `repo_policy.DB_NAME` or `repo_policy.db_path` in this
  batch. They remain reachable from the cutover-report and the
  refusal-probe surfaces above.
- Do not rename the durable PostgreSQL column
  `striatumd.repositories.state_db_path` or its projection key.
- Do not retire `RESIDUAL_LEGACY_SQLITE_TEST_IMPORTS` or
  `RESIDUAL_SQLITE_REFERENCE_TESTS` entries; TODO 61 Track-2 owns
  that test-debt conversion.
- Do not change operator-facing refusal copy, skill/plugin templates,
  or doc references to `.striatum/state.sqlite3`.
- Do not modify CLI verbs, daemon RPC envelopes, MCP method names,
  or capability vocabulary.
- Do not flip the default daemon core or alter RFC 0068 sequencing.
- Do not delete `.striatum/state.sqlite3.tombstone` or
  `.striatum/state.sqlite3.archive-*` operator evidence.

## Open Question (Deferred, Not In Scope)

The RFC 0069 plan's deferred open question — whether remaining
registry-probe / global diagnostic paths should be generated from the
daemon method contract rather than curated by guardrail tests —
remains a separate product decision. It does not block this cleanup
batch and is intentionally left open.
