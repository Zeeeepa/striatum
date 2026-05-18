"""PostgreSQL read projections for RFC 0048 Phase C."""

from __future__ import annotations

from collections import Counter
from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    session_lane_attestation,
)
from striatum.daemon_pg.handlers.recovery_evidence.auto_finalize import (
    dry_run_projection as auto_finalize_dry_run_projection,
)
from striatum.daemon_pg.handlers.recovery_evidence.supervisor_stalls import (
    DEFAULT_SUPERVISOR_STALL_AFTER_SECONDS,
    stalled_supervisors,
)
from striatum.daemon_rpc.envelope import RpcError
from striatum.next_actions import next_actions

from ._sql import (
    parse_json_list,
    parse_json_object,
    row_to_json,
    run_exists,
)


def workflow_for_run(ctx: RepoHandlerContext, run_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT w.workflow_json
            FROM striatumd.runs r
            JOIN striatumd.workflow_snapshots w
              ON w.repository_id = r.repository_id
             AND w.workflow_snapshot_id = r.workflow_snapshot_id
            WHERE r.repository_id = %s AND r.run_id = %s
            """,
            (ctx.repository_id, run_id),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("not_found", f"run not found: {run_id}")
    return parse_json_object(row["workflow_json"])


def status_payload(ctx: RepoHandlerContext, *, run_id: str | None) -> dict[str, Any]:
    where = "r.repository_id = %s"
    args: list[Any] = [ctx.repository_id]
    job_where = "j.repository_id = %s"
    job_args: list[Any] = [ctx.repository_id]
    if run_id is not None:
        where += " AND r.run_id = %s"
        args.append(run_id)
        job_where += " AND j.run_id = %s"
        job_args.append(run_id)
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT r.run_id, r.state, r.branch_name
            FROM striatumd.runs r
            WHERE {where}
            ORDER BY r.created_at, r.run_id
            """,
            tuple(args),
        )
        runs = [row_to_json(row) for row in cur.fetchall()]
        cur.execute(
            f"""
            SELECT j.state, COUNT(*) AS count
            FROM striatumd.jobs j
            WHERE {job_where}
            GROUP BY j.state
            ORDER BY j.state
            """,
            tuple(job_args),
        )
        jobs = {str(row["state"]): int(row["count"]) for row in cur.fetchall()}
        open_blockers = blocker_summaries(ctx, run_id=run_id, severity=None)
        human_checkpoints = blocker_summaries(ctx, run_id=run_id, severity="human_checkpoint")
    non_accepting = latest_non_accepting_verdicts(ctx, run_id=run_id)
    claimable = claimable_jobs(ctx, run_id=run_id)
    health = process_health(ctx, run_id=run_id)
    supervisor_stalls = supervisor_stall_health(ctx, run_id=run_id)
    auto_finalize = (
        auto_finalize_projection(ctx, run_id=run_id) if run_id is not None else None
    )
    actions = next_actions(
        open_blockers=open_blockers,
        human_checkpoints=human_checkpoints,
        non_accepting_verdicts=non_accepting,
        claimable_jobs=claimable,
        has_orphan_supervisor=has_supervisor_lost_with_held_lease(ctx, run_id=run_id),
        has_stale_leases=has_stale_leases_with_on_disk_artifacts(ctx, run_id=run_id),
    )
    if (
        auto_finalize
        and int(auto_finalize.get("eligible_count") or 0) > 0
        and "recovery_auto_finalize" not in actions
    ):
        actions.append("recovery_auto_finalize")
    for extra in health.get("next_actions", []):
        if extra not in actions:
            actions.append(extra)
    for extra in supervisor_stalls.get("next_actions", []):
        if extra not in actions:
            actions.append(extra)
    result = {
        "runs": runs,
        "provenance_mode": provenance_mode(ctx, run_id=run_id),
        "sessions": session_summaries(ctx, run_id=run_id),
        "jobs": jobs,
        "open_blockers": open_blockers,
        "human_checkpoints": human_checkpoints,
        "latest_non_accepting_review_verdicts": non_accepting,
        "verdicts_by_posture": verdicts_by_posture(ctx, run_id=run_id),
        "claimable_jobs": claimable,
        "blocked_downstream_jobs": blocked_downstream_jobs(ctx, run_id=run_id),
        "process_health": health,
        "supervisor_stalls": supervisor_stalls,
        "auto_finalize_dry_run": auto_finalize,
        "next_actions": actions,
    }
    if run_id is not None:
        phase = phase_progress(ctx, run_id=run_id)
        if phase is not None:
            result.update(phase)
    return result


