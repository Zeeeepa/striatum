"""Gated legacy service fallbacks for subprocess web fixtures.

Production service paths are daemon-first. These helpers exist only so old
subprocess web fixtures can keep exercising pre-cutover SQLite semantics under
the explicit test-harness environment gate.
"""

from __future__ import annotations

import json
import os
import sqlite3
import subprocess
import time
from http.server import BaseHTTPRequestHandler
from pathlib import Path
from typing import Any, Callable, Mapping

from striatum.db import db_path
from striatum.web.doctor import shape_doctor_records


JsonObject = dict[str, Any]


def legacy_fixture_fallback_enabled(code: str) -> bool:
    """Return whether service tests may exercise the pre-cutover SQLite path."""
    return code == "repo_not_registered" and os.environ.get("STRIATUM_TEST_HARNESS") == "1"


def legacy_artifact_raw_fallback_enabled(code: str) -> bool:
    return (
        code in {"daemon_unreachable", "repo_not_registered"}
        and os.environ.get("STRIATUM_TEST_HARNESS") == "1"
        and os.environ.get("STRIATUM_DAEMON_REQUIRED") == "0"
    )


def legacy_web_read_fallback_enabled(code: str) -> bool:
    return (
        code in {"daemon_unreachable", "repo_not_registered"}
        and os.environ.get("STRIATUM_TEST_HARNESS") == "1"
        and os.environ.get("STRIATUM_DAEMON_REQUIRED") == "0"
    )


def legacy_run_cancel(repo: Path, *, run_id: str, reason: str | None) -> JsonObject:
    from striatum.db import cancel_run, connect, transaction

    with connect(repo) as conn, transaction(conn):
        return cancel_run(conn, run_id=run_id, reason=reason)


def legacy_run_pause(repo: Path, *, run_id: str, reason: str | None) -> JsonObject:
    from striatum.db import connect, pause_run, transaction

    with connect(repo) as conn, transaction(conn):
        return pause_run(conn, run_id=run_id, reason=reason)


def legacy_run_resume(repo: Path, *, run_id: str) -> JsonObject:
    from striatum.db import connect, resume_run, transaction

    with connect(repo) as conn, transaction(conn):
        return resume_run(conn, run_id=run_id)


def legacy_workflow_run_now(repo: Path, *, workflow_path: Path) -> JsonObject:
    from striatum.cli.mutations import branch_confirm, run_start
    from striatum.db import connect, transaction
    from striatum.workflow import create_run

    with connect(repo) as conn:
        with transaction(conn):
            prepared = create_run(conn, repo=repo, workflow_path=workflow_path)
        run_id = str(prepared["run_id"])
        requires_confirm = (
            prepared.get("state") == "needs_branch_confirmation"
            and prepared.get("branch_mode") != "auto"
        )
        if prepared.get("branch_mode") == "auto":
            suggested = prepared.get("suggested_branch_name")
            if isinstance(suggested, str) and suggested:
                branch_confirm(
                    conn,
                    repo=repo,
                    run_id=run_id,
                    branch=suggested,
                    create=True,
                )
        if requires_confirm:
            return {
                "run_id": run_id,
                "status": "needs_branch_confirmation",
                "suggested_branch_name": prepared.get("suggested_branch_name"),
            }
        run_start(conn, run_id=run_id)
    return {"run_id": run_id, "status": "running"}


def legacy_job_cancel(
    repo: Path,
    *,
    run_id: str,
    job_id: str,
    reason: str,
    cascade: bool,
) -> JsonObject:
    from striatum.cli.recovery import cancel_job
    from striatum.db import connect

    with connect(repo) as conn:
        return cancel_job(
            conn,
            run_id=run_id,
            job_id=job_id,
            reason=reason,
            cascade=cascade,
        )


def legacy_job_retry(repo: Path, *, run_id: str, job_id: str) -> JsonObject:
    from striatum.db import connect, retry_job, transaction

    with connect(repo) as conn, transaction(conn):
        return retry_job(conn, run_id=run_id, job_id=job_id)


