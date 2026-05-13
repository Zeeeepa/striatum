# RFC 0043 V1.5 Design Deltas

author: designer-unknown-model-001

## Scope

This design closes the D102 follow-up findings folded out of dogfood-048:
crash recovery for `migrate-repo-local`, mandatory daemon-required CLI
behavior, missing `daemon migrate-repo-local` CLI wiring, and an end-to-end
exit-code-12 regression test. It does not reopen D094 or RFC 0043 V1. The
Postgres schema remains additive-only, and the existing
`--keep-sqlite-readonly` tombstone behavior remains the default.

## Existing State

`migrate_repo_local()` in
`src/striatum/daemon_pg/repo_local_migration.py:351` resolves the repo,
finds `.striatum/state.sqlite3` through `db_path(repo)`, applies daemon
migrations, opens the SQLite source read-only, and delegates non-dry-run
work to `_migrate_full()` at
`src/striatum/daemon_pg/repo_local_migration.py:397`.

`_migrate_full()` starts a `SERIALIZABLE` Postgres transaction at
`src/striatum/daemon_pg/repo_local_migration.py:443`, ensures repository
registration, checks `_existing_checkpoint()`, copies repo rows, verifies
append-only manifests, writes `striatumd.repo_migrations`, and commits at
`src/striatum/daemon_pg/repo_local_migration.py:469`. Only after that commit
does it call `_tombstone_or_delete_state_db()` at
`src/striatum/daemon_pg/repo_local_migration.py:473`. The finalizer renames
`state.sqlite3` to `state.sqlite3.tombstone`, chmods it `0444`, and removes
WAL/SHM sidecars when `keep_sqlite_readonly` is true; with delete mode it
requires `confirm_delete` before unlinking
`src/striatum/daemon_pg/repo_local_migration.py:682`.

The bug is the re-entry path. When the source file is absent,
`migrate_repo_local()` returns `already_migrated: True` if a checkpoint row
exists at `src/striatum/daemon_pg/repo_local_migration.py:361`. When the
source file is present, `_migrate_full()` also returns `already_migrated:
True` before finalization if `_existing_checkpoint()` finds a row at
`src/striatum/daemon_pg/repo_local_migration.py:447`. A crash after the
Postgres commit but before `_tombstone_or_delete_state_db()` therefore leaves
a writable SQLite file on disk, and the re-run path refuses to finish the
tombstone.

The CLI daemon-required hook is present but default-off.
`resolve_requirement()` skips lifecycle commands, then returns `None` unless
`STRIATUM_DAEMON_REQUIRED == "1"` at
`src/striatum/cli/daemon_required.py:68`. `dispatch()` calls the hook before
touching SQLite, but its comment still says the historical SQLite path remains
usable by default at `src/striatum/cli/dispatch.py:191`. After that, the
legacy direct path still calls `ensure_initialized(repo)` and `connect(repo)`
at `src/striatum/cli/dispatch.py:404`, so the env gate is the active escape.
The `src/striatum/cli/` audit found no other independent daemon-required
policy gate; the remaining SQLite references are legacy command
implementations reached through that single dispatch path.

The migration command body exists in `src/striatum/cli/daemon.py:17`, but
`src/striatum/cli/parser.py:138` registers `daemon start`, `doctor`,
`migrate`, `status`, `stop`, `health`, `audit`, and `sweep` only. The top
level `_dispatch_daemon()` in `src/striatum/cli/dispatch.py:880` handles the
same set and never delegates to `striatum.cli.daemon.dispatch_daemon`.

## Delta 1: Checkpointed Resume For SQLite Finalization

Choose checkpointed resume, not transactional rollback. The current code
already copies all rows and writes `repo_migrations` in one serializable
Postgres transaction, then performs filesystem finalization. Deferring the
Postgres commit until after a SQLite rename would require holding the source
SQLite file and a Postgres transaction across filesystem operations, and it
would not make `state.sqlite3.tombstone` creation transactional. A
checkpointed finalizer is the smaller, clearer repair: Postgres remains the
authoritative checkpoint, while filesystem state is made idempotent.

Add a filesystem sentinel at:

```text
.striatum/state.sqlite3.migrated
```

The sentinel is written after `pg_conn.commit()` and before
`_tombstone_or_delete_state_db()`. Its JSON body should include
`repository_id`, `source_state_db_sha256`, `migrated_at`, and `action`
(`tombstone` or `delete`). Write it via temp file plus `Path.replace()` so the
sentinel itself is atomic. No Postgres schema change is required. If builders
choose to record the sentinel path in Postgres later, it must be a nullable
column on `striatumd.repo_migrations`.

