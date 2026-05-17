from __future__ import annotations

import json
import os
from io import StringIO
from pathlib import Path
from typing import Any, Mapping

from striatum.recovery import PIDFILE_COLLISION_EXIT_CODE, run_daemon_watch


def test_daemon_watch_runs_sweeps_through_daemon_rpc(tmp_path: Path) -> None:
    calls: list[tuple[str, dict[str, Any]]] = []

    def call_method(_repo: Path, method: str, params: Mapping[str, Any]) -> dict[str, Any]:
        calls.append((method, dict(params)))
        assert method == "recovery.sweep"
        return {
            "run_id": params["run_id"],
            "run_state": "running",
            "swept_at": "2026-05-17T00:00:00Z",
            "actions": [],
            "escalations": [],
            "still_stuck": [],
        }

    out = StringIO()

    code = run_daemon_watch(
        tmp_path,
        run_id="run_1",
        interval_seconds=0,
        exit_on_terminal=False,
        max_sweeps=2,
        cli_overrides={
            "autonomous_review_requeue": True,
            "autonomous_process_reconcile": None,
            "max_requeues_per_sweep": 3,
        },
        json_output=True,
        stdout=out,
        install_signal_handlers=False,
        call_method=call_method,
        now=lambda: "2026-05-17T00:00:05Z",
    )

    assert code == 0
    assert calls == [
        (
            "recovery.sweep",
            {
                "run_id": "run_1",
                "autonomous_review_requeue": True,
                "max_requeues_per_sweep": 3,
            },
        ),
        (
            "recovery.sweep",
            {
                "run_id": "run_1",
                "autonomous_review_requeue": True,
                "max_requeues_per_sweep": 3,
            },
        ),
    ]
    lines = [json.loads(line) for line in out.getvalue().splitlines()]
    assert len(lines) == 3
    assert lines[-1] == {
        "event": "watch_exit",
        "run_id": "run_1",
        "reason": "max_sweeps_reached",
        "swept_total": 2,
        "exited_at": "2026-05-17T00:00:05Z",
    }


def test_daemon_watch_exits_when_sweep_reports_terminal_run(
    tmp_path: Path,
) -> None:
    calls: list[tuple[str, dict[str, Any]]] = []

    def call_method(_repo: Path, method: str, params: Mapping[str, Any]) -> dict[str, Any]:
        calls.append((method, dict(params)))
        assert method == "recovery.sweep"
        return {
            "run_id": params["run_id"],
            "run_state": "completed",
            "swept_at": "2026-05-17T00:00:00Z",
            "actions": [],
            "escalations": [],
            "still_stuck": [],
        }

    out = StringIO()

    code = run_daemon_watch(
        tmp_path,
        run_id="run_1",
        interval_seconds=0,
        exit_on_terminal=True,
        json_output=True,
        stdout=out,
        install_signal_handlers=False,
        call_method=call_method,
        now=lambda: "2026-05-17T00:00:05Z",
    )

    assert code == 0
    assert calls == [("recovery.sweep", {"run_id": "run_1"})]
    lines = [json.loads(line) for line in out.getvalue().splitlines()]
    assert lines[-1]["reason"] == "run_terminal"
    assert lines[-1]["swept_total"] == 1


def test_daemon_watch_pidfile_collision_refused(tmp_path: Path) -> None:
    pidfile = tmp_path / ".striatum" / "scratch" / "recovery-watch-run_1.pid"
    pidfile.parent.mkdir(parents=True, exist_ok=True)
    pidfile.write_text(f"{os.getpid()}\n", encoding="ascii")
    out = StringIO()

    code = run_daemon_watch(
        tmp_path,
        run_id="run_1",
        max_sweeps=1,
        json_output=True,
        stdout=out,
        install_signal_handlers=False,
        call_method=lambda _repo, _method, _params: {},
    )

    assert code == PIDFILE_COLLISION_EXIT_CODE
    line = json.loads(out.getvalue().splitlines()[0])
    assert line["event"] == "watch_collision"
    assert "another recovery watch is active" in line["message"]
