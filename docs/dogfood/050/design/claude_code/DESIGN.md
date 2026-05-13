---
schema_version: striatum.handoff.v1
artifact_kind: handoff
title: "Dogfood-050 — RFC 0043 V1.5 Design (close D102 follow-ups)"
---
author: designer-unknown-model-001

# Dogfood-050 — RFC 0043 V1.5 Design

Closes the four follow-up findings recorded in
`docs/dogfood/048/decisions/D102_cycle_exhaustion.md`. Scope is the
V1.5 delta on top of dogfood-048's V1 land: crash-recovery
persistence, CLI escape-path closure, parser/dispatch wiring, and an
end-to-end exit-code-12 test gap. RFC 0043 V2 work (hosted mode,
multi-tenancy, bundled Postgres), RFC 0039 V1.6 Go-daemon follow-up,
and broader doc rewrites are explicitly out of scope.

All changes are additive at the schema level (no `DROP COLUMN`, no
destructive index changes) and preserve the
`--keep-sqlite-readonly` tombstone semantics under every flag
combination.

## Finding 1 — F-crash (CRITICAL): two-phase post-commit sentinel

### Source of the gap

`src/striatum/daemon_pg/repo_local_migration.py:426-490` performs:

```
pg_conn.commit()                         # line 469 — PG durable
_tombstone_or_delete_state_db(...)       # line 473 — SQLite finalized
```

A kill -9 between line 469 and line 473 leaves the repo with both a
`striatumd.repo_migrations` checkpoint row AND a writable
`.striatum/state.sqlite3` on disk. The early-return paths that detect
"checkpoint exists" do not re-attempt finalization:

- `migrate_repo_local()` lines 351-370 — when `source_path.exists()`
  is `False` AND a checkpoint exists, returns `already_migrated: True`
  with no tombstone work. (Safe today because no source file remains;
  becomes load-bearing only if a sidecar mismatch occurs.)
- `_migrate_full()` lines 446-455 — when `source_path` still exists
  but `_existing_checkpoint(...)` returns a row, the function commits
  the empty PG transaction and returns `already_migrated: True`
  without touching the SQLite. This is the exact post-crash
  re-entry path; gemini REVIEW §2.1 cites line 204 (the same
  early-return semantics in an earlier revision).

### Selected shape — (b) checkpointed resume

Keep the existing Postgres-commit-first ordering. Add a sentinel and
make the `already_migrated` re-entry path complete tombstone work
idempotently. Reasons for picking (b) over (a):

- The PG transaction shape (`BEGIN ISOLATION LEVEL SERIALIZABLE`
  at line 444, commit at line 469) is the load-bearing invariant for
  the audit-chain re-anchor (`compute_repo_local_reanchor` at
  lines 414-423); restructuring it to defer commit until SQLite is
  locked would either require dropping `SERIALIZABLE` or holding
  the transaction open across a filesystem rename, both of which
  add new failure modes.
- (b) is purely additive: one sentinel file write, one sentinel
  unlink, one branch in the re-entry path. No change to
  `_copy_repo_rows`, `compute_repo_local_reanchor`, or
  `_write_repo_migration_checkpoint`.
- (b) preserves byte-equivalence verification timing — the
  checkpoint is still written inside the PG transaction, so any PG
  rollback discards both the rows and the checkpoint together.

### Sentinel design

- **Path.** `.striatum/state.sqlite3.migrated`.
  Same `.striatum/` directory as the source SQLite and the
  tombstone, so cleanup colocates with the state file.
- **Mode.** `0444` (read-only) immediately after `os.replace`, so
  an out-of-band process cannot truncate it during resume.
- **Content (single-line JSON, canonical).**

  ```json
  {"repository_id": "repo_<hex>",
   "source_state_db_sha256": "<hex>",
   "keep_sqlite_readonly": true,
   "confirm_delete": false,
   "checkpoint_written_at": "2026-05-13T16:54:19Z"}
  ```

  `source_state_db_sha256` matches the value captured at
  `repo_local_migration.py:371` (`source_state_db_sha256`) and
  re-stored at `_write_repo_migration_checkpoint` line 657. The
  resume path uses it to refuse if the on-disk
  `.striatum/state.sqlite3` SHA has drifted (operator manually
  swapped a different SQLite into place).
