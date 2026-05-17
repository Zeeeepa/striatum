"""Repo-local SQLite workflow-state migration to daemon PostgreSQL."""

from __future__ import annotations

import errno
import fcntl
import hashlib
import json
import os
import sqlite3
import uuid
from contextlib import contextmanager
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any, Iterator

from striatum import daemon as daemon_registry
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import canonical_event_hash
from striatum.daemon_pg.migrations import apply_migrations
from striatum.errors import SchemaVersionError, StriatumError
from striatum.migrations import LATEST_VERSION, current_user_version
from striatum.primitives import json_dumps, utc_now
from striatum.repo_policy import db_path


class MigrationInProgressError(StriatumError):
    """Raised when migrate-repo-local cannot acquire the exclusive lock.

    Uses exit code 8 (the V1.5 ``migrate-repo-local`` refusal code) per the
    dogfood-052 V1.6 design synthesis ("avoid introducing a new exit code
    for this narrow V1.6 slice").
    """

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=8)


@contextmanager
def _exclusive_migrate_lock(repo: Path) -> Iterator[Path]:
    """RFC 0043 V1.6 F-lock: hold an exclusive ``fcntl.flock`` for the
    entire ``migrate-repo-local`` body. Concurrent migrate invocations
    against the same repo refuse with exit code 8.

    The lock is taken on a sidecar (``state.sqlite3.migrate.lock``) rather
    than the source SQLite itself so it (a) does not fight SQLite's own
    POSIX byte-range locks, and (b) survives the source-file rename during
    finalization. The error message names the source SQLite path so the
    operator can correlate the refusal to the repo's state file.
    """
    source_path = repo / ".striatum" / "state.sqlite3"
    lock_path = repo / ".striatum" / "state.sqlite3.migrate.lock"
    lock_path.parent.mkdir(parents=True, exist_ok=True)
    fd = os.open(str(lock_path), os.O_RDWR | os.O_CREAT, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError as exc:
            if exc.errno in (errno.EWOULDBLOCK, errno.EACCES, errno.EAGAIN):
                raise MigrationInProgressError(
                    f"migrate_in_progress: another striatum daemon "
                    f"migrate-repo-local is holding the lock for source "
                    f"SQLite {source_path} (lock file: {lock_path}); wait "
                    f"for it to finish or remove the stale lock file."
                ) from exc
            raise
        yield lock_path
    finally:
        try:
            fcntl.flock(fd, fcntl.LOCK_UN)
        finally:
            os.close(fd)


@dataclass(frozen=True)
class RepoLocalMigrationOptions:
    repo: Path
    postgres_url: str
    dry_run: bool = False
    keep_sqlite_readonly: bool = True
    confirm_delete: bool = False


@dataclass(frozen=True)
class TableSpec:
    name: str
    columns: tuple[str, ...]
    json_columns: frozenset[str] = frozenset()
    bool_columns: frozenset[str] = frozenset()
    order_by: str | None = None


TABLE_SPECS: tuple[TableSpec, ...] = (
    TableSpec(
        "workflow_snapshots",
        (
            "workflow_snapshot_id",
            "workflow_id",
            "workflow_version",
            "source_path",
            "content_sha256",
            "workflow_json",
            "loaded_at",
        ),
        json_columns=frozenset({"workflow_json"}),
    ),
    TableSpec(
        "runs",
        (
            "run_id",
            "workflow_snapshot_id",
            "repo_root",
            "state",
            "branch_name",
            "branch_base",
            "branch_confirmed_at",
            "branch_confirmed_by",
            "created_at",
            "started_at",
            "completed_at",
            "stop_reason",
            "paused_at",
            "paused_reason",
            "cross_repo_run_id",
        ),
    ),
    TableSpec(
        "sessions",
        (
            "session_id",
            "run_id",
            "role_id",
            "lane_id",
            "slug",
            "ordinal",
            "capabilities_json",
            "parent_session_id",
            "first_class",
            "fresh_context",
            "state",
            "registered_at",
            "last_heartbeat_at",
            "expires_at",
            "non_fresh_reason",
            "closed_at",
            "close_reason",
            "operator_label",
        ),
        json_columns=frozenset({"capabilities_json"}),
        bool_columns=frozenset({"first_class", "fresh_context"}),
    ),
    TableSpec(
        "jobs",
        (
            "job_id",
            "run_id",
            "workflow_job_id",
            "title",
            "job_type",
            "role_id",
            "lane_selector_json",
            "capability_requirements_json",
            "state",
            "attempt",
            "max_attempts",
            "fresh_session_required",
            "write_scope_json",
            "expected_artifacts_json",
            "idempotency_key",
            "created_at",
            "ready_at",
            "started_at",
            "completed_at",
            "current_message_id",
            "current_lease_id",
        ),
        json_columns=frozenset(
            {
                "lane_selector_json",
                "capability_requirements_json",
                "write_scope_json",
                "expected_artifacts_json",
            }
        ),
        bool_columns=frozenset({"fresh_session_required"}),
    ),
    TableSpec("job_dependencies", ("job_id", "depends_on_job_id", "gate_json"), json_columns=frozenset({"gate_json"})),
    TableSpec(
        "queue_messages",
        (
            "message_id",
            "run_id",
            "job_id",
            "kind",
            "state",
            "priority",
            "target_session_id",
            "target_role_id",
            "target_lane_id",
            "dedupe_key",
            "payload_json",
            "visible_after",
            "claim_count",
            "max_claims",
            "created_at",
            "updated_at",
            "claimed_at",
            "acked_at",
            "completed_at",
            "current_lease_id",
        ),
        json_columns=frozenset({"payload_json"}),
    ),
    TableSpec(
        "leases",
        (
            "lease_id",
            "run_id",
            "resource_type",
            "resource_id",
            "owner_session_id",
            "state",
            "acquired_at",
            "expires_at",
            "last_heartbeat_at",
            "released_at",
            "release_reason",
        ),
    ),
    TableSpec(
        "work_packets",
        (
            "packet_id",
            "run_id",
            "job_id",
            "message_id",
            "lease_id",
            "session_id",
            "packet_json",
            "packet_sha256",
            "created_at",
        ),
        json_columns=frozenset({"packet_json"}),
    ),
    TableSpec(
        "artifacts",
        (
            "artifact_id",
            "run_id",
            "job_id",
            "session_id",
            "logical_name",
            "artifact_kind",
            "repo_path",
            "content_sha256",
            "size_bytes",
            "publish_mode",
            "created_at",
            "author_line",
        ),
    ),
    TableSpec(
        "verdicts",
        (
            "verdict_id",
            "run_id",
            "job_id",
            "session_id",
            "verdict",
            "rationale",
            "findings_artifact_id",
            "created_at",
            "posture",
        ),
    ),
    TableSpec(
        "blockers",
        (
            "blocker_id",
            "run_id",
            "job_id",
            "session_id",
            "severity",
            "blocker_kind",
            "description",
            "state",
            "created_at",
            "resolved_at",
            "payload_json",
        ),
        json_columns=frozenset({"payload_json"}),
    ),
    TableSpec(
        "command_requests",
        (
            "request_id",
            "run_id",
            "session_id",
            "command_name",
            "payload_sha256",
            "response_json",
            "state",
            "created_at",
            "completed_at",
        ),
        json_columns=frozenset({"response_json"}),
    ),
    TableSpec(
        "process_executions",
        (
            "process_id",
            "run_id",
            "job_id",
            "session_id",
            "lease_id",
            "packet_id",
            "adapter",
            "command_json",
            "cwd",
            "scratch_path",
            "stdin_mode",
            "stdio_mode",
            "pid",
            "state",
            "exit_code",
            "started_at",
            "ended_at",
        ),
        json_columns=frozenset({"command_json"}),
    ),
    TableSpec(
        "events",
        (
            "event_id",
            "run_id",
            "event_type",
            "actor_session_id",
            "job_id",
            "message_id",
            "artifact_id",
            "lease_id",
            "payload_json",
            "created_at",
        ),
        json_columns=frozenset({"payload_json"}),
        order_by="event_id",
    ),
    TableSpec(
        "job_worktrees",
        (
            "worktree_id",
            "run_id",
            "job_id",
            "lease_id",
            "base_branch",
            "worktree_path",
            "state",
            "created_at",
            "released_at",
            "removed_at",
        ),
    ),
    TableSpec(
        "process_supervisors",
        (
            "supervisor_id",
            "run_id",
            "session_id",
            "adapter",
            "command_json",
            "cwd",
            "scratch_path",
            "stdin_pipe_path",
            "pid",
            "state",
            "started_at",
            "heartbeat_at",
            "ended_at",
            "stop_reason",
            "pid_start_time",
        ),
        json_columns=frozenset({"command_json"}),
    ),
    TableSpec(
        "process_supervisor_pointers",
        (
            "supervisor_id",
            "daemon_supervisor_id",
            "run_id",
            "session_id",
            "pid",
            "pid_start_time",
            "state",
            "updated_at",
            "metadata_json",
        ),
        json_columns=frozenset({"metadata_json"}),
    ),
)

TABLE_BY_NAME = {spec.name: spec for spec in TABLE_SPECS}


def migrate_repo_local(options: RepoLocalMigrationOptions) -> dict[str, Any]:
    repo = options.repo.resolve()
    with _exclusive_migrate_lock(repo):
        return _migrate_repo_local_locked(options, repo)


def verify_repo_cutover(options: RepoLocalMigrationOptions) -> dict[str, Any]:
    """Report repo-local SQLite -> Postgres cutover health without opening SQLite.

    This is intentionally separate from :func:`migrate_repo_local`: it does
    not call ``sqlite3.connect()``, does not apply daemon migrations, and does
    not resume tombstone/delete finalization. Local checks are raw file/sentinel
    stat and byte-hash checks; live-state checks are Postgres queries.
    """

    repo = options.repo.resolve()
    source_path = db_path(repo)
    tombstone_path = source_path.with_name(source_path.name + ".tombstone")
    sentinel_path = repo / ".striatum" / "state.sqlite3.migrated"
    conn = connect(options.postgres_url)
    try:
        repository_id = _lookup_registered(conn, repo)
        registration = _registration_report(conn, repository_id)
        checkpoint = _existing_checkpoint(conn, repository_id)
        destination_counts = (
            _destination_counts(conn, repository_id) if repository_id is not None else {}
        )
        count_report = _count_cutover_report(destination_counts, checkpoint)
        file_report = _sqlite_file_cutover_report(
            source_path=source_path,
            tombstone_path=tombstone_path,
            sentinel_path=sentinel_path,
            checkpoint=checkpoint,
        )
        event_chain = (
            _event_chain_report(conn, repository_id)
            if repository_id is not None
            else _event_chain_unregistered_report()
        )
    finally:
        conn.close()

    ok = (
        bool(registration["registered"])
        and checkpoint is not None
        and bool(count_report["ok"])
        and bool(file_report["ok"])
        and bool(event_chain["ok"])
    )
    recommendations: list[str] = []
    if not registration["registered"]:
        recommendations.append("register or migrate the target repository into daemon PostgreSQL")
    if checkpoint is None:
        recommendations.append(
            "run striatum daemon migrate-repo-local --from sqlite --to pg --repo "
            f"{repo}"
        )
    recommendations.extend(str(item) for item in count_report["recommendations"])
    recommendations.extend(str(item) for item in file_report["recommendations"])
    recommendations.extend(str(item) for item in event_chain["recommendations"])
    return {
        "schema_version": "striatum.repo_cutover_report.v1",
        "ok": ok,
        "mode": "repo_cutover_verification",
        "verify_only": True,
        "repo": str(repo),
        "repository": registration,
        "checkpoint": {"present": checkpoint is not None, "record": checkpoint},
        "destination_counts": count_report,
        "sqlite_finalization": file_report,
        "event_chain": event_chain,
        "sqlite_exceptions": _sqlite_exception_notes(),
        "recommendations": recommendations,
    }


def _migrate_repo_local_locked(
    options: RepoLocalMigrationOptions, repo: Path
) -> dict[str, Any]:
    source_path = db_path(repo)
    sentinel_path = repo / ".striatum" / "state.sqlite3.migrated"
    _verify_delete_options(options)
    if not source_path.exists():
        pg_conn = connect(options.postgres_url)
        try:
            apply_migrations(pg_conn)
            registered = _lookup_registered(pg_conn, repo)
            checkpoint = _existing_checkpoint(pg_conn, registered) if registered is not None else None
            if checkpoint is not None:
                resume_result = None
                if not options.dry_run:
                    resume_result = _resume_sqlite_finalization_after_checkpoint(
                        repo,
                        source_path=source_path,
                        sentinel_path=sentinel_path,
                        checkpoint=checkpoint,
                        keep_sqlite_readonly=options.keep_sqlite_readonly,
                        confirm_delete=options.confirm_delete,
                    )
                response: dict[str, Any] = {
                    "mode": "repo_local_migration",
                    "dry_run": bool(options.dry_run),
                    "already_migrated": True,
                    "repository_id": registered,
                    "checkpoint": checkpoint,
                }
                if resume_result is not None:
                    response["sqlite_finalization"] = resume_result
                return response
        finally:
            pg_conn.close()
    source_state_db_sha256 = _file_sha256(source_path)
    # V1.5 F-crash resume guard (test_rerun_with_corrupted_source case):
    # if a prior crashed migration left a sentinel + checkpoint AND the
    # current source bytes don't match the recorded sha256, refuse with
    # exit code 8 BEFORE attempting to open the bytes as SQLite. Without
    # this the corrupted source would raise a raw sqlite3.DatabaseError
    # from _verify_sqlite_current, losing the documented refusal reason.
    if sentinel_path.exists():
        pg_conn_check = connect(options.postgres_url)
        try:
            registered = _lookup_registered(pg_conn_check, repo)
            checkpoint = (
                _existing_checkpoint(pg_conn_check, registered)
                if registered is not None
                else None
            )
        finally:
            pg_conn_check.close()
        if checkpoint is not None:
            expected_sha = checkpoint.get("source_state_db_sha256")
            if isinstance(expected_sha, str) and source_state_db_sha256 != expected_sha:
                raise StriatumError(
                    "repo-local SQLite state changed since the Postgres checkpoint; "
                    "refusing to finalize. "
                    f"expected sha256={expected_sha} observed={source_state_db_sha256}",
                    exit_code=8,
                )
    sqlite_conn = _open_source_readonly(source_path)
    try:
        source_user_version = _verify_sqlite_current(sqlite_conn)
        counts = _source_counts(sqlite_conn)
        source_manifests = _source_append_only_manifests(sqlite_conn)
        pg_conn = connect(options.postgres_url)
        try:
            apply_migrations(pg_conn)
            if options.dry_run:
                registered = _lookup_registered(pg_conn, repo)
                checkpoint = _existing_checkpoint(pg_conn, registered) if registered is not None else None
                return {
                    "mode": "repo_local_migration",
                    "dry_run": True,
                    "repo": str(repo),
                    "source_state_db": str(source_path),
                    "source_user_version": source_user_version,
                    "source_counts": counts,
                    "source_event_manifest_sha256": source_manifests["events"],
                    "source_artifact_manifest_sha256": source_manifests["artifacts"],
                    "source_state_db_sha256": source_state_db_sha256,
                    "repository_id": registered,
                    "would_register_repository": registered is None,
                    "already_migrated": checkpoint is not None,
                }
            return _migrate_full(
                sqlite_conn,
                pg_conn,
                repo=repo,
                source_path=source_path,
                sentinel_path=sentinel_path,
                source_user_version=source_user_version,
                source_state_db_sha256=source_state_db_sha256,
                counts=counts,
                keep_sqlite_readonly=options.keep_sqlite_readonly,
                confirm_delete=options.confirm_delete,
            )
        finally:
            pg_conn.close()
    finally:
        sqlite_conn.close()


def compute_repo_local_reanchor(sqlite_conn: sqlite3.Connection, pg_conn: Any, repository_id: str) -> dict[str, str]:
    result: dict[str, str] = {}
    for table in ("events", "artifacts"):
        source_hash = _manifest_hash(_sqlite_manifest_rows(sqlite_conn, table))
        dest_hash = _manifest_hash(_pg_manifest_rows(pg_conn, table, repository_id))
        result[f"source_{table}_sha256"] = source_hash
        result[f"destination_{table}_sha256"] = dest_hash
        if source_hash != dest_hash:
            raise StriatumError(f"{table} manifest mismatch after repo-local migration", exit_code=13)
    return result


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
    try:
        pg_conn.rollback()
    except Exception:
        pass
    try:
        with pg_conn.cursor() as cur:
            cur.execute("BEGIN ISOLATION LEVEL SERIALIZABLE")
        repository_id = _ensure_registered(pg_conn, repo)
        checkpoint = _existing_checkpoint(pg_conn, repository_id)
        if checkpoint is not None:
            pg_conn.commit()
            resume_result = _resume_sqlite_finalization_after_checkpoint(
                repo,
                source_path=source_path,
                sentinel_path=sentinel_path,
                checkpoint=checkpoint,
                keep_sqlite_readonly=keep_sqlite_readonly,
                confirm_delete=confirm_delete,
            )
            response: dict[str, Any] = {
                "mode": "repo_local_migration",
                "dry_run": False,
                "already_migrated": True,
                "repository_id": repository_id,
                "checkpoint": checkpoint,
            }
            if resume_result is not None:
                response["sqlite_finalization"] = resume_result
            return response
        imported_counts = _copy_repo_rows(sqlite_conn, pg_conn, repository_id)
        reanchor = compute_repo_local_reanchor(sqlite_conn, pg_conn, repository_id)
        tombstone_path = str(source_path.with_name(source_path.name + ".tombstone")) if keep_sqlite_readonly else None
        _write_repo_migration_checkpoint(
            pg_conn,
            repository_id=repository_id,
            source_user_version=source_user_version,
            source_event_manifest_sha256=reanchor["source_events_sha256"],
            source_artifact_manifest_sha256=reanchor["source_artifacts_sha256"],
            source_state_db_sha256=source_state_db_sha256,
            tombstone_path=tombstone_path,
            row_counts=counts,
        )
        pg_conn.commit()
    except Exception:
        pg_conn.rollback()
        raise
    # V1.5 F-crash: write the sentinel atomically between the Postgres
    # commit and the SQLite tombstone/delete so an interrupted finalization
    # can be resumed idempotently on the next ``migrate_repo_local`` call.
    _write_sentinel_atomically(
        sentinel_path,
        repository_id=repository_id,
        source_state_db_sha256=source_state_db_sha256,
        keep_sqlite_readonly=keep_sqlite_readonly,
        confirm_delete=confirm_delete,
    )
    tombstone_result = _tombstone_or_delete_state_db(
        repo,
        keep_sqlite_readonly=keep_sqlite_readonly,
        confirm_delete=confirm_delete,
    )
    _clear_sentinel(sentinel_path)
    return {
        "mode": "repo_local_migration",
        "dry_run": False,
        "already_migrated": False,
        "repository_id": repository_id,
        "source_state_db": str(source_path),
        "source_user_version": source_user_version,
        "source_counts": counts,
        "imported_counts": imported_counts,
        "reanchor": reanchor,
        "source_state_db_sha256": source_state_db_sha256,
        "sqlite_finalization": tombstone_result,
    }


def _resume_sqlite_finalization_after_checkpoint(
    repo: Path,
    *,
    source_path: Path,
    sentinel_path: Path,
    checkpoint: dict[str, Any],
    keep_sqlite_readonly: bool,
    confirm_delete: bool,
) -> dict[str, Any] | None:
    """Finish the SQLite-side finalization after a crashed migration.

    Called from both ``already_migrated`` early-return paths. Inspects the
    ``state.sqlite3`` / sentinel pair and resumes the tombstone/delete
    action idempotently:

    * ``state.sqlite3`` present → verify the SHA against the checkpoint
      and resume the original action recorded in the sentinel (falling
      back to *keep_sqlite_readonly* + *confirm_delete* if the sentinel
      is missing). The sentinel is cleared after the action succeeds.
    * ``state.sqlite3`` absent but sentinel present → orphan; clear the
      sentinel and return.
    * Neither present → fully finalized; return ``None``.

    A SHA mismatch raises :class:`StriatumError` with exit code 8 so the
    operator can investigate without losing data.
    """
    sentinel_payload = _read_sentinel(sentinel_path)
    if source_path.exists():
        observed_sha = _file_sha256(source_path)
        expected_sha = checkpoint.get("source_state_db_sha256")
        if isinstance(expected_sha, str) and observed_sha != expected_sha:
            raise StriatumError(
                "repo-local SQLite state changed since the Postgres checkpoint; "
                "refusing to finalize. "
                f"expected sha256={expected_sha} observed={observed_sha}",
                exit_code=8,
            )
        resume_keep = keep_sqlite_readonly
        resume_confirm = confirm_delete
        if sentinel_payload is not None:
            if "keep_sqlite_readonly" in sentinel_payload:
                resume_keep = bool(sentinel_payload["keep_sqlite_readonly"])
            if "confirm_delete" in sentinel_payload:
                resume_confirm = bool(sentinel_payload["confirm_delete"])
        result = _tombstone_or_delete_state_db(
            repo,
            keep_sqlite_readonly=resume_keep,
            confirm_delete=resume_confirm,
        )
        result = {**result, "resumed_from_checkpoint": True}
        _clear_sentinel(sentinel_path)
        return result
    if sentinel_path.exists():
        _clear_sentinel(sentinel_path)
        return {"action": "cleared_orphan_sentinel", "sentinel": str(sentinel_path)}
    return None


def _write_sentinel_atomically(
    sentinel_path: Path,
    *,
    repository_id: str,
    source_state_db_sha256: str,
    keep_sqlite_readonly: bool,
    confirm_delete: bool,
) -> None:
    payload = {
        "repository_id": repository_id,
        "source_state_db_sha256": source_state_db_sha256,
        "keep_sqlite_readonly": bool(keep_sqlite_readonly),
        "confirm_delete": bool(confirm_delete),
        "written_at": utc_now(),
    }
    body = json.dumps(payload, sort_keys=True, separators=(",", ":")).encode("utf-8")
    sentinel_path.parent.mkdir(parents=True, exist_ok=True)
    tmp_path = sentinel_path.with_name(sentinel_path.name + ".tmp")
    with tmp_path.open("wb") as handle:
        handle.write(body)
        handle.flush()
        os.fsync(handle.fileno())
    tmp_path.replace(sentinel_path)


def _read_sentinel(sentinel_path: Path) -> dict[str, Any] | None:
    if not sentinel_path.exists():
        return None
    try:
        loaded = json.loads(sentinel_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None
    return loaded if isinstance(loaded, dict) else None


def _clear_sentinel(sentinel_path: Path) -> None:
    try:
        sentinel_path.unlink()
    except FileNotFoundError:
        pass


def _open_source_readonly(path: Path) -> sqlite3.Connection:
    if not path.is_file():
        raise StriatumError(f"repo-local SQLite state not found: {path}", exit_code=3)
    uri = f"file:{path}?mode=ro"
    conn = sqlite3.connect(uri, uri=True)
    conn.row_factory = sqlite3.Row
    return conn


def _verify_sqlite_current(conn: sqlite3.Connection) -> int:
    version = current_user_version(conn)
    if version != LATEST_VERSION:
        raise SchemaVersionError(
            "repo-local SQLite schema must be migrated before PostgreSQL cutover: "
            f"database user_version={version}, runner supports={LATEST_VERSION}"
        )
    return version


def _source_counts(conn: sqlite3.Connection) -> dict[str, int]:
    return {
        spec.name: int(conn.execute(f"SELECT COUNT(*) FROM {spec.name}").fetchone()[0])
        for spec in TABLE_SPECS
    }


def _lookup_registered(conn: Any, repo: Path) -> str | None:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT repository_id
            FROM striatumd.repositories
            WHERE state != 'removed' AND repo_root = %s
            ORDER BY repository_id
            LIMIT 1
            """,
            (str(repo),),
        )
        row = cur.fetchone()
        if row is not None:
            return str(row[0])
        state_db = db_path(repo)
        if not state_db.exists():
            return None
        identity = daemon_registry._repo_identity(repo)
        cur.execute(
            """
            SELECT repository_id
            FROM striatumd.repositories
            WHERE state != 'removed' AND repo_identity = %s
            ORDER BY repository_id
            LIMIT 1
            """,
            (identity,),
        )
        row = cur.fetchone()
    return None if row is None else str(row[0])


def _ensure_registered(conn: Any, repo: Path) -> str:
    existing = _lookup_registered(conn, repo)
    if existing is not None:
        return existing
    repository_id = f"repo_{uuid.uuid4().hex}"
    now = utc_now()
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories(repository_id, repo_identity, repo_root,
              state_db_path, display_name, registered_at, removed_at, last_seen_at,
              last_schema_version, state, settings_json)
            VALUES (%s, %s, %s, %s, %s, %s, NULL, %s, %s, 'active', '{}'::jsonb)
            """,
            (
                repository_id,
                daemon_registry._repo_identity(repo),
                str(repo),
                str(db_path(repo)),
                repo.name,
                now,
                now,
                LATEST_VERSION,
            ),
        )
    return repository_id


def _existing_checkpoint(conn: Any, repository_id: str | None) -> dict[str, Any] | None:
    if repository_id is None:
        return None
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT source_user_version, source_event_manifest_sha256,
              source_artifact_manifest_sha256, source_state_db_sha256,
              migrated_at, tombstone_path, row_counts
            FROM striatumd.repo_migrations
            WHERE repository_id = %s AND source_substrate = 'sqlite'
              AND target_substrate = 'postgres'
            """,
            (repository_id,),
        )
        row = cur.fetchone()
    if row is None:
        return None
    return {
        "source_user_version": int(row[0]),
        "source_event_manifest_sha256": row[1],
        "source_artifact_manifest_sha256": row[2],
        "source_state_db_sha256": row[3],
        "migrated_at": _normalize_value(row[4]),
        "tombstone_path": row[5],
        "row_counts": row[6],
    }


def _registration_report(conn: Any, repository_id: str | None) -> dict[str, Any]:
    if repository_id is None:
        return {"registered": False, "repository_id": None}
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT repository_id, repo_identity, repo_root, state_db_path,
              display_name, registered_at, last_seen_at, last_schema_version, state
            FROM striatumd.repositories
            WHERE repository_id = %s
            """,
            (repository_id,),
        )
        row = cur.fetchone()
    if row is None:
        return {"registered": False, "repository_id": repository_id}
    return {
        "registered": True,
        "repository_id": str(row[0]),
        "repo_identity": row[1],
        "repo_root": row[2],
        "state_db_path": row[3],
        "display_name": row[4],
        "registered_at": _normalize_value(row[5]),
        "last_seen_at": _normalize_value(row[6]),
        "last_schema_version": row[7],
        "state": row[8],
    }


