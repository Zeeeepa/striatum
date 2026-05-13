---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

author: designer-unknown-model-002

# DESIGN SYNTHESIS — RFC 0043 V1.6 substrate completion

## Decisions

V1.6 should close the four narrow follow-ups from dogfood-050 without
claiming the full Postgres substrate port is done. Gemini A1 is correct:
daemon-side single-repo business logic still delegates too much through the
SQLite-backed path. That is V2.0 scope because it requires method-by-method
Postgres implementations, compatibility fixtures, daemon RPC coverage, and
retirement of `striatum.api.invoke` delegation. V1.6 only hardens the boundary
so the remaining SQLite code cannot be used accidentally or as a production
escape hatch.

For F-escape, use the shared codex/gemini approach and the claude note that
the resolver may already be half-wired: `STRIATUM_DAEMON_REQUIRED=0` is honored
only when `STRIATUM_TEST_HARNESS=1` is also present. Bare runtime use of the
old env var must still enforce daemon-required mode. The operator migration
story is daemon start plus `striatum daemon migrate-repo-local`, not SQLite
fallback.

For F-split-brain, use local migration evidence in `db.connect()` before it
creates a fresh SQLite file. If `.striatum/state.sqlite3` is absent and either
`.striatum/state.sqlite3.migrated` or `.striatum/state.sqlite3.tombstone`
exists, raise a typed Striatum error with exit code 12 and the same
`repo_not_migrated` remediation shape used by the daemon-required gate. Do not
query the daemon registry from `striatum.db` in V1.6; the filesystem sentinels
cover the accidental split-brain case without adding RPC or Postgres coupling
to the low-level SQLite connector.

For F-lock, take an exclusive non-blocking `fcntl.flock` on the source
`.striatum/state.sqlite3` for the whole migration copy and finalization path.
Dry-run may take the same lock for a simple "one migration command inspects or
migrates at a time" rule. Refuse lock contention with exit code 8 and a clear
message naming the source file; avoid introducing a new exit code for this
narrow V1.6 slice.

For F-help, add the claude DX review polish: a subparser description and help
text for every `migrate-repo-local` flag, including explicit `(default)` on
`--keep-sqlite-readonly`, `--confirm-delete` on the delete path, and
`STRIATUM_DAEMON_DB_URL` on `--postgres-url`.

## Implementation order

Edit `src/striatum/cli/daemon_required.py` first: add or confirm
`ENV_TEST_HARNESS`, tighten `resolve_requirement()`, and keep optional
commands unchanged. Then edit `tests/conftest.py` so the legacy SQLite fixture
sets both env vars, followed by `tests/exit_codes/test_rfc0043_refusals.py` to
cover bare env-var enforcement and paired test-harness opt-out.

Next edit `src/striatum/errors.py` only if a reusable typed error or
remediation helper is needed, then `src/striatum/db.py` for the sentinel /
tombstone guard before `sqlite3.connect(target)` can create a new file. Add the
split-brain regression tests in `tests/exit_codes/test_rfc0043_split_brain.py`
or `tests/test_db_split_brain.py`; keep the assertions focused on no fresh
SQLite file creation and exit code 12.

Then edit `src/striatum/daemon_pg/repo_local_migration.py`: add the
standard-library `fcntl` lock context manager and wrap the full migration
body, including sentinel write and tombstone/delete finalization. Add
`tests/daemon_pg/test_repo_local_migration_locking.py`.

Finally edit `src/striatum/cli/parser.py` for help strings, update or add the
parser help smoke test, and add the CHANGELOG note that V1.6 hardens the
boundary while A1 remains V2.0 work.

## Acceptance

`tests/exit_codes/test_rfc0043_refusals.py` must catch F-escape: bare
`STRIATUM_DAEMON_REQUIRED=0` returns an enforced `DaemonRequirement`, while the
paired test-harness env vars return `None`. The real dispatch refusal test
should continue to assert exit 11 or 12 as appropriate instead of falling
through to SQLite.

The split-brain tests must catch missing `state.sqlite3` plus `.migrated`,
missing `state.sqlite3` plus `.tombstone`, and missing `state.sqlite3` with no
checkpoint. The first two refuse with exit code 12 and do not create
`state.sqlite3`; the last preserves bootstrap behavior under the test harness.

`tests/daemon_pg/test_repo_local_migration_locking.py` must catch F-lock by
holding an external `fcntl.LOCK_EX` and asserting `migrate_repo_local()` refuses
with exit code 8, plus a regression proving the lock is released on success or
exception.

The parser help test must run `striatum daemon migrate-repo-local --help` and
assert every flag appears with distinguishing text: `default`,
`STRIATUM_DAEMON_DB_URL`, and `confirm-delete`. Full acceptance is `make lint`,
`make typecheck`, and the relevant pytest files above passing before the
implementer reports the V1.6 build complete.
