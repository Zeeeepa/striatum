# Implementer Role (Dogfood 050 — claude, Python + tests)

Single implementer, **claude lane** — deliberately not codex, to avoid
the 5-time codex/codex anti-pattern from prior cascades. D102's
cycle-exhaustion override already noted this risk; do not reopen it.
The workflow validator enforces the write scope — stay strictly inside
your job's `write_scope.allowed_paths`.

Owns:

- `src/striatum/daemon_pg/repo_local_migration.py` — F-crash: add the
  crash-recovery resume path per synthesis. The `already_migrated`
  early-return must finish the tombstone idempotently if the SQLite
  source still exists on disk.
- `src/striatum/cli/daemon_required.py` — F-escape: flip
  `resolve_requirement` so daemon-required is the default. Per
  synthesis, retain `STRIATUM_DAEMON_REQUIRED=0` as a test-only
  opt-OUT or remove the env var entirely. Audit the file for any other
  silent SQLite fallback.
- `src/striatum/cli/parser.py` — F-parser: add the
  `migrate-repo-local` subparser under `daemon` with `--confirm-delete`,
  `--keep-sqlite-readonly`, `--dry-run` flags per synthesis.
- `src/striatum/cli/dispatch.py` — F-parser: add the dispatch arm in
  `_dispatch_daemon` that routes `migrate-repo-local` to
  `src/striatum/cli/daemon.py`. Confirm no stale silent SQLite fallback
  remains.
- `src/striatum/cli/daemon.py` — adjust helper signatures only if
  synthesis requires.
- `src/striatum/daemon_pg/sql/` — additive only. If synthesis adds a
  table/column, new file `0006_*.sql` and bump
  `LATEST_DAEMON_DB_VERSION`. No `DROP COLUMN`, no destructive index
  changes.
- `src/striatum/errors.py` — only if a new error class is required.
- `tests/exit_codes/test_rfc0043_refusals.py` OR
  `tests/cli/test_unmigrated_repo_refusal.py` — F-test: the e2e exit-12
  assertion per synthesis.
- `tests/daemon_pg/test_repo_local_migration.py` — F-crash regression
  test (simulate crash via `_tombstone_or_delete_state_db` raising
  mid-call, assert resume completes tombstone).
- `tests/cli/test_daemon_required_default.py` (new, if needed) —
  F-escape regression test.
- `Makefile` — only if synthesis adds a new `make test-*` target; do
  not rename existing targets.
- `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md`
  — append a "V1.5 deltas" section. Do not rewrite V1 sections.
- `docs/dogfood/050/build/HANDOFF.md` — handoff.

Use sub-agents aggressively — one per finding in parallel (F-crash,
F-escape, F-parser, F-test). Reconcile outputs yourself before writing
HANDOFF. Respect the implementation order the synthesis locks.

**Backward-compat callout (non-negotiable in HANDOFF)**:
- Any new SQL file is additive-only — call out which file and what
  columns/tables it adds.
- `--keep-sqlite-readonly` tombstone still works under the new
  crash-recovery resume path — cover with a test.

**Do NOT write to**: anything outside `allowed_paths`. No
`docs/rfcs/README.md`, `docs/TODO.md`, `CHANGELOG.md`, `docs/SPEC.md`,
`docs/HOW_TO_*.md`, `docs/UBIQUITOUS_LANGUAGE.md` — operator handles
those manually (dogfood-042 cascade lesson).

One-shot supervised invocation. Do not ask follow-ups. If `striatum
ack` is denied, write the artifact and exit normally. Lease can expire
if `make test` exceeds ~30 min — prefer focused pytest first. Per
D089/D091, OPERATOR_REPORT.md is the operator's responsibility, not
yours.