def _destination_counts(conn: Any, repository_id: str) -> dict[str, int]:
    counts: dict[str, int] = {}
    with conn.cursor() as cur:
        for spec in TABLE_SPECS:
            cur.execute(
                f"SELECT COUNT(*) FROM striatumd.{spec.name} WHERE repository_id = %s",
                (repository_id,),
            )
            counts[spec.name] = int(cur.fetchone()[0])
    return counts


def _checkpoint_row_counts(checkpoint: dict[str, Any] | None) -> dict[str, int]:
    if checkpoint is None:
        return {}
    raw_counts = checkpoint.get("row_counts") or {}
    if isinstance(raw_counts, str):
        try:
            raw_counts = json.loads(raw_counts)
        except json.JSONDecodeError:
            raw_counts = {}
    if not isinstance(raw_counts, dict):
        return {}
    counts: dict[str, int] = {}
    for spec in TABLE_SPECS:
        value = raw_counts.get(spec.name, 0)
        try:
            counts[spec.name] = int(value)
        except (TypeError, ValueError):
            counts[spec.name] = 0
    return counts


def _count_cutover_report(
    destination_counts: dict[str, int],
    checkpoint: dict[str, Any] | None,
) -> dict[str, Any]:
    checkpoint_counts = _checkpoint_row_counts(checkpoint)
    violations: list[dict[str, Any]] = []
    for table, expected in checkpoint_counts.items():
        actual = int(destination_counts.get(table, 0))
        if actual < expected:
            violations.append({"table": table, "checkpoint_count": expected, "actual_count": actual})
    recommendations: list[str] = []
    if checkpoint is None:
        recommendations.append("no repo_migrations checkpoint is present for sqlite -> postgres")
    if violations:
        recommendations.append(
            "destination Postgres row counts are below the migration checkpoint; "
            "inspect the daemon DB or restore from backup"
        )
    return {
        "ok": checkpoint is not None and not violations,
        "actual": destination_counts,
        "checkpoint_minimum": checkpoint_counts,
        "not_below_checkpoint": not violations if checkpoint is not None else False,
        "violations": violations,
        "recommendations": recommendations,
    }