- **Atomic write.** Write to `.striatum/state.sqlite3.migrated.tmp`,
  `fsync` the file, `os.replace` to the final name, `fsync` the
  parent directory. Standard POSIX atomic-rename idiom.

### Code shape

Two new module-level helpers added to
`src/striatum/daemon_pg/repo_local_migration.py`:

```python
def _write_resume_sentinel(
    repo: Path,
    *,
    repository_id: str,
    source_state_db_sha256: str,
    keep_sqlite_readonly: bool,
    confirm_delete: bool,
) -> Path: ...

def _clear_resume_sentinel(repo: Path) -> None: ...
```

`_migrate_full` (lines 426-490) is amended to call
`_write_resume_sentinel` immediately after `pg_conn.commit()` (line
469) and `_clear_resume_sentinel` immediately after
`_tombstone_or_delete_state_db` (line 473) returns. A `try/except`
arm around the tombstone call leaves the sentinel in place on
failure so the next invocation resumes.

A third helper covers re-entry:

```python
def _resume_tombstone_if_needed(
    repo: Path,
    *,
    checkpoint: dict[str, Any],
    repository_id: str,
) -> dict[str, Any] | None: ...
```

Both `already_migrated` early-return sites are amended to call
`_resume_tombstone_if_needed` before returning:

- `migrate_repo_local` lines 360-368 — call resume helper after
  `checkpoint` is loaded and before the early `return`. (Today this
  branch is reached only when `source_path` does not exist; resume
  helper is a no-op in that case, but the call site is symmetric
  with `_migrate_full` and protects against a future sidecar-only
  retry.)
- `_migrate_full` lines 446-455 — call resume helper after
  `_existing_checkpoint(...)` returns a row and before the early
  `return`. This is the load-bearing re-entry path.

Resume helper logic (concise):

```text
sentinel = repo / ".striatum" / "state.sqlite3.migrated"
state_db = db_path(repo)            # repo / ".striatum" / "state.sqlite3"
tombstone = state_db.with_name(state_db.name + ".tombstone")

if not state_db.exists() and not sentinel.exists():
    return None                     # clean, fully migrated
if state_db.exists():
    on_disk_sha = _file_sha256(state_db)
    if on_disk_sha != checkpoint["source_state_db_sha256"]:
        raise StriatumError(
            "state.sqlite3 on disk does not match the migrated checkpoint; "
            "manual operator intervention required",
            exit_code=8,
        )
    # Resume: write sentinel (idempotent), tombstone, clear sentinel.
    if not sentinel.exists():
        _write_resume_sentinel(...)
    keep_ro = sentinel_data.get("keep_sqlite_readonly", True)
    confirm = sentinel_data.get("confirm_delete", False)
    result = _tombstone_or_delete_state_db(
        repo, keep_sqlite_readonly=keep_ro, confirm_delete=confirm,
    )
    _clear_resume_sentinel(repo)
    return {"action": "resumed_tombstone", **result}
# state.sqlite3 absent but sentinel present — tombstone done last
# run; clean up the orphan.
_clear_resume_sentinel(repo)
return {"action": "cleared_orphan_sentinel"}
```

### Backward-compat anchor

- The tombstone path (`source.replace(tombstone)` then `chmod 0o444`
  at `repo_local_migration.py:690-696`) is unchanged. The sentinel
  is a *peer* file in `.striatum/`, not a modification of the
  tombstone path.
- `_remove_sqlite_sidecars` (lines 710-717) keeps clearing
  `-wal` and `-shm` sidecars as today.
