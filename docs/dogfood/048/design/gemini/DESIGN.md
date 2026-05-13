author: designer-unknown-model-001

# DESIGN: RFC 0043 V1 — Postgres as Sole Substrate and Daemon-Required Runtime

This design specifies the migration of Striatum's repo-local state from SQLite (`.striatum/state.sqlite3`) to the daemon-owned PostgreSQL instance. Following RFC 0043, every CLI operation will henceforth route through the daemon RPC envelope, making the daemon the single writer and authoritative state owner.

## Track A: Schema + migrate-repo-local (codex)

### 1. PostgreSQL Schema Migration
A new migration `src/striatum/daemon_pg/sql/0005_repo_local_state.sql` will define the repo-local tables in the `striatumd` schema. Every table gains a `repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id)` column and composite indexes for efficient repo-scoped access.

The 15 repo-local tables and their structural mapping (from `src/striatum/schema.py` and `src/striatum/migrations.py`):

1.  **runs**: (`repository_id`, `run_id`, `workflow_snapshot_id`, `repo_root`, `state`, `branch_name`, `branch_base`, `branch_confirmed_at`, `branch_confirmed_by`, `created_at`, `started_at`, `completed_at`, `stop_reason`, `paused_at`, `paused_reason`, `cross_repo_run_id`).
2.  **sessions**: (`repository_id`, `session_id`, `run_id`, `role_id`, `lane_id`, `slug`, `ordinal`, `capabilities_json`, `parent_session_id`, `first_class`, `fresh_context`, `state`, `registered_at`, `last_heartbeat_at`, `expires_at`, `non_fresh_reason`, `closed_at`, `close_reason`, `operator_label`).
3.  **jobs**: (`repository_id`, `job_id`, `run_id`, `workflow_job_id`, `title`, `job_type`, `role_id`, `lane_selector_json`, `capability_requirements_json`, `state`, `attempt`, `max_attempts`, `fresh_session_required`, `write_scope_json`, `expected_artifacts_json`, `idempotency_key`, `created_at`, `ready_at`, `started_at`, `completed_at`, `current_message_id`, `current_lease_id`).
4.  **queue_messages**: (`repository_id`, `message_id`, `run_id`, `job_id`, `kind`, `state`, `priority`, `target_session_id`, `target_role_id`, `target_lane_id`, `dedupe_key`, `payload_json`, `visible_after`, `claim_count`, `max_claims`, `created_at`, `updated_at`, `claimed_at`, `acked_at`, `completed_at`, `current_lease_id`).
5.  **leases**: (`repository_id`, `lease_id`, `run_id`, `resource_type`, `resource_id`, `owner_session_id`, `state`, `acquired_at`, `expires_at`, `last_heartbeat_at`, `released_at`, `release_reason`).
6.  **work_packets**: (`repository_id`, `packet_id`, `run_id`, `job_id`, `message_id`, `lease_id`, `session_id`, `packet_json`, `packet_sha256`, `created_at`).
7.  **artifacts**: (`repository_id`, `artifact_id`, `run_id`, `job_id`, `session_id`, `logical_name`, `artifact_kind`, `repo_path`, `content_sha256`, `size_bytes`, `publish_mode`, `created_at`, `author_line`).
8.  **verdicts**: (`repository_id`, `verdict_id`, `run_id`, `job_id`, `session_id`, `verdict`, `rationale`, `findings_artifact_id`, `created_at`, `posture`).
9.  **blockers**: (`repository_id`, `blocker_id`, `run_id`, `job_id`, `session_id`, `severity`, `blocker_kind`, `description`, `state`, `created_at`, `resolved_at`, `payload_json`).
10. **command_requests**: (`repository_id`, `request_id`, `run_id`, `session_id`, `command_name`, `payload_sha256`, `response_json`, `state`, `created_at`, `completed_at`).
11. **process_executions**: (`repository_id`, `process_id`, `run_id`, `job_id`, `session_id`, `lease_id`, `packet_id`, `adapter`, `command_json`, `cwd`, `scratch_path`, `stdin_mode`, `stdio_mode`, `pid`, `state`, `exit_code`, `started_at`, `ended_at`).
12. **events**: (`repository_id`, `event_id`, `run_id`, `event_type`, `actor_session_id`, `job_id`, `message_id`, `artifact_id`, `lease_id`, `payload_json`, `created_at`).
13. **job_worktrees**: (`repository_id`, `worktree_id`, `run_id`, `job_id`, `lease_id`, `base_branch`, `worktree_path`, `state`, `created_at`, `released_at`, `removed_at`).
14. **process_supervisors**: (`repository_id`, `supervisor_id`, `run_id`, `session_id`, `adapter`, `command_json`, `cwd`, `scratch_path`, `stdin_pipe_path`, `pid`, `state`, `started_at`, `heartbeat_at`, `ended_at`, `stop_reason`, `pid_start_time`).
15. **process_supervisor_pointers**: (`repository_id`, `supervisor_id`, `daemon_supervisor_id`, `run_id`, `session_id`, `pid`, `pid_start_time`, `state`, `updated_at`, `metadata_json`).

**Required Structural Note:** `workflow_snapshots` and `job_dependencies` (from `src/striatum/schema.py`) must also be migrated with `repository_id` columns to maintain the integrity of `runs` and `jobs`.