def _sqlite_file_cutover_report(
    *,
    source_path: Path,
    tombstone_path: Path,
    sentinel_path: Path,
    checkpoint: dict[str, Any] | None,
) -> dict[str, Any]:
    expected_tombstone_path = _expected_tombstone_path(tombstone_path, checkpoint)
    source = _file_stat_report(source_path)
    tombstone = _file_stat_report(expected_tombstone_path)
    sentinel = _sentinel_stat_report(sentinel_path)
    expected_action = "unknown"
    if checkpoint is not None:
        expected_action = "tombstone" if checkpoint.get("tombstone_path") else "delete"
    source_sha_matches = _report_sha_matches_checkpoint(source, checkpoint)
    tombstone_sha_matches = _report_sha_matches_checkpoint(tombstone, checkpoint)
    recommendations: list[str] = []
    diagnosis: list[str] = []
    source_exists = bool(source["exists"])
    tombstone_exists = bool(tombstone["exists"])
    sentinel_exists = bool(sentinel["exists"])
    ok = False
    if checkpoint is None:
        status = "not_migrated" if source_exists else "checkpoint_missing"
    elif source_exists:
        status = (
            "incomplete_finalization_after_checkpoint"
            if sentinel_exists
            else "source_left_after_checkpoint"
        )
        diagnosis.append(
            "Postgres checkpoint exists but .striatum/state.sqlite3 is still present"
        )
        if source_sha_matches is False:
            diagnosis.append("source_state_db_sha256 differs from the checkpoint")
        recommendations.append(
            "rerun migrate-repo-local without --verify-cutover only after "
            "confirming the source hash matches the checkpoint"
        )
    elif sentinel_exists:
        status = "orphan_sentinel_after_finalization"
        diagnosis.append("migration sentinel remains after source finalization")
        recommendations.append("rerun migrate-repo-local without --verify-cutover to clear the sentinel")
    elif expected_action == "tombstone":
        if not tombstone_exists:
            status = "tombstone_missing"
            diagnosis.append("checkpoint expected a read-only tombstone, but it is absent")
            recommendations.append("inspect operator cleanup history or restore the tombstone if needed")
        else:
            tombstone_readonly = tombstone.get("mode") == "0444"
            if tombstone_readonly and tombstone_sha_matches is not False:
                status = "tombstoned"
                ok = True
            else:
                status = "tombstone_drift"
                if not tombstone_readonly:
                    diagnosis.append("tombstone mode is not 0444")
                    recommendations.append("chmod the tombstone to 0444 after inspection")
                if tombstone_sha_matches is False:
                    diagnosis.append("tombstone sha256 differs from the migration checkpoint")
    elif expected_action == "delete":
        if tombstone_exists:
            status = "deleted_with_unexpected_tombstone"
            diagnosis.append("checkpoint recorded delete finalization, but a tombstone exists")
            recommendations.append("inspect the unexpected tombstone before removing it")
        else:
            status = "deleted"
            ok = True
    else:
        status = "unknown"
    if sentinel_exists and checkpoint is None:
        diagnosis.append("migration sentinel exists without a Postgres checkpoint")
    return {
        "ok": ok,
        "status": status,
        "expected_action": expected_action,
        "source_state_db_absent": not source_exists,
        "sentinel_absent": not sentinel_exists,
        "source_sha256_matches_checkpoint": source_sha_matches,
        "tombstone_sha256_matches_checkpoint": tombstone_sha_matches,
        "source_state_db": source,
        "tombstone": tombstone,
        "sentinel": sentinel,
        "diagnosis": diagnosis,
        "recommendations": recommendations,
    }


