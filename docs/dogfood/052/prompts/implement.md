# Implement — RFC 0043 V1.6 substrate hardening

Blocked until `review_design` returns an accepting verdict.

Implement V1.6 substrate deltas per
`docs/dogfood/052/DESIGN_SYNTHESIS.md`. Claude implementer (validated
choice from dogfood-050).

**Write scope:** `src/striatum/`, `tests/`,
`docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md`,
`docs/dogfood/052/build/`. No writes to `.striatum/`, `go/`,
`docs/dogfood/049/`, `docs/dogfood/050/`, `docs/dogfood/051/`.

**F-by-F:**

- **F-escape:** In `src/striatum/cli/daemon_required.py`,
  `resolve_requirement` returns `None` only when (a) the command is in
  `DAEMON_OPTIONAL_COMMANDS` OR (b) `STRIATUM_DAEMON_REQUIRED == "0"`
  AND `STRIATUM_TEST_HARNESS == "1"`. The bare opt-out (without test
  harness) now enforces. Update `tests/conftest.py` to export the test
  harness env var. Add regression in
  `tests/exit_codes/test_rfc0043_refusals.py` asserting that
  `STRIATUM_DAEMON_REQUIRED=0` without `STRIATUM_TEST_HARNESS=1` still
  refuses with exit 11.
- **F-split-brain:** In `src/striatum/db.py`'s connect/ensure_initialized
  path, before creating a fresh SQLite, check for the migration
  checkpoint sentinel
  (`.striatum/state.sqlite3.migrated`) or the migration row in the
  daemon registry. If either exists and the source SQLite is absent,
  raise `RepoMigratedError` and propagate to exit code 12 with the
  remediation text "this repo was migrated to Postgres; run striatum
  daemon migrate-repo-local --from sqlite --to pg --repo …".
  Add regression in `tests/daemon_pg/` exercising both the sentinel
  and the registry path.
- **F-lock:** In `src/striatum/daemon_pg/repo_local_migration.py`,
  wrap the migration body in an `flock` exclusive lock on the source
  SQLite file. Acquire before opening Postgres tx; release after
  finalization. Concurrent invocation must refuse with exit code 14
  (`migrate_in_progress`) and a clear message naming the holding pid.
  Regression test in
  `tests/daemon_pg/test_repo_local_migration_concurrent.py` forks two
  processes and asserts one wins, one refuses.
- **F-help:** In `src/striatum/cli/parser.py::register_migrate_repo_local`,
  add `help=` to every flag (`--from`, `--to`, `--repo`,
  `--postgres-url`, `--dry-run`, `--confirm-delete`,
  `--keep-sqlite-readonly`, `--no-keep-sqlite-readonly`, `--json`).
  Add `description=` to the subparser. Verify by snapshot test in
  `tests/cli/test_parser_help.py`.

**Tests:** make lint, make typecheck, pytest -m "not multi_repo"
clean.

**HANDOFF:** `docs/dogfood/052/build/HANDOFF.md` with byline matching
expected_author_line. Summarize shipped scope, test commands,
deviations.

**Sub-agents:** one per F dispatched in parallel.
