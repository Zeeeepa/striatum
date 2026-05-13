# DESIGN — RFC 0043 V1 (Postgres as sole substrate, daemon-required runtime)

author: designer-unknown-model-001

Scope: dogfood-048, RFC 0043 V1. Two parallel implementation tracks land in
the same daemon DB schema bump: Track A (codex lane) ships the per-repo
PostgreSQL schema and the `migrate-repo-local` command; Track B (claude lane)
retires `--no-daemon`, wires exit codes 11 / 12, and expands the RFC 0030
method registry to cover every mutation currently in
`src/striatum/cli/mutations.py`. Out-of-scope items called out in the prompt
(bundled Postgres, `tenant_id` enforcement, hosted-mode auth, RFC 0039 Go
revision, operator-owned README/TODO/CHANGELOG/SPEC/HOW_TO updates) are
explicitly deferred and are not designed here.

References used throughout: `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md`
(§1 substrate boundary, §3 daemon-required CLI, §4 migration, §5 method
registry), `docs/rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md` (§3
schema model, §4 V1→V2 cutover, §5 audit chain, §6 concurrency primitives,
§7 test infra), `docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md`
(§1 envelope, §3 handshake, §4 capability binding, §5 audit shape, §7
routing), and `docs/DECISION_LOG.md` D094 (which supersedes D006/D007/D036
and the SQLite half of D009).

## Track A — Per-repo Postgres schema + `migrate-repo-local`

### A.1 New schema migration file

New file: `src/striatum/daemon_pg/sql/0005_repo_local_tables.sql`. It is
loaded by the existing daemon-DB migration runner (`src/striatum/daemon_pg/
migrations.py::apply_migrations`) that already consumed
`0001_baseline.sql` … `0004_dogfood_surgical_recovery.sql`. Pattern mirror:
`0002_rpc_supervision_apply.sql` is the precedent for a single migration
script introducing a large block of related tables, indexes, and grants;
this design follows the same shape.

The migration creates 15 tables in the `striatumd` schema, each gaining
`repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id)`
as the leading scope column. Per RFC 0043 §1 the namespace is single-schema
keyed by `repository_id`, per the open-question recommendation; this keeps
cross-repo queries (RFC 0032) trivial and avoids per-schema isolation cost.

Tables and their V1-SQLite progenitors in `src/striatum/schema.py`:

| Daemon-DB table                          | V1 source (schema.py lines)         | Repo-local PK extension                          |
|------------------------------------------|-------------------------------------|--------------------------------------------------|
| `striatumd.runs`                         | `runs` (25–48)                      | `(repository_id, run_id)` composite, PK `run_id` |
| `striatumd.sessions`                     | `sessions` (50–67)                  | `(repository_id, session_id)`                    |
| `striatumd.jobs`                         | `jobs` (69–100)                     | `(repository_id, job_id)`                        |
| `striatumd.job_dependencies`             | `job_dependencies` (101–106)        | `(repository_id, job_id, depends_on_job_id)`     |
| `striatumd.queue_messages`               | `queue_messages` (108–135)          | `(repository_id, message_id)`                    |
| `striatumd.leases`                       | `leases` (137–149)                  | `(repository_id, lease_id)`                      |
| `striatumd.work_packets`                 | `work_packets` (151–162)            | `(repository_id, packet_id)`                     |
| `striatumd.artifacts`                    | `artifacts` (164–183)               | `(repository_id, artifact_id)`                   |
| `striatumd.verdicts`                     | `verdicts` (185–199)                | `(repository_id, verdict_id)`                    |
| `striatumd.blockers`                     | `blockers` (201–213)                | `(repository_id, blocker_id)`                    |
| `striatumd.command_requests`             | `command_requests` (215–225)        | `(repository_id, request_id)`                    |
| `striatumd.process_executions`           | `process_executions` (227–247)      | `(repository_id, process_id)`                    |
| `striatumd.events`                       | `events` (249–260)                  | `(repository_id, event_id BIGINT IDENTITY)`      |
| `striatumd.job_worktrees`                | per RFC 0043 §1 list (V1 in `src/striatum/db.py`) | `(repository_id, worktree_id)` |
| `striatumd.process_supervisors`          | RFC 0031 mirror, V1 in repo-local SQLite          | `(repository_id, supervisor_id)`  |
| `striatumd.process_supervisor_pointers`  | RFC 0031 mirror, V1 in repo-local SQLite          | `(repository_id, pointer_id)`     |

(The prompt and RFC 0043 §1 list 15 tables; `job_dependencies` is included
above because the V1 schema carries it and the daemon mirror cannot lose
the FK. It is a strict additive entry; the prompt list was not exhaustive
of the dependent join table.)

Type adjustments from SQLite to Postgres applied uniformly:

- `TEXT NOT NULL` timestamps stored as ISO-8601 strings in V1 become
  `timestamptz NOT NULL` in Postgres. The migration command (§A.3) parses
  the SQLite ISO strings as `timestamp with time zone` on insert; the
  audit-chain re-anchor preserves wall-clock ordering byte-for-byte (§A.4).