def _expected_tombstone_path(
    default_path: Path,
    checkpoint: dict[str, Any] | None,
) -> Path:
    if checkpoint is None:
        return default_path
    raw_path = checkpoint.get("tombstone_path")
    if isinstance(raw_path, str) and raw_path:
        return Path(raw_path)
    return default_path


def _file_stat_report(path: Path) -> dict[str, Any]:
    exists = path.exists()
    report: dict[str, Any] = {
        "path": str(path),
        "exists": exists,
        "is_file": path.is_file() if exists else False,
    }
    if not exists:
        return report
    try:
        stat = path.stat()
    except OSError as exc:
        report["stat_error"] = str(exc)
        return report
    report.update(
        {
            "mode": f"{stat.st_mode & 0o777:04o}",
            "size_bytes": int(stat.st_size),
        }
    )
    if report["is_file"]:
        try:
            report["sha256"] = _raw_file_sha256(path)
        except OSError as exc:
            report["sha256_error"] = str(exc)
    return report


def _sentinel_stat_report(path: Path) -> dict[str, Any]:
    report = _file_stat_report(path)
    if not report.get("is_file"):
        return report
    try:
        loaded = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        report["json_valid"] = False
        report["json_error"] = str(exc)
        return report
    report["json_valid"] = isinstance(loaded, dict)
    if isinstance(loaded, dict):
        report["payload"] = loaded
    return report


