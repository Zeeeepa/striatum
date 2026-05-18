"""Substrate-neutral run-summary formatting helpers."""

from __future__ import annotations

from datetime import datetime

from striatum.primitives import JsonObject, utc_now


def group_verdicts_by_workflow_job(verdicts: list[JsonObject]) -> list[JsonObject]:
    """Group verdicts by workflow_job_id, preserving chronological order."""
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
                "latest_posture": latest.get("posture", "neutral"),
                "prior_verdicts": prior_verdicts,
            }
        )
    return grouped


def format_run_duration(*, started_at: str | None, completed_at: str | None) -> str | None:
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
    sessions = summary.get("sessions", [])
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
    has_non_neutral_posture = any(
        str(entry.get("latest_posture") or "neutral") != "neutral"
        for entry in grouped_verdicts
    )
    if grouped_verdicts:
        for entry in grouped_verdicts:
            attempts = int(entry["attempts"])
            latest = str(entry["latest_verdict"])
            prior = list(entry.get("prior_verdicts") or [])
            line = f"- `{entry['workflow_job_id']}` ({attempts} attempts): `{latest}`"
            if has_non_neutral_posture:
                posture = str(entry.get("latest_posture") or "neutral")
                line += f" [posture: `{posture}`]"
            if prior:
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
                if isinstance(author_line, str) and author_line != "":
                    line += f" - `{author_line}`"
                elif "actual_author_line" in author:
                    line += " - `author: <missing>`"
            lines.append(line)
    else:
        lines.append("- No artifacts recorded.")
    lines.extend(["", "## Sessions", ""])
    if sessions:
        for session in sessions:
            slug = session.get("slug")
            state = session.get("state")
            line = f"- `{slug}` `{state}`"
            closed_at = session.get("closed_at")
            if isinstance(closed_at, str) and closed_at != "":
                line += f" (closed_at: `{closed_at}`)"
            close_reason = session.get("close_reason")
            if isinstance(close_reason, str) and close_reason != "":
                line += f" reason: `{close_reason}`"
            non_fresh_reason = session.get("non_fresh_reason")
            if isinstance(non_fresh_reason, str) and non_fresh_reason != "":
                line += f" non_fresh: `{non_fresh_reason}`"
            lines.append(line)
    else:
        lines.append("- No sessions recorded.")
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
