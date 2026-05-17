"""PG handler for ``run.posture_verdicts``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, session_lane_attestation
from striatum.daemon_rpc.envelope import RpcError

from ._registry import register_pg_handler
from ._sql import parse_json_object, require_text, row_to_json


@register_pg_handler("run.posture_verdicts", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = require_text(params, "run_id")
    posture = _require_posture(params)
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT r.run_id, r.state, r.branch_name
            FROM striatumd.runs r
            WHERE r.repository_id = %s AND r.run_id = %s
            """,
            (ctx.repository_id, run_id),
        )
        run_row = cur.fetchone()
        if run_row is None:
            raise RpcError("not_found", f"run not found: {run_id}")
        cur.execute(
            """
            SELECT v.verdict_id, v.verdict, v.rationale, v.created_at,
                   v.job_id, v.findings_artifact_id, v.session_id, v.posture,
                   j.workflow_job_id, j.role_id, j.lane_selector_json,
                   s.slug AS session_slug
            FROM striatumd.verdicts v
            JOIN striatumd.jobs j
              ON j.repository_id = v.repository_id
             AND j.job_id = v.job_id
            JOIN striatumd.sessions s
              ON s.repository_id = v.repository_id
             AND s.session_id = v.session_id
            WHERE v.repository_id = %s AND v.run_id = %s AND v.posture = %s
            ORDER BY v.created_at DESC, v.verdict_id DESC
            """,
            (ctx.repository_id, run_id, posture),
        )
        verdict_rows = [row_to_json(row) for row in cur.fetchall()]
        cur.execute(
            """
            SELECT e.payload_json
            FROM striatumd.events e
            WHERE e.repository_id = %s
              AND e.run_id = %s
              AND e.event_type = 'verdict.overridden'
            """,
            (ctx.repository_id, run_id),
        )
        override_rows = [row_to_json(row) for row in cur.fetchall()]

    override_verdict_ids = _override_verdict_ids(override_rows)
    verdicts = [
        _shape_verdict(ctx, row, override_verdict_ids=override_verdict_ids)
        for row in verdict_rows
    ]
    return {
        "run": row_to_json(run_row),
        "posture": posture,
        "verdicts": verdicts,
        "count": len(verdicts),
    }


def _require_posture(params: Mapping[str, Any]) -> str:
    posture = require_text(params, "posture")
    if any(part in posture for part in ("/", "\x00", "..")):
        raise RpcError("schema_invalid", "posture contains invalid path characters")
    return posture


def _override_verdict_ids(rows: list[dict[str, Any]]) -> set[str]:
    verdict_ids: set[str] = set()
    for row in rows:
        payload = parse_json_object(row.get("payload_json"))
        verdict_id = payload.get("verdict_id")
        if isinstance(verdict_id, str) and verdict_id:
            verdict_ids.add(verdict_id)
    return verdict_ids


def _shape_verdict(
    ctx: RepoHandlerContext,
    row: dict[str, Any],
    *,
    override_verdict_ids: set[str],
) -> dict[str, Any]:
    item = dict(row)
    lane_selector = parse_json_object(item.pop("lane_selector_json", {}))
    item["lane_id"] = lane_selector.get("lane_id")
    provenance = (
        "operator-override"
        if item.get("verdict_id") in override_verdict_ids
        else "natural"
    )
    item["source"] = provenance
    item["provenance"] = provenance
    item["override_rationale"] = (
        item.get("rationale") if provenance == "operator-override" else None
    )
    attestation = session_lane_attestation(
        ctx,
        session_id=str(item["session_id"]) if item.get("session_id") else "",
    )
    item["lane_attestation_chip"] = {
        "state": attestation.get("state"),
        "attested": bool(attestation.get("attested")),
        "reason": attestation.get("reason"),
        "supervisor_id": attestation.get("supervisor_id"),
        "label": attestation.get("state"),
    }
    return item
