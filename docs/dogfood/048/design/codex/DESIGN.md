# RFC 0043 V1 Design Handoff

author: designer-unknown-model-001
date: 2026-05-13
status: design

## Scope

This handoff designs RFC 0043 V1 across the requested two tracks. Track A moves repo-local workflow state from `.striatum/state.sqlite3` into daemon-owned PostgreSQL under `repository_id`; Track B makes the daemon the required CLI runtime and expands the RFC 0030 method registry so there is no unsupported mutation tail.

D094 is the controlling decision: it supersedes D006, D007, D036, and the SQLite half of D009. The preserved invariant is still "agents call Striatum commands"; the retired invariant is "the CLI process writes repo-local SQLite directly."

## Track A: Schema And Migration

### New Postgres Migration

Add `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql` and register it in `src/striatum/daemon_pg/migrations.py` by bumping `LATEST_DAEMON_DB_VERSION` to `5` and appending `PgMigration(5, "repo-local workflow state", "0005_repo_local_workflow_state.sql")` to `MIGRATIONS`.

Use the existing `striatumd` schema rather than per-repo schemas. RFC 0043's recommended shape is one schema with `repository_id` columns because RFC 0032 cross-repo queries already need joins across participants. Keep `repositories.repository_id` as the foreign key target even though `0001_baseline.sql` currently declares it as `text`; do not introduce an incompatible `uuid` type conversion in the same migration. If the implementation wants UUID typing later, that belongs in a separate migration that converts every existing daemon table consistently.

Create these repo-local tables in Postgres with `repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id)`:

`workflow_snapshots`, `runs`, `sessions`, `jobs`, `job_dependencies`, `queue_messages`, `leases`, `work_packets`, `artifacts`, `verdicts`, `blockers`, `command_requests`, `process_executions`, `events`, `job_worktrees`, `process_supervisors`, and `process_supervisor_pointers`.

The objective listed 15 tables, but the current SQLite source also has `workflow_snapshots` and `job_dependencies` in `src/striatum/schema.py`; both are required for `run.prepare`, `run.start`, `claim_next`, `run summary`, and cross-repo preparation to preserve behavior. Treat omitting them as a bug in the task shorthand, not a product change.

Mirror the current SQLite row shapes from `src/striatum/schema.py` plus migrations from `src/striatum/migrations.py` through version 14:

- `sessions` must include `non_fresh_reason`, `closed_at`, `close_reason`, and `operator_label`.
- `artifacts` must include `author_line` and must not enforce a SQL `artifact_kind` closed set.
- `verdicts` must include `posture` and `idx_verdicts_posture`.
- `blockers` must include `payload_json`.
- `runs` must include `paused_at`, `paused_reason`, and `cross_repo_run_id`.
- `process_supervisors` must include `pid_start_time`.
- `process_supervisor_pointers` must match migration v13.

Use `jsonb` for all current `*_json` columns and `timestamptz` for current RFC3339 timestamp text where the value is semantically a time. Preserve id strings as `text` because the code uses prefixed ids (`run_...`, `sess_...`, `job_...`) rather than raw UUIDs.

### Indexes And Constraints

Every current index in `src/striatum/schema.py` and `src/striatum/migrations.py` needs the repository prefix where the query is repo-scoped:

- `runs(repository_id, state)` and partial `runs(repository_id, cross_repo_run_id) WHERE cross_repo_run_id IS NOT NULL`.
- `sessions(repository_id, run_id, state)`, unique `(repository_id, run_id, slug)`, and unique `(repository_id, run_id, role_id, lane_id, ordinal)`.
- `jobs(repository_id, run_id, state, role_id)`, unique `(repository_id, run_id, workflow_job_id, attempt)`, and unique `(repository_id, run_id, idempotency_key)`.
- `queue_messages(repository_id, run_id, state, target_session_id, target_role_id, target_lane_id, visible_after, priority, created_at)` and partial unique `(repository_id, job_id) WHERE kind = 'work' AND state IN (...)`.
- `leases` partial unique `(repository_id, resource_type, resource_id) WHERE state = 'active'`.
- `work_packets(repository_id, run_id, session_id)` and unique `(repository_id, message_id, lease_id)`.
- `artifacts` unique `(repository_id, run_id, job_id, logical_name)` and `(repository_id, run_id, repo_path, content_sha256)`.
- `events(repository_id, run_id, event_id)` and `(repository_id, job_id, event_id)`.
- `job_worktrees` partial unique `(repository_id, job_id) WHERE state = 'active'` and `(repository_id, run_id, state)`.
- `process_supervisors` partial unique `(repository_id, session_id) WHERE state IN ('starting','attached','detached')` and `(repository_id, run_id, state)`.
- `process_supervisor_pointers` partial unique `(repository_id, session_id) WHERE state IN ('starting','attached','detached')` and `(repository_id, run_id, state)`.

