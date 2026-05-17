"""PG-backed run lifecycle handlers."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    _ts,
    close_remaining_sessions,
    enqueue_job,
    transaction,
    workflow_for_run,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.daemon_rpc.envelope import RpcError
from striatum.errors import InvalidTransitionError, WorkflowError

_TERMINAL_RUN_STATES = {"completed", "failed", "canceled"}


@register_pg_handler("run.start")
def handle_run_start(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = _required_text(params, "run_id", method="run.start")
    with transaction(ctx):
        run = ctx.row_by_id("runs", "run_id", run_id, for_update=True)
        workflow = workflow_for_run(ctx, run_id=run_id)
        if workflow.get("provenance_mode", "advisory") == "sealed_patch":
            raise WorkflowError(
                "provenance_mode sealed_patch is unsupported: no containment mechanism shipped; "
                "sealed runs refuse to start rather than silently downgrading to advisory"
            )
        state = str(run["state"])
        if state == "needs_branch_confirmation":
            raise WorkflowError("branch confirmation is required before run start")
        if state not in {"ready", "running"}:
            raise InvalidTransitionError("run cannot be started from its current state")
        if state == "ready":
            now = ctx.now()
            with ctx.cursor() as cur:
                cur.execute(
                    """
                    UPDATE striatumd.runs
                    SET state = 'running', started_at = %s
                    WHERE repository_id = %s AND run_id = %s
                    """,
                    (now, ctx.repository_id, run_id),
                )
                cur.execute(
                    """
                    SELECT j.job_id
                    FROM striatumd.jobs j
                    WHERE j.repository_id = %s
                      AND j.run_id = %s
                      AND NOT EXISTS (
                        SELECT 1
                        FROM striatumd.job_dependencies dep
                        WHERE dep.repository_id = j.repository_id
                          AND dep.job_id = j.job_id
                      )
                    ORDER BY j.created_at
                    """,
                    (ctx.repository_id, run_id),
                )
                roots = [str(row["job_id"]) for row in cur.fetchall()]
            for job_id in roots:
                enqueue_job(ctx, job_id=job_id)
            ctx.append_event(run_id=run_id, event_type="run.started")
        return {"run_id": run_id, "state": "running"}


@register_pg_handler("run.pause")
def handle_run_pause(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = _required_text(params, "run_id", method="run.pause")
    reason = str(params.get("reason") or "operator_paused")
    with transaction(ctx):
        run = ctx.row_by_id("runs", "run_id", run_id, for_update=True)
        state = str(run["state"])
        if state in _TERMINAL_RUN_STATES:
            raise InvalidTransitionError(f"run is in terminal state {state!r} and cannot be paused")
        if run.get("paused_at") is not None:
            return {
                "run_id": run_id,
                "state": state,
                "paused_at": _ts(run["paused_at"]),
                "status": "already_paused",
            }
        now = ctx.now()
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.runs
                SET paused_at = %s, paused_reason = %s
                WHERE repository_id = %s AND run_id = %s AND paused_at IS NULL
                """,
                (now, reason, ctx.repository_id, run_id),
            )
        ctx.append_event(run_id=run_id, event_type="run.paused", payload={"reason": reason})
        return {"run_id": run_id, "state": state, "paused_at": now, "status": "paused"}


