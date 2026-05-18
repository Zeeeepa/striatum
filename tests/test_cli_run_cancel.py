"""RFC 0024 V3: `striatum run cancel` end-to-end tests."""

from __future__ import annotations

from pathlib import Path

from striatum.legacy_sqlite.db import connect
from test_cli_mvp import (
    WORKFLOW,
    _git_init_repo,
    data,
    init_repo,
    run_cli,
)


def _run_state(repo: Path, run_id: str) -> str:
    with connect(repo) as conn:
        row = conn.execute(
            "SELECT state FROM runs WHERE run_id = ?", (run_id,)
        ).fetchone()
    return str(row[0]) if row else ""


def test_cancel_run_from_ready(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id,
        "--branch", "striatum/cancel-test", "--create",
    ))
    canceled = data(run_cli(tmp_path, "run", "cancel", "--run-id", run_id))
    assert canceled["state"] == "canceled"
    assert _run_state(tmp_path, run_id) == "canceled"


def test_cancel_run_from_running(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id,
        "--branch", "striatum/cancel-running", "--create",
    ))
    data(run_cli(tmp_path, "run", "start", "--run-id", run_id))
    assert _run_state(tmp_path, run_id) == "running"
    canceled = data(run_cli(tmp_path, "run", "cancel", "--run-id", run_id))
    assert canceled["state"] == "canceled"
    # All in-flight jobs should be canceled.
    with connect(tmp_path) as conn:
        non_terminal = conn.execute(
            """
            SELECT COUNT(*) FROM jobs
            WHERE run_id = ? AND state NOT IN ('canceled','completed','failed','skipped')
            """,
            (run_id,),
        ).fetchone()[0]
    assert non_terminal == 0


def test_cancel_run_idempotent(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id,
        "--branch", "striatum/cancel-idem", "--create",
    ))
    first = data(run_cli(tmp_path, "run", "cancel", "--run-id", run_id))
    second = data(run_cli(tmp_path, "run", "cancel", "--run-id", run_id))
    assert first["state"] == "canceled"
    assert second["state"] == "canceled"
    assert second.get("status") == "already_canceled"


def test_cancel_run_with_reason(tmp_path: Path) -> None:
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id,
        "--branch", "striatum/cancel-reason", "--create",
    ))
    data(run_cli(
        tmp_path, "run", "cancel", "--run-id", run_id, "--reason", "spec changed",
    ))
    with connect(tmp_path) as conn:
        stop_reason = conn.execute(
            "SELECT stop_reason FROM runs WHERE run_id = ?", (run_id,),
        ).fetchone()[0]
    assert stop_reason == "spec changed"


def test_cancel_run_refuses_completed(tmp_path: Path) -> None:
    """A completed run cannot be canceled — InvalidTransition (exit 4)."""
    _git_init_repo(tmp_path, initial_branch="main")
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(
        tmp_path, "branch", "confirm", "--run-id", run_id,
        "--branch", "striatum/cancel-completed", "--create",
    ))
    # Force-mark the run completed (test shortcut).
    with connect(tmp_path) as conn:
        conn.execute(
            "UPDATE runs SET state = 'completed' WHERE run_id = ?", (run_id,),
        )
        conn.commit()
    result = run_cli(tmp_path, "run", "cancel", "--run-id", run_id, check=False)
    assert result["returncode"] == 4
