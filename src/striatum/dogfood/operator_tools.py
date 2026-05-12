"""Dogfood operator composite tools.

These helpers intentionally sit outside the generic recovery CLI. They encode
dogfood-run repair sequences that are too privileged to make ordinary workflow
policy.
"""

from __future__ import annotations

import json
import os
import sqlite3
from datetime import UTC, datetime, timedelta
from pathlib import Path, PurePosixPath
from typing import Any, cast

from striatum.artifacts import publish_artifact
from striatum.cli.mutations import ack_work
from striatum.db import (
    JsonObject,
    active_lease_for,
    complete_job,
    insert_event,
    record_review_verdict,
    row_by_id,
    transaction,
    utc_now,
)
from striatum.daemon_supervisor.progress_watcher import progress_advisory_lock
from striatum.errors import InvalidTransitionError
from striatum.identity import process_start_time

_ACTIVE_SUPERVISOR_STATES = ("starting", "attached", "detached")
_TERMINAL_JOB_STATES = {"completed", "failed", "canceled", "skipped"}
_MAX_REASON_LENGTH = 1000
_MIN_EXTEND_SECONDS = 60
_MAX_EXTEND_SECONDS = 3600


class PublishOnBehalfDenied(InvalidTransitionError):
    """Compatibility exception for callers that prefer exception denials."""

    def __init__(self, denial_code: str, message: str) -> None:
        super().__init__(message)
        self.denial_code = denial_code


def publish_on_behalf(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    session_id: str,
    artifact_path: str,
    artifact_kind: str,
    logical_name: str,
    reason: str,
    verdict: str | None = None,
    findings_artifact_id: str | None = None,
    verdict_rationale: str | None = None,
    summary: str | None = None,
) -> JsonObject:
    """Acknowledge, publish, and complete/verdict for a supervised lane."""
    reason = reason.strip()
    if not reason:
        return _failure("reason_required", "operator reason must not be empty")
    if len(reason) > _MAX_REASON_LENGTH:
        return _failure(
            "reason_too_long",
            f"operator reason must be at most {_MAX_REASON_LENGTH} characters",
        )

    work = _active_work_for_artifact(
        conn,
        session_id=session_id,
        artifact_path=artifact_path,
        artifact_kind=artifact_kind,
        logical_name=logical_name,
    )
    if work.get("ok") is not True:
        return work
    job = work["job"]
    lease = work["lease"]
    message = work["message"]
    job_id = str(job["job_id"])
    lease_id = str(lease["lease_id"])
    message_id = str(message["message_id"]) if message is not None else None

    if str(job["job_type"]) == "review" and verdict is None:
        return _failure("verdict_required", "review publish-on-behalf requires a verdict")
    if str(job["job_type"]) != "review" and verdict is not None:
        return _failure("verdict_not_allowed", "verdict is valid only for review jobs")

    composition_steps: list[JsonObject] = []
    ack_result: JsonObject | None = None
    if str(job["state"]) == "claimed":
        if message_id is None:
            return _failure("queue_message_missing", "claimed job has no current message")
        ack_result = ack_work(
            conn,
            session_id=session_id,
            message_id=message_id,
            lease_id=lease_id,
        )
        composition_steps.append({"step": "ack", "status": str(ack_result["status"])})
    else:
        composition_steps.append({"step": "ack", "status": "already_acked"})

    artifact = publish_artifact(
        conn,
        repo=repo,
        session_id=session_id,
        job_id=job_id,
        lease_id=lease_id,
        kind=artifact_kind,
        logical_name=logical_name,
        path_text=artifact_path,
    )
    artifact_id = str(artifact["artifact_id"])
    composition_steps.append({"step": "publish_artifact", "status": str(artifact["status"])})

    result: JsonObject = {
        "ok": True,
        "status": "published_on_behalf",
        "operation": "publish_on_behalf",
        "job_id": job_id,
        "session_id": session_id,
        "lease_id": lease_id,
        "message_id": message_id,
        "artifact_id": artifact_id,
        "artifact": artifact,
        "composition_steps": composition_steps,
        "reason": reason,
    }
    if ack_result is not None:
        result["ack"] = ack_result

    job = row_by_id(conn, "jobs", "job_id", job_id)
    if str(job["job_type"]) == "review":
        assert verdict is not None
        verdict_result = record_review_verdict(
            conn,
            session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            verdict=verdict,
            findings_artifact_id=findings_artifact_id or artifact_id,
            rationale=verdict_rationale,
        )
        result["status"] = "published_on_behalf_reviewed"
        result["verdict"] = verdict_result
        if "verdict_id" in verdict_result:
            result["verdict_id"] = verdict_result["verdict_id"]
        composition_steps.append({"step": "verdict", "status": str(verdict_result["status"])})
    else:
        completion = complete_job(
            conn,
            session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            summary=summary or f"Published on behalf: {reason}",
        )
        result["status"] = "published_on_behalf_completed"
        result["completion"] = completion
        composition_steps.append({"step": "complete", "status": str(completion["status"])})

    with transaction(conn):
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="dogfood.publish_on_behalf",
            actor_session_id=session_id,
            job_id=job_id,
            message_id=message_id,
            lease_id=lease_id,
            artifact_id=artifact_id,
            payload={
                "operation": "publish_on_behalf",
                "operator_reason": reason,
                "artifact_path": artifact_path,
                "artifact_kind": artifact_kind,
                "logical_name": logical_name,
                "verdict": verdict,
                "composition_steps": composition_steps,
            },
        )
    return result