Refactor finalization into a helper:

```python
def _finalize_sqlite_source_after_checkpoint(
    repo: Path,
    *,
    checkpoint: dict[str, Any],
    keep_sqlite_readonly: bool,
    confirm_delete: bool,
) -> dict[str, Any]:
    ...
```

This helper owns three states:

1. `state.sqlite3` exists: write or refresh `state.sqlite3.migrated`, call
   `_tombstone_or_delete_state_db()`, and return the finalizer result.
2. `state.sqlite3` is absent and `state.sqlite3.tombstone` exists: chmod the
   tombstone to `0444`, remove sidecars, ensure the sentinel exists, and
   return an idempotent `already_finalized` result.
3. Neither source nor tombstone exists: ensure the sentinel exists and return
   `already_finalized` for delete mode; for keep-readonly mode, include a
   warning field because the requested tombstone was not available.

Then change both `already_migrated` paths to call this helper instead of
returning early. The source-present early return in `_migrate_full()` at
`src/striatum/daemon_pg/repo_local_migration.py:447` is the critical one:
when `_existing_checkpoint()` is not `None`, commit/rollback as needed, then
finish SQLite finalization and include `sqlite_finalization` in the returned
envelope. The source-absent early return in `migrate_repo_local()` at
`src/striatum/daemon_pg/repo_local_migration.py:361` should also report
finalization state.

Keep `_verify_delete_options()` unchanged:
`--no-keep-sqlite-readonly` without `--confirm-delete` remains an exit-code-8
refusal at `src/striatum/daemon_pg/repo_local_migration.py:705`. The resume
path must respect the operator's current flags. If the original checkpoint was
created for tombstone mode and a re-run asks for delete mode, require
`--confirm-delete` and then delete the still-active source; do not silently
switch to destructive cleanup.

Regression test: extend `tests/daemon_pg/test_repo_local_migration.py`.
Monkeypatch `_tombstone_or_delete_state_db` to raise after the Postgres
checkpoint is committed. The first call should raise and leave
`.striatum/state.sqlite3` present. Restore the helper and call
`migrate_repo_local()` again with the same options. On fixed code the second
call returns `already_migrated: True`, creates
`.striatum/state.sqlite3.tombstone` mode `0444`, removes the original
`state.sqlite3`, and preserves the `repo_migrations` row. On current code the
second call returns before tombstoning, so the original source remains and
the test fails.

## Delta 2: Close The CLI Escape Path

Flip daemon-required enforcement to mandatory by default. Keep an opt-out only
for tests and legacy fixture maintenance:

```text
STRIATUM_DAEMON_REQUIRED=0
```

Any value other than `"0"` means enforcement is active. This preserves a
controlled internal escape for existing SQLite fixture tests while making the
operator-facing default match RFC 0043. Rename the docstring and comments in
`src/striatum/cli/daemon_required.py:1` and
`src/striatum/cli/dispatch.py:191`; they currently describe the no-op default
and will otherwise keep misleading future reviewers.

Repos that have not run `migrate-repo-local` must refuse with exit code 12;
they must not auto-migrate. Rationale: migration is an irreversible trust
boundary crossing involving tombstone/delete choices, and RFC 0043 requires
operator-visible remediation rather than hidden state movement.

The `DAEMON_OPTIONAL_COMMANDS` allowlist can remain, but should stay narrow:
`daemon`, `init`, `skills`, and `plugin` at
`src/striatum/cli/daemon_required.py:42`. `--help` and `--version` continue
to short-circuit in argparse. `daemon migrate-repo-local` remains reachable
because the whole `daemon` family bypasses the hook.

The current `repo_is_migrated()` heuristic at
`src/striatum/cli/daemon_required.py:175` is acceptable only as a fallback
for tests. The production implementation should ask the daemon for the
registered repository and `repo_migrations` checkpoint; if the daemon is
reachable but the checkpoint is absent while `state.sqlite3` exists, raise
`RepoNotMigratedError`. Preserve the canonical remediation text in
`render_repo_not_migrated_message()` at
`src/striatum/cli/daemon_required.py:117`, but keep it aligned with the
subparser that V1.5 wires:

```text
striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>
```

