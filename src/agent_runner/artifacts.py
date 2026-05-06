"""Artifact publisher for durable repo outputs."""

from __future__ import annotations

import sqlite3
from pathlib import Path

from agent_runner.db import (
    ArtifactError,
    active_lease_for,
    insert_event,
    json_loads,
    new_id,
    path_allowed,
    repo_relative_path,
    sha256_bytes,
    transaction,
    utc_now,
)


def publish_artifact(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    session_id: str,
    job_id: str,
    lease_id: str,
    kind: str,
    logical_name: str,
    path_text: str,
) -> dict[str, object]:
    """Record an artifact reference after validating write scope."""
    with transaction(conn):
        job = conn.execute("SELECT * FROM jobs WHERE job_id = ?", (job_id,)).fetchone()
        if job is None:
            raise ArtifactError("job does not exist")
        active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=job_id)
        if kind == "transcript":
            raise ArtifactError("transcript artifacts are not allowed by default")
        write_scope = json_loads(str(job["write_scope_json"]))
        if not path_allowed(repo, path_text, write_scope):
            raise ArtifactError("artifact path is outside the job write scope")
        path = repo_relative_path(repo, path_text)
        if not path.exists() or not path.is_file():
            raise ArtifactError("artifact file does not exist")
        payload = path.read_bytes()
        digest = sha256_bytes(payload)
        existing = conn.execute(
            """
            SELECT * FROM artifacts
            WHERE run_id = ? AND job_id = ? AND logical_name = ?
            """,
            (job["run_id"], job_id, logical_name),
        ).fetchone()
        if existing is not None:
            if existing["content_sha256"] == digest and existing["repo_path"] == path_text:
                return {"status": "already_published", "artifact_id": existing["artifact_id"]}
            raise ArtifactError("artifact logical name already exists with different content")
        artifact_id = new_id("art")
        now = utc_now()
        conn.execute(
            """
            INSERT INTO artifacts (
              artifact_id, run_id, job_id, session_id, logical_name,
              artifact_kind, repo_path, content_sha256, size_bytes,
              publish_mode, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'create', ?)
            """,
            (
                artifact_id,
                job["run_id"],
                job_id,
                session_id,
                logical_name,
                kind,
                path_text,
                digest,
                len(payload),
                now,
            ),
        )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="artifact.published",
            actor_session_id=session_id,
            job_id=job_id,
            artifact_id=artifact_id,
            lease_id=lease_id,
            payload={"logical_name": logical_name, "path": path_text, "sha256": digest},
        )
        return {"status": "published", "artifact_id": artifact_id, "sha256": digest}

