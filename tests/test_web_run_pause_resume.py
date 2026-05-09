"""RFC 0024 V4: HTTP pause/resume + per-job action tests."""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path

from test_web_ui import (
    _git_init_repo,
    _http_get_raw,
    _spawn_service,
    _stop_service,
    _striatum_init,
)
from test_web_workflow_run import _VALID_WORKFLOW, _post


def _prepare_run(repo: Path) -> str:
    target = repo / "examples" / "wf" / "workflow.json"
    target.parent.mkdir(parents=True)
    target.write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")
    proc, port = _spawn_service(repo, "--web", "--allow-mutations")
    try:
        status, body = _post(port, "/workflows/run/examples/wf/workflow.json", {})
        assert status == 200, body
        return str(json.loads(body)["data"]["run_id"])
    finally:
        _stop_service(proc)


def _first_job_id(repo: Path, run_id: str) -> str:
    with sqlite3.connect(repo / ".striatum" / "state.sqlite3") as conn:
        row = conn.execute(
            "SELECT job_id FROM jobs WHERE run_id = ? ORDER BY created_at LIMIT 1",
            (run_id,),
        ).fetchone()
    return str(row[0]) if row else ""


def test_pause_route_200(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _post(port, f"/run/{run_id}/pause", {})
        assert status == 200, body
        payload = json.loads(body)
        assert payload["data"]["status"] == "paused"
    finally:
        _stop_service(proc)


def test_pause_then_resume(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        _post(port, f"/run/{run_id}/pause", {})
        status, body = _post(port, f"/run/{run_id}/resume", {})
        assert status == 200, body
        assert json.loads(body)["data"]["status"] == "resumed"
    finally:
        _stop_service(proc)


def test_pause_without_mutations_405(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _ = _post(port, f"/run/{run_id}/pause", {})
        assert status == 405
    finally:
        _stop_service(proc)


def test_pause_missing_run_404(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _ = _post(port, "/run/run_does_not_exist/pause", {})
        assert status == 404
    finally:
        _stop_service(proc)


def test_run_page_shows_pause_button_when_running(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        _, _, body = _http_get_raw(port, f"/run/{run_id}")
        assert b"run-pause-btn" in body
        assert b"run_pause_resume.js" in body
    finally:
        _stop_service(proc)


def test_run_page_shows_resume_button_when_paused(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        _post(port, f"/run/{run_id}/pause", {})
        _, _, body = _http_get_raw(port, f"/run/{run_id}")
        assert b"run-resume-btn" in body
    finally:
        _stop_service(proc)


def test_job_cancel_route(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    job_id = _first_job_id(tmp_path, run_id)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _post(port, f"/run/{run_id}/job/{job_id}/cancel", {})
        assert status == 200, body
        payload = json.loads(body)
        assert payload["data"]["status"] == "canceled"
    finally:
        _stop_service(proc)


def test_job_retry_route_revives_run(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    job_id = _first_job_id(tmp_path, run_id)
    # First cancel the run to set up the revive scenario.
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        _post(port, f"/run/{run_id}/cancel", {})
        status, body = _post(port, f"/run/{run_id}/job/{job_id}/retry", {})
        assert status == 200, body
        payload = json.loads(body)
        assert payload["data"]["run_revived"] is True
    finally:
        _stop_service(proc)


def test_job_action_without_mutations_405(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    job_id = _first_job_id(tmp_path, run_id)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _ = _post(port, f"/run/{run_id}/job/{job_id}/cancel", {})
        assert status == 405
    finally:
        _stop_service(proc)


def test_job_detail_renders_action_buttons(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    job_id = _first_job_id(tmp_path, run_id)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        _, _, body = _http_get_raw(port, f"/run/{run_id}/job/{job_id}")
        assert b'data-job-action="cancel"' in body
        assert b"job_actions.js" in body
    finally:
        _stop_service(proc)
