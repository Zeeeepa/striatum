# ruff: noqa
"""RFC 0022 V1 tests: Jinja2 SSR multi-page web UI.

Spawn `striatum serve --web` and assert each route returns
server-rendered HTML; verify CSP header preservation; check the
SVG dependency graph renders for a multi-job run; assert the
dark-mode palette is present in base.css; check the legacy hash-
redirect JS island ships.
"""

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


def test_run_list_page_renders_html(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, "/")
        assert status == 200
        assert "text/html" in headers.get("Content-Type", "")
        # Server-rendered: heading present, no JS-rendered placeholder.
        assert b"<h1>Runs</h1>" in body
        # Empty-state copy when no runs exist.
        assert b"No runs yet" in body
    finally:
        _stop_service(proc)


def test_doctor_page_fails_closed_without_daemon(tmp_path: Path) -> None:
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


def test_csp_header_unchanged(tmp_path: Path) -> None:
    """RFC 0022 V1 zero-regression: CSP must be byte-identical to v1.10.0."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        run_id = _prepare_run(tmp_path)
        for path in ("/", "/workflows", f"/run/{run_id}"):
            _, headers, _ = _http_get_raw(port, path)
            csp = headers.get("Content-Security-Policy", "")
            assert "default-src 'self'" in csp
            assert "unsafe-inline" not in csp
            assert "unsafe-eval" not in csp
    finally:
        _stop_service(proc)


def test_dark_mode_palette_present_in_base_css(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        _, _, body = _http_get_raw(port, "/static/base.css")
        assert b"prefers-color-scheme: dark" in body
        assert b"--bg-base" in body
        assert b"--status-running" in body
    finally:
        _stop_service(proc)


def test_legacy_hash_redirect_island_ships(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        _, _, body = _http_get_raw(port, "/static/legacy_hash_redirect.js")
        assert b"window.location.hash" in body
        assert b"location.replace" in body
    finally:
        _stop_service(proc)


def test_unknown_run_id_returns_404(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, _ = _http_get_raw(port, "/run/run_does_not_exist")
        assert status == 404
    finally:
        _stop_service(proc)


# --- Run-detail + SVG graph ------------------------------------------


def _prepare_run(tmp_path: Path) -> str:
    """Create a tiny single-review-job workflow + start a run; return run_id."""
    import json
    import subprocess
    import sys
    workflow = {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "wf-redesign-test",
        "workflow_version": "1",
        "name": "Redesign test",
        "branch": {"mode": "auto", "suggested_name": "wf/redesign-test", "allow_dirty": False},
        "coordinator": {"role_id": "reviewer", "lane_id": "lane_a"},
        "lanes": {
            "lane_a": {
                "adapter": "process",
                "display_model": "X",
                "command": ["echo"],
                "capabilities": ["review"],
            },
        },
        "roles": {"reviewer": {"definition_path": "roles/reviewer.md"}},
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 1,
            "require_disjoint_write_scopes": True,
        },
        "jobs": [
            {
                "id": "review_only",
                "type": "review",
                "title": "Review",
                "role_id": "reviewer",
                "lane_id": "lane_a",
                "fresh_session_required": True,
                "objective": "Review",
                "task_prompt": {"path": "prompts/review.md"},
                "write_scope": {
                    "mode": "review_only_artifact",
                    "repo_write": False,
                    "allowed_paths": ["docs/reviews/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {"logical_name": "review", "kind": "finding",
                     "path": "docs/reviews/REVIEW.md", "required": True},
                ],
            },
        ],
        "edges": [],
        "cycles": [],
    }
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(workflow), encoding="utf-8")
    import os
    env = os.environ.copy()
    env["PYTHONPATH"] = str(Path(__file__).resolve().parents[1] / "src")
    out = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(tmp_path),
         "run", "prepare", "--workflow", str(workflow_path), "--json"],
        cwd=tmp_path, env=env, check=True, capture_output=True, text=True,
    )
    payload = json.loads(out.stdout)
    return str(payload["data"]["run_id"])


def test_run_detail_page_renders_html_and_svg(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, f"/run/{run_id}")
        assert status == 200
        assert "text/html" in headers.get("Content-Type", "")
        assert run_id.encode() in body
        # SVG graph rendered server-side.
        assert b"<svg" in body
        assert b'class="graph-node' in body
    finally:
        _stop_service(proc)


def test_svg_node_link_targets_job_detail(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        _, _, body = _http_get_raw(port, f"/run/{run_id}")
        assert f'href="/run/{run_id}/job/review_only"'.encode() in body
    finally:
        _stop_service(proc)