def auto_finalize_projection(ctx: RepoHandlerContext, *, run_id: str) -> dict[str, Any]:
    return auto_finalize_dry_run_projection(ctx, run_id=run_id)


def provenance_mode(ctx: RepoHandlerContext, *, run_id: str | None) -> str | None:
    if run_id is None:
        return None
    workflow = workflow_for_run(ctx, run_id)
    value = workflow.get("provenance_mode")
    return str(value) if value is not None else None


def session_summaries(ctx: RepoHandlerContext, *, run_id: str | None) -> list[dict[str, Any]]:
    where = "s.repository_id = %s"
    args: list[Any] = [ctx.repository_id]
    if run_id is not None:
        where += " AND s.run_id = %s"
        args.append(run_id)
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT s.session_id, s.run_id, s.role_id, s.lane_id, s.slug, s.ordinal,
                   s.state, s.operator_label,
                   ps.supervisor_id AS supervisor_id
            FROM striatumd.sessions s
            LEFT JOIN striatumd.process_supervisors ps
              ON ps.repository_id = s.repository_id
             AND ps.session_id = s.session_id
             AND ps.state = 'attached'
            WHERE {where}
            ORDER BY s.registered_at, s.session_id
            """,
            tuple(args),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    for row in rows:
        attestation = session_lane_attestation(ctx, session_id=str(row["session_id"]))
        row["lane_attestation"] = attestation.get("state")
        row["lane_attestation_reason"] = attestation.get("reason")
        row["pid"] = attestation.get("pid")
        row["supervisor_id"] = attestation.get("supervisor_id")
    return rows


def blocker_summaries(ctx: RepoHandlerContext, *, run_id: str | None, severity: str | None) -> list[dict[str, Any]]:
    where = "b.repository_id = %s AND b.state = 'open'"
    args: list[Any] = [ctx.repository_id]
    if run_id is not None:
        where += " AND b.run_id = %s"
        args.append(run_id)
    if severity is not None:
        where += " AND b.severity = %s"
        args.append(severity)
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT b.blocker_id, b.run_id, b.job_id, b.session_id, b.severity,
                   b.blocker_kind, b.description, b.state, b.created_at,
                   b.payload_json,
                   j.workflow_job_id, j.state AS job_state
            FROM striatumd.blockers b
            LEFT JOIN striatumd.jobs j
              ON j.repository_id = b.repository_id
             AND j.job_id = b.job_id
            WHERE {where}
            ORDER BY b.created_at, b.blocker_id
            """,
            tuple(args),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def latest_non_accepting_verdicts(ctx: RepoHandlerContext, *, run_id: str | None) -> list[dict[str, Any]]:
    where = "v.repository_id = %s AND v.verdict != 'accept'"
    args: list[Any] = [ctx.repository_id]
    if run_id is not None:
        where += " AND v.run_id = %s"
        args.append(run_id)
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT DISTINCT ON (v.job_id)
                   v.verdict_id, v.run_id, v.job_id, j.workflow_job_id,
                   v.verdict, v.posture, v.created_at
            FROM striatumd.verdicts v
            JOIN striatumd.jobs j
              ON j.repository_id = v.repository_id
             AND j.job_id = v.job_id
            WHERE {where}
            ORDER BY v.job_id, v.created_at DESC, v.verdict_id DESC
            """,
            tuple(args),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def verdicts_by_posture(ctx: RepoHandlerContext, *, run_id: str | None) -> dict[str, dict[str, int]]:
    where = "v.repository_id = %s"
    args: list[Any] = [ctx.repository_id]
    if run_id is not None:
        where += " AND v.run_id = %s"
        args.append(run_id)
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT v.posture, v.verdict, COUNT(*) AS count
            FROM striatumd.verdicts v
            WHERE {where}
            GROUP BY v.posture, v.verdict
            ORDER BY v.posture, v.verdict
            """,
            tuple(args),
        )
        grouped: dict[str, dict[str, int]] = {}
        for row in cur.fetchall():
            grouped.setdefault(str(row["posture"]), {})[str(row["verdict"])] = int(row["count"])
        return grouped


