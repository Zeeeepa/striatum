"""PG handler for ``evidence.export``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.evidence_presentation import redact_evidence_payload, render_evidence_markdown

from .doctor import doctor_payload
from ._read_model import evidence_snapshot, status_payload
from ._registry import register_pg_handler
from ._sql import require_text, row_to_json, safe_output_path, write_text_export


@register_pg_handler("evidence.export", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = require_text(params, "run_id")
    path_text = require_text(params, "path")
    safe_output_path(ctx, path_text)
    run = _run_row(ctx, run_id)
    status = redact_evidence_payload(status_payload(ctx, run_id=run_id))
    doctor = redact_evidence_payload(doctor_payload(ctx, run_id=run_id))
    snapshot = redact_evidence_payload(evidence_snapshot(ctx, run_id=run_id))
    body = render_evidence_markdown(
        run=run,
        status_payload=status,
        doctor_payload=doctor,
        snapshot=snapshot,
    )
    export = write_text_export(ctx, path_text=path_text, body=body)
    return {"status": "exported", "run_id": run_id, **export}


def _run_row(ctx: RepoHandlerContext, run_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            "SELECT r.* FROM striatumd.runs r WHERE r.repository_id = %s AND r.run_id = %s",
            (ctx.repository_id, run_id),
        )
        row = cur.fetchone()
    if row is None:
        from striatum.daemon_rpc.envelope import RpcError

        raise RpcError("not_found", f"run not found: {run_id}")
    return row_to_json(row)
