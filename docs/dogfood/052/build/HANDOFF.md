author: implementer-unknown-model-001

# Build Handoff — RFC 0043 V1.6 substrate hardening

Implementer: claude (validated choice from dogfood-050). Operator-driven
session — the scaffold from `0ce207d` had already landed the four
substrate-hardening edits at the source-file level; this implement pass
audited the scaffold against `docs/dogfood/052/DESIGN_SYNTHESIS.md`,
realigned F-lock with the synthesis exit-code choice (8, not 14), and
added the regression tests the synthesis acceptance section names.

## Shipped Scope

### F-escape — STRIATUM_DAEMON_REQUIRED=0 narrowed to test-harness

- `src/striatum/cli/daemon_required.py` (already in scaffold):
  `resolve_requirement` returns `None` only when the command is on
  `DAEMON_OPTIONAL_COMMANDS` OR `STRIATUM_DAEMON_REQUIRED == "0"` **AND**
  `STRIATUM_TEST_HARNESS == "1"`. `ENV_TEST_HARNESS` constant exported.
  Module docstring updated.
- `tests/conftest.py` (already in scaffold): session-level
  `_legacy_sqlite_fixtures_opt_out` fixture now exports the pair
  (`STRIATUM_DAEMON_REQUIRED=0` + `STRIATUM_TEST_HARNESS=1`) so legacy
  SQLite-backed fixtures stay green. Production callers without the
  `STRIATUM_TEST_HARNESS=1` marker re-enter enforcement.
- `tests/exit_codes/test_rfc0043_refusals.py` (this pass):
  - Autouse fixture now also clears `STRIATUM_TEST_HARNESS` at function
    scope so each test asserts the production code path. Tests that
    intentionally exercise the paired opt-out set both vars.
  - Renamed `test_resolve_requirement_opt_out_with_env_zero` →
    `test_resolve_requirement_paired_opt_out_is_recognized` and tightened
    it to set both env vars.
  - Added `test_resolve_requirement_bare_env_zero_still_enforces` — the
    regression the prompt names: `STRIATUM_DAEMON_REQUIRED=0` without
    `STRIATUM_TEST_HARNESS=1` still returns an enforced
    `DaemonRequirement`.
  - Added `test_resolve_requirement_bare_test_harness_does_not_opt_out`
    for symmetry.
  - Added `test_dispatch_bare_env_zero_still_exits_11` — end-to-end exit
    code 11 from the dispatcher with bare env opt-out + missing socket.

Closes codex dogfood-050 threat-model finding.

### F-split-brain — `db.connect` refuses fresh SQLite when migrated

- `src/striatum/db.py::connect` (already in scaffold): before creating
  a fresh SQLite (source file absent), checks for sentinel
  `.striatum/state.sqlite3.migrated` OR tombstone
  `.striatum/state.sqlite3.tombstone`. If either present, raises
  `StriatumError(exit_code=12)` with `repo_not_migrated` remediation
  text matching the daemon-required path. The synthesis explicitly
  prefers the filesystem sentinel/tombstone check over a daemon
  registry query for V1.6 (no RPC coupling at the low-level connector).
- `tests/exit_codes/test_rfc0043_split_brain.py` (this pass — new file):
  - `test_split_brain_refuses_when_sentinel_present`: asserts exit code
    12 AND that no fresh `state.sqlite3` is created (the entire point
    of the guard — silent file creation is the failure mode).
  - `test_split_brain_refuses_when_tombstone_present`: same for the
    tombstone branch.
  - `test_bootstrap_path_preserved_without_checkpoint`: regression
    proving the V1.5 bootstrap path still creates a fresh SQLite when
    no prior-migration evidence is on disk.
  - `test_split_brain_guard_does_not_block_existing_source`: the
    resume-after-crash window (source SQLite present alongside the
    sentinel) must not refuse — `db.connect` is not responsible for
    the resume; `migrate_repo_local` is.

Closes gemini A2.

### F-lock — exclusive flock on `migrate-repo-local`