def claimable_jobs(ctx: RepoHandlerContext, *, run_id: str | None) -> list[dict[str, Any]]:
    where = "q.repository_id = %s AND q.state = 'pending' AND (q.visible_after IS NULL OR q.visible_after <= now())"
    args: list[Any] = [ctx.repository_id]
    if run_id is not None:
        where += " AND q.run_id = %s"
        args.append(run_id)
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT q.run_id, q.job_id, j.workflow_job_id, q.target_role_id AS role_id,
                   q.target_lane_id AS lane_id, COUNT(*) AS count
            FROM striatumd.queue_messages q
            JOIN striatumd.jobs j
              ON j.repository_id = q.repository_id
             AND j.job_id = q.job_id
            WHERE {where}
            GROUP BY q.run_id, q.job_id, j.workflow_job_id, q.target_role_id, q.target_lane_id
            ORDER BY q.target_role_id, q.target_lane_id, j.workflow_job_id
            """,
            tuple(args),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def blocked_downstream_jobs(ctx: RepoHandlerContext, *, run_id: str | None) -> list[dict[str, Any]]:
    where = "j.repository_id = %s AND j.state = 'blocked'"
    args: list[Any] = [ctx.repository_id]
    if run_id is not None:
        where += " AND j.run_id = %s"
        args.append(run_id)
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT j.run_id, j.job_id, j.workflow_job_id, j.state
            FROM striatumd.jobs j
            WHERE {where}
            ORDER BY j.created_at, j.workflow_job_id
            """,
            tuple(args),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def process_health(ctx: RepoHandlerContext, *, run_id: str | None) -> dict[str, Any]:
    if run_id is None:
        return {
            "running_count": 0,
            "stale_running_count": 0,
            "lost_count": 0,
            "timed_out_count": 0,
            "next_actions": [],
        }
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT
              COUNT(*) FILTER (WHERE p.state = 'running') AS running_count,
              COUNT(*) FILTER (
                WHERE p.state = 'running' AND l.state = 'expired'
              ) AS stale_running_count,
              COUNT(*) FILTER (WHERE p.state = 'lost') AS lost_count,
              COUNT(*) FILTER (WHERE p.state = 'timed_out') AS timed_out_count
            FROM striatumd.process_executions p
            LEFT JOIN striatumd.leases l
              ON l.repository_id = p.repository_id
             AND l.lease_id = p.lease_id
            WHERE p.repository_id = %s AND p.run_id = %s
            """,
            (ctx.repository_id, run_id),
        )
        row = cur.fetchone() or {}
    stale_running_count = int(row.get("stale_running_count") or 0)
    return {
        "running_count": int(row.get("running_count") or 0),
        "stale_running_count": stale_running_count,
        "lost_count": int(row.get("lost_count") or 0),
        "timed_out_count": int(row.get("timed_out_count") or 0),
        "next_actions": ["recovery_process_reconcile"] if stale_running_count > 0 else [],
    }


def supervisor_stall_health(ctx: RepoHandlerContext, *, run_id: str | None) -> dict[str, Any]:
    rows = stalled_supervisors(
        ctx,
        run_id=run_id,
        stall_after_seconds=DEFAULT_SUPERVISOR_STALL_AFTER_SECONDS,
    )
    expired = [row for row in rows if row["lease_expired"]]
    warnings = [row for row in rows if not row["lease_expired"]]
    return {
        "stalled_count": len(rows),
        "warning_count": len(warnings),
        "expired_count": len(expired),
        "stall_after_seconds": DEFAULT_SUPERVISOR_STALL_AFTER_SECONDS,
        "supervisors": [_supervisor_stall_summary(row) for row in rows],
        "next_actions": ["supervisor_stall_investigate"] if rows else [],
    }


def _supervisor_stall_summary(row: Mapping[str, Any]) -> dict[str, Any]:
    return {
        "supervisor_id": row["supervisor_id"],
        "run_id": row["run_id"],
        "session_id": row["session_id"],
        "job_id": row["job_id"],
        "workflow_job_id": row["workflow_job_id"],
        "lease_id": row["lease_id"],
        "message_id": row.get("message_id"),
        "last_progress_at": row["last_progress_at"],
        "last_progress_age_seconds": row["last_progress_age_seconds"],
        "lease_expires_at": row["lease_expires_at"],
        "lease_expired": row["lease_expired"],
    }


def has_supervisor_lost_with_held_lease(ctx: RepoHandlerContext, *, run_id: str | None) -> bool:
    where = "s.repository_id = %s AND s.state = 'lost'"
    args: list[Any] = [ctx.repository_id]
    if run_id is not None:
        where += " AND s.run_id = %s"
        args.append(run_id)
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT 1
            FROM striatumd.process_supervisors s
            JOIN striatumd.leases l
              ON l.repository_id = s.repository_id
             AND l.owner_session_id = s.session_id
             AND l.state = 'active'
             AND l.expires_at > now()
            WHERE {where}
            LIMIT 1
            """,
            tuple(args),
        )
        return cur.fetchone() is not None


