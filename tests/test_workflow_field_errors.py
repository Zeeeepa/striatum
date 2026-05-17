"""RFC 0024 V2: validate_workflow yields field_path on tagged raise sites."""

from __future__ import annotations

import json
from typing import Any

import pytest

from striatum.errors import WorkflowError
from striatum.workflow import validate_workflow


_VALID = {
    "schema_version": "striatum.workflow.v1",
    "workflow_id": "wf-test",
    "workflow_version": "1",
    "name": "T",
    "branch": {"mode": "confirm", "suggested_name": "wf/t", "allow_dirty": False},
    "coordinator": {"role_id": "reviewer", "lane_id": "lane_a"},
    "lanes": {
        "lane_a": {
            "adapter": "process", "display_model": "X",
            "command": ["echo"], "capabilities": ["review"],
        },
    },
    "roles": {"reviewer": {"definition_path": "p.md"}},
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


def _copy() -> dict[str, Any]:
    return json.loads(json.dumps(_VALID))  # type: ignore[no-any-return]


def test_workflowerror_carries_optional_field_path() -> None:
    err = WorkflowError("oops", field_path="jobs[0].role_id")
    assert err.field_path == "jobs[0].role_id"
    assert str(err) == "oops"


def test_workflowerror_default_field_path_is_none() -> None:
    err = WorkflowError("oops")
    assert err.field_path is None


def test_unknown_role_yields_jobs_index_role_id() -> None:
    bad = _copy()
    bad["jobs"][0]["role_id"] = "ghost"
    with pytest.raises(WorkflowError) as exc_info:
        validate_workflow(bad)
    assert exc_info.value.field_path == "jobs[0].role_id"


def test_unknown_lane_yields_jobs_index_lane_id() -> None:
    bad = _copy()
    bad["jobs"][0]["lane_id"] = "ghost_lane"
    with pytest.raises(WorkflowError) as exc_info:
        validate_workflow(bad)
    assert exc_info.value.field_path == "jobs[0].lane_id"


def test_duplicate_job_id_yields_jobs_index_id() -> None:
    bad = _copy()
    second = json.loads(json.dumps(bad["jobs"][0]))
    bad["jobs"].append(second)
    with pytest.raises(WorkflowError) as exc_info:
        validate_workflow(bad)
    assert exc_info.value.field_path == "jobs[1].id"


def test_invalid_artifact_path_yields_artifact_index() -> None:
    bad = _copy()
    bad["jobs"][0]["expected_artifacts"][0]["path"] = "/abs/path"
    with pytest.raises(WorkflowError) as exc_info:
        validate_workflow(bad)
    assert exc_info.value.field_path == "jobs[0].expected_artifacts[0].path"


def test_bad_schema_version_yields_top_level_path() -> None:
    bad = _copy()
    bad["schema_version"] = "wrong"
    with pytest.raises(WorkflowError) as exc_info:
        validate_workflow(bad)
    assert exc_info.value.field_path == "schema_version"


def test_unconverted_raise_site_keeps_none() -> None:
    """V1.5 backward-compat: untagged raise sites still raise without field_path."""
    bad = _copy()
    del bad["coordinator"]
    with pytest.raises(WorkflowError) as exc_info:
        validate_workflow(bad)
    # Missing top-level fields raise site is not tagged in V2.
    assert exc_info.value.field_path is None


def test_valid_workflow_does_not_raise() -> None:
    validate_workflow(_VALID)


def test_process_lane_supervision_stdin_delivery_is_closed() -> None:
    workflow = _copy()
    lane = workflow["lanes"]["lane_a"]
    lane["supervision"] = {"stdin_delivery": "one_shot_eof"}
    validate_workflow(workflow)

    bad = _copy()
    bad["lanes"]["lane_a"]["supervision"] = {"stdin_delivery": "stream_until_magic"}
    with pytest.raises(WorkflowError, match="supervision.stdin_delivery"):
        validate_workflow(bad)

    bad = _copy()
    bad["lanes"]["lane_a"]["supervision"] = {
        "transport": "pty_helper",
        "stdin_delivery": "one_shot_eof",
    }
    with pytest.raises(WorkflowError, match="requires supervision.transport 'pipe'"):
        validate_workflow(bad)


def test_require_daemon_must_be_boolean() -> None:
    bad = _copy()
    bad["require_daemon"] = "yes"
    with pytest.raises(WorkflowError) as exc_info:
        validate_workflow(bad)
    assert exc_info.value.field_path == "require_daemon"


def test_sealed_patch_rejects_explicit_require_daemon_false() -> None:
    bad = _copy()
    bad["provenance_mode"] = "sealed_patch"
    bad["protected_paths"] = ["src/"]
    bad["operator_writable_paths"] = ["docs/"]
    bad["require_daemon"] = False
    with pytest.raises(WorkflowError) as exc_info:
        validate_workflow(bad)
    assert exc_info.value.field_path == "require_daemon"


def test_sealed_patch_provider_is_closed_and_sealed_only() -> None:
    bad = _copy()
    bad["provenance_mode"] = "sealed_patch"
    bad["protected_paths"] = ["src/"]
    bad["operator_writable_paths"] = ["docs/"]
    bad["sealed_patch_provider"] = "magic"
    with pytest.raises(WorkflowError) as exc_info:
        validate_workflow(bad)
    assert exc_info.value.field_path == "sealed_patch_provider"

    bad = _copy()
    bad["sealed_patch_provider"] = "refuse"
    with pytest.raises(WorkflowError) as exc_info:
        validate_workflow(bad)
    assert exc_info.value.field_path == "sealed_patch_provider"


def test_apply_gate_field_validation() -> None:
    bad = _copy()
    bad["jobs"][0]["apply_gate"] = "true"
    with pytest.raises(WorkflowError) as exc_info:
        validate_workflow(bad)
    assert exc_info.value.field_path == "jobs[0].apply_gate"

    bad = _copy()
    bad["jobs"][0]["apply_gate"] = True
    with pytest.raises(WorkflowError) as exc_info:
        validate_workflow(bad)
    assert exc_info.value.field_path == "jobs[0].apply_gate"


def test_apply_gate_accepts_build_with_patch_summary_artifact() -> None:
    workflow = _copy()
    job = workflow["jobs"][0]
    job["type"] = "build"
    job["write_scope"] = {
        "mode": "repo_write",
        "repo_write": True,
        "allowed_paths": ["docs/reviews/"],
        "forbidden_paths": [".striatum/"],
    }
    job["expected_artifacts"] = [
        {
            "logical_name": "patch_summary",
            "kind": "patch_summary",
            "path": "docs/reviews/PATCH_SUMMARY.md",
            "required": True,
        },
    ]
    job["apply_gate"] = True

    validate_workflow(workflow)
