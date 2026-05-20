"""PG-backed escalation projection over blocker rows."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, _jsonb, transaction
from striatum.daemon_pg.handlers.reads._registry import register_pg_handler
from striatum.daemon_pg.handlers.reads._sql import json_value, optional_text, positive_limit
from striatum.daemon_rpc.envelope import RpcError
from striatum.errors import InvalidTransitionError
from striatum.escalations import ESCALATION_BLOCKER_KINDS


@register_pg_handler("escalation.list", read_only=True)
def list_escalations(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = optional_text(params, "run_id")
    state = optional_text(params, "state") or "open"
    if state not in {"open", "resolved", "canceled", "all"}:
        raise RpcError("schema_invalid", "state must be open, resolved, canceled, or all")
    limit = positive_limit(params, default=100)

    filters = ["b.repository_id = %s", _escalation_predicate()]
    values: list[Any] = [ctx.repository_id]
    if run_id is not None:
        filters.append("b.run_id = %s")
        values.append(run_id)
    if state != "all":
        filters.append("b.state = %s")
        values.append(state)
    values.append(limit)

    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT b.blocker_id AS escalation_id,
                   b.blocker_id,
                   b.run_id,
                   b.job_id,
                   j.workflow_job_id,
                   b.session_id,
                   s.role_id AS session_role_id,
                   s.lane_id AS session_lane_id,
                   b.severity,
                   b.blocker_kind AS class,
                   b.description,
                   b.state,
                   b.created_at,
                   b.resolved_at,
                   b.payload_json,
                   a.artifact_id AS linked_artifact_id,
                   a.repo_path AS linked_repo_path,
                   a.content_sha256 AS linked_content_sha256,
                   e.state AS inbox_state,
                   e.viewed_at,
                   e.decision_artifact_id,
                   e.resolution_note
              FROM striatumd.blockers b
              LEFT JOIN striatumd.jobs j
                ON j.repository_id = b.repository_id
               AND j.job_id = b.job_id
              LEFT JOIN striatumd.sessions s
                ON s.repository_id = b.repository_id
               AND s.session_id = b.session_id
              LEFT JOIN striatumd.artifacts a
                ON a.repository_id = b.repository_id
               AND a.artifact_id = b.payload_json #>> '{{escalation_artifact,artifact_id}}'
              LEFT JOIN striatumd.escalation_inbox e
                ON e.repository_id = b.repository_id
               AND e.escalation_id = b.blocker_id
             WHERE {" AND ".join(filters)}
             ORDER BY b.created_at ASC, b.blocker_id ASC
             LIMIT %s
            """,
            tuple(values),
        )
        rows = [_project_row(row) for row in cur.fetchall()]

    return {
        "repository_id": ctx.repository_id,
        "run_id": run_id,
        "state": state,
        "count": len(rows),
        "escalations": rows,
    }


@register_pg_handler("escalation.show", read_only=False)
def show_escalation(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    escalation_id = _required_escalation_id(params)
    with transaction(ctx):
        now = ctx.now()
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.escalation_inbox
                   SET state = 'viewed', viewed_at = %s
                 WHERE repository_id = %s
                   AND escalation_id = %s
                   AND state = 'pending'
                """,
                (now, ctx.repository_id, escalation_id),
            )
    return {"escalation": _load_escalation(ctx, escalation_id=escalation_id)}


@register_pg_handler("escalation.resolve", read_only=False)
def resolve_escalation(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    escalation_id = _required_escalation_id(params)
    decision_id = optional_text(params, "decision_id")
    resolution_note = optional_text(params, "resolution_note")
    with transaction(ctx):
        blocker = _load_blocker_for_update(ctx, escalation_id=escalation_id)
        if blocker["state"] != "open":
            raise InvalidTransitionError("escalation is not open")
        now = ctx.now()
        decision_artifact_id = None
        if decision_id is not None:
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
                    (ctx.repository_id, str(blocker["run_id"]), decision_id),
                )
                row = cur.fetchone()
            if row is None:
                from striatum.errors import NotFoundError
                raise NotFoundError(
                    f"decision artifact for decision_id={decision_id!r} not found in run"
                )
            if row["job_id"] is not None or row["session_id"] is not None:
                raise InvalidTransitionError(
                    "decision artifact must be run-level (no job or session binding)"
                )
            decision_artifact_id = str(row["artifact_id"])

        payload = dict(blocker.get("payload_json") or {})
        resolution_payload: dict[str, Any] = {}
        if decision_id is not None:
            resolution_payload["decision_id"] = decision_id
        if resolution_note is not None:
            resolution_payload["resolution_note"] = resolution_note
        if resolution_payload:
            payload["escalation_resolution"] = resolution_payload
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.blockers
                   SET state = 'resolved',
                       resolved_at = %s,
                       payload_json = %s
                 WHERE repository_id = %s
                   AND blocker_id = %s
                """,
                (now, _jsonb(payload), ctx.repository_id, escalation_id),
            )
            cur.execute(
                """
                UPDATE striatumd.escalation_inbox
                   SET state = 'resolved',
                       resolved_at = %s,
                       decision_artifact_id = %s,
                       resolution_note = %s,
                       payload_json = %s
                 WHERE repository_id = %s
                   AND escalation_id = %s
                """,
                (
                    now,
                    decision_artifact_id,
                    resolution_note,
                    _jsonb(payload),
                    ctx.repository_id,
                    escalation_id,
                ),
            )
        event_payload: dict[str, Any] = {"escalation_id": escalation_id}
        event_payload.update(resolution_payload)
        ctx.append_event(
            run_id=str(blocker["run_id"]),
            event_type="escalation.resolved",
            actor_session_id=None,
            job_id=_optional_str(blocker.get("job_id")),
            payload=event_payload,
        )
    return {
        "status": "resolved",
        "escalation": _load_escalation(ctx, escalation_id=escalation_id),
    }