def has_stale_leases_with_on_disk_artifacts(ctx: RepoHandlerContext, *, run_id: str | None) -> bool:
    where = (
        "j.repository_id = %s AND l.state = 'expired' "
        "AND j.state IN ('claimed', 'running', 'stale_lease')"
    )
    args: list[Any] = [ctx.repository_id]
    if run_id is not None:
        where += " AND j.run_id = %s"
        args.append(run_id)
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT j.expected_artifacts_json
            FROM striatumd.jobs j
            JOIN striatumd.leases l
              ON l.repository_id = j.repository_id
             AND l.lease_id = j.current_lease_id
            WHERE {where}
            """,
            tuple(args),
        )
        rows = cur.fetchall()
    for row in rows:
        for artifact in parse_json_list(row.get("expected_artifacts_json")):
            if not isinstance(artifact, Mapping) or not artifact.get("required", True):
                continue
            path = artifact.get("path")
            if isinstance(path, str) and path and (ctx.repo_root / path).exists():
                return True
    return False


def phase_progress(ctx: RepoHandlerContext, *, run_id: str) -> dict[str, Any] | None:
    workflow = workflow_for_run(ctx, run_id)
    phases = workflow.get("phases")
    if not isinstance(phases, list):
        return None
    return {"phases": phases, "current_phase_id": workflow.get("current_phase_id")}


def events_for(
    ctx: RepoHandlerContext,
    *,
    run_id: str | None = None,
    job_id: str | None = None,
    session_id: str | None = None,
    artifact_id: str | None = None,
    limit: int = 50,
) -> list[dict[str, Any]]:
    where = "e.repository_id = %s"
    args: list[Any] = [ctx.repository_id]
    if run_id is not None:
        where += " AND e.run_id = %s"
        args.append(run_id)
    if job_id is not None:
        where += " AND e.job_id = %s"
        args.append(job_id)
    if session_id is not None:
        where += " AND e.actor_session_id = %s"
        args.append(session_id)
    if artifact_id is not None:
        where += " AND e.artifact_id = %s"
        args.append(artifact_id)
    args.append(limit)
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT e.event_id, e.run_id, e.event_type, e.actor_session_id,
                   e.job_id, e.message_id, e.artifact_id, e.lease_id,
                   e.payload_json, e.created_at
            FROM striatumd.events e
            WHERE {where}
            ORDER BY e.event_id DESC
            LIMIT %s
            """,
            tuple(args),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    return list(reversed(rows))


