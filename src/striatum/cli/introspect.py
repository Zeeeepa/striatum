"""Read-side introspection helpers (status, why, doctor) for the Striatum CLI."""

from __future__ import annotations

import importlib
import json
import os
import sqlite3
from pathlib import Path
from types import ModuleType
from typing import Callable

from striatum.db import (
    JsonObject,
    expire_leases,
    json_loads,
    latest_verdict,
    row_by_id,
)
from striatum.identity import session_lane_attestation
from striatum.errors import InvalidTransitionError, NotFoundError, WorkflowError
from striatum.supervisor import SUPERVISOR_ACTIVE_STATES
from striatum.db import utc_now
from striatum.workflow import (
    mermaid_state_class,
    workflow_phase_index,
    workflow_graph_data,
    workflow_graph_mermaid,
)

PHASE_ACTIVE_STATES = frozenset({
    "queued",
    "claimed",
    "running",
    "waiting_human",
    "blocked",
})


def _install_module() -> ModuleType:
    """Return the running ``striatum`` package module.

    Wrapped in a function so tests can monkeypatch the imported
    ``striatum.__file__`` to simulate an editable install pointing at an
    out-of-repo path (HARNESS-002).
    """
    return importlib.import_module("striatum")


def _reviewer_independence_breaches(
    conn: sqlite3.Connection, *, run_id: str | None
) -> list[JsonObject]:
    """Detect observable reviewer-independence breaches (HARNESS-003).

    Two signals:

    1. Two active sessions on the same run whose supervisor rows share a
       pid. Same OS process is driving both lanes; whatever
       ``reviewer_context_policy: fresh`` was supposed to mean is broken.
    2. A reviewer session on a run where the author session has an active
       supervisor but the reviewer does not. The supervised vs
       unsupervised mix means the reviewer is almost certainly being
       driven by the operator from the same shell as the author.

    The runner cannot tell the difference between a deliberate
    operator-driven workflow and an accidental independence breach.
    Doctor surfaces the signal; the operator decides.
    """
    breaches: list[JsonObject] = []
    shared_pid_rows = conn.execute(
        f"""
        SELECT s1.session_id AS s1_id, s1.role_id AS s1_role,
               s2.session_id AS s2_id, s2.role_id AS s2_role,
               sup1.pid AS pid, sup1.run_id AS run_id
        FROM sessions s1
        JOIN sessions s2 ON s2.run_id = s1.run_id AND s2.session_id < s1.session_id
        JOIN process_supervisors sup1
          ON sup1.session_id = s1.session_id
         AND sup1.state IN ({_placeholders_csv(SUPERVISOR_ACTIVE_STATES)})
        JOIN process_supervisors sup2
          ON sup2.session_id = s2.session_id
         AND sup2.state IN ({_placeholders_csv(SUPERVISOR_ACTIVE_STATES)})
         AND sup2.pid = sup1.pid
        WHERE s1.state = 'active' AND s2.state = 'active'
          AND (? IS NULL OR s1.run_id = ?)
        """,
        (
            *SUPERVISOR_ACTIVE_STATES,
            *SUPERVISOR_ACTIVE_STATES,
            run_id,
            run_id,
        ),
    ).fetchall()
    for row in shared_pid_rows:
        breaches.append(
            {
                "session_id": str(row["s1_id"]),
                "message": (
                    "reviewer-independence unverified: sessions share supervisor pid "
                    f"{row['pid']} ({row['s1_id']}/{row['s1_role']} vs "
                    f"{row['s2_id']}/{row['s2_role']})"
                ),
                "context": {
                    "run_id": row["run_id"],
                    "shared_pid": row["pid"],
                    "session_a": {
                        "session_id": row["s1_id"],
                        "role_id": row["s1_role"],
                    },
                    "session_b": {
                        "session_id": row["s2_id"],
                        "role_id": row["s2_role"],
                    },
                },
            }
        )
    asymmetric_rows = conn.execute(
        f"""
        SELECT reviewer.session_id AS reviewer_session,
               reviewer.role_id AS reviewer_role,
               author.session_id AS author_session,
               author.role_id AS author_role,
               author_sup.pid AS author_pid,
               reviewer.run_id AS run_id
        FROM sessions reviewer
        JOIN sessions author ON author.run_id = reviewer.run_id
        JOIN process_supervisors author_sup
          ON author_sup.session_id = author.session_id
         AND author_sup.state IN ({_placeholders_csv(SUPERVISOR_ACTIVE_STATES)})
        LEFT JOIN process_supervisors reviewer_sup
          ON reviewer_sup.session_id = reviewer.session_id
         AND reviewer_sup.state IN ({_placeholders_csv(SUPERVISOR_ACTIVE_STATES)})
        WHERE reviewer.state = 'active'
          AND author.state = 'active'
          AND reviewer.role_id = 'reviewer'
          AND author.role_id = 'author'
          AND reviewer_sup.session_id IS NULL
          AND (? IS NULL OR reviewer.run_id = ?)
        """,
        (
            *SUPERVISOR_ACTIVE_STATES,
            *SUPERVISOR_ACTIVE_STATES,
            run_id,
            run_id,
        ),
    ).fetchall()
    for row in asymmetric_rows:
        breaches.append(
            {
                "session_id": str(row["reviewer_session"]),
                "message": (
                    "reviewer-independence unverified: reviewer session has no supervisor "
                    f"but author {row['author_session']} runs supervised (pid={row['author_pid']})"
                ),
                "context": {
                    "run_id": row["run_id"],
                    "reviewer_session_id": row["reviewer_session"],
                    "author_session_id": row["author_session"],
                    "author_pid": row["author_pid"],
                },
            }
        )
    return breaches


def _placeholders_csv(values: tuple[str, ...]) -> str:
    return ",".join(["?"] * len(values))


def status(conn: sqlite3.Connection, *, run_id: str | None) -> JsonObject:
    """Return current state summary."""
    if run_id is not None:
        expire_leases(conn, run_id=run_id)
    runs = conn.execute(
        "SELECT run_id, state, branch_name FROM runs WHERE (? IS NULL OR run_id = ?) ORDER BY created_at",
        (run_id, run_id),
    ).fetchall()
    jobs = conn.execute(
        """
        SELECT state, COUNT(*) AS count FROM jobs
        WHERE (? IS NULL OR run_id = ?)
        GROUP BY state ORDER BY state
        """,
        (run_id, run_id),
    ).fetchall()
    open_blockers = blocker_summaries(conn, run_id=run_id, severity=None)
    human_checkpoints = blocker_summaries(conn, run_id=run_id, severity="human_checkpoint")
    non_accepting = latest_non_accepting_verdicts(conn, run_id=run_id)
    verdicts_by_posture = _count_verdicts_by_posture(conn, run_id=run_id)
    claimable = claimable_jobs_by_role_lane(conn, run_id=run_id)
    blocked_downstream = blocked_downstream_jobs(conn, run_id=run_id)
    has_orphan_supervisor = _has_supervisor_lost_with_held_lease(conn, run_id=run_id)
    process_health = _process_health(conn, run_id=run_id)
    actions = next_actions(
        open_blockers=open_blockers,
        human_checkpoints=human_checkpoints,
        non_accepting_verdicts=non_accepting,
        claimable_jobs=claimable,
        has_orphan_supervisor=has_orphan_supervisor,
    )
    if process_health["next_actions"]:
        for extra in process_health["next_actions"]:
            if extra not in actions:
                actions.append(extra)
    result: JsonObject = {
        "runs": [dict(row) for row in runs],
        "provenance_mode": _provenance_mode_for_status(conn, run_id=run_id),
        "sessions": _session_attestation_summaries(conn, run_id=run_id),
        "jobs": {str(row["state"]): int(row["count"]) for row in jobs},
        "open_blockers": open_blockers,
        "human_checkpoints": human_checkpoints,
        "latest_non_accepting_review_verdicts": non_accepting,
        "verdicts_by_posture": verdicts_by_posture,
        "claimable_jobs": claimable,
        "blocked_downstream_jobs": blocked_downstream,
        "process_health": process_health,
        "next_actions": actions,
    }
    if run_id is not None:
        phase_progress = phase_progress_for_run(conn, run_id=run_id)
        if phase_progress is not None:
            result.update(phase_progress)
    return result


