"""PG-backed checkpoint resolution handler."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    enqueue_job,
    maybe_complete_run,
    transaction,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.daemon_rpc.envelope import RpcError
from striatum.errors import InvalidTransitionError, NotFoundError


@register_pg_handler("checkpoint.resolve")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    blocker_id = _required_text(params, "blocker_id")
    action = _required_text(params, "action")
    decision_id = _optional_text(params.get("decision_id"))
    if action not in {"continue", "cancel"}:
        raise InvalidTransitionError(f"unknown checkpoint resolve action {action!r}")

    with transaction(ctx):
        blocker = ctx.row_by_id("blockers", "blocker_id", blocker_id, for_update=True)
        if blocker["state"] != "open":
            raise InvalidTransitionError("blocker is not open")
        if blocker["severity"] != "human_checkpoint":
            raise InvalidTransitionError(
                "checkpoint resolve only applies to human_checkpoint blockers"
            )

        run_id = str(blocker["run_id"])
        blocker_job_id = _optional_text(blocker.get("job_id"))
        decision_artifact_id = (
            _decision_artifact_id(ctx, run_id=run_id, decision_id=decision_id)
            if decision_id is not None
            else None
        )

        now = ctx.now()
        downstream_jobs: list[dict[str, Any]] = []
        if action == "continue":
            _resolve_blocker(ctx, blocker_id=blocker_id, resolved_at=now)
            if blocker_job_id is not None:
                downstream_jobs = _continue_checkpoint_job(
                    ctx,
                    job_id=blocker_job_id,
                    ready_at=now,
                )
            event_type = "checkpoint.resolved"
            next_actions = ["claim_available_work", "monitor_run_progress"]
        else:
            _resolve_blocker(ctx, blocker_id=blocker_id, resolved_at=now)
            if blocker_job_id is not None:
                downstream_jobs = _cancel_checkpoint_job(
                    ctx,
                    job_id=blocker_job_id,
                    completed_at=now,
                )
            event_type = "checkpoint.canceled"
            maybe_complete_run(ctx, run_id=run_id)
            next_actions = ["inspect_run_state", "export_run_evidence"]

        event_payload: dict[str, Any] = {"blocker_id": blocker_id, "action": action}
        if decision_id is not None:
            event_payload["decision_id"] = decision_id
        if decision_artifact_id is not None:
            event_payload["decision_artifact_id"] = decision_artifact_id
        ctx.append_event(
            run_id=run_id,
            event_type=event_type,
            job_id=blocker_job_id,
            artifact_id=decision_artifact_id,
            payload=event_payload,
        )
        run = ctx.row_by_id("runs", "run_id", run_id)
        return {
            "status": "resolved",
            "blocker_id": blocker_id,
            "job_id": blocker_job_id,
            "action": action,
            "decision_id": decision_id,
            "decision_artifact_id": decision_artifact_id,
            "run_state": str(run["state"]),
            "downstream_jobs": downstream_jobs,
            "next_actions": next_actions,
        }


def _continue_checkpoint_job(
    ctx: RepoHandlerContext,
    *,
    job_id: str,
    ready_at: str,
) -> list[dict[str, Any]]:
    job = ctx.row_by_id("jobs", "job_id", job_id, for_update=True)
    _require_waiting_human(job)
    message_id = _optional_text(job.get("current_message_id"))
    with ctx.cursor() as cur:
        if message_id is not None:
            cur.execute(
                """
                UPDATE striatumd.queue_messages
                SET state = 'pending', current_lease_id = NULL, updated_at = %s
                WHERE repository_id = %s AND message_id = %s
                """,
                (ready_at, ctx.repository_id, message_id),
            )
            cur.execute(
                """
                UPDATE striatumd.jobs
                SET state = 'queued', current_lease_id = NULL, ready_at = %s
                WHERE repository_id = %s AND job_id = %s
                """,
                (ready_at, ctx.repository_id, job_id),
            )
        else:
            cur.execute(
                """
                UPDATE striatumd.jobs
                SET state = 'blocked', current_lease_id = NULL
                WHERE repository_id = %s AND job_id = %s
                """,
                (ctx.repository_id, job_id),
            )
    if message_id is None:
        enqueue_job(ctx, job_id=job_id)
    return _downstream_jobs(ctx, job_id=job_id)


def _cancel_checkpoint_job(
    ctx: RepoHandlerContext,
    *,
    job_id: str,
    completed_at: str,
) -> list[dict[str, Any]]:
    job = ctx.row_by_id("jobs", "job_id", job_id, for_update=True)
    _require_waiting_human(job)
    message_id = _optional_text(job.get("current_message_id"))
    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET state = 'canceled',
                current_lease_id = NULL,
                current_message_id = NULL,
                completed_at = %s
            WHERE repository_id = %s AND job_id = %s
            """,
            (completed_at, ctx.repository_id, job_id),
        )
        if message_id is not None:
            cur.execute(
                """
                UPDATE striatumd.queue_messages
                SET state = 'canceled', current_lease_id = NULL, updated_at = %s
                WHERE repository_id = %s AND message_id = %s
                """,
                (completed_at, ctx.repository_id, message_id),
            )
    return _downstream_jobs(ctx, job_id=job_id)


def _resolve_blocker(ctx: RepoHandlerContext, *, blocker_id: str, resolved_at: str) -> None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.blockers
            SET state = 'resolved', resolved_at = %s
            WHERE repository_id = %s AND blocker_id = %s
            """,
            (resolved_at, ctx.repository_id, blocker_id),
        )


def _decision_artifact_id(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    decision_id: str,
) -> str:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT artifact_id, job_id, session_id
            FROM striatumd.artifacts
            WHERE repository_id = %s
              AND artifact_kind = 'decision'
              AND run_id = %s
              AND logical_name = %s
            LIMIT 1
            """,
            (ctx.repository_id, run_id, decision_id),
        )
        row = cur.fetchone()
    if row is None:
        raise NotFoundError(
            f"decision artifact for decision_id={decision_id!r} not found in run"
        )
    if row["job_id"] is not None or row["session_id"] is not None:
        raise InvalidTransitionError(
            "decision artifact must be run-level (no job or session binding)"
        )
    return str(row["artifact_id"])


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


def _require_waiting_human(job: Mapping[str, Any]) -> None:
    if job["state"] != "waiting_human":
        raise InvalidTransitionError(
            f"checkpoint job is not in waiting_human (state={job['state']!r})"
        )


def _required_text(params: Mapping[str, Any], key: str) -> str:
    value = str(params.get(key) or "")
    if value == "":
        raise RpcError("schema_invalid", "checkpoint.resolve requires blocker_id and action")
    return value


def _optional_text(value: object) -> str | None:
    if value is None:
        return None
    text = str(value)
    return text if text != "" else None


__all__ = ["handle"]
