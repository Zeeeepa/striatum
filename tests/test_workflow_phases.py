from __future__ import annotations

from typing import Any

import pytest

from striatum.errors import WorkflowError
from striatum.workflow import (
    edge_dependency_pairs,
    validate_workflow,
    workflow_phase_index,
)


def _job(job_id: str, phase_id: str, job_type: str = "handoff") -> dict[str, Any]:
    return {
        "id": job_id,
        "type": job_type,
        "title": job_id,
        "role_id": "worker",
        "lane_id": "lane_a",
        "phase_id": phase_id,
        "objective": job_id,
        "task_prompt": {"path": "prompts/work.md"},
        "write_scope": {
            "mode": "repo_write",
            "repo_write": True,
            "allowed_paths": [f"docs/{job_id}/"],
            "forbidden_paths": [".striatum/"],
        },
        "expected_artifacts": [],
    }


def _workflow() -> dict[str, Any]:
    return {
        "schema_version": "striatum.workflow.v1.1",
        "workflow_id": "wf-phases",
        "workflow_version": "1",
        "name": "Phases",
        "branch": {"mode": "confirm", "suggested_name": "wf/phases"},
        "coordinator": {"role_id": "worker", "lane_id": "lane_a"},
        "lanes": {
            "lane_a": {
                "adapter": "process",
                "display_model": "test",
                "command": ["echo"],
                "capabilities": ["workflow"],
            }
        },
        "roles": {"worker": {"definition_path": "roles/worker.md"}},
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 2,
            "require_disjoint_write_scopes": True,
        },
        "phases": [
            {"id": "phase_1_design", "name": "Design"},
            {"id": "phase_2_build", "name": "Build"},
        ],
        "jobs": [
            _job("design_a", "phase_1_design"),
            _job("synthesize_design", "phase_1_design", "phase_synthesis"),
            _job("build_a", "phase_2_build"),
            _job("synthesize_build", "phase_2_build", "phase_synthesis"),
        ],
        "edges": [
            {"from": "synthesize_design", "to": "build_a", "on": "completed"},
        ],
        "cycles": [],
    }


def test_v1_1_phases_validate_and_materialize_synthesis_fan_in() -> None:
    workflow = _workflow()

    validate_workflow(workflow)

    phase_index = workflow_phase_index(workflow)
    assert phase_index["phase_order"] == ["phase_1_design", "phase_2_build"]
    assert phase_index["synthesis_by_phase"] == {
        "phase_1_design": "synthesize_design",
        "phase_2_build": "synthesize_build",
    }
    assert edge_dependency_pairs(workflow, include_phase_materialized=False) == [
        (
            "synthesize_design",
            "build_a",
            {"on": "completed", "from": "synthesize_design", "to": "build_a"},
        )
    ]
    materialized = {
        (from_id, to_id)
        for from_id, to_id, _gate in edge_dependency_pairs(workflow)
    }
    assert materialized == {
        ("design_a", "synthesize_design"),
        ("synthesize_design", "build_a"),
        ("build_a", "synthesize_build"),
    }


def test_v1_1_accepts_canonical_phase_field_from_editor() -> None:
    workflow = _workflow()
    for job in workflow["jobs"]:
        job["phase"] = job.pop("phase_id")

    validate_workflow(workflow)

    phase_index = workflow_phase_index(workflow)
    assert phase_index["job_phase"] == {
        "design_a": "phase_1_design",
        "synthesize_design": "phase_1_design",
        "build_a": "phase_2_build",
        "synthesize_build": "phase_2_build",
    }


def test_v1_rejects_phase_fields() -> None:
    workflow = _workflow()
    workflow["schema_version"] = "striatum.workflow.v1"

    with pytest.raises(WorkflowError, match="must not declare phases"):
        validate_workflow(workflow)

    workflow = _workflow()
    workflow["schema_version"] = "striatum.workflow.v1"
    del workflow["phases"]

    with pytest.raises(WorkflowError, match="must not declare phase_id"):
        validate_workflow(workflow)

    workflow = _workflow()
    workflow["schema_version"] = "striatum.workflow.v1"
    del workflow["phases"]
    for job in workflow["jobs"]:
        job["phase"] = job.pop("phase_id")

    with pytest.raises(WorkflowError, match="must not declare phase"):
        validate_workflow(workflow)


def test_declared_phases_require_every_job_to_have_known_phase_id() -> None:
    workflow = _workflow()
    del workflow["jobs"][2]["phase_id"]

    with pytest.raises(WorkflowError, match="must declare phase"):
        validate_workflow(workflow)

    workflow = _workflow()
    workflow["jobs"][2]["phase_id"] = "missing"

    with pytest.raises(WorkflowError, match="references unknown phase"):
        validate_workflow(workflow)


def test_declared_phases_reject_conflicting_phase_aliases() -> None:
    workflow = _workflow()
    workflow["jobs"][0]["phase"] = "phase_2_build"

    with pytest.raises(WorkflowError, match="conflicting phase and phase_id"):
        validate_workflow(workflow)


def test_each_phase_requires_exactly_one_synthesis_job() -> None:
    workflow = _workflow()
    workflow["jobs"][1]["type"] = "handoff"

    with pytest.raises(WorkflowError, match="exactly one phase_synthesis"):
        validate_workflow(workflow)

    workflow = _workflow()
    workflow["jobs"].append(_job("synthesize_design_2", "phase_1_design", "phase_synthesis"))

    with pytest.raises(WorkflowError, match="multiple phase_synthesis"):
        validate_workflow(workflow)


def test_phase_synthesis_rejects_review_only_fields() -> None:
    workflow = _workflow()
    workflow["jobs"][1]["reviewer_access_scope"] = "document_only"

    with pytest.raises(WorkflowError, match="non-review job"):
        validate_workflow(workflow)


def test_cross_phase_edges_must_use_source_synthesis_to_next_phase() -> None:
    workflow = _workflow()
    workflow["edges"] = [{"from": "design_a", "to": "build_a", "on": "completed"}]

    with pytest.raises(WorkflowError, match="crosses phases without"):
        validate_workflow(workflow)

    workflow = _workflow()
    workflow["edges"] = [
        {"from": "synthesize_design", "to": "synthesize_build", "on": "completed"}
    ]

    with pytest.raises(WorkflowError, match="cannot target a later phase_synthesis"):
        validate_workflow(workflow)

    workflow = _workflow()
    workflow["edges"] = [{"from": "build_a", "to": "design_a", "on": "completed"}]

    with pytest.raises(WorkflowError, match="later phase"):
        validate_workflow(workflow)


def test_cross_phase_edges_cannot_skip_phases() -> None:
    workflow = _workflow()
    workflow["phases"].append({"id": "phase_3_ship", "name": "Ship"})
    workflow["jobs"].extend(
        [
            _job("ship_a", "phase_3_ship"),
            _job("synthesize_ship", "phase_3_ship", "phase_synthesis"),
        ]
    )
    workflow["edges"] = [
        {"from": "synthesize_design", "to": "ship_a", "on": "completed"}
    ]

    with pytest.raises(WorkflowError, match="skips phases"):
        validate_workflow(workflow)


def test_revision_cycles_must_stay_within_one_phase() -> None:
    workflow = _workflow()
    workflow["cycles"] = [
        {
            "from": "synthesize_build",
            "to": "design_a",
            "on_verdict": "needs_revision",
            "max_iterations": 1,
        }
    ]

    with pytest.raises(WorkflowError, match="revision cycles must stay within"):
        validate_workflow(workflow)
