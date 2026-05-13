---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/050/design/codex/DESIGN.md", "docs/dogfood/050/design/claude_code/DESIGN.md", "docs/dogfood/050/design/gemini/DESIGN.md"]
---
author: designer-unknown-model-002

# RFC 0043 V1.5 Design Synthesis

This synthesis closes the D102 follow-up findings without reopening RFC 0043 V1. Implementation order is parser wiring, daemon-required default flip, exit-code-12 coverage, then crash-resume hardening. Parser wiring must land before the exit-code-12 test because the refusal text points at `daemon migrate-repo-local`; the default flip must land before the test so the unmigrated repo path is exercised without `STRIATUM_DAEMON_REQUIRED=1`.

## 1. Wire `daemon migrate-repo-local`

Add the missing subcommand in `src/striatum/cli/parser.py` immediately after the existing `daemon migrate` block:

```python
daemon_migrate_repo = daemon_sub.add_parser(
    "migrate-repo-local",
    help="migrate one repo-local .striatum/state.sqlite3 into daemon PostgreSQL state",
)
daemon_migrate_repo.add_argument("--from", dest="from_substrate", choices=["sqlite"], required=True)
daemon_migrate_repo.add_argument("--to", dest="to_substrate", choices=["pg"], required=True)
daemon_migrate_repo.add_argument("--repo", dest="repo_local_repo", default=None)
daemon_migrate_repo.add_argument("--postgres-url")
daemon_migrate_repo.add_argument("--dry-run", action="store_true")
daemon_migrate_repo.add_argument(
    "--keep-sqlite-readonly",
    dest="keep_sqlite_readonly",
    action="store_true",
    default=True,
    help="rename state.sqlite3 to state.sqlite3.tombstone and chmod it 0444",
)
daemon_migrate_repo.add_argument(
    "--no-keep-sqlite-readonly",
    dest="keep_sqlite_readonly",
    action="store_false",
    help="delete state.sqlite3 after migration; requires --confirm-delete",
)
daemon_migrate_repo.add_argument("--confirm-delete", action="store_true")
daemon_migrate_repo.add_argument("--json", action="store_true")
```

Add the first arm in `src/striatum/cli/dispatch.py::_dispatch_daemon`:

```python
if args.daemon_command == "migrate-repo-local":
    from striatum.cli.daemon import dispatch_daemon

    return dispatch_daemon(args)
```

In `src/striatum/cli/daemon.py`, resolve the repository with the top-level fallback:

```python
repo=Path(args.repo_local_repo or args.repo)
```

The operator smoke command is `striatum daemon migrate-repo-local --help`. It must show `--from {sqlite}`, `--to {pg}`, `--repo REPO_LOCAL_REPO`, `--postgres-url`, `--dry-run`, `--keep-sqlite-readonly`, `--no-keep-sqlite-readonly`, `--confirm-delete`, and `--json`, and it must not require a daemon.

## 2. Flip daemon-required mode on by default

Keep `STRIATUM_DAEMON_REQUIRED=0` as an explicit opt-out for tests and legacy fixture maintenance only. Any other value, including unset, enforces daemon-required behavior. This is preferable to removing the variable because the current test suite still has SQLite-backed fixtures that should migrate incrementally rather than by a single disruptive rewrite.

The exact change in `src/striatum/cli/daemon_required.py:resolve_requirement` is:

```python
if command in DAEMON_OPTIONAL_COMMANDS:
    return None
if os.environ.get(ENV_DAEMON_REQUIRED) == "0":
    return None
return DaemonRequirement(enforced=True, socket_path=resolve_socket_path())
```

Update the module docstring and the comment in `src/striatum/cli/dispatch.py` so they no longer describe the helper as default-off or recommend `STRIATUM_DAEMON_REQUIRED=1`.

CLI escape-path audit result: none beyond this one top-level gate. The direct SQLite command implementations in `mutations.py`, `introspect.py`, `recovery.py`, `worktree.py`, `run_summary.py`, and `evidence.py` are legacy implementation paths reached only after `dispatch()` calls `enforce_daemon_required(args.command, repo)`. They are not independent silent-fallback gates.

## 3. Add the exit-code-12 end-to-end test

