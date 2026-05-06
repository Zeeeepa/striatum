"""SQLite helpers and state transitions for the V1 MVP."""

from __future__ import annotations

import hashlib
import json
import sqlite3
import uuid
from contextlib import contextmanager
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any, Iterator, cast

from agent_runner.errors import (
    AgentRunnerError,
    ArtifactError,
    BranchConfirmationError,
    InvalidTransitionError,
    LeaseError,
    NotFoundError,
)
from agent_runner.schema import SCHEMA_SQL

# JSON columns are intentionally untyped at the SQLite boundary.
JsonObject = dict[str, Any]

STATE_DIR = ".agent_runner"
DB_NAME = "state.sqlite3"


def utc_now() -> str:
    """Return an RFC3339 UTC timestamp."""
    return datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def new_id(prefix: str) -> str:
    """Return an opaque stable-enough local id."""
    return f"{prefix}_{uuid.uuid4().hex}"


def json_dumps(value: object) -> str:
    """Serialize JSON deterministically for hashing and storage."""
    return json.dumps(value, sort_keys=True, separators=(",", ":"))


def json_loads(value: str) -> JsonObject:
    """Load a JSON object from a SQLite text column."""
    loaded = json.loads(value)
    if not isinstance(loaded, dict):
        raise InvalidTransitionError("stored JSON value is not an object")
    return cast(JsonObject, loaded)


def sha256_bytes(payload: bytes) -> str:
    """Return a hex SHA-256 digest."""
    return hashlib.sha256(payload).hexdigest()


def state_dir(repo: Path) -> Path:
    """Return the repo-local state directory."""
    return repo / STATE_DIR


def db_path(repo: Path) -> Path:
    """Return the repo-local SQLite database path."""
    return state_dir(repo) / DB_NAME


def connect(repo: Path) -> sqlite3.Connection:
    """Connect to the repo-local SQLite database."""
    conn = sqlite3.connect(db_path(repo))
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    conn.execute("PRAGMA journal_mode = WAL")
    conn.execute("PRAGMA busy_timeout = 5000")
    return conn


@contextmanager
def transaction(conn: sqlite3.Connection) -> Iterator[sqlite3.Connection]:
    """Run a short write transaction."""
    conn.execute("BEGIN IMMEDIATE")
    try:
        yield conn
    except Exception:
        conn.rollback()
        raise
    else:
        conn.commit()


def init_repo(repo: Path) -> None:
    """Create state storage and initialize schema."""
    state_dir(repo).mkdir(parents=True, exist_ok=True)
    ignore_path = repo / ".gitignore"
    existing = ignore_path.read_text(encoding="utf-8") if ignore_path.exists() else ""
    if ".agent_runner/" not in existing.splitlines():
        prefix = "" if existing == "" or existing.endswith("\n") else "\n"
        ignore_path.write_text(f"{existing}{prefix}.agent_runner/\n", encoding="utf-8")
    with connect(repo) as conn:
        conn.executescript(SCHEMA_SQL)


def ensure_initialized(repo: Path) -> None:
    """Raise if the repo has not been initialized."""
    if not db_path(repo).exists():
        raise AgentRunnerError("agent_runner state is not initialized; run agent_runner init", exit_code=3)


def insert_event(
    conn: sqlite3.Connection,
    *,
    run_id: str | None,
    event_type: str,
    actor_session_id: str | None = None,
    job_id: str | None = None,
    message_id: str | None = None,
    artifact_id: str | None = None,
    lease_id: str | None = None,
    payload: JsonObject | None = None,
) -> int:
    """Insert an append-only event row."""
    cursor = conn.execute(
        """
        INSERT INTO events (
          run_id, event_type, actor_session_id, job_id, message_id,
          artifact_id, lease_id, payload_json, created_at
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        """,
        (
            run_id,
            event_type,
            actor_session_id,
            job_id,
            message_id,
            artifact_id,
            lease_id,
            json_dumps(payload or {}),
            utc_now(),
        ),
    )
    return int(cursor.lastrowid)


def row_by_id(conn: sqlite3.Connection, table: str, column: str, value: str) -> sqlite3.Row:
    """Fetch a row or raise NotFoundError."""
    cursor = conn.execute(f"SELECT * FROM {table} WHERE {column} = ?", (value,))
    row = cursor.fetchone()
    if row is None:
        raise NotFoundError(f"could not find {table} row for {column}={value!r}")
    return cast(sqlite3.Row, row)


