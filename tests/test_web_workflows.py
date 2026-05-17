"""RFC 0024 V1: workflow browser tests."""

from __future__ import annotations

import json
import os
from pathlib import Path

from striatum.web.workflows import (
    WorkflowFileError,
    discover,
    load_workflow_at,
    save_workflow_file,
    workflow_edit_payload,
)

from test_web_ui import (
    _git_init_repo,
    _http_get_raw,
    _spawn_service,
    _stop_service,
    _striatum_init,
)


_VALID_WORKFLOW = {
    "schema_version": "striatum.workflow.v1",
    "workflow_id": "wf-test-valid",
    "workflow_version": "1",
    "name": "Test valid",
    "branch": {"mode": "confirm", "suggested_name": "wf/test", "allow_dirty": False},
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


def _same_model_warning_workflow() -> dict[str, object]:
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "wf-test-lint-warn",
        "workflow_version": "1",
        "name": "Test lint warning",
        "branch": {"mode": "confirm", "suggested_name": "wf/lint", "allow_dirty": False},
        "coordinator": {"role_id": "author", "lane_id": "author"},
        "lanes": {
            "author": {
                "adapter": "process",
                "display_model": "codex-gpt-5",
                "command": ["echo"],
                "worktree_isolation": "per_job",
            },
            "reviewer": {
                "adapter": "process",
                "display_model": "codex-gpt-5.1",
                "command": ["echo"],
                "capabilities": ["review"],
            },
        },
        "roles": {"author": {}, "reviewer": {}},
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 1,
            "require_disjoint_write_scopes": True,
        },
        "review_revision_policy": {"root_review_needs_revision": "human_checkpoint"},
        "jobs": [
            {
                "id": "draft",
                "type": "build",
                "title": "Draft",
                "role_id": "author",
                "lane_id": "author",
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": ["docs/drafts/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "draft",
                        "kind": "handoff",
                        "path": "docs/drafts/DRAFT.md",
                        "required": True,
                    }
                ],
            },
            {
                "id": "review",
                "type": "review",
                "title": "Review",
                "role_id": "reviewer",
                "lane_id": "reviewer",
                "fresh_session_required": True,
                "write_scope": {
                    "mode": "review_only_artifact",
                    "repo_write": False,
                    "allowed_paths": ["docs/reviews/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "review",
                        "kind": "finding",
                        "path": "docs/reviews/REVIEW.md",
                        "required": True,
                    }
                ],
            },
        ],
        "edges": [{"from": "draft", "to": "review", "on": "completed"}],
        "cycles": [],
    }


# --- discover() unit tests -----------------------------------------


def test_discover_finds_workflow_json(tmp_path: Path) -> None:
    (tmp_path / "examples").mkdir()
    (tmp_path / "examples" / "workflow.json").write_text(
        json.dumps(_VALID_WORKFLOW), encoding="utf-8",
    )
    found = discover(tmp_path)
    assert len(found) == 1
    assert found[0]["path"] == "examples/workflow.json"
    assert found[0]["status"] == "valid"
    assert found[0]["workflow_id"] == "wf-test-valid"
    assert found[0]["job_count"] == 1
    assert found[0]["modified_at"].endswith("Z")


def test_discover_surfaces_lint_warning_summary(tmp_path: Path) -> None:
    (tmp_path / "examples").mkdir()
    (tmp_path / "examples" / "workflow.json").write_text(
        json.dumps(_same_model_warning_workflow()), encoding="utf-8",
    )
    found = discover(tmp_path)

    assert found[0]["status"] == "valid"
    assert found[0]["lint_warning_count"] == 1
    assert found[0]["lint_warnings"][0]["rule"] == "same_model_review_pair"
    assert "same model family" in found[0]["lint_warnings"][0]["message"]


def test_discover_reports_modified_at_utc(tmp_path: Path) -> None:
    (tmp_path / "examples").mkdir()
    workflow_path = tmp_path / "examples" / "workflow.json"
    workflow_path.write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")
    os.utime(workflow_path, (1_700_000_000, 1_700_000_000))
    found = discover(tmp_path)
    assert found[0]["modified_at"] == "2023-11-14T22:13:20Z"


def test_discover_skips_hidden_dirs(tmp_path: Path) -> None:
    (tmp_path / ".git").mkdir()
    (tmp_path / ".git" / "workflow.json").write_text("{}", encoding="utf-8")
    (tmp_path / ".striatum").mkdir()
    (tmp_path / ".striatum" / "workflow.json").write_text("{}", encoding="utf-8")
    (tmp_path / ".venv").mkdir()
    (tmp_path / ".venv" / "workflow.json").write_text("{}", encoding="utf-8")
    (tmp_path / "node_modules").mkdir()
    (tmp_path / "node_modules" / "workflow.json").write_text("{}", encoding="utf-8")
    found = discover(tmp_path)
    assert found == []