def _raw_file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _report_sha_matches_checkpoint(
    report: dict[str, Any],
    checkpoint: dict[str, Any] | None,
) -> bool | None:
    if checkpoint is None or not report.get("exists"):
        return None
    expected = checkpoint.get("source_state_db_sha256")
    observed = report.get("sha256")
    if not isinstance(expected, str) or not isinstance(observed, str):
        return None
    return observed == expected


def _event_chain_unregistered_report() -> dict[str, Any]:
    return {
        "ok": False,
        "status": "repository_not_registered",
        "event_count": 0,
        "anchored_event_count": 0,
        "legacy_unanchored_event_count": 0,
        "payload_side_anchor_count": 0,
        "head": None,
        "problems": [],
        "recommendations": ["repository must be registered before event-chain health can be checked"],
    }


def _event_chain_report(conn: Any, repository_id: str) -> dict[str, Any]:
    rows = _event_chain_rows(conn, repository_id)
    head = _event_chain_head(conn, repository_id)
    problems: list[dict[str, Any]] = []
    payload_side_anchor_count = 0
    anchored_count = 0
    legacy_unanchored_count = 0
    previous_anchored_hash: str | None = None
    anchored_seen = False
    latest_anchored: dict[str, Any] | None = None
    for row in rows:
        event_id = int(row["event_id"])
        payload = row.get("payload_json")
        if isinstance(payload, dict) and "_event_chain" in payload:
            payload_side_anchor_count += 1
            problems.append({"event_id": event_id, "problem": "payload_side_event_chain_anchor"})
        row_hash = row.get("row_hash")
        previous_hash = row.get("previous_hash")
        if isinstance(row_hash, str) and row_hash:
            anchored_count += 1
            if anchored_seen and previous_hash != previous_anchored_hash:
                problems.append(
                    {
                        "event_id": event_id,
                        "problem": "previous_hash_mismatch",
                        "expected_previous_hash": previous_anchored_hash,
                        "actual_previous_hash": previous_hash,
                    }
                )
            if not anchored_seen and previous_hash is not None:
                problems.append(
                    {
                        "event_id": event_id,
                        "problem": "first_anchored_event_has_previous_hash",
                        "actual_previous_hash": previous_hash,
                    }
                )
            computed_hash = canonical_event_hash(row, previous_hash=previous_hash)
            if computed_hash != row_hash:
                problems.append(
                    {
                        "event_id": event_id,
                        "problem": "row_hash_mismatch",
                        "expected_row_hash": computed_hash,
                        "actual_row_hash": row_hash,
                    }
                )
            anchored_seen = True
            previous_anchored_hash = row_hash
            latest_anchored = {"event_id": event_id, "row_hash": row_hash}
        else:
            legacy_unanchored_count += 1
            if anchored_seen:
                problems.append({"event_id": event_id, "problem": "unanchored_after_anchor"})
    if anchored_count:
        if head is None:
            problems.append({"problem": "missing_event_chain_head"})
        elif latest_anchored is not None and (
            int(head["last_event_id"]) != latest_anchored["event_id"]
            or head["last_hash"] != latest_anchored["row_hash"]
        ):
            problems.append(
                {
                    "problem": "event_chain_head_mismatch",
                    "expected": latest_anchored,
                    "actual": head,
                }
            )
    elif head is not None:
        problems.append({"problem": "head_without_anchored_events", "actual": head})
    status = _event_chain_status(
        event_count=len(rows),
        anchored_count=anchored_count,
        legacy_unanchored_count=legacy_unanchored_count,
        problems=problems,
    )
    recommendations: list[str] = []
    if problems:
        recommendations.append("run daemon doctor and inspect striatumd.events chain anchors")
    return {
        "ok": not problems,
        "status": status,
        "event_count": len(rows),
        "anchored_event_count": anchored_count,
        "legacy_unanchored_event_count": legacy_unanchored_count,
        "payload_side_anchor_count": payload_side_anchor_count,
        "head": head,
        "latest_anchored_event": latest_anchored,
        "problems": problems,
        "recommendations": recommendations,
    }


