---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/048/design/codex/DESIGN.md", "docs/dogfood/048/design/claude_code/DESIGN.md", "docs/dogfood/048/design/gemini/DESIGN.md"]
---
author: designer-unknown-model-002

# Design Synthesis: RFC 0043 V1

RFC 0043 lands as one substrate flip: repo-local workflow state moves into daemon-owned PostgreSQL, all normal CLI verbs route through the daemon RPC envelope, and `.striatum/state.sqlite3` becomes a migration source or read-only tombstone only. The build is split into two tracks that can proceed in parallel once the shared schema and method vocabulary below are accepted.

The substrate strategy is a single shared daemon schema, `striatumd.*`, with `repository_id text NOT NULL` on every repo-scoped table. This confirms RFC 0043's single-schema recommendation while using the schema name Striatum already owns in `0001_baseline.sql`; per-repo schemas would make cross-repo queries and registry joins harder without adding useful local isolation.

## Track A: Schema And migrate-repo-local

Track A owns the Postgres schema, the SQLite-to-Postgres migration command, and the migration fixture tests.

Add `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql`, then update `src/striatum/daemon_pg/migrations.py` by changing `LATEST_DAEMON_DB_VERSION` from `4` to `5` and appending `PgMigration(5, "repo-local workflow state", "0005_repo_local_workflow_state.sql")` to `MIGRATIONS`. The migration creates repo-local workflow tables in dependency order:

`workflow_snapshots`, `runs`, `sessions`, `jobs`, `job_dependencies`, `queue_messages`, `leases`, `work_packets`, `artifacts`, `verdicts`, `blockers`, `command_requests`, `process_executions`, `events`, `job_worktrees`, `process_supervisors`, and `process_supervisor_pointers`.

The prompt names 15 tables, but `workflow_snapshots` and `job_dependencies` are required structural tables in `src/striatum/schema.py`; omitting them would break `runs.workflow_snapshot_id` and job gating. Preserve string ids as `text`; current ids are prefixed strings, not raw UUIDs. Use `jsonb` for `*_json` columns, `boolean` for current 0/1 booleans, and `timestamptz` only for timestamp fields where the source value is semantically time. Because `striatumd.repositories.repository_id` is currently `text`, use `repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id)` rather than forcing a UUID conversion into this migration.

The index strategy is repository-prefixed versions of the current SQLite access paths plus the newer migration indexes:

- `runs(repository_id, state)` and partial `runs(repository_id, cross_repo_run_id) WHERE cross_repo_run_id IS NOT NULL`.
- `sessions(repository_id, run_id, state)`, unique `(repository_id, run_id, slug)`, and unique `(repository_id, run_id, role_id, lane_id, ordinal)`.
- `jobs(repository_id, run_id, state, role_id)`, unique `(repository_id, run_id, workflow_job_id, attempt)`, and unique `(repository_id, run_id, idempotency_key)`.
- `queue_messages(repository_id, run_id, state, target_session_id, target_role_id, target_lane_id, visible_after, priority, created_at)` and partial unique `(repository_id, job_id) WHERE kind = 'work' AND state IN ('pending','claimed','acked')`.
- `leases(repository_id, resource_type, resource_id) WHERE state = 'active'`.
- `work_packets(repository_id, run_id, session_id)` and unique `(repository_id, message_id, lease_id)`.
- `artifacts` unique `(repository_id, run_id, job_id, logical_name)` and `(repository_id, run_id, repo_path, content_sha256)`.
- `verdicts(repository_id, posture)` plus unique `(repository_id, job_id, session_id)`.
- `events(repository_id, run_id, event_id)` and `(repository_id, job_id, event_id)`.
- `job_worktrees(repository_id, run_id, state)` and partial unique `(repository_id, job_id) WHERE state = 'active'`.
- `process_supervisors(repository_id, run_id, state)` and partial unique `(repository_id, session_id) WHERE state IN ('starting','attached','detached')`.
- `process_supervisor_pointers(repository_id, run_id, state)` and partial unique `(repository_id, session_id) WHERE state IN ('starting','attached','detached')`.

The same SQL file creates `striatumd.repo_migrations` with `repository_id`, `source_substrate`, `target_substrate`, `source_user_version`, `source_event_manifest_sha256`, `source_artifact_manifest_sha256`, `source_state_db_sha256`, `migrated_at`, `tombstone_path`, and `row_counts jsonb`. Implement append-only trigger functions for `events` and `artifacts`, and revoke `UPDATE` and `DELETE` on those tables from the daemon runtime role.

Implement the command body in `src/striatum/daemon_pg/repo_local_migration.py`, not in `cutover.py`, so daemon-registry cutover and repo-local workflow cutover stay separate. Public functions and types are:

- `RepoLocalMigrationOptions(repo: Path, postgres_url: str, dry_run: bool = False, keep_sqlite_readonly: bool = True, confirm_delete: bool = False)`.
- `migrate_repo_local(options: RepoLocalMigrationOptions) -> dict[str, Any]`.
- `compute_repo_local_reanchor(sqlite_conn, pg_conn, repository_id: str) -> dict[str, str]`.
- `_source_counts(sqlite_conn)`, `_verify_sqlite_current(sqlite_conn)`, `_ensure_registered(pg_conn, repo)`, `_copy_repo_rows(sqlite_conn, pg_conn, repository_id)`, `_write_repo_migration_checkpoint(...)`, and `_tombstone_or_delete_state_db(repo, keep_sqlite_readonly, confirm_delete)`.

Wire the CLI through a new `src/striatum/cli/daemon.py` helper with `dispatch_daemon(args: argparse.Namespace) -> object`, then have `src/striatum/cli/dispatch.py::_dispatch_daemon` delegate to it. Add a `daemon migrate-repo-local` subparser immediately after the existing `daemon migrate` parser in `src/striatum/cli/parser.py` with `--from sqlite`, `--to pg`, required `--repo`, `--postgres-url`, `--dry-run`, `--keep-sqlite-readonly` defaulting true through `argparse.BooleanOptionalAction`, `--confirm-delete`, and `--json`.

The migration algorithm is: authorize daemon admin, resolve or implicitly register the repository, refuse if a `repo_migrations` row already exists by returning `already_migrated: true`, open `.striatum/state.sqlite3` read-only, verify `PRAGMA user_version == striatum.migrations.LATEST_VERSION`, and for full runs copy every row in one `SERIALIZABLE` Postgres transaction. Dry-run writes nothing and returns source row counts plus event/artifact manifest hashes. Full-run writes the checkpoint, commits, then renames `.striatum/state.sqlite3` to `.striatum/state.sqlite3.tombstone` with mode `0444` by default. If `--no-keep-sqlite-readonly` is used, deletion requires `--confirm-delete`; otherwise refuse with exit code 8.

`compute_repo_local_reanchor` defines the byte-equivalence check: canonical JSON arrays of source rows ordered by stable primary key for `events` and `artifacts`, projected to source-column names and compact UTF-8 JSON. After copy, it queries the corresponding Postgres rows, projects them back to the same source-column shape, and requires SHA-256 equality. This is the correct RFC 0043 re-anchor because current repo-local `events` are append-only but not hash-chained.

Track A tests live in `tests/test_daemon_pg_repo_local_migration.py` and use `tests/fixtures/v1_repo_local_sqlite/state.sqlite3`. Required cases are dry-run writes nothing, full-run copies all rows under one `repository_id`, rerun returns the checkpoint, default tombstone is read-only, `--confirm-delete` removes SQLite, old or newer SQLite `user_version` is refused, and event/artifact manifests match byte-equivalently.

## Track B: CLI Surface And RPC Registry

Track B owns daemon-required dispatch, exit codes 11 and 12, parser retirement of `--no-daemon`, and RFC 0030 method registry coverage.

Remove the `--no-daemon` flag at `src/striatum/cli/parser.py:26` through `src/striatum/cli/parser.py:30`. Do not leave a hidden alias. The unknown-option path is argparse's standard behavior: exit code `2`, stderr shaped as `striatum: error: unrecognized arguments: --no-daemon`. Because parse fails before `args.json` exists, there is no JSON envelope for this retired flag.

Add `DaemonUnreachableError` and `RepoNotMigratedError` in `src/striatum/errors.py`. Add `src/striatum/cli/daemon_required.py::dispatch_via_daemon_required(args, repo: Path) -> object` and call it from `src/striatum/cli/dispatch.py::dispatch` for every stateful command except local `--help`, `--version`, daemon lifecycle/config commands, and local file-only workflow/template preview helpers that Track B explicitly marks read-only. `src/striatum/cli/dispatch.py:169` through `src/striatum/cli/dispatch.py:185` is replaced; `daemon_forced` becomes the normal route rather than an opt-in gate.

Exit code 11 is `daemon_unreachable`, implementing RFC 0043 §3. Stderr template:

```text
daemon_unreachable: could not connect to Striatum daemon at {socket_path}
Start the daemon and verify PostgreSQL configuration.
Linux systemd: systemctl --user start striatumd
macOS launchd: launchctl bootstrap gui/$UID ~/Library/LaunchAgents/io.striatum.striatumd.plist
Foreground: striatumd --foreground
Postgres: run `striatum daemon doctor --postgres-url <url>` or set STRIATUM_DAEMON_DB_URL.
```

The JSON error envelope for `--json` is:

```json
{"ok": false, "error": {"message": "daemon_unreachable: could not connect to Striatum daemon at <socket>", "code": 11, "hint": "start striatumd; run striatum daemon doctor --postgres-url <url>"}}
```

