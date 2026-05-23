from __future__ import annotations

import copy
from typing import Any

import pytest

from striatum.errors import WorkflowError
from striatum.repo_policy import adapter_constraint_enforcement
from striatum.workflow import validate_workflow


def _workflow_with_lane(lane: dict[str, Any]) -> dict[str, Any]:
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "adapter-constraints-test",
        "workflow_version": "2026-05-23",
        "name": "Adapter constraints test",
        "branch": {"mode": "confirm", "suggested_name": "striatum/adapter-constraints"},
        "coordinator": {"role_id": "author", "lane_id": "local"},
        "lanes": {"local": lane},
        "roles": {"author": {"description": "Author"}},
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 1,
            "require_disjoint_write_scopes": True,
        },
        "jobs": [
            {
                "id": "draft",
                "type": "build",
                "title": "Draft",
                "role_id": "author",
                "lane_id": "local",
                "objective": "Draft artifact.",
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": ["docs/out/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "out",
                        "kind": "handoff",
                        "path": "docs/out/OUT.md",
                        "required": True,
                    }
                ],
            }
        ],
        "edges": [],
        "cycles": [],
    }


def test_process_adapter_constraint_matrix_is_honest_about_current_scope() -> None:
    assert (
        adapter_constraint_enforcement(
            "process", constraint="network", requested="forbidden"
        )
        == "advisory_strict"
    )
    assert (
        adapter_constraint_enforcement(
            "process", constraint="repo_scope", requested="local_only"
        )
        == "advisory_strict"
    )
    assert (
        adapter_constraint_enforcement(
            "process", constraint="transcripts", requested="off"
        )
        == "enforced"
    )

    validate_workflow(
        _workflow_with_lane(
            {
                "adapter": "process",
                "command": ["echo", "ok"],
                "constraints": {
                    "network": "forbidden",
                    "repo_scope": "local_only",
                    "transcripts": "off",
                },
                "required_enforcement": {
                    "network": "advisory_strict",
                    "repo_scope": "advisory_strict",
                    "transcripts": "enforced",
                },
            }
        )
    )


def test_worktree_isolation_does_not_claim_enforced_repo_scope() -> None:
    workflow = _workflow_with_lane(
        {
            "adapter": "process",
            "command": ["echo", "ok"],
            "worktree_isolation": "per_job",
            "constraints": {"repo_scope": "local_only"},
            "required_enforcement": {"repo_scope": "enforced"},
        }
    )

    with pytest.raises(WorkflowError, match="requires 'enforced' enforcement"):
        validate_workflow(workflow)

    relaxed = copy.deepcopy(workflow)
    relaxed["lanes"]["local"]["required_enforcement"]["repo_scope"] = "advisory_strict"
    validate_workflow(relaxed)
