# ruff: noqa
"""Tests for RFC 0018 V1 (Review Postures): validator + packet exposure.

V1 covers steps 1+2: ``review_posture`` validator + packet exposure, and
``required_review_postures`` + the workflow-validation reachability gate.
Step 3 (verdicts.posture column + introspection surfaces) is deferred per
the RFC's own implementation path; not exercised here.
"""

from __future__ import annotations
import pytest; pytest.skip("legacy sqlite / web_ui test helpers are retired", allow_module_level=True)

from pathlib import Path
from typing import Any, cast

import pytest

from striatum.errors import WorkflowError
from striatum.workflow import ALLOWED_POSTURES, POSTURE_INSTRUCTIONS, validate_workflow

from test_cli_mvp import (
    JsonDict,
    data,
    init_repo,
    run_cli,
    temporary_workflow,
)


def _base_workflow_with_review_only() -> JsonDict:
    """Return a minimal valid single-review-job workflow."""
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "wf-postures",
        "workflow_version": "1",
        "name": "Posture",
        "branch": {"mode": "confirm", "suggested_name": "wf/posture", "allow_dirty": False},
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
                    {
                        "logical_name": "review",
                        "kind": "finding",
                        "path": "docs/reviews/REVIEW.md",
                        "required": True,
                    }
                ],
            },
        ],
        "edges": [],
        "cycles": [],
    }


def _base_workflow_with_build_then_review() -> JsonDict:
    """Build job → review job, both reachable via a forward edge."""
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "wf-postures-build",
        "workflow_version": "1",
        "name": "Posture build",
        "branch": {"mode": "confirm", "suggested_name": "wf/posture-b", "allow_dirty": False},
        "coordinator": {"role_id": "implementer", "lane_id": "lane_a"},
        "lanes": {
            "lane_a": {
                "adapter": "process",
                "display_model": "X",
                "command": ["echo"],
                "capabilities": ["review", "write"],
            },
        },
        "roles": {
            "implementer": {"definition_path": "roles/impl.md"},
            "reviewer": {"definition_path": "roles/rev.md"},
        },
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 1,
            "require_disjoint_write_scopes": True,
        },
        "jobs": [
            {
                "id": "build_step",
                "type": "build",
                "title": "Build",
                "role_id": "implementer",
                "lane_id": "lane_a",
                "objective": "Build",
                "task_prompt": {"path": "prompts/build.md"},
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": ["src/", "docs/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "handoff",
                        "kind": "handoff",
                        "path": "docs/BUILD.md",
                        "required": True,
                    }
                ],
            },
            {
                "id": "review_step",
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
                    {
                        "logical_name": "review",
                        "kind": "finding",
                        "path": "docs/reviews/REVIEW.md",
                        "required": True,
                    }
                ],
            },
        ],
        "edges": [{"from": "build_step", "to": "review_step", "on": "completed"}],
        "cycles": [],
    }


# --- Step 1: review_posture validator ----------------------------------


def test_review_posture_rejected_on_non_review_job() -> None:
    workflow = _base_workflow_with_build_then_review()
    workflow["jobs"][0]["review_posture"] = "security"  # build job
    with pytest.raises(
        WorkflowError,
        match=r"non-review job 'build_step' cannot declare review_posture",
    ):
        validate_workflow(workflow)


def test_review_posture_rejects_unknown_value() -> None:
    workflow = _base_workflow_with_review_only()
    workflow["jobs"][0]["review_posture"] = "telepathy"
    with pytest.raises(WorkflowError, match=r"unknown review_posture 'telepathy'"):
        validate_workflow(workflow)


def test_review_posture_rejects_empty_string() -> None:
    workflow = _base_workflow_with_review_only()
    workflow["jobs"][0]["review_posture"] = ""
    with pytest.raises(WorkflowError, match=r"review_posture must be a non-empty string"):
        validate_workflow(workflow)


def test_review_posture_rejects_bare_custom_prefix() -> None:
    workflow = _base_workflow_with_review_only()
    workflow["jobs"][0]["review_posture"] = "custom:"
    with pytest.raises(WorkflowError, match=r"review_posture 'custom:' has empty custom name"):
        validate_workflow(workflow)


def test_review_posture_rejects_whitespace_only_custom_name() -> None:
    workflow = _base_workflow_with_review_only()
    workflow["jobs"][0]["review_posture"] = "custom:   "
    with pytest.raises(WorkflowError, match=r"has empty custom name"):
        validate_workflow(workflow)


def test_review_posture_accepts_each_first_class_value() -> None:
    for value in sorted(ALLOWED_POSTURES):
        workflow = _base_workflow_with_review_only()
        workflow["jobs"][0]["review_posture"] = value
        validate_workflow(workflow)


