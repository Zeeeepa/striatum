from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from striatum.api import invoke
from striatum.workflow import lint_workflow


def _risky_review_workflow() -> dict[str, Any]:
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "wf-lint",
        "workflow_version": "1",
        "name": "Lint fixture",
        "branch": {"mode": "auto", "suggested_name": "striatum/lint"},
        "coordinator": {"role_id": "author", "lane_id": "author"},
        "lanes": {
            "author": {
                "adapter": "process",
                "display_model": "codex-gpt-5",
                "command": ["echo"],
            },
            "reviewer": {
                "adapter": "process",
                "display_model": "codex-gpt-5.1",
                "command": ["echo"],
            },
        },
        "roles": {"author": {}, "reviewer": {}},
        "context_docs": [],
        "parallelism": {"mode": "declared", "max_active_jobs": 1, "require_disjoint_write_scopes": True},
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
                    "allowed_paths": ["."],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "draft",
                        "kind": "handoff",
                        "path": "docs/DRAFT.md",
                    }
                ],
            },
            {
                "id": "review",
                "type": "review",
                "title": "Review",
                "role_id": "reviewer",
                "lane_id": "reviewer",
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
                        "path": "docs/reviews/FINDING.md",
                    }
                ],
            },
        ],
        "edges": [{"from": "draft", "to": "review", "on": "completed"}],
        "cycles": [],
    }


def _same_model_revision_cycle_workflow() -> dict[str, Any]:
    workflow = _risky_review_workflow()
    workflow["workflow_id"] = "wf-lint-cycle"
    workflow["review_revision_policy"] = {"root_review_needs_revision": "declared_cycle"}
    workflow["cycles"] = [
        {
            "from": "review",
            "to": "draft",
            "on_verdict": "needs_revision",
            "max_iterations": 1,
        }
    ]
    return workflow


def test_lint_workflow_reports_dogfood_risk_rules() -> None:
    payload = lint_workflow(_risky_review_workflow())

    assert payload["valid"] is True
    rules = {warning["rule"] for warning in payload["warnings"]}
    assert {
        "same_model_review_pair",
        "review_without_fresh_context",
        "broad_write_scope",
        "missing_review_escalation_path",
    }.issubset(rules)
    assert "repo_write_without_worktree_isolation" not in rules
    coverage = payload["coverage"]
    assert coverage["level"] == "weak"
    assert coverage["score"] < coverage["max_score"]
    check_ids = {check["id"] for check in coverage["checks"]}
    assert {
        "reviewer_independence",
        "fresh_context",
        "write_isolation",
        "revision_or_escalation_path",
        "posture_diversity",
    } == check_ids


def test_lint_workflow_reports_same_model_review_pair_and_revision_cycle() -> None:
    payload = lint_workflow(_same_model_revision_cycle_workflow())

    assert payload["valid"] is True
    rules = {warning["rule"] for warning in payload["warnings"]}
    assert "same_model_review_pair" in rules
    assert "same_model_revision_cycle" in rules


def test_lint_workflow_reports_worktree_isolation_only_when_repo_writes_can_overlap() -> None:
    workflow = _risky_review_workflow()
    workflow["parallelism"] = {"mode": "declared", "max_active_jobs": 2}

    payload = lint_workflow(workflow)

    assert payload["valid"] is True
    rules = {warning["rule"] for warning in payload["warnings"]}
    assert "repo_write_without_worktree_isolation" in rules


def test_workflow_lint_cli_returns_structured_warnings(
    tmp_path: Path, monkeypatch: Any
) -> None:
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(_risky_review_workflow()), encoding="utf-8")

    result = invoke(["workflow", "lint", str(workflow_path), "--json"], repo=tmp_path)

    assert result["ok"] is True
    payload = result["data"]
    assert payload["workflow_id"] == "wf-lint"
    assert payload["warning_count"] >= 4


def test_workflow_validate_refuses_same_model_pairing_without_override(
    tmp_path: Path, monkeypatch: Any
) -> None:
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(_same_model_revision_cycle_workflow()), encoding="utf-8")

    result = invoke(["workflow", "validate", str(workflow_path)], repo=tmp_path)

    assert result["ok"] is False
    error = result["error"]
    assert error["code"] == 8
    assert "--allow-same-model-pairing" in error["message"]
    details = error["details"]
    assert details["rule"] == "same_model_review_pair"
    rules = {finding["rule"] for finding in details["findings"]}
    assert {"same_model_review_pair", "same_model_revision_cycle"}.issubset(rules)


