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
JOB_STATE_ORDER: tuple[str, ...] = (
    "blocked",
    "queued",
    "claimed",
    "running",
    "waiting_human",
    "completed",
    "failed",
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

    ensure_initialized(repo)
    with connect(repo) as conn:
        # Resolve the run early so an unknown id fails with the standard exit code.
        run_row = conn.execute(
            "SELECT run_id, state, branch_name FROM runs WHERE run_id = ?",
            (run_id,),
        ).fetchone()
        if run_row is None:
            raise NotFoundError(f"unknown run_id {run_id!r}")
        status_payload = status_command(conn, run_id=run_id)
        events = recent_events_for_run(conn, run_id=run_id, limit=10)
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

    run = dict(run_row)
    return {
        "run": run,
        "status": status_payload,
        "events": events,
        "verdict_counts": verdict_counts,
        "updated_at": utc_now(),
    }


def render_frame(payload: Mapping[str, Any], *, terminal_width: int) -> str:
    """Render one dashboard frame as plain text.

    Pure: takes a payload from ``gather_payload`` and a terminal width and
    returns the rendered string. Easy to unit test.
    """
    width = max(60, int(terminal_width))
    run = _as_dict(payload.get("run"))
    status_payload = _as_dict(payload.get("status"))
    events = _as_list(payload.get("events"))
    updated_at = str(payload.get("updated_at") or "")
    refresh_seconds = payload.get("refresh_seconds")

    run_id = str(run.get("run_id") or "<unknown>")
    branch = str(run.get("branch_name") or "<unconfirmed>")
    state = str(run.get("state") or "unknown")

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
    claimable = _as_list(status_payload.get("claimable_jobs"))
    next_actions = _as_list(status_payload.get("next_actions"))

    left_lines = _render_left_column(job_counts)
    right_lines = _render_right_column(verdict_counts, blocker_counts)
    for combined in _zip_columns(left_lines, right_lines, left_col_width, right_col_width):
        lines.append(combined)
    lines.append("")

    claim_lines = _render_claimable(claimable)
    next_lines = _render_next_actions(next_actions, right_col_width - 4)
    for combined in _zip_columns(claim_lines, next_lines, left_col_width, right_col_width):
        lines.append(combined)
    lines.append("")

    lines.extend(_render_events(events, width))

    rendered = "\n".join(_truncate(line, width) for line in lines)
    return rendered + "\n"


def run(
    repo: Path,
    *,
    run_id: str,
    refresh_seconds: float = 2.0,
    once: bool = False,
    stdout: TextIO | None = None,
) -> None:
    """Render the dashboard until interrupted (or once when ``once=True``)."""
    out = stdout if stdout is not None else sys.stdout
    is_tty = bool(getattr(out, "isatty", lambda: False)())

    def write_frame(text: str, *, clear: bool) -> None:
        if clear and is_tty:
            out.write(ANSI_CLEAR_HOME)
        out.write(text)
        out.flush()

    if once:
        payload = gather_payload(repo, run_id=run_id)
        payload["refresh_seconds"] = None
        width = _detect_width(out)
        write_frame(render_frame(payload, terminal_width=width), clear=False)
        return

    if is_tty:
        out.write(ANSI_HIDE_CURSOR)
        out.flush()
    try:
        while True:
            payload = gather_payload(repo, run_id=run_id)
            payload["refresh_seconds"] = refresh_seconds
            width = _detect_width(out)
            write_frame(render_frame(payload, terminal_width=width), clear=True)
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


def _render_left_column(job_counts: Mapping[str, Any]) -> list[str]:
    lines: list[str] = ["Jobs:"]
    seen: set[str] = set()
    for state in JOB_STATE_ORDER:
        count = int(job_counts.get(state, 0) or 0)
        seen.add(state)
        lines.append(f"  {state:<14} {count}")
    # Surface any unexpected states the engine reports without dropping them silently.
    for state in sorted(job_counts):
        if state in seen:
            continue
        lines.append(f"  {state:<14} {int(job_counts.get(state, 0) or 0)}")
    return lines


def _render_right_column(
    verdict_counts: Mapping[str, int],
    blocker_counts: Mapping[str, int],
) -> list[str]:
    lines: list[str] = ["Verdicts:"]
    for verdict in VERDICT_ORDER:
        count = int(verdict_counts.get(verdict, 0))
        lines.append(f"  {verdict:<20} {count}")
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
        text = str(action)
        lines.append("  - " + _truncate(text, max(8, width)))
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


def _short_time(timestamp: str) -> str:
    # RFC3339 like 2026-05-07T12:34:56Z -> 12:34:56
    if "T" in timestamp:
        tail = timestamp.split("T", 1)[1]
        if tail.endswith("Z"):
            tail = tail[:-1]
        return tail[:8]
    return timestamp[:8]


def _truncate(text: str, width: int) -> str:
    if width <= 0:
        return ""
    if len(text) <= width:
        return text
    if width <= 1:
        return text[:width]
    return text[: width - 1] + "_"


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
    "gather_payload",
    "render_frame",
    "run",
]
