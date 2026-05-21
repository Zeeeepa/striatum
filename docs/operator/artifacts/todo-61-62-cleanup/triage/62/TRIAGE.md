---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["todo_62", "docs/rfcs/0069-pg-only-daemon-global-surfaces.md", "docs/operator/plans/rfc-0069-pg-only-daemon-global-surfaces.md", "docs/architecture/COMMAND_AUTHORITY_MATRIX.md", "docs/operator/artifacts/todo-61-62-cleanup/triage/61/TRIAGE.md"]
---

# TODO 62 Residual Cleanup Triage
author: triager-claude-code-001

## Summary

RFC 0069's substantive surfaces are landed: production `connect_registry()` is
gated behind a paired test-harness escape, the Python daemon module is
deleted, daemon-global reads (`dashboard.all`, MCP resources, status, stop,
health, audit, doctor) read from PostgreSQL, and the Go daemon owns
`repo.*` plus a resident recovery scheduler over PostgreSQL active runs.

The residuals are narrow and concrete:

1. The daemon doctor still treats the legacy SQLite filename
   (`.striatum/state.sqlite3`) as the operational-scratch existence probe and
   raises a misleading warning on every repo registered before the
   `state_db_path = .striatum/` shape landed.
2. The repo registrar refuses migration based on a hardcoded
   `.striatum/state.sqlite3` filename rather than calling
   `striatum.repo_policy.db_path`; the cleanup should retire `repo_policy.DB_NAME`
   alongside the doctor fix or treat it as the only legacy filename allowed.
3. The Python `repo_resolve_pg` reach-around path is structurally identical to
   the deleted `connect_registry()` guardrail concern: it canonicalises
   `state = repo / .striatum / state.sqlite3` purely to refuse symlinks. The
   probe is correct; the literal still encodes SQLite as live state.
4. `repo_policy.DB_NAME`, `db_path`, and the cutover-report reliance on it are
   the last live-state names in the Python production tree; the test
   architecture quarantine permits them by category but they should follow
   TODO 61 retirement to remove the last "state.sqlite3 is live" surface.

The blocked open question on the RFC 0069 plan — "whether registry-probe /
global diagnostic paths should be generated from the daemon method contract"
— stays a product decision, not implementation work.

## Highest Priority Cleanup

1. Stop the daemon-doctor `daemon_repo_state_missing` symptom on
   pre-cutover registrations.

   Symptom: `striatum daemon doctor --authority --json` reports
   `daemon_repo_state_missing` for `/.../.striatum/state.sqlite3` even though
   `.striatum/` exists, PostgreSQL is authoritative, and SQLite live state is
   retired. Verified locally on this repo:

   ```
   "problem_records": [
     {"check": "daemon_repo_state_missing",
      "context": {"repo_root": "/home/halbritt/git/striatum",
                  "state_db_path": "/home/halbritt/git/striatum/.striatum/state.sqlite3"},
      "id": "repo_a89ecd1664764f039a127c62ab7da3f3",
      "message": "registered repository operational scratch is missing"}
   ]
   ```

   Root cause: `daemon_doctor_records_pg` in
   `src/striatum/daemon_pg/client_admin.py:517-520` queries the
   `striatumd.repositories.state_db_path` column and asserts
   `not state_path.exists()`. Both the Python registrar
   (`src/striatum/daemon_pg/repositories.py:96` and `:108`) and the Go registrar
   (`go/pkg/repositories/service.go:94` and `go/pkg/admin/repo_init.go:119`)
   now persist the `.striatum` *directory* into that column, but any
   repository registered before that change still has the row pointing at the
   `state.sqlite3` *file*, which is now archived/tombstoned. The check then
   flags a perfectly healthy operational-scratch directory as missing.

   Implementer paths (either is acceptable; pick one and stay consistent):

   a. Change the doctor probe to test `Path(repo_root) / ".striatum"`
      (operational scratch is repo-root-relative) and rename the check id to
      `daemon_repo_scratch_missing` with a message that says "registered
      repository `.striatum/` operational scratch is missing". Drop
      `state_db_path` from the warning context and report `state_dir` instead.
      Keep `repo_root` in the context. The new check should refuse to fire if
      `Path(repo_root) / ".striatum"` is a directory regardless of what the
      column stored historically.

   b. Forward-fix existing rows by adding a one-shot daemon-startup
      normalisation that rewrites
      `striatumd.repositories.state_db_path` to the parent operational-scratch
      directory when the stored value ends in `state.sqlite3` and the parent
      exists. Land this as a tiny PostgreSQL migration (or in
      `striatum.daemon_pg.repositories` on the next `repo.list`/`repo.resolve`)
      so operator inspection and projection clients converge.

   Either path should keep emitting `daemon_repo_state_missing` (renamed) for
   the true failure mode where the registered repo has no `.striatum/`
   directory at all.

   Files to update for path (a):
   - `src/striatum/daemon_pg/client_admin.py:477-525` — the
     `daemon_doctor_records_pg` body.
   - `src/striatum/daemon_pg/mcp_resources.py:405` — read the same column
     name; the resource projection should not emit
     `state.sqlite3` for new clients.
   - `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` — surface the warning
     rename if/when the matrix lists it.

   Regression coverage to add:
   - Extend `tests/test_daemon_pg_doctor.py` and
     `tests/cli/test_dispatch_daemon_doctor.py` with a case where the
     `state_db_path` column is `.../.striatum/state.sqlite3` and the
     `.striatum/` directory exists; doctor must report `"problem_records":
     []`, `"problems": []`, and the authority report must remain `ok: true`.
   - Add a parity case where `.striatum/` itself does not exist and the
     renamed warning fires, including `repo_root` and `state_dir` in the
     context.

   Files to update for path (b):
   - `src/striatum/daemon_pg/repositories.py:84-108` plus a tiny migration
     under `src/striatum/daemon_pg/sql/` and `go/pkg/db/sql/` so the
     normalised value lands once and stays correct.

