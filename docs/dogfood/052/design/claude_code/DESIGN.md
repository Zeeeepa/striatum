# Design — RFC 0043 V1.6 substrate completion (claude lane)

author: designer-unknown-model-001

## Scope

Four in-scope V1.6 deltas. The gemini A1 daemon-side single-repo
business-logic port is out of scope (deferred to V2.0).

- **F-escape** — remove the production `STRIATUM_DAEMON_REQUIRED=0`
  bypass; keep it only when the test-harness marker is also present.
- **F-split-brain** — `striatum.db.connect` refuses to create a fresh
  SQLite when a migration checkpoint exists.
- **F-lock** — `striatum daemon migrate-repo-local` takes an exclusive
  `flock` on the source `state.sqlite3` for the duration of the run.
- **F-help** — per-flag `help=` text on every `migrate-repo-local`
  argparse flag plus a subparser `description=`.

## F-escape — production opt-out removal

### Files

- `src/striatum/cli/daemon_required.py`
- `tests/conftest.py`
- `tests/exit_codes/test_rfc0043_refusals.py`

### Change

`resolve_requirement` already pairs `STRIATUM_DAEMON_REQUIRED=0` with
`STRIATUM_TEST_HARNESS=1` (`daemon_required.py:65-72`), but the test
suite's session fixture sets only the former (`tests/conftest.py:13-31`).
The current state means *every* test invocation today silently fails to
opt out — the bypass is half-wired. V1.6 finishes both halves:

1. Confirm the resolver requires both env vars and add a docstring line
   naming the test-harness marker as the sole allowed opt-out path.
2. Update `tests/conftest.py` to set `STRIATUM_TEST_HARNESS=1` alongside
   the existing `STRIATUM_DAEMON_REQUIRED=0`, restoring the session
   fixture's intent.
3. Delete the dogfood-050 HANDOFF language documenting the env var as
   an "operator migration path" — that survives only in the CHANGELOG
   v1.38.0 "Known follow-ups (V1.6)" entry, which the build job will
   roll forward.

```python
# tests/conftest.py
os.environ["STRIATUM_DAEMON_REQUIRED"] = "0"
os.environ["STRIATUM_TEST_HARNESS"] = "1"
```

### Acceptance verifier

`tests/exit_codes/test_rfc0043_refusals.py`:

- `test_resolve_requirement_bare_env_zero_no_longer_opts_out` — set
  `STRIATUM_DAEMON_REQUIRED=0`, leave `STRIATUM_TEST_HARNESS` unset,
  assert `resolve_requirement("status")` returns a populated
  `DaemonRequirement` (enforcement still active).
- `test_resolve_requirement_opt_out_requires_test_harness_marker` —
  both env vars set, assert `None`.

## F-split-brain — refuse fresh SQLite when migrated

### Files

- `src/striatum/db.py`
- `src/striatum/errors.py`
- `src/striatum/cli/dispatch.py`
- `tests/test_db_split_brain.py` (new)

### Change

Today `db.connect` opens a fresh `sqlite3.connect(target)` even when
`target` does not exist — the daemon-required gate normally catches
this earlier, but the test-harness opt-out path and any internal helper
that bypasses dispatch (e.g. `striatum.api.invoke` per gemini A1) still
hits it. After RFC 0043 migration, the absence of `state.sqlite3` is
the *correct* steady state; recreating it silently is the split-brain.

The fix is a guarded create. `connect()` checks for a migration
checkpoint signal before creating a new file. Two signals count:

1. `.striatum/state.sqlite3.migrated` sentinel (already written by
   `migrate_repo_local`).
2. `.striatum/state.sqlite3.tombstone` (already produced by the
   `--keep-sqlite-readonly` finalization).