Audit result for `src/striatum/cli/`: the silent fallback is centralized in
`dispatch()` after `enforce_daemon_required()`. Modules such as
`mutations.py`, `introspect.py`, `recovery.py`, `worktree.py`,
`run_summary.py`, and `evidence.py` still type against `sqlite3.Connection`,
but they are not separate bypasses; they are reached after the single
top-level enforcement gate and should move behind daemon RPC in the larger RFC
0043 implementation.

## Delta 3: Wire `daemon migrate-repo-local`

Add the missing parser block immediately after the existing `daemon migrate`
block in `src/striatum/cli/parser.py:149`:

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

The `--repo` default should fall back to the top-level `--repo` value. Either
set the parser default to `None` and resolve it in `src/striatum/cli/daemon.py`
with `Path(args.repo_local_repo or args.repo)`, or set it during dispatch.
The latter keeps the migration helper independent of top-level parser
structure.

Then add the dispatch arm at the top of `_dispatch_daemon()` in
`src/striatum/cli/dispatch.py:880`:

```python
if args.daemon_command == "migrate-repo-local":
    from striatum.cli.daemon import dispatch_daemon

    return dispatch_daemon(args)
```

Update `src/striatum/cli/daemon.py:27` so `repo=Path(args.repo_local_repo)`
uses the fallback described above. The existing substrate validation in
`src/striatum/cli/daemon.py:20` should stay: only `--from sqlite --to pg` is
valid in V1.5.

Verification: `striatum daemon migrate-repo-local --help` must parse and show
`--from {sqlite}`, `--to {pg}`, `--repo REPO_LOCAL_REPO`, `--postgres-url`,
`--dry-run`, `--keep-sqlite-readonly`, `--no-keep-sqlite-readonly`,
`--confirm-delete`, and `--json`. The help command must not require a running
daemon.

## Delta 4: End-To-End Exit-Code-12 Test

Add `tests/exit_codes/test_rfc0043_repo_not_migrated_e2e.py`. The existing
`tests/exit_codes/test_rfc0043_refusals.py` checks helper construction and a
mocked `dispatch.main(...)` path, but it also asserts
`test_enforce_daemon_required_no_op_by_default`, which must be inverted when
the default flips.

Fixture shape:

1. Use `tmp_path` as a target repo.
2. Create `.striatum/state.sqlite3` from the existing
   `tests/fixtures/v1_repo_local_sqlite/` material or initialize a minimal
   SQLite DB at the current pre-cutover `LATEST_VERSION`.
3. Monkeypatch `daemon_socket_is_reachable` to return `True` so the test
   reaches the migration check rather than exit code 11.
4. Ensure `STRIATUM_DAEMON_REQUIRED` is unset. This is what proves the default
   is mandatory.
5. Call `striatum.cli.dispatch.main(["--repo", str(repo), "status"])`.

Assertions:

```python
assert rc == 12
stderr = capsys.readouterr().err
assert "repo_not_migrated" in stderr
assert "striatum daemon migrate-repo-local --from sqlite --to pg" in stderr
```

Add the JSON sibling:

```python
rc = dispatch.main(["--repo", str(repo), "status", "--json"])
assert rc == 12
payload = json.loads(capsys.readouterr().out)
assert payload["error"]["code"] == 12
assert "migrate-repo-local" in payload["error"]["hint"]
```

Optionally keep the exit-code-11 sibling already present in
`tests/exit_codes/test_rfc0043_refusals.py`, but update it so unset env still
enforces. The explicit opt-out test should become:

```python
monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
enforce_daemon_required("status", repo)
```

That test documents the only supported SQLite fallback for test fixtures.

## Acceptance Criteria

- A crash after Postgres checkpoint commit but before SQLite finalization is
  recoverable by re-running `striatum daemon migrate-repo-local`; the rerun
  finishes tombstone/delete idempotently and returns finalization metadata.
- `--keep-sqlite-readonly` remains the default and produces
  `.striatum/state.sqlite3.tombstone` with mode `0444` on both first-run and
  resume paths.
- The CLI enforces daemon-required mode unless
  `STRIATUM_DAEMON_REQUIRED=0` is explicitly set. Unmigrated repos refuse with
  exit code 12 and a migration command; no command silently creates, reads, or
  mutates SQLite after the gate.
- `striatum daemon migrate-repo-local --help` is reachable and lists the V1.5
  flags.
- End-to-end `dispatch.main(...)` tests cover exit code 12 for an unmigrated
  repo with the env var unset, plus the JSON error envelope.