def _event_chain_rows(conn: Any, repository_id: str) -> list[dict[str, Any]]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT repository_id, event_id, run_id, event_type, actor_session_id,
              job_id, message_id, artifact_id, lease_id, payload_json, created_at,
              previous_hash, row_hash
            FROM striatumd.events
            WHERE repository_id = %s
            ORDER BY event_id
            """,
            (repository_id,),
        )
        rows = cur.fetchall()
        names = [desc[0] for desc in cur.description]
    return [dict(zip(names, row, strict=True)) for row in rows]


def _event_chain_head(conn: Any, repository_id: str) -> dict[str, Any] | None:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT last_event_id, last_hash, updated_at
            FROM striatumd.repo_event_chain_heads
            WHERE repository_id = %s
            """,
            (repository_id,),
        )
        row = cur.fetchone()
    if row is None:
        return None
    return {
        "last_event_id": int(row[0]),
        "last_hash": row[1],
        "updated_at": _normalize_value(row[2]),
    }


def _event_chain_status(
    *,
    event_count: int,
    anchored_count: int,
    legacy_unanchored_count: int,
    problems: list[dict[str, Any]],
) -> str:
    if problems:
        return "broken"
    if event_count == 0:
        return "empty"
    if anchored_count == 0:
        return "legacy_unanchored"
    if legacy_unanchored_count:
        return "mixed_legacy_and_anchored"
    return "anchored"