- `--keep-sqlite-readonly` semantics are preserved: when set, the
  resume path renames to `.tombstone`; when unset with
  `--confirm-delete`, the resume path deletes. The sentinel records
  the original flag values so an interrupted destructive deletion
  still completes destructively (not silently converted to a
  tombstone).
- No Postgres schema change. The audit-chain re-anchor invariant is
  untouched.

### Regression test

`tests/daemon_pg/test_repo_local_migration_crash_resume.py` (new).
Two cases — one for each early-return path — both fail on the
unfixed code:

1. **`test_resume_tombstone_after_crash_between_commit_and_rename`**
   - Fixture: ephemeral Postgres (`STRIATUM_PG_TEST_URL`), the
     committed `tests/fixtures/v1_repo_local_sqlite/state.sqlite3`
     copied into `tmp_path / ".striatum"`.
   - Patch `_tombstone_or_delete_state_db` to raise
     `RuntimeError("kill -9 simulation")` on the first invocation.
   - Call `migrate_repo_local(...)` with
     `keep_sqlite_readonly=True`. Assert the `RuntimeError`
     propagates and assert `.striatum/state.sqlite3` still exists,
     `.striatum/state.sqlite3.migrated` exists with the expected
     SHA, the `striatumd.repo_migrations` row is present.
   - Unpatch. Call `migrate_repo_local(...)` again. Assert the
     result `action == "resumed_tombstone"`,
     `.striatum/state.sqlite3` is gone,
     `.striatum/state.sqlite3.tombstone` exists with mode `0o444`,
     `.striatum/state.sqlite3.migrated` is gone, and the call
     returns `already_migrated: True` at the outer level.

2. **`test_resume_refuses_when_on_disk_sha_drifted`**
   - Same setup, simulate the crash, then OVERWRITE
     `.striatum/state.sqlite3` with `b"corrupted"`. Re-run the
     migration. Assert it raises `StriatumError` with `exit_code=8`
     and the message names the drift; assert the sentinel and the
     `repo_migrations` row are both still present (no destructive
     cleanup on detected drift).

Both tests skip when `STRIATUM_PG_TEST_URL` is unset, matching the
existing convention in `tests/daemon_pg/test_repo_local_migration.py`.

## Finding 2 — F-escape (MAJOR): default-flip + env-var removal

### Source of the gap

`src/striatum/cli/daemon_required.py:60-72` reads:

```python
def resolve_requirement(command: str | None) -> DaemonRequirement | None:
    if command in DAEMON_OPTIONAL_COMMANDS:
        return None
    if os.environ.get(ENV_DAEMON_REQUIRED) != "1":
        return None
    return DaemonRequirement(enforced=True, socket_path=resolve_socket_path())
```

Default behaviour is `None` → `enforce_daemon_required` no-ops →
`dispatch()` falls through to the legacy SQLite path
(`src/striatum/cli/dispatch.py:191-198`). RFC 0043 §3 requires
daemon-required to be the default; the env-gated opt-in is the
escape path gemini REVIEW §2.2 calls out.

### CLI-tree audit for sibling escape paths

Grepped `src/striatum/cli/` for paths that open SQLite directly
without going through `enforce_daemon_required`. Findings:

- `src/striatum/cli/dispatch.py:188-217` is the **only** central
  gate. `enforce_daemon_required` runs at line 195 before any
  command-specific branch. Verified by reading the dispatch arms
  for `daemon`, `repo`, `cross-repo`, `--daemon` read routes, and
  the legacy SQLite-backed mutations.
- `src/striatum/cli/daemon.py:17-34` is the daemon subcommand
  helper; it never touches SQLite (only Postgres via
  `repo_local_migration`). It is reached through the
  `daemon` command in `DAEMON_OPTIONAL_COMMANDS`, which is correct
  — daemon-lifecycle verbs must work without an already-running
  daemon.
- `src/striatum/cli/parser.py:19` declares the global `--repo`
  argument; no parser-level path opens SQLite.