2. Stop emitting the SQLite filename when persisting a fresh registration.

   Both registrars already store the `.striatum/` directory in
   `state_db_path`, but the Python pre-flight refusal in
   `src/striatum/daemon_pg/repositories.py:38-44` still hardcodes
   `legacy_state = repo / ".striatum" / "state.sqlite3"`. The probe is
   correct, but the literal duplicates `striatum.repo_policy.db_path`.
   The cleanup is to call `db_path(repo)` (or, if `repo_policy.DB_NAME`
   is retired alongside this slice, the explicit constant kept inside
   the registrar with a comment) so the only place the legacy filename
   appears in production code is the single migration-refusal probe.

   The matching Python symlink probe at
   `src/striatum/daemon_pg/repositories.py:217-219` is similarly fine in
   behaviour but encodes the literal. Move it to `db_path(repo)` for
   consistency with item 1.

3. Retire `repo_policy.DB_NAME` / `db_path` as the last
   "SQLite-is-live-state" production names, or quarantine them explicitly.

   Evidence: `src/striatum/repo_policy.py:11-28` exports
   `DB_NAME = "state.sqlite3"` and `db_path(repo)` returning
   `repo / .striatum / state.sqlite3`. The only production callers are the
   cutover-report `src/striatum/daemon_pg/repo_cutover_report.py:14, 47, 122`
   (correct — it reports cutover status of the tombstoned file) and the
   two registrar probes from item 2. Day-zero inspection in
   `src/striatum/day_zero.py:143, 206, 212` reaches the literal directly.

   The test architecture quarantine in
   `tests/architecture/test_legacy_sqlite_quarantine.py:42-62` lists
   `DB_NAME`, `STATE_DIR`, `db_path`, and `state_dir` in
   `NEUTRAL_DB_REEXPORTS`. That set is the legacy `striatum.db` facade
   reanchor — the test asserts they are not reimported through
   `striatum.db`. The constants themselves remain product-acceptable on
   `repo_policy`.

   Implementer rule: do not delete `DB_NAME` / `db_path` opportunistically.
   Touch them only when item 2 lands. If the cutover-report can survive
   without `db_path`, retiring `DB_NAME` becomes a one-line follow-up that
   removes the last production reference to the literal `state.sqlite3`
   filename outside cutover diagnostics.