Keep append-only enforcement on `events` and `artifacts`. The current SQLite triggers are `events_no_update`, `events_no_delete`, `artifacts_no_update`, and `artifacts_no_delete`; in Postgres implement schema-local trigger functions like the existing `striatumd.refuse_audit_change()` in `0001_baseline.sql`. Also revoke `UPDATE` and `DELETE` from the daemon runtime role for `events` and `artifacts` if the test harness has role-level coverage available; if not, at minimum land trigger enforcement with a TODO for role grants.

Add `striatumd.repo_migrations`:

```sql
CREATE TABLE IF NOT EXISTS striatumd.repo_migrations (
  repository_id text PRIMARY KEY REFERENCES striatumd.repositories(repository_id),
  source_substrate text NOT NULL CHECK (source_substrate = 'sqlite'),
  target_substrate text NOT NULL CHECK (target_substrate = 'pg'),
  source_user_version integer NOT NULL,
  source_event_head_sha256 text,
  source_artifact_manifest_sha256 text,
  migrated_at timestamptz NOT NULL DEFAULT now(),
  tombstone_path text,
  row_counts jsonb NOT NULL
);
```

This is the idempotency checkpoint and the `repo_not_migrated` gate for normal CLI verbs.

### Migration Command

Add parser support in `src/striatum/cli/parser.py` under `daemon`:

```text
striatum daemon migrate-repo-local --from sqlite --to pg --repo <path> [--dry-run] [--keep-sqlite-readonly] [--confirm-delete] [--postgres-url]
```

The top-level parser already has a global `--repo`; RFC 0043 explicitly wants the migration verb's `--repo <path>`, so the daemon subcommand should define its own required `--repo` and dispatch must use that value for this verb. `--keep-sqlite-readonly` should default to true in behavior. The simplest argparse shape is `--keep-sqlite-readonly` with default true plus `--delete-sqlite` as a clearer inverse, but the RFC names `--confirm-delete`; if we keep only RFC flags, interpret "no confirm-delete" as keep-readonly and "confirm-delete" as delete after successful migration.

Implement the command in a new `src/striatum/daemon_pg/repo_local_migration.py`, not in `cutover.py`. Reuse `cutover.py` patterns for source counts, destination hash checks, dry-run envelopes, and marker writes, but keep registry cutover and repo-local migration separate.

Entry points:

- `RepoLocalMigrationOptions(repo: Path, postgres_url: str, dry_run: bool, keep_sqlite_readonly: bool, confirm_delete: bool)`.
- `migrate_repo_local(options: RepoLocalMigrationOptions) -> dict[str, Any]`.
- `_source_counts(sqlite_conn)`.
- `_verify_sqlite_current(sqlite_conn)` using `striatum.migrations.LATEST_VERSION` and `PRAGMA user_version`.
- `_ensure_registered(pg_conn, repo)` using the same repo identity helper currently used by `tests/_harness/repos.py::register_repo` (`striatum.daemon._repo_identity`).
- `_copy_repo_rows(source, pg_conn, repository_id)` with one inserter per table.
- `_write_repo_migration_checkpoint(...)`.
- `_tombstone_or_delete_state_db(repo, keep_sqlite_readonly, confirm_delete)`.

The migration must run as one Postgres transaction at `SERIALIZABLE` isolation. In psycopg this should be set before the transaction block, then all inserts and checkpoint writes happen before one commit. Do not commit inside row-copy helpers. The current `daemon_pg/cutover.py::_import_all` commits internally; do not reuse that function as-is.

Dry-run must open SQLite read-only, verify schema, verify destination reachability, enumerate counts for every source table, report the current `events` head, report artifact count/hash manifest, and write nothing to Postgres or disk.

Full-run algorithm:

1. Authorize admin before opening the SQLite source. The current CLI has local daemon-admin flows in `src/striatum/daemon.py` and token helpers in `tests/_harness/tokens.py`; reuse the daemon admin capability path rather than adding a migration-only bypass.
2. Resolve or create the `repositories` row. If the repo is not registered, implicitly register with init false and record that in the response.
3. Refuse if `repo_migrations` already has this `repository_id`; return `already_migrated: true` with the checkpoint row and exit 0.
4. Refuse if `.striatum/state.sqlite3` is missing and no checkpoint exists; normal CLI verbs will later return exit code 12.
5. Verify `PRAGMA user_version == striatum.migrations.LATEST_VERSION`.
6. Start one `SERIALIZABLE` Postgres transaction and copy rows in dependency order: `workflow_snapshots`, `runs`, `sessions`, `jobs`, `job_dependencies`, `queue_messages`, `leases`, `work_packets`, `artifacts`, `verdicts`, `blockers`, `command_requests`, `process_executions`, `events`, `job_worktrees`, `process_supervisors`, `process_supervisor_pointers`.
7. Insert preserving source ids, timestamps, event ordering, and autoincrement `event_id` via `OVERRIDING SYSTEM VALUE` if `events.event_id` uses identity.
8. Recompute the source event/artifact anchor using canonical row serialization. The current repo-local SQLite event table is not hash-chained, so define the re-anchor as byte-equivalent table manifests: canonical JSON of ordered event rows and canonical JSON of ordered artifact rows. Do not invent transcript or file-content capture.
9. Insert `repo_migrations` with row counts and hashes, commit, then tombstone or delete SQLite.

Because RFC 0043 says "audit-chain byte-equivalence re-anchor" but current `events` has no row hash fields, the implementation must document the exact re-anchor format in code comments and tests. If reviewers want a true event hash chain, add fields in the new Postgres schema and backfill from canonical source rows during migration, while preserving the original event ids and row bytes in the manifest.

Tombstone behavior:

- Default: rename `.striatum/state.sqlite3` to `.striatum/state.sqlite3.tombstone` and `chmod 0444`.
- Also handle `-wal` and `-shm`: checkpoint SQLite before rename, then remove empty sidecars or tombstone them consistently. Tests should assert no live `state.sqlite3` remains.
- Delete path: require `--confirm-delete`; otherwise refuse destructive cleanup with exit code 8.

### Track A Tests

Add `tests/fixtures/v1_repo_local_sqlite/state.sqlite3` generated from the current highest SQLite schema. Keep the fixture small but non-empty: one workflow snapshot, one run, one session, one queued/acked work message, one lease, one packet, one artifact, one verdict, one blocker, one command request, one process execution, one job worktree, one supervisor, one supervisor pointer, and multiple events.

Add `tests/test_daemon_pg_repo_local_migration.py`:

- `test_repo_local_migration_dry_run_reports_counts_and_writes_nothing`.
- `test_repo_local_migration_full_run_copies_rows_under_repository_id`.
- `test_repo_local_migration_idempotent_rerun_uses_repo_migrations_checkpoint`.
- `test_repo_local_migration_tombstones_sqlite_readonly_by_default`.
- `test_repo_local_migration_confirm_delete_removes_sqlite`.
- `test_repo_local_migration_refuses_old_or_new_sqlite_user_version`.
- `test_repo_local_migration_preserves_event_and_artifact_manifests`.

Extend `tests/_harness/MultiRepoHarness` and `tests/_harness/repos.py`. Today `RepoLocalRunner.prepare/start/cancel/participant_intact/human_checkpoint` use `sqlite3.connect(db_path(...))`; under RFC 0043 they must use `self.pg_conn()` and repo-local Postgres tables by `repository_id`. Keep a compatibility helper only for the migration fixture tests.

## Track B: CLI And RPC Registry

### Parser And Dispatch

Remove the top-level `--no-daemon` argument from `src/striatum/cli/parser.py`. Do not leave a hidden deprecated flag; argparse should return the standard unknown-option parse error.

Keep `--daemon` only if it still has a useful meaning after RFC 0043. The cleaner V1 implementation is to ignore/remove it for workflow verbs because daemon routing is mandatory. If retained for compatibility on read commands, it must not imply there is a fallback.

Refactor `src/striatum/cli/dispatch.py` so it no longer imports `sqlite3`, `connect`, `transaction`, `ensure_initialized`, `db_path`, or direct mutation helpers for normal command execution. The main dispatch shape should be:

- `--help` and `--version`: local only.
- `daemon doctor`: local configuration/Postgres checks only; allowed when daemon is down.
- `daemon start/status/stop/health/audit/sweep/migrate/migrate-repo-local`: daemon/admin command handling.
- Everything else: call a daemon RPC client.

