author: designer-unknown-model-001

# RFC 0043 V1.6 Design

Status: proposed handoff
Date: 2026-05-13

## Scope

V1.6 should close four narrow gaps from dogfood-050 without pretending to
finish the full substrate migration. The daemon-side single-repo business
logic port identified by Gemini A1 is real, but it is a V2.0 RFC-sized body of
work: method-by-method Postgres implementations, compatibility harnesses, and
retirement of SQLite-backed `striatum.api.invoke` delegation. V1.6 instead
hardens the transition boundary so the remaining SQLite code cannot silently
create new state or remain available as an operator escape hatch.

## F-escape: test-only opt-out

Touch `src/striatum/cli/daemon_required.py` and `tests/conftest.py`.
`resolve_requirement()` should enforce daemon-required mode unless the command
is explicitly daemon-optional or the process is running under the test harness.
The design is deliberately two-key: `STRIATUM_DAEMON_REQUIRED=0` is ignored by
normal runtime callers unless `STRIATUM_TEST_HARNESS=1` is also present.

Code shape:

```python
ENV_TEST_HARNESS = "STRIATUM_TEST_HARNESS"

if (
    os.environ.get(ENV_DAEMON_REQUIRED) == "0"
    and os.environ.get(ENV_TEST_HARNESS) == "1"
):
    return None
```

`tests/conftest.py` should set both environment variables in the session
fixture, and restore both on teardown. The exit-code tests should delete both
vars when asserting production behavior. Update comments and docstrings that
still describe the bare env var as an operator migration path; the user-facing
remediation is now only: start the daemon, then run `striatum daemon
migrate-repo-local`.

Verifier: update `tests/exit_codes/test_rfc0043_refusals.py` so
`test_resolve_requirement_opt_out_with_env_zero` becomes two tests: bare
`STRIATUM_DAEMON_REQUIRED=0` still returns a `DaemonRequirement`, while
`STRIATUM_DAEMON_REQUIRED=0` plus `STRIATUM_TEST_HARNESS=1` returns `None`.

## F-split-brain: refuse fresh SQLite after checkpoint

Touch `src/striatum/db.py`, `src/striatum/errors.py`, and targeted tests. The
bug is in `connect(repo)`: `sqlite3.connect(target)` creates a new empty
database when `state.sqlite3` is missing. After a successful migration, that is
a split-brain foot-gun because daemon-delegated legacy logic can recreate
SQLite beside Postgres.

Before opening SQLite, `connect()` should detect migration evidence:

- `.striatum/state.sqlite3.migrated` exists; or
- `.striatum/state.sqlite3.tombstone` exists; or
- a daemon registry checkpoint exists, when a cheap lookup is possible through
  the configured Postgres URL.

The local filesystem checks are mandatory and dependency-free. The registry
lookup can be best-effort in V1.6 to avoid making every SQLite test require
Postgres; absence of `STRIATUM_DAEMON_DB_URL` should not mask the sentinel or
tombstone checks.

Code shape:

```python
def _migration_checkpoint_exists(repo: Path) -> bool:
    state = state_dir(repo)
    return (
        (state / "state.sqlite3.migrated").exists()
        or (state / "state.sqlite3.tombstone").exists()
        or _daemon_repo_checkpoint_exists(repo)
    )

def connect(repo: Path) -> sqlite3.Connection:
    target = db_path(repo)
    if not target.exists() and _migration_checkpoint_exists(repo):
        raise RepoNotMigratedError(render_repo_not_migrated_message(repo), ...)
    ...
```

To avoid a CLI import cycle, put the remediation string builder somewhere
neutral (`errors.py` or a small `migration_refusals.py`) and let
`daemon_required.py` reuse it. The raised typed error must keep exit code 12.

Verifier: add `tests/exit_codes/test_rfc0043_split_brain.py` cases for missing
`state.sqlite3` plus sentinel, missing `state.sqlite3` plus tombstone, and
missing `state.sqlite3` with no checkpoint. The first two refuse with code 12;
the last preserves legacy test behavior when the test harness opt-out is set.

## F-lock: exclusive source SQLite lock

Touch `src/striatum/daemon_pg/repo_local_migration.py`. The migration must hold
an exclusive advisory file lock on `.striatum/state.sqlite3` for the whole
copy-and-finalize interval. Use standard-library `fcntl.flock` on POSIX; this
project targets Unix-like local workflows today, and adding a dependency for
Windows portability is not justified in this narrow fix.

Wrap the existing migration body:

```python
@contextmanager
def _exclusive_source_lock(source_path: Path):
    with source_path.open("rb") as handle:
        try:
            fcntl.flock(handle.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            raise StriatumError(
                f"repo-local SQLite migration already in progress: {source_path}",
                exit_code=8,
            ) from exc
        try:
            yield
        finally:
            fcntl.flock(handle.fileno(), fcntl.LOCK_UN)
```

`migrate_repo_local()` should acquire the lock after delete-option validation
and before `_open_source_readonly()`, then keep it through `_migrate_full()` and
the sentinel/tombstone/delete finalization. Dry-run may also take the shared or
exclusive lock; prefer exclusive for simpler semantics and to keep the operator
signal consistent: only one migration command inspects or migrates at a time.

Verifier: add `tests/daemon_pg/test_repo_local_migration_locking.py` with a
manual `fcntl.LOCK_EX` held by the test process, then assert
`migrate_repo_local()` refuses clearly. Also add a regression where the
monkeypatched `_copy_repo_rows` checks that the lock is still held while the
copy runs.

## F-help: complete operator help

Touch `src/striatum/cli/parser.py`. The subparser should have a description,
and every `migrate-repo-local` flag needs `help=` text:
`--from`, `--to`, `--repo`, `--postgres-url`, `--dry-run`,
`--keep-sqlite-readonly`, `--no-keep-sqlite-readonly`, `--confirm-delete`, and
`--json`. The `--keep-sqlite-readonly` line must state `(default)`; the delete
line must state that it requires `--confirm-delete`.

Verifier: extend the parser/help smoke test to call
`striatum daemon migrate-repo-local --help`, assert exit 0, and assert each
flag name plus at least one distinguishing phrase (`default`,
`STRIATUM_DAEMON_DB_URL`, `confirm-delete`) is present.

## Acceptance

V1.6 is accepted when bare runtime `STRIATUM_DAEMON_REQUIRED=0` no longer
bypasses daemon enforcement, migrated repos cannot recreate fresh SQLite,
concurrent migrations fail closed under an exclusive source lock, help output
is self-explanatory, and the CHANGELOG names A1 as deferred to V2.0 rather than
closed.
