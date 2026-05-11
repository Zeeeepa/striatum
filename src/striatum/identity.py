"""Shared identity helpers for jobs, sessions, and artifacts."""

from __future__ import annotations

import json
import os
import re
import sqlite3
from dataclasses import dataclass
from typing import Mapping, TypedDict


OPERATOR_LABEL_RE = re.compile(r"^[a-z0-9._-]{1,64}$")
ATTESTED_BYLINE_BODY_RE = re.compile(r"^[a-z0-9.]+-[a-z0-9.]+(?:-[a-z0-9.]+)*-\d{3}$")
RESERVED_OPERATOR_LABELS = frozenset({
    "attested",
    "supervised",
    "lane",
    "model",
})


class ArtifactAuthorIdentity(TypedDict, total=False):
    """Author metadata for evidence export plus optional artifact byline.

    The required-shape keys are always present; ``actual_author_line`` is
    populated by the artifact-list path when the snapshot reflects the
    file's actual byline (HARNESS-003 byline integrity), and absent on
    the job-list path where there is no published file yet.
    """

    role_id: str
    lane_id: str | None
    display_model: str | None
    workflow_job_id: str
    ordinal: int | None
    line: str | None
    actual_author_line: str | None


@dataclass(frozen=True)
class LaneAttestation:
    """Derived lane-liveness attestation for one session at one point in time."""

    attested: bool
    state: str
    supervisor_id: str | None = None
    pid: int | None = None
    reason: str | None = None


def artifact_author_identity(
    workflow: Mapping[str, object],
    *,
    role_id: str,
    lane_id: str | None,
    workflow_job_id: str,
    ordinal: int | None = None,
    attested: bool = True,
    operator_label: str | None = None,
) -> ArtifactAuthorIdentity:
    """Return stable export identity and a non-leaky artifact byline."""
    display_model = lane_display_model(workflow, lane_id)
    line: str | None = None
    if ordinal is not None:
        if attested:
            line = (
                f"author: {author_part(role_id)}-"
                f"{author_part(display_model or 'unknown-model')}-"
                f"{ordinal:03d}"
            )
        else:
            line = operator_author_line(operator_label)
    return {
        "role_id": role_id,
        "lane_id": lane_id,
        "display_model": display_model,
        "workflow_job_id": workflow_job_id,
        "ordinal": ordinal,
        "line": line,
    }


def lane_display_model(workflow: Mapping[str, object], lane_id: str | None) -> str | None:
    """Return the workflow display model for a lane when declared."""
    if lane_id is None:
        return None
    lanes = workflow.get("lanes")
    if not isinstance(lanes, dict):
        return None
    lane_config = lanes.get(lane_id)
    if not isinstance(lane_config, dict):
        return None
    display_model = lane_config.get("display_model")
    return display_model if isinstance(display_model, str) else None


def author_part(value: str) -> str:
    """Normalize a role or model label for a low-leak artifact byline."""
    normalized = re.sub(r"[^a-z0-9.]+", "-", value.lower()).strip("-")
    return normalized or "unknown"


def operator_author_line(operator_label: str | None) -> str:
    """Return the byline for an operator-authored/unattested artifact."""
    if operator_label is None or operator_label == "":
        return "author: operator"
    return f"author: operator [self-declared: {operator_label}]"


def validate_operator_label(label: str, *, workflow: Mapping[str, object]) -> str:
    """Validate and canonicalize an operator self-label.

    Labels are intentionally narrow and self-declared. They must not resemble
    attested role/model/ordinal bylines or reserved attestation terms.
    """
    cleaned = label.strip().lower()
    if not OPERATOR_LABEL_RE.fullmatch(cleaned):
        raise ValueError(
            "operator label must match ^[a-z0-9._-]{1,64}$"
        )
    reserved = set(RESERVED_OPERATOR_LABELS)
    lanes = workflow.get("lanes")
    if isinstance(lanes, dict):
        reserved.update(str(key).lower() for key in lanes)
    if cleaned in reserved:
        raise ValueError(f"operator label {cleaned!r} is reserved")
    if ATTESTED_BYLINE_BODY_RE.fullmatch(cleaned):
        raise ValueError("operator label must not resemble an attested byline")
    return cleaned


