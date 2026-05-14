"""Compact terminal dashboard over the existing SQLite state.

The dashboard is a thin, dependency-free renderer over the same data exposed
by ``striatum status`` and the events table. It uses raw ANSI escape codes
to clear the screen and redraw on a fixed cadence; it deliberately avoids
``curses`` so that it works the same on every terminal a coding agent might
run in.
"""

from __future__ import annotations

import json
import shutil
import sys
import time
from pathlib import Path
from typing import Any, Iterable, Mapping, Sequence, TextIO

from striatum.db import (
    connect,
    ensure_initialized,
    json_loads,
    utc_now,
)
from striatum.errors import NotFoundError

# Canonical ordering for compact summary panels.
RUN_STATE_ORDER: tuple[str, ...] = (
    "needs_branch_confirmation",
    "ready",
    "running",
    "blocked",
    "completed",
    "failed",
    "canceled",
    "paused",
)

JOB_STATE_ORDER: tuple[str, ...] = (
    "blocked",
    "queued",
    "ready",
    "claimed",
    "running",
    "waiting_human",
    "completed",
    "failed",
    "canceled",
    "skipped",
    "stale_lease",
)

VERDICT_ORDER: tuple[str, ...] = (
    "accept",
    "accept_with_findings",
    "needs_revision",
    "reject",
)

BLOCKER_SEVERITY_ORDER: tuple[str, ...] = (
    "human_checkpoint",
    "blocked",
)

ATTESTATION_REASON_ORDER: tuple[str, ...] = (
    "no_attached_supervisor",
    "pid_gone",
    "pid_identity_mismatch",
    "lane_command_mismatch",
    "session_missing",
    "session_mismatch",
    "run_mismatch",
)

LANE_EVIDENCE_NOT_YET_CORRELATED = "not_yet_correlated"

# ANSI: clear entire screen + home the cursor.
ANSI_CLEAR_HOME = "\x1b[2J\x1b[H"
# ANSI: hide / show cursor for cleaner refreshes.
ANSI_HIDE_CURSOR = "\x1b[?25l"
ANSI_SHOW_CURSOR = "\x1b[?25h"


def gather_payload(repo: Path, *, run_id: str) -> dict[str, Any]:
    """Collect status + recent events for the dashboard.

    Returns a dict shaped for ``render_frame``. Raises ``StriatumError``
    (subclass) if the run is unknown or the state is unreadable.
    """
    from striatum.cli import recent_events_for_run, status as status_command
    from striatum.workflow import compute_node_states

    ensure_initialized(repo)
    with connect(repo) as conn:
        # Resolve the run early so an unknown id fails with the standard exit code.
        run_row = conn.execute(
            "SELECT run_id, state, branch_name, workflow_snapshot_id FROM runs WHERE run_id = ?",
            (run_id,),
        ).fetchone()
        if run_row is None:
            raise NotFoundError(f"unknown run_id {run_id!r}")
        status_payload = status_command(conn, run_id=run_id)
        blocker_payload_rows = conn.execute(
            """
            SELECT blocker_id, created_at, payload_json
            FROM blockers
            WHERE run_id = ? AND state = 'open'
            """,
            (run_id,),
        ).fetchall()
        blocker_payloads = {
            str(row["blocker_id"]): {
                "created_at": row["created_at"],
                "payload_json": row["payload_json"],
            }
            for row in blocker_payload_rows
        }
        for key in ("open_blockers", "human_checkpoints"):
            for blocker in status_payload.get(key, []):
                if isinstance(blocker, dict):
                    blocker.update(blocker_payloads.get(str(blocker.get("blocker_id")), {}))
        events = recent_events_for_run(conn, run_id=run_id, limit=10)
        for event in events:
            if not isinstance(event, dict) or event.get("event_type") != "verdict.overridden":
                continue
            payload = _json_object(event.get("payload_json"))
            verdict_id = payload.get("verdict_id")
            if verdict_id and not payload.get("rationale"):
                rationale_row = conn.execute(
                    "SELECT rationale FROM verdicts WHERE verdict_id = ?",
                    (str(verdict_id),),
                ).fetchone()
                if rationale_row is not None and rationale_row["rationale"]:
                    payload["rationale"] = rationale_row["rationale"]
                    event["payload_json"] = json.dumps(payload)
        verdict_rows = conn.execute(
            """
            SELECT verdict, COUNT(*) AS count
            FROM verdicts
            WHERE run_id = ?
            GROUP BY verdict
            """,
            (run_id,),
        ).fetchall()
        verdict_counts = {str(row["verdict"]): int(row["count"]) for row in verdict_rows}
        override_rows = conn.execute(
            """
            SELECT payload_json
            FROM events
            WHERE run_id = ? AND event_type = 'verdict.overridden'
            ORDER BY event_id
            """,
            (run_id,),
        ).fetchall()
        override_verdict_counts: dict[str, int] = {}
        override_verdicts: list[dict[str, Any]] = []
        for row in override_rows:
            payload = _json_object(row["payload_json"])
            verdict = str(payload.get("verdict") or "")
            if verdict:
                override_verdict_counts[verdict] = override_verdict_counts.get(verdict, 0) + 1
            verdict_id = payload.get("verdict_id")
            if verdict_id and not payload.get("rationale"):
                rationale_row = conn.execute(
                    "SELECT rationale FROM verdicts WHERE verdict_id = ?",
                    (str(verdict_id),),
                ).fetchone()
                if rationale_row is not None and rationale_row["rationale"]:
                    payload["rationale"] = rationale_row["rationale"]
            override_verdicts.append(payload)
        # RFC 0018 step 3 (V1.5): per-posture counts for the dashboard
        # verdicts panel. Rendered only when at least one non-neutral
        # posture exists in the run.
        posture_rows = conn.execute(
            """
            SELECT posture, COUNT(*) AS count
            FROM verdicts
            WHERE run_id = ?
            GROUP BY posture
            """,
            (run_id,),
        ).fetchall()
        posture_counts = {str(row["posture"]): int(row["count"]) for row in posture_rows}
        # RFC 0016 V1: workflow snapshot + node_states for the optional
        # graph panel.
        snapshot_row = conn.execute(
            "SELECT workflow_json FROM workflow_snapshots WHERE workflow_snapshot_id = ?",
            (str(run_row["workflow_snapshot_id"]),),
        ).fetchone()
        workflow_payload: dict[str, Any] = {}
        if snapshot_row is not None:
            workflow_payload = json_loads(str(snapshot_row["workflow_json"]))
        node_states = compute_node_states(conn, run_id=run_id)

    run = {
        "run_id": run_row["run_id"],
        "state": run_row["state"],
        "branch_name": run_row["branch_name"],
    }
    return {
        "run": run,
        "status": status_payload,
        "events": events,
        "verdict_counts": verdict_counts,
        "posture_counts": posture_counts,
        "updated_at": utc_now(),
        "workflow": workflow_payload,
        "node_states": node_states,
        "override_verdict_counts": override_verdict_counts,
        "override_verdicts": override_verdicts,
    }