def test_review_posture_accepts_custom_named_value() -> None:
    for value in ("custom:my_thing", "custom:foo_bar", "custom:security:strict"):
        workflow = _base_workflow_with_review_only()
        workflow["jobs"][0]["review_posture"] = value
        validate_workflow(workflow)


# --- Step 1: packet exposure ------------------------------------------


def _packet_for_review_with_posture(tmp_path: Path, posture: str | None) -> dict[str, Any]:
    """Prepare a run with a single review job, claim it, return the packet."""
    workflow = _base_workflow_with_review_only()
    if posture is not None:
        workflow["jobs"][0]["review_posture"] = posture
    # Add an access-scope so RFC 0002's review_policy block path is exercised
    # for both posture-declared and posture-omitted cases (zero-regression).
    workflow["jobs"][0]["reviewer_access_scope"] = "document_only"
    workflow_path = temporary_workflow(tmp_path, workflow)
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(workflow_path)))
    run_id = str(prepared["run_id"])
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "wf/posture-test")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)
    sess = data(run_cli(
        tmp_path, "register-session",
        "--run-id", run_id, "--role", "reviewer", "--lane", "lane_a", "--fresh",
    ))
    claim = data(run_cli(tmp_path, "claim-next", "--session-id", str(sess["session_id"])))
    return cast(dict[str, Any], claim["packet"])


def test_packet_exposes_posture_when_declared(tmp_path: Path) -> None:
    packet = _packet_for_review_with_posture(tmp_path, "security")
    review_policy = packet.get("review_policy")
    assert review_policy is not None
    assert review_policy.get("posture") == "security"
    assert "security-focused review" in review_policy["instruction"]


def test_packet_omits_posture_when_undeclared(tmp_path: Path) -> None:
    packet = _packet_for_review_with_posture(tmp_path, None)
    review_policy = packet.get("review_policy")
    assert review_policy is not None
    assert "posture" not in review_policy
    # Instruction must not include any posture sentence.
    for sentence in POSTURE_INSTRUCTIONS.values():
        if sentence:
            assert sentence not in review_policy["instruction"]


def test_packet_instruction_appends_posture_sentence_for_first_class(
    tmp_path: Path,
) -> None:
    packet = _packet_for_review_with_posture(tmp_path, "threat_model")
    review_policy = packet.get("review_policy")
    assert review_policy is not None
    assert POSTURE_INSTRUCTIONS["threat_model"] in review_policy["instruction"]


def test_packet_instruction_unchanged_for_neutral_posture(tmp_path: Path) -> None:
    """Declaring `neutral` exposes the posture key but appends no sentence."""
    packet = _packet_for_review_with_posture(tmp_path, "neutral")
    review_policy = packet.get("review_policy")
    assert review_policy is not None
    assert review_policy["posture"] == "neutral"
    for sentence in POSTURE_INSTRUCTIONS.values():
        if sentence:
            assert sentence not in review_policy["instruction"]


def test_packet_instruction_unchanged_for_custom_posture(tmp_path: Path) -> None:
    """Custom postures expose the literal string but get no auto-sentence."""
    packet = _packet_for_review_with_posture(tmp_path, "custom:my_thing")
    review_policy = packet.get("review_policy")
    assert review_policy is not None
    assert review_policy["posture"] == "custom:my_thing"
    for sentence in POSTURE_INSTRUCTIONS.values():
        if sentence:
            assert sentence not in review_policy["instruction"]


# --- Step 2: required_review_postures validator -----------------------


def test_required_review_postures_rejected_on_non_review_job() -> None:
    """The field is allowed on build jobs only; non-review-non-build also rejects."""
    workflow = _base_workflow_with_review_only()
    workflow["jobs"][0]["required_review_postures"] = ["security"]
    with pytest.raises(
        WorkflowError,
        match=r"non-build job 'review_only' cannot declare required_review_postures",
    ):
        validate_workflow(workflow)


def test_required_review_postures_rejects_empty_list() -> None:
    workflow = _base_workflow_with_build_then_review()
    workflow["jobs"][0]["required_review_postures"] = []
    with pytest.raises(WorkflowError, match=r"must be a non-empty list"):
        validate_workflow(workflow)


def test_required_review_postures_rejects_unknown_entry() -> None:
    workflow = _base_workflow_with_build_then_review()
    workflow["jobs"][0]["required_review_postures"] = ["telepathy"]
    with pytest.raises(WorkflowError, match=r"contains invalid entry 'telepathy'"):
        validate_workflow(workflow)


def test_required_review_postures_rejects_non_list() -> None:
    workflow = _base_workflow_with_build_then_review()
    workflow["jobs"][0]["required_review_postures"] = "security"
    with pytest.raises(WorkflowError, match=r"must be a non-empty list"):
        validate_workflow(workflow)


