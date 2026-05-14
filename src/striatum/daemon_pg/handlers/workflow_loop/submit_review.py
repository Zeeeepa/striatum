"""PG-backed ``review.submit`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    _json_list,
    active_lease_for,
    publish_artifact_pg,
    transaction,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.errors import InvalidTransitionError

from .record_verdict import _enforce_required_attestation_for_verdict, record_verdict_inline


@register_pg_handler("review.submit", "submit_review")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = str(params["session_id"])
    job_id = str(params["job_id"])
    lease_id = str(params["lease_id"])
    path_text = str(params["path"] if "path" in params else params["path_text"])
    verdict = str(params["verdict"])
    logical_name = str(params.get("logical_name") or "review")
    kind = str(params.get("kind") or "finding")
    rationale = str(params["rationale"]) if params.get("rationale") is not None else None

    with transaction(ctx):
        job = ctx.row_by_id("jobs", "job_id", job_id, for_update=True)
        _prevalidate_submit_review(
            ctx,
            job=job,
            session_id=session_id,
            lease_id=lease_id,
            logical_name=logical_name,
            kind=kind,
            path_text=path_text,
        )
        if job["state"] == "claimed" and job["current_message_id"] is not None:
            _ack_inline(
                ctx,
                session_id=session_id,
                message_id=str(job["current_message_id"]),
                lease_id=lease_id,
            )
        artifact = publish_artifact_inline(
            ctx,
            session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            kind=kind,
            logical_name=logical_name,
            path_text=path_text,
        )
        verdict_result = record_verdict_inline(
            ctx,
            session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            verdict=verdict,
            findings_artifact_id=str(artifact["artifact_id"]),
            rationale=rationale,
        )
        final_job = ctx.row_by_id("jobs", "job_id", job_id)
        run = ctx.row_by_id("runs", "run_id", str(final_job["run_id"]))
        return {
            "artifact": artifact,
            "verdict": verdict_result,
            "job_state": final_job["state"],
            "run_state": run["state"],
            "blocker_id": verdict_result.get("blocker_id"),
            "downstream_jobs": _downstream_jobs(ctx, job_id=job_id),
        }


def publish_artifact_inline(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    kind: str,
    logical_name: str,
    path_text: str,
) -> dict[str, object]:
    return publish_artifact_pg(
        ctx,
        session_id=session_id,
        job_id=job_id,
        lease_id=lease_id,
        kind=kind,
        logical_name=logical_name,
        path_text=path_text,
    )


def _prevalidate_submit_review(
    ctx: RepoHandlerContext,
    *,
    job: Mapping[str, Any],
    session_id: str,
    lease_id: str,
    logical_name: str,
    kind: str,
    path_text: str,
) -> None:
    if job["job_type"] not in {"review", "phase_synthesis"}:
        raise InvalidTransitionError("submit-review is valid only for verdict-capable jobs")
    if job["state"] not in {"claimed", "running"}:
        raise InvalidTransitionError("verdict-capable job must be claimed or running before submit-review")
    if job["state"] == "claimed" and job["current_message_id"] is None:
        raise InvalidTransitionError("claimed verdict-capable job is missing its current message")
    active_lease_for(ctx, lease_id=lease_id, session_id=session_id, job_id=str(job["job_id"]))
    _enforce_required_attestation_for_verdict(ctx, job=job, session_id=session_id)
    expected = _json_list(job["expected_artifacts_json"])
    for item in expected:
        if not isinstance(item, dict) or item.get("required") is not True:
            continue
        expected_logical_name = item.get("logical_name")
        expected_kind = item.get("kind")
        expected_path = item.get("path")
        if (expected_logical_name, expected_kind, expected_path) == (logical_name, kind, path_text):
            continue
        with ctx.cursor() as cur:
            cur.execute(
                """
                SELECT 1
                FROM striatumd.artifacts
                WHERE repository_id = %s AND job_id = %s AND logical_name = %s
                  AND artifact_kind = %s AND repo_path = %s
                LIMIT 1
                """,
                (
                    ctx.repository_id,
                    job["job_id"],
                    expected_logical_name,
                    expected_kind,
                    expected_path,
                ),
            )
            found = cur.fetchone()
        if found is None:
            raise InvalidTransitionError(
                "required artifact would still be missing after submit-review: "
                f"logical_name={expected_logical_name!r}, kind={expected_kind!r}, path={expected_path!r}"
            )


def _ack_inline(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    message_id: str,
    lease_id: str,
) -> None:
    message = ctx.row_by_id("queue_messages", "message_id", message_id, for_update=True)
    job = ctx.row_by_id("jobs", "job_id", str(message["job_id"]), for_update=True)
    active_lease_for(ctx, lease_id=lease_id, session_id=session_id, job_id=str(job["job_id"]))
    if message["state"] == "acked":
        return
    if message["state"] != "claimed" or job["state"] != "claimed":
        raise InvalidTransitionError("work must be claimed before ack")
    now = ctx.now()
    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.queue_messages
            SET state = 'acked', acked_at = %s, updated_at = %s
            WHERE repository_id = %s AND message_id = %s
            """,
            (now, now, ctx.repository_id, message_id),
        )
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET state = 'running', started_at = %s
            WHERE repository_id = %s AND job_id = %s
            """,
            (now, ctx.repository_id, job["job_id"]),
        )
    ctx.append_event(
        run_id=str(job["run_id"]),
        event_type="queue.acked",
        actor_session_id=session_id,
        job_id=str(job["job_id"]),
        message_id=message_id,
        lease_id=lease_id,
    )


def _downstream_jobs(ctx: RepoHandlerContext, *, job_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT j.job_id, j.workflow_job_id, j.state
            FROM striatumd.job_dependencies dep
            JOIN striatumd.jobs j
              ON j.repository_id = dep.repository_id
             AND j.job_id = dep.job_id
            WHERE dep.repository_id = %s AND dep.depends_on_job_id = %s
            ORDER BY j.created_at, j.job_id
            """,
            (ctx.repository_id, job_id),
        )
        return [dict(row) for row in cur.fetchall()]


__all__ = [
    "handle",
    "publish_artifact_inline",
]