def _active_work_for_artifact(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    artifact_path: str,
    artifact_kind: str,
    logical_name: str,
) -> JsonObject:
    rows = conn.execute(
        """
        SELECT l.lease_id, l.resource_id AS job_id, j.current_message_id,
               j.current_lease_id AS job_lease_id, j.state AS job_state,
               qm.message_id, qm.state AS message_state,
               qm.current_lease_id AS message_lease_id
        FROM leases l
        JOIN jobs j ON j.job_id = l.resource_id
        LEFT JOIN queue_messages qm ON qm.message_id = j.current_message_id
        WHERE l.owner_session_id = ?
          AND l.resource_type = 'job'
          AND l.state = 'active'
          AND l.expires_at >= ?
          AND j.state NOT IN ('completed', 'failed', 'canceled', 'skipped')
        ORDER BY l.acquired_at DESC, l.lease_id DESC
        """,
        (session_id, utc_now()),
    ).fetchall()
    if len(rows) != 1:
        return _failure(
            "no_active_lease",
            "publish_on_behalf requires exactly one active non-terminal lease for the session",
            details={"active_lease_count": len(rows)},
        )

    row = rows[0]
    lease_id = str(row["lease_id"])
    job_id = str(row["job_id"])
    try:
        lease = active_lease_for(
            conn, lease_id=lease_id, session_id=session_id, job_id=job_id
        )
    except Exception as exc:
        return _failure("no_active_lease", str(exc))
    job = row_by_id(conn, "jobs", "job_id", job_id)
    message = None
    if row["message_id"] is not None:
        message = row_by_id(conn, "queue_messages", "message_id", str(row["message_id"]))

    if message is None:
        return _failure("lease_busy", "active lease has no current queue message")
    if str(row["job_lease_id"]) != lease_id or str(row["message_lease_id"]) != lease_id:
        return _failure(
            "lease_busy",
            "active lease is not current for both job and queue message",
        )
    message_state = str(row["message_state"])
    job_state = str(row["job_state"])
    if (message_state, job_state) not in {("claimed", "claimed"), ("acked", "running")}:
        return _failure(
            "lease_busy",
            "active lease is not in a recoverable claimed or acked shape",
            details={"job_state": job_state, "message_state": message_state},
        )

    contract = _validate_publish_artifact_contract(
        job=job,
        artifact_path=artifact_path,
        artifact_kind=artifact_kind,
        logical_name=logical_name,
    )
    if contract.get("ok") is not True:
        return contract
    return {"ok": True, "job": job, "lease": lease, "message": message}


def _validate_publish_artifact_contract(
    *,
    job: sqlite3.Row,
    artifact_path: str,
    artifact_kind: str,
    logical_name: str,
) -> JsonObject:
    try:
        expected = json.loads(str(job["expected_artifacts_json"]))
    except json.JSONDecodeError:
        return _failure("expected_artifacts_invalid", "expected artifacts JSON is invalid")
    if not isinstance(expected, list):
        return _failure("expected_artifacts_invalid", "expected artifacts must be a list")
    for item in expected:
        if not isinstance(item, dict):
            continue
        if (
            item.get("path") == artifact_path
            and item.get("kind") == artifact_kind
            and item.get("logical_name") == logical_name
        ):
            return {"ok": True}
    return _failure(
        "artifact_not_expected",
        "artifact tuple is not declared in the job expected_artifacts contract",
        details={
            "artifact_path": artifact_path,
            "artifact_kind": artifact_kind,
            "logical_name": logical_name,
        },
    )


