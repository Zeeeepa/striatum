"""Run summary export helpers for the Striatum CLI."""

from __future__ import annotations

import sqlite3
from pathlib import Path

from striatum.db import (
    insert_event,
    row_by_id,
)
from striatum.git_helpers import current_git_branch
from striatum.primitives import JsonObject, json_loads, sha256_bytes
from striatum.repo_policy import repo_relative_path
from striatum.run_summary_format import (
    format_run_duration as _format_run_duration,
    group_verdicts_by_workflow_job as _group_verdicts_by_workflow_job,
    render_run_summary_markdown,
)

from striatum.cli.evidence import evidence_artifact_summaries, evidence_session_summaries

def run_summary_export(conn: sqlite3.Connection, *, repo: Path, run_id: str, path_text: str) -> JsonObject:
    """Write a compact reader-facing run summary."""
    run = row_by_id(conn, "runs", "run_id", run_id)
    target = repo_relative_path(repo, path_text)
    target.parent.mkdir(parents=True, exist_ok=True)
    summary = run_summary_snapshot(conn, repo=repo, run_id=run_id)
    body = render_run_summary_markdown(run=dict(run), summary=summary)
    target.write_text(body, encoding="utf-8")
    digest = sha256_bytes(body.encode("utf-8"))
    insert_event(
        conn,
        run_id=run_id,
        event_type="run_summary.exported",
        payload={"path": path_text, "sha256": digest},
    )
    return {"status": "exported", "run_id": run_id, "path": path_text, "sha256": digest}


def run_summary_snapshot(conn: sqlite3.Connection, *, repo: Path, run_id: str) -> JsonObject:
    """Return compact run facts for publishable summaries.

    The artifact list carries author identity (loaded from the snapshotted
    workflow), verdicts are grouped by ``workflow_job_id`` so authors can see
    review attempts at a glance, and branch/timing context surfaces what is
    needed to reason about a run after the fact.
    """
    # Look up status/doctor via the package so test monkeypatches against
    # ``striatum.cli`` continue to work.
    from striatum import cli as _cli

    run = row_by_id(conn, "runs", "run_id", run_id)
    snapshot_row = row_by_id(
        conn, "workflow_snapshots", "workflow_snapshot_id", str(run["workflow_snapshot_id"])
    )
    workflow = json_loads(str(snapshot_row["workflow_json"]))
    artifacts = evidence_artifact_summaries(conn, run_id=run_id, workflow=workflow)
    sessions = evidence_session_summaries(conn, run_id=run_id)
    verdicts = conn.execute(
        """
        SELECT v.verdict_id, v.job_id, j.workflow_job_id, v.verdict, v.findings_artifact_id,
               v.created_at, v.posture
        FROM verdicts v
        JOIN jobs j ON j.job_id = v.job_id
        WHERE v.run_id = ?
        ORDER BY v.created_at
        """,
        (run_id,),
    ).fetchall()
    verdict_dicts = [dict(row) for row in verdicts]
    grouped_verdicts = _group_verdicts_by_workflow_job(verdict_dicts)
    blockers = conn.execute(
        """
        SELECT blocker_id, job_id, severity, blocker_kind, state
        FROM blockers
        WHERE run_id = ?
        ORDER BY created_at
        """,
        (run_id,),
    ).fetchall()
    branch_context = {
        "recorded": run["branch_name"],
        "current": current_git_branch(repo),
    }
    branch_context["mismatch"] = (
        branch_context["recorded"] is not None
        and branch_context["current"] is not None
        and branch_context["recorded"] != branch_context["current"]
    )
    timing = {
        "created_at": run["created_at"],
        "started_at": run["started_at"],
        "completed_at": run["completed_at"],
        "duration": _format_run_duration(
            started_at=run["started_at"],
            completed_at=run["completed_at"],
        ),
    }
    return {
        "status": _cli.status(conn, run_id=run_id),
        "doctor": _cli.doctor(conn, repo=repo, run_id=run_id),
        "artifacts": artifacts,
        "sessions": sessions,
        "verdicts": verdict_dicts,
        "verdicts_by_workflow_job": grouped_verdicts,
        "blockers": [dict(row) for row in blockers],
        "branch_context": branch_context,
        "timing": timing,
    }
