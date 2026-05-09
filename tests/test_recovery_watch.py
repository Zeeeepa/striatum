"""RFC 0020 step 3 tests: ``recovery watch`` daemon.

Subprocess-based — the watcher's signal handling and pidfile semantics
need a real process. The patterns mirror tests/test_service.py and
tests/test_web_ui.py.
"""

from __future__ import annotations

import json
import os
import signal
import subprocess
import sys
import time
from pathlib import Path

from test_recovery_extended import _drive_to_human_checkpoint
from test_cli_mvp import data, run_cli

ROOT = Path(__file__).resolve().parents[1]


def _spawn_watch(
    repo: Path, run_id: str, *args: str
) -> subprocess.Popen[str]:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    return subprocess.Popen(
        [
            sys.executable, "-m", "striatum.cli",
            "--repo", str(repo),
            "recovery", "watch",
            "--run-id", run_id,
            *args,
        ],
        cwd=repo, env=env,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        text=True,
    )


def _read_jsonl_lines(text: str) -> list[dict[str, object]]:
    return [json.loads(line) for line in text.splitlines() if line.strip()]


def _pidfile_for(repo: Path, run_id: str) -> Path:
    return repo.resolve() / ".striatum" / "scratch" / f"recovery-watch-{run_id}.pid"


# ----- 1. Max-sweeps + JSONL ----------------------------------------


def test_watch_runs_max_sweeps_then_exits(tmp_path: Path) -> None:
    run_id, _, _, _, _ = _drive_to_human_checkpoint(tmp_path)
    proc = _spawn_watch(
        tmp_path, run_id,
        "--max-sweeps", "3",
        "--interval-seconds", "0",
        "--json",
    )
    stdout, stderr = proc.communicate(timeout=15)
    assert proc.returncode == 0, f"stderr={stderr}"
    lines = _read_jsonl_lines(stdout)
    # 3 sweep envelopes + 1 watch_exit line.
    assert len(lines) == 4, lines
    sweeps = [line for line in lines if "swept_at" in line]
    exits = [line for line in lines if line.get("event") == "watch_exit"]
    assert len(sweeps) == 3
    assert len(exits) == 1
    assert exits[0]["reason"] == "max_sweeps_reached"
    assert exits[0]["swept_total"] == 3


def test_watch_emits_jsonl_envelopes_when_json_set(tmp_path: Path) -> None:
    run_id, _, _, _, _ = _drive_to_human_checkpoint(tmp_path)
    proc = _spawn_watch(
        tmp_path, run_id,
        "--max-sweeps", "1",
        "--interval-seconds", "0",
        "--json",
    )
    stdout, stderr = proc.communicate(timeout=15)
    assert proc.returncode == 0, f"stderr={stderr}"
    # Every line parses as JSON; tolerates no extra noise.
    for raw in stdout.splitlines():
        if raw.strip():
            json.loads(raw)


# ----- 2. Pidfile semantics ------------------------------------------


def test_watch_pidfile_collision_refused(tmp_path: Path) -> None:
    run_id, _, _, _, _ = _drive_to_human_checkpoint(tmp_path)
    pidfile = _pidfile_for(tmp_path, run_id)
    pidfile.parent.mkdir(parents=True, exist_ok=True)
    # Write the current process pid (alive by construction).
    pidfile.write_text(f"{os.getpid()}\n", encoding="ascii")
    try:
        proc = _spawn_watch(
            tmp_path, run_id,
            "--max-sweeps", "1",
            "--interval-seconds", "0",
            "--json",
        )
        stdout, stderr = proc.communicate(timeout=10)
        # InvalidTransitionError → exit code 4.
        assert proc.returncode == 4, f"stdout={stdout} stderr={stderr}"
        combined = stdout + stderr
        assert "another recovery watch is active" in combined
    finally:
        pidfile.unlink(missing_ok=True)