- `TEXT` JSON payloads (`payload_json`, `packet_json`, `lane_selector_json`,
  `capability_requirements_json`, `gate_json`, `command_json`,
  `capabilities_json`, `settings_json`, `last_result_json`) become `jsonb`.
  This matches RFC 0033 §3's existing pattern (`striatumd.repositories.
  settings_json jsonb NOT NULL DEFAULT '{}'::jsonb`,
  `0001_baseline.sql:33`).
- SQLite `INTEGER`-as-boolean (`first_class`, `fresh_context`) become
  `boolean NOT NULL`.
- `events.event_id INTEGER PRIMARY KEY AUTOINCREMENT` becomes
  `event_id bigint GENERATED BY DEFAULT AS IDENTITY` with a deterministic
  seed strategy described in §A.4.

CHECK constraints from V1 are preserved verbatim by inlining the same
enum lists in the Postgres `CHECK` clauses (`runs.state`,
`sessions.state`, `jobs.job_type`, `queue_messages.kind`/`state`,
`leases.resource_type`/`state`, `artifacts.artifact_kind`/`publish_mode`,
`verdicts.verdict`, `blockers.severity`/`state`, `command_requests.state`,
`process_executions.stdin_mode`/`stdio_mode`/`state`).

### A.2 Indexes and role grants

Indexes follow the access patterns in `src/striatum/db.py` and
`src/striatum/cli/introspect.py`. Every index from V1 SQLite at
`schema.py:262–280` is reproduced with `repository_id` as the leading
column:

- `CREATE INDEX idx_runs_state ON striatumd.runs(repository_id, state);`
- `CREATE INDEX idx_sessions_run_state ON striatumd.sessions(repository_id, run_id, state);`
- `CREATE INDEX idx_jobs_run_state ON striatumd.jobs(repository_id, run_id, state, role_id);`
- `CREATE INDEX idx_queue_claimable ON striatumd.queue_messages(repository_id, run_id, state, target_session_id, target_role_id, target_lane_id, visible_after, priority, created_at);`
- `CREATE INDEX idx_events_run_time ON striatumd.events(repository_id, run_id, event_id);`
- `CREATE INDEX idx_events_job ON striatumd.events(repository_id, job_id, event_id);`
- `CREATE INDEX idx_process_executions_run_job ON striatumd.process_executions(repository_id, run_id, job_id, started_at);`
- `CREATE UNIQUE INDEX uq_active_resource_lease ON striatumd.leases(repository_id, resource_type, resource_id) WHERE state = 'active';`
- `CREATE UNIQUE INDEX uq_active_work_message_per_job ON striatumd.queue_messages(repository_id, job_id) WHERE kind = 'work' AND state IN ('pending','claimed','acked');`

Role privileges reuse the RFC 0033 §5 model. The migration runs
`GRANT INSERT, SELECT, UPDATE, DELETE ON ... TO striatumd_rw;` for every
new table, then revokes `UPDATE, DELETE` on `striatumd.events` and
`striatumd.artifacts` from `striatumd_rw` to preserve append-only
enforcement at the role level (parallel to the existing audit-row
revocations in `0001_baseline.sql` per RFC 0033 §5). The migration role
gets `ALL`; analytics / web read role gets `SELECT` only. Role names are
the same `striatumd_rw` / `striatumd_migrate` already named in the V1
daemon-DB grants (no new roles in this RFC).

### A.3 CLI verb: `striatum daemon migrate-repo-local`

Argparse plumbing in `src/striatum/cli/parser.py`. The current
`daemon migrate` subparser (defined `parser.py:150–157`) is the
daemon-registry migration from RFC 0033 and stays unchanged.
A new sibling subparser is added immediately after it:

```python
daemon_migrate_repo = daemon_sub.add_parser("migrate-repo-local")
daemon_migrate_repo.add_argument(
    "--from", dest="from_substrate", choices=["sqlite"], required=True
)
daemon_migrate_repo.add_argument(
    "--to", dest="to_substrate", choices=["pg"], required=True
)
daemon_migrate_repo.add_argument(
    "--repo", dest="repo_path", required=True,
    help="target repository root (must already be `repo add`-registered)"
)
daemon_migrate_repo.add_argument("--postgres-url")
daemon_migrate_repo.add_argument("--dry-run", action="store_true")
daemon_migrate_repo.add_argument(
    "--keep-sqlite-readonly", dest="keep_sqlite_readonly",
    action=argparse.BooleanOptionalAction, default=True,
    help="rename state.sqlite3 -> state.sqlite3.tombstone mode 0444 (default)"
)
daemon_migrate_repo.add_argument(
    "--confirm-delete", action="store_true",
    help="required when --no-keep-sqlite-readonly: irreversibly delete the SQLite file"
)
daemon_migrate_repo.add_argument("--json", action="store_true")
```

Dispatch wiring in `src/striatum/cli/dispatch.py::_dispatch_daemon`
(currently lines 849–896). A new branch routes to a new module:

```python
if args.daemon_command == "migrate-repo-local":
    from striatum.daemon_pg.repo_local_cutover import (
        RepoLocalCutoverOptions, migrate_repo_local,
    )
    config = resolve_config(postgres_url=getattr(args, "postgres_url", None))
    if config.url is None:
        raise StriatumError("daemon PostgreSQL URL is not configured", exit_code=13)
    return migrate_repo_local(RepoLocalCutoverOptions(
        repo_path=Path(args.repo_path),
        postgres_url=config.url,
        dry_run=bool(args.dry_run),
        keep_sqlite_readonly=bool(args.keep_sqlite_readonly),
        confirm_delete=bool(args.confirm_delete),
    ))
```

New module: `src/striatum/daemon_pg/repo_local_cutover.py`. It mirrors
the existing `src/striatum/daemon_pg/cutover.py` shape (the
`CutoverOptions` dataclass and `migrate()` function at `cutover.py:21–73`)
but operates on per-repo workflow tables instead of daemon-global
registry tables. Public surface:

```python
@dataclass(frozen=True)
class RepoLocalCutoverOptions:
    repo_path: Path
    postgres_url: str
    dry_run: bool = False
    keep_sqlite_readonly: bool = True
    confirm_delete: bool = False

