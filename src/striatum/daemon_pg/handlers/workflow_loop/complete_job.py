"""PG-backed ``work.complete`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    active_lease_for,
    maybe_complete_run,
    maybe_enqueue_downstream,
    transaction,
    verify_required_artifacts,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.errors import InvalidTransitionError


@register_pg_handler("work.complete", "complete")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = str(params["session_id"])
    job_id = str(params["job_id"])
    lease_id = str(params["lease_id"])
    summary = params.get("summary")
    return complete_inline(
        ctx,
        session_id=session_id,
        job_id=job_id,
        lease_id=lease_id,
        summary=summary,
    )


def complete_inline(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    summary: Any = None,
) -> dict[str, Any]:
    """Run the work.complete substrate transition.

    RFC 0048 V1.5 #2: ``recovery.resume --complete`` and ``recovery.auto``
    live-mode auto-publish call this inside their own transaction so the
    blocker-resolution or artifact-publish step and the job completion
    are serialized together. Implemented as a thin alias over the same
    logic that ``handle`` runs — psycopg's nested ``transaction()`` is
    a savepoint, so re-entry from another transaction is safe.
    """
    with transaction(ctx):
        job = ctx.row_by_id("jobs", "job_id", job_id, for_update=True)
        active_lease_for(ctx, lease_id=lease_id, session_id=session_id, job_id=job_id)
        if job["job_type"] in {"review", "phase_synthesis"}:
            label = "phase_synthesis" if job["job_type"] == "phase_synthesis" else "review"
            raise InvalidTransitionError(
                f"{label} jobs must use submit-review instead of complete"
            )
        if job["state"] != "running":
            raise InvalidTransitionError("job must be running before completion")
        verify_required_artifacts(ctx, job_id=job_id)
        now = ctx.now()
        message_id = job["current_message_id"]
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.jobs
                SET state = 'completed', completed_at = %s, current_lease_id = NULL
                WHERE repository_id = %s AND job_id = %s
                """,
                (now, ctx.repository_id, job_id),
            )
            if message_id is not None:
                cur.execute(
                    """
                    UPDATE striatumd.queue_messages
                    SET state = 'completed', completed_at = %s, updated_at = %s,
                        current_lease_id = NULL
                    WHERE repository_id = %s AND message_id = %s
                    """,
                    (now, now, ctx.repository_id, message_id),
                )
            cur.execute(
                """
                UPDATE striatumd.leases
                SET state = 'released', released_at = %s, release_reason = 'completed'
                WHERE repository_id = %s AND lease_id = %s
                """,
                (now, ctx.repository_id, lease_id),
            )
        ctx.append_event(
            run_id=str(job["run_id"]),
            event_type="job.completed",
            actor_session_id=session_id,
            job_id=job_id,
            message_id=message_id,
            lease_id=lease_id,
            payload={"summary": summary},
        )
        maybe_enqueue_downstream(ctx, completed_job_id=job_id)
        maybe_complete_run(ctx, run_id=str(job["run_id"]))
        return {"status": "completed", "job_id": job_id}
