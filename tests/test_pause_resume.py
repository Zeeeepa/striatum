"""RFC 0024 V4: pause/resume + claim-next gate tests."""

from __future__ import annotations

import sqlite3
from pathlib import Path

from test_cli_mvp import (
    WORKFLOW,
    _git_init_repo,
    data,
    init_repo,
    run_cli,
)


def _state_dir(repo: Path) -> Path:
    return repo / ".striatum" / "state.sqlite3"


def _paused_at(repo: Path, run_id: str) -> str | None:
    with sqlite3.connect(_state_dir(repo)) as conn:
        row = conn.execute(
            "SELECT paused_at FROM runs WHERE run_id = ?", (run_id,),
        ).fetchone()
    return row[0] if row and row[0] else None


def test_pause_run_sets_paused_at(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id,
        "--branch", "striatum/pause-test", "--create",
    ))
    data(run_cli(tmp_path, "run", "start", "--run-id", run_id))
    paused = data(run_cli(tmp_path, "run", "pause", "--run-id", run_id))
    assert paused["status"] == "paused"
    assert _paused_at(tmp_path, run_id) is not None


def test_pause_idempotent(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id,
        "--branch", "striatum/pause-idem", "--create",
    ))
    data(run_cli(tmp_path, "run", "start", "--run-id", run_id))
    first = data(run_cli(tmp_path, "run", "pause", "--run-id", run_id))
    second = data(run_cli(tmp_path, "run", "pause", "--run-id", run_id))
    assert first["status"] == "paused"
    assert second["status"] == "already_paused"


def test_pause_with_reason(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id,
        "--branch", "striatum/pause-reason", "--create",
    ))
    data(run_cli(tmp_path, "run", "start", "--run-id", run_id))
    data(run_cli(tmp_path, "run", "pause", "--run-id", run_id, "--reason", "ad-hoc maintenance"))
    with sqlite3.connect(_state_dir(tmp_path)) as conn:
        reason = conn.execute(
            "SELECT paused_reason FROM runs WHERE run_id = ?", (run_id,),
        ).fetchone()[0]
    assert reason == "ad-hoc maintenance"


def test_resume_clears_paused_at(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id,
        "--branch", "striatum/resume-test", "--create",
    ))
    data(run_cli(tmp_path, "run", "start", "--run-id", run_id))
    data(run_cli(tmp_path, "run", "pause", "--run-id", run_id))
    assert _paused_at(tmp_path, run_id) is not None
    resumed = data(run_cli(tmp_path, "run", "resume", "--run-id", run_id))
    assert resumed["status"] == "resumed"
    assert _paused_at(tmp_path, run_id) is None


def test_resume_idempotent(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id,
        "--branch", "striatum/resume-idem", "--create",
    ))
    data(run_cli(tmp_path, "run", "start", "--run-id", run_id))
    not_paused = data(run_cli(tmp_path, "run", "resume", "--run-id", run_id))
    assert not_paused["status"] == "not_paused"


def test_pause_refuses_completed(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id,
        "--branch", "striatum/pause-completed", "--create",
    ))
    with sqlite3.connect(_state_dir(tmp_path)) as conn:
        conn.execute("UPDATE runs SET state='completed' WHERE run_id=?", (run_id,))
        conn.commit()
    result = run_cli(tmp_path, "run", "pause", "--run-id", run_id, check=False)
    assert result["returncode"] == 4


def test_resume_refuses_terminal(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id,
        "--branch", "striatum/resume-terminal", "--create",
    ))
    with sqlite3.connect(_state_dir(tmp_path)) as conn:
        conn.execute("UPDATE runs SET state='canceled' WHERE run_id=?", (run_id,))
        conn.commit()
    result = run_cli(tmp_path, "run", "resume", "--run-id", run_id, check=False)
    assert result["returncode"] == 4
