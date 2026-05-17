"""PG handler for ``artifact.show``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_rpc.envelope import RpcError

from ._registry import register_pg_handler
from ._sql import optional_text, parse_json_list, parse_json_object, require_text, row_to_json


@register_pg_handler("artifact.show", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    artifact_id = require_text(params, "artifact_id")
    run_id = optional_text(params, "run_id")
    include_web_context = bool(params.get("include_web_context", False))
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT artifact_id, run_id, job_id, session_id, logical_name,
                   artifact_kind, repo_path, content_sha256, size_bytes,
                   publish_mode, created_at, author_line,
                   attestation_override_rationale
            FROM striatumd.artifacts
            WHERE repository_id = %s AND artifact_id = %s
            """,
            (ctx.repository_id, artifact_id),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("not_found", f"artifact not found: {artifact_id}")
    artifact = row_to_json(row)
    if run_id is not None and artifact.get("run_id") != run_id:
        raise RpcError("not_found", f"artifact not found in run: {artifact_id}")
    if not include_web_context:
        return {"artifact": artifact}
    artifact_run_id = str(artifact["run_id"])
    return {
        "artifact": artifact,
        "run": _run_row(ctx, run_id=artifact_run_id),
        "expected_author_line": _expected_author_line(ctx, artifact=artifact),
        "provenance_trail": _artifact_provenance_trail(
            ctx,
            artifact_id=artifact_id,
            run_id=artifact_run_id,
        ),
    }


def _run_row(ctx: RepoHandlerContext, *, run_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT r.run_id, r.state, r.branch_name
            FROM striatumd.runs r
            WHERE r.repository_id = %s AND r.run_id = %s
            """,
            (ctx.repository_id, run_id),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("not_found", f"run not found: {run_id}")
    return row_to_json(row)


def _expected_author_line(ctx: RepoHandlerContext, *, artifact: Mapping[str, Any]) -> str | None:
    repo_path = artifact.get("repo_path")
    job_id = artifact.get("job_id")
    if not isinstance(repo_path, str) or not repo_path or not isinstance(job_id, str) or not job_id:
        return None
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT j.expected_artifacts_json
            FROM striatumd.jobs j
            WHERE j.repository_id = %s AND j.job_id = %s
            """,
            (ctx.repository_id, job_id),
        )
        job_row = cur.fetchone()
        if job_row is None:
            return None
        declared = parse_json_list(job_row.get("expected_artifacts_json"))
        if not any(isinstance(item, Mapping) and item.get("path") == repo_path for item in declared):
            return None
        cur.execute(
            """
            SELECT wp.packet_json
            FROM striatumd.work_packets wp
            WHERE wp.repository_id = %s AND wp.job_id = %s
            ORDER BY wp.created_at DESC, wp.packet_id DESC
            LIMIT 1
            """,
            (ctx.repository_id, job_id),
        )
        packet_row = cur.fetchone()
    if packet_row is None:
        return None
    packet = parse_json_object(packet_row.get("packet_json"))
    expected_artifacts = parse_json_list(packet.get("expected_artifacts"))
    for item in expected_artifacts:
        if not isinstance(item, Mapping) or item.get("path") != repo_path:
            continue
        author_line = item.get("author_line")
        return author_line if isinstance(author_line, str) and author_line else None
    return None


def _artifact_provenance_trail(
    ctx: RepoHandlerContext,
    *,
    artifact_id: str,
    run_id: str,
) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT e.event_id, e.event_type, e.actor_session_id, e.job_id,
                   e.artifact_id, e.lease_id, e.payload_json, e.created_at
            FROM striatumd.events e
            WHERE e.repository_id = %s
              AND e.run_id = %s
              AND e.event_type IN (
                'recovery.auto_published',
                'provenance.publish_without_process_execution'
              )
              AND (e.artifact_id = %s OR e.payload_json::text LIKE %s)
            ORDER BY e.event_id
            """,
            (ctx.repository_id, run_id, artifact_id, f"%{artifact_id}%"),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    for row in rows:
        row["payload"] = parse_json_object(row.get("payload_json"))
    return rows