```python
# src/striatum/db.py
def connect(repo: Path) -> sqlite3.Connection:
    target = db_path(repo)
    already_existed = target.exists()
    if not already_existed and _migration_checkpoint_exists(repo):
        raise RepoMigratedError(
            f"refusing to create fresh SQLite at {target}; "
            f"repo already migrated to daemon PostgreSQL state",
            hint=(
                f"run striatum daemon migrate-repo-local --from sqlite "
                f"--to pg --repo {repo} to inspect status"
            ),
        )
    conn = sqlite3.connect(target)
    ...

def _migration_checkpoint_exists(repo: Path) -> bool:
    sd = state_dir(repo)
    return (sd / "state.sqlite3.migrated").exists() or (
        sd / "state.sqlite3.tombstone"
    ).exists()
```

`RepoMigratedError` is a new `StriatumError` subclass in
`src/striatum/errors.py` with `exit_code=12` (reusing the existing
`repo_not_migrated` exit code — the operator-visible refusal shape is
identical: the daemon owns this repo's state, the CLI should not be
opening SQLite). The dispatcher already handles `StriatumError` exit
codes; no dispatch wiring is required.

Out of scope (V2.0): authoritative daemon-registry checkpoint lookup
(gemini A2 recommended this). V1.6 stays filesystem-local so the helper
works without a daemon RPC round-trip on every connect.

### Acceptance verifier

`tests/test_db_split_brain.py`:

- `test_connect_refuses_when_sentinel_exists` — `tmp_path/.striatum/`
  with a `state.sqlite3.migrated` file but no SQLite; `db.connect(tmp_path)`
  raises `RepoMigratedError` with `exit_code == 12`.
- `test_connect_refuses_when_tombstone_exists` — same with
  `state.sqlite3.tombstone`.
- `test_connect_succeeds_when_neither_exists` — empty `.striatum/`,
  `db.connect` returns a working connection (the V1 init bootstrap path).

## F-lock — exclusive flock during migrate-repo-local

### Files

- `src/striatum/daemon_pg/repo_local_migration.py`
- `src/striatum/errors.py`
- `tests/daemon_pg/test_repo_local_migration_lock.py` (new)

### Change

`migrate_repo_local` opens the source with `mode=ro` (gemini A3 noted
the gap). Concurrent invocations race on the Postgres side and silently
clobber each other's finalization (sentinel writes are last-writer-wins).
V1.6 takes a non-blocking `fcntl.flock(LOCK_EX | LOCK_NB)` on a
dedicated lock fd over `state.sqlite3` and holds it across the whole
migration body — Postgres transaction, sentinel write, tombstone/delete.

```python
# src/striatum/daemon_pg/repo_local_migration.py
import fcntl

def migrate_repo_local(options):
    ...
    if not options.dry_run and source_path.exists():
        with _exclusive_source_lock(source_path):
            return _migrate_full(...)
    return _migrate_full(...)  # dry-run + post-migration resume paths

@contextmanager
def _exclusive_source_lock(source_path: Path):
    fd = os.open(source_path, os.O_RDONLY)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            raise StriatumError(
                f"migrate-repo-local already in progress on {source_path}",
                exit_code=14,
            ) from exc
        yield
    finally:
        os.close(fd)  # close releases the lock
```