def repo_relative_path(repo: Path, path_text: str) -> Path:
    """Resolve a repo-relative path and reject escapes."""
    return _repo_relative_path(repo, path_text, allow_state=False)


def _repo_relative_path(repo: Path, path_text: str, *, allow_state: bool) -> Path:
    """Resolve a repo-relative path with optional state-dir allowance."""
    path = Path(path_text)
    if path.is_absolute():
        raise ArtifactError("artifact path must be repo-relative")
    resolved = (repo / path).resolve()
    repo_resolved = repo.resolve()
    try:
        resolved.relative_to(repo_resolved)
    except ValueError as exc:
        raise ArtifactError("artifact path must stay inside the repository") from exc
    if not allow_state and (
        resolved == repo_resolved / STATE_DIR or (repo_resolved / STATE_DIR) in resolved.parents
    ):
        raise ArtifactError("artifact path cannot be under .agent_runner")
    return resolved


def path_allowed(repo: Path, path_text: str, write_scope: JsonObject) -> bool:
    """Return whether a repo-relative path is allowed by the job write scope."""
    resolved = repo_relative_path(repo, path_text)
    allowed = write_scope.get("allowed_paths", [])
    forbidden = write_scope.get("forbidden_paths", [STATE_DIR])
    if not isinstance(allowed, list) or not isinstance(forbidden, list):
        return False
    for item in forbidden:
        if not isinstance(item, str):
            continue
        denied = _repo_relative_path(repo, item, allow_state=True).resolve()
        if resolved == denied or denied in resolved.parents:
            return False
    for item in allowed:
        if not isinstance(item, str):
            continue
        base = _repo_relative_path(repo, item, allow_state=True).resolve()
        if resolved == base or base in resolved.parents:
            return True
    return False


def is_repo_write(job: sqlite3.Row) -> bool:
    """Return whether a job can write non-review artifacts."""
    scope = json_loads(str(job["write_scope_json"]))
    return scope.get("repo_write") is True or scope.get("mode") == "repo_write"


def active_lease_for(
    conn: sqlite3.Connection,
    *,
    lease_id: str,
    session_id: str,
    job_id: str | None = None,
) -> sqlite3.Row:
    """Fetch an active lease and validate ownership."""
    lease = row_by_id(conn, "leases", "lease_id", lease_id)
    if lease["state"] != "active":
        raise LeaseError("lease is not active")
    if lease["owner_session_id"] != session_id:
        raise LeaseError("lease is owned by another session")
    if job_id is not None and lease["resource_id"] != job_id:
        raise LeaseError("lease does not belong to the job")
    if str(lease["expires_at"]) < utc_now():
        raise LeaseError("lease is expired")
    return lease


def expire_leases(conn: sqlite3.Connection, *, run_id: str) -> None:
    """Expire stale leases lazily during CLI mutations."""
    now = utc_now()
    rows = conn.execute(
        "SELECT * FROM leases WHERE run_id = ? AND state = 'active' AND expires_at < ?",
        (run_id, now),
    ).fetchall()
    for lease in rows:
        job = row_by_id(conn, "jobs", "job_id", str(lease["resource_id"]))
        message_id = job["current_message_id"]
        if is_repo_write(job):
            job_state = "stale_lease"
            message_state = "blocked"
        else:
            job_state = "queued"
            message_state = "pending"
        conn.execute(
            """
            UPDATE leases
            SET state = 'expired', released_at = ?, release_reason = 'expired'
            WHERE lease_id = ?
            """,
            (now, lease["lease_id"]),
        )
        conn.execute(
            """
            UPDATE jobs
            SET state = ?, current_lease_id = NULL
            WHERE job_id = ?
            """,
            (job_state, job["job_id"]),
        )
        if message_id is not None:
            conn.execute(
                """
                UPDATE queue_messages
                SET state = ?, current_lease_id = NULL, updated_at = ?
                WHERE message_id = ?
                """,
                (message_state, now, message_id),
            )
        insert_event(
            conn,
            run_id=run_id,
            event_type="lease.expired",
            job_id=str(job["job_id"]),
            message_id=message_id,
            lease_id=str(lease["lease_id"]),
            payload={"job_state": job_state, "message_state": message_state},
        )