def phase_progress_for_run(conn: sqlite3.Connection, *, run_id: str) -> JsonObject | None:
    """Return phase progress derived from the workflow snapshot and jobs.

    RFC 0045 V1 does not add phase runtime tables or a ``jobs.phase_id``
    column. Phase status is therefore a read-model projection over the
    immutable workflow snapshot plus the latest job attempt rows for a run.
    """
    run_row = conn.execute(
        "SELECT workflow_snapshot_id FROM runs WHERE run_id = ?",
        (run_id,),
    ).fetchone()
    if run_row is None:
        return None
    snapshot_row = conn.execute(
        "SELECT workflow_json FROM workflow_snapshots WHERE workflow_snapshot_id = ?",
        (str(run_row["workflow_snapshot_id"]),),
    ).fetchone()
    if snapshot_row is None:
        return None
    try:
        workflow = json_loads(str(snapshot_row["workflow_json"]))
    except (json.JSONDecodeError, InvalidTransitionError):
        return None

    try:
        phase_index = workflow_phase_index(workflow)
    except WorkflowError:
        return None
    if phase_index.get("declared") is not True:
        return None

    latest_job_rows = conn.execute(
        """
        SELECT j.*
        FROM jobs j
        JOIN (
            SELECT workflow_job_id, MAX(attempt) AS max_attempt
            FROM jobs
            WHERE run_id = ?
            GROUP BY workflow_job_id
        ) latest
          ON latest.workflow_job_id = j.workflow_job_id
         AND latest.max_attempt = j.attempt
        WHERE j.run_id = ?
        """,
        (run_id, run_id),
    ).fetchall()
    latest_jobs = {str(row["workflow_job_id"]): row for row in latest_job_rows}

    phases: list[JsonObject] = []
    phase_order = list(phase_index["phase_order"])
    phase_by_id = phase_index["phase_by_id"]
    job_phase = phase_index["job_phase"]
    synthesis_by_phase = phase_index["synthesis_by_phase"]
    jobs_by_phase: dict[str, list[str]] = {str(phase_id): [] for phase_id in phase_order}
    for workflow_job_id, phase_id in job_phase.items():
        jobs_by_phase.setdefault(str(phase_id), []).append(str(workflow_job_id))

    for index, phase_id_value in enumerate(phase_order):
        phase_id = str(phase_id_value)
        phase = phase_by_id[phase_id]
        workflow_job_ids = jobs_by_phase.get(phase_id, [])
        jobs_by_state: dict[str, int] = {}
        job_states: list[str] = []
        for workflow_job_id in workflow_job_ids:
            row = latest_jobs.get(workflow_job_id)
            state = str(row["state"]) if row is not None else "pending"
            job_states.append(state)
            jobs_by_state[state] = jobs_by_state.get(state, 0) + 1

        jobs_total = len(workflow_job_ids)
        jobs_completed = jobs_by_state.get("completed", 0)
        synthesis_job_id = synthesis_by_phase.get(phase_id)
        synthesis_row = latest_jobs.get(synthesis_job_id or "")
        synthesis_state = str(synthesis_row["state"]) if synthesis_row is not None else None
        synthesis_verdict = (
            latest_verdict(conn, job_id=str(synthesis_row["job_id"]))
            if synthesis_row is not None
            else None
        )
        phase_state = _derive_phase_state(job_states)
        phase_payload: JsonObject = {
            "id": phase_id,
            "name": str(phase.get("name") or phase_id),
            "index": index,
            "state": phase_state,
            "jobs_total": jobs_total,
            "jobs_completed": jobs_completed,
            "jobs_by_state": jobs_by_state,
            "synthesis_job_id": synthesis_job_id,
            "synthesis_state": synthesis_state,
            "synthesis_verdict": synthesis_verdict,
        }
        for optional_key in ("description", "color"):
            optional_value = phase.get(optional_key)
            if isinstance(optional_value, str):
                phase_payload[optional_key] = optional_value
        phases.append(phase_payload)

    if not phases:
        return None
    current_phase_id = phases[-1]["id"]
    for phase in phases:
        if phase["state"] != "completed":
            current_phase_id = phase["id"]
            break
    return {"phases": phases, "current_phase_id": current_phase_id}


def _derive_phase_state(job_states: list[str]) -> str:
    if job_states and all(state == "completed" for state in job_states):
        return "completed"
    if any(state in PHASE_ACTIVE_STATES for state in job_states):
        return "active"
    if any(state == "failed" for state in job_states):
        return "failed"
    incomplete = [state for state in job_states if state != "completed"]
    if incomplete and all(state in {"canceled", "skipped"} for state in incomplete):
        return "canceled"
    return "pending"


def _provenance_mode_for_status(
    conn: sqlite3.Connection, *, run_id: str | None
) -> str | dict[str, str]:
    rows = conn.execute(
        """
        SELECT r.run_id, ws.workflow_json
        FROM runs r
        JOIN workflow_snapshots ws
          ON ws.workflow_snapshot_id = r.workflow_snapshot_id
        WHERE (? IS NULL OR r.run_id = ?)
        ORDER BY r.created_at
        """,
        (run_id, run_id),
    ).fetchall()
    modes: dict[str, str] = {}
    for row in rows:
        try:
            workflow = json.loads(str(row["workflow_json"]))
        except json.JSONDecodeError:
            mode = "advisory"
        else:
            mode = workflow.get("provenance_mode", "advisory") if isinstance(workflow, dict) else "advisory"
            if not isinstance(mode, str):
                mode = "advisory"
        modes[str(row["run_id"])] = mode
    if run_id is not None:
        return modes.get(run_id, "advisory")
    return modes


def _session_attestation_summaries(
    conn: sqlite3.Connection, *, run_id: str | None
) -> list[JsonObject]:
    rows = conn.execute(
        """
        SELECT session_id, run_id, role_id, lane_id, slug, state, operator_label
        FROM sessions
        WHERE (? IS NULL OR run_id = ?)
        ORDER BY registered_at
        """,
        (run_id, run_id),
    ).fetchall()
    result: list[JsonObject] = []
    for row in rows:
        attestation = session_lane_attestation(conn, session_id=str(row["session_id"]))
        result.append(
            {
                "session_id": row["session_id"],
                "run_id": row["run_id"],
                "role_id": row["role_id"],
                "lane_id": row["lane_id"],
                "slug": row["slug"],
                "state": row["state"],
                "operator_label": row["operator_label"],
                "lane_attestation": attestation.state,
                "lane_attestation_reason": attestation.reason,
                "supervisor_id": attestation.supervisor_id,
                "pid": attestation.pid,
            }
        )
    return result


def _count_verdicts_by_posture(
    conn: sqlite3.Connection, *, run_id: str | None
) -> dict[str, int]:
    """RFC 0018 step 3 (V1.5): aggregate verdict counts by posture.

    Returns a mapping of posture → count of recorded verdicts. Counts
    *all* verdicts regardless of value (operators want to see whether a
    posture review even ran). Always emitted (empty dict when no
    verdicts exist) for stable shape across posture-omitting and
    posture-declaring runs.
    """
    rows = conn.execute(
        """
        SELECT posture, COUNT(*) AS count FROM verdicts
        WHERE (? IS NULL OR run_id = ?)
        GROUP BY posture
        """,
        (run_id, run_id),
    ).fetchall()
    return {str(row["posture"]): int(row["count"]) for row in rows}


def _process_health(
    conn: sqlite3.Connection, *, run_id: str | None
) -> JsonObject:
    """RFC 0014 V1: summary of process_executions liveness for a run.

    Returns running/stale_running/lost/timed_out counts plus a
    next_actions list that recommends ``recovery process-reconcile``
    when stale_running > 0.
    """
    if run_id is None:
        return {
            "running_count": 0,
            "stale_running_count": 0,
            "lost_count": 0,
            "timed_out_count": 0,
            "next_actions": [],
        }
    running_total = conn.execute(
        "SELECT COUNT(*) AS c FROM process_executions WHERE run_id = ? AND state = 'running'",
        (run_id,),
    ).fetchone()
    stale_running = conn.execute(
        """
        SELECT COUNT(*) AS c FROM process_executions pe
        JOIN leases l ON l.lease_id = pe.lease_id
        WHERE pe.run_id = ? AND pe.state = 'running' AND l.state = 'expired'
        """,
        (run_id,),
    ).fetchone()
    lost_total = conn.execute(
        "SELECT COUNT(*) AS c FROM process_executions WHERE run_id = ? AND state = 'lost'",
        (run_id,),
    ).fetchone()
    timed_out_total = conn.execute(
        "SELECT COUNT(*) AS c FROM process_executions WHERE run_id = ? AND state = 'timed_out'",
        (run_id,),
    ).fetchone()
    running_count = int(running_total["c"]) if running_total else 0
    stale_running_count = int(stale_running["c"]) if stale_running else 0
    lost_count = int(lost_total["c"]) if lost_total else 0
    timed_out_count = int(timed_out_total["c"]) if timed_out_total else 0
    next_acts: list[str] = []
    if stale_running_count > 0:
        next_acts.append("recovery_process_reconcile")
    return {
        "running_count": running_count,
        "stale_running_count": stale_running_count,
        "lost_count": lost_count,
        "timed_out_count": timed_out_count,
        "next_actions": next_acts,
    }