def _sqlite_exception_notes() -> list[dict[str, str]]:
    return [
        {
            "scope": "migration_source_import",
            "note": (
                "migrate-repo-local may open .striatum/state.sqlite3 read-only "
                "only when importing a pre-D094 source; --verify-cutover does not"
            ),
        },
        {
            "scope": "operator_tombstone_inspection",
            "note": (
                "a .striatum/state.sqlite3.tombstone is optional operator evidence; "
                "Striatum does not use it as live state"
            ),
        },
        {
            "scope": "tests_and_fixtures",
            "note": "legacy SQLite remains in bounded tests, fixtures, and migration code only",
        },
    ]


def _copy_repo_rows(sqlite_conn: sqlite3.Connection, pg_conn: Any, repository_id: str) -> dict[str, int]:
    counts: dict[str, int] = {}
    with pg_conn.cursor() as cur:
        for spec in TABLE_SPECS:
            rows = sqlite_conn.execute(
                f"SELECT {', '.join(spec.columns)} FROM {spec.name} ORDER BY {spec.order_by or spec.columns[0]}"
            ).fetchall()
            if not rows:
                counts[spec.name] = 0
                continue
            target_columns = ("repository_id", *spec.columns)
            placeholders = ["%s"]
            for column in spec.columns:
                placeholders.append("%s::jsonb" if column in spec.json_columns else "%s")
            sql = (
                f"INSERT INTO striatumd.{spec.name}({', '.join(target_columns)}) "
                f"VALUES ({', '.join(placeholders)})"
            )
            for row in rows:
                values = [repository_id]
                for column in spec.columns:
                    value = row[column]
                    if column in spec.bool_columns and value is not None:
                        value = bool(value)
                    elif column in spec.json_columns and value is not None:
                        value = _canonical_json(value)
                    values.append(value)
                cur.execute(sql, tuple(values))
            counts[spec.name] = len(rows)
        cur.execute(
            """
            SELECT setval(
              pg_get_serial_sequence('striatumd.events', 'event_id'),
              GREATEST((SELECT COALESCE(MAX(event_id), 1) FROM striatumd.events), 1),
              true
            )
            """
        )
    return counts