Add errors in `src/striatum/errors.py` or a new CLI error helper:

- `DaemonUnreachableError(StriatumError)` exit code 11.
- `RepoNotMigratedError(StriatumError)` exit code 12.

`DaemonUnreachableError` should include the attempted socket path from `src/striatum/daemon_rpc/transport_unix.py` or the config resolver, plus remediation:

- Linux systemd: `systemctl --user start striatumd`.
- macOS launchd: `launchctl bootstrap gui/$UID ~/Library/LaunchAgents/...`.
- Foreground fallback: `striatumd --foreground` or current `striatum daemon start`.
- Postgres hint: run `striatum daemon doctor --postgres-url ...` or set `STRIATUM_DAEMON_DB_URL`.

`RepoNotMigratedError` should name the repo path and `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>`.

The daemon client should perform the repo-migration check before invoking repo-scoped methods: resolve repo to `repository_id`, check `striatumd.repo_migrations`, and return exit code 12 if absent and `.striatum/state.sqlite3` or `.striatum/state.sqlite3.tombstone` indicates a pre-cutover repo. Fresh `init` should register the repo and create the Postgres rows without SQLite.

### RPC Method Registry

Update `src/striatum/daemon_rpc/registry.py` to use RFC 0043 names and capabilities. Current entries include legacy names like `ack`, `publish_artifact`, `claim_next`, `complete`, and mark `workflow.validate` as `write`; those should be corrected.

Required registry entries:

| CLI surface | RPC method | Capability | Scope |
|---|---|---|---|
| `init` | `repo.init` | `admin` | single_repo |
| `register-session` | `session.register` | `claim` | single_repo |
| `session close` | `session.close` | `claim` | single_repo |
| `claim-next` | `work.claim_next` | `claim` | single_repo |
| `ack` | `work.ack` | `claim` | single_repo |
| `heartbeat` | `work.heartbeat` | `claim` | single_repo |
| `release` | `work.release` | `claim` | single_repo |
| `send` | `work.send_message` | `write` | single_repo |
| `block` | `work.block` | `write` | single_repo |
| `complete` | `work.complete` | `write` | single_repo |
| `publish-artifact` | `artifact.publish` | `write` | single_repo |
| `submit-review` | `review.submit` | `review` | single_repo |
| `verdict` | `review.verdict` | `review` | single_repo |
| `override-verdict` | `review.override` | `admin` | single_repo |
| `decision record` | `decision.record` | `admin` | single_repo |
| `checkpoint resolve` | `checkpoint.resolve` | `admin` | single_repo |
| `recovery stale-leases` | `recovery.stale_leases` | `recovery` | single_repo |
| `recovery requeue-stale` | `recovery.requeue_stale` | `recovery` | single_repo |
| `recovery cancel-job` | `recovery.cancel_job` | `recovery` | single_repo |
| `recovery process-reconcile` | `recovery.process_reconcile` | `recovery` | single_repo |
| `recovery resume` | `recovery.resume` | `recovery` | single_repo |
| `recovery auto/watch` | `recovery.auto` / `recovery.watch` | `recovery` | single_repo |
| `worktree create/release/list` | `worktree.create` / `worktree.release` / `worktree.list` | `write` / `write` / `read` | single_repo |
| `branch confirm` | `branch.confirm` | `admin` | single_repo |
| `run prepare/start/pause/resume/cancel/retry-job` | `run.prepare` / `run.start` / `run.pause` / `run.resume` / `run.cancel` / `run.retry_job` | `admin` | single_repo |
| `run summary/graph` | `run.summary` / `run.graph` | `read` | single_repo |
| `workflow validate/plan/graph/templates/show/templates/list` | `workflow.validate` / `workflow.plan` / `workflow.graph` / `workflow.templates.show` / `workflow.templates.list` | `read` | single_repo |
| `workflow init/generate/upgrade` | `workflow.init` / `workflow.generate` / `workflow.upgrade` | `write` | single_repo |
| `status/why/doctor/dashboard/list/evidence export/corpus export` | `status` / `why` / `doctor` / `dashboard` / `list.*` / `evidence.export` / `corpus.export` | `read` | single_repo |
| `supervise start/send/stop` | `supervise.start` / `supervise.send` / `supervise.stop` | `claim` | single_repo |
| `supervise status/list/reattach_status` | same current names | `read` | single_repo |
| `repo add/remove/list` | `repo.add` / `repo.remove` / `repo.list` | `admin` / `admin` / `read` | daemon_global |
| `daemon migrate` | `daemon.migrate` | `admin` | daemon_global |
| `daemon migrate-repo-local` | `daemon.migrate_repo_local` | `admin` | single_repo |