def jobs_for_run(ctx: RepoHandlerContext, *, run_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT j.*
            FROM striatumd.jobs j
            WHERE j.repository_id = %s AND j.run_id = %s
            ORDER BY j.created_at, j.workflow_job_id, j.attempt
            """,
            (ctx.repository_id, run_id),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def evidence_snapshot(ctx: RepoHandlerContext, *, run_id: str) -> dict[str, Any]:
    workflow = workflow_for_run(ctx, run_id)
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT r.run_id, r.branch_name, r.state, w.workflow_id, w.workflow_version
            FROM striatumd.runs r
            JOIN striatumd.workflow_snapshots w
              ON w.repository_id = r.repository_id
             AND w.workflow_snapshot_id = r.workflow_snapshot_id
            WHERE r.repository_id = %s AND r.run_id = %s
            """,
            (ctx.repository_id, run_id),
        )
        run = cur.fetchone()
    if run is None:
        raise RpcError("not_found", f"run not found: {run_id}")
    return {
        "schema_version": "striatum.evidence.v1",
        "exported_at": ctx.now(),
        "workflow": {"workflow_id": run["workflow_id"], "workflow_version": run["workflow_version"]},
        "run": {"run_id": run["run_id"], "branch_name": run["branch_name"], "state": run["state"]},
        "jobs": evidence_job_summaries(ctx, run_id=run_id, workflow=workflow),
        "artifacts": evidence_artifact_summaries(ctx, run_id=run_id, workflow=workflow),
        "sessions": evidence_session_summaries(ctx, run_id=run_id),
        "verdicts": verdict_rows(ctx, run_id=run_id),
        "blockers": blocker_rows(ctx, run_id=run_id),
        "blocked_downstream_jobs": blocked_downstream_jobs(ctx, run_id=run_id),
    }


def evidence_job_summaries(
    ctx: RepoHandlerContext, *, run_id: str, workflow: Mapping[str, Any]
) -> list[dict[str, Any]]:
    _ = workflow
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT j.job_id, j.workflow_job_id, j.state, j.role_id,
                   j.lane_selector_json, j.attempt, j.max_attempts, j.job_type,
                   j.expected_artifacts_json
            FROM striatumd.jobs j
            WHERE j.repository_id = %s AND j.run_id = %s
            ORDER BY j.created_at, j.workflow_job_id, j.attempt
            """,
            (ctx.repository_id, run_id),
        )
        rows = cur.fetchall()
    items = []
    for row in rows:
        item = row_to_json(row)
        item["lane_id"] = parse_json_object(row.get("lane_selector_json")).get("lane_id")
        item["expected_artifacts"] = parse_json_list(row.get("expected_artifacts_json"))
        item.pop("lane_selector_json", None)
        item.pop("expected_artifacts_json", None)
        items.append(item)
    return items


def evidence_artifact_summaries(
    ctx: RepoHandlerContext, *, run_id: str, workflow: Mapping[str, Any]
) -> list[dict[str, Any]]:
    _ = workflow
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT a.artifact_id, a.job_id, a.session_id, a.logical_name,
                   a.artifact_kind, a.repo_path, a.content_sha256, a.size_bytes,
                   a.created_at, a.author_line, j.workflow_job_id, j.role_id,
                   j.lane_selector_json, s.lane_id, s.ordinal, s.operator_label
            FROM striatumd.artifacts a
            LEFT JOIN striatumd.jobs j
              ON j.repository_id = a.repository_id
             AND j.job_id = a.job_id
            LEFT JOIN striatumd.sessions s
              ON s.repository_id = a.repository_id
             AND s.session_id = a.session_id
            WHERE a.repository_id = %s AND a.run_id = %s
            ORDER BY a.created_at, a.artifact_id
            """,
            (ctx.repository_id, run_id),
        )
        rows = cur.fetchall()
    items = []
    for row in rows:
        item = row_to_json(row)
        item["author"] = {
            "role_id": item.get("role_id"),
            "lane_id": item.get("lane_id"),
            "ordinal": item.get("ordinal"),
            "workflow_job_id": item.get("workflow_job_id"),
            "author_line": item.get("author_line"),
            "operator_label": item.get("operator_label"),
        }
        item.pop("lane_selector_json", None)
        item.pop("role_id", None)
        item.pop("lane_id", None)
        item.pop("ordinal", None)
        item.pop("workflow_job_id", None)
        item.pop("operator_label", None)
        items.append(item)
    return items


