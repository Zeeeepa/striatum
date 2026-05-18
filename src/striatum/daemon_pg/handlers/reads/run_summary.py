"""PG handler for ``run.summary``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.run_summary_format import (
    format_run_duration,
    group_verdicts_by_workflow_job,
    render_run_summary_markdown,
)
from striatum.cli.mutations import current_git_branch

from .doctor import doctor_payload
from ._read_model import (
    blocker_rows,
    evidence_artifact_summaries,
    evidence_session_summaries,
    status_payload,
    workflow_for_run,
)
from ._registry import register_pg_handler
from ._sql import require_text, row_to_json, safe_output_path, write_text_export


@register_pg_handler("run.summary", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = require_text(params, "run_id")
    path_text = require_text(params, "path")
    safe_output_path(ctx, path_text)
    run = _run_row(ctx, run_id)
    summary = run_summary_payload(ctx, run_id=run_id, run=run)
    body = render_run_summary_markdown(run=run, summary=summary)
    export = write_text_export(ctx, path_text=path_text, body=body)
    return {"status": "exported", "run_id": run_id, **export}


def run_summary_payload(ctx: RepoHandlerContext, *, run_id: str, run: dict[str, Any] | None = None) -> dict[str, Any]:
    run_row = run or _run_row(ctx, run_id)
    workflow = workflow_for_run(ctx, run_id)
    verdicts = _verdict_rows(ctx, run_id=run_id)
    branch_context = {
        "recorded": run_row.get("branch_name"),
        "current": current_git_branch(ctx.repo_root),
    }
    branch_context["mismatch"] = (
        branch_context["recorded"] is not None
        and branch_context["current"] is not None
        and branch_context["recorded"] != branch_context["current"]
    )
    return {
        "status": status_payload(ctx, run_id=run_id),
        "doctor": doctor_payload(ctx, run_id=run_id),
        "artifacts": evidence_artifact_summaries(ctx, run_id=run_id, workflow=workflow),
        "sessions": evidence_session_summaries(ctx, run_id=run_id),
        "verdicts": verdicts,
        "verdicts_by_workflow_job": group_verdicts_by_workflow_job(verdicts),
        "blockers": blocker_rows(ctx, run_id=run_id),
        "branch_context": branch_context,
        "timing": {
            "created_at": run_row.get("created_at"),
            "started_at": run_row.get("started_at"),
            "completed_at": run_row.get("completed_at"),
            "duration": format_run_duration(
                started_at=str(run_row["started_at"]) if run_row.get("started_at") is not None else None,
                completed_at=str(run_row["completed_at"]) if run_row.get("completed_at") is not None else None,
            ),
        },
    }


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


def _verdict_rows(ctx: RepoHandlerContext, *, run_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT v.verdict_id, v.job_id, j.workflow_job_id, v.verdict,
                   v.findings_artifact_id, v.created_at, v.posture
            FROM striatumd.verdicts v
            JOIN striatumd.jobs j
              ON j.repository_id = v.repository_id
             AND j.job_id = v.job_id
            WHERE v.repository_id = %s AND v.run_id = %s
            ORDER BY v.created_at, v.verdict_id
            """,
            (ctx.repository_id, run_id),
        )
        return [row_to_json(row) for row in cur.fetchall()]