def session_lane_attestation(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    mark_lost: bool = False,
) -> LaneAttestation:
    """Return whether ``session_id`` has an attached, live lane supervisor.

    Attestation is deliberately narrower than authorship proof. It means the
    runner has an ``attached`` supervisor for this exact session/run, the pid
    still names the same process identity captured at launch time, and the
    supervisor command matches the session lane's snapshotted command.
    """
    session = conn.execute(
        "SELECT * FROM sessions WHERE session_id = ?",
        (session_id,),
    ).fetchone()
    if session is None:
        return LaneAttestation(attested=False, state="unattested", reason="session_missing")
    row = conn.execute(
        """
        SELECT * FROM process_supervisors
        WHERE session_id = ? AND state = 'attached'
        ORDER BY started_at DESC
        LIMIT 1
        """,
        (session_id,),
    ).fetchone()
    if row is None:
        return LaneAttestation(attested=False, state="unattested", reason="no_attached_supervisor")
    supervisor_id = str(row["supervisor_id"])
    pid_value = row["pid"]
    pid = int(pid_value) if pid_value is not None else None
    reason = _inactive_reason(conn, session=session, supervisor=row, pid=pid)
    if reason is not None:
        if mark_lost and reason in {"pid_gone", "pid_identity_mismatch"}:
            _mark_supervisor_lost(conn, supervisor_id=supervisor_id, reason=reason)
        return LaneAttestation(
            attested=False,
            state="unattested",
            supervisor_id=supervisor_id,
            pid=pid,
            reason=reason,
        )
    return LaneAttestation(
        attested=True,
        state="attested",
        supervisor_id=supervisor_id,
        pid=pid,
    )


def _inactive_reason(
    conn: sqlite3.Connection,
    *,
    session: sqlite3.Row,
    supervisor: sqlite3.Row,
    pid: int | None,
) -> str | None:
    if str(supervisor["run_id"]) != str(session["run_id"]):
        return "run_mismatch"
    if str(supervisor["session_id"]) != str(session["session_id"]):
        return "session_mismatch"
    if pid is None or not _pid_alive(pid):
        return "pid_gone"
    expected_start = supervisor["pid_start_time"] if "pid_start_time" in supervisor.keys() else None
    if expected_start is None or expected_start == "":
        return "pid_identity_unavailable"
    current_start = process_start_time(pid)
    if current_start != str(expected_start):
        return "pid_identity_mismatch"
    expected_command = _session_lane_command(conn, session=session)
    if expected_command is None:
        return "lane_command_missing"
    try:
        recorded_command = json.loads(str(supervisor["command_json"]))
    except json.JSONDecodeError:
        return "supervisor_command_invalid"
    if recorded_command != expected_command:
        return "lane_command_mismatch"
    return None


def _session_lane_command(
    conn: sqlite3.Connection, *, session: sqlite3.Row
) -> list[str] | None:
    run = conn.execute(
        "SELECT * FROM runs WHERE run_id = ?",
        (session["run_id"],),
    ).fetchone()
    if run is None:
        return None
    snapshot = conn.execute(
        "SELECT workflow_json FROM workflow_snapshots WHERE workflow_snapshot_id = ?",
        (run["workflow_snapshot_id"],),
    ).fetchone()
    if snapshot is None:
        return None
    try:
        workflow = json.loads(str(snapshot["workflow_json"]))
    except json.JSONDecodeError:
        return None
    if not isinstance(workflow, dict):
        return None
    lanes = workflow.get("lanes")
    if not isinstance(lanes, dict):
        return None
    lane = lanes.get(str(session["lane_id"]))
    if not isinstance(lane, dict):
        return None
    command = lane.get("command")
    if not isinstance(command, list) or not all(isinstance(part, str) for part in command):
        return None
    return list(command)


def process_start_time(pid: int) -> str | None:
    """Return a stable per-process start token for ``pid`` when supported."""
    if pid <= 0:
        return None
    stat_path = f"/proc/{pid}/stat"
    try:
        raw = open(stat_path, encoding="utf-8").read()
    except OSError:
        return None
    # Field 2 may contain spaces inside parentheses. Split after the final
    # close-paren; field 22 is then index 19 in the remaining fields because
    # fields 1 and 2 were consumed by the prefix.
    try:
        rest = raw.rsplit(")", 1)[1].strip().split()
    except IndexError:
        return None
    if len(rest) <= 19:
        return None
    return rest[19]


def _pid_alive(pid: int) -> bool:
    if pid <= 0:
        return False
    try:
        os.kill(pid, 0)
    except (ProcessLookupError, PermissionError):
        return False
    return True


def _mark_supervisor_lost(
    conn: sqlite3.Connection, *, supervisor_id: str, reason: str
) -> None:
    conn.execute(
        """
        UPDATE process_supervisors
        SET state = 'lost', ended_at = COALESCE(ended_at, datetime('now')),
            stop_reason = ?
        WHERE supervisor_id = ? AND state = 'attached'
        """,
        (reason, supervisor_id),
    )