def legacy_run_detail_payload(repo: Path, *, run_id: str) -> JsonObject:
    from striatum.cli.introspect import status as status_command

    with sqlite3.connect(str(db_path(repo))) as conn:
        conn.row_factory = sqlite3.Row
        run_row = conn.execute(
            "SELECT * FROM runs WHERE run_id = ?", (run_id,)
        ).fetchone()
        if run_row is None:
            raise KeyError(run_id)
        run = dict(run_row)
        job_rows = conn.execute(
            "SELECT * FROM jobs WHERE run_id = ? ORDER BY workflow_job_id",
            (run_id,),
        ).fetchall()
        jobs = [dict(row) for row in job_rows]
        for job in jobs:
            lane_selector = _json_loads_object(job.get("lane_selector_json"), {})
            lane_id = lane_selector.get("lane_id") if isinstance(lane_selector, dict) else None
            job["lane_id"] = lane_id
            job["state_chip"] = _state_chip("job", job.get("state"))
            session_id = None
            packet = _latest_work_packet_for_job(conn, job_id=str(job["job_id"]))
            if job.get("current_lease_id"):
                lease_row = conn.execute(
                    "SELECT owner_session_id FROM leases WHERE lease_id = ?",
                    (job["current_lease_id"],),
                ).fetchone()
                if lease_row is not None:
                    session_id = str(lease_row["owner_session_id"])
            if session_id is None and packet and packet.get("session_id"):
                session_id = str(packet["session_id"])
            job["lane_attestation_chip"] = _lane_attestation_chip(
                conn,
                session_id=session_id,
            )
            artifact_rows = conn.execute(
                "SELECT * FROM artifacts WHERE job_id = ? ORDER BY created_at",
                (job["job_id"],),
            ).fetchall()
            artifacts = [dict(row) for row in artifact_rows]
            job["expected_artifact_rows"] = _expected_artifact_rows(
                job=job,
                artifacts=artifacts,
                packet=packet,
            )
            job["artifacts"] = legacy_shape_artifact_rows(
                conn,
                artifacts=artifacts,
                expected_rows=job["expected_artifact_rows"],
            )
            verdict_rows = conn.execute(
                """
                SELECT *
                FROM verdicts
                WHERE job_id = ?
                ORDER BY created_at DESC, verdict_id DESC
                """,
                (job["job_id"],),
            ).fetchall()
            shaped_verdicts = _shape_verdict_rows(
                conn,
                verdicts=[dict(row) for row in verdict_rows],
            )
            job["latest_verdict"] = shaped_verdicts[0] if shaped_verdicts else None
        snapshot_row = conn.execute(
            "SELECT workflow_json FROM workflow_snapshots WHERE workflow_snapshot_id = ?",
            (str(run["workflow_snapshot_id"]),),
        ).fetchone()
        workflow = (
            json.loads(str(snapshot_row["workflow_json"]))
            if snapshot_row is not None else {}
        )
        status_payload = status_command(conn, run_id=run_id)
        next_actions = status_payload.get("next_actions") or []
        recovery_panel = _recovery_panel_payload(
            conn,
            run_id=run_id,
            next_actions=list(next_actions),
            auto_finalize_dry_run=(
                status_payload.get("auto_finalize_dry_run")
                if isinstance(status_payload.get("auto_finalize_dry_run"), Mapping)
                else None
            ),
        )
        run["state_chip"] = _state_chip("run", run.get("state"))
    suggested_branch_name = ""
    allow_dirty = False
    try:
        br = workflow.get("branch") or {}
        if isinstance(br, dict):
            suggested_branch_name = str(br.get("suggested_name") or "")
            allow_dirty = bool(br.get("allow_dirty"))
    except (AttributeError, TypeError):
        pass
    return {
        "run": run,
        "jobs": jobs,
        "workflow": workflow,
        "next_actions": list(next_actions),
        "recovery_panel": recovery_panel,
        "verdicts_by_posture": status_payload.get("verdicts_by_posture") or {},
        "sessions": status_payload.get("sessions") or [],
        "phase_progress": status_payload.get("phases") or [],
        "current_phase_id": status_payload.get("current_phase_id"),
        "suggested_branch_name": suggested_branch_name,
        "allow_dirty": allow_dirty,
    }


def legacy_job_detail_payload(repo: Path, *, run_id: str, job_ref: str) -> JsonObject:
    from striatum.cli.introspect import latest_verdict_row

    with sqlite3.connect(str(db_path(repo))) as conn:
        conn.row_factory = sqlite3.Row
        run_row = conn.execute(
            "SELECT * FROM runs WHERE run_id = ?", (run_id,)
        ).fetchone()
        job_row = conn.execute(
            "SELECT * FROM jobs WHERE run_id = ? "
            "AND (job_id = ? OR workflow_job_id = ?)",
            (run_id, job_ref, job_ref),
        ).fetchone()
        if run_row is None or job_row is None:
            raise KeyError(job_ref)
        job_id = str(job_row["job_id"])
        run = dict(run_row)
        job = dict(job_row)
        lane_selector = _json_loads_object(job.get("lane_selector_json"), {})
        job["lane_id"] = (
            lane_selector.get("lane_id") if isinstance(lane_selector, dict) else None
        )
        job["state_chip"] = _state_chip("job", job.get("state"))
        run["state_chip"] = _state_chip("run", run.get("state"))
        artifact_rows = conn.execute(
            "SELECT * FROM artifacts WHERE job_id = ? ORDER BY created_at",
            (job_id,),
        ).fetchall()
        artifacts = [dict(row) for row in artifact_rows]
        packet = _latest_work_packet_for_job(conn, job_id=job_id)
        job["work_packet"] = packet
        job["expected_artifact_rows"] = _expected_artifact_rows(
            job=job,
            artifacts=artifacts,
            packet=packet,
        )
        artifacts = legacy_shape_artifact_rows(
            conn,
            artifacts=artifacts,
            expected_rows=job["expected_artifact_rows"],
        )
        job["lane_attestation_chip"] = _lane_attestation_chip(
            conn,
            session_id=str(packet["session_id"]) if packet and packet.get("session_id") else None,
        )
        job["process_evidence"] = _process_evidence_rows(
            conn,
            run_id=run_id,
            job_id=job_id,
        )
        verdict_rows = conn.execute(
            "SELECT * FROM verdicts WHERE job_id = ? ORDER BY created_at DESC, verdict_id DESC",
            (job_id,),
        ).fetchall()
        verdicts = _shape_verdict_rows(
            conn,
            verdicts=[dict(row) for row in verdict_rows],
        )
        latest_verdict = verdicts[0] if verdicts else latest_verdict_row(conn, job_id=job_id)
    return {
        "run": run,
        "job": job,
        "artifacts": artifacts,
        "latest_verdict": dict(latest_verdict) if isinstance(latest_verdict, sqlite3.Row) else latest_verdict,
        "verdicts": verdicts,
        "expected_artifact_rows": job["expected_artifact_rows"],
        "process_evidence": job["process_evidence"],
    }


