"""RFC 0024 V4: retry_job mutation tests."""

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


def _job_state(repo: Path, job_id: str) -> str:
    with connect(repo) as conn:
        row = conn.execute(
            "SELECT state FROM jobs WHERE job_id = ?", (job_id,),
        ).fetchone()
    return str(row[0]) if row else ""


def _run_state(repo: Path, run_id: str) -> str:
    with connect(repo) as conn:
        row = conn.execute(
            "SELECT state FROM runs WHERE run_id = ?", (run_id,),
        ).fetchone()
    return str(row[0]) if row else ""


def _first_job_id(repo: Path, run_id: str) -> str:
    with connect(repo) as conn:
        row = conn.execute(
            "SELECT job_id FROM jobs WHERE run_id = ? ORDER BY created_at LIMIT 1",
            (run_id,),
        ).fetchone()
    return str(row[0]) if row else ""


def _prepare_started_run(repo: Path, branch: str) -> tuple[str, str]:
    _git_init_repo(repo, initial_branch="main")
    init_repo(repo)
    prepared = data(run_cli(repo, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = prepared["run_id"]
    data(run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", branch, "--create"))
    data(run_cli(repo, "run", "start", "--run-id", run_id))
    job_id = _first_job_id(repo, run_id)
    return run_id, job_id


def test_retry_failed_job_resets_to_queued(tmp_path: Path) -> None:
    run_id, job_id = _prepare_started_run(tmp_path, "striatum/retry-failed")
    with connect(tmp_path) as conn:
        conn.execute("UPDATE jobs SET state='failed' WHERE job_id=?", (job_id,))
        conn.commit()
    result = data(run_cli(tmp_path, "run", "retry-job", "--run-id", run_id, "--job-id", job_id))
    assert result["new_state"] == "queued"
    assert result["previous_state"] == "failed"
    assert _job_state(tmp_path, job_id) == "queued"


def test_retry_canceled_job_revives_run(tmp_path: Path) -> None:
    run_id, job_id = _prepare_started_run(tmp_path, "striatum/retry-canceled")
    with connect(tmp_path) as conn:
        conn.execute("UPDATE jobs SET state='canceled' WHERE job_id=?", (job_id,))
        conn.execute("UPDATE runs SET state='canceled' WHERE run_id=?", (run_id,))
        conn.commit()
    assert _run_state(tmp_path, run_id) == "canceled"
    result = data(run_cli(tmp_path, "run", "retry-job", "--run-id", run_id, "--job-id", job_id))
    assert result["run_revived"] is True
    assert _run_state(tmp_path, run_id) == "running"


def test_retry_blocked_job_resets(tmp_path: Path) -> None:
    run_id, job_id = _prepare_started_run(tmp_path, "striatum/retry-blocked")
    with connect(tmp_path) as conn:
        conn.execute("UPDATE jobs SET state='blocked' WHERE job_id=?", (job_id,))
        conn.commit()
    result = data(run_cli(tmp_path, "run", "retry-job", "--run-id", run_id, "--job-id", job_id))
    assert result["new_state"] == "queued"


def test_retry_refuses_running(tmp_path: Path) -> None:
    run_id, job_id = _prepare_started_run(tmp_path, "striatum/retry-running")
    with connect(tmp_path) as conn:
        conn.execute("UPDATE jobs SET state='running' WHERE job_id=?", (job_id,))
        conn.commit()
    result = run_cli(
        tmp_path, "run", "retry-job", "--run-id", run_id, "--job-id", job_id, check=False,
    )
    assert result["returncode"] == 4


def test_retry_refuses_completed_run(tmp_path: Path) -> None:
    """F4 from design review: retry on a completed run is refused."""
    run_id, job_id = _prepare_started_run(tmp_path, "striatum/retry-completed")
    with connect(tmp_path) as conn:
        conn.execute("UPDATE jobs SET state='failed' WHERE job_id=?", (job_id,))
        conn.execute("UPDATE runs SET state='completed' WHERE run_id=?", (run_id,))
        conn.commit()
    result = run_cli(
        tmp_path, "run", "retry-job", "--run-id", run_id, "--job-id", job_id, check=False,
    )
    assert result["returncode"] == 4