def test_watch_stale_pidfile_overwritten(tmp_path: Path) -> None:
    run_id, _, _, _, _ = _drive_to_human_checkpoint(tmp_path)
    pidfile = _pidfile_for(tmp_path, run_id)
    pidfile.parent.mkdir(parents=True, exist_ok=True)
    # Pick a pid extremely unlikely to be alive on the test box.
    pidfile.write_text("99999999\n", encoding="ascii")
    proc = _spawn_watch(
        tmp_path, run_id,
        "--max-sweeps", "1",
        "--interval-seconds", "0",
        "--json",
    )
    stdout, stderr = proc.communicate(timeout=15)
    assert proc.returncode == 0, f"stderr={stderr}"
    # Pidfile cleaned up on exit.
    assert not pidfile.exists()


def test_watch_pidfile_path_under_scratch(tmp_path: Path) -> None:
    from striatum.recovery import pidfile_path

    expected = tmp_path.resolve() / ".striatum" / "scratch" / "recovery-watch-run_x.pid"
    assert pidfile_path(tmp_path, "run_x") == expected


# ----- 3. Exit-on-terminal ------------------------------------------


def _drive_run_to_terminal(tmp_path: Path) -> str:
    """Drive a tiny code-change run to terminal so the watch can exit."""
    from test_cli_mvp import (
        WORKFLOW,
        init_repo,
    )

    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = str(prepared["run_id"])
    run_cli(
        tmp_path, "branch", "confirm",
        "--run-id", run_id,
        "--branch", "wf/recovery-watch-test",
    )
    run_cli(tmp_path, "run", "start", "--run-id", run_id)
    return run_id


def test_watch_no_exit_on_terminal_continues(tmp_path: Path) -> None:
    """`--no-exit-on-terminal --max-sweeps 2` keeps looping past
    terminal and exits on max_sweeps."""
    run_id, _, _, _, _ = _drive_to_human_checkpoint(tmp_path)
    proc = _spawn_watch(
        tmp_path, run_id,
        "--max-sweeps", "2",
        "--interval-seconds", "0",
        "--no-exit-on-terminal",
        "--json",
    )
    stdout, stderr = proc.communicate(timeout=15)
    assert proc.returncode == 0, f"stderr={stderr}"
    lines = _read_jsonl_lines(stdout)
    exit_line = [line for line in lines if line.get("event") == "watch_exit"][0]
    # The fixture run is still running; max-sweeps wins.
    assert exit_line["reason"] == "max_sweeps_reached"


# ----- 4. Signal-driven shutdown ------------------------------------


def test_watch_sigterm_clean_shutdown(tmp_path: Path) -> None:
    run_id, _, _, _, _ = _drive_to_human_checkpoint(tmp_path)
    pidfile = _pidfile_for(tmp_path, run_id)
    proc = _spawn_watch(
        tmp_path, run_id,
        "--interval-seconds", "30",   # long sleep so the signal interrupts it
        "--json",
    )
    # Wait for the first sweep to land so the pidfile exists.
    deadline = time.time() + 8.0
    while time.time() < deadline:
        if pidfile.exists():
            break
        time.sleep(0.05)
    assert pidfile.exists(), "watcher never wrote pidfile"

    proc.send_signal(signal.SIGTERM)
    stdout, stderr = proc.communicate(timeout=10)
    assert proc.returncode == 0, f"stderr={stderr}"
    lines = _read_jsonl_lines(stdout)
    exit_line = [line for line in lines if line.get("event") == "watch_exit"][0]
    assert exit_line["reason"] == "signal"
    assert not pidfile.exists()


# ----- 5. Module-level smoke (no subprocess needed) -----------------


def test_run_watch_returns_zero_on_max_sweeps(tmp_path: Path) -> None:
    """Direct module call to run_watch with install_signal_handlers=False
    so it can run inside the test process."""
    from io import StringIO

    from striatum.recovery import run_watch

    run_id, _, _, _, _ = _drive_to_human_checkpoint(tmp_path)
    out = StringIO()
    code = run_watch(
        repo=tmp_path,
        run_id=run_id,
        interval_seconds=0.0,
        exit_on_terminal=False,
        max_sweeps=1,
        json_output=True,
        stdout=out,
        install_signal_handlers=False,
    )
    assert code == 0
    text = out.getvalue()
    assert "swept_at" in text
    assert "watch_exit" in text