def _required_escalation_id(params: Mapping[str, Any]) -> str:
    value = params.get("escalation_id", params.get("blocker_id"))
    if not isinstance(value, str) or not value:
        raise RpcError("schema_invalid", "escalation_id must be a non-empty string")
    return value


def _load_escalation(ctx: RepoHandlerContext, *, escalation_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT b.blocker_id AS escalation_id,
                   b.blocker_id,
                   b.run_id,
                   b.job_id,
                   j.workflow_job_id,
                   b.session_id,
                   s.role_id AS session_role_id,
                   s.lane_id AS session_lane_id,
                   b.severity,
                   b.blocker_kind AS class,
                   b.description,
                   b.state,
                   b.created_at,
                   b.resolved_at,
                   b.payload_json,
                   a.artifact_id AS linked_artifact_id,
                   a.repo_path AS linked_repo_path,
                   a.content_sha256 AS linked_content_sha256,
                   e.state AS inbox_state,
                   e.viewed_at,
                   e.decision_artifact_id,
                   e.resolution_note
              FROM striatumd.blockers b
              LEFT JOIN striatumd.jobs j
                ON j.repository_id = b.repository_id
               AND j.job_id = b.job_id
              LEFT JOIN striatumd.sessions s
                ON s.repository_id = b.repository_id
               AND s.session_id = b.session_id
              LEFT JOIN striatumd.artifacts a
                ON a.repository_id = b.repository_id
               AND a.artifact_id = b.payload_json #>> '{{escalation_artifact,artifact_id}}'
              LEFT JOIN striatumd.escalation_inbox e
                ON e.repository_id = b.repository_id
               AND e.escalation_id = b.blocker_id
             WHERE b.repository_id = %s
               AND b.blocker_id = %s
               AND {_escalation_predicate()}
            """,
            (ctx.repository_id, escalation_id),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("not_found", f"escalation not found: {escalation_id}")
    return _project_row(row)


def _load_blocker_for_update(ctx: RepoHandlerContext, *, escalation_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT *
              FROM striatumd.blockers b
             WHERE b.repository_id = %s
               AND b.blocker_id = %s
               AND {_escalation_predicate()}
             FOR UPDATE
            """,
            (ctx.repository_id, escalation_id),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("not_found", f"escalation not found: {escalation_id}")
    return dict(row)


def _escalation_predicate() -> str:
    # The set is constant and not user-provided; inline literals keep callers'
    # parameter lists simple across the list/show/resolve query shapes.
    quoted_kinds = ", ".join(f"'{kind}'" for kind in sorted(ESCALATION_BLOCKER_KINDS))
    return f"(b.severity = 'human_checkpoint' OR b.blocker_kind IN ({quoted_kinds}))"


def _project_row(row: Mapping[str, Any]) -> dict[str, Any]:
    payload = row.get("payload_json") or {}
    source = "human_checkpoint" if row.get("severity") == "human_checkpoint" else "blocker"
    return {
        "escalation_id": str(row["escalation_id"]),
        "blocker_id": str(row["blocker_id"]),
        "run_id": str(row["run_id"]),
        "job_id": _optional_str(row.get("job_id")),
        "workflow_job_id": _optional_str(row.get("workflow_job_id")),
        "session_id": _optional_str(row.get("session_id")),
        "session_role_id": _optional_str(row.get("session_role_id")),
        "session_lane_id": _optional_str(row.get("session_lane_id")),
        "source": source,
        "class": str(row["class"]),
        "severity": str(row["severity"]),
        "description": str(row["description"]),
        "state": str(row["state"]),
        "inbox_state": _optional_str(row.get("inbox_state")),
        "viewed_at": json_value(row.get("viewed_at")),
        "created_at": json_value(row.get("created_at")),
        "resolved_at": json_value(row.get("resolved_at")),
        "decision_artifact_id": _optional_str(row.get("decision_artifact_id")),
        "resolution_note": _optional_str(row.get("resolution_note")),
        "escalation_artifact": _escalation_artifact_summary(payload, row),
        "payload": payload if isinstance(payload, dict) else {},
    }


def _escalation_artifact_summary(
    payload: object,
    row: Mapping[str, Any],
) -> dict[str, Any] | None:
    if not isinstance(payload, Mapping):
        return None
    value = payload.get("escalation_artifact")
    if not isinstance(value, Mapping):
        return None
    required = ("artifact_id", "repo_path", "content_sha256", "linked_at", "link_source")
    if not all(isinstance(value.get(key), str) and value.get(key) for key in required):
        return None
    if (
        row.get("linked_artifact_id") != value["artifact_id"]
        or row.get("linked_repo_path") != value["repo_path"]
        or row.get("linked_content_sha256") != value["content_sha256"]
    ):
        return None
    return {key: str(value[key]) for key in required}


def _optional_str(value: object) -> str | None:
    return None if value is None else str(value)


__all__ = [
    "ESCALATION_BLOCKER_KINDS",
    "list_escalations",
    "show_escalation",
    "resolve_escalation",
]