def enqueue_job(conn: sqlite3.Connection, *, job_id: str) -> str:
    """Enqueue a work message for a queued job."""
    job = row_by_id(conn, "jobs", "job_id", job_id)
    if job["state"] not in ("blocked", "queued"):
        raise InvalidTransitionError("job is not enqueueable")
    now = utc_now()
    message_id = new_id("msg")
    lane_selector = json_loads(str(job["lane_selector_json"]))
    target_lane = lane_selector.get("lane_id")
    if target_lane is not None and not isinstance(target_lane, str):
        target_lane = None
    conn.execute(
        """
        INSERT INTO queue_messages (
          message_id, run_id, job_id, kind, state, priority, target_role_id,
          target_lane_id, payload_json, claim_count, max_claims, created_at, updated_at
        )
        VALUES (?, ?, ?, 'work', 'pending', 0, ?, ?, '{}', 0, ?, ?, ?)
        """,
        (
            message_id,
            job["run_id"],
            job_id,
            job["role_id"],
            target_lane,
            job["max_attempts"],
            now,
            now,
        ),
    )
    conn.execute(
        """
        UPDATE jobs
        SET state = 'queued', ready_at = ?, current_message_id = ?
        WHERE job_id = ?
        """,
        (now, message_id, job_id),
    )
    insert_event(
        conn,
        run_id=str(job["run_id"]),
        event_type="queue.message_enqueued",
        job_id=job_id,
        message_id=message_id,
        payload={"workflow_job_id": job["workflow_job_id"]},
    )
    return message_id


def maybe_enqueue_downstream(conn: sqlite3.Connection, *, completed_job_id: str) -> None:
    """Enqueue jobs whose dependencies are all completed."""
    dependents = conn.execute(
        "SELECT job_id FROM job_dependencies WHERE depends_on_job_id = ?",
        (completed_job_id,),
    ).fetchall()
    for dependent in dependents:
        job_id = str(dependent["job_id"])
        job = row_by_id(conn, "jobs", "job_id", job_id)
        if job["state"] != "blocked":
            continue
        incomplete = conn.execute(
            """
            SELECT 1
            FROM job_dependencies dep
            JOIN jobs upstream ON upstream.job_id = dep.depends_on_job_id
            WHERE dep.job_id = ? AND upstream.state != 'completed'
            LIMIT 1
            """,
            (job_id,),
        ).fetchone()
        if incomplete is None:
            enqueue_job(conn, job_id=job_id)


def maybe_complete_run(conn: sqlite3.Connection, *, run_id: str) -> None:
    """Mark a run completed when all jobs are terminally completed."""
    remaining = conn.execute(
        """
        SELECT 1 FROM jobs
        WHERE run_id = ? AND state NOT IN ('completed','skipped','canceled')
        LIMIT 1
        """,
        (run_id,),
    ).fetchone()
    if remaining is not None:
        return
    now = utc_now()
    conn.execute(
        "UPDATE runs SET state = 'completed', completed_at = ? WHERE run_id = ?",
        (now, run_id),
    )
    insert_event(conn, run_id=run_id, event_type="run.completed")


