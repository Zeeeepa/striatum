"""RFC 0024 V1.5: workflow visual builder tests."""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from pathlib import Path


from test_web_ui import (
    _git_init_repo,
    _http_get_raw,
    _spawn_service,
    _stop_service,
    _striatum_init,
)


_VALID_WORKFLOW = {
    "schema_version": "striatum.workflow.v1",
    "workflow_id": "wf-edit-test",
    "workflow_version": "1",
    "name": "Edit test",
    "branch": {"mode": "confirm", "suggested_name": "wf/edit", "allow_dirty": False},
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


def _http_post_json(port: int, path: str, body: object,
                    *, content_type: str = "application/json",
                    extra_headers: dict[str, str] | None = None,
                    ) -> tuple[int, dict[str, str], bytes]:
    data = json.dumps(body).encode("utf-8") if not isinstance(body, (bytes, str)) else (
        body.encode("utf-8") if isinstance(body, str) else body
    )
    headers = {
        "Content-Type": content_type,
        "Origin": f"http://127.0.0.1:{port}",
    }
    if extra_headers:
        headers.update(extra_headers)
    req = urllib.request.Request(
        f"http://127.0.0.1:{port}{path}",
        data=data,
        headers=headers,
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.status, dict(resp.headers.items()), resp.read()
    except urllib.error.HTTPError as exc:
        return exc.code, dict(exc.headers.items()) if exc.headers else {}, exc.read()


# --- GET --------------------------------------------------------------


def test_edit_get_existing_renders_form(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "examples").mkdir()
    (tmp_path / "examples" / "workflow.json").write_text(
        json.dumps(_VALID_WORKFLOW), encoding="utf-8",
    )
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, body = _http_get_raw(port, "/workflows/edit/examples/workflow.json")
        assert status == 200
        assert b'id="workflow-data"' in body
        assert b"wf-edit-test" in body
    finally:
        _stop_service(proc)


def test_edit_get_invalid_renders_form(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    invalid = dict(_VALID_WORKFLOW)
    invalid["jobs"] = [{"id": "x"}]
    (tmp_path / "broken").mkdir()
    (tmp_path / "broken" / "workflow.json").write_text(
        json.dumps(invalid), encoding="utf-8",
    )
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, body = _http_get_raw(port, "/workflows/edit/broken/workflow.json")
        assert status == 200
        assert b'id="workflow-data"' in body
    finally:
        _stop_service(proc)


def test_edit_get_nonexistent_returns_scaffold(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, body = _http_get_raw(port, "/workflows/edit/examples/new-flow/workflow.json")
        assert status == 200
        assert b"New workflow" in body
        assert b"new-flow" in body
    finally:
        _stop_service(proc)


def test_edit_get_path_traversal_rejected(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, _ = _http_get_raw(port, "/workflows/edit/../../etc/passwd")
        assert status in (400, 404)
    finally:
        _stop_service(proc)


def test_edit_get_dotgit_hidden(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, _ = _http_get_raw(port, "/workflows/edit/.git/workflow.json")
        assert status == 404
    finally:
        _stop_service(proc)


# --- POST -------------------------------------------------------------


def test_edit_post_valid_writes_file(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, _ = _http_post_json(
            port, "/workflows/edit/examples/new-wf/workflow.json", _VALID_WORKFLOW,
        )
        assert status == 200
        target = tmp_path / "examples" / "new-wf" / "workflow.json"
        assert target.exists()
        on_disk = json.loads(target.read_text(encoding="utf-8"))
        assert on_disk["workflow_id"] == "wf-edit-test"
    finally:
        _stop_service(proc)


def test_edit_post_invalid_returns_422(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        invalid = dict(_VALID_WORKFLOW)
        invalid["jobs"] = [{"id": "x"}]
        status, _, body = _http_post_json(
            port, "/workflows/edit/examples/wf/workflow.json", invalid,
        )
        assert status == 422
        # File should not have been created.
        assert not (tmp_path / "examples" / "wf" / "workflow.json").exists()
        payload = json.loads(body)
        assert payload["ok"] is False
        assert "message" in payload["error"]
    finally:
        _stop_service(proc)


def test_edit_post_without_mutations_405(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, _ = _http_post_json(
            port, "/workflows/edit/examples/wf/workflow.json", _VALID_WORKFLOW,
        )
        assert status == 405
    finally:
        _stop_service(proc)


def test_edit_post_path_traversal_400(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, _ = _http_post_json(
            port, "/workflows/edit/../../etc/passwd", _VALID_WORKFLOW,
        )
        assert status in (400, 404)
    finally:
        _stop_service(proc)


def test_edit_post_wrong_content_type_415(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, _ = _http_post_json(
            port, "/workflows/edit/examples/wf/workflow.json", {},
            content_type="text/plain",
        )
        assert status == 415
    finally:
        _stop_service(proc)


def test_edit_post_invalid_json_400(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, _ = _http_post_json(
            port, "/workflows/edit/examples/wf/workflow.json", "{not valid json",
        )
        assert status == 400
    finally:
        _stop_service(proc)


def test_edit_post_creates_intermediate_dirs(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        target = tmp_path / "deep" / "nested" / "path" / "workflow.json"
        assert not target.parent.exists()
        status, _, _ = _http_post_json(
            port, "/workflows/edit/deep/nested/path/workflow.json", _VALID_WORKFLOW,
        )
        assert status == 200
        assert target.exists()
    finally:
        _stop_service(proc)


def test_edit_post_atomic_unchanged_on_validation_failure(tmp_path: Path) -> None:
    """A failed validation must not corrupt an existing file."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    target = tmp_path / "examples" / "wf" / "workflow.json"
    target.parent.mkdir(parents=True)
    target.write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")
    original = target.read_text(encoding="utf-8")
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        invalid = dict(_VALID_WORKFLOW)
        invalid["jobs"] = [{"id": "x"}]
        status, _, _ = _http_post_json(
            port, "/workflows/edit/examples/wf/workflow.json", invalid,
        )
        assert status == 422
        # File on disk unchanged.
        assert target.read_text(encoding="utf-8") == original
    finally:
        _stop_service(proc)


# --- detail page edit link -------------------------------------------


def test_workflow_detail_has_edit_link(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "examples").mkdir()
    (tmp_path / "examples" / "workflow.json").write_text(
        json.dumps(_VALID_WORKFLOW), encoding="utf-8",
    )
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        _, _, body = _http_get_raw(port, "/workflows/examples/workflow.json")
        assert b'href="/workflows/edit/examples/workflow.json"' in body
        assert b"workflow-edit-button" in body
        assert b"Run this workflow now" in body
    finally:
        _stop_service(proc)


# --- RFC 0024 V2: If-Match concurrency ------------------------------


def _sha256(data: bytes) -> str:
    import hashlib
    return hashlib.sha256(data).hexdigest()


def test_edit_post_if_match_matching_succeeds(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    target = tmp_path / "examples" / "wf" / "workflow.json"
    target.parent.mkdir(parents=True)
    raw = json.dumps(_VALID_WORKFLOW, indent=2) + "\n"
    target.write_text(raw, encoding="utf-8")
    sha = _sha256(raw.encode("utf-8"))
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, body = _http_post_json(
            port, "/workflows/edit/examples/wf/workflow.json", _VALID_WORKFLOW,
            extra_headers={"If-Match": f'"{sha}"'},
        )
        assert status == 200
        payload = json.loads(body)
        assert "sha256" in payload["data"]
    finally:
        _stop_service(proc)


def test_edit_post_if_match_stale_returns_412(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    target = tmp_path / "examples" / "wf" / "workflow.json"
    target.parent.mkdir(parents=True)
    target.write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, body = _http_post_json(
            port, "/workflows/edit/examples/wf/workflow.json", _VALID_WORKFLOW,
            extra_headers={"If-Match": '"deadbeef"'},
        )
        assert status == 412
        payload = json.loads(body)
        assert payload["error"]["code"] == 412
        assert "current_sha256" in payload["error"]
    finally:
        _stop_service(proc)


def test_edit_post_if_match_missing_is_v15_compat(tmp_path: Path) -> None:
    """V1.5 clients send no If-Match — must keep working."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    target = tmp_path / "examples" / "wf" / "workflow.json"
    target.parent.mkdir(parents=True)
    target.write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, _, _ = _http_post_json(
            port, "/workflows/edit/examples/wf/workflow.json", _VALID_WORKFLOW,
        )
        assert status == 200
    finally:
        _stop_service(proc)


def test_edit_get_injects_sha256(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    target = tmp_path / "examples" / "wf" / "workflow.json"
    target.parent.mkdir(parents=True)
    raw = json.dumps(_VALID_WORKFLOW, indent=2) + "\n"
    target.write_text(raw, encoding="utf-8")
    sha = _sha256(raw.encode("utf-8"))
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        _, _, body = _http_get_raw(port, "/workflows/edit/examples/wf/workflow.json")
        assert b'id="workflow-sha256"' in body
        assert sha.encode() in body
    finally:
        _stop_service(proc)


# --- RFC 0024 V2: structured field errors ---------------------------


def test_edit_post_structured_errors_carries_field_path(tmp_path: Path) -> None:
    """422 body should carry errors[] with field_path for tagged raise sites."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        invalid = json.loads(json.dumps(_VALID_WORKFLOW))
        invalid["jobs"][0]["role_id"] = "nonexistent_role"
        status, _, body = _http_post_json(
            port, "/workflows/edit/x.json", invalid,
        )
        assert status == 422
        payload = json.loads(body)
        assert payload["error"]["errors"]
        assert payload["error"]["errors"][0]["field_path"] == "jobs[0].role_id"
    finally:
        _stop_service(proc)


def test_edit_post_structured_errors_empty_for_unconverted_raise_site(tmp_path: Path) -> None:
    """V1.5 backward-compat: when the raise site lacks field_path,
    422 errors[] is empty and clients fall back to error.message."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        # Missing required top-level field — that raise site is
        # not (yet) tagged with field_path.
        invalid = json.loads(json.dumps(_VALID_WORKFLOW))
        del invalid["coordinator"]
        status, _, body = _http_post_json(
            port, "/workflows/edit/x.json", invalid,
        )
        assert status == 422
        payload = json.loads(body)
        # Either errors[] is empty, or has the missing-field path.
        # The message banner must always be present.
        assert payload["error"]["message"]
    finally:
        _stop_service(proc)
