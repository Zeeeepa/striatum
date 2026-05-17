"""PG handler for read-only run archive creation."""

from __future__ import annotations

from collections.abc import Mapping
from pathlib import Path
from typing import Any

from striatum.archive import write_run_archive
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_rpc.envelope import RpcError

from ._sql import require_text, row_to_json, safe_output_path
from ._registry import register_pg_handler


@register_pg_handler("archive.create", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = require_text(params, "run_id")
    out_text = require_text(params, "out")
    out = _safe_output_dir(ctx, out_text)
    run = _run_row(ctx, run_id)
    workflow_snapshot = _workflow_snapshot_row(ctx, run_id)
    rows = {kind: _archive_rows(ctx, table=table, run_id=run_id, order_by=order_by)
            for kind, table, order_by in _RUN_SCOPED_TABLES}
    rows["job_dependencies"] = _job_dependency_rows(ctx, run_id=run_id)
    return write_run_archive(
        out,
        repo_root=ctx.repo_root,
        repository_id=ctx.repository_id,
        run_id=run_id,
        run=run,
        workflow_snapshot=workflow_snapshot,
        rows=rows,
    )


def _safe_output_dir(ctx: RepoHandlerContext, path_text: str) -> Path:
    out = safe_output_path(ctx, path_text)
    if out.exists() and not out.is_dir():
        raise RpcError("path_outside_scope", "--out must be a directory", exit_code=8)
    return out


def _run_row(ctx: RepoHandlerContext, run_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT r.*
            FROM striatumd.runs r
            WHERE r.repository_id = %s AND r.run_id = %s
            """,
            (ctx.repository_id, run_id),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("not_found", f"run not found: {run_id}")
    return row_to_json(row)


def _workflow_snapshot_row(ctx: RepoHandlerContext, run_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT w.*
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
    return row_to_json(row)


_RUN_SCOPED_TABLES: tuple[tuple[str, str, str], ...] = (
    ("sessions", "sessions", "registered_at, session_id"),
    ("jobs", "jobs", "created_at, job_id"),
    ("queue_messages", "queue_messages", "created_at, message_id"),
    ("leases", "leases", "acquired_at, lease_id"),
    ("work_packets", "work_packets", "created_at, packet_id"),
    ("artifacts", "artifacts", "created_at, artifact_id"),
    ("verdicts", "verdicts", "created_at, verdict_id"),
    ("blockers", "blockers", "created_at, blocker_id"),
    ("command_requests", "command_requests", "created_at, request_id"),
    ("process_executions", "process_executions", "started_at, process_id"),
    ("job_worktrees", "job_worktrees", "created_at, worktree_id"),
    ("process_supervisors", "process_supervisors", "started_at, supervisor_id"),
    (
        "process_supervisor_pointers",
        "process_supervisor_pointers",
        "updated_at, supervisor_id",
    ),
    ("events", "events", "event_id"),
)


def _archive_rows(
    ctx: RepoHandlerContext,
    *,
    table: str,
    run_id: str,
    order_by: str,
) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT *
            FROM striatumd.{table}
            WHERE repository_id = %s AND run_id = %s
            ORDER BY {order_by}
            """,
            (ctx.repository_id, run_id),
        )
        return [row_to_json(row) for row in cur.fetchall()]


def _job_dependency_rows(ctx: RepoHandlerContext, *, run_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT d.*
            FROM striatumd.job_dependencies d
            JOIN striatumd.jobs j
              ON j.repository_id = d.repository_id
             AND j.job_id = d.job_id
            WHERE d.repository_id = %s AND j.run_id = %s
            ORDER BY d.job_id, d.depends_on_job_id
            """,
            (ctx.repository_id, run_id),
        )
        return [row_to_json(row) for row in cur.fetchall()]