def test_required_review_postures_accepts_first_class_entries() -> None:
    workflow = _base_workflow_with_build_then_review()
    workflow["jobs"][0]["required_review_postures"] = ["security", "threat_model"]
    workflow["jobs"][1]["review_posture"] = "security"
    # Need a second review for threat_model.
    workflow["roles"]["reviewer2"] = {"definition_path": "roles/rev2.md"}
    workflow["jobs"].append({
        "id": "review_threat",
        "type": "review",
        "title": "Threat review",
        "role_id": "reviewer2",
        "lane_id": "lane_a",
        "fresh_session_required": True,
        "review_posture": "threat_model",
        "objective": "Threat",
        "task_prompt": {"path": "prompts/threat.md"},
        "write_scope": {
            "mode": "review_only_artifact",
            "repo_write": False,
            "allowed_paths": ["docs/reviews/threat/"],
            "forbidden_paths": [".striatum/"],
        },
        "expected_artifacts": [
            {"logical_name": "threat", "kind": "finding",
             "path": "docs/reviews/threat/REVIEW.md", "required": True},
        ],
    })
    workflow["edges"].append({"from": "build_step", "to": "review_threat", "on": "completed"})
    validate_workflow(workflow)


def test_required_review_postures_accepts_custom_entries() -> None:
    workflow = _base_workflow_with_build_then_review()
    workflow["jobs"][0]["required_review_postures"] = ["custom:my_thing"]
    workflow["jobs"][1]["review_posture"] = "custom:my_thing"
    validate_workflow(workflow)


# --- Step 2: reachability gate ----------------------------------------


def test_workflow_validates_when_required_posture_is_reachable_via_forward_edge() -> None:
    """build → review (forward edge); review_posture matches required."""
    workflow = _base_workflow_with_build_then_review()
    workflow["jobs"][0]["required_review_postures"] = ["security"]
    workflow["jobs"][1]["review_posture"] = "security"
    validate_workflow(workflow)


def test_workflow_validates_when_required_posture_is_reachable_via_reverse_edge() -> None:
    """review → build (reverse edge — design-review-before-build pattern)."""
    workflow = _base_workflow_with_build_then_review()
    # Flip the edge: review → build instead of build → review.
    workflow["edges"] = [{"from": "review_step", "to": "build_step", "on": "completed"}]
    workflow["jobs"][0]["required_review_postures"] = ["security"]
    workflow["jobs"][1]["review_posture"] = "security"
    validate_workflow(workflow)


def test_workflow_rejects_unreachable_required_posture() -> None:
    """The build declares a required posture that no reachable review carries."""
    workflow = _base_workflow_with_build_then_review()
    workflow["jobs"][0]["required_review_postures"] = ["security"]
    workflow["jobs"][1]["review_posture"] = "ergonomics_dx"
    with pytest.raises(
        WorkflowError,
        match=r"build job 'build_step' requires review posture 'security' "
        r"but no reachable review job declares it",
    ):
        validate_workflow(workflow)


def test_workflow_rejects_when_review_with_required_posture_exists_but_disconnected() -> None:
    """The right-posture review exists but no edge connects it to the build."""
    workflow = _base_workflow_with_build_then_review()
    workflow["jobs"][0]["required_review_postures"] = ["security"]
    workflow["jobs"][1]["review_posture"] = "security"
    # Drop the connecting edge so the review is graph-disconnected.
    workflow["edges"] = []
    with pytest.raises(WorkflowError, match=r"no reachable review job declares it"):
        validate_workflow(workflow)


# --- Zero-regression contract -----------------------------------------


def test_zero_regression_for_posture_omitting_workflow_validator() -> None:
    """A workflow that declares neither field validates exactly as it did pre-V1."""
    workflow = _base_workflow_with_build_then_review()
    # Neither field declared; should validate without complaint.
    validate_workflow(workflow)


def test_zero_regression_for_posture_omitting_workflow_packet(tmp_path: Path) -> None:
    """A review job declaring neither posture nor access-scope produces no review_policy block."""
    workflow = _base_workflow_with_review_only()
    # No posture, no reviewer_access_scope, no reviewer_context_policy.
    workflow_path = temporary_workflow(tmp_path, workflow)
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(workflow_path)))
    run_id = str(prepared["run_id"])
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "wf/zero-reg")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)
    sess = data(run_cli(
        tmp_path, "register-session",
        "--run-id", run_id, "--role", "reviewer", "--lane", "lane_a", "--fresh",
    ))
    claim = data(run_cli(tmp_path, "claim-next", "--session-id", str(sess["session_id"])))
    packet = claim["packet"]
    # Pre-V1: undeclared review_policy means no block at all.
    assert packet.get("review_policy") is None