def legacy_artifact_metadata(repo: Path, *, artifact_id: str) -> JsonObject | None:
    conn: sqlite3.Connection | None = None
    try:
        conn = sqlite3.connect(str(db_path(repo)))
        conn.row_factory = sqlite3.Row
        row = conn.execute(
            """
            SELECT artifact_id, run_id, job_id, session_id, logical_name,
                   artifact_kind, repo_path, content_sha256, size_bytes,
                   publish_mode, created_at, author_line
            FROM artifacts
            WHERE artifact_id = ?
            """,
            (artifact_id,),
        ).fetchone()
        return dict(row) if row is not None else None
    finally:
        if conn is not None:
            conn.close()


def legacy_run_list_items_for_test_harness(
    repo: Path,
    *,
    shape_run: Callable[[Mapping[str, Any]], dict[str, Any]],
) -> list[dict[str, Any]]:
    with sqlite3.connect(str(db_path(repo))) as conn:
        conn.row_factory = sqlite3.Row
        rows = conn.execute(
            """
            SELECT r.run_id, r.state, r.branch_name, r.created_at,
                   r.started_at, r.completed_at,
                   ws.workflow_snapshot_id, ws.workflow_id,
                   ws.workflow_version, ws.source_path,
                   ws.workflow_json
            FROM runs r
            LEFT JOIN workflow_snapshots ws
              ON ws.workflow_snapshot_id = r.workflow_snapshot_id
            ORDER BY r.created_at DESC
            """
        ).fetchall()
    runs = []
    for row in rows:
        run = dict(row)
        workflow_id = str(run.get("workflow_id") or "")
        workflow_name = workflow_id
        try:
            workflow = json.loads(str(run.pop("workflow_json") or "{}"))
            if isinstance(workflow, dict):
                workflow_id = workflow_id or str(workflow.get("workflow_id") or "")
                workflow_name = str(workflow.get("name") or workflow_name or workflow_id)
        except json.JSONDecodeError:
            run.pop("workflow_json", None)
        run["workflow_id"] = workflow_id
        run["workflow_name"] = workflow_name
        runs.append(shape_run(run))
    return runs


def legacy_run_posture_verdicts_payload(repo: Path, *, run_id: str, posture: str) -> JsonObject:
    with sqlite3.connect(str(db_path(repo))) as conn:
        conn.row_factory = sqlite3.Row
        run_row = conn.execute(
            "SELECT run_id, state, branch_name FROM runs WHERE run_id = ?", (run_id,),
        ).fetchone()
        if run_row is None:
            raise KeyError("run not found")
        rows = conn.execute(
            """
            SELECT v.verdict_id, v.verdict, v.rationale, v.created_at,
                   v.job_id, v.findings_artifact_id, v.session_id,
                   j.workflow_job_id, j.role_id, j.lane_selector_json,
                   s.slug AS session_slug
            FROM verdicts v
            JOIN jobs j ON j.job_id = v.job_id
            JOIN sessions s ON s.session_id = v.session_id
            WHERE v.run_id = ? AND v.posture = ?
            ORDER BY v.created_at DESC
            """,
            (run_id, posture),
        ).fetchall()
        verdicts = []
        for row in rows:
            item = dict(row)
            try:
                item["lane_id"] = (json.loads(item.get("lane_selector_json") or "{}")).get("lane_id")
            except (json.JSONDecodeError, TypeError):
                item["lane_id"] = None
            verdicts.append(item)
        verdicts = _shape_verdict_rows(conn, verdicts=verdicts)
    return {
        "run": dict(run_row),
        "posture": posture,
        "verdicts": verdicts,
        "count": len(verdicts),
    }


