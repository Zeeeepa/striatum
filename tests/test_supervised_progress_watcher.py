# ruff: noqa
from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import json
import os
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, cast

from striatum.cli.mutations import heartbeat
from striatum.legacy_sqlite.db import connect
from striatum.daemon_supervisor.progress_watcher import (
    ProgressWatcherConfig,
    SupervisedProgressWatcher,
    SupervisedProgressTarget,
    progress_advisory_lock,
    check_progress_once,
)


ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"


def _target(log_path: Path) -> SupervisedProgressTarget:
    return SupervisedProgressTarget(
        supervisor_id="sup_test",
        session_id="sess_test",
        lease_id="lease_test",
        pid=os.getpid(),
        log_path=log_path,
    )


def _run_cli(repo: Path, *args: str) -> dict[str, Any]:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    result = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), *args, "--json"],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    return cast(dict[str, Any], json.loads(result.stdout)["data"])


def _claimed_packet(repo: Path) -> tuple[str, dict[str, Any]]:
    from striatum.legacy_sqlite.db import init_repo as legacy_init_repo

    legacy_init_repo(repo)
    prepared = _run_cli(repo, "run", "prepare", "--workflow", str(WORKFLOW))
    run_id = str(prepared["run_id"])
    _run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/test")
    _run_cli(repo, "run", "start", "--run-id", run_id)
    session = _run_cli(
        repo,
        "register-session",
        "--run-id",
        run_id,
        "--role",
        "author",
        "--lane",
        "codex",
        "--capability",
        "write",
    )
    claimed = _run_cli(repo, "claim-next", "--session-id", str(session["session_id"]))
    packet = cast(dict[str, Any], claimed["packet"])
    lease = cast(dict[str, Any], packet["lease"])
    job = cast(dict[str, Any], packet["job"])
    packet["lease_id"] = str(lease["lease_id"])
    packet["job_id"] = str(job["job_id"])
    packet["run_id"] = run_id
    return str(session["session_id"]), packet


def test_progress_watcher_heartbeats_when_log_is_fresh(tmp_path: Path) -> None:
    log_path = tmp_path / "packet-0001.log"
    log_path.write_text("progress\n", encoding="utf-8")
    calls: list[tuple[str, str]] = []

    result = check_progress_once(
        _target(log_path),
        heartbeat=lambda session_id, lease_id: calls.append((session_id, lease_id)),
        config=ProgressWatcherConfig(refresh_threshold_seconds=60, idle_threshold_seconds=600),
        now=time.time(),
    )

    assert result.status == "heartbeat"
    assert calls == [("sess_test", "lease_test")]


def test_progress_watcher_reports_idle_without_heartbeat(tmp_path: Path) -> None:
    log_path = tmp_path / "packet-0001.log"
    log_path.write_text("old\n", encoding="utf-8")
    old = time.time() - 700
    os.utime(log_path, (old, old))
    idle: list[str] = []

    result = check_progress_once(
        _target(log_path),
        heartbeat=lambda _session_id, _lease_id: None,
        config=ProgressWatcherConfig(refresh_threshold_seconds=60, idle_threshold_seconds=600),
        now=time.time(),
        on_idle=idle.append,
    )

    assert result.status == "idle"
    assert idle == ["sup_test"]


def test_progress_watcher_reports_lost_process(tmp_path: Path) -> None:
    log_path = tmp_path / "packet-0001.log"
    log_path.write_text("progress\n", encoding="utf-8")
    lost: list[str] = []
    target = SupervisedProgressTarget(
        supervisor_id="sup_test",
        session_id="sess_test",
        lease_id="lease_test",
        pid=999_999_999,
        log_path=log_path,
    )

    result = check_progress_once(
        target,
        heartbeat=lambda _session_id, _lease_id: None,
        on_lost=lost.append,
    )

    assert result.status == "process_lost"
    assert lost == ["sup_test"]


def test_db_aware_progress_watcher_heartbeats_active_lease(tmp_path: Path) -> None:
    session_id, packet = _claimed_packet(tmp_path)
    scratch = tmp_path / ".striatum" / "scratch" / "sup_test" / "codex-logs"
    scratch.mkdir(parents=True)
    log_path = scratch / "packet-0001.log"
    log_path.write_text("progress\n", encoding="utf-8")

    with connect(tmp_path) as conn:
        before = conn.execute(
            "SELECT expires_at FROM leases WHERE lease_id = ?",
            (str(packet["lease_id"]),),
        ).fetchone()
        assert before is not None
        watcher = SupervisedProgressWatcher(
            conn=conn,
            repo=tmp_path,
            heartbeat_callback=lambda *, session_id, lease_id, extend_seconds: heartbeat(
                conn,
                session_id=session_id,
                lease_id=lease_id,
                extend_seconds=extend_seconds,
            ),
            config=ProgressWatcherConfig(refresh_threshold_seconds=60, heartbeat_extend_seconds=1200),
            clock=time.time,
        )

        result = watcher.tick(
            SupervisedProgressTarget(
                supervisor_id="sup_test",
                session_id=session_id,
                lease_id=str(packet["lease_id"]),
                job_id=str(packet["job_id"]),
                pid=os.getpid(),
                scratch_path=scratch.parent,
            )
        )

        after = conn.execute(
            "SELECT expires_at FROM leases WHERE lease_id = ?",
            (str(packet["lease_id"]),),
        ).fetchone()
        event = conn.execute(
            "SELECT event_type FROM events WHERE event_type = 'lease.heartbeat'"
        ).fetchone()

    assert result.status == "heartbeat"
    assert result.heartbeat_called is True
    assert result.job_id == str(packet["job_id"])
    assert after is not None
    assert after["expires_at"] != before["expires_at"]
    assert event is not None


def test_db_aware_progress_watcher_respects_progress_lock(tmp_path: Path) -> None:
    session_id, packet = _claimed_packet(tmp_path)
    scratch = tmp_path / ".striatum" / "scratch" / "sup_test"
    scratch.mkdir(parents=True)
    log_path = scratch / "packet-0001.log"
    log_path.write_text("progress\n", encoding="utf-8")

    with connect(tmp_path) as conn:
        watcher = SupervisedProgressWatcher(
            conn=conn,
            repo=tmp_path,
            heartbeat_callback=lambda *, session_id, lease_id, extend_seconds: heartbeat(
                conn,
                session_id=session_id,
                lease_id=lease_id,
                extend_seconds=extend_seconds,
            ),
            config=ProgressWatcherConfig(refresh_threshold_seconds=60),
            clock=time.time,
        )
        with progress_advisory_lock(tmp_path, job_id=str(packet["job_id"])) as acquired:
            assert acquired is True
            result = watcher.tick(
                SupervisedProgressTarget(
                    supervisor_id="sup_test",
                    session_id=session_id,
                    lease_id=str(packet["lease_id"]),
                    job_id=str(packet["job_id"]),
                    pid=os.getpid(),
                    scratch_path=scratch,
                )
            )

    assert result.status == "lock_busy"
