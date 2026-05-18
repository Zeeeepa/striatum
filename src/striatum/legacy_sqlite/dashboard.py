"""Legacy SQLite dashboard payload reader for paired test-harness fixtures."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from striatum.db import connect, ensure_initialized, json_loads, utc_now
from striatum.errors import NotFoundError


def gather_payload(repo: Path, *, run_id: str) -> dict[str, Any]:
    """Collect status + recent events from repo-local SQLite test fixtures."""
    from striatum.cli import recent_events_for_run, status as status_command
    from striatum.workflow import compute_node_states

    ensure_initialized(repo)
    with connect(repo) as conn:
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


def _json_object(value: Any) -> dict[str, Any]:
    if isinstance(value, dict):
        return value
    if isinstance(value, str):
        try:
            loaded = json_loads(value)
        except (TypeError, ValueError):
            return {}
        return loaded if isinstance(loaded, dict) else {}
    return {}