- `src/striatum/cli/mutations.py` (referenced from the RFC 0030
  registry expansion at
  `src/striatum/daemon_rpc/registry.py`) holds the V1 mutation
  bodies; every mutation is dispatched from `dispatch()`, so the
  gate at line 195 covers them.

No sibling escape paths. The single gate is load-bearing and
removing the env-gate closes the surface.

### Decision — REMOVE the env var entirely

Selected shape:

- Delete `ENV_DAEMON_REQUIRED` from `daemon_required.py`. The
  module no longer reads any env var to decide whether enforcement
  applies.
- `resolve_requirement(command)` becomes:

  ```python
  def resolve_requirement(command: str | None) -> DaemonRequirement | None:
      if command in DAEMON_OPTIONAL_COMMANDS:
          return None
      return DaemonRequirement(enforced=True, socket_path=resolve_socket_path())
  ```

- `ENV_DAEMON_SOCKET` stays. It is configuration (socket path
  override), not an enforcement gate.
- Tests that need the legacy SQLite path use a single fixture in
  `tests/conftest.py`:

  ```python
  @pytest.fixture
  def disable_daemon_required(monkeypatch: pytest.MonkeyPatch) -> None:
      """Bypass RFC 0043 §3 enforcement for tests that target the legacy
      SQLite-backed dispatch path. Use sparingly; new tests should
      target the daemon-mediated path through MultiRepoHarness."""
      monkeypatch.setattr(
          "striatum.cli.dispatch.enforce_daemon_required",
          lambda command, repo: None,
      )
  ```

Rationale (one sentence): an opt-OUT env var preserves the same
"default is enforcement, escape path is one env var away" shape
gemini REVIEW §2.2 already flagged, so V1.5 removes the gate and
pays the test-fixture cost once; the fixture is a per-test
explicit opt-in instead of a process-global env toggle.

### Decision — unmigrated repo: REFUSE with exit code 12

`enforce_daemon_required(command, repo)` already raises
`RepoNotMigratedError(exit_code=12)` when the daemon socket
listens but `repo_is_migrated(repo)` returns `False`
(`daemon_required.py:175-190` for the heuristic;
lines 193-206 for the gate). The default-flip keeps this branch
unchanged. The stderr template at lines 117-124 already names
`striatum daemon migrate-repo-local --from sqlite --to pg --repo
<path>`.

Rationale (one sentence): auto-migration is a destructive operation
that requires admin authorization and `--confirm-delete` semantics
under RFC 0043 §4, so silent auto-migration would bypass the
documented consent gate and surprise operators whose repos were
manipulated without an explicit verb invocation.

### Code shape

- `src/striatum/cli/daemon_required.py` —
  delete `ENV_DAEMON_REQUIRED` and the env-gate in
  `resolve_requirement`. Keep `ENV_DAEMON_SOCKET`.
  Remove the env-gate language from the module docstring;
  update the docstring to say "always-on enforcement; the only
  bypass is the `DAEMON_OPTIONAL_COMMANDS` allowlist."
- `src/striatum/cli/dispatch.py:188-217` — no change to the
  gate location; the gate now fires for every command not in the
  allowlist. The comment at lines 191-194 is updated to drop the
  "Default off" sentence.
- `tests/exit_codes/test_rfc0043_refusals.py` — existing tests
  remove their `monkeypatch.setenv(ENV_DAEMON_REQUIRED, "1")`
  calls; the env import is dropped. The
  `_clear_daemon_required_env` fixture drops the
  `ENV_DAEMON_REQUIRED` line.
- `tests/conftest.py` — add the `disable_daemon_required`
  fixture above.
- Any other test using `dispatch.main([...])` against a
  legacy-SQLite fixture either (a) adopts the
  `disable_daemon_required` fixture or (b) is migrated to
  `MultiRepoHarness`. Expected delta: small;
  `Grep "STRIATUM_DAEMON_REQUIRED" tests/` returns only the
  `tests/exit_codes/test_rfc0043_refusals.py` callsites.

### Backward-compat anchor