def legacy_artifact_view_payload(repo: Path, *, run_id: str, artifact_id: str) -> JsonObject:
    with sqlite3.connect(str(db_path(repo))) as conn:
        conn.row_factory = sqlite3.Row
        run_row = conn.execute(
            "SELECT * FROM runs WHERE run_id = ?", (run_id,)
        ).fetchone()
        artifact_row = conn.execute(
            "SELECT * FROM artifacts WHERE artifact_id = ? AND run_id = ?",
            (artifact_id, run_id),
        ).fetchone()
        if run_row is None or artifact_row is None:
            raise KeyError("not found")
        artifact = dict(artifact_row)
        job_row = None
        if artifact.get("job_id"):
            job_row = conn.execute(
                "SELECT * FROM jobs WHERE job_id = ?",
                (str(artifact["job_id"]),),
            ).fetchone()
        packet = (
            _latest_work_packet_for_job(conn, job_id=str(artifact["job_id"]))
            if artifact.get("job_id")
            else None
        )
        expected_rows = (
            _expected_artifact_rows(
                job=dict(job_row),
                artifacts=[artifact],
                packet=packet,
            )
            if job_row is not None
            else []
        )
        shaped = legacy_shape_artifact_rows(
            conn,
            artifacts=[artifact],
            expected_rows=expected_rows,
        )
        artifact = shaped[0]
        artifact["provenance_trail"] = _artifact_provenance_trail(
            conn,
            artifact_id=artifact_id,
            run_id=run_id,
        )
        return {"run": dict(run_row), "artifact": artifact}


def legacy_doctor_page_payload(repo: Path) -> JsonObject:
    from striatum.cli.introspect import doctor as doctor_command

    with sqlite3.connect(str(db_path(repo))) as conn:
        conn.row_factory = sqlite3.Row
        doctor_payload = doctor_command(conn, repo=repo, run_id=None, verbose=True)
    records = shape_doctor_records(list(doctor_payload.get("problem_records") or []))
    doctor_payload["problem_records"] = records
    return doctor_payload


def legacy_view_file_run_breadcrumb(repo: Path, *, rel_path: str) -> JsonObject | None:
    try:
        with sqlite3.connect(str(db_path(repo))) as conn:
            conn.row_factory = sqlite3.Row
            return _view_file_run_breadcrumb(conn, rel_path=rel_path)
    except sqlite3.Error:
        return None


def legacy_stream_events_body(
    handler: BaseHTTPRequestHandler,
    *,
    repo: Path,
    run_id: str,
    since: int,
    poll_interval_seconds: float,
) -> None:
    conn: sqlite3.Connection | None = None
    try:
        handler.send_response(200)
        handler.send_header("Content-Type", "text/event-stream")
        handler.send_header("Cache-Control", "no-cache")
        handler.send_header("Connection", "close")
        handler.send_header("X-Accel-Buffering", "no")
        handler.end_headers()
        conn = sqlite3.connect(str(db_path(repo)))
        conn.row_factory = sqlite3.Row
        last_id = since
        terminal_states = {"completed", "failed", "canceled"}
        state = getattr(handler, "state")
        write_event = getattr(handler, "_write_sse_event")
        while not state.shutting_down:
            rows = conn.execute(
                "SELECT * FROM events WHERE run_id = ? AND event_id > ? ORDER BY event_id LIMIT 100",
                (run_id, last_id),
            ).fetchall()
            for row in rows:
                payload = {
                    "event_id": int(row["event_id"]),
                    "run_id": row["run_id"],
                    "type": row["event_type"],
                    "actor_session_id": row["actor_session_id"],
                    "job_id": row["job_id"],
                    "payload": json.loads(row["payload_json"] or "{}"),
                    "created_at": row["created_at"],
                }
                write_event("striatum.event", row["event_id"], payload)
                last_id = int(row["event_id"])
            run_state = conn.execute(
                "SELECT state FROM runs WHERE run_id = ?",
                (run_id,),
            ).fetchone()
            if run_state is not None and run_state["state"] in terminal_states and not rows:
                write_event(
                    "striatum.run_terminal",
                    last_id,
                    {"run_id": run_id, "state": run_state["state"]},
                )
                break
            time.sleep(poll_interval_seconds)
        if state.shutting_down:
            write_event(
                "striatum.shutdown",
                last_id,
                {"run_id": run_id, "reason": "service_shutting_down"},
            )
    except (BrokenPipeError, ConnectionResetError):
        return
    finally:
        if conn is not None:
            conn.close()


def legacy_verify_state_health(
    repo: Path,
    *,
    error_type: type[Exception],
) -> None:
    target = db_path(repo)
    if not target.exists():
        # No existing state — first-time init via dispatch.py is fine.
        return
    try:
        conn = sqlite3.connect(str(target), timeout=2.0)
    except sqlite3.OperationalError as exc:
        raise error_type(
            f"refusing to start serve: cannot open existing {target}: {exc}. "
            f"Inspect (and quarantine if corrupt) before retry."
        ) from exc
    try:
        result = conn.execute("PRAGMA integrity_check").fetchone()
        status = result[0] if result else None
        if status != "ok":
            raise error_type(
                f"refusing to start serve: PRAGMA integrity_check returned {status!r} "
                f"on {target}. Quarantine the file (rename to .corrupt) and run "
                f"`striatum init` to recreate, then retry."
            )
        # Force a WAL checkpoint to a clean snapshot so any in-flight
        # writer state is durable before the new serve takes the lock.
        conn.execute("PRAGMA wal_checkpoint(TRUNCATE)")
    finally:
        conn.close()


def _json_loads_object(raw: Any, default: Any) -> Any:
    if raw is None:
        return default
    try:
        value = json.loads(str(raw))
    except (json.JSONDecodeError, TypeError, ValueError):
        return default
    return value