def surgical_recovery(
    conn: sqlite3.Connection,
    *,
    job_id: str,
    reason: str,
    extend_lease_seconds: int = 900,
) -> JsonObject:
    """Restore one stale repo-write dogfood job after operator inspection.

    The function returns structured failure envelopes instead of raising for
    ordinary validation denials. Successful recovery is atomic: the expired
    lease, queue message, job row, and matching lost supervisor/pointer are
    updated in one SQLite transaction.
    """
    reason = reason.strip()
    if not reason:
        return _failure("reason_required", "operator reason must not be empty")
    if len(reason) > _MAX_REASON_LENGTH:
        return _failure(
            "reason_too_long",
            f"operator reason must be at most {_MAX_REASON_LENGTH} characters",
        )
    if extend_lease_seconds < _MIN_EXTEND_SECONDS or extend_lease_seconds > _MAX_EXTEND_SECONDS:
        return _failure(
            "invalid_lease_extension",
            (
                "extend_lease_seconds must be between "
                f"{_MIN_EXTEND_SECONDS} and {_MAX_EXTEND_SECONDS}"
            ),
            details={"extend_lease_seconds": extend_lease_seconds},
        )

    with transaction(conn):
        job = conn.execute("SELECT * FROM jobs WHERE job_id = ?", (job_id,)).fetchone()
        if job is None:
            return _failure("job_not_found", f"job not found: {job_id}")
        run_id = str(job["run_id"])
        run = conn.execute("SELECT * FROM runs WHERE run_id = ?", (run_id,)).fetchone()
        if run is None:
            return _failure("run_not_found", f"run not found for job: {run_id}")
        lock = progress_advisory_lock(Path(str(run["repo_root"])), job_id=job_id)
        lock_acquired = lock.__enter__()
        if not lock_acquired:
            lock.__exit__(None, None, None)
            return _failure("progress_lock_busy", "job progress watcher is currently refreshing this lease")
        try:
            result = _surgical_recovery_locked(
                conn,
                job=job,
                run=run,
                job_id=job_id,
                reason=reason,
                extend_lease_seconds=extend_lease_seconds,
            )
        finally:
            lock.__exit__(None, None, None)
        return result