The lock is taken before the `_open_source_readonly` step but after
dry-run / already-migrated short-circuits, so read-only inspection paths
do not block each other. Exit code 14 is new (next free after 13
already used by `migrate-repo-local`'s manifest mismatch); documented
in `docs/CLI_REFERENCE.md` as `migrate_in_progress`.

### Acceptance verifier

`tests/daemon_pg/test_repo_local_migration_lock.py`:

- `test_concurrent_migrate_refuses_with_exit_14` — spawn a goroutine /
  thread that holds `fcntl.flock(fd, LOCK_EX)` on a tmp SQLite, then
  invoke `migrate_repo_local()` and assert `StriatumError` with
  `exit_code == 14` and stderr naming the source path.
- `test_lock_released_on_success` — single-process happy path, assert a
  second flock attempt after the migration succeeds.
- `test_lock_released_on_exception` — monkeypatch `_migrate_full` to
  raise mid-flight; assert subsequent flock attempt succeeds.

## F-help — per-flag help on migrate-repo-local

### Files

- `src/striatum/cli/parser.py` (`daemon_migrate_repo` block, lines 167-199)
- `tests/cli/test_parser_help.py` (new or extend existing)

### Change

Add `description=` on the subparser and `help=` on every flag that
currently lacks one. Append " (default)" / " (off by default)" to the
existing `--keep-sqlite-readonly` / `--no-keep-sqlite-readonly` help
strings so the implicit default is discoverable (per claude F-dx-2 from
dogfood-050).

```python
daemon_migrate_repo = daemon_sub.add_parser(
    "migrate-repo-local",
    help="migrate one repo-local .striatum/state.sqlite3 into daemon PostgreSQL state",
    description=(
        "Convert .striatum/state.sqlite3 in --repo into per-repo Postgres "
        "rows in the daemon-owned database, then either tombstone or "
        "delete the source file. Required: --from sqlite --to pg."
    ),
)
daemon_migrate_repo.add_argument(
    "--from", dest="from_substrate", choices=["sqlite"], required=True,
    help="source substrate; only 'sqlite' is supported today",
)
daemon_migrate_repo.add_argument(
    "--to", dest="to_substrate", choices=["pg"], required=True,
    help="destination substrate; only 'pg' is supported today",
)
daemon_migrate_repo.add_argument(
    "--repo", dest="repo_local_repo", default=None,
    help="target repo path; falls back to top-level --repo",
)
daemon_migrate_repo.add_argument(
    "--postgres-url",
    help="override STRIATUM_DAEMON_DB_URL for this invocation",
)
daemon_migrate_repo.add_argument(
    "--dry-run", action="store_true",
    help="enumerate row counts and manifests without writing to Postgres",
)
daemon_migrate_repo.add_argument(
    "--keep-sqlite-readonly", dest="keep_sqlite_readonly",
    action="store_true", default=True,
    help="rename state.sqlite3 to state.sqlite3.tombstone and chmod 0444 (default)",
)
daemon_migrate_repo.add_argument(
    "--no-keep-sqlite-readonly", dest="keep_sqlite_readonly",
    action="store_false",
    help="delete state.sqlite3 after migration; requires --confirm-delete (off by default)",
)
daemon_migrate_repo.add_argument(
    "--confirm-delete", action="store_true",
    help="required acknowledgement when --no-keep-sqlite-readonly is set",
)
daemon_migrate_repo.add_argument(
    "--json", action="store_true",
    help="emit the result as a structured JSON envelope on stdout",
)
```

### Acceptance verifier

`tests/cli/test_parser_help.py`:

- `test_migrate_repo_local_subparser_help_lists_every_flag` — invoke
  `parser.format_help()` on the `daemon migrate-repo-local` subparser
  and assert each flag name appears with its help string fragment
  (e.g. `"falls back to top-level --repo"` for `--repo`).
- `test_keep_sqlite_readonly_help_mentions_default` — assert the
  `(default)` marker on `--keep-sqlite-readonly` and `(off by default)`
  on `--no-keep-sqlite-readonly`.

## Non-scope notes

- gemini A1 (daemon RPC routes single-repo verbs back to SQLite logic)
  is deferred to V2.0. F-split-brain is the V1.6 mitigation — when the
  back-routed SQLite path tries to recreate state, it now refuses with
  exit 12 instead of silently rebuilding.
- No new SQL migration. F-split-brain is filesystem-local; F-lock is
  fcntl-local; F-escape and F-help are CLI-layer.
- Exit code 14 is reserved here. If a concurrent RFC has claimed 14,
  the implementer takes the next free code and updates this design's
  acceptance verifiers accordingly — the numbering is not
  load-bearing.

## Implementer summary

Five files change in source, four small test files land (one new for
each finding plus a help-snapshot extension). One new `StriatumError`
subclass (`RepoMigratedError`) and one new exit code (14). No schema
delta, no daemon RPC delta. Test-suite session fixture gains the
`STRIATUM_TEST_HARNESS=1` marker so the existing opt-out keeps working
under the tightened resolver.
