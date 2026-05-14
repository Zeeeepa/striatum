"""PG-backed ``review.override`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    after_timestamp,
    maybe_complete_run,
    maybe_enqueue_downstream,
    resolve_review_posture,
    transaction,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.errors import InvalidTransitionError


@register_pg_handler("review.override")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = str(params["session_id"])
    job_id = str(params["job_id"])
    verdict = str(params["verdict"])
    rationale = str(params["rationale"])
    findings_artifact_id = (
        str(params["findings_artifact_id"])
        if params.get("findings_artifact_id") is not None
        else None
    )
    if verdict not in {"accept", "accept_with_findings"}:
        raise InvalidTransitionError("override verdict must be 'accept' or 'accept_with_findings'")
    cleaned_rationale = rationale.strip()
    if cleaned_rationale == "":
        raise InvalidTransitionError("override rationale must not be empty")

    with transaction(ctx):
        job = ctx.row_by_id("jobs", "job_id", job_id, for_update=True)
        if job["job_type"] != "review":
            raise InvalidTransitionError("verdict override is valid only for review jobs")
        if job["state"] not in {"completed", "waiting_human"}:
            raise InvalidTransitionError("verdict override requires a completed or waiting_human review job")
        session = ctx.row_by_id("sessions", "session_id", session_id)
        if str(session["run_id"]) != str(job["run_id"]):
            raise InvalidTransitionError("override session does not belong to the job run")
        if session["state"] != "active":
            raise InvalidTransitionError("override session must be active")
        with ctx.cursor() as cur:
            cur.execute(
                """
                SELECT 1
                FROM striatumd.verdicts
                WHERE repository_id = %s AND job_id = %s AND session_id = %s
                LIMIT 1
                """,
                (ctx.repository_id, job_id, session_id),
            )
            if cur.fetchone() is not None:
                raise InvalidTransitionError(
                    "override session already has a verdict for this job; register a fresh session"
                )
            cur.execute(
                """
                SELECT *
                FROM striatumd.verdicts
                WHERE repository_id = %s AND job_id = %s
                ORDER BY created_at DESC, verdict_id DESC
                LIMIT 1
                """,
                (ctx.repository_id, job_id),
            )
            previous = cur.fetchone()
        if previous is None:
            raise InvalidTransitionError("review job has no prior verdict to override")
        previous = dict(previous)
        previous_verdict = str(previous["verdict"])
        if previous_verdict in {"accept", "accept_with_findings"}:
            return {
                "status": "already_accepting",
                "job_id": job_id,
                "previous_verdict": previous_verdict,
            }

        effective_findings_artifact_id = findings_artifact_id
        if effective_findings_artifact_id is None and previous["findings_artifact_id"] is not None:
            effective_findings_artifact_id = str(previous["findings_artifact_id"])
        if effective_findings_artifact_id is not None:
            artifact = ctx.row_by_id("artifacts", "artifact_id", effective_findings_artifact_id)
            if str(artifact["run_id"]) != str(job["run_id"]):
                raise InvalidTransitionError("findings artifact belongs to a different run")
            if str(artifact["job_id"]) != job_id:
                raise InvalidTransitionError("findings artifact belongs to a different job")

        now = after_timestamp(str(previous["created_at"]), ctx.now())
        verdict_id = ctx.new_id("verdict")
        posture = resolve_review_posture(ctx, job=job)
        with ctx.cursor() as cur:
            cur.execute(
                """
                INSERT INTO striatumd.verdicts (
                  repository_id, verdict_id, run_id, job_id, session_id,
                  verdict, rationale, findings_artifact_id, created_at, posture
                )
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                """,
                (
                    ctx.repository_id,
                    verdict_id,
                    job["run_id"],
                    job_id,
                    session_id,
                    verdict,
                    cleaned_rationale,
                    effective_findings_artifact_id,
                    now,
                    posture,
                ),
            )

        resolved_blockers = 0
        if job["state"] == "waiting_human":
            message_id = job["current_message_id"]
            with ctx.cursor() as cur:
                cur.execute(
                    """
                    UPDATE striatumd.jobs
                    SET state = 'completed', completed_at = %s
                    WHERE repository_id = %s AND job_id = %s
                    """,
                    (now, ctx.repository_id, job_id),
                )
                if message_id is not None:
                    cur.execute(
                        """
                        UPDATE striatumd.queue_messages
                        SET state = 'completed', completed_at = %s, updated_at = %s
                        WHERE repository_id = %s AND message_id = %s
                        """,
                        (now, now, ctx.repository_id, message_id),
                    )
                cur.execute(
                    """
                    UPDATE striatumd.blockers
                    SET state = 'resolved', resolved_at = %s
                    WHERE repository_id = %s AND job_id = %s AND state = 'open'
                      AND severity = 'human_checkpoint'
                      AND blocker_kind = 'revision_routing'
                    """,
                    (now, ctx.repository_id, job_id),
                )
                resolved_blockers = int(cur.rowcount or 0)

        ctx.append_event(
            run_id=str(job["run_id"]),
            event_type="verdict.overridden",
            actor_session_id=session_id,
            job_id=job_id,
            artifact_id=effective_findings_artifact_id,
            payload={
                "previous_verdict": previous_verdict,
                "verdict": verdict,
                "previous_verdict_id": previous["verdict_id"],
                "verdict_id": verdict_id,
                "resolved_blockers": resolved_blockers,
            },
        )
        maybe_enqueue_downstream(ctx, completed_job_id=job_id)
        maybe_complete_run(ctx, run_id=str(job["run_id"]))
        return {
            "status": "overridden",
            "job_id": job_id,
            "previous_verdict": previous_verdict,
            "verdict": verdict,
            "verdict_id": verdict_id,
            "findings_artifact_id": effective_findings_artifact_id,
            "resolved_blockers": resolved_blockers,
        }


__all__ = ["handle"]