def _surgical_recovery_locked(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
    run: sqlite3.Row,
    job_id: str,
    reason: str,
    extend_lease_seconds: int,
) -> JsonObject:
    run_id = str(job["run_id"])

    validation = _validate_recoverable_shape(conn, job=job, run=run)
    if validation.get("ok") is not True:
        return validation

    lease = validation["lease"]
    message = validation["message"]
    supervisor = validation.get("supervisor")
    pointer = validation.get("pointer")
    validated_paths = validation["validated_artifact_paths"]
    session_id = str(lease["owner_session_id"])

    now = utc_now()
    new_expires_at = (
        datetime.now(UTC) + timedelta(seconds=extend_lease_seconds)
    ).replace(microsecond=0).isoformat().replace("+00:00", "Z")

    before: JsonObject = {
        "job_state": str(job["state"]),
        "lease_state": str(lease["state"]),
        "message_state": str(message["state"]),
    }
    if supervisor is not None:
        before["supervisor_state"] = str(supervisor["state"])
    if pointer is not None:
        before["supervisor_pointer_state"] = str(pointer["state"])

    conn.execute(
        """
        UPDATE leases
        SET state = 'active',
            expires_at = ?,
            last_heartbeat_at = ?,
            released_at = NULL,
            release_reason = NULL
        WHERE lease_id = ?
        """,
        (new_expires_at, now, str(lease["lease_id"])),
    )
    conn.execute(
        """
        UPDATE queue_messages
        SET state = 'acked', current_lease_id = ?, updated_at = ?
        WHERE message_id = ?
        """,
        (str(lease["lease_id"]), now, str(message["message_id"])),
    )
    conn.execute(
        """
        UPDATE jobs
        SET state = 'running', current_lease_id = ?, current_message_id = ?
        WHERE job_id = ?
        """,
        (str(lease["lease_id"]), str(message["message_id"]), job_id),
    )

    composition_steps: list[str] = [
        "lease_reactivate",
        "queue_message_restore",
        "job_state_restore",
    ]
    supervisor_id: str | None = None
    if supervisor is not None:
        supervisor_id = str(supervisor["supervisor_id"])
        conn.execute(
            """
            UPDATE process_supervisors
            SET state = 'attached',
                heartbeat_at = ?,
                ended_at = NULL,
                stop_reason = NULL
            WHERE supervisor_id = ?
            """,
            (now, supervisor_id),
        )
        composition_steps.append("supervisor_reattach")
    if pointer is not None:
        conn.execute(
            """
            UPDATE process_supervisor_pointers
            SET state = 'attached', updated_at = ?
            WHERE supervisor_id = ?
            """,
            (now, str(pointer["supervisor_id"])),
        )
        composition_steps.append("supervisor_pointer_reattach")

    after: JsonObject = {
        "job_state": "running",
        "lease_state": "active",
        "message_state": "acked",
    }
    if supervisor_id is not None:
        after["supervisor_state"] = "attached"
    if pointer is not None:
        after["supervisor_pointer_state"] = "attached"

    payload: JsonObject = {
        "operation": "dogfood.surgical_recovery",
        "operator_reason": reason,
        "before": before,
        "after": after,
        "validated_artifact_paths": validated_paths,
        "composition_steps": composition_steps,
        "extend_lease_seconds": extend_lease_seconds,
    }
    if supervisor_id is not None:
        payload["supervisor_id"] = supervisor_id
    insert_event(
        conn,
        run_id=run_id,
        event_type="dogfood.surgical_recovery",
        actor_session_id=session_id,
        job_id=job_id,
        message_id=str(message["message_id"]),
        lease_id=str(lease["lease_id"]),
        payload=payload,
    )
    return {
        "ok": True,
        "status": "recovered",
        "operation": "dogfood.surgical_recovery",
        "run_id": run_id,
        "job_id": job_id,
        "session_id": session_id,
        "lease_id": str(lease["lease_id"]),
        "message_id": str(message["message_id"]),
        "supervisor_id": supervisor_id,
        "new_expires_at": new_expires_at,
        "validated_artifact_paths": validated_paths,
        "composition_steps": composition_steps,
        "reason": reason,
    }


def _validate_recoverable_shape(
    conn: sqlite3.Connection, *, job: sqlite3.Row, run: sqlite3.Row
) -> JsonObject:
    job_id = str(job["job_id"])
    run_id = str(job["run_id"])
    if not _job_is_repo_write(job):
        return _failure(
            "not_repo_write",
            "surgical recovery is only for repo-write jobs",
            details={"job_id": job_id},
        )
    if str(job["state"]) in _TERMINAL_JOB_STATES:
        return _failure(
            "terminal_job",
            f"job state {job['state']!r} is terminal and cannot be recovered",
            details={"job_state": str(job["state"])},
        )
    if str(job["state"]) != "stale_lease":
        return _failure(
            "job_not_stale",
            "surgical recovery requires a stale_lease job",
            details={"job_state": str(job["state"])},
        )

    active = conn.execute(
        """
        SELECT lease_id FROM leases
        WHERE resource_type = 'job' AND resource_id = ? AND state = 'active'
        LIMIT 1
        """,
        (job_id,),
    ).fetchone()
    if active is not None:
        return _failure(
            "concurrent_active_lease",
            "job already has an active lease",
            details={"lease_id": str(active["lease_id"])},
        )

    lease = conn.execute(
        """
        SELECT * FROM leases
        WHERE run_id = ? AND resource_type = 'job' AND resource_id = ?
          AND state = 'expired'
        ORDER BY released_at DESC, expires_at DESC, acquired_at DESC
        LIMIT 1
        """,
        (run_id, job_id),
    ).fetchone()
    if lease is None:
        return _failure("expired_lease_missing", "job has no expired lease to recover")
    session_id = str(lease["owner_session_id"])

    message = _message_for_job_and_lease(conn, job=job, lease=lease)
    if message is None:
        return _failure(
            "queue_message_missing",
            "job has no queue message tied to the expired lease",
            details={"lease_id": str(lease["lease_id"])},
        )
    if str(message["state"]) not in {"blocked", "acked", "claimed"}:
        return _failure(
            "queue_message_not_recoverable",
            "queue message is not in a recoverable state",
            details={"message_state": str(message["state"])},
        )

    validated = _validate_required_artifact_files(job=job, repo_root=Path(str(run["repo_root"])))
    if validated.get("ok") is not True:
        return validated

    active_supervisors = conn.execute(
        f"""
        SELECT * FROM process_supervisors
        WHERE session_id = ? AND state IN ({_placeholders(_ACTIVE_SUPERVISOR_STATES)})
        ORDER BY started_at DESC
        """,
        (session_id, *_ACTIVE_SUPERVISOR_STATES),
    ).fetchall()
    if active_supervisors:
        return _failure(
            "concurrent_supervisor",
            "session already has an active supervisor",
            details={
                "supervisor_ids": [
                    str(row["supervisor_id"]) for row in active_supervisors
                ]
            },
        )

    supervisor = conn.execute(
        """
        SELECT * FROM process_supervisors
        WHERE session_id = ? AND run_id = ? AND state = 'lost'
        ORDER BY ended_at DESC, started_at DESC
        LIMIT 1
        """,
        (session_id, run_id),
    ).fetchone()
    if supervisor is not None:
        supervisor_check = _validate_supervisor_identity(supervisor)
        if supervisor_check.get("ok") is not True:
            return supervisor_check

    pointer = conn.execute(
        """
        SELECT * FROM process_supervisor_pointers
        WHERE session_id = ? AND run_id = ? AND state = 'lost'
        ORDER BY updated_at DESC
        LIMIT 1
        """,
        (session_id, run_id),
    ).fetchone()
    if pointer is not None:
        pointer_check = _validate_pointer_identity(pointer)
        if pointer_check.get("ok") is not True:
            return pointer_check

    return {
        "ok": True,
        "lease": lease,
        "message": message,
        "supervisor": supervisor,
        "pointer": pointer,
        "validated_artifact_paths": validated["validated_artifact_paths"],
    }