def _table_columns(conn: sqlite3.Connection, table: str) -> set[str]:
    return {str(row[1]) for row in conn.execute(f"PRAGMA table_info({table})").fetchall()}


def _state_chip(kind: str, state: Any) -> JsonObject:
    normalized = str(state or "unknown")
    return {
        "kind": kind,
        "state": normalized,
        "label": normalized,
        "css_class": f"status-pill status-{normalized}",
    }


def _lane_attestation_chip(
    conn: sqlite3.Connection,
    *,
    session_id: str | None,
    historical_ok: bool = False,
) -> JsonObject:
    if not session_id:
        return {
            "state": "unattested",
            "attested": False,
            "reason": "session_missing",
            "supervisor_id": None,
            "label": "unattested",
        }
    from striatum.identity import session_lane_attestation

    attestation = session_lane_attestation(conn, session_id=session_id)
    if historical_ok and not attestation.attested:
        previous = conn.execute(
            """
            SELECT supervisor_id, state
            FROM process_supervisors
            WHERE session_id = ? AND state IN ('lost', 'stopped')
            ORDER BY ended_at DESC, started_at DESC
            LIMIT 1
            """,
            (session_id,),
        ).fetchone()
        if previous is not None:
            return {
                "state": "previously_attested",
                "attested": False,
                "reason": f"session_{previous['state']}",
                "supervisor_id": previous["supervisor_id"],
                "label": "previously attested",
            }
    return {
        "state": attestation.state,
        "attested": attestation.attested,
        "reason": attestation.reason,
        "supervisor_id": attestation.supervisor_id,
        "label": attestation.state,
    }


def _recorded_artifact_attestation_chip(
    author_line: Any,
    *,
    expected_author_line: Any = None,
    attestation_override_rationale: Any = None,
) -> JsonObject:
    actual = str(author_line).strip().lower() if author_line else ""
    expected = (
        str(expected_author_line).strip().lower()
        if expected_author_line is not None
        else ""
    )
    if attestation_override_rationale:
        return {
            "state": "unattested",
            "attested": False,
            "reason": "operator_override",
            "supervisor_id": None,
            "label": "unattested",
        }
    if actual.startswith("author: operator"):
        return {
            "state": "unattested",
            "attested": False,
            "reason": "operator_byline",
            "supervisor_id": None,
            "label": "unattested",
        }
    if expected and actual == expected:
        return {
            "state": "attested",
            "attested": True,
            "reason": None,
            "supervisor_id": None,
            "label": "attested",
        }
    return {
        "state": "unattested",
        "attested": False,
        "reason": "expected_author_line_mismatch" if expected else "expected_author_line_missing",
        "supervisor_id": None,
        "label": "unattested",
    }


def _lane_evidence_chip(*, attestation_override_rationale: Any = None) -> JsonObject:
    rationale = (
        str(attestation_override_rationale).strip()
        if attestation_override_rationale is not None
        else ""
    )
    if rationale:
        return {
            "state": "override",
            "label": "override",
            "muted": False,
            "rationale": rationale,
        }
    return {
        "state": "not_yet_correlated",
        "label": "not yet correlated",
        "muted": True,
        "rationale": None,
    }


def _byline_line(
    author_line: Any,
    *,
    expected_author_line: Any = None,
    attested: bool | None = None,
    operator_label: Any = None,
) -> JsonObject:
    actual = str(author_line) if author_line is not None else None
    expected = str(expected_author_line) if expected_author_line is not None else None
    if attested is False:
        label = str(operator_label).strip() if operator_label else ""
        display = f"author: operator [self-declared: {label}]" if label else "author: operator"
    else:
        display = actual if actual else "author: <missing>"
    return {
        "author_line": actual,
        "expected_author_line": expected,
        "display": display,
        "attested": attested,
        "matches_expected": (
            None if actual is None or expected is None else actual == expected
        ),
    }


def _verdict_chip(verdict: Any, *, provenance: str, rationale: Any = None) -> JsonObject:
    normalized = str(verdict or "unknown")
    normalized_provenance = provenance.replace("_", "-")
    return {
        "verdict": normalized,
        "label": normalized,
        "provenance": normalized_provenance,
        "source": normalized_provenance,
        "override_rationale": str(rationale) if normalized_provenance == "operator-override" and rationale else None,
        "css_class": f"status-pill status-{normalized}",
    }


def _latest_work_packet_for_job(
    conn: sqlite3.Connection,
    *,
    job_id: str,
) -> JsonObject | None:
    row = conn.execute(
        """
        SELECT packet_json, session_id, lease_id, message_id
        FROM work_packets
        WHERE job_id = ?
        ORDER BY created_at DESC, packet_id DESC
        LIMIT 1
        """,
        (job_id,),
    ).fetchone()
    if row is None:
        return None
    packet = _json_loads_object(row["packet_json"], {})
    if not isinstance(packet, dict):
        packet = {}
    packet.setdefault("session_id", row["session_id"])
    packet.setdefault("lease_id", row["lease_id"])
    packet.setdefault("message_id", row["message_id"])
    return packet


