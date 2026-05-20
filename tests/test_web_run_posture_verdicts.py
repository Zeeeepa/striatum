# ruff: noqa
"""RFC 0024 V4.1: drill-down for verdicts-by-posture chips."""

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


def _insert_verdict(repo: Path, run_id: str, *, posture: str, verdict: str = "accept") -> None:
    """Helper: insert a verdict row directly so the drill-down has data."""
    with connect(repo) as conn:
        job = conn.execute(
            "SELECT job_id FROM jobs WHERE run_id = ? LIMIT 1", (run_id,),
        ).fetchone()
        # Need a session with the same run_id; create one.
        conn.execute(
            """
            INSERT INTO sessions (
                session_id, run_id, role_id, lane_id, slug, ordinal,
                state, registered_at, capabilities_json, first_class, fresh_context
            )
            VALUES (?, ?, 'reviewer', 'lane_a', ?, 1, 'closed', '2026-05-09T00:00:00Z', '[]', 1, 1)
            """,
            (f"sess_test_{posture}", run_id, f"reviewer-test-{posture}"),
        )
        conn.execute(
            """
            INSERT INTO verdicts (
                verdict_id, run_id, job_id, session_id, verdict,
                rationale, created_at, posture
            )
            VALUES (?, ?, ?, ?, ?, ?, '2026-05-09T00:00:00Z', ?)
            """,
            (
                f"v_test_{posture}",
                run_id,
                str(job["job_id"]),
                f"sess_test_{posture}",
                verdict,
                f"survived posture sweep ({posture})",
                posture,
            ),
        )
        conn.commit()


def test_posture_drill_down_lists_verdicts(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    _insert_verdict(tmp_path, run_id, posture="devils_advocate")
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        _, _, body = _http_get_raw(port, f"/run/{run_id}/posture/devils_advocate")
        assert b"devils_advocate" in body
        assert b"survived posture sweep" in body
    finally:
        _stop_service(proc)


def test_posture_drill_down_empty_state(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        _, _, body = _http_get_raw(port, f"/run/{run_id}/posture/security")
        assert b"No verdicts recorded" in body
    finally:
        _stop_service(proc)


def test_posture_drill_down_missing_run_404(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, _ = _http_get_raw(port, "/run/run_does_not_exist/posture/devils_advocate")
        assert status == 404
    finally:
        _stop_service(proc)


def test_run_page_chip_links_to_posture_view(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    _insert_verdict(tmp_path, run_id, posture="devils_advocate")
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        _, _, body = _http_get_raw(port, f"/run/{run_id}")
        # Chip should be wrapped in a link to the posture view.
        href = f'href="/run/{run_id}/posture/devils_advocate"'.encode()
        assert href in body
    finally:
        _stop_service(proc)