- `src/striatum/daemon_pg/repo_local_migration.py`:
  - Existing `MigrationInProgressError(StriatumError)` and
    `_exclusive_migrate_lock(repo)` from the scaffold.
  - **Exit code realigned with the synthesis** —
    `MigrationInProgressError.exit_code` is now `8` (the V1.5
    `migrate-repo-local` refusal code), not `14`. The synthesis says
    explicitly: "avoid introducing a new exit code for this narrow
    V1.6 slice." No registry change required.
  - Lock-acquisition refusal message now names the source SQLite path
    (`.striatum/state.sqlite3`) in addition to the sidecar lock path,
    so the operator can correlate the refusal to the repo's state file
    (matches the synthesis acceptance text "naming the source file").
  - Lock is taken on a sidecar (`state.sqlite3.migrate.lock`) rather
    than the source SQLite itself so it (a) does not fight SQLite's own
    POSIX byte-range locks, and (b) survives the source-file rename
    during finalization. The docstring captures the rationale.
- `tests/daemon_pg/test_repo_local_migration_locking.py` (this pass —
  new file):
  - `test_lock_contention_refuses_with_exit_code_8`: external holder
    on the sidecar → `migrate_repo_local` refuses with
    `MigrationInProgressError(exit_code=8)` and a `migrate_in_progress`
    message that names `state.sqlite3`.
  - `test_lock_released_after_context_manager_exit` and
    `..._after_context_manager_exception`: prove the flock is released
    on normal exit and on exception so subsequent invocations succeed.
  - `test_lock_path_is_sidecar_not_source`: asserts the lock path is
    the sidecar, not the source SQLite.
  - `test_concurrent_invocation_via_forked_process`: forks a child that
    holds the lock and asserts the parent's `migrate_repo_local` call
    refuses — the synthesis acceptance shape ("forks two processes and
    asserts one wins, one refuses").
  - `test_lock_acquire_fast_path_does_not_clobber_existing_lock`:
    defensive — proves `_exclusive_migrate_lock` does not silently
    acquire when another fd already holds `LOCK_EX` (regression for the
    correct flag combination).
  - `test_external_helpers_self_check`: smoke-checks the test helpers
    themselves so a regression in `_hold_external_lock` doesn't quietly
    turn the contention tests into no-ops.
- `CHANGELOG.md`: V1.6 F-lock entry updated to record exit code 8 and
  the sidecar/rename-survival rationale.

Closes gemini A3.

### F-help — per-flag help on `migrate-repo-local`

- `src/striatum/cli/parser.py` (already in scaffold): `description=`
  on the `migrate-repo-local` subparser plus `help=` on every flag:
  `--from`, `--to`, `--repo`, `--postgres-url`, `--dry-run`,
  `--confirm-delete`, `--keep-sqlite-readonly`,
  `--no-keep-sqlite-readonly`, `--json`. Help texts cover the
  synthesis-named operator hooks: explicit `(default)` on
  `--keep-sqlite-readonly`, `STRIATUM_DAEMON_DB_URL` on
  `--postgres-url`, and the `--confirm-delete` requirement on the
  delete-mode flag.
- `tests/cli/test_parser_help.py` (this pass — new file):
  - `test_help_includes_description`: subparser description mentions
    SQLite → PostgreSQL.
  - `test_help_documents_every_migrate_flag`: every flag named in the
    synthesis appears in the `--help` output.
  - `test_help_mentions_default_and_env_and_confirm_delete`: substring
    coverage for `(default)`, `STRIATUM_DAEMON_DB_URL`, and
    `--confirm-delete`.

Closes claude dogfood-050 F-dx-1.

## Tests

Recommended runs for the reviewer:

- `make lint`
- `make typecheck`
- `.venv/bin/pytest -m "not multi_repo" tests/exit_codes/test_rfc0043_refusals.py tests/exit_codes/test_rfc0043_split_brain.py tests/cli/test_parser_help.py tests/daemon_pg/test_repo_local_migration_locking.py`
- `make test` (`pytest -m "not multi_repo"` is the implicit selector for
  the touched modules; `multi_repo` Postgres tests are out-of-band).

### Implementer test-run status

The implementer session ran inside a permissioned harness where
`bash` invocations of `.venv/bin/pytest`, `.venv/bin/ruff`,
`.venv/bin/mypy`, `make`, and `striatum` all returned
"requires approval" without an approver being present. None of the
gates were executed in this session. The reviewer must re-run the
three suites named above before publishing an accepting verdict; the
code changes here are scoped narrowly (one new constant export in
`daemon_required.py`, one exit-code re-typing in
`repo_local_migration.py`, and four new test files) so the failure
modes to look for are:

1. `tests/exit_codes/test_rfc0043_refusals.py` — confirm the autouse
   `_clear_daemon_required_env` clears `STRIATUM_TEST_HARNESS` (its
   absence would mask the new bare-env regression).
2. `tests/daemon_pg/test_repo_local_migration_locking.py` — confirm
   `MigrationInProgressError.exit_code == 8` end-to-end and that the
   forked-process test does not leak a child on failure.
3. `tests/exit_codes/test_rfc0043_split_brain.py` — confirm no fresh
   `state.sqlite3` is created when the guard fires (the test asserts
   `not db_path(tmp_path).exists()`).
4. `tests/cli/test_parser_help.py` — confirm the `(default)` substring
   is present in the rendered `--help` output (argparse formatting can
   wrap long help strings; the substring check is whitespace-tolerant).

The new tests do not require Postgres and are designed to pass under
`pytest -m 'not multi_repo'`.

## Deviations from the prompt

The prompt asks for behavior that the synthesis revised; this build
follows the synthesis (the validated design):

- **Exit code for F-lock contention**: prompt says 14, synthesis says
  8. Build uses 8 — quoting the synthesis: "avoid introducing a new
  exit code for this narrow V1.6 slice." `CHANGELOG.md` and the
  HANDOFF reflect 8.
- **F-lock message**: prompt says "naming the holding pid", synthesis
  says "naming the source file". Build names the source file. The
  `fcntl.flock` API does not expose the holding pid portably (would
  require `/proc/locks` parsing on Linux only); the synthesis text
  takes precedence.
- **F-split-brain check**: prompt also mentions checking the daemon
  registry; synthesis says "Do not query the daemon registry from
  `striatum.db` in V1.6; the filesystem sentinels cover the
  accidental split-brain case without adding RPC or Postgres coupling
  to the low-level SQLite connector." Build checks sentinel/tombstone
  only.
- **F-lock test file name**: prompt says
  `tests/daemon_pg/test_repo_local_migration_concurrent.py`; synthesis
  says `tests/daemon_pg/test_repo_local_migration_locking.py`. Build
  uses the synthesis name.
- **Gemini A1 (daemon-side single-repo business logic on Postgres)** is
  intentionally **not** shipped in V1.6 — it is V2.0 scope per the
  synthesis ("V2.0 because it requires method-by-method Postgres
  implementations, compatibility fixtures, daemon RPC coverage, and
  retirement of `striatum.api.invoke` delegation").

## Sub-agents

The prompt suggests "one per F dispatched in parallel" but the four
edits are tightly coupled (shared exit-code constants, a shared
conftest, overlapping test-file conventions), so a single
implementer pass kept the cross-F invariants (exit-code choice,
synthesis vs prompt deltas, message shape) coherent rather than
serializing them across sub-agents. The synthesis itself names a
single-track implementation order
(`daemon_required.py` → `conftest.py` → `errors.py?` → `db.py` →
`repo_local_migration.py` → `parser.py` → CHANGELOG), which this
build follows.

## V2.0 follow-ups

- Daemon-side substrate migration (gemini A1) — the substrate flip at
  the daemon RPC business-logic layer; full multi-week RFC.
- Postgres-backed `repo_migrations` row check from
  `enforce_daemon_required` (replacing the filesystem-signal
  pre-cutover stand-in in `repo_is_migrated`).
- An optional `holding_pid` reader for the F-lock refusal message on
  Linux (parsing `/proc/locks`) — synthesis defers this; the source-
  file-name message is sufficient for V1.6.
