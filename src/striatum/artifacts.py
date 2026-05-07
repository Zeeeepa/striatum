"""Artifact publisher for durable repo outputs."""

from __future__ import annotations

import sqlite3
from pathlib import Path

from striatum.db import (
    active_lease_for,
    insert_event,
    json_loads,
    new_id,
    path_allowed,
    repo_relative_path,
    row_by_id,
    sha256_bytes,
    transaction,
    utc_now,
)
from striatum.errors import ArtifactError
from striatum.identity import artifact_author_identity


MARKDOWN_SUFFIXES = {".md", ".markdown"}


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
        validate_optional_markdown_author_line(
            conn,
            job=job,
            session_id=session_id,
            path=path,
            payload=payload,
        )
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


def validate_optional_markdown_author_line(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
    session_id: str,
    path: Path,
    payload: bytes,
) -> None:
    """Validate Markdown artifact author metadata when the file provides it."""
    if path.suffix.lower() not in MARKDOWN_SUFFIXES:
        return
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ArtifactError("markdown artifact must be UTF-8") from exc
    author_lines = markdown_title_block_author_lines(text)
    if not author_lines:
        return
    expected = expected_author_line(conn, job=job, session_id=session_id)
    for line in author_lines:
        if line.strip() != expected:
            raise ArtifactError("markdown artifact author line must match expected work packet author line")


def markdown_title_block_author_lines(text: str) -> list[str]:
    """Return author metadata lines from YAML front matter or a Markdown title block."""
    lines = text.splitlines()
    if lines and lines[0].strip() == "---":
        front_matter: list[str] = []
        for line in lines[1:]:
            if line.strip() == "---":
                break
            front_matter.append(line)
        return [line for line in front_matter if line.strip().lower().startswith("author:")]

    title_block = lines[:40]
    author_lines: list[str] = []
    for line in title_block:
        if line.startswith("## "):
            break
        stripped = line.strip()
        if stripped.lower().startswith("author:"):
            author_lines.append(line)
    return author_lines


def expected_author_line(conn: sqlite3.Connection, *, job: sqlite3.Row, session_id: str) -> str:
    """Return the exact work-packet author line expected for this job/session."""
    run = row_by_id(conn, "runs", "run_id", str(job["run_id"]))
    snapshot = row_by_id(
        conn,
        "workflow_snapshots",
        "workflow_snapshot_id",
        str(run["workflow_snapshot_id"]),
    )
    session = row_by_id(conn, "sessions", "session_id", session_id)
    lane = json_loads(str(job["lane_selector_json"])).get("lane_id")
    lane_id = lane if isinstance(lane, str) else None
    author = artifact_author_identity(
        json_loads(str(snapshot["workflow_json"])),
        role_id=str(job["role_id"]),
        lane_id=lane_id,
        workflow_job_id=str(job["workflow_job_id"]),
        ordinal=int(session["ordinal"]),
    )
    line = author["line"]
    if line is None:
        raise ArtifactError("expected artifact author line could not be derived")
    return line