def _expected_artifact_rows(
    *,
    job: dict[str, Any],
    artifacts: list[dict[str, Any]],
    packet: JsonObject | None,
) -> list[JsonObject]:
    declared = _json_loads_object(job.get("expected_artifacts_json"), [])
    if not isinstance(declared, list):
        declared = []
    packet_expected = packet.get("expected_artifacts") if isinstance(packet, dict) else None
    if not isinstance(packet_expected, list):
        packet_expected = []
    packet_by_path = {
        str(item.get("path")): item
        for item in packet_expected
        if isinstance(item, dict) and item.get("path") is not None
    }
    artifacts_by_path = {str(item.get("repo_path")): item for item in artifacts}
    rows: list[JsonObject] = []
    for item in declared:
        if not isinstance(item, dict):
            continue
        path = str(item.get("path") or "")
        packet_item = packet_by_path.get(path, {})
        expected_author_line = (
            packet_item.get("author_line") if isinstance(packet_item, dict) else None
        )
        actual = artifacts_by_path.get(path)
        actual_author_line = actual.get("author_line") if actual else None
        required = bool(item.get("required", True))
        if actual is not None:
            status = (
                "byline_drift"
                if expected_author_line and actual_author_line != expected_author_line
                else "published"
            )
        else:
            status = "missing_required" if required else "missing_optional"
        recipe = None
        if actual is None and path:
            recipe_parts = [
                "striatum",
                "publish-artifact",
                "--job-id",
                str(job.get("job_id")),
                "--path",
                path,
            ]
            if packet and packet.get("session_id"):
                recipe_parts.extend(["--session-id", str(packet["session_id"])])
            if packet and packet.get("lease_id"):
                recipe_parts.extend(["--lease-id", str(packet["lease_id"])])
            if item.get("kind"):
                recipe_parts.extend(["--kind", str(item["kind"])])
            if item.get("logical_name"):
                recipe_parts.extend(["--logical-name", str(item["logical_name"])])
            recipe = " ".join(recipe_parts)
        rows.append(
            {
                "logical_name": item.get("logical_name"),
                "kind": item.get("kind"),
                "path": path,
                "required": required,
                "expected_author_line": expected_author_line,
                "actual_artifact": actual,
                "actual_author_line": actual_author_line,
                "byline_line": _byline_line(
                    actual_author_line,
                    expected_author_line=expected_author_line,
                ),
                "status": status,
                "publish_recipe": recipe,
                "lane_evidence_chip": _lane_evidence_chip(
                    attestation_override_rationale=(
                        actual.get("attestation_override_rationale") if actual else None
                    ),
                ),
            }
        )
    return rows


def _open_blocker_rows(conn: sqlite3.Connection, *, run_id: str, job_id: str | None = None) -> list[JsonObject]:
    query = """
        SELECT b.*, j.workflow_job_id, j.job_type, j.state AS job_state
        FROM blockers b
        LEFT JOIN jobs j ON j.job_id = b.job_id
        WHERE b.run_id = ? AND b.state = 'open'
    """
    params: list[Any] = [run_id]
    if job_id is not None:
        query += " AND b.job_id = ?"
        params.append(job_id)
    query += " ORDER BY CASE b.severity WHEN 'human_checkpoint' THEN 0 WHEN 'blocked' THEN 1 ELSE 2 END, b.created_at"
    rows: list[JsonObject] = []
    for row in conn.execute(query, params).fetchall():
        item = dict(row)
        payload = _json_loads_object(item.get("payload_json"), {})
        item["payload"] = payload if isinstance(payload, dict) else {}
        item["recipes"] = _recipes_for_blocker(item)
        rows.append(item)
    return rows


def _recipes_for_blocker(blocker: Mapping[str, Any]) -> list[str]:
    payload = blocker.get("payload")
    payload_obj = payload if isinstance(payload, dict) else {}
    recipes = payload_obj.get("recovery_commands")
    if isinstance(recipes, list):
        return [str(recipe) for recipe in recipes if recipe]
    run_id = str(blocker.get("run_id") or "")
    job_id = str(blocker.get("job_id") or "")
    blocker_id = str(blocker.get("blocker_id") or "")
    kind = str(blocker.get("blocker_kind") or "")
    severity = str(blocker.get("severity") or "")
    if severity == "human_checkpoint":
        return [
            f"striatum checkpoint resolve --blocker-id {blocker_id} --action continue",
            f"striatum checkpoint resolve --blocker-id {blocker_id} --action cancel",
        ]
    if kind.startswith("process_") and run_id:
        return [f"striatum recovery process-reconcile --run-id {run_id}"]
    if job_id:
        return [
            f'striatum recovery cancel-job --run-id {run_id} --job-id {job_id} --reason "operator inspected blocker {blocker_id}"'
        ]
    return []