@register_pg_handler("run.resume")
def handle_run_resume(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = _required_text(params, "run_id", method="run.resume")
    with transaction(ctx):
        run = ctx.row_by_id("runs", "run_id", run_id, for_update=True)
        state = str(run["state"])
        if state in _TERMINAL_RUN_STATES:
            raise InvalidTransitionError(f"run is in terminal state {state!r}; use retry_job to revive")
        if run.get("paused_at") is None:
            return {"run_id": run_id, "state": state, "paused_at": None, "status": "not_paused"}
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.runs
                SET paused_at = NULL, paused_reason = NULL
                WHERE repository_id = %s AND run_id = %s
                """,
                (ctx.repository_id, run_id),
            )
        ctx.append_event(run_id=run_id, event_type="run.resumed", payload={})
        return {"run_id": run_id, "state": state, "paused_at": None, "status": "resumed"}


@register_pg_handler("run.cancel")
def handle_run_cancel(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = _required_text(params, "run_id", method="run.cancel")
    reason = str(params.get("reason") or "operator_canceled")
    with transaction(ctx):
        run = ctx.row_by_id("runs", "run_id", run_id, for_update=True)
        state = str(run["state"])
        if state == "canceled":
            return {"run_id": run_id, "state": "canceled", "status": "already_canceled"}
        if state in {"completed", "failed"}:
            raise InvalidTransitionError(f"run is in terminal state {state!r} and cannot be canceled")
        now = ctx.now()
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.jobs
                SET state = 'canceled', completed_at = %s
                WHERE repository_id = %s
                  AND run_id = %s
                  AND state IN ('queued','running','blocked','ready','claimed')
                """,
                (now, ctx.repository_id, run_id),
            )
            cur.execute(
                """
                UPDATE striatumd.leases
                SET state = 'released', released_at = %s, release_reason = 'run_canceled'
                WHERE repository_id = %s
                  AND run_id = %s
                  AND state = 'active'
                """,
                (now, ctx.repository_id, run_id),
            )
            cur.execute(
                """
                UPDATE striatumd.runs
                SET state = 'canceled', completed_at = %s, stop_reason = %s
                WHERE repository_id = %s AND run_id = %s
                """,
                (now, reason, ctx.repository_id, run_id),
            )
        ctx.append_event(run_id=run_id, event_type="run.canceled", payload={"reason": reason})
        close_remaining_sessions(ctx, run_id=run_id, source="run_canceled", reason="run_canceled")
        return {"run_id": run_id, "state": "canceled", "status": "canceled"}


@register_pg_handler("run.retry_job")
def handle_run_retry_job(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = _required_text(params, "run_id", method="run.retry_job")
    job_id = _required_text(params, "job_id", method="run.retry_job")
    with transaction(ctx):
        run = ctx.row_by_id("runs", "run_id", run_id, for_update=True)
        if str(run["state"]) == "completed":
            raise InvalidTransitionError("run is completed; retry would revive a finished run")
        job = ctx.row_by_id("jobs", "job_id", job_id, for_update=True)
        if str(job["run_id"]) != run_id:
            raise InvalidTransitionError("job does not belong to the requested run")
        previous_state = str(job["state"])
        if previous_state not in {"failed", "canceled", "blocked"}:
            raise InvalidTransitionError(
                f"job state {previous_state!r} is not retriable"
                " (must be failed, canceled, or blocked)"
            )
        now = ctx.now()
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.jobs
                SET state = 'queued',
                    started_at = NULL,
                    completed_at = NULL,
                    current_lease_id = NULL,
                    current_message_id = NULL,
                    attempt = attempt + 1
                WHERE repository_id = %s AND job_id = %s
                """,
                (ctx.repository_id, job_id),
            )
            cur.execute(
                """
                UPDATE striatumd.blockers
                SET state = 'canceled', resolved_at = %s
                WHERE repository_id = %s AND job_id = %s AND resolved_at IS NULL
                """,
                (now, ctx.repository_id, job_id),
            )
            cur.execute(
                """
                UPDATE striatumd.queue_messages
                SET state = 'canceled', updated_at = %s
                WHERE repository_id = %s
                  AND job_id = %s
                  AND state IN ('pending','claimed','acked')
                """,
                (now, ctx.repository_id, job_id),
            )
        enqueue_job(ctx, job_id=job_id)
        run_revived = False
        if str(run["state"]) in {"failed", "canceled"}:
            with ctx.cursor() as cur:
                cur.execute(
                    """
                    UPDATE striatumd.runs
                    SET state = 'running', completed_at = NULL, stop_reason = NULL
                    WHERE repository_id = %s AND run_id = %s
                    """,
                    (ctx.repository_id, run_id),
                )
            ctx.append_event(
                run_id=run_id,
                event_type="run.revived",
                payload={"trigger_job_id": job_id, "previous_run_state": str(run["state"])},
            )
            run_revived = True
        ctx.append_event(
            run_id=run_id,
            event_type="job.retried",
            job_id=job_id,
            payload={"previous_state": previous_state, "attempt": int(job["attempt"]) + 1},
        )
        return {
            "run_id": run_id,
            "job_id": job_id,
            "previous_state": previous_state,
            "new_state": "queued",
            "run_revived": run_revived,
        }


def _required_text(params: Mapping[str, Any], key: str, *, method: str) -> str:
    value = str(params.get(key) or "")
    if value == "":
        required = "run_id and job_id" if method == "run.retry_job" else key
        raise RpcError("schema_invalid", f"{method} requires {required}")
    return value