- The `--keep-sqlite-readonly` tombstone path is reached only via
  `striatum daemon migrate-repo-local`, which sits in
  `DAEMON_OPTIONAL_COMMANDS` and is therefore not gated by
  enforcement. The tombstone path works under every flag
  combination, including the new crash-resume re-entry from
  Finding 1.
- The `--daemon` flag (V1 RFC 0028 read-routing) remains usable
  for the four supported read verbs; enforcement still applies to
  it (it is not in the allowlist), so an operator using
  `--daemon status` against an unreachable daemon gets exit
  code 11, not the legacy SQLite read.

## Finding 3 — F-parser (MODERATE): subparser + dispatch wiring

### Source of the gap

- Migration body: `src/striatum/daemon_pg/repo_local_migration.py`
  (`migrate_repo_local`, lines 351-411).
- Daemon-command helper:
  `src/striatum/cli/daemon.py:17-34`.
- Parser: `src/striatum/cli/parser.py:138-167` constructs the
  `daemon` subparser (`start`, `doctor`, `migrate`, `status`,
  `stop`, `health`, `audit`, `sweep`). The
  `migrate-repo-local` subcommand is **not** present.
- Dispatch: `src/striatum/cli/dispatch.py:880-927`
  (`_dispatch_daemon`) has arms for the eight existing
  subcommands. The `migrate-repo-local` arm is **not** present.

Result: the migration body is callable programmatically but the
operator-facing `striatum daemon migrate-repo-local ...` command
errors out at argparse with "invalid choice" (exit code 2).

### Subparser block to insert

Insert immediately after `daemon_sweep` at
`src/striatum/cli/parser.py:167`:

```python
daemon_migrate_repo_local = daemon_sub.add_parser(
    "migrate-repo-local",
    help=(
        "migrate this repo's .striatum/state.sqlite3 into the daemon "
        "PostgreSQL substrate"
    ),
)
daemon_migrate_repo_local.add_argument(
    "--from",
    dest="from_substrate",
    choices=["sqlite"],
    required=True,
    help="source substrate; only 'sqlite' is supported",
)
daemon_migrate_repo_local.add_argument(
    "--to",
    dest="to_substrate",
    choices=["pg"],
    required=True,
    help="target substrate; only 'pg' is supported",
)
daemon_migrate_repo_local.add_argument(
    "--postgres-url",
    help="override STRIATUM_DAEMON_DB_URL for this invocation",
)
daemon_migrate_repo_local.add_argument(
    "--dry-run",
    action="store_true",
    help="report row counts and audit-chain anchors; write nothing",
)
daemon_migrate_repo_local.add_argument(
    "--keep-sqlite-readonly",
    action=argparse.BooleanOptionalAction,
    default=True,
    help=(
        "after migration, rename state.sqlite3 to "
        "state.sqlite3.tombstone (mode 0444). Default on. Pass "
        "--no-keep-sqlite-readonly with --confirm-delete to delete "
        "instead."
    ),
)
daemon_migrate_repo_local.add_argument(
    "--confirm-delete",
    action="store_true",
    help="required when --no-keep-sqlite-readonly is used; opt-in to destructive cleanup",
)
daemon_migrate_repo_local.add_argument("--json", action="store_true")
```

Notes:

- The subparser does NOT redeclare `--repo`. The top-level
  `--repo` at `parser.py:19` carries the target. This requires a
  one-line change in `src/striatum/cli/daemon.py:28` to read
  `args.repo` instead of `args.repo_local_repo`; the
  `repo_local_repo` attribute is internal to the unwired-V1
  helper and has no other consumers.
- `argparse.BooleanOptionalAction` produces both
  `--keep-sqlite-readonly` and `--no-keep-sqlite-readonly`,
  matching the verb shape described in RFC 0043 §4 step 7. (Python
  ≥3.9 — already required by the project.)
- `--from sqlite --to pg` is explicit (rather than implicit
  defaults) so the verb is forward-compatible with a future
  second source substrate.