def _has_supervisor_lost_with_held_lease(
    conn: sqlite3.Connection, *, run_id: str | None
) -> bool:
    """Cheap precheck for the HARNESS-001 next_action signal.

    Mirrors the doctor query but returns a bool. Kept separate so
    ``status`` does not depend on the full doctor result every call.
    """
    row = conn.execute(
        """
        SELECT 1 FROM process_supervisors s
        JOIN leases l
          ON l.owner_session_id = s.session_id
         AND l.state = 'active'
         AND l.expires_at > ?
        WHERE s.state = 'lost'
          AND (? IS NULL OR s.run_id = ?)
        LIMIT 1
        """,
        (utc_now(), run_id, run_id),
    ).fetchone()
    return row is not None


def why(conn: sqlite3.Connection, *, target_id: str) -> JsonObject:
    """Explain a state id by returning related rows and events."""
    run = conn.execute("SELECT * FROM runs WHERE run_id = ?", (target_id,)).fetchone()
    if run is not None:
        run_id = str(run["run_id"])
        return {
            "target_type": "run",
            "run": dict(run),
            "jobs": [dict(row) for row in jobs_for_run(conn, run_id=run_id)],
            "open_blockers": blocker_summaries(conn, run_id=run_id, severity=None),
            "events": events_for(conn, run_id=run_id),
            "next_actions": status(conn, run_id=run_id)["next_actions"],
        }

    job = conn.execute("SELECT * FROM jobs WHERE job_id = ? OR workflow_job_id = ?", (target_id, target_id)).fetchone()
    message = conn.execute("SELECT * FROM queue_messages WHERE message_id = ?", (target_id,)).fetchone()
    if job is not None or message is not None:
        job_id = str(job["job_id"] if job is not None else message["job_id"])
        return {
            "target_type": "job" if job is not None else "message",
            "job": dict(job) if job is not None else dict(row_by_id(conn, "jobs", "job_id", job_id)),
            "message": dict(message) if message is not None else None,
            "verdict": latest_verdict_row(conn, job_id=job_id),
            "blockers": blockers_for_job(conn, job_id=job_id),
            "downstream_jobs": downstream_jobs(conn, job_id=job_id),
            "events": events_for(conn, job_id=job_id),
        }

    blocker = conn.execute("SELECT * FROM blockers WHERE blocker_id = ?", (target_id,)).fetchone()
    if blocker is not None:
        blocker_job_id = str(blocker["job_id"]) if blocker["job_id"] is not None else None
        run_id = str(blocker["run_id"])
        blocked_jobs = (
            downstream_jobs(conn, job_id=blocker_job_id) if blocker_job_id is not None else []
        )
        blocker_payload = dict(blocker)
        if blocker_job_id is not None:
            blocker_job = row_by_id(conn, "jobs", "job_id", blocker_job_id)
            blocker_payload["workflow_job_id"] = blocker_job["workflow_job_id"]
            blocker_payload["job_state"] = blocker_job["state"]
        checkpoint_context = (
            human_checkpoint_context(blocker_payload, blocked_jobs=blocked_jobs)
            if blocker["severity"] == "human_checkpoint"
            else None
        )
        return {
            "target_type": "blocker",
            "blocker": blocker_payload,
            "run": dict(row_by_id(conn, "runs", "run_id", run_id)),
            "job": dict(row_by_id(conn, "jobs", "job_id", blocker_job_id))
            if blocker_job_id is not None
            else None,
            "session": dict(row_by_id(conn, "sessions", "session_id", str(blocker["session_id"])))
            if blocker["session_id"] is not None
            else None,
            "related_verdict": latest_verdict_row(conn, job_id=blocker_job_id)
            if blocker_job_id is not None
            else None,
            "blocked_downstream_jobs": blocked_jobs,
            "human_checkpoint": checkpoint_context,
            "next_actions": checkpoint_context["next_actions"]
            if checkpoint_context is not None
            else ["inspect_blocker", "export_run_evidence"],
            "events": events_for(conn, job_id=blocker_job_id)
            if blocker_job_id is not None
            else events_for(conn, run_id=run_id),
        }

    artifact = conn.execute("SELECT * FROM artifacts WHERE artifact_id = ?", (target_id,)).fetchone()
    if artifact is not None:
        artifact_job_id = str(artifact["job_id"]) if artifact["job_id"] is not None else None
        return {
            "target_type": "artifact",
            "artifact": dict(artifact),
            "job": dict(row_by_id(conn, "jobs", "job_id", artifact_job_id))
            if artifact_job_id is not None
            else None,
            "verdicts": verdicts_for_artifact(conn, artifact_id=target_id),
            "events": events_for(conn, artifact_id=target_id),
        }

    verdict = conn.execute("SELECT * FROM verdicts WHERE verdict_id = ?", (target_id,)).fetchone()
    if verdict is not None:
        artifact_id = verdict["findings_artifact_id"]
        return {
            "target_type": "verdict",
            "verdict": dict(verdict),
            "job": dict(row_by_id(conn, "jobs", "job_id", str(verdict["job_id"]))),
            "artifact": dict(row_by_id(conn, "artifacts", "artifact_id", str(artifact_id)))
            if artifact_id is not None
            else None,
            "blockers": blockers_for_job(conn, job_id=str(verdict["job_id"])),
            "events": events_for(conn, job_id=str(verdict["job_id"])),
        }

    session = conn.execute("SELECT * FROM sessions WHERE session_id = ? OR slug = ?", (target_id, target_id)).fetchone()
    if session is not None:
        return {
            "target_type": "session",
            "session": dict(session),
            "jobs": jobs_for_session(conn, session_id=str(session["session_id"])),
            "events": events_for(conn, session_id=str(session["session_id"])),
        }

    if table_exists(conn, "process_executions"):
        process = conn.execute("SELECT * FROM process_executions WHERE process_id = ?", (target_id,)).fetchone()
        if process is not None:
            return {
                "target_type": "process",
                "process": dict(process),
                "job": dict(row_by_id(conn, "jobs", "job_id", str(process["job_id"]))),
                "session": dict(row_by_id(conn, "sessions", "session_id", str(process["session_id"]))),
                "events": events_for_process(conn, process_id=target_id),
            }

    raise NotFoundError(
        "target id is not a known run, job, message, blocker, artifact, verdict, session, or process"
    )


def blocker_summaries(conn: sqlite3.Connection, *, run_id: str | None, severity: str | None) -> list[JsonObject]:
    """Return open blocker summaries."""
    rows = conn.execute(
        """
        SELECT b.blocker_id, b.run_id, b.job_id, b.session_id, b.severity,
               b.blocker_kind, b.description, b.state, j.workflow_job_id, j.state AS job_state
        FROM blockers b
        LEFT JOIN jobs j ON j.job_id = b.job_id
        WHERE b.state = 'open'
          AND (? IS NULL OR b.run_id = ?)
          AND (? IS NULL OR b.severity = ?)
        ORDER BY b.created_at
        """,
        (run_id, run_id, severity, severity),
    ).fetchall()
    summaries: list[JsonObject] = []
    for row in rows:
        summary = dict(row)
        if row["severity"] == "human_checkpoint":
            job_id = str(row["job_id"]) if row["job_id"] is not None else None
            blocked_jobs = downstream_jobs(conn, job_id=job_id) if job_id is not None else []
            summary["human_checkpoint"] = human_checkpoint_context(summary, blocked_jobs=blocked_jobs)
        summaries.append(summary)
    return summaries