def test_discover_invalid_json_reports_parse_error(tmp_path: Path) -> None:
    (tmp_path / "broken.dir").mkdir()
    (tmp_path / "broken.dir" / "workflow.json").write_text("{not json", encoding="utf-8")
    found = discover(tmp_path)
    assert len(found) == 1
    assert found[0]["status"] == "parse_error"
    assert "JSONDecodeError" in (found[0]["message"] or "")


def test_discover_invalid_workflow_reports_workflow_error(tmp_path: Path) -> None:
    invalid = dict(_VALID_WORKFLOW)
    invalid["jobs"] = [{"id": "x"}]  # missing required fields
    (tmp_path / "wf").mkdir()
    (tmp_path / "wf" / "workflow.json").write_text(json.dumps(invalid), encoding="utf-8")
    found = discover(tmp_path)
    assert len(found) == 1
    assert found[0]["status"] == "workflow_error"
    assert found[0]["message"]


def test_discover_sorts_paths(tmp_path: Path) -> None:
    (tmp_path / "z").mkdir()
    (tmp_path / "z" / "workflow.json").write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")
    (tmp_path / "a").mkdir()
    (tmp_path / "a" / "workflow.json").write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")
    found = discover(tmp_path)
    paths = [e["path"] for e in found]
    assert paths == sorted(paths)


# --- load_workflow_at() --------------------------------------------


def test_load_workflow_at_valid(tmp_path: Path) -> None:
    (tmp_path / "wf").mkdir()
    (tmp_path / "wf" / "workflow.json").write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")
    entry = load_workflow_at(tmp_path, "wf/workflow.json")
    assert entry is not None
    assert entry["status"] == "valid"
    assert "data" in entry
    assert str(entry["modified_at"]).endswith("Z")


def test_load_workflow_at_reports_modified_at_utc(tmp_path: Path) -> None:
    (tmp_path / "wf").mkdir()
    workflow_path = tmp_path / "wf" / "workflow.json"
    workflow_path.write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")
    os.utime(workflow_path, (1_700_000_000, 1_700_000_000))
    entry = load_workflow_at(tmp_path, "wf/workflow.json")
    assert entry is not None
    assert entry["modified_at"] == "2023-11-14T22:13:20Z"


def test_load_workflow_at_traversal_refused(tmp_path: Path) -> None:
    assert load_workflow_at(tmp_path, "../../etc/passwd") is None


def test_load_workflow_at_hidden_refused(tmp_path: Path) -> None:
    (tmp_path / ".git").mkdir()
    (tmp_path / ".git" / "workflow.json").write_text("{}", encoding="utf-8")
    assert load_workflow_at(tmp_path, ".git/workflow.json") is None


def test_load_workflow_at_missing(tmp_path: Path) -> None:
    assert load_workflow_at(tmp_path, "nope.json") is None


# --- workflow editor file helpers -----------------------------------


def test_workflow_edit_payload_scaffolds_missing_workflow(tmp_path: Path) -> None:
    payload = workflow_edit_payload(tmp_path, "examples/new-flow/workflow.json")

    assert payload["rel_path"] == "examples/new-flow/workflow.json"
    assert payload["is_new"] is True
    assert payload["workflow_sha256"] == ""
    assert payload["workflow_data"]["workflow_id"] == "new-flow"


def test_workflow_edit_payload_refuses_hidden_path(tmp_path: Path) -> None:
    try:
        workflow_edit_payload(tmp_path, ".striatum/workflow.json")
    except WorkflowFileError as exc:
        assert exc.status_code == 404
        assert exc.message == "hidden path"
    else:  # pragma: no cover - pytest assertion context.
        raise AssertionError("expected hidden path refusal")


def test_save_workflow_file_writes_valid_json_and_sha(tmp_path: Path) -> None:
    result = save_workflow_file(
        tmp_path,
        "examples/new-wf/workflow.json",
        dict(_VALID_WORKFLOW),
    )

    target = tmp_path / "examples" / "new-wf" / "workflow.json"
    assert result["path"] == "examples/new-wf/workflow.json"
    assert result["status"] == "saved"
    assert result["sha256"]
    assert json.loads(target.read_text(encoding="utf-8"))["workflow_id"] == "wf-test-valid"


def test_save_workflow_file_stale_if_match_reports_current_sha(tmp_path: Path) -> None:
    target = tmp_path / "examples" / "wf" / "workflow.json"
    target.parent.mkdir(parents=True)
    target.write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")

    try:
        save_workflow_file(
            tmp_path,
            "examples/wf/workflow.json",
            dict(_VALID_WORKFLOW),
            if_match="deadbeef",
        )
    except WorkflowFileError as exc:
        assert exc.status_code == 412
        assert exc.current_sha256
    else:  # pragma: no cover - pytest assertion context.
        raise AssertionError("expected If-Match refusal")


