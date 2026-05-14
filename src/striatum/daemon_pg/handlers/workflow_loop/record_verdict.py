"""PG-backed ``review.verdict`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any, cast

from psycopg.types.json import Jsonb

from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    _json_loads,
    _jsonb,
    active_lease_for,
    enqueue_job,
    maybe_complete_run,
    maybe_enqueue_downstream,
    resolve_review_posture,
    session_lane_attestation,
    transaction,
    verify_required_artifacts,
    workflow_for_run,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.errors import InvalidTransitionError, NotFoundError


@register_pg_handler("review.verdict", "verdict")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    with transaction(ctx):
        return record_verdict_inline(
            ctx,
            session_id=str(params["session_id"]),
            job_id=str(params["job_id"]),
            lease_id=str(params["lease_id"]),
            verdict=str(params["verdict"]),
            findings_artifact_id=(
                str(params["findings_artifact_id"])
                if params.get("findings_artifact_id") is not None
                else None
            ),
            rationale=(
                str(params["rationale"]) if params.get("rationale") is not None else None
            ),
        )


def record_verdict_inline(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    verdict: str,
    findings_artifact_id: str | None,
    rationale: str | None,
) -> dict[str, Any]:
    """Record a review verdict inside the caller's transaction."""
    job = ctx.row_by_id("jobs", "job_id", job_id, for_update=True)
    if job["job_type"] not in {"review", "phase_synthesis"}:
        raise InvalidTransitionError("verdict is valid only for verdict-capable jobs")
    active_lease_for(ctx, lease_id=lease_id, session_id=session_id, job_id=job_id)
    if job["state"] != "running":
        job_label = "phase_synthesis" if job["job_type"] == "phase_synthesis" else "review"
        raise InvalidTransitionError(f"{job_label} job must be running before verdict")
    _enforce_required_attestation_for_verdict(ctx, job=job, session_id=session_id)
    verify_required_artifacts(ctx, job_id=job_id)

    verdict_id = ctx.new_id("verdict")
    now = ctx.now()
    posture = "neutral" if job["job_type"] == "phase_synthesis" else resolve_review_posture(ctx, job=job)
    with ctx.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.verdicts (
              repository_id, verdict_id, run_id, job_id, session_id, verdict,
              rationale, findings_artifact_id, created_at, posture
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
                rationale,
                findings_artifact_id,
                now,
                posture,
            ),
        )
    ctx.append_event(
        run_id=str(job["run_id"]),
        event_type="verdict.recorded",
        actor_session_id=session_id,
        job_id=job_id,
        lease_id=lease_id,
        payload={
            "verdict": verdict,
            "posture": posture,
            "lane_attestation": session_lane_attestation(ctx, session_id=session_id)["state"],
        },
    )

    if verdict in ("accept", "accept_with_findings"):
        _complete_review_job(
            ctx,
            job=job,
            session_id=session_id,
            lease_id=lease_id,
            summary=verdict,
        )
        maybe_enqueue_downstream(ctx, completed_job_id=job_id)
        maybe_complete_run(ctx, run_id=str(job["run_id"]))
        return {
            "status": "completed",
            "job_id": job_id,
            "verdict": verdict,
            "verdict_id": verdict_id,
        }
    if verdict == "needs_revision":
        result = _request_revision_for_cycle(
            ctx,
            review_job=job,
            session_id=session_id,
            lease_id=lease_id,
        )
        result["verdict_id"] = verdict_id
        return result
    if verdict == "reject":
        _fail_review_job(ctx, job=job, session_id=session_id, lease_id=lease_id)
        maybe_complete_run(ctx, run_id=str(job["run_id"]))
        return {
            "status": "failed",
            "job_id": job_id,
            "verdict": verdict,
            "verdict_id": verdict_id,
        }
    raise InvalidTransitionError(f"unknown verdict {verdict!r}")


def _complete_review_job(
    ctx: RepoHandlerContext,
    *,
    job: Mapping[str, Any],
    session_id: str,
    lease_id: str,
    summary: str,
) -> None:
    now = ctx.now()
    message_id = job["current_message_id"]
    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET state = 'completed', completed_at = %s, current_lease_id = NULL
            WHERE repository_id = %s AND job_id = %s
            """,
            (now, ctx.repository_id, job["job_id"]),
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
            SET state = 'released', released_at = %s, release_reason = 'verdict'
            WHERE repository_id = %s AND lease_id = %s
            """,
            (now, ctx.repository_id, lease_id),
        )
    ctx.append_event(
        run_id=str(job["run_id"]),
        event_type="job.completed",
        actor_session_id=session_id,
        job_id=str(job["job_id"]),
        message_id=message_id,
        lease_id=lease_id,
        payload={"summary": summary},
    )


