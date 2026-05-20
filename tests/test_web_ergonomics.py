# ruff: noqa
"""RFC 0037 web UI ergonomic improvement tests."""

from __future__ import annotations
import pytest; pytest.skip("legacy sqlite / web_ui test helpers are retired", allow_module_level=True)

from pathlib import Path

from test_web_ui import (
    _git_init_repo,
    _http_get_raw,
    _spawn_service,
    _stop_service,
    _striatum_init,
)
from test_web_ui_redesign import _prepare_run


ROOT = Path(__file__).resolve().parents[1]


def test_run_list_filter_controls_data_island_and_duration(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/")
        assert status == 200
        assert b'id="run-list-filter"' in body
        assert b"Search runs by id, branch, workflow" in body
        assert b"Last 7 days" in body
        assert b"<th>Duration</th>" in body
        assert b"run-list-data" in body
        assert b"/static/run_list.js" in body
    finally:
        _stop_service(proc)


def test_run_list_renders_workflow_identity_and_links(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/")
        assert status == 200
        assert b'data-workflow-name="Redesign test"' in body
        assert b"Redesign test" in body
        assert b"wf-redesign-test" in body
        assert b"/workflows/" in body
    finally:
        _stop_service(proc)


def test_doctor_empty_state_and_template_grouping_hooks(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, "/doctor")
        assert status == 503
        assert "application/json" in headers.get("Content-Type", "")
        assert b'"ok": false' in body
        assert b"daemon_unreachable" in body
    finally:
        _stop_service(proc)
    template = (ROOT / "src/striatum/web/templates/doctor.html").read_text(encoding="utf-8")
    assert "doctor-hide-terminal" in template
    assert "<details class=\"doctor-group\"" in template
    assert "data-terminal-run" in template


def test_run_detail_graph_tooltip_hooks_and_next_actions_banner(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, f"/run/{run_id}")
        assert status == 200
        assert b"/static/run_detail.js" in body
        assert b"data-job-id=" in body
        assert b"data-role-id=" in body
        assert b"data-state=" in body
        assert b"data-graph-controls" in body
        assert b"data-graph-pan-zoom" in body
    finally:
        _stop_service(proc)
    template = (ROOT / "src/striatum/web/templates/run_detail.html").read_text(encoding="utf-8")
    assert "next-actions-banner" in template
    assert "run.state not in ('completed', 'failed', 'canceled')" in template
    script = (ROOT / "src/striatum/web/static/run_detail.js").read_text(encoding="utf-8")
    assert "enableGraphViewport" in script
    assert "data-graph-zoom" in script
    assert "ArrowLeft" in script


def test_base_js_localtime_storage_and_shortcuts() -> None:
    script = (ROOT / "src/striatum/web/static/base.js").read_text(encoding="utf-8")
    assert "striatum.ui.time.mode" in script
    assert "window.StriatumUI" in script
    assert "formatDuration" in script
    assert 'routes = { r: "/", w: "/workflows", c: "/chat", d: "/doctor" }' in script
    assert "isEditableTarget" in script


def test_app_css_dark_mode_parity_selectors() -> None:
    css = (ROOT / "src/striatum/web/static/app.css").read_text(encoding="utf-8")
    assert "@media (prefers-color-scheme: dark)" in css
    for selector in (
        ".job-list",
        ".job-link",
        ".status-pill",
        ".posture-chip",
        ".run-grid",
        ".run-jobs-rail",
        ".run-meta",
        ".run-events",
        ".workflow-graph",
        ".workflow-edit-form",
    ):
        assert selector in css