### Dispatch arm to insert

Insert immediately before the trailing `raise StriatumError("unknown
daemon command", exit_code=2)` at
`src/striatum/cli/dispatch.py:927`:

```python
if args.daemon_command == "migrate-repo-local":
    from striatum.cli.daemon import dispatch_daemon as repo_local_dispatch
    return repo_local_dispatch(args)
```

The dispatch arm forwards into `cli/daemon.py:dispatch_daemon`
unchanged (the helper already routes on
`args.daemon_command == "migrate-repo-local"` and reads
`args.from_substrate`, `args.to_substrate`, `args.postgres_url`,
`args.dry_run`, `args.keep_sqlite_readonly`, `args.confirm_delete`).
The single update to `cli/daemon.py:28` swaps
`Path(args.repo_local_repo)` for `Path(args.repo)`.

### CLI shape after wiring

`striatum daemon migrate-repo-local --help` prints:

```text
usage: striatum daemon migrate-repo-local [-h] --from {sqlite} --to {pg}
       [--postgres-url POSTGRES_URL] [--dry-run]
       [--keep-sqlite-readonly | --no-keep-sqlite-readonly]
       [--confirm-delete] [--json]

migrate this repo's .striatum/state.sqlite3 into the daemon
PostgreSQL substrate

options:
  -h, --help                       show this help message and exit
  --from {sqlite}                  source substrate; only 'sqlite' is supported
  --to {pg}                        target substrate; only 'pg' is supported
  --postgres-url POSTGRES_URL      override STRIATUM_DAEMON_DB_URL for this invocation
  --dry-run                        report row counts and audit-chain anchors; write nothing
  --keep-sqlite-readonly, --no-keep-sqlite-readonly
                                   after migration, rename state.sqlite3 to
                                   state.sqlite3.tombstone (mode 0444). Default on.
                                   Pass --no-keep-sqlite-readonly with --confirm-delete
                                   to delete instead.
  --confirm-delete                 required when --no-keep-sqlite-readonly is used;
                                   opt-in to destructive cleanup
  --json
```

The top-level `striatum --help` is unchanged (subparsers are not
recursively listed at the root); `striatum daemon --help` adds
`migrate-repo-local` to the subcommand list.

### Backward-compat anchor

- The existing `daemon migrate` subparser (Track A's daemon-DB
  migration, parser.py:149-156) is untouched. The two verbs are
  distinct: `daemon migrate` migrates daemon-global state; the new
  `daemon migrate-repo-local` migrates per-repo workflow state.
- The unwired V1 helper attribute `args.repo_local_repo` is
  removed in favour of the top-level `args.repo`. There is no
  external caller, so the rename is purely internal.

## Finding 4 — F-test (LOW): end-to-end exit-code-12

### Source of the gap

claude's `accept_with_findings` REVIEW noted that
`tests/exit_codes/test_rfc0043_refusals.py:155-166` exercises the
exit-code-11 path through `dispatch.main(...)` but the matching
exit-code-12 path is only exercised at the helper level
(`enforce_daemon_required` unit test at lines 129-152), not
end-to-end through `dispatch.main(...)` against a real
unmigrated-repo fixture.

### Test path and fixture

- File: `tests/exit_codes/test_rfc0043_unmigrated_repo_e2e.py`
  (new).
- Fixture: copy `tests/fixtures/v1_repo_local_sqlite/state.sqlite3`
  (the existing Track A V1 schema fixture) into
  `tmp_path / ".striatum" / "state.sqlite3"`. No tombstone marker.
  This matches the `repo_is_migrated` heuristic at
  `daemon_required.py:175-190` for the pre-cutover signal.

### Assertion

