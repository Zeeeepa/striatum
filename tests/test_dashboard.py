from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any

from test_cli_mvp import prepare_started_run, run_cli


ROOT = Path(__file__).resolve().parents[1]


def _import_dashboard() -> Any:
    sys.path.insert(0, str(ROOT / "src"))
    try:
        import striatum.dashboard as dashboard  # noqa: WPS433 - test-time import
    finally:
        sys.path.pop(0)
    return dashboard


def test_render_frame_shows_job_state_counts() -> None:
    dashboard = _import_dashboard()
    payload = {
        "run": {"run_id": "run_test", "branch_name": "main", "state": "running"},
        "status": {
            "jobs": {
                "queued": 2,
                "running": 1,
                "completed": 3,
                "blocked": 1,
            },
            "open_blockers": [{"severity": "blocked"}],
            "human_checkpoints": [{"severity": "human_checkpoint"}],
            "latest_non_accepting_review_verdicts": [],
            "claimable_jobs": [
                {"role_id": "author", "lane_id": "codex", "count": 2},
            ],
            "next_actions": ["claim_available_work"],
        },
        "events": [],
        "verdict_counts": {"accept": 1, "needs_revision": 2},
        "updated_at": "2026-05-07T00:00:00Z",
    }
    output = dashboard.render_frame(payload, terminal_width=100)
    assert "run_test" in output
    assert "branch main" in output
    assert "Jobs:" in output
    # Spot-check ordered job state lines for canonical labels and counts.
    assert "queued         2" in output
    assert "running        1" in output
    assert "completed      3" in output
    assert "blocked        1" in output
    assert "Verdicts:" in output
    assert "accept               1" in output
    assert "needs_revision       2" in output
    assert "Blockers (open):" in output
    assert "human_checkpoint 1" in output
    assert "blocked          1" in output
    assert "Claimable now:" in output
    assert "author/codex x 2" in output
    assert "Next actions:" in output
    assert "claim_available_work" in output


def test_render_frame_truncates_long_event_payloads() -> None:
    dashboard = _import_dashboard()
    long_blob = "x" * 500
    payload = {
        "run": {"run_id": "run_test", "branch_name": "main", "state": "running"},
        "status": {
            "jobs": {"completed": 1},
            "open_blockers": [],
            "human_checkpoints": [],
            "latest_non_accepting_review_verdicts": [],
            "claimable_jobs": [],
            "next_actions": [],
        },
        "events": [
            {
                "event_id": 1,
                "event_type": "job.completed",
                "workflow_job_id": "job-with-an-extremely-long-id",
                "payload_json": json.dumps({"summary": long_blob}),
                "created_at": "2026-05-07T12:34:56Z",
            }
        ],
        "verdict_counts": {},
        "updated_at": "2026-05-07T12:34:56Z",
    }
    width = 80
    output = dashboard.render_frame(payload, terminal_width=width)
    for line in output.splitlines():
        assert len(line) <= width, f"line exceeded width {width}: {line!r}"
    # The recent-events section must be present and the long payload visibly truncated.
    assert "Recent events" in output
    assert long_blob not in output


def _binary_env() -> dict[str, str]:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    return env


def test_dashboard_once_renders_one_frame_and_exits(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    result = subprocess.run(
        [
            sys.executable,
            "-m",
            "striatum.cli",
            "--repo",
            str(tmp_path),
            "dashboard",
            "--run-id",
            run_id,
            "--once",
        ],
        cwd=tmp_path,
        env=_binary_env(),
        text=True,
        capture_output=True,
        check=False,
    )
    assert result.returncode == 0, (
        f"dashboard --once failed: stdout={result.stdout!r} stderr={result.stderr!r}"
    )
    assert run_id in result.stdout
    assert "Jobs:" in result.stdout
    # At least one canonical job state row should appear.
    assert any(
        token in result.stdout
        for token in (
            "queued ",
            "claimed ",
            "running ",
            "completed ",
            "blocked ",
            "waiting_human ",
            "failed ",
        )
    )


def test_dashboard_handles_unknown_run_cleanly(tmp_path: Path) -> None:
    # Initialize so the state DB exists; the run id is bogus.
    run_cli(tmp_path, "init")
    result = subprocess.run(
        [
            sys.executable,
            "-m",
            "striatum.cli",
            "--repo",
            str(tmp_path),
            "dashboard",
            "--run-id",
            "run_does_not_exist",
            "--once",
        ],
        cwd=tmp_path,
        env=_binary_env(),
        text=True,
        capture_output=True,
        check=False,
    )
    assert result.returncode != 0
    combined = result.stdout + result.stderr
    assert "run_does_not_exist" in combined or "unknown" in combined.lower()