def human_checkpoint_context(blocker: JsonObject, *, blocked_jobs: list[JsonObject]) -> JsonObject:
    """Return explicit operator context for a human checkpoint."""
    affected_jobs: list[JsonObject] = []
    if blocker.get("job_id") is not None:
        affected_jobs.append(
            {
                "job_id": blocker["job_id"],
                "workflow_job_id": blocker.get("workflow_job_id"),
                "state": blocker.get("job_state"),
                "relationship": "checkpoint_job",
            }
        )
    for job in blocked_jobs:
        affected_jobs.append(
            {
                "job_id": job["job_id"],
                "workflow_job_id": job["workflow_job_id"],
                "state": job["state"],
                "relationship": "blocked_downstream",
            }
        )
    return {
        "decision_required": "Human decision required before downstream work can continue.",
        "unblock_path": [
            "inspect_checkpoint_blocker",
            "review_related_verdict_and_artifact",
            "record_owner_decision_or_adjust_workflow",
            "resume_or_requeue_affected_work",
        ],
        "affected_jobs": affected_jobs,
        "next_actions": ["inspect_blocker", "resolve_human_checkpoint", "export_run_evidence"],
    }


def latest_non_accepting_verdicts(conn: sqlite3.Connection, *, run_id: str | None) -> list[JsonObject]:
    """Return latest non-accepting verdicts on waiting or failed review jobs."""
    rows = conn.execute(
        """
        SELECT v.verdict_id, v.run_id, v.job_id, j.workflow_job_id, j.state AS job_state,
               v.session_id, v.verdict, v.findings_artifact_id, v.rationale
        FROM verdicts v
        JOIN jobs j ON j.job_id = v.job_id
        WHERE j.job_type = 'review'
          AND j.state IN ('waiting_human','failed')
          AND v.verdict NOT IN ('accept','accept_with_findings')
          AND (? IS NULL OR v.run_id = ?)
          AND v.created_at = (
            SELECT MAX(v2.created_at) FROM verdicts v2 WHERE v2.job_id = v.job_id
          )
        ORDER BY v.created_at
        """,
        (run_id, run_id),
    ).fetchall()
    return [dict(row) for row in rows]


def claimable_jobs_by_role_lane(conn: sqlite3.Connection, *, run_id: str | None) -> list[JsonObject]:
    """Return pending work grouped by target role and lane."""
    rows = conn.execute(
        """
        SELECT qm.target_role_id AS role_id, qm.target_lane_id AS lane_id,
               COUNT(*) AS count, GROUP_CONCAT(j.workflow_job_id) AS workflow_job_ids
        FROM queue_messages qm
        JOIN jobs j ON j.job_id = qm.job_id
        WHERE qm.kind = 'work' AND qm.state = 'pending'
          AND (? IS NULL OR qm.run_id = ?)
        GROUP BY qm.target_role_id, qm.target_lane_id
        ORDER BY qm.target_role_id, qm.target_lane_id
        """,
        (run_id, run_id),
    ).fetchall()
    result: list[JsonObject] = []
    for row in rows:
        workflow_ids = str(row["workflow_job_ids"] or "").split(",") if row["workflow_job_ids"] else []
        result.append(
            {
                "role_id": row["role_id"],
                "lane_id": row["lane_id"],
                "count": int(row["count"]),
                "workflow_job_ids": workflow_ids,
            }
        )
    return result


def blocked_downstream_jobs(conn: sqlite3.Connection, *, run_id: str | None) -> list[JsonObject]:
    """Return blocked jobs with unsatisfied upstream dependency context."""
    jobs = conn.execute(
        """
        SELECT * FROM jobs
        WHERE state = 'blocked' AND (? IS NULL OR run_id = ?)
        ORDER BY workflow_job_id
        """,
        (run_id, run_id),
    ).fetchall()
    result: list[JsonObject] = []
    for job in jobs:
        dependencies = dependency_context(conn, job_id=str(job["job_id"]))
        if not dependencies:
            continue
        result.append(
            {
                "job_id": job["job_id"],
                "workflow_job_id": job["workflow_job_id"],
                "state": job["state"],
                "role_id": job["role_id"],
                "lane": json_loads(str(job["lane_selector_json"])).get("lane_id"),
                "blocked_by": dependencies,
            }
        )
    return result


def dependency_context(conn: sqlite3.Connection, *, job_id: str) -> list[JsonObject]:
    """Return dependency rows with upstream state and verdict context."""
    dependencies = conn.execute(
        """
        SELECT dep.depends_on_job_id, dep.gate_json, up.workflow_job_id, up.state, up.job_type
        FROM job_dependencies dep
        JOIN jobs up ON up.job_id = dep.depends_on_job_id
        WHERE dep.job_id = ?
        ORDER BY up.workflow_job_id
        """,
        (job_id,),
    ).fetchall()
    result: list[JsonObject] = []
    for dependency in dependencies:
        gate = json_loads(str(dependency["gate_json"]))
        verdict = latest_verdict(conn, job_id=str(dependency["depends_on_job_id"]))
        satisfied = dependency["state"] == "completed"
        required = gate.get("requires_verdict")
        if isinstance(required, list):
            satisfied = satisfied and verdict in set(required)
        if satisfied:
            continue
        result.append(
            {
                "depends_on_job_id": dependency["depends_on_job_id"],
                "workflow_job_id": dependency["workflow_job_id"],
                "state": dependency["state"],
                "required_verdicts": required,
                "latest_verdict": verdict,
            }
        )
    return result


def next_actions(
    *,
    open_blockers: list[JsonObject],
    human_checkpoints: list[JsonObject],
    non_accepting_verdicts: list[JsonObject],
    claimable_jobs: list[JsonObject],
    has_orphan_supervisor: bool = False,
) -> list[str]:
    """Return deterministic coordinator next-action names."""
    actions: list[str] = []
    if claimable_jobs:
        actions.append("claim_available_work")
    if has_orphan_supervisor:
        # HARNESS-001: a lost supervisor with a still-held lease silently
        # blocks the run. Surface a stable action name so dashboards and
        # scripts can react before the lease expires (default 30 minutes
        # is a long time to wait if the operator is not watching).
        actions.append("recover_orphan_supervisor")
    if open_blockers:
        actions.extend(["inspect_blocker", "export_run_evidence"])
    if human_checkpoints:
        actions.append("resolve_human_checkpoint")
    if non_accepting_verdicts:
        actions.append("revise_workflow_cycle")
    return list(dict.fromkeys(actions))


def downstream_jobs(conn: sqlite3.Connection, *, job_id: str) -> list[JsonObject]:
    """Return immediate downstream jobs and their dependency context."""
    rows = conn.execute(
        """
        SELECT j.* FROM job_dependencies dep
        JOIN jobs j ON j.job_id = dep.job_id
        WHERE dep.depends_on_job_id = ?
        ORDER BY j.workflow_job_id
        """,
        (job_id,),
    ).fetchall()
    return [
        {
            "job_id": row["job_id"],
            "workflow_job_id": row["workflow_job_id"],
            "state": row["state"],
            "blocked_by": dependency_context(conn, job_id=str(row["job_id"])),
        }
        for row in rows
    ]


def latest_verdict_row(conn: sqlite3.Connection, *, job_id: str | None) -> JsonObject | None:
    """Return the latest verdict row for a job."""
    if job_id is None:
        return None
    row = conn.execute(
        "SELECT * FROM verdicts WHERE job_id = ? ORDER BY created_at DESC, verdict_id DESC LIMIT 1",
        (job_id,),
    ).fetchone()
    return dict(row) if row is not None else None