def _recovery_panel_payload(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    next_actions: list[Any],
    auto_finalize_dry_run: Mapping[str, Any] | None = None,
) -> JsonObject:
    blockers = _open_blocker_rows(conn, run_id=run_id)
    auto_publish_recipe = (
        f"striatum recovery auto-publish --run-id {run_id} --dry-run"
        if "recovery_auto_publish" in {str(action) for action in next_actions}
        else None
    )
    recipes: list[JsonObject] = []
    if auto_publish_recipe:
        recipes.append({"label": "Auto-publish dry run", "command": auto_publish_recipe})
    for blocker in blockers:
        for recipe in blocker.get("recipes") or []:
            recipes.append({"label": str(blocker.get("blocker_kind") or "Recovery command"), "command": str(recipe)})
    return {
        "run_id": run_id,
        "blockers": blockers,
        "human_checkpoints": [b for b in blockers if b.get("severity") == "human_checkpoint"],
        "blocked": [b for b in blockers if b.get("severity") != "human_checkpoint"],
        "next_actions": [str(action) for action in next_actions],
        "auto_publish_recipe": auto_publish_recipe,
        "auto_finalize_dry_run": dict(auto_finalize_dry_run or {}),
        "recipes": recipes,
    }


def _process_evidence_rows(conn: sqlite3.Connection, *, run_id: str, job_id: str) -> list[JsonObject]:
    process_rows = [
        dict(row)
        for row in conn.execute(
            """
            SELECT *
            FROM process_executions
            WHERE run_id = ? AND job_id = ?
            ORDER BY started_at DESC, process_id DESC
            """,
            (run_id, job_id),
        ).fetchall()
    ]
    blockers_by_process: dict[str, list[JsonObject]] = {}
    for blocker in _open_blocker_rows(conn, run_id=run_id, job_id=job_id):
        payload = blocker.get("payload")
        if isinstance(payload, dict) and payload.get("process_id"):
            blockers_by_process.setdefault(str(payload["process_id"]), []).append(blocker)
    for process in process_rows:
        process["command"] = _json_loads_object(process.get("command_json"), [])
        process["blockers"] = blockers_by_process.get(str(process.get("process_id")), [])
        process["diagnostics"] = [
            blocker.get("payload") for blocker in process["blockers"] if isinstance(blocker.get("payload"), dict)
        ]
    return process_rows


def _artifact_provenance_trail(
    conn: sqlite3.Connection,
    *,
    artifact_id: str,
    run_id: str,
) -> list[JsonObject]:
    event_rows = conn.execute(
        """
        SELECT event_id, event_type, actor_session_id, job_id, artifact_id,
               lease_id, payload_json, created_at
        FROM events
        WHERE run_id = ?
          AND event_type IN ('recovery.auto_published', 'provenance.publish_without_process_execution')
          AND (artifact_id = ? OR payload_json LIKE ?)
        ORDER BY event_id
        """,
        (run_id, artifact_id, f"%{artifact_id}%"),
    ).fetchall()
    trail: list[JsonObject] = []
    for row in event_rows:
        item = dict(row)
        payload = _json_loads_object(item.get("payload_json"), {})
        item["payload"] = payload if isinstance(payload, dict) else {}
        trail.append(item)
    return trail


def _view_file_run_breadcrumb(conn: sqlite3.Connection, *, rel_path: str) -> JsonObject | None:
    parts = Path(rel_path).parts
    if len(parts) < 4 or parts[0] != "docs" or parts[1] != "dogfood":
        return None
    dogfood_id = parts[2]
    if not dogfood_id.isdigit():
        return None
    branch_fragment = f"striatum/dogfood-{dogfood_id}-"
    rows = conn.execute(
        """
        SELECT run_id, branch_name
        FROM runs
        WHERE branch_name LIKE ?
        ORDER BY created_at DESC, run_id DESC
        """,
        (branch_fragment + "%",),
    ).fetchall()
    if len(rows) != 1:
        return None
    row = rows[0]
    return {"run_id": row["run_id"], "branch_name": row["branch_name"]}


