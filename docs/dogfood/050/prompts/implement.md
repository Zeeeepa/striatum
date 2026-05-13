# Implement: RFC 0043 V1.5 (claude — close D102 follow-up findings)

Blocked until `review_design` returns an accepting verdict.

Implement RFC 0043 V1.5 per `docs/dogfood/050/DESIGN_SYNTHESIS.md`. **You are the claude lane implementer** — deliberately NOT codex, to avoid the 5-time codex/codex anti-pattern (D102 cycle-exhaustion override flagged this risk). You write Python under `src/striatum/` and tests under `tests/`.

**Your scope:**

- `src/striatum/daemon_pg/repo_local_migration.py` — F-crash: add the crash-recovery resume path per synthesis (sentinel-based checkpoint OR transactional rollback). The `already_migrated` early-return must finish the tombstone idempotently if the SQLite source still exists.
- `src/striatum/cli/daemon_required.py` — F-escape: flip `resolve_requirement` to daemon-required by default. Either retain `STRIATUM_DAEMON_REQUIRED=0` as an opt-OUT or remove the env var entirely per synthesis. Audit the file for any other silent-SQLite-fallback paths.
- `src/striatum/cli/parser.py` — F-parser: add the `migrate-repo-local` subparser under the `daemon` subparser with `--confirm-delete`, `--keep-sqlite-readonly`, `--dry-run` flags per synthesis.
- `src/striatum/cli/dispatch.py` — F-parser: add the dispatch arm in `_dispatch_daemon` that routes `migrate-repo-local` to `src/striatum/cli/daemon.py`. Confirm no stale silent-SQLite-fallback path remains.
- `src/striatum/cli/daemon.py` — adjust helper signatures only if synthesis requires.
- `src/striatum/daemon_pg/sql/` — additive only. If the synthesis requires a new table or column, add a new file `0006_*.sql` and bump `LATEST_DAEMON_DB_VERSION`. No `DROP COLUMN`, no destructive index changes.
- `src/striatum/errors.py` — only if a new error class is required (unlikely; `DaemonUnreachableError` / `RepoNotMigratedError` already exist).
- `tests/exit_codes/test_rfc0043_refusals.py` OR `tests/cli/test_unmigrated_repo_refusal.py` — F-test: the e2e exit-12 assertion per synthesis.
- `tests/daemon_pg/test_repo_local_migration.py` — F-crash regression test (simulate crash via `_tombstone_or_delete_state_db` raising mid-call, assert resume completes tombstone).
- `tests/cli/test_daemon_required_default.py` (new, if needed) — F-escape regression test (assert daemon-required is on without setting the env var).
- `Makefile` — only if synthesis adds a new `make test-*` target; do not rename existing targets.
- `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md` — append a "V1.5 deltas" section summarizing F-crash / F-escape / F-parser / F-test. Do not rewrite V1 sections.
- `docs/dogfood/050/build/HANDOFF.md` — handoff summarizing shipped scope, files touched, test results, deviations from synthesis (if any) with one-line rationale.

**Use sub-agents aggressively** — one per finding, dispatched in parallel:

- Sub-agent F-crash: crash-recovery resume in `repo_local_migration.py` + regression test.
- Sub-agent F-escape: default-flip in `daemon_required.py` + audit other cli/ paths + regression test.
- Sub-agent F-parser: parser subparser + dispatch arm + smoke test for `--help`.
- Sub-agent F-test: e2e exit-12 test + (optional sibling exit-11 test).

Reconcile sub-agent outputs yourself before writing HANDOFF. Respect the implementation order the synthesis locks (e.g. F-parser before F-test if the test invokes the subcommand).

**Do NOT write to**: anything outside `allowed_paths`. **No README / TODO / CHANGELOG / RFC index / SPEC / HOW_TO updates** — the operator handles those manually after the dogfood lands (no in-workflow consolidate job; dogfood-042 cascade lesson).

Backward-compat (hard constraint):
- Postgres schema changes additive only.
- `--keep-sqlite-readonly` tombstone (rename to `.tombstone`, mode 0444) must keep working under the new crash-recovery resume path. Cover it with a test.

Verification: `make lint`, `make typecheck`, `make test` all pass. The new exit-12 e2e test passes. The F-crash regression test fails on the un-fixed code and passes on the fixed code (assert this in HANDOFF).

One-shot supervised invocation. Do not ask follow-ups. If `striatum ack` is denied, write the HANDOFF and exit normally.