def evidence_session_summaries(ctx: RepoHandlerContext, *, run_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT s.session_id, s.role_id, s.lane_id, s.slug, s.ordinal,
                   s.state, s.closed_at, s.close_reason, s.operator_label
            FROM striatumd.sessions s
            WHERE s.repository_id = %s AND s.run_id = %s
            ORDER BY s.registered_at, s.session_id
            """,
            (ctx.repository_id, run_id),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def verdict_rows(ctx: RepoHandlerContext, *, run_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT v.verdict_id, v.job_id, v.session_id, v.verdict,
                   v.findings_artifact_id, v.rationale, v.posture, v.created_at
            FROM striatumd.verdicts v
            WHERE v.repository_id = %s AND v.run_id = %s
            ORDER BY v.created_at, v.verdict_id
            """,
            (ctx.repository_id, run_id),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def blocker_rows(ctx: RepoHandlerContext, *, run_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT b.blocker_id, b.job_id, b.session_id, b.severity,
                   b.blocker_kind, b.description, b.state, b.created_at
            FROM striatumd.blockers b
            WHERE b.repository_id = %s AND b.run_id = %s
            ORDER BY b.created_at, b.blocker_id
            """,
            (ctx.repository_id, run_id),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def latest_verdict_for_job(ctx: RepoHandlerContext, *, job_id: str) -> dict[str, Any] | None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT v.*
            FROM striatumd.verdicts v
            WHERE v.repository_id = %s AND v.job_id = %s
            ORDER BY v.created_at DESC, v.verdict_id DESC
            LIMIT 1
            """,
            (ctx.repository_id, job_id),
        )
        row = cur.fetchone()
    return row_to_json(row) if row is not None else None


def _override_verdict_payloads(ctx: RepoHandlerContext, *, run_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT e.event_id, e.payload_json, e.created_at
            FROM striatumd.events e
            WHERE e.repository_id = %s
              AND e.run_id = %s
              AND e.event_type = 'verdict.overridden'
            ORDER BY e.event_id
            """,
            (ctx.repository_id, run_id),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    payloads: list[dict[str, Any]] = []
    for row in rows:
        payload = parse_json_object(row.get("payload_json"))
        verdict_id = payload.get("verdict_id")
        if isinstance(verdict_id, str) and verdict_id and not payload.get("rationale"):
            rationale = _verdict_rationale(ctx, verdict_id=verdict_id)
            if rationale:
                payload["rationale"] = rationale
        payload["event_id"] = row.get("event_id")
        payload["created_at"] = row.get("created_at")
        payloads.append(payload)
    return payloads


def _verdict_rationale(ctx: RepoHandlerContext, *, verdict_id: str) -> str | None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT v.rationale
            FROM striatumd.verdicts v
            WHERE v.repository_id = %s AND v.verdict_id = %s
            """,
            (ctx.repository_id, verdict_id),
        )
        row = cur.fetchone()
    if row is None or not row.get("rationale"):
        return None
    return str(row["rationale"])


def dashboard_payload(ctx: RepoHandlerContext, *, run_id: str) -> dict[str, Any]:
    if not run_exists(ctx, run_id):
        raise RpcError("not_found", f"unknown run_id {run_id}")
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT r.*
            FROM striatumd.runs r
            WHERE r.repository_id = %s AND r.run_id = %s
            """,
            (ctx.repository_id, run_id),
        )
        run = row_to_json(cur.fetchone())
    events = events_for(ctx, run_id=run_id, limit=30)
    verdict_counts = Counter(row["verdict"] for row in verdict_rows(ctx, run_id=run_id))
    posture_counts = Counter(row["posture"] for row in verdict_rows(ctx, run_id=run_id))
    override_verdicts = _override_verdict_payloads(ctx, run_id=run_id)
    override_verdict_counts = Counter(
        str(row["verdict"])
        for row in override_verdicts
        if row.get("verdict")
    )
    return {
        "run": run,
        "status": status_payload(ctx, run_id=run_id),
        "events": events,
        "verdict_counts": dict(verdict_counts),
        "posture_counts": dict(posture_counts),
        "updated_at": ctx.now(),
        "workflow": workflow_for_run(ctx, run_id),
        "node_states": {job["workflow_job_id"]: job["state"] for job in jobs_for_run(ctx, run_id=run_id)},
        "override_verdict_counts": dict(override_verdict_counts),
        "override_verdicts": override_verdicts,
    }
