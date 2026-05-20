"""RFC 0024 V3: HTTP cancel-run route tests."""

from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import json
from pathlib import Path

from striatum.legacy_sqlite.db import connect
from test_web_ui import (
    _git_init_repo,
    _http_get_raw,
    _spawn_service,
    _stop_service,
    _striatum_init,
)
from test_web_workflow_run import _VALID_WORKFLOW, _post


def _prepare_run(repo: Path) -> str:
    """Prepare a workflow into a fresh run via direct DB-backed CLI."""
    target = repo / "examples" / "wf" / "workflow.json"
    target.parent.mkdir(parents=True)
    target.write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")
    proc, port = _spawn_service(repo, "--web", "--allow-mutations")
    try:
        status, body = _post(port, "/workflows/run/examples/wf/workflow.json", {})
        assert status == 200, body
        run_id = json.loads(body)["data"]["run_id"]
        return str(run_id)
    finally:
        _stop_service(proc)


def test_cancel_run_route_200(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _post(port, f"/run/{run_id}/cancel", {})
        assert status == 200, body
        payload = json.loads(body)
        assert payload["data"]["state"] == "canceled"
    finally:
        _stop_service(proc)


def test_cancel_run_idempotent(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        first_status, _ = _post(port, f"/run/{run_id}/cancel", {})
        second_status, body = _post(port, f"/run/{run_id}/cancel", {})
        assert first_status == 200
        assert second_status == 200
        payload = json.loads(body)
        assert payload["data"].get("status") == "already_canceled"
    finally:
        _stop_service(proc)


def test_cancel_without_mutations_returns_405(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _ = _post(port, f"/run/{run_id}/cancel", {})
        assert status == 405
    finally:
        _stop_service(proc)


def test_cancel_missing_run_returns_404(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _ = _post(port, "/run/run_does_not_exist/cancel", {})
        assert status == 404
    finally:
        _stop_service(proc)


def test_cancel_completed_returns_409(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    # Force-mark completed.
    with connect(tmp_path) as conn:
        conn.execute("UPDATE runs SET state = 'completed' WHERE run_id = ?", (run_id,))
        conn.commit()
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _ = _post(port, f"/run/{run_id}/cancel", {})
        assert status == 409
    finally:
        _stop_service(proc)


def test_run_page_renders_cancel_button(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        _, _, body = _http_get_raw(port, f"/run/{run_id}")
        assert b'id="run-cancel-btn"' in body
        assert b"run_cancel.js" in body
    finally:
        _stop_service(proc)