def render_frame(
    payload: Mapping[str, Any],
    *,
    terminal_width: int,
    terminal_height: int = 30,
    graph: bool | None = None,
    graph_only: bool = False,
    graph_style: str = "auto",
    graph_no_cycles: bool = False,
    graph_orient: str = "tb",
    color: bool = False,
) -> str:
    """Render one dashboard frame as plain text.

    Pure: takes a payload from ``gather_payload`` and a terminal width and
    returns the rendered string. Easy to unit test.

    RFC 0016 V1: when ``graph`` is True (or auto with a wide-enough TTY),
    appends the graph panel after the events panel. ``graph=False``
    forces the panel off and produces the same byte-for-byte output as
    earlier versions of the dashboard.
    """
    width = max(60, int(terminal_width))
    run = _as_dict(payload.get("run"))
    status_payload = _as_dict(payload.get("status"))
    events = _as_list(payload.get("events"))
    updated_at = str(payload.get("updated_at") or "")
    refresh_seconds = payload.get("refresh_seconds")

    run_id = str(run.get("run_id") or "<unknown>")
    branch = str(run.get("branch_name") or "<unconfirmed>")
    state = _run_state_chip(str(run.get("state") or "unknown"), paused_at=run.get("paused_at"))

    lines: list[str] = []

    header = f"striatum dashboard - run {run_id} - branch {branch} - state {state}"
    lines.append(_truncate(header, width))
    refresh_text = (
        f"refresh {_format_refresh(refresh_seconds)}s   ctrl-c to quit"
        if refresh_seconds is not None
        else "ctrl-c to quit"
    )
    lines.append(_truncate(f"updated {updated_at}   {refresh_text}", width))
    lines.append("")

    # Two-column layout: jobs/verdicts/blockers on the left, claimable/next on right.
    left_col_width = max(34, width // 2 - 2)
    right_col_width = max(30, width - left_col_width - 2)

    job_counts = _as_dict(status_payload.get("jobs"))
    explicit_verdicts = payload.get("verdict_counts")
    if isinstance(explicit_verdicts, Mapping):
        verdict_counts = {key: int(explicit_verdicts.get(key, 0) or 0) for key in VERDICT_ORDER}
    else:
        verdict_counts = _verdict_counts(
            _as_list(status_payload.get("latest_non_accepting_review_verdicts"))
        )
    blocker_counts = _blocker_counts(
        _as_list(status_payload.get("open_blockers")),
        _as_list(status_payload.get("human_checkpoints")),
    )
    phases = _as_list(status_payload.get("phases"))
    claimable = _as_list(status_payload.get("claimable_jobs"))
    next_actions = _as_list(status_payload.get("next_actions"))

    workflow = payload.get("workflow")
    node_states = payload.get("node_states") or {}
    has_edges = bool(
        isinstance(workflow, Mapping)
        and isinstance(workflow.get("edges"), list)
        and workflow.get("edges")
    )
    if graph is None:
        # RFC 0016 § "auto" rule: graph rendered when terminal is wide
        # AND tall AND the workflow has at least one edge.
        graph_active = (
            width >= 100 and int(terminal_height) >= 30 and has_edges
        )
    else:
        graph_active = bool(graph) and has_edges

    if not graph_only:
        left_lines = _render_left_column(job_counts)
        right_lines = _render_right_column(
            verdict_counts, blocker_counts,
            posture_counts=payload.get("posture_counts"),
            override_verdict_counts=_as_dict(payload.get("override_verdict_counts")),
        )
        for combined in _zip_columns(left_lines, right_lines, left_col_width, right_col_width):
            lines.append(combined)
        lines.append("")

        lines.extend(_render_phases(phases, width))
        if phases:
            lines.append("")

        session_lines = _render_sessions(_as_list(status_payload.get("sessions")), width)
        if session_lines:
            lines.extend(session_lines)
            lines.append("")

        blocker_lines = _render_blocker_triage(
            _as_list(status_payload.get("open_blockers")),
            width,
        )
        if blocker_lines:
            lines.extend(blocker_lines)
            lines.append("")

        override_lines = _render_verdict_overrides(
            _as_list(payload.get("override_verdicts")),
            width,
        )
        if override_lines:
            lines.extend(override_lines)
            lines.append("")

        claim_lines = _render_claimable(claimable)
        next_lines = _render_next_actions(next_actions, right_col_width - 4)
        for combined in _zip_columns(claim_lines, next_lines, left_col_width, right_col_width):
            lines.append(combined)
        lines.append("")

        lines.extend(_render_events(events, width))

    if graph_active and isinstance(workflow, Mapping):
        if not graph_only:
            lines.append("")
        graph_lines = render_graph_panel(
            workflow=workflow,
            node_states=node_states if isinstance(node_states, Mapping) else {},
            width=width,
            height_budget=max(8, int(terminal_height) - len(lines)),
            style=graph_style,
            color=color,
            no_cycles=graph_no_cycles,
            orient=graph_orient,
        )
        lines.extend(graph_lines)

    rendered = "\n".join(_truncate(line, width) for line in lines)
    return rendered + "\n"


def run(
    repo: Path,
    *,
    run_id: str,
    refresh_seconds: float = 2.0,
    once: bool = False,
    stdout: TextIO | None = None,
    graph: bool | None = None,
    graph_only: bool = False,
    graph_style: str = "auto",
    graph_no_cycles: bool = False,
    graph_orient: str = "tb",
) -> None:
    """Render the dashboard until interrupted (or once when ``once=True``).

    RFC 0016 V1+step 3: ``graph`` (None=auto, True=force, False=suppress),
    ``graph_only``, ``graph_style``, ``graph_no_cycles``, ``graph_orient``
    plumb to the graph panel. Color is gated on ``isatty()`` and
    ``NO_COLOR``.
    """
    import os as _os

    out = stdout if stdout is not None else sys.stdout
    is_tty = bool(getattr(out, "isatty", lambda: False)())
    color_active = is_tty and not _os.environ.get("NO_COLOR")

    def write_frame(text: str, *, clear: bool) -> None:
        if clear and is_tty:
            out.write(ANSI_CLEAR_HOME)
        out.write(text)
        out.flush()

    def render(payload: dict[str, Any], *, color: bool) -> str:
        width, height = _detect_size(out)
        return render_frame(
            payload,
            terminal_width=width,
            terminal_height=height,
            graph=graph,
            graph_only=graph_only,
            graph_style=graph_style,
            graph_no_cycles=graph_no_cycles,
            graph_orient=graph_orient,
            color=color,
        )

    if once:
        payload = gather_payload(repo, run_id=run_id)
        payload["refresh_seconds"] = None
        # --once defaults to non-TTY output (no ANSI clear/colors).
        write_frame(render(payload, color=False), clear=False)
        return

    if is_tty:
        out.write(ANSI_HIDE_CURSOR)
        out.flush()
    try:
        while True:
            payload = gather_payload(repo, run_id=run_id)
            payload["refresh_seconds"] = refresh_seconds
            write_frame(render(payload, color=color_active), clear=True)
            time.sleep(max(0.1, float(refresh_seconds)))
    except KeyboardInterrupt:
        if is_tty:
            out.write(ANSI_CLEAR_HOME)
            out.write(ANSI_SHOW_CURSOR)
        out.write("bye\n")
        out.flush()
    finally:
        if is_tty:
            out.write(ANSI_SHOW_CURSOR)
            out.flush()


# ---------------------------------------------------------------------------
# rendering helpers
# ---------------------------------------------------------------------------


def _run_state_chip(state: str, *, paused_at: Any = None) -> str:
    if paused_at and state in {"ready", "running"}:
        return "paused"
    return state


def _job_state_chip(state: str) -> str:
    return state


def _verdict_chip(
    verdict: str,
    *,
    override: bool = False,
    rationale: str | None = None,
) -> str:
    suffix = ""
    if override:
        suffix = " (override)"
        if rationale:
            suffix += f": {_truncate(rationale, 72)}"
    return f"{verdict}{suffix}"


def _lane_attestation_chip(row: Mapping[str, Any], *, max_reason_len: int | None = None) -> str:
    state = str(row.get("lane_attestation") or "")
    reason = str(row.get("lane_attestation_reason") or row.get("reason") or "")
    if state == "attested":
        return "attested"
    if state == "unattested" or reason:
        if max_reason_len is not None and max_reason_len > 0 and len(reason) > max_reason_len:
            reason = reason.split("_", 1)[0] if "_" in reason else _truncate(reason, max_reason_len)
        return f"unattested:{reason or 'unknown'}"
    return state or "unattested:unknown"


def _byline_line(row: Mapping[str, Any]) -> str | None:
    if _lane_attestation_chip(row).startswith("unattested:"):
        label = row.get("operator_label")
        if label:
            return f"author: operator [self-declared: {label}]"
        return "author: operator"
    for key in ("author_line", "expected_author_line", "byline"):
        value = row.get(key)
        if value:
            return str(value)
    return None


def _lane_evidence_chip(value: Any = None) -> str:
    text = str(value or LANE_EVIDENCE_NOT_YET_CORRELATED)
    return text


def _render_left_column(job_counts: Mapping[str, Any]) -> list[str]:
    lines: list[str] = ["Jobs:"]
    seen: set[str] = set()
    for state in JOB_STATE_ORDER:
        count = int(job_counts.get(state, 0) or 0)
        seen.add(state)
        lines.append(f"  {_job_state_chip(state):<14} {count}")
    # Surface any unexpected states the engine reports without dropping them silently.
    for state in sorted(job_counts):
        if state in seen:
            continue
        lines.append(f"  {_job_state_chip(str(state)):<14} {int(job_counts.get(state, 0) or 0)}")
    return lines


def _render_right_column(
    verdict_counts: Mapping[str, int],
    blocker_counts: Mapping[str, int],
    posture_counts: Mapping[str, int] | None = None,
    override_verdict_counts: Mapping[str, int] | None = None,
) -> list[str]:
    lines: list[str] = ["Verdicts:"]
    for verdict in VERDICT_ORDER:
        count = int(verdict_counts.get(verdict, 0))
        lines.append(f"  {_verdict_chip(verdict):<20} {count}")
        if override_verdict_counts:
            override_count = int(override_verdict_counts.get(verdict, 0))
            if override_count:
                lines.append(f"  {_verdict_chip(verdict, override=True):<20} {override_count}")
    # RFC 0018 step 3 (V1.5): per-posture summary line when at least
    # one non-neutral posture exists. Sort by count desc, then posture
    # name asc for deterministic tie-break (Finding 3). Truncate to top-3
    # with `+N more` overflow.
    if posture_counts:
        non_neutral = {
            posture: count
            for posture, count in posture_counts.items()
            if posture != "neutral" and count > 0
        }
        if non_neutral:
            ordered = sorted(
                non_neutral.items(),
                key=lambda item: (-item[1], item[0]),
            )
            head = ordered[:3]
            extra = len(ordered) - len(head)
            parts = [f"{posture}={count}" for posture, count in head]
            if extra > 0:
                parts.append(f"+{extra} more")
            lines.append(f"  Postures: {', '.join(parts)}")
    lines.append("")
    lines.append("Blockers (open):")
    for severity in BLOCKER_SEVERITY_ORDER:
        count = int(blocker_counts.get(severity, 0))
        lines.append(f"  {severity:<16} {count}")
    return lines


def _render_claimable(rows: Iterable[Mapping[str, Any]]) -> list[str]:
    lines: list[str] = ["Claimable now:"]
    rendered_any = False
    for row in rows:
        role = str(row.get("role_id") or "?")
        lane = str(row.get("lane_id") or "?")
        count = int(row.get("count") or 0)
        lines.append(f"  {role}/{lane} x {count}")
        rendered_any = True
    if not rendered_any:
        lines.append("  (none)")
    return lines


def _render_next_actions(actions: Sequence[Any], width: int) -> list[str]:
    lines: list[str] = ["Next actions:"]
    if not actions:
        lines.append("  (none)")
        return lines
    for action in actions:
        text = _format_next_action(action)
        lines.append("  - " + _truncate(text, max(8, width)))
    return lines


def _render_phases(phases: Sequence[Any], width: int) -> list[str]:
    if not phases:
        return []
    entries: list[str] = []
    for raw_phase in phases:
        if not isinstance(raw_phase, Mapping):
            continue
        name = str(raw_phase.get("name") or raw_phase.get("id") or "?")
        completed = int(raw_phase.get("jobs_completed") or 0)
        total = int(raw_phase.get("jobs_total") or 0)
        state = str(raw_phase.get("state") or "pending")
        entries.append(f"{name} {completed}/{total} {state}")
    if not entries:
        return []
    return [_truncate("Phases: " + " | ".join(entries), width)]


def _render_sessions(sessions: Sequence[Any], width: int) -> list[str]:
    if not sessions:
        return []
    lines = ["Sessions:"]
    for raw_session in sessions:
        if not isinstance(raw_session, Mapping):
            continue
        role = str(raw_session.get("role_id") or "?")
        lane = str(raw_session.get("lane_id") or "?")
        state = str(raw_session.get("state") or "?")
        attestation = _lane_attestation_chip(raw_session)
        byline = _byline_line(raw_session)
        parts = [f"{role}/{lane}", state, attestation]
        if byline:
            parts.append(byline)
        lines.append(_truncate("  " + "  ".join(parts), width))
    return lines if len(lines) > 1 else []


def _render_blocker_triage(blockers: Sequence[Any], width: int) -> list[str]:
    rows = [row for row in blockers if isinstance(row, Mapping)]
    if not rows:
        return []
    lines = ["Blockers (open details):"]
    for blocker in rows:
        severity = str(blocker.get("severity") or "blocked")
        kind = str(blocker.get("blocker_kind") or "?")
        workflow_job_id = str(blocker.get("workflow_job_id") or "-")
        lane = str(blocker.get("lane_id") or blocker.get("target_lane_id") or "-")
        created = str(blocker.get("created_at") or "")
        next_action = _first_blocker_next_action(blocker)
        head = f"  {severity} {kind} {workflow_job_id}/{lane}"
        if created:
            head += f" {created}"
        if next_action:
            head += f" next: {next_action}"
        lines.append(_truncate(head, width))
        lines.extend(_render_process_evidence(blocker, width))
    return lines


def _first_blocker_next_action(blocker: Mapping[str, Any]) -> str:
    for candidate in (
        blocker.get("next_actions"),
        _as_dict(blocker.get("human_checkpoint")).get("next_actions"),
    ):
        actions = _as_list(candidate)
        if actions:
            return _format_next_action(actions[0])
    return ""


def _render_process_evidence(blocker: Mapping[str, Any], width: int) -> list[str]:
    payload = _json_object(blocker.get("payload_json"))
    envelope = _json_object(payload.get("envelope")) or payload
    if not envelope:
        return []
    if "envelope_version" not in envelope and "recovery_commands" not in envelope:
        return []
    lines = [f"    provenance evidence: {_lane_evidence_chip()}"]
    for key in (
        "envelope_version",
        "process_id",
        "command",
        "exit_code",
        "duration",
        "timeout",
        "review_verdict_missing",
    ):
        if key in envelope:
            lines.append(_truncate(f"    {key}: {_evidence_value(envelope[key])}", width))
    missing = _as_list(envelope.get("missing_artifact_paths"))
    if missing:
        lines.append("    missing_artifact_paths:")
        for path in missing:
            lines.append(_truncate(f"      - {path}", width))
    recovery_commands = _as_list(envelope.get("recovery_commands"))
    if recovery_commands:
        lines.append("    recovery_commands:")
        for command in recovery_commands:
            lines.append(_truncate(f"      - {_format_next_action(command)}", width))
    return lines


def _render_verdict_overrides(overrides: Sequence[Any], width: int) -> list[str]:
    rows = [row for row in overrides if isinstance(row, Mapping)]
    if not rows:
        return []
    lines = ["Verdict overrides:"]
    for row in rows:
        rationale = str(row.get("rationale") or row.get("override_rationale") or "")
        verdict = _verdict_chip(
            str(row.get("verdict") or "?"),
            override=True,
            rationale=rationale or None,
        )
        previous = str(row.get("previous_verdict") or "?")
        detail = f"  {previous} -> {verdict}"
        if rationale:
            lines.append(_truncate(f"  rationale: {rationale}", width))
        lines.append(_truncate(detail, width))
    return lines


def _render_events(events: Sequence[Mapping[str, Any]], width: int) -> list[str]:
    lines: list[str] = ["Recent events (last 10):"]
    if not events:
        lines.append("  (no events)")
        return lines
    # Column layout: "  HH:MM:SS  event_type  workflow_job_id  hint"
    time_col = 8
    type_col = 22
    job_col = 18
    prefix = 2
    fixed = prefix + time_col + 2 + type_col + 2 + job_col + 2
    hint_col = max(8, width - fixed)
    for event in events:
        created_at = str(event.get("created_at") or "")
        timestamp = _short_time(created_at)
        event_type = _truncate(str(event.get("event_type") or ""), type_col)
        workflow_job_id = _truncate(str(event.get("workflow_job_id") or "-"), job_col)
        payload_hint = _payload_hint(event.get("payload_json"))
        hint = _truncate(payload_hint, hint_col)
        line = (
            "  "
            + f"{timestamp:<{time_col}}"
            + "  "
            + f"{event_type:<{type_col}}"
            + "  "
            + f"{workflow_job_id:<{job_col}}"
            + "  "
            + hint
        )
        lines.append(_truncate(line, width))
    return lines


def _zip_columns(left: Sequence[str], right: Sequence[str], left_w: int, right_w: int) -> list[str]:
    height = max(len(left), len(right))
    rows: list[str] = []
    for index in range(height):
        left_text = left[index] if index < len(left) else ""
        right_text = right[index] if index < len(right) else ""
        left_padded = _pad(left_text, left_w)
        rows.append(left_padded + "  " + right_text[:right_w])
    return rows


def _verdict_counts(verdicts: Sequence[Mapping[str, Any]]) -> dict[str, int]:
    counts: dict[str, int] = {key: 0 for key in VERDICT_ORDER}
    for row in verdicts:
        verdict = str(row.get("verdict") or "")
        if verdict in counts:
            counts[verdict] += 1
    return counts


def _blocker_counts(
    open_blockers: Sequence[Mapping[str, Any]],
    human_checkpoints: Sequence[Mapping[str, Any]],
) -> dict[str, int]:
    counts: dict[str, int] = {key: 0 for key in BLOCKER_SEVERITY_ORDER}
    for row in open_blockers:
        severity = str(row.get("severity") or "blocked")
        if severity == "human_checkpoint":
            continue  # counted separately below
        counts["blocked"] = counts.get("blocked", 0) + 1
    counts["human_checkpoint"] = len(list(human_checkpoints))
    return counts


def _payload_hint(payload_json: Any) -> str:
    if not payload_json:
        return ""
    if isinstance(payload_json, str):
        try:
            payload = json_loads(payload_json) if payload_json.strip().startswith("{") else None
        except Exception:  # noqa: BLE001 - hint must never crash the dashboard
            payload = None
        if payload is None:
            return payload_json
    elif isinstance(payload_json, Mapping):
        payload = dict(payload_json)
    else:
        return str(payload_json)
    fragments: list[str] = []
    for key in sorted(payload):
        value = payload[key]
        if isinstance(value, (dict, list)):
            text = json.dumps(value, separators=(",", ":"))
        else:
            text = str(value)
        fragments.append(f"{key}={text}")
    return " ".join(fragments)


def _json_object(value: Any) -> dict[str, Any]:
    if isinstance(value, Mapping):
        return dict(value)
    if isinstance(value, str) and value.strip().startswith("{"):
        try:
            loaded = json_loads(value)
        except Exception:  # noqa: BLE001 - malformed optional payloads are ignored
            return {}
        if isinstance(loaded, Mapping):
            return dict(loaded)
    return {}


def _format_next_action(action: Any) -> str:
    if isinstance(action, str):
        return action
    if isinstance(action, Sequence) and not isinstance(action, (bytes, bytearray, str)):
        return " ".join(str(part) for part in action)
    if isinstance(action, Mapping):
        for key in ("command", "recipe", "argv", "action", "name"):
            value = action.get(key)
            if value is None:
                continue
            if isinstance(value, Sequence) and not isinstance(value, (bytes, bytearray, str)):
                return " ".join(str(part) for part in value)
            return str(value)
    return str(action)


def _evidence_value(value: Any) -> str:
    if isinstance(value, Sequence) and not isinstance(value, (bytes, bytearray, str)):
        return "[" + ", ".join(str(part) for part in value) + "]"
    if isinstance(value, Mapping):
        return json.dumps(dict(value), separators=(",", ":"), sort_keys=True)
    return str(value)


def _short_time(timestamp: str) -> str:
    # RFC3339 like 2026-05-07T12:34:56Z -> 12:34:56
    if "T" in timestamp:
        tail = timestamp.split("T", 1)[1]
        if tail.endswith("Z"):
            tail = tail[:-1]
        return tail[:8]
    return timestamp[:8]


_ANSI_RE = None  # lazily compiled


def _ansi_visible_len(text: str) -> int:
    """Length of ``text`` counting only printable, non-escape characters."""
    global _ANSI_RE
    if _ANSI_RE is None:
        import re

        _ANSI_RE = re.compile("\x1b\\[[0-9;]*m")
    return len(_ANSI_RE.sub("", text))


def _truncate(text: str, width: int) -> str:
    if width <= 0:
        return ""
    if "\x1b[" not in text:
        if len(text) <= width:
            return text
        if width <= 1:
            return text[:width]
        return text[: width - 1] + "_"
    # ANSI-aware: count visible characters; preserve the trailing reset code
    # so terminals don't bleed color onto subsequent lines.
    visible = _ansi_visible_len(text)
    if visible <= width:
        return text
    # Walk the string, dropping characters past the budget; keep escape
    # sequences intact (they don't consume budget).
    out: list[str] = []
    used = 0
    cap = max(0, width - 1)
    i = 0
    while i < len(text) and used < cap:
        if text[i] == "\x1b" and i + 1 < len(text) and text[i + 1] == "[":
            # Copy the whole escape sequence verbatim.
            j = text.find("m", i)
            if j == -1:
                break
            out.append(text[i : j + 1])
            i = j + 1
            continue
        out.append(text[i])
        used += 1
        i += 1
    out.append("_")
    out.append(ANSI_RESET)
    return "".join(out)


def _pad(text: str, width: int) -> str:
    if len(text) >= width:
        return text[:width]
    return text + " " * (width - len(text))


def _format_refresh(refresh_seconds: Any) -> str:
    try:
        value = float(refresh_seconds)
    except (TypeError, ValueError):
        return str(refresh_seconds)
    if value.is_integer():
        return str(int(value))
    return f"{value:.1f}"


def _detect_width(stream: TextIO) -> int:
    try:
        size = shutil.get_terminal_size(fallback=(80, 24))
    except OSError:
        return 80
    return max(60, int(size.columns))


def _detect_size(stream: TextIO) -> tuple[int, int]:
    try:
        size = shutil.get_terminal_size(fallback=(80, 24))
    except OSError:
        return 80, 24
    return max(60, int(size.columns)), max(10, int(size.lines))


def _as_dict(value: Any) -> dict[str, Any]:
    if isinstance(value, Mapping):
        return dict(value)
    return {}


def _as_list(value: Any) -> list[Any]:
    if isinstance(value, list):
        return list(value)
    return []


__all__ = [
    "ANSI_CLEAR_HOME",
    "ANSI_STATE_COLORS",
    "gather_payload",
    "render_frame",
    "render_graph_panel",
    "run",
]


# -- RFC 0016 V1: graph panel ---------------------------------------------

# ANSI 16-color mapping mirrors striatum.workflow.MERMAID_STATE_FILLS.
# Adding a new state class to that table without adding a key here will
# fail the test_ansi_state_colors_keys_match guard.
ANSI_STATE_COLORS: dict[str, str] = {
    "state-completed": "\x1b[32m",
    "state-running": "\x1b[34m",
    "state-claimed": "\x1b[34m",
    "state-acked": "\x1b[34m",
    "state-blocked": "\x1b[33m",
    "state-stale_lease": "\x1b[33m",
    "state-waiting_human": "\x1b[33m",
    "state-failed": "\x1b[31m",
    "state-canceled": "\x1b[31m",
    "state-queued": "\x1b[39m",
    "state-pending": "\x1b[2m",
}
ANSI_RESET = "\x1b[0m"

# RFC 0016 § 2: one-letter state mnemonics.
_STATE_CHARS: dict[str, str] = {
    "queued": "Q",
    "claimed": "R",
    "running": "R",
    "acked": "R",
    "completed": "C",
    "blocked": "B",
    "waiting_human": "H",
    "failed": "F",
    "canceled": "X",
    "skipped": "X",
    "stale_lease": "S",
    "pending": "P",
}


def render_graph_panel(
    *,
    workflow: Mapping[str, Any],
    node_states: Mapping[str, str],
    width: int,
    height_budget: int,
    style: str = "auto",
    color: bool = False,
    no_cycles: bool = False,
    orient: str = "tb",
) -> list[str]:
    """RFC 0016 V1+step 3: render the run dependency graph.

    Pure: deterministic for a given input. Used by both the dashboard
    (live frame) and ``striatum run graph --format ascii`` (one-shot
    snapshot).

    ``style`` ∈ {auto, layered, list, fancy}. ``orient`` ∈ {tb, lr};
    ``lr`` arranges layers as columns instead of rows. Each
    requested upgrade has a deterministic fall-back: fancy → layered
    when the per-slot width can't fit Unicode boxes; lr → tb when
    too many layers don't fit horizontally; layered → list when
    slot width drops below 12.
    """
    if style not in {"auto", "layered", "list", "fancy"}:
        style = "auto"
    if orient not in {"tb", "lr"}:
        orient = "tb"

    nodes, edges, cycles = _graph_topology(workflow)
    if not nodes:
        return ["Graph: (no jobs declared)"]

    layers = _layer_assignment(nodes, edges)
    max_layer_size = max((len(group) for group in layers), default=1)
    num_layers = len(layers)
    slot_width = (width - 4) // max(1, max_layer_size)
    column_width = (width - 4) // max(1, num_layers)
    box_width = max(12, slot_width)

    chosen = style
    if chosen == "auto":
        chosen = "layered" if slot_width >= 12 else "list"
    if chosen == "fancy" and slot_width < 14:
        chosen = "layered"
    if chosen == "layered" and slot_width < 12:
        chosen = "list"

    chosen_orient = orient
    if chosen in {"layered", "fancy"} and orient == "lr" and column_width < 14:
        chosen_orient = "tb"

    if chosen == "list":
        lines = _render_graph_list(
            nodes=nodes,
            layers=layers,
            cycles=cycles,
            node_states=node_states,
            width=width,
            color=color,
            no_cycles=no_cycles,
        )
    elif chosen_orient == "lr":
        lines = _render_graph_lr(
            nodes=nodes,
            layers=layers,
            cycles=cycles,
            node_states=node_states,
            width=width,
            column_width=max(12, column_width),
            fancy=(chosen == "fancy"),
            color=color,
            no_cycles=no_cycles,
        )
    elif chosen == "fancy":
        lines = _render_graph_fancy(
            nodes=nodes,
            layers=layers,
            cycles=cycles,
            node_states=node_states,
            width=width,
            box_width=box_width,
            color=color,
            no_cycles=no_cycles,
        )
    else:
        lines = _render_graph_layered(
            nodes=nodes,
            layers=layers,
            edges=edges,
            cycles=cycles,
            node_states=node_states,
            width=width,
            box_width=box_width,
            color=color,
            no_cycles=no_cycles,
        )

    if len(lines) > height_budget:
        lines = lines[: height_budget - 1] + [
            _truncate(f"  ... ({len(lines) - height_budget + 1} more line(s) clipped)", width)
        ]
    return lines


def _graph_topology(
    workflow: Mapping[str, Any],
) -> tuple[list[dict[str, Any]], list[dict[str, Any]], list[dict[str, Any]]]:
    """Return ``(nodes, forward_edges, cycle_edges)`` from the workflow."""
    raw_jobs = workflow.get("jobs")
    nodes: list[dict[str, Any]] = []
    if isinstance(raw_jobs, list):
        for job in raw_jobs:
            if not isinstance(job, Mapping):
                continue
            nodes.append({
                "id": str(job.get("id") or ""),
                "type": str(job.get("type") or ""),
                "role_id": str(job.get("role_id") or ""),
                "lane_id": str((job.get("lane_id") or "") or ""),
                "parallel_group": str(job.get("parallel_group") or ""),
            })
    edges: list[dict[str, Any]] = []
    raw_edges = workflow.get("edges")
    if isinstance(raw_edges, list):
        for edge in raw_edges:
            if not isinstance(edge, Mapping):
                continue
            edges.append({
                "from": str(edge.get("from") or ""),
                "to": str(edge.get("to") or ""),
                "on": str((edge.get("on") if isinstance(edge.get("on"), str) else (edge.get("gate") or {}).get("on") if isinstance(edge.get("gate"), Mapping) else "completed")),
            })
    cycles: list[dict[str, Any]] = []
    raw_cycles = workflow.get("cycles")
    if isinstance(raw_cycles, list):
        for cycle in raw_cycles:
            if not isinstance(cycle, Mapping):
                continue
            cycles.append({
                "from": str(cycle.get("from") or ""),
                "to": str(cycle.get("to") or ""),
                "on_verdict": str(cycle.get("on_verdict") or "needs_revision"),
                "max_iterations": cycle.get("max_iterations"),
            })
    return nodes, edges, cycles


def _layer_assignment(
    nodes: Sequence[Mapping[str, Any]],
    edges: Sequence[Mapping[str, Any]],
) -> list[list[str]]:
    """Assign each node to a layer. Forward edges only.

    ``layer(node) = 1 + max(layer(parent))``; nodes with no incoming
    forward edges sit on layer 0.
    """
    incoming: dict[str, list[str]] = {str(n["id"]): [] for n in nodes}
    outgoing: dict[str, list[str]] = {str(n["id"]): [] for n in nodes}
    for edge in edges:
        src = str(edge["from"])
        dst = str(edge["to"])
        if src in outgoing and dst in incoming:
            outgoing[src].append(dst)
            incoming[dst].append(src)
    layer_of: dict[str, int] = {}
    progressed = True
    while progressed:
        progressed = False
        for nid in incoming:
            if nid in layer_of:
                continue
            parents = incoming[nid]
            if not parents:
                layer_of[nid] = 0
                progressed = True
                continue
            if all(parent in layer_of for parent in parents):
                layer_of[nid] = 1 + max(layer_of[parent] for parent in parents)
                progressed = True
    # Any node still missing (cycle in forward edges, defensive) lands on
    # layer 0 to keep the renderer total.
    for nid in incoming:
        if nid not in layer_of:
            layer_of[nid] = 0
    max_layer = max(layer_of.values()) if layer_of else 0
    grouped: list[list[str]] = [[] for _ in range(max_layer + 1)]
    for nid, layer in sorted(layer_of.items()):
        grouped[layer].append(nid)
    return grouped


def _render_graph_layered(
    *,
    nodes: Sequence[Mapping[str, Any]],
    layers: Sequence[Sequence[str]],
    edges: Sequence[Mapping[str, Any]],
    cycles: Sequence[Mapping[str, Any]],
    node_states: Mapping[str, str],
    width: int,
    box_width: int,
    color: bool,
    no_cycles: bool,
) -> list[str]:
    by_id = {str(n["id"]): n for n in nodes}
    lines: list[str] = ["Graph (layered):"]
    for layer_index, layer_ids in enumerate(layers):
        if not layer_ids:
            continue
        boxes = [
            _format_box(by_id[nid], node_states.get(nid, "pending"), box_width, color=color)
            for nid in layer_ids
        ]
        layer_label = f"L{layer_index} "
        line = layer_label + " ".join(boxes)
        lines.append(_truncate(line, width))
        # Connector line between layers (skip after last layer).
        if layer_index < len(layers) - 1 and layers[layer_index + 1]:
            connector = " " * len(layer_label) + " ".join(
                "│".center(box_width) for _ in layer_ids
            )
            lines.append(_truncate(connector, width))
    if cycles and not no_cycles:
        lines.append("")
        lines.append("Cycles (needs_revision):")
        for cycle in cycles:
            limit = cycle.get("max_iterations")
            limit_text = f"max {limit}" if limit is not None else ""
            lines.append(
                _truncate(
                    f"  {cycle['from']} ~~> {cycle['to']}  {limit_text}".strip(),
                    width,
                )
            )
    return lines


def _render_graph_list(
    *,
    nodes: Sequence[Mapping[str, Any]],
    layers: Sequence[Sequence[str]],
    cycles: Sequence[Mapping[str, Any]],
    node_states: Mapping[str, str],
    width: int,
    color: bool,
    no_cycles: bool,
) -> list[str]:
    by_id = {str(n["id"]): n for n in nodes}
    lines: list[str] = ["Graph (layers):"]
    for layer_index, layer_ids in enumerate(layers):
        if not layer_ids:
            continue
        chunks: list[str] = []
        for nid in layer_ids:
            state = node_states.get(nid, "pending")
            chunks.append(_format_inline(by_id[nid], state, color=color))
        line = f"  L{layer_index}  " + "  ".join(chunks)
        lines.append(_truncate(line, width))
    if cycles and not no_cycles:
        lines.append("Cycles:")
        for cycle in cycles:
            limit = cycle.get("max_iterations")
            limit_text = f", max {limit}" if limit is not None else ""
            lines.append(
                _truncate(
                    f"  cycle: {cycle['from']} -> {cycle['to']}{limit_text}",
                    width,
                )
            )
    return lines


def _format_box(
    node: Mapping[str, Any],
    state: str,
    box_width: int,
    *,
    color: bool,
) -> str:
    nid = str(node.get("id") or "")
    state_char = _STATE_CHARS.get(state, _STATE_CHARS.get("pending", "P"))
    inner_budget = max(1, box_width - 4)
    label = nid
    if len(label) > inner_budget - 2:
        label = label[: inner_budget - 3] + "…"
    inner = f"{label} {state_char}"
    box = f"[{inner.ljust(inner_budget)}]"
    return _colorize_box(box, state, color=color)


def _format_inline(
    node: Mapping[str, Any],
    state: str,
    *,
    color: bool,
) -> str:
    nid = str(node.get("id") or "")
    state_char = _STATE_CHARS.get(state, _STATE_CHARS.get("pending", "P"))
    text = f"{nid} [{state_char}]"
    if not color:
        return text
    state_class = _state_class(state)
    code = ANSI_STATE_COLORS.get(state_class, "")
    return f"{code}{text}{ANSI_RESET}" if code else text


def _state_class(state: str) -> str:
    if state == "skipped":
        return "state-canceled"
    candidate = f"state-{state}"
    if candidate in ANSI_STATE_COLORS:
        return candidate
    return "state-pending"


def _colorize_box(box: str, state: str, *, color: bool) -> str:
    if not color:
        return box
    state_class = _state_class(state)
    code = ANSI_STATE_COLORS.get(state_class, "")
    return f"{code}{box}{ANSI_RESET}" if code else box


# RFC 0016 step 3: Unicode `fancy` style + `--graph-orient {tb,lr}`.

# Box-drawing characters (BMP, portable across modern terminals).
_FANCY_TL = "┌"
_FANCY_TR = "┐"
_FANCY_BL = "└"
_FANCY_BR = "┘"
_FANCY_H = "─"
_FANCY_V = "│"
_FANCY_CYCLE_ARROW = "╌╌▶"  # dashed back-edge marker
_FANCY_LR_ARROW = "─→"      # left-to-right edge marker


def _format_fancy_box(
    node: Mapping[str, Any],
    state: str,
    box_width: int,
    *,
    color: bool,
) -> tuple[str, str, str]:
    """Return ``(top, mid, bottom)`` strings for a Unicode-box rendering.

    Inner budget is ``box_width - 4`` (two corner glyphs + two pad spaces).
    Color wraps the inner content (not the frame) to keep the frame
    uniform across states.
    """
    nid = str(node.get("id") or "")
    state_char = _STATE_CHARS.get(state, _STATE_CHARS.get("pending", "P"))
    inner_budget = max(1, box_width - 4)
    label = nid
    if len(label) > inner_budget - 2:
        label = label[: inner_budget - 3] + "…"
    inner = f"{label} {state_char}".ljust(inner_budget)
    if color:
        state_class = _state_class(state)
        code = ANSI_STATE_COLORS.get(state_class, "")
        if code:
            inner = f"{code}{inner}{ANSI_RESET}"
    top = _FANCY_TL + (_FANCY_H * (box_width - 2)) + _FANCY_TR
    mid = f"{_FANCY_V} {inner} {_FANCY_V}"
    bot = _FANCY_BL + (_FANCY_H * (box_width - 2)) + _FANCY_BR
    return top, mid, bot


def _render_graph_fancy(
    *,
    nodes: Sequence[Mapping[str, Any]],
    layers: Sequence[Sequence[str]],
    cycles: Sequence[Mapping[str, Any]],
    node_states: Mapping[str, str],
    width: int,
    box_width: int,
    color: bool,
    no_cycles: bool,
) -> list[str]:
    """RFC 0016 step 3: Unicode-box top-to-bottom layered renderer."""
    by_id = {str(n["id"]): n for n in nodes}
    lines: list[str] = ["Graph (fancy):"]
    for layer_index, layer_ids in enumerate(layers):
        if not layer_ids:
            continue
        layer_label = f"L{layer_index} "
        boxes: list[tuple[str, str, str]] = [
            _format_fancy_box(by_id[nid], node_states.get(nid, "pending"), box_width, color=color)
            for nid in layer_ids
        ]
        tops = " ".join(b[0] for b in boxes)
        mids = " ".join(b[1] for b in boxes)
        bots = " ".join(b[2] for b in boxes)
        lines.append(_truncate(" " * len(layer_label) + tops, width))
        lines.append(_truncate(layer_label + mids, width))
        lines.append(_truncate(" " * len(layer_label) + bots, width))
        if layer_index < len(layers) - 1 and layers[layer_index + 1]:
            connector = " " * len(layer_label) + " ".join(
                _FANCY_V.center(box_width) for _ in layer_ids
            )
            lines.append(_truncate(connector, width))
    if cycles and not no_cycles:
        lines.append("")
        lines.append("Cycles (needs_revision):")
        for cycle in cycles:
            limit = cycle.get("max_iterations")
            limit_text = f"max {limit}" if limit is not None else ""
            lines.append(
                _truncate(
                    f"  {cycle['from']} {_FANCY_CYCLE_ARROW} {cycle['to']}  {limit_text}".strip(),
                    width,
                )
            )
    return lines


def _render_graph_lr(
    *,
    nodes: Sequence[Mapping[str, Any]],
    layers: Sequence[Sequence[str]],
    cycles: Sequence[Mapping[str, Any]],
    node_states: Mapping[str, str],
    width: int,
    column_width: int,
    fancy: bool,
    color: bool,
    no_cycles: bool,
) -> list[str]:
    """RFC 0016 step 3: left-to-right column-major layered renderer.

    Each layer becomes a vertical column; columns are separated by an
    arrow glyph (``─→`` in fancy mode, ``->`` in plain).
    """
    by_id = {str(n["id"]): n for n in nodes}
    lines: list[str] = [f"Graph (lr, {'fancy' if fancy else 'layered'}):"]
    arrow = f"  {_FANCY_LR_ARROW}  " if fancy else "  ->  "

    inner_box = max(8, column_width - len(arrow))
    column_blocks: list[list[str]] = []
    for layer_index, layer_ids in enumerate(layers):
        block: list[str] = [f"L{layer_index}".ljust(inner_box)]
        for nid in layer_ids or []:
            state = node_states.get(nid, "pending")
            if fancy:
                top, mid, bot = _format_fancy_box(by_id[nid], state, inner_box, color=color)
                block.extend([top, mid, bot, ""])
            else:
                block.append(_format_box(by_id[nid], state, inner_box, color=color))
                block.append("")
        column_blocks.append(block)

    height = max((len(c) for c in column_blocks), default=1)
    for col in column_blocks:
        col.extend([" " * inner_box] * (height - len(col)))

    for row in range(height):
        cells = [col[row].ljust(inner_box) for col in column_blocks]
        line = arrow.join(cells)
        lines.append(_truncate(line, width))

    if cycles and not no_cycles:
        lines.append("")
        lines.append("Cycles (needs_revision):")
        cycle_arrow = _FANCY_CYCLE_ARROW if fancy else "~~>"
        for cycle in cycles:
            limit = cycle.get("max_iterations")
            limit_text = f"max {limit}" if limit is not None else ""
            lines.append(
                _truncate(
                    f"  {cycle['from']} {cycle_arrow} {cycle['to']}  {limit_text}".strip(),
                    width,
                )
            )
    return lines
