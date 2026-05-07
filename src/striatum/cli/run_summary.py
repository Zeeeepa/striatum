"""Run summary export helpers for the Striatum CLI."""

from __future__ import annotations

import sqlite3
from datetime import datetime
from pathlib import Path

from striatum.db import (
    JsonObject,
    insert_event,
    json_loads,
    repo_relative_path,
    row_by_id,
    sha256_bytes,
    utc_now,
)

from striatum.cli.evidence import evidence_artifact_summaries
from striatum.cli.mutations import current_git_branch


def run_summary_export(conn: sqlite3.Connection, *, repo: Path, run_id: str, path_text: str) -> JsonObject:
    """Write a compact human-facing run summary."""
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
    verdicts = conn.execute(
        """
        SELECT v.verdict_id, v.job_id, j.workflow_job_id, v.verdict, v.findings_artifact_id,
               v.created_at
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
        "verdicts": verdict_dicts,
        "verdicts_by_workflow_job": grouped_verdicts,
        "blockers": [dict(row) for row in blockers],
        "branch_context": branch_context,
        "timing": timing,
    }


def _group_verdicts_by_workflow_job(verdicts: list[JsonObject]) -> list[JsonObject]:
    """Group verdicts by workflow_job_id, preserving chronological order.

    Returns one record per review job with the latest verdict, attempt count,
    and the ordered list of verdict values that preceded it. Verdicts arriving
    out of order would be unusual (verdicts has UNIQUE(job_id, session_id)),
    but we still sort by created_at to be defensive.
    """
    groups: dict[str, list[JsonObject]] = {}
    order: list[str] = []
    for verdict in verdicts:
        workflow_job_id = str(verdict["workflow_job_id"])
        if workflow_job_id not in groups:
            groups[workflow_job_id] = []
            order.append(workflow_job_id)
        groups[workflow_job_id].append(verdict)
    grouped: list[JsonObject] = []
    for workflow_job_id in order:
        items = sorted(
            groups[workflow_job_id],
            key=lambda entry: str(entry.get("created_at") or ""),
        )
        latest = items[-1]
        prior_verdicts = [str(item["verdict"]) for item in items[:-1]]
        grouped.append(
            {
                "workflow_job_id": workflow_job_id,
                "attempts": len(items),
                "latest_verdict": latest["verdict"],
                "latest_verdict_id": latest["verdict_id"],
                "prior_verdicts": prior_verdicts,
            }
        )
    return grouped


def _format_run_duration(*, started_at: str | None, completed_at: str | None) -> str | None:
    """Render the wall-clock duration between started_at and completed_at."""
    if started_at is None:
        return None
    end = completed_at if completed_at is not None else utc_now()
    try:
        start_dt = datetime.fromisoformat(started_at.replace("Z", "+00:00"))
        end_dt = datetime.fromisoformat(end.replace("Z", "+00:00"))
    except ValueError:
        return None
    delta = end_dt - start_dt
    total_seconds = int(delta.total_seconds())
    if total_seconds < 0:
        return None
    hours, remainder = divmod(total_seconds, 3600)
    minutes, seconds = divmod(remainder, 60)
    return f"{hours}h {minutes}m {seconds}s"


def render_run_summary_markdown(*, run: JsonObject, summary: JsonObject) -> str:
    """Render a compact run note intended for durable provenance."""
    status_payload = summary["status"]
    doctor_payload = summary["doctor"]
    jobs = status_payload["jobs"]
    artifacts = summary["artifacts"]
    grouped_verdicts = summary.get("verdicts_by_workflow_job", [])
    blockers = summary["blockers"]
    branch_context = summary.get("branch_context", {})
    timing = summary.get("timing", {})
    recorded_branch = branch_context.get("recorded")
    current_branch = branch_context.get("current")
    branch_line = f"Branch: `{recorded_branch}`"
    if branch_context.get("mismatch"):
        branch_line += f" (current: `{current_branch}`) (MISMATCH)"
    elif current_branch is not None and current_branch != recorded_branch:
        # Only one of the two is missing; surface the current branch for context.
        branch_line += f" (current: `{current_branch}`)"
    lines = [
        "# Striatum Run Summary",
        "",
        f"Run ID: `{run['run_id']}`",
        branch_line,
        f"Run state: `{run['state']}`",
        f"Verification: `doctor ok={str(doctor_payload['ok']).lower()}`",
        "",
        "## Timing",
        "",
        f"- Created at: `{timing.get('created_at')}`",
        f"- Started at: `{timing.get('started_at')}`",
        f"- Completed at: `{timing.get('completed_at')}`",
        f"- Duration: `{timing.get('duration')}`",
        "",
        "## Jobs",
        "",
    ]
    if jobs:
        for state, count in sorted(jobs.items()):
            lines.append(f"- `{state}`: {count}")
    else:
        lines.append("- No jobs recorded.")
    lines.extend(["", "## Verdicts", ""])
    if grouped_verdicts:
        for entry in grouped_verdicts:
            attempts = int(entry["attempts"])
            latest = str(entry["latest_verdict"])
            prior = list(entry.get("prior_verdicts") or [])
            line = f"- `{entry['workflow_job_id']}` ({attempts} attempts): `{latest}`"
            if prior:
                # Compress like ``2x needs_revision, 1x reject`` so a long
                # cycle stays readable on one line.
                tally: dict[str, int] = {}
                order: list[str] = []
                for value in prior:
                    if value not in tally:
                        tally[value] = 0
                        order.append(value)
                    tally[value] += 1
                summary_parts = [f"{tally[value]}x `{value}`" for value in order]
                line += f" after {', '.join(summary_parts)}"
            lines.append(line)
    else:
        lines.append("- No verdicts recorded.")
    lines.extend(["", "## Artifacts", ""])
    if artifacts:
        for artifact in artifacts:
            line = (
                f"- `{artifact['artifact_kind']}` `{artifact['logical_name']}`: "
                f"`{artifact['repo_path']}`"
            )
            author = artifact.get("author")
            if isinstance(author, dict):
                author_line = author.get("line")
                # ``line`` already includes the ``author: `` prefix from the
                # identity helper, so we just wrap it in a code span instead
                # of re-prefixing.
                if isinstance(author_line, str) and author_line != "":
                    line += f" - `{author_line}`"
            lines.append(line)
    else:
        lines.append("- No artifacts recorded.")
    lines.extend(["", "## Blockers", ""])
    if blockers:
        for blocker in blockers:
            lines.append(
                f"- `{blocker['state']}` `{blocker['severity']}` "
                f"`{blocker['blocker_kind']}` ({blocker['blocker_id']})"
            )
    else:
        lines.append("- No blockers recorded.")
    lines.extend(["", "## Next Actions", ""])
    next_actions = status_payload["next_actions"]
    if next_actions:
        for action in next_actions:
            lines.append(f"- `{action}`")
    else:
        lines.append("- No deterministic next actions.")
    lines.append("")
    return "\n".join(lines)
