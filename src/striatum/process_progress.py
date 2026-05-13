"""Daemon-side supervised progress polling."""

from __future__ import annotations

import sqlite3
from dataclasses import dataclass
from pathlib import Path
from typing import Callable

from striatum.cli.mutations import heartbeat
from striatum.daemon_supervisor.progress_watcher import (
    ProgressWatcherConfig,
    ProgressWatcherResult,
    SupervisedProgressTarget,
    SupervisedProgressWatcher,
)
from striatum.db import JsonObject, insert_event, transaction, utc_now
from striatum.identity import process_start_time


@dataclass(frozen=True)
class ProcessProgressConfig:
    watcher: ProgressWatcherConfig = ProgressWatcherConfig()
    startup_grace_seconds: float = 60.0


def progress_loop_once(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    config: ProcessProgressConfig | None = None,
    should_stop: Callable[[], bool] | None = None,
) -> JsonObject:
    """Run one bounded supervised-progress pass for a repository."""
    cfg = config or ProcessProgressConfig()
    stop = should_stop or (lambda: False)
    rows = conn.execute(
        """
        SELECT ps.*
        FROM process_supervisors ps
        JOIN runs r ON r.run_id = ps.run_id
        WHERE ps.state = 'attached'
          AND r.state IN ('running', 'paused')
        ORDER BY ps.started_at, ps.supervisor_id
        """
    ).fetchall()
    results: list[JsonObject] = []
    counts: dict[str, int] = {}
    watcher = SupervisedProgressWatcher(
        conn=conn,
        repo=repo,
        heartbeat_callback=lambda *, session_id, lease_id, extend_seconds: heartbeat(
            conn,
            session_id=session_id,
            lease_id=lease_id,
            extend_seconds=extend_seconds,
        ),
        config=cfg.watcher,
    )

    for row in rows:
        if stop():
            break
        result = _tick_supervisor(conn, repo=repo, row=row, watcher=watcher, config=cfg, should_stop=stop)
        results.append(result)
        status = str(result["status"])
        counts[status] = counts.get(status, 0) + 1

    return {
        "supervisors_checked": len(results),
        "counts": counts,
        "results": results,
    }


def _tick_supervisor(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    row: sqlite3.Row,
    watcher: SupervisedProgressWatcher,
    config: ProcessProgressConfig,
    should_stop: Callable[[], bool],
) -> JsonObject:
    supervisor_id = str(row["supervisor_id"])
    pid = int(row["pid"]) if row["pid"] is not None else 0
    expected_start = row["pid_start_time"] if "pid_start_time" in row.keys() else None
    current_start = process_start_time(pid) if pid > 0 else None
    if expected_start and current_start != str(expected_start):
        _mark_lost(conn, row=row, status="process_identity_changed")
        return {
            "supervisor_id": supervisor_id,
            "status": "process_identity_changed",
            "pid": pid,
        }
    scratch_path = Path(str(row["scratch_path"])) if row["scratch_path"] is not None else repo / ".striatum" / "scratch" / supervisor_id
    if _within_startup_grace(row, config=config) and not scratch_path.exists():
        return {"supervisor_id": supervisor_id, "status": "waiting_for_log"}
    if should_stop():
        return {"supervisor_id": supervisor_id, "status": "stopping"}

    target = SupervisedProgressTarget(
        supervisor_id=supervisor_id,
        session_id=str(row["session_id"]),
        pid=pid,
        scratch_path=scratch_path,
        run_id=str(row["run_id"]),
        state=str(row["state"]),
    )
    try:
        tick = watcher.tick(target)
    except Exception as exc:  # noqa: BLE001 - progress polling must not abort sweep.
        return {"supervisor_id": supervisor_id, "status": "error", "error": str(exc)}
    if tick.status in {"process_gone", "process_identity_changed"}:
        _mark_lost(conn, row=row, status=tick.status)
    elif tick.status in {"heartbeat", "idle", "no_log"}:
        _record_progress_event(conn, row=row, result=tick)
    return _result_payload(tick)


def _within_startup_grace(row: sqlite3.Row, *, config: ProcessProgressConfig) -> bool:
    started = row["started_at"]
    if started is None:
        return False
    try:
        from datetime import datetime

        started_at = datetime.fromisoformat(str(started).replace("Z", "+00:00"))
        now = datetime.fromisoformat(utc_now().replace("Z", "+00:00"))
    except ValueError:
        return False
    return (now - started_at).total_seconds() <= config.startup_grace_seconds


def _record_progress_event(
    conn: sqlite3.Connection, *, row: sqlite3.Row, result: ProgressWatcherResult
) -> None:
    event_type = {
        "heartbeat": "supervisor.progress_watcher_heartbeat",
        "idle": "supervisor.progress_watcher_idle",
        "no_log": "supervisor.progress_watcher_idle",
    }.get(result.status)
    if event_type is None:
        return
    with transaction(conn):
        insert_event(
            conn,
            run_id=str(row["run_id"]),
            event_type=event_type,
            actor_session_id=str(row["session_id"]),
            job_id=result.job_id,
            lease_id=result.lease_id,
            payload={
                "supervisor_id": str(row["supervisor_id"]),
                "status": result.status,
                "age_seconds": result.age_seconds,
                "warning": result.warning,
            },
        )


def _mark_lost(conn: sqlite3.Connection, *, row: sqlite3.Row, status: str) -> None:
    now = utc_now()
    with transaction(conn):
        conn.execute(
            """
            UPDATE process_supervisors
            SET state = 'lost', ended_at = ?, stop_reason = ?
            WHERE supervisor_id = ?
            """,
            (now, status, str(row["supervisor_id"])),
        )
        insert_event(
            conn,
            run_id=str(row["run_id"]),
            event_type="supervisor.progress_watcher_lost",
            actor_session_id=str(row["session_id"]),
            payload={"supervisor_id": str(row["supervisor_id"]), "status": status},
        )


def _result_payload(result: ProgressWatcherResult) -> JsonObject:
    return {
        "supervisor_id": result.supervisor_id,
        "status": result.status,
        "lease_id": result.lease_id,
        "job_id": result.job_id,
        "heartbeat_called": result.heartbeat_called,
        "age_seconds": result.age_seconds,
        "warning": result.warning,
    }


__all__ = ["ProcessProgressConfig", "progress_loop_once"]