def run_graph(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    output_format: str = "mermaid",
    graph_orient: str = "tb",
    graph_style: str = "layered",
) -> JsonObject | str:
    """Render the workflow graph for a run, annotated with current job states.

    For ``mermaid`` format, returns a string with Mermaid ``classDef`` lines and
    per-node ``class`` assignments so renderers can highlight job state.

    For ``json`` format, returns a dict whose graph nodes carry an additional
    ``current_state`` (defaulting to ``pending`` when the job has not yet been
    materialised), ``attempt`` (highest known attempt for that workflow job),
    and, for review jobs, ``latest_verdict``.
    """
    expire_leases(conn, run_id=run_id)
    run = row_by_id(conn, "runs", "run_id", run_id)
    snapshot = row_by_id(
        conn, "workflow_snapshots", "workflow_snapshot_id", str(run["workflow_snapshot_id"])
    )
    workflow = json_loads(str(snapshot["workflow_json"]))

    # RFC 0016 V1: shared compute_node_states helper. Workflow jobs that
    # have not been materialised yet are absent from the dict; callers
    # fall back to "pending".
    from striatum.workflow import compute_node_states

    node_states = compute_node_states(conn, run_id=run_id)

    # Some downstream paths still want row-level detail (attempt,
    # job_type, latest_verdict) for the JSON envelope; build that here.
    job_rows = conn.execute(
        """
        SELECT job_id, workflow_job_id, state, attempt, job_type
        FROM jobs
        WHERE run_id = ?
        ORDER BY workflow_job_id, attempt DESC
        """,
        (run_id,),
    ).fetchall()
    latest_by_workflow_job: dict[str, sqlite3.Row] = {}
    for row in job_rows:
        wf_job_id = str(row["workflow_job_id"])
        if wf_job_id not in latest_by_workflow_job:
            latest_by_workflow_job[wf_job_id] = row

    if output_format == "mermaid":
        return workflow_graph_mermaid(workflow, node_states=node_states)

    if output_format == "ascii":
        # RFC 0016 V1: reuse the dashboard's pure renderer for a
        # one-shot ASCII snapshot.
        from striatum.dashboard import render_graph_panel

        lines = render_graph_panel(
            workflow=workflow,
            node_states=node_states,
            width=80,
            height_budget=10000,
            style=graph_style,
            orient=graph_orient,
            color=False,
        )
        return "\n".join(lines) + "\n"

    if output_format != "json":
        raise InvalidTransitionError(f"unknown run graph format {output_format!r}")

    graph_payload = workflow_graph_data(workflow)
    graph = graph_payload["graph"]
    assert isinstance(graph, dict)
    nodes = graph.get("nodes", [])
    assert isinstance(nodes, list)
    annotated_nodes: list[JsonObject] = []
    for node in nodes:
        assert isinstance(node, dict)
        annotated = dict(node)
        wf_id = str(node["job_id"])
        row = latest_by_workflow_job.get(wf_id)
        if row is None:
            annotated["current_state"] = "pending"
            annotated["attempt"] = None
            annotated["state_class"] = mermaid_state_class("pending")
        else:
            state = str(row["state"])
            annotated["current_state"] = state
            annotated["attempt"] = int(row["attempt"])
            annotated["state_class"] = mermaid_state_class(state)
            if str(row["job_type"]) == "review":
                verdict_row = latest_verdict_row(conn, job_id=str(row["job_id"]))
                if verdict_row is not None:
                    annotated["latest_verdict"] = {
                        "verdict_id": verdict_row["verdict_id"],
                        "verdict": verdict_row["verdict"],
                        "rationale": verdict_row.get("rationale"),
                        "findings_artifact_id": verdict_row.get("findings_artifact_id"),
                        "posture": verdict_row.get("posture", "neutral"),
                    }
                else:
                    annotated["latest_verdict"] = None
        annotated_nodes.append(annotated)
    graph["nodes"] = annotated_nodes
    return {
        "run_id": run_id,
        "workflow_id": graph_payload["workflow_id"],
        "workflow_version": graph_payload.get("workflow_version"),
        "graph": graph,
    }


def blockers_for_job(conn: sqlite3.Connection, *, job_id: str) -> list[JsonObject]:
    """Return blockers for a job."""
    rows = conn.execute("SELECT * FROM blockers WHERE job_id = ? ORDER BY created_at", (job_id,)).fetchall()
    return [dict(row) for row in rows]


def verdicts_for_artifact(conn: sqlite3.Connection, *, artifact_id: str) -> list[JsonObject]:
    """Return verdicts that cite an artifact."""
    rows = conn.execute(
        "SELECT * FROM verdicts WHERE findings_artifact_id = ? ORDER BY created_at",
        (artifact_id,),
    ).fetchall()
    return [dict(row) for row in rows]


def jobs_for_run(conn: sqlite3.Connection, *, run_id: str) -> list[sqlite3.Row]:
    """Return jobs for a run."""
    return conn.execute(
        "SELECT * FROM jobs WHERE run_id = ? ORDER BY workflow_job_id, attempt",
        (run_id,),
    ).fetchall()


def jobs_for_session(conn: sqlite3.Connection, *, session_id: str) -> list[JsonObject]:
    """Return jobs touched by a session."""
    rows = conn.execute(
        """
        SELECT DISTINCT j.*
        FROM jobs j
        LEFT JOIN leases l ON l.resource_id = j.job_id
        LEFT JOIN verdicts v ON v.job_id = j.job_id
        WHERE l.owner_session_id = ? OR v.session_id = ?
        ORDER BY j.workflow_job_id
        """,
        (session_id, session_id),
    ).fetchall()
    return [dict(row) for row in rows]


def events_for(
    conn: sqlite3.Connection,
    *,
    run_id: str | None = None,
    job_id: str | None = None,
    session_id: str | None = None,
    artifact_id: str | None = None,
) -> list[JsonObject]:
    """Return matching append-only events."""
    clauses: list[str] = []
    values: list[str] = []
    if run_id is not None:
        clauses.append("run_id = ?")
        values.append(run_id)
    if job_id is not None:
        clauses.append("job_id = ?")
        values.append(job_id)
    if session_id is not None:
        clauses.append("actor_session_id = ?")
        values.append(session_id)
    if artifact_id is not None:
        clauses.append("artifact_id = ?")
        values.append(artifact_id)
    where = " AND ".join(clauses) if clauses else "1 = 1"
    rows = conn.execute(
        f"SELECT event_id, event_type, payload_json FROM events WHERE {where} ORDER BY event_id",
        values,
    ).fetchall()
    return [dict(row) for row in rows]


def events_for_process(conn: sqlite3.Connection, *, process_id: str) -> list[JsonObject]:
    """Return process lifecycle events for a process id."""
    rows = conn.execute(
        """
        SELECT e.event_id, e.event_type, e.payload_json
        FROM events e
        JOIN process_executions p ON p.run_id = e.run_id AND p.job_id = e.job_id
        WHERE p.process_id = ?
          AND e.event_type LIKE 'process.%'
        ORDER BY e.event_id
        """,
        (process_id,),
    ).fetchall()
    result: list[JsonObject] = []
    for row in rows:
        payload = json_loads(str(row["payload_json"]))
        if payload.get("process_id") != process_id:
            continue
        result.append(dict(row))
    return result


def recent_events_for_run(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    limit: int = 10,
) -> list[JsonObject]:
    """Return the most recent events for a run with workflow_job_id context.

    Used by the dashboard renderer; kept as a thin helper so dashboard.py
    does not have to write SQL directly.
    """
    capped = max(1, min(int(limit), 1000))
    rows = conn.execute(
        f"""
        SELECT e.event_id, e.event_type, e.payload_json, e.created_at,
               e.job_id, j.workflow_job_id
        FROM events e
        LEFT JOIN jobs j ON j.job_id = e.job_id
        WHERE e.run_id = ?
        ORDER BY e.event_id DESC
        LIMIT {capped}
        """,
        (run_id,),
    ).fetchall()
    return list(reversed([dict(row) for row in rows]))


def table_exists(conn: sqlite3.Connection, table_name: str) -> bool:
    """Return whether a SQLite table exists."""
    row = conn.execute(
        "SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?",
        (table_name,),
    ).fetchone()
    return row is not None