def test_workflow_validate_accepts_same_model_pairing_with_explicit_override(
    tmp_path: Path, monkeypatch: Any
) -> None:
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(_same_model_revision_cycle_workflow()), encoding="utf-8")

    result = invoke(
        ["workflow", "validate", str(workflow_path), "--allow-same-model-pairing"],
        repo=tmp_path,
    )

    assert result["ok"] is True
    payload = result["data"]
    assert payload["workflow_id"] == "wf-lint-cycle"
    assert payload["valid"] is True


def test_workflow_lint_strict_refuses_warnings_without_override(
    tmp_path: Path, monkeypatch: Any
) -> None:
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(_risky_review_workflow()), encoding="utf-8")

    result = invoke(
        ["workflow", "lint", str(workflow_path), "--strict", "--json"],
        repo=tmp_path,
    )

    assert result["ok"] is False
    error = result["error"]
    assert error["code"] == 8
    details = error["details"]
    assert details["strict"]["mode"] == "refused"
    assert details["strict"]["reason"] == "warnings_require_override_rationale"
    assert details["warning_count"] >= 4


def test_workflow_lint_strict_refuses_same_model_revision_cycle_without_override(
    tmp_path: Path, monkeypatch: Any
) -> None:
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(
        json.dumps(_same_model_revision_cycle_workflow()), encoding="utf-8",
    )

    result = invoke(
        ["workflow", "lint", str(workflow_path), "--strict", "--json"],
        repo=tmp_path,
    )

    assert result["ok"] is False
    details = result["error"]["details"]
    rules = {warning["rule"] for warning in details["warnings"]}
    assert details["strict"]["reason"] == "warnings_require_override_rationale"
    assert "same_model_revision_cycle" in rules


def test_workflow_lint_strict_accepts_explicit_override(
    tmp_path: Path, monkeypatch: Any
) -> None:
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(_risky_review_workflow()), encoding="utf-8")

    result = invoke(
        [
            "workflow",
            "lint",
            str(workflow_path),
            "--strict",
            "--override-rationale",
            "accepted for fixture coverage",
            "--json",
        ],
        repo=tmp_path,
    )

    assert result["ok"] is True
    payload = result["data"]
    assert payload["strict"] == {
        "mode": "overridden",
        "warning_count": payload["warning_count"],
        "override_rationale": "accepted for fixture coverage",
    }


def test_workflow_lint_strict_override_records_accepted_risk_decision(
    tmp_path: Path, monkeypatch: Any
) -> None:
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(_risky_review_workflow()), encoding="utf-8")

    result = invoke(
        [
            "workflow",
            "lint",
            str(workflow_path),
            "--strict",
            "--override-rationale",
            "accepted for dogfood continuity",
            "--accepted-risk-decision-id",
            "D123",
            "--json",
        ],
        repo=tmp_path,
    )

    assert result["ok"] is True
    payload = result["data"]
    assert payload["strict"] == {
        "mode": "overridden",
        "warning_count": payload["warning_count"],
        "override_rationale": "accepted for dogfood continuity",
        "accepted_risk_decision_id": "D123",
    }


def test_workflow_lint_override_rationale_requires_strict(
    tmp_path: Path, monkeypatch: Any
) -> None:
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(_risky_review_workflow()), encoding="utf-8")

    result = invoke(
        [
            "workflow",
            "lint",
            str(workflow_path),
            "--override-rationale",
            "not attached to strict mode",
        ],
        repo=tmp_path,
    )

    assert result["ok"] is False
    assert result["error"]["code"] == 2
    assert "--strict" in result["error"]["message"]


def test_workflow_lint_accepted_risk_decision_requires_strict_override(
    tmp_path: Path, monkeypatch: Any
) -> None:
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(_risky_review_workflow()), encoding="utf-8")

    result = invoke(
        [
            "workflow",
            "lint",
            str(workflow_path),
            "--accepted-risk-decision-id",
            "D123",
        ],
        repo=tmp_path,
    )

    assert result["ok"] is False
    assert result["error"]["code"] == 2
    assert "--override-rationale" in result["error"]["message"]


def test_lint_workflow_reports_structural_errors_without_raising() -> None:
    payload = lint_workflow({"schema_version": "striatum.workflow.v1"})

    assert payload["valid"] is False
    assert "missing required fields" in payload["errors"][0]["message"]
    assert payload["coverage"]["level"] == "weak"


def test_workflow_lint_strict_refuses_invalid_workflow(
    tmp_path: Path, monkeypatch: Any
) -> None:
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(
        json.dumps({"schema_version": "striatum.workflow.v1"}),
        encoding="utf-8",
    )

    result = invoke(["workflow", "lint", str(workflow_path), "--strict"], repo=tmp_path)

    assert result["ok"] is False
    assert result["error"]["code"] == 8
    details = result["error"]["details"]
    assert details["valid"] is False
    assert details["strict"] == {
        "mode": "refused",
        "reason": "invalid_workflow",
    }