Keep existing `apply.*`, `cross_repo.*`, `daemon.token.*`, `daemon.key.rotate`, `daemon.shutdown`, `dogfood.publish_on_behalf`, and `dogfood.surgical_recovery`, but verify capabilities include the current expanded set. `src/striatum/daemon_pg/sql/0002_rpc_supervision_apply.sql` still constrains `rpc_methods.required_capability` to six values and excludes `recovery` and `surgical_recovery`; add a SQL migration to widen that CHECK or rebuild the table before syncing the registry.

### RPC Handlers

Create a handler registry beside `src/striatum/daemon_rpc/server.py` that maps each method to an implementation function. The least risky transition is to move the existing direct functions behind repository-scoped Postgres adapters rather than rewriting every state machine at once.

Recommended modules:

- `src/striatum/daemon_rpc/repo_context.py`: resolve repo path, token scope, and `repository_id`; enforce `repo_migrations`.
- `src/striatum/daemon_repo_state.py`: Postgres equivalent of the current `src/striatum/db.py` operations (`claim_next`, `complete_job`, `insert_event`, `row_by_id`, `maybe_complete_run`, run pause/resume/cancel/retry).
- `src/striatum/daemon_rpc/handlers.py`: thin RPC param validation and calls into `daemon_repo_state`, `artifacts`, `workflow`, `supervisor`, and `recovery` modules.

Do not make the daemon handler shell out to the CLI. That would preserve the direct SQLite dependency and break the "daemon is the single writer" boundary.

### Track B Tests

Add parser tests:

- `tests/test_cli_daemon_required.py::test_no_daemon_is_unknown_option`.
- `test_help_and_version_do_not_require_daemon`.
- `test_daemon_doctor_runs_without_daemon`.

Add dispatch/client refusal tests:

- daemon socket missing returns exit code 11 and stderr includes attempted socket path plus Linux/macOS/foreground remediation.
- unmigrated repo returns exit code 12 and names `daemon migrate-repo-local`.

Add registry tests:

- `tests/test_daemon_rpc_registry.py::test_registry_covers_cli_mutations` introspects `src/striatum/cli/parser.py` command surfaces and verifies every mutation listed above has a method.
- `test_registry_uses_rfc0043_method_names` rejects legacy `ack`, `publish_artifact`, `claim_next`, `complete`, `verdict`, and `submit_review` names unless aliases are explicitly marked deprecated.
- `test_registry_capabilities_match_rfc0043`.
- `test_pg_rpc_methods_capability_check_accepts_recovery_and_surgical_recovery`.

Extend `tests/_harness/MultiRepoHarness` so end-to-end flows use daemon RPC for `run.prepare`, `run.start`, `session.register`, `work.claim_next`, `work.ack`, `artifact.publish`, `review.verdict`, and `work.complete`. This is the acceptance gate that proves `src/striatum/db.py` is no longer the product write path.

## Rollout Order

1. Land Postgres migration 0005 plus migration registration and tests for applying it.
2. Land `repo_local_migration.py` and fixture tests while direct SQLite still exists.
3. Add `repo_migrations` gate and exit code 12 behind daemon-routed reads.
4. Expand `daemon_rpc/registry.py` and SQL `rpc_methods` capability constraints.
5. Build daemon RPC handlers backed by Postgres repo-local tables.
6. Refactor `dispatch.py` to daemon-required routing.
7. Remove parser `--no-daemon` and retire direct SQLite tests.
8. Update harness to run normal workflows on Postgres only.

This order keeps the migration testable before the direct path is removed, but the shipped RFC 0043 behavior should appear atomically in one release: no SQLite fallback, no `--no-daemon`, and no partially daemonized mutation surface.

## Out Of Scope

Do not bundle Postgres, add tenant enforcement, add hosted-mode auth, rewrite the Go daemon, or rewrite historical dogfood scaffolds. Do not update README, TODO, CHANGELOG, SPEC, HOW_TO, or UBIQUITOUS_LANGUAGE in this agent artifact; the prompt reserves those doc updates for the operator after the dogfood lands.