def _fail_review_job(
    ctx: RepoHandlerContext,
    *,
    job: Mapping[str, Any],
    session_id: str,
    lease_id: str,
) -> None:
    now = ctx.now()
    message_id = job["current_message_id"]
    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET state = 'failed', completed_at = %s, current_lease_id = NULL
            WHERE repository_id = %s AND job_id = %s
            """,
            (now, ctx.repository_id, job["job_id"]),
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
            SET state = 'released', released_at = %s, release_reason = 'reject'
            WHERE repository_id = %s AND lease_id = %s
            """,
            (now, ctx.repository_id, lease_id),
        )
    ctx.append_event(
        run_id=str(job["run_id"]),
        event_type="job.failed",
        actor_session_id=session_id,
        job_id=str(job["job_id"]),
        message_id=message_id,
        lease_id=lease_id,
        payload={"reason": "reject"},
    )


def _request_revision_for_cycle(
    ctx: RepoHandlerContext,
    *,
    review_job: Mapping[str, Any],
    session_id: str,
    lease_id: str,
) -> dict[str, Any]:
    workflow = workflow_for_run(ctx, run_id=str(review_job["run_id"]))
    cycle = _matching_revision_cycle(
        workflow,
        workflow_job_id=str(review_job["workflow_job_id"]),
    )
    if cycle is None:
        policy = workflow.get("review_revision_policy", {})
        description = "needs_revision verdict has no matching workflow cycle"
        if isinstance(policy, dict) and policy.get("root_review_needs_revision") == "human_checkpoint":
            configured = policy.get("description")
            description = (
                configured
                if isinstance(configured, str) and configured != ""
                else "needs_revision routed to configured human checkpoint"
            )
        blocker_id = _open_human_checkpoint(
            ctx,
            job=review_job,
            session_id=session_id,
            lease_id=lease_id,
            description=description,
        )
        return {
            "status": "waiting_human",
            "job_id": review_job["job_id"],
            "verdict": "needs_revision",
            "blocker_id": blocker_id,
        }

    target_workflow_job_id = str(cycle["to"])
    max_iterations = int(cycle["max_iterations"])
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT COUNT(*) AS count
            FROM striatumd.jobs
            WHERE repository_id = %s AND run_id = %s
              AND workflow_job_id = %s AND attempt > 1
            """,
            (ctx.repository_id, review_job["run_id"], target_workflow_job_id),
        )
        completed_attempts = int(cur.fetchone()["count"])
    if completed_attempts >= max_iterations:
        blocker_id = _open_human_checkpoint(
            ctx,
            job=review_job,
            session_id=session_id,
            lease_id=lease_id,
            description="needs_revision cycle exhausted max_iterations",
        )
        return {
            "status": "waiting_human",
            "job_id": review_job["job_id"],
            "verdict": "needs_revision",
            "blocker_id": blocker_id,
        }

    attempt = int(review_job["attempt"]) + 1
    target_job = _latest_job_for_workflow_id(
        ctx,
        run_id=str(review_job["run_id"]),
        workflow_job_id=target_workflow_job_id,
    )
    next_target_id = _clone_job_attempt(ctx, source=target_job, attempt=attempt)
    next_review_id = _clone_job_attempt(ctx, source=review_job, attempt=attempt)
    with ctx.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.job_dependencies (
              repository_id, job_id, depends_on_job_id, gate_json
            )
            VALUES (%s, %s, %s, %s)
            ON CONFLICT (repository_id, job_id, depends_on_job_id) DO NOTHING
            """,
            (
                ctx.repository_id,
                next_review_id,
                next_target_id,
                _jsonb(
                    {
                        "on": "completed",
                        "from": target_workflow_job_id,
                        "to": review_job["workflow_job_id"],
                    }
                ),
            ),
        )
        cur.execute(
            """
            UPDATE striatumd.job_dependencies
            SET depends_on_job_id = %s
            WHERE repository_id = %s AND depends_on_job_id = %s
            """,
            (next_review_id, ctx.repository_id, review_job["job_id"]),
        )
    _complete_review_job(
        ctx,
        job=review_job,
        session_id=session_id,
        lease_id=lease_id,
        summary="needs_revision",
    )
    target_state = ctx.row_by_id("jobs", "job_id", next_target_id, for_update=True)
    if target_state["state"] == "blocked":
        enqueue_job(ctx, job_id=next_target_id)
    ctx.append_event(
        run_id=str(review_job["run_id"]),
        event_type="revision.requested",
        actor_session_id=session_id,
        job_id=str(review_job["job_id"]),
        lease_id=lease_id,
        payload={
            "next_job_id": next_target_id,
            "next_review_job_id": next_review_id,
            "attempt": attempt,
        },
    )
    return {
        "status": "revision_requested",
        "job_id": review_job["job_id"],
        "verdict": "needs_revision",
        "next_job_id": next_target_id,
    }