**RBAC Enforcement:** Reuse RFC 0033 roles via `GRANT` in the SQL script:
- `striatumd_rw`: Full CRUD on all tables.
- `striatumd_append`: `INSERT` only on `events` and `artifacts` (enforced by revoking `UPDATE`/`DELETE`).
- `striatumd_migration`: Owner for DDL.

### 2. Migration CLI Verb
A new CLI module `src/striatum/cli/daemon.py` (wired in `dispatch.py`) implements:
`striatum daemon migrate-repo-local --from sqlite --to pg --repo <path> [--dry-run] [--keep-sqlite-readonly] [--confirm-delete]`

Implementation in `src/striatum/daemon_pg/cutover.py`:
- **Auth:** Requires daemon admin token (capability `admin`).
- **Isolation:** `SERIALIZABLE` single transaction.
- **Rollup Hash:** Computes `source_audit_chain_final_hash` as `sha256(canonical_csv_of_events_rows)`.
- **Audit Verification:** Re-anchors by replaying the `events` log and comparing the final rollup hash between substrates.
- **Idempotency:** Records success in a new `striatumd.repo_migrations` table: `(repository_id, migrated_at, source_sqlite_sha256, audit_chain_final_hash)`.
- **Tombstone:** Rename `state.sqlite3` → `state.sqlite3.tombstone` (mode 0444).

### 3. Test Fixture
- **Path:** `tests/fixtures/v1_repo_local_sqlite/state.sqlite3`.
- **Content:** A V14 schema snapshot (highest current version) populated with a sample run (1 session, 2 jobs, 1 artifact, 5 events).
- **Tests:** `tests/daemon_pg/test_migrate_repo_local.py` covering preflight, dry-run, full-run, idempotency, and tombstone handling.

---

## Track B: CLI surface + RPC registry (claude)

### 1. Parser Retirement
- Remove `--no-daemon` from `build_parser()` in `src/striatum/cli/parser.py`.
- Ensure `--daemon` is kept for backward compatibility but is effectively a no-op as it becomes the only mode.

### 2. Exit Code Wiring
In `src/striatum/cli/dispatch.py`:
- **Exit 11 (`daemon_unreachable`):** Raised when `striatum.daemon_rpc.client` fails to connect to the Unix socket. Stderr provides remediation:
  - Linux: `systemctl --user start striatumd`
  - macOS: `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/io.striatum.striatumd.plist`
  - General: `striatumd --foreground`
- **Exit 12 (`repo_not_migrated`):** Raised in `dispatch()` when the target repo contains `.striatum/state.sqlite3` but no corresponding entry in `striatumd.repo_migrations`. Stderr remediation: `striatum daemon migrate-repo-local`.

### 3. RPC Registry Expansion
Expand `_ENTRIES` in `src/striatum/daemon_rpc/registry.py` to cover all mutations currently in `src/striatum/cli/mutations.py` following RFC 0043 §5:

| Method | Required Capability | Repository Scope |
|---|---|---|
| `run.prepare` | `admin` | True |
| `run.start` | `admin` | True |
| `run.pause` | `admin` | True |
| `run.resume` | `admin` | True |
| `run.cancel` | `admin` | True |
| `run.retry_job` | `admin` | True |
| `branch.confirm` | `admin` | True |
| `session.register` | `claim` | True |
| `session.close` | `write` | True |
| `work.claim_next` | `claim` | True |
| `work.ack` | `claim` | True |
| `work.heartbeat` | `claim` | True |
| `work.complete` | `write` | True |
| `work.block` | `write` | True |
| `work.release` | `claim` | True |
| `artifact.publish` | `write` | True |
| `review.submit` | `review` | True |
| `review.verdict` | `review` | True |
| `decision.record` | `admin` | True |
| `checkpoint.resolve` | `admin` | True |
| `recovery.stale_leases` | `recovery` | True |
| `recovery.requeue_stale` | `recovery` | True |
| `recovery.cancel_job` | `recovery` | True |
| `recovery.resume` | `recovery` | True |
| `worktree.create` | `write` | True |
| `worktree.release` | `write` | True |
| `message.send` | `write` | True |

### 4. CLI Dispatch Logic
- `src/striatum/cli/dispatch.py`'s `dispatch()` function is refactored to check `daemon_forced` by default.
- Every mutation entry point (e.g., `run_start`, `publish_artifact`) is updated to route through `striatum.daemon_rpc.client` when the repo is migrated.
- `striatum init` updated to register with the daemon via `repo.add` RPC instead of creating a local SQLite.

---

## Technical Citations

- **Schema:** `src/striatum/schema.py` (source of table definitions), `src/striatum/daemon_pg/sql/0001_baseline.sql` (role/schema pattern).
- **Audit:** `src/striatum/daemon_pg/audit.py` (v2_row_hash pattern), `src/striatum/daemon.py:1041` (`_audit_chain_records`).
- **CLI:** `src/striatum/cli/parser.py:20` (`build_parser` flag removal), `src/striatum/cli/dispatch.py:88` (`dispatch` mode enforcement).
- **RPC:** `src/striatum/daemon_rpc/registry.py:46` (`_ENTRIES` tuple).
- **Migration Logic:** `src/striatum/daemon_pg/cutover.py:50` (entry point for registry migration; template for `migrate-repo-local`).
