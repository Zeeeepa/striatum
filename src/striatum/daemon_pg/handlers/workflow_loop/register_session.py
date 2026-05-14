"""PG-backed ``session.register`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    _json_loads,
    _jsonb,
    session_lane_attestation,
    transaction,
    validate_operator_label,
    workflow_declares_fresh_reviewer,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.errors import InvalidTransitionError


@register_pg_handler("session.register")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = str(params["run_id"])
    role = str(params["role"])
    lane = str(params["lane"])
    capabilities = list(params.get("capabilities") or params.get("capability") or [])
    fresh = bool(params.get("fresh", False))
    parent_session_id = params.get("parent_session_id")
    force_non_fresh = bool(params.get("force_non_fresh", False))
    non_fresh_reason = params.get("non_fresh_reason") or params.get("reason")
    operator_label = params.get("operator_label")
    with transaction(ctx):
        run = ctx.row_by_id("runs", "run_id", run_id, for_update=True)
        snapshot = ctx.row_by_id("workflow_snapshots", "workflow_snapshot_id", str(run["workflow_snapshot_id"]))
        workflow = _json_loads(snapshot["workflow_json"])
        roles = workflow.get("roles", {})
        lanes = workflow.get("lanes", {})
        if not isinstance(roles, dict) or role not in roles:
            raise InvalidTransitionError(f"unknown role {role!r} for run")
        if not isinstance(lanes, dict) or lane not in lanes:
            raise InvalidTransitionError(f"unknown lane {lane!r} for run")
        recorded_label = None
        if operator_label is not None:
            try:
                recorded_label = validate_operator_label(str(operator_label), workflow=workflow)
            except ValueError as exc:
                raise InvalidTransitionError(str(exc)) from exc
        recorded_reason = None
        if role == "reviewer" and workflow_declares_fresh_reviewer(workflow):
            with ctx.cursor() as cur:
                cur.execute(
                    """
                    SELECT 1 FROM striatumd.sessions
                    WHERE repository_id = %s AND run_id = %s AND role_id = 'author' AND state = 'active'
                    LIMIT 1
                    """,
                    (ctx.repository_id, run_id),
                )
                author_active = cur.fetchone()
            if author_active is not None:
                if not force_non_fresh:
                    raise InvalidTransitionError(
                        "workflow declares reviewer_context_policy: fresh and an active author session already exists on this run; pass --force-non-fresh --reason \"...\" to register a non-fresh reviewer explicitly"
                    )
                if non_fresh_reason is None or not str(non_fresh_reason).strip():
                    raise InvalidTransitionError("--force-non-fresh requires a non-empty --reason")
                recorded_reason = str(non_fresh_reason).strip()
        with ctx.cursor() as cur:
            cur.execute(
                """
                SELECT ordinal
                FROM striatumd.sessions
                WHERE repository_id = %s AND run_id = %s AND role_id = %s AND lane_id = %s
                FOR UPDATE
                """,
                (ctx.repository_id, run_id, role, lane),
            )
            rows = cur.fetchall()
            ordinal = max((int(row["ordinal"]) for row in rows), default=0) + 1
        session_id = ctx.new_id("sess")
        slug = f"{role}-{lane}-{ordinal}"
        now = ctx.now()
        with ctx.cursor() as cur:
            cur.execute(
                """
                INSERT INTO striatumd.sessions (
                  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
                  capabilities_json, parent_session_id, fresh_context, state,
                  registered_at, last_heartbeat_at, non_fresh_reason, operator_label
                )
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, 'active', %s, %s, %s, %s)
                """,
                (
                    ctx.repository_id,
                    session_id,
                    run_id,
                    role,
                    lane,
                    slug,
                    ordinal,
                    _jsonb(capabilities),
                    parent_session_id,
                    fresh,
                    now,
                    now,
                    recorded_reason,
                    recorded_label,
                ),
            )
        payload: dict[str, Any] = {"role": role, "lane": lane, "slug": slug}
        if recorded_reason is not None:
            payload["non_fresh_reason"] = recorded_reason
        if recorded_label is not None:
            payload["operator_label"] = recorded_label
        ctx.append_event(run_id=run_id, event_type="session.registered", actor_session_id=session_id, payload=payload)
        attestation = session_lane_attestation(ctx, session_id=session_id)
        return {
            "session_id": session_id,
            "slug": slug,
            "lane_attestation": attestation["state"],
            "lane_attestation_reason": attestation["reason"],
            "operator_label": recorded_label,
            "supervise_hint": f"striatum supervise start --session-id {session_id}",
        }
