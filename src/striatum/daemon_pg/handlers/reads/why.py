"""PG handler for ``why``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_rpc.envelope import RpcError

from ._read_model import (
    blocker_summaries,
    dashboard_payload,
    events_for,
    jobs_for_run,
    latest_verdict_for_job,
    status_payload,
)
from ._registry import register_pg_handler
from ._sql import require_text, row_to_json


@register_pg_handler("why", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    target_id = _target_id(params)
    return explain(ctx, target_id=target_id)


def explain(ctx: RepoHandlerContext, *, target_id: str) -> dict[str, Any]:
    run = _one(ctx, "runs", "run_id = %s", [target_id])
    if run is not None:
        run_id = str(run["run_id"])
        return {
            "target_type": "run",
            "run": run,
            "jobs": jobs_for_run(ctx, run_id=run_id),
            "open_blockers": blocker_summaries(ctx, run_id=run_id, severity=None),
            "events": events_for(ctx, run_id=run_id),
            "next_actions": status_payload(ctx, run_id=run_id)["next_actions"],
        }

    job = _one(ctx, "jobs", "job_id = %s OR workflow_job_id = %s", [target_id, target_id])
    message = _one(ctx, "queue_messages", "message_id = %s", [target_id])
    if job is not None or message is not None:
        if job is None:
            if message is None:
                raise RpcError("not_found", f"target no longer exists: {target_id}")
            job_id = str(message["job_id"])
            job = _one(ctx, "jobs", "job_id = %s", [job_id])
        else:
            job_id = str(job["job_id"])
        return {
            "target_type": "job" if job is not None and job.get("job_id") == target_id else "message",
            "job": job,
            "message": message,
            "verdict": latest_verdict_for_job(ctx, job_id=job_id),
            "blockers": _all(ctx, "blockers", "job_id = %s", [job_id], "created_at"),
            "downstream_jobs": _downstream_jobs(ctx, job_id=job_id),
            "events": events_for(ctx, job_id=job_id),
        }

    blocker = _one(ctx, "blockers", "blocker_id = %s", [target_id])
    if blocker is not None:
        blocker_job_id = _optional_str(blocker.get("job_id"))
        run_id = str(blocker["run_id"])
        return {
            "target_type": "blocker",
            "blocker": blocker,
            "run": _one(ctx, "runs", "run_id = %s", [run_id]),
            "job": _one(ctx, "jobs", "job_id = %s", [blocker_job_id])
            if blocker_job_id is not None
            else None,
            "session": _one(ctx, "sessions", "session_id = %s", [str(blocker["session_id"])])
            if blocker.get("session_id") is not None
            else None,
            "related_verdict": latest_verdict_for_job(ctx, job_id=blocker_job_id)
            if blocker_job_id is not None
            else None,
            "blocked_downstream_jobs": _downstream_jobs(ctx, job_id=blocker_job_id)
            if blocker_job_id is not None
            else [],
            "next_actions": dashboard_payload(ctx, run_id=run_id)["status"]["next_actions"],
            "events": events_for(ctx, job_id=blocker_job_id)
            if blocker_job_id is not None
            else events_for(ctx, run_id=run_id),
        }

    artifact = _one(ctx, "artifacts", "artifact_id = %s", [target_id])
    if artifact is not None:
        artifact_job_id = _optional_str(artifact.get("job_id"))
        return {
            "target_type": "artifact",
            "artifact": artifact,
            "job": _one(ctx, "jobs", "job_id = %s", [artifact_job_id])
            if artifact_job_id is not None
            else None,
            "verdicts": _all(ctx, "verdicts", "findings_artifact_id = %s", [target_id], "created_at"),
            "events": events_for(ctx, artifact_id=target_id),
        }

    verdict = _one(ctx, "verdicts", "verdict_id = %s", [target_id])
    if verdict is not None:
        artifact_id = _optional_str(verdict.get("findings_artifact_id"))
        return {
            "target_type": "verdict",
            "verdict": verdict,
            "job": _one(ctx, "jobs", "job_id = %s", [str(verdict["job_id"])]),
            "artifact": _one(ctx, "artifacts", "artifact_id = %s", [artifact_id])
            if artifact_id is not None
            else None,
            "blockers": _all(ctx, "blockers", "job_id = %s", [str(verdict["job_id"])], "created_at"),
            "events": events_for(ctx, job_id=str(verdict["job_id"])),
        }

    session = _one(ctx, "sessions", "session_id = %s OR slug = %s", [target_id, target_id])
    if session is not None:
        session_id = str(session["session_id"])
        return {
            "target_type": "session",
            "session": session,
            "jobs": _jobs_for_session(ctx, session_id=session_id),
            "events": events_for(ctx, session_id=session_id),
        }

    process = _one(ctx, "process_executions", "process_id = %s", [target_id])
    if process is not None:
        return {
            "target_type": "process",
            "process": process,
            "job": _one(ctx, "jobs", "job_id = %s", [str(process["job_id"])]),
            "session": _one(ctx, "sessions", "session_id = %s", [str(process["session_id"])]),
            "events": [],
        }

    raise RpcError(
        "not_found",
        "target id is not a known run, job, message, blocker, artifact, verdict, session, or process",
    )


def _target_id(params: Mapping[str, Any]) -> str:
    if "target_id" in params:
        return require_text(params, "target_id")
    if "id" in params:
        return require_text(params, "id")
    raise RpcError("schema_invalid", "target_id must be a non-empty string")


def _one(
    ctx: RepoHandlerContext, table: str, where_sql: str, values: list[Any]
) -> dict[str, Any] | None:
    with ctx.cursor() as cur:
        cur.execute(
            f"SELECT * FROM striatumd.{table} WHERE repository_id = %s AND ({where_sql}) LIMIT 1",
            [ctx.repository_id, *values],
        )
        row = cur.fetchone()
    return row_to_json(row) if row is not None else None


def _all(
    ctx: RepoHandlerContext,
    table: str,
    where_sql: str,
    values: list[Any],
    order_by: str,
) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            f"SELECT * FROM striatumd.{table} WHERE repository_id = %s AND ({where_sql}) ORDER BY {order_by}",
            [ctx.repository_id, *values],
        )
        return [row_to_json(row) for row in cur.fetchall()]


def _downstream_jobs(ctx: RepoHandlerContext, *, job_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT j.job_id, j.workflow_job_id, j.state
            FROM striatumd.job_dependencies d
            JOIN striatumd.jobs j
              ON j.repository_id = d.repository_id
             AND j.job_id = d.job_id
            WHERE d.repository_id = %s AND d.depends_on_job_id = %s
            ORDER BY j.created_at, j.workflow_job_id
            """,
            (ctx.repository_id, job_id),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def _jobs_for_session(ctx: RepoHandlerContext, *, session_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT DISTINCT j.*
            FROM striatumd.jobs j
            LEFT JOIN striatumd.leases l
              ON l.repository_id = j.repository_id
             AND l.resource_id = j.job_id
            LEFT JOIN striatumd.verdicts v
              ON v.repository_id = j.repository_id
             AND v.job_id = j.job_id
            WHERE j.repository_id = %s
              AND (l.owner_session_id = %s OR v.session_id = %s)
            ORDER BY j.created_at, j.workflow_job_id
            """,
            (ctx.repository_id, session_id, session_id),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def _optional_str(value: Any) -> str | None:
    if value is None:
        return None
    text = str(value)
    return text if text else None