def _shape_verdict_rows(
    conn: sqlite3.Connection,
    *,
    verdicts: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    verdict_columns = _table_columns(conn, "verdicts")
    verdict_ids = [
        str(row["verdict_id"])
        for row in verdicts
        if row.get("verdict_id") is not None
    ]
    override_verdict_ids: set[str] = set()
    if verdict_ids:
        verdict_id_set = set(verdict_ids)
        event_rows = conn.execute(
            """
            SELECT payload_json
            FROM events
            WHERE event_type = 'verdict.overridden'
            """
        ).fetchall()
        for event_row in event_rows:
            payload = _json_loads_object(event_row["payload_json"], {})
            verdict_id = payload.get("verdict_id")
            if isinstance(verdict_id, str) and verdict_id in verdict_id_set:
                override_verdict_ids.add(verdict_id)
    shaped: list[dict[str, Any]] = []
    for row in sorted(verdicts, key=lambda item: str(item.get("created_at") or "")):
        source = str(row.get("source") or "") if ("source" in verdict_columns or row.get("source")) else ""
        if not source and row.get("verdict_id") in override_verdict_ids:
            source = "operator_override"
        provenance = (source or "natural").replace("_", "-")
        row["source"] = provenance
        row["provenance"] = provenance
        row["override_rationale"] = (
            row.get("rationale") if provenance == "operator-override" else None
        )
        row["verdict_chip"] = _verdict_chip(
            row.get("verdict"),
            provenance=provenance,
            rationale=row.get("rationale"),
        )
        row["lane_attestation_chip"] = _lane_attestation_chip(
            conn,
            session_id=str(row["session_id"]) if row.get("session_id") else None,
            historical_ok=True,
        )
        shaped.append(row)
    return list(reversed(shaped))


def legacy_shape_artifact_rows(
    conn: sqlite3.Connection | None,
    *,
    artifacts: list[dict[str, Any]],
    expected_rows: list[JsonObject],
) -> list[dict[str, Any]]:
    del conn
    expected_by_path = {str(row.get("path")): row for row in expected_rows}
    for artifact in artifacts:
        expected = expected_by_path.get(str(artifact.get("repo_path")))
        expected_author_line = expected.get("expected_author_line") if expected else None
        override_rationale = artifact.get("attestation_override_rationale")
        attestation = _recorded_artifact_attestation_chip(
            artifact.get("author_line"),
            expected_author_line=expected_author_line,
            attestation_override_rationale=override_rationale,
        )
        artifact["expected_author_line"] = expected_author_line
        artifact["byline_line"] = _byline_line(
            artifact.get("author_line"),
            expected_author_line=expected_author_line,
            attested=bool(attestation.get("attested")),
        )
        artifact["lane_attestation_chip"] = attestation
        artifact["lane_evidence_chip"] = _lane_evidence_chip(
            attestation_override_rationale=override_rationale,
        )
        artifact["attestation_override_rationale"] = override_rationale
    return artifacts


def send_legacy_fixture_error(handler: BaseHTTPRequestHandler, exc: Exception) -> bool:
    from striatum.errors import InvalidTransitionError, NotFoundError, StriatumError

    if isinstance(exc, NotFoundError):
        _handler_send_json(handler, 404, {"ok": False, "error": {"code": 404, "message": str(exc)}})
        return True
    if isinstance(exc, InvalidTransitionError):
        _handler_send_json(
            handler,
            409,
            {"ok": False, "error": {"code": 409, "message": str(exc), "kind": "invalid_transition"}},
        )
        return True
    if isinstance(exc, StriatumError):
        _handler_send_json(handler, 400, {"ok": False, "error": {"code": 400, "message": str(exc)}})
        return True
    return False


def send_legacy_run_now_error(
    handler: BaseHTTPRequestHandler,
    repo: Path,
    exc: Exception,
) -> bool:
    from striatum.errors import (
        BranchConfirmationError,
        InvalidTransitionError,
        WorkflowError,
    )

    if isinstance(exc, BranchConfirmationError):
        _handler_send_json(
            handler,
            409,
            {"ok": False, "error": {"code": 409, "message": str(exc), "kind": "branch_confirmation"}},
        )
        return True
    if isinstance(exc, InvalidTransitionError):
        _handler_send_json(
            handler,
            409,
            {"ok": False, "error": {"code": 409, "message": str(exc), "kind": "invalid_transition"}},
        )
        return True
    if isinstance(exc, WorkflowError):
        msg = str(exc)
        if "git checkout failed" in msg:
            _handler_send_json(
                handler,
                409,
                {
                    "ok": False,
                    "error": {
                        "code": 409,
                        "message": msg,
                        "kind": "dirty_tree",
                        "git_status": short_git_status(repo),
                    },
                },
            )
            return True
        errors = []
        if exc.field_path:
            errors.append({"field_path": exc.field_path, "message": msg})
        _handler_send_json(
            handler,
            422,
            {"ok": False, "error": {"code": 422, "message": msg, "errors": errors}},
        )
        return True
    return send_legacy_fixture_error(handler, exc)


def short_git_status(repo: Path) -> str:
    try:
        proc = subprocess.run(
            ["git", "status", "--short"],
            cwd=repo,
            capture_output=True,
            text=True,
            timeout=5,
            check=False,
        )
    except (OSError, subprocess.SubprocessError):
        return ""
    lines = (proc.stdout or "").splitlines()
    if len(lines) > 80:
        lines = lines[:80] + [f"... ({len(lines) - 80} more lines)"]
    return "\n".join(lines)


def _handler_send_json(handler: BaseHTTPRequestHandler, status: int, body: Mapping[str, Any]) -> None:
    send_json = getattr(handler, "_send_json")
    send_json(status, dict(body))


__all__ = [
    "legacy_artifact_metadata",
    "legacy_artifact_raw_fallback_enabled",
    "legacy_artifact_view_payload",
    "legacy_doctor_page_payload",
    "legacy_fixture_fallback_enabled",
    "legacy_job_cancel",
    "legacy_job_detail_payload",
    "legacy_job_retry",
    "legacy_run_cancel",
    "legacy_run_detail_payload",
    "legacy_run_pause",
    "legacy_run_list_items_for_test_harness",
    "legacy_run_posture_verdicts_payload",
    "legacy_run_resume",
    "legacy_shape_artifact_rows",
    "legacy_stream_events_body",
    "legacy_verify_state_health",
    "legacy_view_file_run_breadcrumb",
    "legacy_web_read_fallback_enabled",
    "legacy_workflow_run_now",
    "send_legacy_fixture_error",
    "send_legacy_run_now_error",
    "short_git_status",
]
