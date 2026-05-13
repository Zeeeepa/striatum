# Design — RFC 0043 V1.6 substrate completion

Read in order:
- `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md`
- `docs/dogfood/050/review/build/codex/REVIEW.md` (escape path finding)
- `docs/dogfood/050/review/build/gemini/REVIEW.md` (A1/A2/A3)
- `docs/dogfood/050/review/build/claude/REVIEW.md` (F-dx-1 help text)
- `CHANGELOG.md` v1.38.0 "Known follow-ups (V1.6)" section
- `src/striatum/cli/daemon_required.py`, `src/striatum/cli/dispatch.py`,
  `src/striatum/db.py`, `src/striatum/daemon_pg/repo_local_migration.py`,
  `src/striatum/cli/parser.py` to ground the changes

Design closure of the V1.6 deltas that **are in scope** for V1.6. The
gemini A1 finding (daemon-side substrate migration — full single-repo
business logic on Postgres) is **deferred to V2.0** as a separate
multi-week RFC; do not design it here.

In-scope V1.6:

- **F-escape:** Remove the `STRIATUM_DAEMON_REQUIRED=0` runtime opt-out
  for production callers. Keep it only as a test-harness gate, accepted
  exclusively when an explicit pytest marker / `STRIATUM_TEST_HARNESS=1`
  is also present. Edit `src/striatum/cli/daemon_required.py` so the
  opt-out is conditional. Update `tests/conftest.py` to set the test
  marker. Closes codex threat-model finding from dogfood-050.
- **F-split-brain:** `src/striatum/db.connect` must refuse to create a
  fresh SQLite when a migration checkpoint
  (`.striatum/state.sqlite3.migrated` or migration row in the daemon
  registry) exists. Raise a typed error → CLI returns exit code 12
  (`repo_not_migrated`) with the same remediation message. Closes
  gemini A2.
- **F-lock:** `striatum daemon migrate-repo-local` must take an
  exclusive `flock` on `.striatum/state.sqlite3` for the duration of
  the migration. Concurrent migrate-repo-local invocations must refuse
  with a clear error rather than racing. Closes gemini A3.
- **F-help:** Per-flag `help=` text on every `migrate-repo-local`
  argparse flag (`--from`, `--to`, `--repo`, `--postgres-url`,
  `--dry-run`, `--confirm-delete`, `--keep-sqlite-readonly`,
  `--no-keep-sqlite-readonly`, `--json`). Closes claude F-dx-1.

Out of scope (V2.0): full daemon-side single-repo business logic on
Postgres (gemini A1).

Output: design proposal listing files to touch, key code sketches,
acceptance verifiers. 600-1200 words.