4. Keep the architecture guardrails covering everything RFC 0069 promised.

   Evidence: `tests/architecture/test_authority_guardrails.py`,
   `tests/architecture/test_legacy_sqlite_quarantine.py`, and
   `tests/cli/test_daemon_doctor_without_daemon.py` already assert (a) no
   production sources import `striatum.daemon`, (b)
   `src/striatum/daemon.py` stays deleted, (c) the only allowed legacy
   SQLite usages are migration / service-transition / dogfood-fixture /
   bootstrap-admin / test-fixture. Don't relax these to satisfy the cleanup.

   Verify after item 1 lands that the daemon doctor unit tests
   (`tests/test_daemon_pg_doctor.py`, `tests/test_daemon_pg_sweep.py`,
   `tests/cli/test_dispatch_daemon_doctor.py`) still pass without the false
   `daemon_repo_state_missing` record, and that the
   `striatum.authority_report.v1` envelope retains `ok: true` on a healthy
   repo with the directory present.

## Lower Priority But Worth Doing

- `docs/POSTGRES_TRANSITION.md` and `docs/HOW_TO_AGENT.md` describe the
  retired SQLite path with the literal `.striatum/state.sqlite3` filename
  several times. Those are correct operator-facing descriptions of the
  one-time migration window and should stay; do not rewrite them as part of
  the doctor fix.
- The Python skill / plugin templates
  (`src/striatum/plugins/templates/*/skills/mcp.md.tmpl`,
  `src/striatum/skills/templates/*/STRIATUM_*_GUIDE.md.tmpl`,
  `src/striatum/skills/context.py:60-65`) still tell agents
  "do not rely on `.striatum/state.sqlite3`". This is correct guidance and
  should not change.
- `src/striatum/cli/daemon_required.py:189-220`
  (`is_repo_migrated_for_daemon`) is an explicit pre-cutover signal; the
  literal filenames are load-bearing for the refusal envelope and should
  stay until daemon-required enforcement is removed (which it should not
  be).
- The TODO 61 triage (sibling artifact) already covers retiring legacy
  SQLite test imports. Item 1 here can land independently of that
  retirement.

## Suggested Verification Commands

After landing item 1:

```bash
striatum daemon doctor --authority --json | python3 -m json.tool \
  | grep -E '"(check|message|state_(db_)?path|live_state_authority|ok)"'
pytest tests/test_daemon_pg_doctor.py tests/test_daemon_pg_sweep.py \
       tests/cli/test_dispatch_daemon_doctor.py \
       tests/cli/test_daemon_doctor_without_daemon.py
pytest tests/architecture/test_legacy_sqlite_quarantine.py \
       tests/architecture/test_authority_guardrails.py
```

After item 2 lands:

```bash
rg -n "state\\.sqlite3" src/striatum/daemon_pg
pytest tests/daemon_pg/test_repo_registration.py tests/exit_codes/test_rfc0043_refusals.py
```

After item 3 (only if `DB_NAME` / `db_path` retirement is in scope):

```bash
rg -n "from striatum\\.repo_policy import .*(DB_NAME|db_path)" src tests
pytest tests/architecture/test_legacy_sqlite_quarantine.py
```

## Non-Goals For This Cleanup

- Do not reintroduce `connect_registry()` or any SQLite-backed daemon
  registry helper. The retired Python daemon module must stay deleted; the
  paired `STRIATUM_TEST_HARNESS=1` + `STRIATUM_DAEMON_REQUIRED=0` escape
  remains the only legacy path.
- Do not change the daemon RPC envelope, capability vocabulary, or
  `repo.*` semantics; RFC 0069 §Non-Goals applies.
- Do not flip the default daemon core or alter RFC 0068 sequencing.
- Do not delete `.striatum/state.sqlite3.tombstone` or
  `.striatum/state.sqlite3.archive-*` operator evidence on this repo or
  any registered repo; D108/D111 keep them as inspection-only artifacts.
- Do not modify cutover-report semantics in
  `src/striatum/daemon_pg/repo_cutover_report.py` beyond replacing the
  `repo_policy.db_path` literal if item 3 reaches it; the report is the
  designated diagnostic surface for the retired SQLite file.
