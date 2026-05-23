from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from striatum import service_routes
from striatum.web.static_assets import load_static_asset
from striatum.web.template_env import jinja_env
from striatum.web.workflows import (
    WorkflowRouteContext,
    handle_workflow_accept_risk,
    workflow_detail_page_response,
)


_VALID_WORKFLOW: dict[str, Any] = {
    "schema_version": "striatum.workflow.v1",
    "workflow_id": "wf-accepted-risk-ui",
    "workflow_version": "1",
    "name": "Accepted risk UI",
    "branch": {"mode": "confirm", "suggested_name": "wf/accepted-risk"},
    "coordinator": {"role_id": "reviewer", "lane_id": "lane_a"},
    "lanes": {
        "lane_a": {
            "adapter": "process",
            "display_model": "codex-gpt-5",
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
            "task_prompt": {"path": "p.md"},
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
                },
            ],
        },
    ],
    "edges": [],
    "cycles": [],
}


def _write_workflow(repo: Path) -> Path:
    path = repo / "workflows" / "demo" / "workflow.json"
    path.parent.mkdir(parents=True)
    path.write_text(json.dumps(_VALID_WORKFLOW), encoding="utf-8")
    return path


def test_workflow_detail_uses_daemon_lint_and_accepted_risk_list(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    _write_workflow(tmp_path)
    calls: list[tuple[str, dict[str, Any]]] = []

    def fake_call_repo_method(_repo: Path, method: str, params: dict[str, Any]) -> dict[str, Any]:
        calls.append((method, dict(params)))
        if method == "workflow.lint":
            return {
                "workflow_id": "wf-accepted-risk-ui",
                "valid": True,
                "errors": [],
                "warnings": [
                    {
                        "rule": "same_model_review_pair",
                        "severity": "warning",
                        "message": "same model family",
                        "fingerprint": "f1" * 32,
                        "accepted": True,
                        "accepted_risk_ids": ["war_existing"],
                    },
                    {
                        "rule": "broad_write_scope",
                        "severity": "warning",
                        "message": "broad write scope",
                        "fingerprint": "f2" * 32,
                        "accepted": False,
                    },
                ],
                "warning_count": 2,
                "coverage": {"level": "weak"},
                "workflow_fingerprint_sha256": "a" * 64,
                "workflow_snapshot_id": None,
                "source_path": "workflows/demo/workflow.json",
            }
        if method == "workflow.accepted_risks.list":
            assert params == {"workflow_fingerprint_sha256": "a" * 64}
            return {
                "accepted_risks": [
                    {
                        "accepted_risk_id": "war_existing",
                        "workflow_fingerprint_sha256": "a" * 64,
                        "workflow_id": "wf-accepted-risk-ui",
                        "workflow_version": "1",
                        "source_path": "workflows/demo/workflow.json",
                        "lint_rule": "same_model_review_pair",
                        "finding_fingerprint_sha256": "f1" * 32,
                        "finding": {"message": "same model family"},
                        "decision_artifact_ref": "docs/decisions/D124.md",
                        "accepted_by": "operator",
                        "accepted_at": "2026-05-23T00:00:00Z",
                        "rationale": "documented operator decision",
                    }
                ],
                "count": 1,
            }
        raise AssertionError(f"unexpected method: {method}")

    monkeypatch.setattr("striatum.service_daemon.call_repo_method", fake_call_repo_method)

    response = workflow_detail_page_response(
        tmp_path,
        "workflows/demo/workflow.json",
        include_daemon_lint=True,
    )

    assert calls[0] == (
        "workflow.lint",
        {"workflow_path": "workflows/demo/workflow.json"},
    )
    assert calls[1][0] == "workflow.accepted_risks.list"
    assert response.workflow["lint_source"] == "daemon"
    assert response.workflow["lint_warning_count"] == 2
    assert response.workflow["accepted_risk_count"] == 1
    assert response.workflow["lint_warnings"][0]["accepted"] is True
    assert response.workflow["lint_warnings"][1]["fingerprint"] == "f2" * 32


def test_workflow_detail_template_renders_accept_risk_controls(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    _write_workflow(tmp_path)

    def fake_call_repo_method(_repo: Path, method: str, _params: dict[str, Any]) -> dict[str, Any]:
        if method == "workflow.lint":
            return {
                "workflow_id": "wf-accepted-risk-ui",
                "valid": True,
                "warnings": [
                    {
                        "rule": "broad_write_scope",
                        "severity": "warning",
                        "message": "broad write scope",
                        "fingerprint": "f2" * 32,
                        "accepted": False,
                    }
                ],
                "warning_count": 1,
                "coverage": {"level": "weak"},
                "workflow_fingerprint_sha256": "a" * 64,
            }
        if method == "workflow.accepted_risks.list":
            return {"accepted_risks": [], "count": 0}
        raise AssertionError(f"unexpected method: {method}")

    monkeypatch.setattr("striatum.service_daemon.call_repo_method", fake_call_repo_method)
    response = workflow_detail_page_response(
        tmp_path,
        "workflows/demo/workflow.json",
        include_daemon_lint=True,
    )

    html = jinja_env().get_template("workflow_detail.html").render(
        workflow=response.workflow,
        graph_svg=response.graph_svg,
        allow_mutations=True,
    )

    assert "data-accept-risk-open" in html
    assert 'data-fingerprint="f2f2' in html
    assert "Accept risk" in html
    assert "workflow_accepted_risk.js" in html
    assert load_static_asset("workflow_accepted_risk.js").data


def test_handle_workflow_accept_risk_routes_daemon_mutation_without_file_write(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    workflow_path = _write_workflow(tmp_path)
    original = workflow_path.read_text(encoding="utf-8")
    responses: list[tuple[int, dict[str, Any]]] = []
    calls: list[tuple[str, dict[str, Any]]] = []
    body = {
        "finding_fingerprint_sha256": "f2" * 32,
        "decision_artifact_ref": "docs/decisions/D124.md",
        "rationale": "accepted for bounded dogfood continuity",
        "accepted_by": "operator",
    }

    def fake_call_repo_method(_repo: Path, method: str, params: dict[str, Any]) -> dict[str, Any]:
        calls.append((method, dict(params)))
        return {"accepted_risks": [{"accepted_risk_id": "war_new"}], "count": 1}

    monkeypatch.setattr("striatum.service_daemon.call_repo_method", fake_call_repo_method)
    ctx = WorkflowRouteContext(
        repo=tmp_path,
        allow_mutations=True,
        headers={},
        rfile=None,
        send_json=lambda status, payload: responses.append((status, payload)),
        send_html=lambda _status, _html: None,
        read_json_body_strict=lambda _max_bytes: dict(body),
        jinja_env=jinja_env,
    )

    handle_workflow_accept_risk(ctx, "workflows/demo/workflow.json")

    assert responses[0][0] == 200
    assert calls == [
        (
            "workflow.accept_risk",
            {
                "workflow_path": "workflows/demo/workflow.json",
                "finding_fingerprints": ["f2" * 32],
                "decision_artifact_ref": "docs/decisions/D124.md",
                "rationale": "accepted for bounded dogfood continuity",
                "accepted_by": "operator",
            },
        )
    ]
    assert workflow_path.read_text(encoding="utf-8") == original


def test_handle_workflow_accept_risk_requires_mutation_gate(tmp_path: Path) -> None:
    _write_workflow(tmp_path)
    responses: list[tuple[int, dict[str, Any]]] = []
    ctx = WorkflowRouteContext(
        repo=tmp_path,
        allow_mutations=False,
        headers={},
        rfile=None,
        send_json=lambda status, payload: responses.append((status, payload)),
        send_html=lambda _status, _html: None,
        read_json_body_strict=lambda _max_bytes: (_ for _ in ()).throw(
            AssertionError("body should not be read")
        ),
        jinja_env=jinja_env,
    )

    handle_workflow_accept_risk(ctx, "workflows/demo/workflow.json")

    assert responses == [
        (
            405,
            {
                "ok": False,
                "error": {"code": 405, "message": "accept risk requires --allow-mutations"},
            },
        )
    ]


def test_service_routes_dispatches_workflow_accept_risk_post() -> None:
    class Handler:
        path = "/workflows/accept-risk/workflows/demo/workflow.json"

        class State:
            web_enabled = True

        state = State()
        called: str | None = None

        def _authenticate(self) -> bool:
            return True

        def _requires_same_origin(self, _path: str) -> bool:
            return False

        def _handle_workflow_accept_risk(self, rel_path: str) -> None:
            self.called = rel_path

    handler = Handler()

    service_routes.dispatch_post(handler)

    assert handler.called == "workflows/demo/workflow.json"