Use the existing file `tests/exit_codes/test_rfc0043_refusals.py`; invert the existing default-off assertions and add the end-to-end unmigrated-repo case there so all RFC 0043 refusal tests stay together.

Fixture shape: create a `tmp_path` target repo with `.striatum/state.sqlite3`. Prefer copying `tests/fixtures/v1_repo_local_sqlite/state.sqlite3` when present; otherwise create a minimal placeholder file because the current refusal helper only needs the pre-cutover disk signal. Monkeypatch or bind a temporary Unix socket so the test passes the exit-code-11 daemon-unreachable check and reaches `repo_is_migrated()`.

Assertion shape: do not assert `SystemExit`. Call `dispatch_mod.main(["--repo", str(tmp_path), "status"])`, assert the returned code is `12`, then use `capsys.readouterr().err` to assert it contains `repo_not_migrated` and `striatum daemon migrate-repo-local --from sqlite --to pg --repo`.

Also update `test_resolve_requirement_returns_none_without_env` to assert a `DaemonRequirement` is returned when the env var is unset, and add a replacement assertion that `STRIATUM_DAEMON_REQUIRED=0` makes `resolve_requirement("status") is None`.

## 4. Harden crash recovery with checkpointed resume

Choose checkpointed resume, not transactional rollback. The existing migration already uses one Postgres transaction at `BEGIN ISOLATION LEVEL SERIALIZABLE` for row copy, re-anchor verification, and `repo_migrations` checkpoint insertion; trying to include a filesystem rename in that transaction is impossible and holding the transaction open across finalization would only add a cross-resource failure mode.

The sentinel path is:

```text
.striatum/state.sqlite3.migrated
```

Change the `_migrate_full` signature in `src/striatum/daemon_pg/repo_local_migration.py` to pass the finalizer sentinel explicitly:

```python
def _migrate_full(
    sqlite_conn: sqlite3.Connection,
    pg_conn: Any,
    *,
    repo: Path,
    source_path: Path,
    sentinel_path: Path,
    source_user_version: int,
    source_state_db_sha256: str,
    counts: dict[str, int],
    keep_sqlite_readonly: bool,
    confirm_delete: bool,
) -> dict[str, Any]:
```

`migrate_repo_local()` computes `sentinel_path = repo / ".striatum" / "state.sqlite3.migrated"` and passes it to `_migrate_full`. After `pg_conn.commit()`, `_migrate_full` writes the sentinel atomically, then calls `_tombstone_or_delete_state_db()`, then removes the sentinel only after finalization succeeds. The sentinel JSON should include `repository_id`, `source_state_db_sha256`, `keep_sqlite_readonly`, and `confirm_delete`.

Both early-return paths that find an existing checkpoint must call a new idempotent helper before returning:

```python
def _resume_sqlite_finalization_after_checkpoint(
    repo: Path,
    *,
    source_path: Path,
    sentinel_path: Path,
    checkpoint: dict[str, Any],
    keep_sqlite_readonly: bool,
    confirm_delete: bool,
) -> dict[str, Any] | None:
```

If `state.sqlite3` still exists, the helper verifies its SHA against `checkpoint["source_state_db_sha256"]`, resumes the original tombstone/delete action, clears the sentinel, and returns the `sqlite_finalization` result. If only the sentinel remains, it clears the orphan. If neither file exists, it returns `None`.

Regression test path: `tests/daemon_pg/test_repo_local_migration_crash_resume.py`. The primary test monkeypatches `_tombstone_or_delete_state_db` to raise after the Postgres checkpoint commits, asserts `.striatum/state.sqlite3` and `.striatum/state.sqlite3.migrated` remain, then reruns `migrate_repo_local()` and asserts `already_migrated: True`, `.striatum/state.sqlite3.tombstone` exists with mode `0444`, the source file is gone, and the sentinel is cleared. A second test corrupts the source before rerun and asserts a non-destructive exit-code-8 refusal.

No new SQL file is required under `src/striatum/daemon_pg/sql/`; V1.5 is schema-additive but this selected fix needs no schema change. The existing `--keep-sqlite-readonly` tombstone semantics are preserved in normal migration, resumed migration, dry-run, and already-migrated paths.