def migrate_repo_local(options: RepoLocalCutoverOptions) -> dict[str, object]: ...
```

Behavior (mapped to RFC 0043 §4 steps):

1. **Authorization** (RFC 0043 §4 step 1). Calls a new helper
   `striatum.daemon_pg.connection::authorize_admin(postgres_url)` that
   verifies the connection role is `striatumd_migrate` (or carries an
   admin capability row in `striatumd.client_capabilities`); refuses with
   `exit_code=4` (existing `InvalidTransitionError` shape) otherwise.

2. **Repo registration** (RFC 0043 §4 step 2). Looks up the repository
   in `striatumd.repositories` via the existing
   `striatum.daemon::repo_list()` / `repo_add()` helpers. If absent,
   calls `repo_add(Path(repo_path), display_name=None, no_migrate=True,
   init=False)` implicitly and prints the resulting `repository_id` in
   the JSON envelope under a new `implicit_repo_add` key. The
   `init=False` is the explicit short-circuit that avoids creating a
   fresh SQLite file we are trying to retire.

3. **SQLite source check** (RFC 0043 §4 step 3). Opens
   `<repo_path>/.striatum/state.sqlite3` read-only via
   `sqlite3.connect("file:...?mode=ro", uri=True)`. Reads
   `PRAGMA user_version` and compares to
   `striatum.migrations.LATEST_VERSION` (existing constant in
   `migrations.py`). If lower, raises `StriatumError(... exit_code=9)`
   pointing the operator at `striatum migrate` to bring the local
   SQLite forward first. If the SQLite file is missing or fails the
   integrity check (`PRAGMA integrity_check`), exits with the
   `exit_code=8` "structural" path (RFC 0043 §4 step 4).

4. **Dry-run** (RFC 0043 §4 step 4). Returns
   `{"mode": "repo_local_cutover", "dry_run": True,
   "source_counts": {...}, "source_event_log_head_hash": "...",
   "source_event_log_count": N, "destination_schema_version": M,
   "implicit_repo_add": "...?"}`. `source_counts` enumerates row counts
   per V1 table (15 tables from §A.1); `source_event_log_head_hash` is
   the SHA-256 of the canonical-JSON serialization of the latest row
   in `events`, the V1 audit-chain head equivalent for repo-local
   state.

5. **Full run** (RFC 0043 §4 step 5). Opens a single Postgres
   transaction at SERIALIZABLE isolation using `psycopg.Connection.
   transaction(isolation=psycopg.IsolationLevel.SERIALIZABLE)`
   (matches `daemon_pg/cutover.py` shape). Inserts every row preserving
   the V1 `created_at` and event-log ordinal. The event_id sequence on
   `striatumd.events` is advanced to `max(event_id)+1` post-insert via
   `setval()`. The audit chain re-anchor follows RFC 0043 §4 step 5 and
   reuses the byte-equivalence approach from RFC 0033 §5 / `cutover.py::
   _source_final_hash`: the migration replays each V1 event row in
   ascending `event_id` order, computes the canonical SHA-256 against
   the previous-hash chain, and refuses (rollback) if any computed
   hash diverges from the V1 source byte-for-byte. The checkpoint
   marker is a new daemon-DB table `striatumd.repo_migrations`:

   ```sql
   CREATE TABLE striatumd.repo_migrations (
     repository_id text PRIMARY KEY REFERENCES striatumd.repositories(repository_id),
     migrated_at timestamptz NOT NULL,
     source_state_db_sha256 text NOT NULL,
     source_event_log_head_hash text,
     row_counts jsonb NOT NULL,
     keep_sqlite_readonly boolean NOT NULL
   );
   ```

   `source_state_db_sha256` is the SHA-256 of the read-only SQLite
   file at migration time, captured before any rename (§A.5).

6. **Tombstone** (RFC 0043 §4 step 6). With `keep_sqlite_readonly=True`
   (default), after the Postgres transaction commits the command
   renames `<repo>/.striatum/state.sqlite3` to
   `<repo>/.striatum/state.sqlite3.tombstone` and sets mode `0o444` via
   `Path.chmod(0o444)`. The rename happens after the commit so a
   crash between commit and rename is recoverable (idempotent re-run,
   step 8).

7. **Explicit delete** (RFC 0043 §4 step 7). With
   `keep_sqlite_readonly=False` and `confirm_delete=True`, deletes the
   file. Refuses with `exit_code=4` if `confirm_delete` is missing.

8. **Idempotent re-run** (RFC 0043 §4 step 8). Selects from
   `striatumd.repo_migrations` for the repository_id at command start.
   If a row exists, returns
   `{"mode": "repo_local_cutover", "already_migrated": True,
   "migrated_at": "...", "row_counts": {...}}` and exits 0 without
   reopening the SQLite file or touching Postgres.

Error code 11 (`daemon_unreachable`) is raised at step 1 if the
Postgres connection fails (matches RFC 0043 §4 final paragraph; the
migration command is itself a daemon-mediated operation).

### A.4 Audit-chain re-anchor (byte-equivalent)

RFC 0043 §4 step 5 requires "byte-equivalent re-anchor." Today's
V1 repo-local `events` table is not itself hash-chained (only the
daemon-DB `audit_log` is, per `0001_baseline.sql:83–101`). The V1
audit chain that needs preservation is the **daemon-DB audit chain**
that records every RPC call against this repository — already
migrated by RFC 0033 §4. For repo-local migration, "byte-equivalent
re-anchor" means a different invariant: the canonical-JSON SHA-256 of
each row inserted into `striatumd.events` matches the canonical-JSON
SHA-256 of the corresponding V1 SQLite row.

Implementation: a new helper
`striatum.daemon_pg.repo_local_cutover::_row_canonical_sha256(row,
fields)` computes the SHA-256 of `json.dumps(row, sort_keys=True,
separators=(",", ":"), default=str)`. The migration computes this
hash per row in the V1 source (left side) and again in the Postgres
destination (right side) post-insert via a `SELECT row_to_json` and
verifies left==right inside the same transaction. Mismatch rolls
back; the operator's V1 SQLite is unchanged because
`keep_sqlite_readonly` defaults true and the tombstone rename has
not yet run.

For the `events` table specifically, every row's `event_id` is
preserved verbatim (the V1 column is `INTEGER PRIMARY KEY
AUTOINCREMENT`; the Postgres column is `bigint GENERATED BY DEFAULT
AS IDENTITY`, which accepts explicit values). The identity sequence
is advanced post-import with `SELECT setval(pg_get_serial_sequence(
'striatumd.events', 'event_id'), (SELECT MAX(event_id) FROM
striatumd.events WHERE repository_id = $1));`. Subsequent rows
inserted by the daemon under this repository_id continue the V1
ordinal without collision.

### A.5 Test fixture and coverage

Fixture: `tests/fixtures/v1_repo_local_sqlite/state.sqlite3`. Generated
by a checked-in builder script
`tests/fixtures/v1_repo_local_sqlite/build_fixture.py` that runs
`striatum init` against a temp repo, prepares one workflow, claims
and completes two jobs, publishes one artifact, records one verdict,
opens one blocker, and writes one process_executions row — enough
coverage that every CHECK constraint and every FK is exercised on
import. The fixture is committed at the V1
`PRAGMA user_version = striatum.migrations.LATEST_VERSION` snapshot;
when LATEST_VERSION bumps, the builder is re-run and the fixture is
refreshed (the `state_db_sha256` recorded in the test golden
updates with it).

Test files under `tests/`:

- `tests/test_migrate_repo_local_dry_run.py` — opens the fixture
  inside the `MultiRepoHarness` (RFC 0035, `tests/_harness/
  MultiRepoHarness`), runs `striatum daemon migrate-repo-local
  --from sqlite --to pg --dry-run --repo <tmp>`, asserts the
  envelope shape and that no Postgres tables gained rows
  (`SELECT COUNT(*)` on every table in §A.1).
- `tests/test_migrate_repo_local_full.py` — runs without
  `--dry-run`; asserts row counts match the fixture; asserts
  per-row canonical-JSON SHA-256 byte-equivalence (the §A.4 invariant);
  asserts `striatumd.repo_migrations` row was written; asserts
  `state.sqlite3.tombstone` exists with mode `0o444` and
  `state.sqlite3` does not.
- `tests/test_migrate_repo_local_idempotent.py` — runs migration
  twice; the second invocation returns `already_migrated=True`
  without touching the tombstone (file mtime unchanged).
- `tests/test_migrate_repo_local_tombstone_modes.py` — covers
  `--no-keep-sqlite-readonly --confirm-delete` (file deleted) and
  the refusal when `--confirm-delete` is missing
  (`exit_code=4`).
- `tests/test_migrate_repo_local_unmigrated_pragma.py` — fixture
  with a deliberately-lowered `PRAGMA user_version` refuses with
  `exit_code=9` and points the operator at `striatum migrate`.

Per RFC 0033 §7 / RFC 0035, the multi-repo harness already boots an
ephemeral Postgres; the new tests reuse the same fixture path. No
new test infrastructure is required.

## Track B — CLI surface + RPC method registry

### B.1 Retire `--no-daemon` (and `--daemon`) at the parser

`src/striatum/cli/parser.py:20–30` currently defines a mutually-
exclusive group of `--daemon` / `--no-daemon`. RFC 0043 §3 retires
direct CLI mode entirely; the mutually-exclusive group is removed
and both flags are dropped from `build_parser()`. After the change,
`striatum --no-daemon status …` and `striatum --daemon status …`
both produce argparse's standard "unrecognized arguments" error
(exit code 2 from argparse itself, matching the RFC 0043 §3 phrase
"the standard unknown-option error" — argparse already prints the
canonical message).

The parser-level removal is mechanical: delete the
`daemon_group = parser.add_mutually_exclusive_group()` block and
its two `add_argument` calls at `parser.py:20–30`. Any test that
previously passed `--no-daemon` is rewritten in Track B's test
updates (§B.4).

### B.2 Exit codes 11 and 12

`src/striatum/errors.py` gains two new error classes mirroring the
existing pattern at `errors.py:14–68`:

```python
class DaemonUnreachableError(StriatumError):
    """Raised when no daemon is reachable on the configured socket."""

    def __init__(self, message: str, *, socket_path: str | None = None) -> None:
        super().__init__(message, exit_code=11)
        self.socket_path = socket_path


