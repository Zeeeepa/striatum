"""PG handler for ``doctor``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_rpc.envelope import RpcError

from ._registry import register_pg_handler
from ._sql import optional_text


@register_pg_handler("doctor", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = optional_text(params, "run_id")
    verbose = bool(params.get("verbose", False))
    return doctor_payload(ctx, run_id=run_id, verbose=verbose)


def doctor_payload(ctx: RepoHandlerContext, *, run_id: str | None, verbose: bool = False) -> dict[str, Any]:
    problems: list[str] = []
    records: list[dict[str, Any]] = []
    if run_id is not None:
        with ctx.cursor() as cur:
            cur.execute(
                "SELECT 1 FROM striatumd.runs r WHERE r.repository_id = %s AND r.run_id = %s",
                (ctx.repository_id, run_id),
            )
            if cur.fetchone() is None:
                raise RpcError("not_found", f"run not found: {run_id}")
            cur.execute(
                """
                SELECT j.job_id, j.workflow_job_id
                FROM striatumd.jobs j
                WHERE j.repository_id = %s AND j.run_id = %s
                  AND j.state = 'completed'
                  AND NOT EXISTS (
                    SELECT 1
                    FROM striatumd.artifacts a
                    WHERE a.repository_id = j.repository_id
                      AND a.job_id = j.job_id
                  )
                  AND j.expected_artifacts_json <> '[]'::jsonb
                """,
                (ctx.repository_id, run_id),
            )
            for row in cur.fetchall():
                problems.append("completed_job_missing_expected_artifact")
                records.append(
                    {
                        "check": "completed_job_missing_expected_artifact",
                        "severity": "warning",
                        "job_id": row["job_id"],
                        "workflow_job_id": row["workflow_job_id"],
                    }
                )
    result: dict[str, Any] = {"ok": not problems, "schema_version": 5, "problems": problems}
    if verbose:
        result["problem_records"] = records
    return result
