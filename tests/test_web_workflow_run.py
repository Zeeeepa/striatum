"""RFC 0024 V2: POST /workflows/run/<path> tests."""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from pathlib import Path


from test_web_ui import (
    _git_init_repo,
    _spawn_service,
    _stop_service,
    _striatum_init,
)


_VALID_WORKFLOW = {
    "schema_version": "striatum.workflow.v1",
    "workflow_id": "wf-run-now-test",
    "workflow_version": "1",
    "name": "Run-now test",
    "branch": {"mode": "auto", "suggested_name": "wf/run-now", "allow_dirty": False},
    "coordinator": {"role_id": "reviewer", "lane_id": "lane_a"},
    "lanes": {
        "lane_a": {
            "adapter": "process", "display_model": "X",
            "command": ["echo"], "capabilities": ["review"],
        },
    },
    "roles": {"reviewer": {"definition_path": "roles/reviewer.md"}},
    "context_docs": [],
    "parallelism": {
        "mode": "declared", "max_active_jobs": 1,
        "require_disjoint_write_scopes": True,
    },
    "jobs": [
        {
            "id": "review_only", "type": "review", "title": "Review",
            "role_id": "reviewer", "lane_id": "lane_a",
            "fresh_session_required": True, "objective": "Review",
            "task_prompt": {"path": "p.md"},
            "write_scope": {
                "mode": "review_only_artifact", "repo_write": False,
                "allowed_paths": ["docs/reviews/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [
                {"logical_name": "r", "kind": "finding",
                 "path": "docs/reviews/REVIEW.md", "required": True},
            ],
        },
    ],
    "edges": [],
    "cycles": [],
}


def _post(port: int, path: str, body: object,
          *, content_type: str = "application/json",
          ) -> tuple[int, bytes]:
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(
        f"http://127.0.0.1:{port}{path}",
        data=data,
        headers={
            "Content-Type": content_type,
            "Origin": f"http://127.0.0.1:{port}",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as exc:
        return exc.code, exc.read()


def _write_workflow(repo: Path) -> Path:
    target = repo / "examples" / "wf" / "workflow.json"
    target.parent.mkdir(parents=True)
    target.write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")
    return target


def test_run_now_creates_run_returns_run_id(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    _write_workflow(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _post(port, "/workflows/run/examples/wf/workflow.json", {})
        assert status == 200
        payload = json.loads(body)
        assert payload["ok"] is True
        assert payload["data"]["run_id"].startswith("run_")
        assert payload["data"]["status"] == "running"
    finally:
        _stop_service(proc)


def test_run_now_without_mutations_returns_405(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    _write_workflow(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, body = _post(port, "/workflows/run/examples/wf/workflow.json", {})
        assert status == 405
        payload = json.loads(body)
        assert payload["error"]["code"] == 405
    finally:
        _stop_service(proc)


def test_run_now_invalid_workflow_returns_422(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    target = tmp_path / "examples" / "wf" / "workflow.json"
    target.parent.mkdir(parents=True)
    invalid = json.loads(json.dumps(_VALID_WORKFLOW))
    invalid["jobs"][0]["role_id"] = "missing_role"
    target.write_text(json.dumps(invalid), encoding="utf-8")
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _post(port, "/workflows/run/examples/wf/workflow.json", {})
        assert status == 422
        payload = json.loads(body)
        assert payload["error"]["code"] == 422
        assert payload["error"]["errors"]
        assert payload["error"]["errors"][0]["field_path"] == "jobs[0].role_id"
    finally:
        _stop_service(proc)


def test_run_now_missing_path_returns_404(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _post(port, "/workflows/run/does/not/exist.json", {})
        assert status == 404
    finally:
        _stop_service(proc)


def test_run_now_traversal_returns_400(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _ = _post(port, "/workflows/run/../etc/passwd", {})
        assert status == 400
    finally:
        _stop_service(proc)


def test_run_now_wrong_content_type_returns_415(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    _write_workflow(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _ = _post(
            port, "/workflows/run/examples/wf/workflow.json",
            {}, content_type="text/plain",
        )
        assert status == 415
    finally:
        _stop_service(proc)


def test_run_detail_page_has_run_now_button(tmp_path: Path) -> None:
    """Detail page should render the Run-now button only for valid workflows."""
    from test_web_ui import _http_get_raw
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    _write_workflow(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        _, _, body = _http_get_raw(port, "/workflows/examples/wf/workflow.json")
        assert b'id="run-now-btn"' in body
        assert b"workflow_run.js" in body
    finally:
        _stop_service(proc)