class RepoNotMigratedError(StriatumError):
    """Raised when a repo's local SQLite has not been cut over to Postgres."""

    def __init__(self, message: str, *, repo_path: str | None = None) -> None:
        super().__init__(message, exit_code=12)
        self.repo_path = repo_path
```

`StriatumError` already has the `exit_code` plumbing wired through
`dispatch.py:88–103`, so the JSON envelope and stderr path inherit
the new codes for free.

**Existing code 12 conflict.** `src/striatum/cli/dispatch.py:182–185`
and `:962–967` raise `StriatumError(..., exit_code=12)` today as the
"command is not daemon-routable in V1" refusal. Those branches go
away with `--daemon` retirement (§B.1) — they are unreachable once
the flag is parsed away. Track B deletes them in the same patch as
the parser change. No grandfathered code-12 callers remain.

**Daemon-unreachable detection.** A new helper
`src/striatum/daemon_rpc/client.py::ensure_daemon_reachable()` opens
the Unix socket from the existing transport layer
(`src/striatum/daemon_rpc/transport_unix.py`) and, on `ConnectionRefusedError`,
`FileNotFoundError`, or `PermissionError`, raises
`DaemonUnreachableError` with the socket path that was tried.
`dispatch.main()` catches `DaemonUnreachableError` ahead of the
generic `StriatumError` block and writes a multi-line stderr
remediation (matched to RFC 0043 §3):

```
striatum: daemon is not reachable at <socket_path>
  start it via one of:
    • systemd:    systemctl --user start striatumd
    • macOS LaunchAgent: launchctl bootstrap gui/$UID ~/Library/LaunchAgents/io.striatum.daemon.plist
    • foreground: striatumd --foreground
  if Postgres is not installed, see `striatum daemon doctor --postgres-install-hints`.