# --- routes ---------------------------------------------------------


def test_workflows_index_renders(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "examples").mkdir()
    (tmp_path / "examples" / "workflow.json").write_text(
        json.dumps(_VALID_WORKFLOW), encoding="utf-8",
    )
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/workflows")
        assert status == 200
        assert b"<h1>Workflows</h1>" in body
        assert b"examples/workflow.json" in body
        assert b"valid" in body
        assert b"Last modified" in body
        assert b"<time datetime=" in body
        assert b"workflows-index-data" in body
        assert b"/static/workflows_index.js" in body
        assert b'href="/workflows/new"' in body
    finally:
        _stop_service(proc)


def test_workflows_index_renders_lint_warning_count_and_text(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "examples").mkdir()
    (tmp_path / "examples" / "workflow.json").write_text(
        json.dumps(_same_model_warning_workflow()), encoding="utf-8",
    )
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/workflows")
        assert status == 200
        assert b"1 warning" in body
        assert b"same_model_review_pair" in body
        assert b"same model family" in body
    finally:
        _stop_service(proc)


def test_workflows_new_renders_chooser_island(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/workflows/new")
        assert status == 200
        assert b'id="island-workflow-chooser"' in body
        assert b"/static/build/island-workflow-chooser.js" in body
    finally:
        _stop_service(proc)


def test_workflows_edit_renders_graph_editor_island(tmp_path: Path) -> None:
    """RFC 0038 V1.5 regression — `/workflows/edit/<path>` ships the graph
    editor island bundle. Pins the bundle URL so packaging and template
    surface stay aligned with `/static/build/`."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "examples").mkdir()
    (tmp_path / "examples" / "workflow.json").write_text(
        json.dumps(_VALID_WORKFLOW), encoding="utf-8",
    )
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/workflows/edit/examples/workflow.json")
        assert status == 200
        assert b'id="island-workflow-graph-editor"' in body
        assert b"/static/build/island-workflow-graph-editor.js" in body
    finally:
        _stop_service(proc)


def test_workflows_index_empty_state(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/workflows")
        assert status == 200
        assert b"No workflow.json files found" in body
    finally:
        _stop_service(proc)


def test_workflow_detail_renders_valid(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "examples").mkdir()
    (tmp_path / "examples" / "workflow.json").write_text(
        json.dumps(_VALID_WORKFLOW), encoding="utf-8",
    )
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/workflows/examples/workflow.json")
        assert status == 200
        assert b"wf-test-valid" in body
        assert b"<svg" in body
        assert b"review_only" in body
    finally:
        _stop_service(proc)


def test_workflow_detail_renders_lint_warning_count_and_text(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    (tmp_path / "examples").mkdir()
    (tmp_path / "examples" / "workflow.json").write_text(
        json.dumps(_same_model_warning_workflow()), encoding="utf-8",
    )
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/workflows/examples/workflow.json")
        assert status == 200
        assert b"lint warnings: <code>1</code>" in body
        assert b"1 warning" in body
        assert b"same_model_review_pair" in body
        assert b"same model family" in body
    finally:
        _stop_service(proc)


def test_workflow_detail_renders_invalid_no_500(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    invalid = dict(_VALID_WORKFLOW)
    invalid["jobs"] = [{"id": "x"}]
    (tmp_path / "broken").mkdir()
    (tmp_path / "broken" / "workflow.json").write_text(json.dumps(invalid), encoding="utf-8")
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/workflows/broken/workflow.json")
        assert status == 200
        assert b"workflow error" in body
    finally:
        _stop_service(proc)


def test_workflow_detail_path_traversal_400(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, _ = _http_get_raw(port, "/workflows/../../etc/passwd")
        assert status in (400, 404)
    finally:
        _stop_service(proc)


def test_workflow_detail_missing_404(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, _ = _http_get_raw(port, "/workflows/nope/workflow.json")
        assert status == 404
    finally:
        _stop_service(proc)


def test_workflows_nav_link_present(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        _, _, body = _http_get_raw(port, "/")
        assert b'href="/workflows"' in body
    finally:
        _stop_service(proc)


# --- list_workflows chat tool --------------------------------------


def test_list_workflows_chat_tool(tmp_path: Path) -> None:
    from striatum.web.chat_tools import execute_tool
    (tmp_path / "wf").mkdir()
    (tmp_path / "wf" / "workflow.json").write_text(
        json.dumps(_VALID_WORKFLOW), encoding="utf-8",
    )
    out = execute_tool("list_workflows", {}, repo=tmp_path)
    assert "wf/workflow.json" in out
    assert "valid" in out
    assert "wf-test-valid" in out


def test_list_workflows_chat_tool_empty(tmp_path: Path) -> None:
    from striatum.web.chat_tools import execute_tool
    out = execute_tool("list_workflows", {}, repo=tmp_path)
    assert "[no workflow.json files found]" in out