def _write_repo_migration_checkpoint(
    conn: Any,
    *,
    repository_id: str,
    source_user_version: int,
    source_event_manifest_sha256: str,
    source_artifact_manifest_sha256: str,
    source_state_db_sha256: str,
    tombstone_path: str | None,
    row_counts: dict[str, int],
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repo_migrations(repository_id, source_substrate,
              target_substrate, source_user_version, source_event_manifest_sha256,
              source_artifact_manifest_sha256, source_state_db_sha256, tombstone_path,
              row_counts)
            VALUES (%s, 'sqlite', 'postgres', %s, %s, %s, %s, %s, %s::jsonb)
            """,
            (
                repository_id,
                source_user_version,
                source_event_manifest_sha256,
                source_artifact_manifest_sha256,
                source_state_db_sha256,
                tombstone_path,
                json_dumps(row_counts),
            ),
        )


def _tombstone_or_delete_state_db(
    repo: Path,
    *,
    keep_sqlite_readonly: bool,
    confirm_delete: bool,
) -> dict[str, Any]:
    source = db_path(repo)
    tombstone = source.with_name(source.name + ".tombstone")
    if keep_sqlite_readonly:
        if source.exists():
            source.replace(tombstone)
        if tombstone.exists():
            tombstone.chmod(0o444)
        _remove_sqlite_sidecars(source)
        return {"action": "tombstone", "path": str(tombstone), "mode": "0444"}
    if not confirm_delete:
        raise StriatumError("--confirm-delete is required when --no-keep-sqlite-readonly is used", exit_code=8)
    if source.exists():
        source.unlink()
    _remove_sqlite_sidecars(source)
    return {"action": "delete", "path": str(source)}


def _verify_delete_options(options: RepoLocalMigrationOptions) -> None:
    if not options.keep_sqlite_readonly and not options.confirm_delete:
        raise StriatumError("--confirm-delete is required when --no-keep-sqlite-readonly is used", exit_code=8)


def _remove_sqlite_sidecars(source: Path) -> None:
    for suffix in ("-wal", "-shm"):
        sidecar = source.with_name(source.name + suffix)
        try:
            if sidecar.exists():
                sidecar.unlink()
        except FileNotFoundError:
            pass


def _source_append_only_manifests(conn: sqlite3.Connection) -> dict[str, str]:
    return {
        "events": _manifest_hash(_sqlite_manifest_rows(conn, "events")),
        "artifacts": _manifest_hash(_sqlite_manifest_rows(conn, "artifacts")),
    }


def _sqlite_manifest_rows(conn: sqlite3.Connection, table: str) -> list[dict[str, Any]]:
    spec = TABLE_BY_NAME[table]
    rows = conn.execute(
        f"SELECT {', '.join(spec.columns)} FROM {table} ORDER BY {spec.order_by or spec.columns[0]}"
    ).fetchall()
    return [_project_row(row, spec) for row in rows]


def _pg_manifest_rows(conn: Any, table: str, repository_id: str) -> list[dict[str, Any]]:
    spec = TABLE_BY_NAME[table]
    with conn.cursor() as cur:
        cur.execute(
            f"""
            SELECT {', '.join(spec.columns)}
            FROM striatumd.{table}
            WHERE repository_id = %s
            ORDER BY {spec.order_by or spec.columns[0]}
            """,
            (repository_id,),
        )
        rows = cur.fetchall()
        names = [desc[0] for desc in cur.description]
    return [_project_mapping(dict(zip(names, row, strict=True)), spec) for row in rows]


def _project_row(row: sqlite3.Row, spec: TableSpec) -> dict[str, Any]:
    return _project_mapping({column: row[column] for column in spec.columns}, spec)


def _project_mapping(row: dict[str, Any], spec: TableSpec) -> dict[str, Any]:
    projected: dict[str, Any] = {}
    for column in spec.columns:
        value = row[column]
        if column in spec.json_columns and value is not None:
            value = _canonical_json(value)
        else:
            value = _normalize_value(value)
        projected[column] = value
    return projected


def _manifest_hash(rows: list[dict[str, Any]]) -> str:
    payload = json.dumps(rows, sort_keys=True, separators=(",", ":")).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def _canonical_json(value: Any) -> str:
    if isinstance(value, str):
        loaded = json.loads(value)
    else:
        loaded = value
    return json.dumps(loaded, sort_keys=True, separators=(",", ":"))


def _normalize_value(value: Any) -> Any:
    if isinstance(value, datetime):
        if value.tzinfo is None:
            value = value.replace(tzinfo=UTC)
        value = value.astimezone(UTC).replace(microsecond=0)
        return value.isoformat().replace("+00:00", "Z")
    return value


def _file_sha256(path: Path) -> str:
    if not path.is_file():
        raise StriatumError(f"repo-local SQLite state not found: {path}", exit_code=3)
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()