```

Stderr remediation is constructed by a new
`src/striatum/cli/daemon_remediation.py::format_remediation(socket_path)`
helper that branches on `sys.platform` (`linux` → systemd snippet,
`darwin` → launchctl snippet; the foreground reminder and Postgres
hint are unconditional). The Postgres install hints live in
`daemon doctor` (RFC 0033 §8 already names them) and are referenced
by message, not duplicated.

**Repo-not-migrated detection.** A new helper
`striatum.daemon_pg.repo_local_cutover::ensure_repo_migrated(
postgres_url, repo_path)` queries
`striatumd.repo_migrations` for the repository_id (resolved via
`striatumd.repositories`) and raises `RepoNotMigratedError(repo_path=...)`
if no row exists. Stderr message:

```
striatum: repository <repo_path> has not been migrated to Postgres
  run: striatum daemon migrate-repo-local --from sqlite --to pg --repo <repo_path>
```

`ensure_repo_migrated` is invoked at the top of every mutating dispatch
path (where the current code calls `ensure_initialized(repo)` at
`dispatch.py:373`) inside a new
`dispatch.py::_resolve_repo_or_refuse(repo)` shim. Read verbs that
still operate without the daemon (`--help`, `--version`, and
`daemon doctor`) bypass the shim — `daemon doctor` is called out
specifically by RFC 0043 §3 as the "runs even when the daemon is
down" verb because it reads configuration only.

### B.3 RFC 0030 method registry expansion

`src/striatum/daemon_rpc/registry.py:48–100` currently lists 50
methods. Track B grows the registry to cover every mutation in
`src/striatum/cli/mutations.py` per RFC 0043 §5 plus matching
read-capability methods. The table below pins each new method to
the corresponding CLI verb and to a registry-pattern citation from
the V2 daemon (RFC 0030 §4 / `registry.py`).

| New / renamed method                       | Required capability  | Scope        | Wired CLI verb                                                                                          | Registers existing handler                                        |
|--------------------------------------------|----------------------|--------------|---------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------|
| `session.register` (was `session.register`) | `claim`              | single_repo  | `register-session` (`dispatch.py:447`)                                                                  | `cli.mutations.register_session`                                  |
| `session.close`                            | `write`              | single_repo  | `session close` (`parser.py:380`)                                                                       | `cli.mutations.close_session`                                     |
| `work.claim_next` (rename `claim_next`)    | `claim`              | single_repo  | `claim-next` (`parser.py:386`)                                                                          | `db.claim_next`                                                   |
| `work.ack` (rename `ack`)                  | `claim`              | single_repo  | `ack`                                                                                                   | `cli.mutations.ack_work`                                          |
| `work.heartbeat` (rename `heartbeat`)      | `claim`              | single_repo  | `heartbeat`                                                                                             | `cli.mutations.heartbeat`                                         |
| `work.release` (rename `release`)          | `claim`              | single_repo  | `release`                                                                                               | `cli.mutations.release_work`                                      |
| `work.block` (rename `block`)              | `write`              | single_repo  | `block`                                                                                                 | `cli.mutations.block_work`                                        |
| `work.complete` (rename `complete`)        | `write`              | single_repo  | `complete`                                                                                              | `db.complete_job`                                                 |
| `work.send_message`                        | `write`              | single_repo  | `send` (`parser.py:405`)                                                                                | `cli.mutations.send_message`                                      |
| `artifact.publish` (rename `publish_artifact`) | `write`          | single_repo  | `publish-artifact`                                                                                       | `artifacts.publish_artifact`                                      |
| `review.submit` (was `submit_review`)      | `review`             | single_repo  | `submit-review` (`parser.py:461`)                                                                       | `cli.mutations.submit_review`                                     |
| `review.verdict` (was `verdict`)           | `review`             | single_repo  | `verdict`                                                                                               | `cli.mutations.verdict_work`                                      |
| `review.override_verdict`                  | `review`             | single_repo  | `override-verdict` (`parser.py:449`)                                                                    | `db.override_review_verdict`                                      |
| `decision.record`                          | `admin`              | single_repo  | `decision record` (`parser.py:490`)                                                                     | `cli.mutations.decision_record`                                   |
| `checkpoint.resolve`                       | `admin`              | single_repo  | `checkpoint resolve` (`parser.py:676`)                                                                  | `cli.mutations.checkpoint_resolve`                                |
| `recovery.stale_leases`                    | `recovery`           | single_repo  | `recovery stale-leases` (`parser.py:525`)                                                               | `cli.recovery.stale_leases` *(already present, audit class added)*|
| `recovery.requeue_stale`                   | `recovery`           | single_repo  | `recovery requeue-stale`                                                                                | `cli.recovery.requeue_stale` *(already present)*                  |
| `recovery.cancel_job`                      | `recovery`           | single_repo  | `recovery cancel-job`                                                                                   | `cli.recovery.cancel_job` *(already present)*                     |
| `recovery.process_reconcile`               | `recovery`           | single_repo  | `recovery process-reconcile` (`parser.py:542`)                                                          | `cli.recovery.process_reconcile` *(already present)*              |
| `recovery.resume`                          | `recovery`           | single_repo  | `recovery resume` (`parser.py:552`)                                                                     | `cli.recovery.resume_blocker` *(already present)*                 |
| `recovery.auto`                            | `recovery`           | single_repo  | `recovery auto` (`parser.py:584`)                                                                       | `recovery.run_auto_sweep`                                         |
| `recovery.watch`                           | `recovery`           | single_repo  | `recovery watch` (`parser.py:625`)                                                                      | `recovery.run_watch`                                              |
| `worktree.create`                          | `write`              | single_repo  | `worktree create` (`parser.py:713`)                                                                     | `cli.worktree.worktree_create`                                    |
| `worktree.release`                         | `write`              | single_repo  | `worktree release` (`parser.py:718`)                                                                    | `cli.worktree.worktree_release`                                   |
| `worktree.list`                            | `read`               | single_repo  | `worktree list`                                                                                         | `cli.worktree.worktree_list`                                      |
| `branch.confirm`                           | `admin`              | single_repo  | `branch confirm` (`parser.py:344`)                                                                      | `cli.mutations.branch_confirm`                                    |
| `run.prepare`                              | `admin`              | single_repo  | `run prepare` (`parser.py:299`)                                                                         | `workflow.create_run`                                             |
| `run.start`                                | `admin`              | single_repo  | `run start`                                                                                             | `cli.mutations.run_start`                                         |
| `run.pause`                                | `admin`              | single_repo  | `run pause` (`parser.py:309`)                                                                           | `db.pause_run`                                                    |
| `run.resume`                               | `admin`              | single_repo  | `run resume`                                                                                            | `db.resume_run`                                                   |
| `run.cancel`                               | `admin`              | single_repo  | `run cancel`                                                                                            | `db.cancel_run`                                                   |
| `run.retry_job`                            | `admin`              | single_repo  | `run retry-job` (`parser.py:316`)                                                                       | `db.retry_job`                                                    |
| `run.summary`                              | `read`               | single_repo  | `run summary` (`parser.py:320`)                                                                         | `cli.run_summary.run_summary_export`                              |
| `run.graph`                                | `read`               | single_repo  | `run graph` (`parser.py:324`)                                                                           | `cli.introspect.run_graph`                                        |
| `workflow.validate`                        | `read`               | single_repo  | `workflow validate` (`parser.py:209`) — capability **changed from `write` → `read`** per RFC 0043 §5    | `workflow.validate_workflow`                                      |
| `workflow.plan`                            | `read`               | single_repo  | `workflow plan`                                                                                         | `workflow.plan_workflow`                                          |
| `workflow.graph`                           | `read`               | single_repo  | `workflow graph`                                                                                        | `workflow.workflow_graph_*`                                       |
| `workflow.init`                            | `write`              | single_repo  | `workflow init` (`parser.py:219`)                                                                       | `cli.workflow_init.workflow_init`                                 |
| `workflow.upgrade`                         | `write`              | single_repo  | `workflow upgrade` (`parser.py:228`)                                                                    | `cli.workflow.workflow_upgrade`                                   |
| `workflow.generate`                        | `write`              | single_repo  | `workflow generate`                                                                                     | `workflow_generator.generate_workflow`                            |
| `workflow.templates.list`                  | `read`               | daemon_global | `workflow templates list` (`parser.py:265`)                                                            | `workflow_generator.catalog.list_templates`                       |
| `workflow.templates.show`                  | `read`               | daemon_global | `workflow templates show`                                                                              | `workflow_generator.catalog.get_template`                         |
| `evidence.export`                          | `read`               | single_repo  | `evidence export` (`parser.py:476`) *(already present, audit class verified)*                          | `cli.evidence.evidence_export`                                    |
| `corpus.export`                            | `read`               | single_repo  | `corpus export` (`parser.py:483`)                                                                       | `corpus.export_corpus_bundle`                                     |
| `list.runs` / `list.sessions` / `list.jobs` / `list.artifacts` / `list.workflows` | `read` | single_repo | `list runs`/`list sessions`/`list jobs`/`list artifacts` (`parser.py:788`), `list workflows` daemon_global | `cli.list_commands.list_*`                          |
| `status`                                   | `read`               | single_repo  | `status` *(already present)*                                                                            | `cli.introspect.status`                                           |
| `why`                                      | `read`               | single_repo  | `why` *(already present)*                                                                               | `cli.introspect.why`                                              |
| `doctor`                                   | `read`               | single_repo  | `doctor` *(already present)*                                                                            | `cli.introspect.doctor`                                           |
| `dashboard`                                | `read`               | single_repo  | `dashboard --run-id` *(already present)*                                                                | `dashboard.run`                                                   |
| `dashboard.all`                            | `read`               | daemon_global | `dashboard --all` *(already present)*                                                                  | `daemon.dashboard_all`                                            |

Adapter / supervise / apply methods (`supervise.*`, `apply.*`,
`daemon.token.*`, `repo.*`, `cross_repo.*`) are already present in
`registry.py:79–99` and gain only audit-class verification, not new
entries.

**Rename strategy.** Today's registry uses the bare verb name as
the method id (`claim_next`, `ack`, `complete`, `publish_artifact`,
`verdict`, `submit_review`). RFC 0043 §5 standardizes on a dotted
vocabulary (`work.claim_next`, `work.ack`, `work.complete`,
`artifact.publish`, `review.verdict`, `review.submit`). The old
names stay in the registry for one minor release with
`deprecated=True`, citing the `MethodEntry.deprecated` field at
`registry.py:27`. The `daemon.describe` payload (`registry.py:108`)
exposes both names; CLI clients prefer the new names. `methods_etag`
(`registry.py:103`) recomputes on the rename, which is the RFC 0030
§3 mechanism that forces clients to re-fetch the registry.

**Audit class.** Every new entry gets an explicit `audit_class` —
existing entries default to `"metadata"` per
`registry.py:25`. Capability-write methods set `audit_class="mutation"`
to match the RFC 0030 §5 audit-row vocabulary; capability-read
methods stay `"metadata"`.

**Server dispatch.** `src/striatum/daemon_rpc/server.py` routes
incoming envelopes to handler functions. New methods get a thin
adapter in a new module
`src/striatum/daemon_rpc/handlers/repo_local.py` that translates
each envelope's `params` into the existing CLI mutation signature
and calls the corresponding helper in `cli.mutations`,
`cli.recovery`, `cli.worktree`, etc. — the CLI handlers stay the
single source of mutation logic; the RPC server is a parameter
adapter on top of them. This mirrors the existing pattern in
`src/striatum/daemon_rpc/server.py` where `daemon.describe` and
`daemon.hello` are thin adapters over `registry.describe_methods()`
and `handshake.welcome()`.

### B.4 `daemon doctor` and stub coverage

RFC 0043 §3 states: "`daemon doctor` continues to run without the
daemon — it touches configuration only." Today's implementation at
`src/striatum/cli/dispatch.py:858–869` and
`src/striatum/daemon_pg/connection.py::doctor` already runs without
a live daemon process; the change is to extend the JSON envelope
with three new keys:

- `"substrate"`: `"pg"` (the only supported substrate post-RFC-0043).
- `"schema_version"`: integer from
  `striatum.daemon_pg.migrations.read_schema_version()` — already
  present, surface as a top-level key instead of nested.
- `"audit_chain"`: result of the existing
  `striatum.daemon_pg.audit::verify_chain()` helper.

A new section reports Postgres install hints when the
`STRIATUM_DAEMON_DB_URL` is unset; the existing
`daemon_pg.connection.doctor` already accepts an `apply` flag and
distinguishes URL-missing from URL-bad cases. The platform-specific
hint message lives in `src/striatum/daemon_pg/install_hints.py`
(new module) keyed by `sys.platform`. The `--help` and `--version`
paths bypass `ensure_daemon_reachable` via the parser's existing
`action="version"` shape; no change needed.

### B.5 Stub-out plan for `src/striatum/db.py` and `src/striatum/migrations.py`

RFC 0043 §3 implementation directive: "Every `striatum` CLI verb
routes through the daemon RPC envelope per RFC 0030." Track B keeps
the V1 SQLite modules in place during the same release that ships
the migration command (operators still need them to read tombstones
or run `striatum migrate` to bring SQLite forward before
`migrate-repo-local`). Track B does **not** delete `src/striatum/db.py`
or `src/striatum/schema.py`; that is acceptance-criterion territory
named in RFC 0043 explicitly as "retired or reduced to the
daemon-client schema layer." This RFC scope is the substrate flip
and CLI surface; the Python module retirement is dogfood-049 (or
later) follow-up.

What Track B **does** do: every CLI dispatch path that currently
calls `ensure_initialized(repo)` + `connect(repo)` (the SQLite
context at `dispatch.py:373–374`) is rewritten to:

```python
postgres_url = resolve_config(...).url
ensure_daemon_reachable(postgres_url)
ensure_repo_migrated(postgres_url, repo)
client = DaemonRpcClient(postgres_url)
return client.call(method=<entry>, params={...})
```

The `cli.mutations.*` helpers retain their SQLite signatures during
the transition. The new RPC server handlers in
`daemon_rpc/handlers/repo_local.py` translate Postgres-backed
parameters into a per-request SQLite-shim connection bound by the
daemon to the daemon-DB Postgres connection pool, calling the same
helper functions. This is the **temporary** shim path RFC 0043
implicitly requires while `db.py` is still SQLite-shaped; full
removal of the SQLite shim is the dogfood-049 follow-up. Track B
makes the shim explicit, names the file
(`src/striatum/daemon_rpc/handlers/sqlite_compat_shim.py`), and
documents its retirement with a `# RFC 0043 transitional shim:
remove in dogfood-049 follow-up` header so readers know not to
build new callers on it.

### B.6 Test paths

Track B test coverage lives in:

- `tests/test_cli_no_daemon_retired.py` — passes `--no-daemon` /
  `--daemon` and asserts argparse `SystemExit(2)` with the canonical
  "unrecognized arguments" message in stderr.
- `tests/test_daemon_unreachable.py` — runs every CLI verb against a
  configured-but-unreachable socket path and asserts
  `exit_code=11`, the socket path is in stderr, and the
  platform-specific remediation line appears. Linux variant is the
  CI default; macOS variant runs on darwin runners only.
- `tests/test_repo_not_migrated.py` — registers a repo via `repo
  add --init=false` with no migration row, runs a mutating verb,
  asserts `exit_code=12` and the `migrate-repo-local` remediation.
- `tests/test_method_registry_completeness.py` — iterates the
  parser's subcommand tree (`build_parser()`) and asserts every
  mutating verb has a matching `MethodEntry` with a non-`None`
  `required_capability`. The test fails closed: any new CLI verb
  added without a matching registry entry breaks CI.
- `tests/test_method_registry_legacy_aliases.py` — asserts
  the deprecated entries (`claim_next`, `ack`, `complete`,
  `publish_artifact`, `verdict`, `submit_review`) coexist with
  the new dotted names and that `daemon.describe` surfaces both.

Existing daemon-RPC tests under `tests/test_daemon_rpc_*.py` are
unaffected; they exercise envelope shape and version handshake,
neither of which changes.

### B.7 Lane attestation and capability boundary

Per RFC 0030 §4 / `registry.py::CAPABILITIES`, every method carries
a single required capability. The CLI client today (no separate
operator-token flow in V1) implicitly carries the daemon-startup
admin grant. RFC 0043 V1 does not introduce per-verb token issuance
beyond what the current daemon ships (RFC 0030 §4 covers token
creation/revocation via `daemon.token.create`, already present in
the registry). The relevant change Track B makes is the registry
entries themselves — once the registry covers every mutation,
`daemon doctor` can answer the question "what can this token do?"
and the existing capability scope check
(`daemon_rpc/capability.py`) refuses unknown method ids per
RFC 0030 §3 with `exit_code=10`.

`workflow.validate` capability is downgraded from `write` (today,
`registry.py:57`) to `read` — RFC 0043 §5 example table lists
`workflow.validate` as `read`. The downgrade is safe because
`validate_workflow()` is read-only (no SQLite or Postgres writes).
The CHANGELOG note for this downgrade is operator-owned per the
prompt's out-of-scope list.

## Cross-track coordination

- **Daemon-DB migration ordering.** Track A's
  `0005_repo_local_tables.sql` lands in the same release as Track
  B's `--no-daemon` retirement. Operators upgrade the daemon
  (Postgres schema flips forward automatically per
  `daemon_pg.migrations::apply_migrations`), then run
  `striatum daemon migrate-repo-local --repo <path>` for each
  registered repo. The new CLI version refuses any mutation on an
  unmigrated repo (exit 12), so an operator who upgrades the CLI
  but forgets the per-repo migration sees the documented
  remediation immediately.
- **Method registry ordering.** Track B's registry expansion is
  additive (plus deprecation of six legacy aliases) and does not
  break clients on the prior `methods_etag`. The deprecated entries
  let the prior CLI continue to call the old method names against
  a daemon that knows both — relevant only for the daemon-upgrade-
  before-CLI-upgrade case.
- **`daemon migrate` vs `daemon migrate-repo-local`.** The two
  commands stay distinct. `daemon migrate` (RFC 0033 §4) is the
  daemon-global registry cutover that already shipped; it is run
  once per daemon install. `daemon migrate-repo-local` (RFC 0043
  §4, Track A here) is per-repository and runs once per registered
  repo. Operators who already ran `daemon migrate` are not asked
  to re-run it; the daemon doctor envelope distinguishes the two
  checkpoints (`pg_cutover_completed_at` for registry, the new
  `repo_migrations` table for per-repo).

## Open questions explicitly inherited from RFC 0043

These open questions are inherited unresolved into the implementation
and are flagged for the review lane to challenge:

- **Schema namespacing** (RFC 0043 Open Q1). Track A takes the
  single-schema-with-`repository_id`-columns recommendation.
- **Connection-pool sizing** (RFC 0043 Open Q2). Track A assumes the
  daemon's existing `psycopg` pool; no new pool. RFC 0043 §2
  recommendation matches.
- **Migration command authorization** (RFC 0043 Open Q3). Track A
  requires explicit daemon admin (`striatumd_migrate` role or admin
  capability row), matching the RFC 0033 daemon-migration posture.
- **Exit code numbering** (RFC 0043 Open Q4). Track B reserves 11
  and 12 as specified; no other RFC has claimed them in flight on
  this branch.
- **Direct-mode retirement timing** (RFC 0043 Open Q7). Track B
  takes the immediate retirement (no deprecation cycle) the RFC
  recommends. Reviewers may push back here; the alternative is one
  release with `--no-daemon` printing a "direct mode is being
  retired" warning before the parser delete in the next release.

## Files this RFC will touch (summary)

Track A (codex):
- New: `src/striatum/daemon_pg/sql/0005_repo_local_tables.sql`
- New: `src/striatum/daemon_pg/repo_local_cutover.py`
- New: `tests/fixtures/v1_repo_local_sqlite/state.sqlite3`,
  `tests/fixtures/v1_repo_local_sqlite/build_fixture.py`
- New: `tests/test_migrate_repo_local_*.py` (5 files, §A.5)
- Modified: `src/striatum/cli/parser.py` (add `migrate-repo-local` subparser, §A.3)
- Modified: `src/striatum/cli/dispatch.py::_dispatch_daemon` (new
  `migrate-repo-local` branch, §A.3)
- Modified: `src/striatum/daemon_pg/migrations.py` (loads
  `0005_repo_local_tables.sql` via existing version-iteration loop;
  no API change)

Track B (claude):
- Modified: `src/striatum/cli/parser.py:20–30` (remove `--daemon` /
  `--no-daemon`)
- Modified: `src/striatum/errors.py` (add `DaemonUnreachableError`,
  `RepoNotMigratedError`)
- Modified: `src/striatum/cli/dispatch.py` (remove `--daemon`
  routing branches at `:179–185` and `:950–968`; introduce
  `_resolve_repo_or_refuse`; wire `ensure_daemon_reachable` /
  `ensure_repo_migrated`)
- Modified: `src/striatum/daemon_rpc/registry.py` (expand `_ENTRIES`
  per §B.3 table; add deprecation aliases; recompute
  `METHODS_ETAG`)
- New: `src/striatum/daemon_rpc/handlers/repo_local.py`,
  `src/striatum/daemon_rpc/handlers/sqlite_compat_shim.py`
- New: `src/striatum/cli/daemon_remediation.py`
- New: `src/striatum/daemon_pg/install_hints.py`
- Modified: `src/striatum/daemon_pg/connection.py::doctor` (surface
  `substrate`/`schema_version`/`audit_chain` keys)
- New: `tests/test_cli_no_daemon_retired.py`,
  `tests/test_daemon_unreachable.py`,
  `tests/test_repo_not_migrated.py`,
  `tests/test_method_registry_completeness.py`,
  `tests/test_method_registry_legacy_aliases.py`

Out of scope (operator-owned, per prompt): `README.md`,
`docs/TODO.md`, `CHANGELOG.md`, `docs/SPEC.md`,
`docs/HOW_TO_HUMAN.md`, `docs/HOW_TO_AGENT.md`,
`docs/CLI_REFERENCE.md`, `docs/MCP.md`,
`docs/UBIQUITOUS_LANGUAGE.md`, RFC 0039 revision.