Exit code 12 is `repo_not_migrated`, also from RFC 0043 §3. It is raised after repository resolution but before calling repo-scoped RPC methods when `striatumd.repo_migrations` has no row for the repository and an old `.striatum/state.sqlite3` or tombstone indicates a pre-cutover repo. Stderr template:

```text
repo_not_migrated: {repo_path} has not been migrated to daemon PostgreSQL state
Run: striatum daemon migrate-repo-local --from sqlite --to pg --repo {repo_path}
```

The JSON envelope is:

```json
{"ok": false, "error": {"message": "repo_not_migrated: <repo> has not been migrated to daemon PostgreSQL state", "code": 12, "hint": "striatum daemon migrate-repo-local --from sqlite --to pg --repo <repo>"}}
```

Expand `src/striatum/daemon_rpc/registry.py::_ENTRIES` to remove legacy undotted names and cover the full RFC 0043 surface. Keep existing `apply.*`, `cross_repo.*`, `daemon.token.*`, `daemon.key.rotate`, `daemon.shutdown`, `dogfood.publish_on_behalf`, and `dogfood.surgical_recovery`. Use these names and capabilities for the repo-local CLI surface:

`repo.init/admin`, `session.register/claim`, `session.close/claim`, `work.claim_next/claim`, `work.ack/claim`, `work.heartbeat/claim`, `work.release/claim`, `work.send_message/write`, `work.block/write`, `work.complete/write`, `artifact.publish/write`, `review.submit/review`, `review.verdict/review`, `review.override/admin`, `decision.record/admin`, `checkpoint.resolve/admin`, `recovery.stale_leases/recovery`, `recovery.requeue_stale/recovery`, `recovery.cancel_job/recovery`, `recovery.process_reconcile/recovery`, `recovery.resume/recovery`, `recovery.auto/recovery`, `recovery.watch/recovery`, `worktree.create/write`, `worktree.release/write`, `worktree.list/read`, `branch.confirm/admin`, `run.prepare/admin`, `run.start/admin`, `run.pause/admin`, `run.resume/admin`, `run.cancel/admin`, `run.retry_job/admin`, `run.summary/read`, `run.graph/read`, `workflow.validate/read`, `workflow.plan/read`, `workflow.graph/read`, `workflow.templates.show/read`, `workflow.templates.list/read`, `workflow.init/write`, `workflow.generate/write`, `workflow.upgrade/write`, `status/read`, `why/read`, `doctor/read`, `dashboard/read`, `dashboard.all/read`, `list.runs/read`, `list.jobs/read`, `list.sessions/read`, `list.artifacts/read`, `list.workflows/read`, `evidence.export/read`, `corpus.export/read`, `supervise.start/claim`, `supervise.send/claim`, `supervise.stop/claim`, `supervise.status/read`, `supervise.list/read`, and `supervise.reattach_status/read`.

Daemon-global additions are `repo.add/admin`, `repo.remove/admin`, `repo.list/read`, `daemon.migrate/admin`, and `daemon.migrate_repo_local/admin`. Read-capability method names are locked as `status`, `why`, `doctor`, `dashboard`, `dashboard.all`, `run.summary`, and `run.graph`; do not introduce parallel names like `status.summary`.

Add SQL migration `src/striatum/daemon_pg/sql/0006_rpc_registry_rfc0043.sql` only if Track A's `0005` does not already widen `striatumd.rpc_methods.required_capability`; the CHECK from `0002_rpc_supervision_apply.sql` currently lacks `recovery` and `surgical_recovery`. Preferred ordering is to include the widened CHECK in `0005_repo_local_workflow_state.sql` so the final daemon DB version is 5 for this RFC.

Track B tests live in `tests/test_cli_daemon_required.py` and `tests/test_daemon_rpc_registry.py`. Required cases are `--no-daemon` is unknown, help/version do not require a daemon, `daemon doctor` runs without a daemon, missing daemon socket exits 11 with socket path and Linux/macOS/foreground/Postgres remediation, unmigrated repo exits 12 with `migrate-repo-local`, registry rejects legacy `ack`, `claim_next`, `publish_artifact`, `complete`, `verdict`, and `submit_review` unless they are explicitly deprecated aliases, registry capabilities match the list above, and Postgres accepts `recovery` plus `surgical_recovery` capabilities.

## Integration Order

Land the implementation in this order: schema migration and migration registration; repo-local migration module and fixture tests; RPC registry vocabulary and capability CHECK widening; daemon-required error classes and client dispatch; Postgres-backed repo-state handlers; parser removal of `--no-daemon`; MultiRepoHarness conversion to daemon RPC. The release boundary is atomic: no user-facing build should ship with the schema present but a partial mutation registry or any SQLite fallback.