def _open_human_checkpoint(
    ctx: RepoHandlerContext,
    *,
    job: Mapping[str, Any],
    session_id: str,
    lease_id: str,
    description: str,
) -> str:
    now = ctx.now()
    blocker_id = ctx.new_id("blk")
    with ctx.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.blockers (
              repository_id, blocker_id, run_id, job_id, session_id, severity,
              blocker_kind, description, state, created_at
            )
            VALUES (%s, %s, %s, %s, %s, 'human_checkpoint',
                    'revision_routing', %s, 'open', %s)
            """,
            (
                ctx.repository_id,
                blocker_id,
                job["run_id"],
                job["job_id"],
                session_id,
                description,
                now,
            ),
        )
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET state = 'waiting_human', current_lease_id = NULL
            WHERE repository_id = %s AND job_id = %s
            """,
            (ctx.repository_id, job["job_id"]),
        )
        if job["current_message_id"] is not None:
            cur.execute(
                """
                UPDATE striatumd.queue_messages
                SET state = 'blocked', current_lease_id = NULL, updated_at = %s
                WHERE repository_id = %s AND message_id = %s
                """,
                (now, ctx.repository_id, job["current_message_id"]),
            )
        cur.execute(
            """
            UPDATE striatumd.leases
            SET state = 'released', released_at = %s,
                release_reason = 'needs_revision'
            WHERE repository_id = %s AND lease_id = %s
            """,
            (now, ctx.repository_id, lease_id),
        )
    ctx.append_event(
        run_id=str(job["run_id"]),
        event_type="human_checkpoint.opened",
        actor_session_id=session_id,
        job_id=str(job["job_id"]),
        lease_id=lease_id,
        payload={"blocker_id": blocker_id, "description": description},
    )
    return blocker_id


def _matching_revision_cycle(
    workflow: Mapping[str, Any],
    *,
    workflow_job_id: str,
) -> dict[str, Any] | None:
    cycles = workflow.get("cycles", [])
    if not isinstance(cycles, list):
        return None
    for cycle in cycles:
        if not isinstance(cycle, dict):
            continue
        if cycle.get("from") == workflow_job_id and cycle.get("on_verdict") == "needs_revision":
            return cast(dict[str, Any], cycle)
    return None


def _latest_job_for_workflow_id(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    workflow_job_id: str,
) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT *
            FROM striatumd.jobs
            WHERE repository_id = %s AND run_id = %s AND workflow_job_id = %s
            ORDER BY attempt DESC
            LIMIT 1
            """,
            (ctx.repository_id, run_id, workflow_job_id),
        )
        row = cur.fetchone()
    if row is None:
        raise NotFoundError(f"could not find job for workflow_job_id={workflow_job_id!r}")
    return dict(row)


def _clone_job_attempt(
    ctx: RepoHandlerContext,
    *,
    source: Mapping[str, Any],
    attempt: int,
) -> str:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT job_id
            FROM striatumd.jobs
            WHERE repository_id = %s AND run_id = %s
              AND workflow_job_id = %s AND attempt = %s
            LIMIT 1
            """,
            (
                ctx.repository_id,
                source["run_id"],
                source["workflow_job_id"],
                attempt,
            ),
        )
        existing = cur.fetchone()
    if existing is not None:
        return str(existing["job_id"])

    job_id = f"job_{source['run_id']}_{source['workflow_job_id']}_a{attempt}"
    now = ctx.now()
    with ctx.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.jobs (
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              attempt, max_attempts, fresh_session_required, write_scope_json,
              expected_artifacts_json, idempotency_key, created_at
            )
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, 'blocked',
                    %s, %s, %s, %s, %s, %s, %s)
            """,
            (
                ctx.repository_id,
                job_id,
                source["run_id"],
                source["workflow_job_id"],
                source["title"],
                source["job_type"],
                source["role_id"],
                _coerce_jsonb(source["lane_selector_json"]),
                _coerce_jsonb(source["capability_requirements_json"]),
                attempt,
                source["max_attempts"],
                source["fresh_session_required"],
                _coerce_jsonb(source["write_scope_json"]),
                _coerce_jsonb(source["expected_artifacts_json"]),
                f"{source['run_id']}:{source['workflow_job_id']}:{attempt}",
                now,
            ),
        )
    return job_id


def _coerce_jsonb(value: object) -> Jsonb:
    if isinstance(value, (dict, list)):
        return _jsonb(value)
    return _jsonb(_json_loads(value))


def _enforce_required_attestation_for_verdict(
    ctx: RepoHandlerContext,
    *,
    job: Mapping[str, Any],
    session_id: str,
) -> None:
    workflow = workflow_for_run(ctx, run_id=str(job["run_id"]))
    job_def = _workflow_job_def(workflow, workflow_job_id=str(job["workflow_job_id"]))
    if not isinstance(job_def, dict) or job_def.get("require_attested_lane") is not True:
        return
    attestation = session_lane_attestation(ctx, session_id=session_id)
    if attestation["attested"]:
        return
    reason = f" ({attestation['reason']})" if attestation.get("reason") else ""
    raise InvalidTransitionError(
        "review job requires an attached lane supervisor before recording a verdict"
        f"{reason}; recovery: striatum supervise start --session-id {session_id}"
    )


def _workflow_job_def(
    workflow: Mapping[str, Any],
    *,
    workflow_job_id: str,
) -> dict[str, Any] | None:
    jobs = workflow.get("jobs")
    if not isinstance(jobs, list):
        return None
    for item in jobs:
        if isinstance(item, dict) and item.get("id") == workflow_job_id:
            return cast(dict[str, Any], item)
    return None


__all__ = [
    "handle",
    "record_verdict_inline",
]