def _expected_artifact_items(job: sqlite3.Row) -> list[dict[str, Any]]:
    try:
        loaded = json.loads(str(job["expected_artifacts_json"]))
    except json.JSONDecodeError:
        return []
    if not isinstance(loaded, list):
        return []
    return [item for item in loaded if isinstance(item, dict)]


def _message_for_job_and_lease(
    conn: sqlite3.Connection, *, job: sqlite3.Row, lease: sqlite3.Row
) -> sqlite3.Row | None:
    message_id = job["current_message_id"]
    if message_id is not None:
        row = conn.execute(
            "SELECT * FROM queue_messages WHERE message_id = ? AND job_id = ?",
            (str(message_id), str(job["job_id"])),
        ).fetchone()
        if row is not None:
            return cast(sqlite3.Row, row)
    row = conn.execute(
        """
        SELECT qm.*
        FROM work_packets wp
        JOIN queue_messages qm ON qm.message_id = wp.message_id
        WHERE wp.lease_id = ? AND wp.job_id = ?
        ORDER BY wp.created_at DESC
        LIMIT 1
        """,
        (str(lease["lease_id"]), str(job["job_id"])),
    ).fetchone()
    return cast(sqlite3.Row | None, row)


def _validate_required_artifact_files(*, job: sqlite3.Row, repo_root: Path) -> JsonObject:
    try:
        expected = json.loads(str(job["expected_artifacts_json"]))
    except json.JSONDecodeError:
        return _failure("expected_artifacts_invalid", "expected artifacts JSON is invalid")
    if not isinstance(expected, list):
        return _failure("expected_artifacts_invalid", "expected artifacts must be a list")

    try:
        write_scope = json.loads(str(job["write_scope_json"]))
    except json.JSONDecodeError:
        write_scope = {}
    allowed_paths = _allowed_paths(write_scope)

    missing: list[str] = []
    invalid_paths: list[str] = []
    outside_scope: list[str] = []
    validated: list[str] = []
    for item in expected:
        if not isinstance(item, dict) or item.get("required") is not True:
            continue
        raw_path = item.get("path")
        if not isinstance(raw_path, str) or not _safe_repo_relative_path(raw_path):
            invalid_paths.append(str(raw_path))
            continue
        if allowed_paths and not _path_allowed(raw_path, allowed_paths):
            outside_scope.append(raw_path)
            continue
        full_path = repo_root / raw_path
        if not full_path.is_file():
            missing.append(raw_path)
            continue
        validated.append(raw_path)

    if invalid_paths:
        return _failure(
            "invalid_expected_artifact_path",
            "required expected artifact path is not repo-relative",
            details={"invalid_paths": invalid_paths},
        )
    if outside_scope:
        return _failure(
            "expected_artifact_outside_write_scope",
            "required expected artifact path is outside the job write scope",
            details={"outside_scope": outside_scope},
        )
    if missing:
        return _failure(
            "missing_expected_artifacts",
            "required expected artifacts are missing on disk",
            details={"missing_paths": missing},
        )
    return {"ok": True, "validated_artifact_paths": validated}