```python
def test_dispatch_refuses_unmigrated_repo_with_exit_code_12(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    import socket as socket_mod

    # Stand up a real listening socket so the daemon-unreachable
    # branch does not pre-empt the repo-not-migrated branch.
    socket_path = tmp_path / "striatumd.sock"
    listener = socket_mod.socket(socket_mod.AF_UNIX, socket_mod.SOCK_STREAM)
    listener.bind(str(socket_path))
    listener.listen(1)
    monkeypatch.setenv("STRIATUM_DAEMON_SOCKET", str(socket_path))

    # Real unmigrated-repo fixture: pre-cutover state.sqlite3, no tombstone.
    striatum_dir = tmp_path / ".striatum"
    striatum_dir.mkdir()
    fixture = Path("tests/fixtures/v1_repo_local_sqlite/state.sqlite3")
    (striatum_dir / "state.sqlite3").write_bytes(fixture.read_bytes())

    try:
        rc = dispatch_mod.main(["--repo", str(tmp_path), "status"])
    finally:
        listener.close()

    assert rc == 12
    captured = capsys.readouterr()
    assert "repo_not_migrated" in captured.err
    assert (
        f"striatum daemon migrate-repo-local --from sqlite --to pg --repo {tmp_path}"
        in captured.err
    )
```

The assertion shape uses the returned `rc` (matching the existing
convention at `test_rfc0043_refusals.py:163, 179`); `dispatch.main`
returns the exit code rather than raising `SystemExit`. The
canonical remediation block is sourced from
`render_repo_not_migrated_message` (`daemon_required.py:117-124`).

### Optional sibling — exit-11 e2e

`test_rfc0043_refusals.py:155-166` already exercises exit-code-11
through `dispatch.main(...)`. The sibling adds no new coverage and
is dropped from V1.5 scope.

### Optional sibling — JSON envelope for exit-12

If the same e2e test is desired in `--json` mode (matching the
exit-11 JSON test at `test_rfc0043_refusals.py:169-190`), it adds:

```python
def test_dispatch_refuses_unmigrated_repo_json_envelope(...):
    # ... same setup ...
    rc = dispatch_mod.main(["--repo", str(tmp_path), "status", "--json"])
    assert rc == 12
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is False
    assert payload["error"]["code"] == 12
    assert "striatum daemon migrate-repo-local" in payload["error"]["hint"]
```

Recommended: include both. The two tests share fixture setup and
together cover the human and JSON output surfaces. Total test
delta: one new file, two test functions.

### Backward-compat anchor

The fixture at `tests/fixtures/v1_repo_local_sqlite/state.sqlite3`
already exists and is committed (Track A V1 fixture). The new test
file is additive and adds no new dependencies. Existing exit-code
tests are unaffected.

## Summary of files touched

- `src/striatum/daemon_pg/repo_local_migration.py` — three
  new helpers (`_write_resume_sentinel`,
  `_clear_resume_sentinel`, `_resume_tombstone_if_needed`); call
  sites at the two `already_migrated` early-returns and at
  `_migrate_full` lines 469/473.
- `src/striatum/cli/daemon_required.py` — drop
  `ENV_DAEMON_REQUIRED`; simplify `resolve_requirement`.
- `src/striatum/cli/dispatch.py` — add `migrate-repo-local`
  dispatch arm at line 927; update comment at lines 191-194.
- `src/striatum/cli/parser.py` — add `migrate-repo-local`
  subparser block after line 167.
- `src/striatum/cli/daemon.py` — read `args.repo` instead of
  `args.repo_local_repo` (line 28).
- `tests/conftest.py` — add `disable_daemon_required` fixture.
- `tests/exit_codes/test_rfc0043_refusals.py` — drop env-var
  setenv calls in fixture and tests.
- `tests/daemon_pg/test_repo_local_migration_crash_resume.py`
  (new) — two crash-resume regression cases.
- `tests/exit_codes/test_rfc0043_unmigrated_repo_e2e.py`
  (new) — end-to-end exit-12 against an unmigrated repo fixture
  (text and JSON variants).

No Postgres schema migration is required. No additions to the
RFC 0030 method registry. `STRIATUM_DAEMON_DB_URL` and the
tombstone semantics are preserved end-to-end.