def claim_next(conn: sqlite3.Connection, *, repo: Path, session_id: str, lease_seconds: int) -> JsonObject:
    """Claim the next eligible work item for a registered session."""
    with transaction(conn):
        session = row_by_id(conn, "sessions", "session_id", session_id)
        run = row_by_id(conn, "runs", "run_id", str(session["run_id"]))
        expire_leases(conn, run_id=str(run["run_id"]))
        if run["state"] in ("needs_branch_confirmation", "ready"):
            raise BranchConfirmationError("branch confirmation and run start are required before claims")
        if run["state"] != "running":
            return {"status": "no_work"}
        messages = conn.execute(
            """
            SELECT qm.*
            FROM queue_messages qm
            JOIN jobs j ON j.job_id = qm.job_id
            WHERE qm.run_id = ?
              AND qm.kind = 'work'
              AND qm.state = 'pending'
              AND qm.target_role_id = ?
              AND (qm.target_lane_id IS NULL OR qm.target_lane_id = ?)
            ORDER BY qm.priority DESC, qm.created_at ASC
            """,
            (run["run_id"], session["role_id"], session["lane_id"]),
        ).fetchall()
        chosen: sqlite3.Row | None = None
        for message in messages:
            job = row_by_id(conn, "jobs", "job_id", str(message["job_id"]))
            if job["fresh_session_required"] == 1:
                prior = conn.execute(
                    "SELECT 1 FROM work_packets WHERE run_id = ? AND session_id = ? LIMIT 1",
                    (run["run_id"], session_id),
                ).fetchone()
                if prior is not None:
                    continue
            chosen = message
            break
        if chosen is None:
            return {"status": "no_work"}
        job = row_by_id(conn, "jobs", "job_id", str(chosen["job_id"]))
        now = utc_now()
        lease_id = new_id("lease")
        packet_id = new_id("wp")
        expires_at = (
            datetime.now(UTC) + timedelta(seconds=lease_seconds)
        ).replace(microsecond=0).isoformat().replace("+00:00", "Z")
        conn.execute(
            """
            INSERT INTO leases (
              lease_id, run_id, resource_type, resource_id, owner_session_id,
              state, acquired_at, expires_at, last_heartbeat_at
            )
            VALUES (?, ?, 'job', ?, ?, 'active', ?, ?, ?)
            """,
            (lease_id, run["run_id"], job["job_id"], session_id, now, expires_at, now),
        )
        conn.execute(
            """
            UPDATE queue_messages
            SET state = 'claimed', claimed_at = ?, updated_at = ?,
                current_lease_id = ?, claim_count = claim_count + 1
            WHERE message_id = ?
            """,
            (now, now, lease_id, chosen["message_id"]),
        )
        conn.execute(
            """
            UPDATE jobs
            SET state = 'claimed', current_lease_id = ?, started_at = ?
            WHERE job_id = ?
            """,
            (lease_id, now, job["job_id"]),
        )
        packet = build_packet(
            conn=conn,
            repo=repo,
            run=run,
            session=session,
            job=job,
            message_id=str(chosen["message_id"]),
            lease_id=lease_id,
            lease_expires_at=expires_at,
            packet_id=packet_id,
        )
        packet_json = json_dumps(packet)
        conn.execute(
            """
            INSERT INTO work_packets (
              packet_id, run_id, job_id, message_id, lease_id, session_id,
              packet_json, packet_sha256, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                packet_id,
                run["run_id"],
                job["job_id"],
                chosen["message_id"],
                lease_id,
                session_id,
                packet_json,
                sha256_bytes(packet_json.encode("utf-8")),
                now,
            ),
        )
        insert_event(
            conn,
            run_id=str(run["run_id"]),
            event_type="queue.claimed",
            actor_session_id=session_id,
            job_id=str(job["job_id"]),
            message_id=str(chosen["message_id"]),
            lease_id=lease_id,
        )
        return {"status": "claimed", "packet": packet}


def build_packet(
    *,
    conn: sqlite3.Connection,
    repo: Path,
    run: sqlite3.Row,
    session: sqlite3.Row,
    job: sqlite3.Row,
    message_id: str,
    lease_id: str,
    lease_expires_at: str,
    packet_id: str,
) -> JsonObject:
    """Build a structured work packet from stored workflow state."""
    snapshot = row_by_id(
        conn,
        "workflow_snapshots",
        "workflow_snapshot_id",
        str(run["workflow_snapshot_id"]),
    )
    workflow = json_loads(str(snapshot["workflow_json"]))
    roles = cast(JsonObject, workflow.get("roles", {}))
    role_def = roles.get(str(job["role_id"]), {})
    context_docs = workflow.get("context_docs", [])
    write_scope = json_loads(str(job["write_scope_json"]))
    expected_artifacts = json.loads(str(job["expected_artifacts_json"]))
    lane = json_loads(str(job["lane_selector_json"])).get("lane_id")
    return {
        "packet_version": "agent-runner.work-packet.v1",
        "packet_id": packet_id,
        "run": {
            "run_id": run["run_id"],
            "workflow_id": workflow.get("workflow_id"),
            "repo_root": str(repo),
            "branch": {"name": run["branch_name"], "confirmed": run["branch_confirmed_at"] is not None},
        },
        "session": {
            "session_id": session["session_id"],
            "slug": session["slug"],
            "role_id": session["role_id"],
            "lane_id": session["lane_id"],
            "capabilities": json.loads(str(session["capabilities_json"])),
        },
        "lease": {
            "lease_id": lease_id,
            "message_id": message_id,
            "expires_at": lease_expires_at,
            "heartbeat_after_seconds": 300,
        },
        "job": {
            "job_id": job["job_id"],
            "workflow_job_id": job["workflow_job_id"],
            "attempt": job["attempt"],
            "type": job["job_type"],
            "title": job["title"],
            "objective": json_loads(str(job["capability_requirements_json"])).get("objective"),
            "fresh_session_required": job["fresh_session_required"] == 1,
        },
        "role": {
            "role_id": job["role_id"],
            "definition_path": role_def.get("definition_path") if isinstance(role_def, dict) else None,
            "inline_summary": role_def.get("summary") if isinstance(role_def, dict) else None,
        },
        "context": {"docs": context_docs, "content_mode": "references"},
        "task_prompt": json_loads(str(job["capability_requirements_json"])).get("task_prompt", {}),
        "inputs": json_loads(str(job["capability_requirements_json"])).get("inputs", []),
        "write_scope": write_scope,
        "expected_artifacts": expected_artifacts,
        "commands": {
            "ack": f"agent_runner ack --session-id {session['session_id']} --message-id {message_id} --lease-id {lease_id}",
            "heartbeat": f"agent_runner heartbeat --session-id {session['session_id']} --lease-id {lease_id}",
            "publish_artifact": f"agent_runner publish-artifact --session-id {session['session_id']} --job-id {job['job_id']} --lease-id {lease_id}",
            "block": f"agent_runner block --session-id {session['session_id']} --job-id {job['job_id']} --lease-id {lease_id}",
            "verdict": f"agent_runner verdict --session-id {session['session_id']} --job-id {job['job_id']} --lease-id {lease_id}",
            "complete": f"agent_runner complete --session-id {session['session_id']} --job-id {job['job_id']} --lease-id {lease_id}",
        },
        "artifact_policy": {"publish_transcripts": False, "curated_artifacts_only": True},
    }


def complete_job(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    summary: str | None,
) -> JsonObject:
    """Complete a running job after required artifacts are present."""
    with transaction(conn):
        job = row_by_id(conn, "jobs", "job_id", job_id)
        active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=job_id)
        if job["state"] != "running":
            raise InvalidTransitionError("job must be running before completion")
        verify_required_artifacts(conn, job_id=job_id)
        now = utc_now()
        message_id = job["current_message_id"]
        conn.execute(
            "UPDATE jobs SET state = 'completed', completed_at = ?, current_lease_id = NULL WHERE job_id = ?",
            (now, job_id),
        )
        if message_id is not None:
            conn.execute(
                """
                UPDATE queue_messages
                SET state = 'completed', completed_at = ?, updated_at = ?,
                    current_lease_id = NULL
                WHERE message_id = ?
                """,
                (now, now, message_id),
            )
        conn.execute(
            """
            UPDATE leases
            SET state = 'released', released_at = ?, release_reason = 'completed'
            WHERE lease_id = ?
            """,
            (now, lease_id),
        )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="job.completed",
            actor_session_id=session_id,
            job_id=job_id,
            message_id=message_id,
            lease_id=lease_id,
            payload={"summary": summary},
        )
        maybe_enqueue_downstream(conn, completed_job_id=job_id)
        maybe_complete_run(conn, run_id=str(job["run_id"]))
        return {"status": "completed", "job_id": job_id}


def verify_required_artifacts(conn: sqlite3.Connection, *, job_id: str) -> None:
    """Ensure all required artifacts for a job were published."""
    job = row_by_id(conn, "jobs", "job_id", job_id)
    expected = json.loads(str(job["expected_artifacts_json"]))
    if not isinstance(expected, list):
        raise InvalidTransitionError("expected artifacts must be a list")
    for item in expected:
        if not isinstance(item, dict) or item.get("required") is not True:
            continue
        logical_name = item.get("logical_name")
        found = conn.execute(
            "SELECT 1 FROM artifacts WHERE job_id = ? AND logical_name = ? LIMIT 1",
            (job_id, logical_name),
        ).fetchone()
        if found is None:
            raise InvalidTransitionError(f"required artifact {logical_name!r} is missing")