def _validate_supervisor_identity(supervisor: sqlite3.Row) -> JsonObject:
    pid_value = supervisor["pid"]
    if pid_value is None:
        return _failure(
            "supervisor_pid_missing",
            "lost supervisor cannot be reattached without a pid",
            details={"supervisor_id": str(supervisor["supervisor_id"])},
        )
    pid = int(pid_value)
    if not _pid_alive(pid):
        return _failure(
            "supervisor_pid_gone",
            "lost supervisor pid is not alive",
            details={"supervisor_id": str(supervisor["supervisor_id"]), "pid": pid},
        )
    expected_start = supervisor["pid_start_time"] if "pid_start_time" in supervisor.keys() else None
    current_start = process_start_time(pid)
    if expected_start and current_start != str(expected_start):
        return _failure(
            "supervisor_pid_identity_mismatch",
            "lost supervisor pid does not match its recorded process identity",
            details={
                "supervisor_id": str(supervisor["supervisor_id"]),
                "pid": pid,
                "expected_pid_start_time": str(expected_start),
                "current_pid_start_time": current_start,
            },
        )
    return {"ok": True}


def _validate_pointer_identity(pointer: sqlite3.Row) -> JsonObject:
    pid_value = pointer["pid"]
    if pid_value is None:
        return _failure(
            "supervisor_pointer_pid_missing",
            "lost supervisor pointer cannot be reattached without a pid",
            details={"supervisor_id": str(pointer["supervisor_id"])},
        )
    pid = int(pid_value)
    if not _pid_alive(pid):
        return _failure(
            "supervisor_pointer_pid_gone",
            "lost supervisor pointer pid is not alive",
            details={"supervisor_id": str(pointer["supervisor_id"]), "pid": pid},
        )
    expected_start = pointer["pid_start_time"]
    current_start = process_start_time(pid)
    if expected_start and current_start != str(expected_start):
        return _failure(
            "supervisor_pointer_pid_identity_mismatch",
            "lost supervisor pointer pid does not match its recorded process identity",
            details={
                "supervisor_id": str(pointer["supervisor_id"]),
                "pid": pid,
                "expected_pid_start_time": str(expected_start),
                "current_pid_start_time": current_start,
            },
        )
    return {"ok": True}


def _allowed_paths(write_scope: Any) -> list[str]:
    if not isinstance(write_scope, dict):
        return []
    raw = write_scope.get("allowed_paths")
    if not isinstance(raw, list):
        return []
    return [item for item in raw if isinstance(item, str) and item]


def _job_is_repo_write(job: sqlite3.Row) -> bool:
    try:
        write_scope = json.loads(str(job["write_scope_json"]))
    except json.JSONDecodeError:
        return False
    if not isinstance(write_scope, dict):
        return False
    return bool(write_scope.get("repo_write")) or write_scope.get("mode") == "repo_write"


def _path_allowed(path: str, allowed_paths: list[str]) -> bool:
    for allowed in allowed_paths:
        if allowed.endswith("/"):
            if path.startswith(allowed):
                return True
        elif path == allowed:
            return True
    return False


def _safe_repo_relative_path(path: str) -> bool:
    pure = PurePosixPath(path)
    if pure.is_absolute():
        return False
    if any(part in {"", ".", ".."} for part in pure.parts):
        return False
    if pure.parts and pure.parts[0] == ".striatum":
        return False
    return True


def _pid_alive(pid: int) -> bool:
    if pid <= 0:
        return False
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    return True


def _placeholders(values: tuple[str, ...]) -> str:
    return ",".join(["?"] * len(values))


def _failure(code: str, message: str, *, details: JsonObject | None = None) -> JsonObject:
    error: JsonObject = {"code": code, "message": message}
    if details:
        error["details"] = details
    return {"ok": False, "status": "refused", "error": error}