def doctor(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    run_id: str | None,
    verbose: bool = False,
) -> JsonObject:
    """Return consistency checks for the state database.

    The string ``problems`` list is the historical, stable contract that
    callers (status renderers, evidence export, smoke scripts) depend on. When
    ``verbose`` is true the payload is augmented with ``problem_records`` —
    structured rows describing each problem with a stable ``check`` name and
    minimal ``context``. The string list is preserved verbatim regardless of
    the verbose flag so callers that grep or display ``problems`` keep working.
    """
    # Stable check names. These are the public vocabulary for problem_records
    # and should be considered API: existing names must not be renamed without
    # a deprecation cycle. New checks should add a new constant here.
    DOCTOR_CHECKS: tuple[str, ...] = (
        "active_job_without_active_lease",
        "dependency_gate_json_invalid",
        "completed_review_dependency_lacks_accepting_verdict",
        "required_artifact_kind_or_path_mismatch",
        "run_running_with_no_progressable_jobs",
        "stale_queue_message_claim",
        "job_current_message_id_inconsistent",
        "job_current_lease_id_inconsistent",
        "active_session_on_terminal_run",
        "unreaped_expired_lease",
        "open_blocker_on_terminal_run",
        "orphan_work_packet",
        "worktree_orphaned_lease",
        "worktree_path_missing_on_disk",
        "supervisor_lost_with_held_lease",
        "process_running_but_pid_gone",
        "process_running_with_expired_lease",
        "editable_install_outside_repo",
        "reviewer_independence_unverified",
        "skills_missing",
        "skills_outdated",
        "plugin_missing",
        "plugin_outdated",
        "sealed_patch_unsupported",
    )
    problems: list[str] = []
    records: list[JsonObject] = []

    def report(check: str, *, identifier: str, message: str, context: JsonObject) -> None:
        # Keep the legacy string and the structured record in lockstep so
        # callers cannot drift between the two views.
        assert check in DOCTOR_CHECKS, f"unknown doctor check: {check}"
        problems.append(message)
        records.append({"check": check, "id": identifier, "context": context})

    sealed_rows = conn.execute(
        """
        SELECT r.run_id, r.state, ws.workflow_json
        FROM runs r
        JOIN workflow_snapshots ws
          ON ws.workflow_snapshot_id = r.workflow_snapshot_id
        WHERE r.state NOT IN ('completed','failed','canceled')
          AND (? IS NULL OR r.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in sealed_rows:
        try:
            workflow = json.loads(str(row["workflow_json"]))
        except json.JSONDecodeError:
            continue
        if not isinstance(workflow, dict):
            continue
        if workflow.get("provenance_mode", "advisory") != "sealed_patch":
            continue
        report(
            "sealed_patch_unsupported",
            identifier=str(row["run_id"]),
            message=(
                "sealed_patch run cannot start on this runner: no hard "
                f"containment mechanism is available for run {row['run_id']}"
            ),
            context={
                "run_id": row["run_id"],
                "run_state": row["state"],
                "provenance_mode": "sealed_patch",
                "reason": "containment_unsupported",
            },
        )

    orphan_jobs = conn.execute(
        """
        SELECT j.job_id, j.workflow_job_id, j.state
        FROM jobs j
        LEFT JOIN leases l ON l.lease_id = j.current_lease_id AND l.state = 'active'
        WHERE j.state IN ('claimed','running') AND l.lease_id IS NULL
          AND (? IS NULL OR j.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in orphan_jobs:
        report(
            "active_job_without_active_lease",
            identifier=str(row["job_id"]),
            message=f"active job without active lease: {row['job_id']}",
            context={
                "workflow_job_id": row["workflow_job_id"],
                "job_state": row["state"],
            },
        )
    dependencies = conn.execute(
        """
        SELECT dep.job_id, dep.depends_on_job_id, dep.gate_json
        FROM job_dependencies dep
        JOIN jobs upstream ON upstream.job_id = dep.depends_on_job_id
        WHERE (? IS NULL OR upstream.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in dependencies:
        try:
            gate = json_loads(str(row["gate_json"]))
        except (json.JSONDecodeError, InvalidTransitionError):
            report(
                "dependency_gate_json_invalid",
                identifier=str(row["depends_on_job_id"]),
                message=(
                    f"dependency gate_json is invalid: {row['depends_on_job_id']} -> {row['job_id']}"
                ),
                context={
                    "depends_on_job_id": row["depends_on_job_id"],
                    "downstream_job_id": row["job_id"],
                },
            )
            continue
        if gate.get("requires_verdict") is None:
            continue
        upstream = row_by_id(conn, "jobs", "job_id", str(row["depends_on_job_id"]))
        if upstream["state"] == "completed" and latest_verdict(conn, job_id=str(upstream["job_id"])) not in {
            "accept",
            "accept_with_findings",
        }:
            report(
                "completed_review_dependency_lacks_accepting_verdict",
                identifier=str(upstream["job_id"]),
                message=(
                    "completed review dependency lacks accepting verdict: "
                    f"{upstream['workflow_job_id']} -> {row['job_id']}"
                ),
                context={
                    "upstream_workflow_job_id": upstream["workflow_job_id"],
                    "downstream_job_id": row["job_id"],
                    "latest_verdict": latest_verdict(conn, job_id=str(upstream["job_id"])),
                },
            )
    jobs = conn.execute(
        "SELECT job_id, expected_artifacts_json FROM jobs WHERE (? IS NULL OR run_id = ?)",
        (run_id, run_id),
    ).fetchall()
    for job in jobs:
        expected = json.loads(str(job["expected_artifacts_json"]))
        if not isinstance(expected, list):
            continue
        for item in expected:
            if not isinstance(item, dict) or item.get("required") is not True:
                continue
            logical_name = item.get("logical_name")
            existing = conn.execute(
                "SELECT artifact_kind, repo_path FROM artifacts WHERE job_id = ? AND logical_name = ?",
                (job["job_id"], logical_name),
            ).fetchone()
            if existing is None:
                continue
            if existing["artifact_kind"] != item.get("kind") or existing["repo_path"] != item.get("path"):
                report(
                    "required_artifact_kind_or_path_mismatch",
                    identifier=str(job["job_id"]),
                    message=(
                        "required artifact mismatch: "
                        f"job_id={job['job_id']}, logical_name={logical_name!r}, "
                        f"expected kind={item.get('kind')!r}, path={item.get('path')!r}"
                    ),
                    context={
                        "logical_name": logical_name,
                        "expected_kind": item.get("kind"),
                        "expected_path": item.get("path"),
                        "actual_kind": existing["artifact_kind"],
                        "actual_path": existing["repo_path"],
                    },
                )
    stuck_runs = conn.execute(
        """
        SELECT r.run_id
        FROM runs r
        WHERE r.state = 'running'
          AND (? IS NULL OR r.run_id = ?)
          AND NOT EXISTS (
            SELECT 1 FROM jobs j
            WHERE j.run_id = r.run_id
              AND j.state IN (
                'queued','claimed','running','blocked','stale_lease','waiting_human'
              )
          )
          AND EXISTS (
            SELECT 1 FROM jobs j
            WHERE j.run_id = r.run_id AND j.state = 'failed'
          )
        """,
        (run_id, run_id),
    ).fetchall()
    for row in stuck_runs:
        report(
            "run_running_with_no_progressable_jobs",
            identifier=str(row["run_id"]),
            message=f"run is running but no progressable jobs remain: {row['run_id']}",
            context={"run_state": "running"},
        )
    stale_messages = conn.execute(
        """
        SELECT m.message_id, m.current_lease_id, m.state
        FROM queue_messages m
        LEFT JOIN leases l ON l.lease_id = m.current_lease_id AND l.state = 'active'
        WHERE m.state IN ('claimed','acked')
          AND m.current_lease_id IS NOT NULL
          AND l.lease_id IS NULL
          AND (? IS NULL OR m.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in stale_messages:
        report(
            "stale_queue_message_claim",
            identifier=str(row["message_id"]),
            message=f"queue message has stale claim: {row['message_id']}",
            context={
                "message_state": row["state"],
                "current_lease_id": row["current_lease_id"],
            },
        )
    bad_message_pointers = conn.execute(
        """
        SELECT j.job_id, j.current_message_id
        FROM jobs j
        LEFT JOIN queue_messages m ON m.message_id = j.current_message_id
        WHERE j.current_message_id IS NOT NULL
          AND (m.message_id IS NULL OR m.job_id IS NOT j.job_id)
          AND (? IS NULL OR j.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in bad_message_pointers:
        report(
            "job_current_message_id_inconsistent",
            identifier=str(row["job_id"]),
            message=f"job current_message_id is inconsistent: {row['job_id']}",
            context={"current_message_id": row["current_message_id"]},
        )
    bad_lease_pointers = conn.execute(
        """
        SELECT j.job_id, j.current_lease_id
        FROM jobs j
        LEFT JOIN leases l ON l.lease_id = j.current_lease_id
        WHERE j.current_lease_id IS NOT NULL
          AND (l.lease_id IS NULL OR l.resource_id IS NOT j.job_id)
          AND (? IS NULL OR j.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in bad_lease_pointers:
        report(
            "job_current_lease_id_inconsistent",
            identifier=str(row["job_id"]),
            message=f"job current_lease_id is inconsistent: {row['job_id']}",
            context={"current_lease_id": row["current_lease_id"]},
        )
    terminal_sessions = conn.execute(
        """
        SELECT s.session_id, s.run_id, r.state AS run_state
        FROM sessions s
        JOIN runs r ON r.run_id = s.run_id
        WHERE s.state = 'active'
          AND r.state IN ('completed','failed','canceled')
          AND (? IS NULL OR s.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in terminal_sessions:
        report(
            "active_session_on_terminal_run",
            identifier=str(row["session_id"]),
            message=f"active session on terminal run: {row['session_id']}",
            context={
                "run_id": row["run_id"],
                "run_state": row["run_state"],
            },
        )
    expired_leases = conn.execute(
        """
        SELECT l.lease_id, l.expires_at, l.resource_id
        FROM leases l
        WHERE l.state = 'active'
          AND l.expires_at < ?
          AND (? IS NULL OR l.run_id = ?)
        """,
        (utc_now(), run_id, run_id),
    ).fetchall()
    for row in expired_leases:
        report(
            "unreaped_expired_lease",
            identifier=str(row["lease_id"]),
            message=f"active lease has expired without reap: {row['lease_id']}",
            context={
                "expires_at": row["expires_at"],
                "resource_id": row["resource_id"],
            },
        )
    open_blockers = conn.execute(
        """
        SELECT b.blocker_id, b.run_id, r.state AS run_state, b.severity, b.blocker_kind
        FROM blockers b
        JOIN runs r ON r.run_id = b.run_id
        WHERE b.state = 'open'
          AND r.state IN ('completed','canceled')
          AND (? IS NULL OR b.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in open_blockers:
        report(
            "open_blocker_on_terminal_run",
            identifier=str(row["blocker_id"]),
            message=f"open blocker on terminal run: {row['blocker_id']}",
            context={
                "run_id": row["run_id"],
                "run_state": row["run_state"],
                "severity": row["severity"],
                "blocker_kind": row["blocker_kind"],
            },
        )
    orphan_packets = conn.execute(
        """
        SELECT p.packet_id, p.lease_id, p.session_id
        FROM work_packets p
        LEFT JOIN leases l ON l.lease_id = p.lease_id
        LEFT JOIN sessions s ON s.session_id = p.session_id
        WHERE (l.lease_id IS NULL OR s.session_id IS NULL)
          AND (? IS NULL OR p.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in orphan_packets:
        report(
            "orphan_work_packet",
            identifier=str(row["packet_id"]),
            message=f"work packet references missing lease/session: {row['packet_id']}",
            context={
                "lease_id": row["lease_id"],
                "session_id": row["session_id"],
            },
        )
    orphan_worktrees = conn.execute(
        """
        SELECT w.worktree_id, w.lease_id, w.worktree_path
        FROM job_worktrees w
        LEFT JOIN leases l ON l.lease_id = w.lease_id AND l.state = 'active'
        WHERE w.state = 'active' AND l.lease_id IS NULL
          AND (? IS NULL OR w.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in orphan_worktrees:
        report(
            "worktree_orphaned_lease",
            identifier=str(row["worktree_id"]),
            message=f"active worktree without active lease: {row['worktree_id']}",
            context={
                "lease_id": row["lease_id"],
                "worktree_path": row["worktree_path"],
            },
        )
    drifted_worktrees = conn.execute(
        """
        SELECT worktree_id, worktree_path
        FROM job_worktrees
        WHERE state = 'active' AND (? IS NULL OR run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in drifted_worktrees:
        target = repo / str(row["worktree_path"])
        if not target.exists():
            report(
                "worktree_path_missing_on_disk",
                identifier=str(row["worktree_id"]),
                message=(
                    "active worktree directory missing on disk: "
                    f"{row['worktree_id']} ({row['worktree_path']})"
                ),
                context={"worktree_path": row["worktree_path"]},
            )
    # RFC 0009: surface supervisors whose state claims liveness but whose
    # OS pid is gone, and attached supervisors whose stdin pipe has been
    # removed out from under them. Both are operator-actionable signals;
    # doctor never auto-mutates state.
    supervisor_active_placeholders = ",".join(["?"] * len(SUPERVISOR_ACTIVE_STATES))
    active_supervisors = conn.execute(
        f"""
        SELECT supervisor_id, pid, state, stdin_pipe_path
        FROM process_supervisors
        WHERE state IN ({supervisor_active_placeholders})
          AND (? IS NULL OR run_id = ?)
        """,
        (*SUPERVISOR_ACTIVE_STATES, run_id, run_id),
    ).fetchall()
    for row in active_supervisors:
        pid_value = row["pid"]
        if pid_value is None:
            continue
        try:
            os.kill(int(pid_value), 0)
        except ProcessLookupError:
            problems.append(f"supervisor pid is gone: {row['supervisor_id']}")
            continue
        except PermissionError:
            problems.append(f"supervisor pid is gone: {row['supervisor_id']}")
            continue
        if row["state"] == "attached":
            pipe_path = row["stdin_pipe_path"]
            if pipe_path is None or not Path(str(pipe_path)).exists():
                problems.append(
                    f"supervisor stdin pipe missing: {row['supervisor_id']}"
                )
    # HARNESS-001: a supervisor whose row is already ``lost`` but whose
    # session still holds an unexpired lease leaves the run silently
    # stuck. The pid-gone string-only emission above only fires while the
    # supervisor row is still in an active state; once the next CLI call
    # transitions it to ``lost``, that signal disappears. Surface the
    # held-lease consequence as a structured check so operators see it
    # via ``doctor --verbose`` and (via ``next_actions``) via ``status``.
    supervisor_lost_rows = conn.execute(
        """
        SELECT s.supervisor_id, s.run_id, s.session_id, s.pid, s.ended_at,
               s.stop_reason, l.lease_id, l.expires_at, l.resource_id
        FROM process_supervisors s
        JOIN leases l
          ON l.owner_session_id = s.session_id
         AND l.state = 'active'
         AND l.expires_at > ?
        WHERE s.state = 'lost'
          AND (? IS NULL OR s.run_id = ?)
        """,
        (utc_now(), run_id, run_id),
    ).fetchall()
    for row in supervisor_lost_rows:
        report(
            "supervisor_lost_with_held_lease",
            identifier=str(row["supervisor_id"]),
            message=(
                f"supervisor lost with held lease: {row['supervisor_id']} "
                f"(session={row['session_id']}, lease={row['lease_id']})"
            ),
            context={
                "run_id": row["run_id"],
                "session_id": row["session_id"],
                "lease_id": row["lease_id"],
                "lease_expires_at": row["expires_at"],
                "ended_at": row["ended_at"],
                "stop_reason": row["stop_reason"],
            },
        )
    # RFC 0014 V1: process_running_but_pid_gone — process_executions rows
    # in ``running`` state whose pid is no longer reachable from this
    # process. Surfaced so doctor flags externally-killed processes that
    # bypassed the runner's bookkeeping; the recommended action is
    # ``recovery process-reconcile``.
    process_running_rows = conn.execute(
        """
        SELECT pe.process_id, pe.run_id, pe.job_id, pe.pid, pe.started_at, pe.lease_id
        FROM process_executions pe
        WHERE pe.state = 'running'
          AND (? IS NULL OR pe.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    import os as _os_for_doctor
    for row in process_running_rows:
        pid = row["pid"]
        if pid is None:
            alive = False
        else:
            try:
                _os_for_doctor.kill(int(pid), 0)
                alive = True
            except ProcessLookupError:
                alive = False
            except PermissionError:
                alive = True
            except OSError:
                alive = True
        if not alive:
            report(
                "process_running_but_pid_gone",
                identifier=str(row["process_id"]),
                message=(
                    f"process_executions row {row['process_id']} is in 'running' "
                    f"state but pid {pid} is gone; run "
                    f"`striatum recovery process-reconcile --run-id {row['run_id']}` "
                    "to reconcile"
                ),
                context={
                    "run_id": row["run_id"],
                    "job_id": row["job_id"],
                    "pid": pid,
                    "started_at": row["started_at"],
                    "lease_id": row["lease_id"],
                },
            )
    # RFC 0014 V1: process_running_with_expired_lease — ``running`` rows
    # whose lease has expired. The pid may still be alive; doctor surfaces
    # the bookkeeping mismatch so an operator can decide whether to
    # reconcile or kill the lingering process.
    process_lease_rows = conn.execute(
        """
        SELECT pe.process_id, pe.run_id, pe.job_id, pe.pid, l.lease_id, l.expires_at
        FROM process_executions pe
        JOIN leases l ON l.lease_id = pe.lease_id
        WHERE pe.state = 'running' AND l.state = 'expired'
          AND (? IS NULL OR pe.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in process_lease_rows:
        report(
            "process_running_with_expired_lease",
            identifier=str(row["process_id"]),
            message=(
                f"process_executions row {row['process_id']} is in 'running' "
                f"state but lease {row['lease_id']} expired at "
                f"{row['expires_at']}; run "
                f"`striatum recovery process-reconcile --run-id {row['run_id']}`"
            ),
            context={
                "run_id": row["run_id"],
                "job_id": row["job_id"],
                "pid": row["pid"],
                "lease_id": row["lease_id"],
                "lease_expires_at": row["expires_at"],
            },
        )
    # HARNESS-002: warn loudly when the running ``striatum`` install is
    # outside the repo argument. Catches the foot-gun where ``pip install
    # -e`` silently pinned to a temporary worktree and the runner is now
    # behind on migrations or allowed-kinds.
    #
    # Only fires when the repo argument itself is a Striatum source tree
    # (carries ``src/striatum/migrations.py``); when the operator is
    # using the CLI as a tool against a foreign target repo, the install
    # path is *expected* to live outside the target. Without this guard
    # the check would be noise on every legitimate non-self-hosting run.
    if (repo / "src" / "striatum" / "migrations.py").is_file():
        install_path = Path(str(getattr(_install_module(), "__file__", ""))).resolve()
        repo_resolved = repo.resolve()
        try:
            install_path.relative_to(repo_resolved)
        except ValueError:
            report(
                "editable_install_outside_repo",
                identifier=str(install_path),
                message=(
                    "editable install is outside the repo: "
                    f"install={install_path} repo={repo_resolved}"
                ),
                context={
                    "install_path": str(install_path),
                    "repo_path": str(repo_resolved),
                },
            )
    # HARNESS-003: surface reviewer-independence breaches at the
    # observable layer (shared supervisor pid, or unsupervised reviewer
    # session co-existing with a supervised author session on the same
    # run). The runner cannot enforce true context independence; doctor
    # is the warning surface so the operator at least sees the breach.
    independence_rows = _reviewer_independence_breaches(conn, run_id=run_id)
    for breach in independence_rows:
        report(
            "reviewer_independence_unverified",
            identifier=str(breach["session_id"]),
            message=breach["message"],
            context=breach["context"],
        )
    # RFC 0015: skill-bundle drift checks. Surface missing or outdated
    # bundles so the operator runs `striatum skills install`. The runner
    # never auto-regenerates.
    _check_skill_bundle(repo=repo, report=report)
    schema_version = conn.execute(
        "SELECT value FROM schema_meta WHERE key = 'schema_version'"
    ).fetchone()
    payload: JsonObject = {
        "ok": len(problems) == 0,
        "schema_version": schema_version["value"] if schema_version is not None else None,
        "problems": problems,
    }
    if verbose:
        payload["problem_records"] = records
    return payload


def _check_skill_bundle(*, repo: Path, report: Callable[..., None]) -> None:
    """RFC 0015: surface missing / outdated skill bundles."""
    from striatum import __version__ as STRIATUM_VERSION
    from striatum.skills import (
        bundled_template_sha256,
        load_manifest,
        manifest_path_for,
        skill_files_present,
    )

    for profile in ("claude_code", "codex", "gemini", "generic"):
        try:
            path = manifest_path_for(target=repo, profile=profile, scope="project")
        except Exception:  # noqa: BLE001 - doctor must never crash the run
            continue
        manifest = load_manifest(path)
        if manifest is None:
            continue
        missing = skill_files_present(target=repo, manifest=manifest)
        if missing:
            report(
                "skills_missing",
                identifier=str(path),
                message=(
                    f"skill bundle missing files for profile {profile!r}: "
                    + ", ".join(missing)
                    + f" — run `striatum --repo {repo} skills install --profile {profile}`"
                ),
                context={
                    "profile": profile,
                    "manifest_path": str(path),
                    "missing": missing,
                    "recovery_command": (
                        f"striatum --repo {repo} skills install --profile {profile}"
                    ),
                },
            )
        recorded_version = str(manifest.get("striatum_version") or "")
        version_drift = (
            recorded_version != "" and recorded_version != STRIATUM_VERSION
        )
        template_drift: list[str] = []
        for entry in manifest.get("files", []):
            if not isinstance(entry, dict):
                continue
            tmpl = str(entry.get("template") or "")
            recorded = str(entry.get("template_sha256") or "")
            if not tmpl or not recorded:
                continue
            try:
                bundled = bundled_template_sha256(tmpl)
            except Exception:  # noqa: BLE001 - missing template -> drift
                template_drift.append(tmpl)
                continue
            if bundled != recorded:
                template_drift.append(tmpl)
        if version_drift or template_drift:
            report(
                "skills_outdated",
                identifier=str(path),
                message=(
                    f"skill bundle outdated for profile {profile!r}: "
                    f"manifest_version={recorded_version!r} "
                    f"running_version={STRIATUM_VERSION!r} "
                    f"templates_changed={template_drift!r} "
                    f"— run `striatum --repo {repo} skills install --profile {profile}`"
                ),
                context={
                    "profile": profile,
                    "manifest_path": str(path),
                    "manifest_version": recorded_version,
                    "running_version": STRIATUM_VERSION,
                    "templates_changed": template_drift,
                    "recovery_command": (
                        f"striatum --repo {repo} skills install --profile {profile}"
                    ),
                },
            )

    # RFC 0025 V1 Step 1: plugin bundle health.
    from striatum.plugins.install import (
        ALLOWED_PROFILES as PLUGIN_PROFILES,
        _bundled_template_sha256 as plugin_template_sha256,
        _load_manifest as load_plugin_manifest,
    )
    for plugin_profile in PLUGIN_PROFILES:
        bundle_root = repo / ".striatum" / "plugins" / plugin_profile
        plugin_manifest = load_plugin_manifest(bundle_root / ".manifest.json")
        if plugin_manifest is None:
            continue
        plugin_missing_files: list[str] = []
        for entry in plugin_manifest.get("files", []):
            if not isinstance(entry, dict):
                continue
            rel_path = str(entry.get("path") or "")
            if rel_path and not (bundle_root / rel_path).is_file():
                plugin_missing_files.append(rel_path)
        if plugin_missing_files:
            report(
                "plugin_missing",
                identifier=str(bundle_root / ".manifest.json"),
                message=(
                    f"plugin bundle missing files for profile {plugin_profile!r}: "
                    + ", ".join(plugin_missing_files)
                    + f" — run `striatum --repo {repo} plugin install --profile {plugin_profile}`"
                ),
                context={
                    "profile": plugin_profile,
                    "manifest_path": str(bundle_root / ".manifest.json"),
                    "missing": plugin_missing_files,
                    "recovery_command": (
                        f"striatum --repo {repo} plugin install --profile {plugin_profile}"
                    ),
                },
            )
        plugin_recorded_version = str(plugin_manifest.get("striatum_version") or "")
        plugin_version_drift = (
            plugin_recorded_version != "" and plugin_recorded_version != STRIATUM_VERSION
        )
        plugin_template_drift: list[str] = []
        for entry in plugin_manifest.get("files", []):
            if not isinstance(entry, dict):
                continue
            tmpl = str(entry.get("template") or "")
            recorded = str(entry.get("template_sha256") or "")
            if not tmpl or not recorded:
                continue
            try:
                bundled = plugin_template_sha256(tmpl)
            except Exception:  # noqa: BLE001
                plugin_template_drift.append(tmpl)
                continue
            if bundled != recorded:
                plugin_template_drift.append(tmpl)
        if plugin_version_drift or plugin_template_drift:
            report(
                "plugin_outdated",
                identifier=str(bundle_root / ".manifest.json"),
                message=(
                    f"plugin bundle outdated for profile {plugin_profile!r}: "
                    f"manifest_version={plugin_recorded_version!r} "
                    f"running_version={STRIATUM_VERSION!r} "
                    f"templates_changed={plugin_template_drift!r} "
                    f"— run `striatum --repo {repo} plugin install --profile {plugin_profile}`"
                ),
                context={
                    "profile": plugin_profile,
                    "manifest_path": str(bundle_root / ".manifest.json"),
                    "manifest_version": plugin_recorded_version,
                    "running_version": STRIATUM_VERSION,
                    "templates_changed": plugin_template_drift,
                    "recovery_command": (
                        f"striatum --repo {repo} plugin install --profile {plugin_profile}"
                    ),
                },
            )
