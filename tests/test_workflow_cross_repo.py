from __future__ import annotations

from typing import Any

import pytest

from striatum.errors import WorkflowError
from striatum.workflow import validate_workflow


def _workflow() -> dict[str, Any]:
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "cross-repo-test",
        "workflow_version": "1",
        "name": "Cross repo test",
        "branch": {"mode": "auto", "suggested_name": "test/cross-repo"},
        "coordinator": {"role_id": "coordinator", "lane_id": "codex"},
        "repositories": {
            "primary": {"repo_id": "repo_primary"},
            "consumer": {"repo_id": "repo_consumer"},
        },
        "primary_repository": "primary",
        "lanes": {"codex": {"adapter": "manual"}},
        "roles": {
            "coordinator": {},
            "author": {},
            "reviewer": {},
        },
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 2,
            "require_disjoint_write_scopes": True,
            "per_repo_max_active_jobs": {"primary": 1, "consumer": 1},
        },
        "jobs": [
            {
                "id": "draft",
                "type": "draft",
                "repository": "primary",
                "role_id": "author",
                "lane_id": "codex",
                "parallel_group": "both",
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": ["docs/"],
                    "forbidden_paths": [],
                },
                "expected_artifacts": [{"logical_name": "out", "kind": "handoff", "path": "docs/out.md"}],
            },
            {
                "id": "consumer",
                "type": "draft",
                "repository": "consumer",
                "role_id": "author",
                "lane_id": "codex",
                "parallel_group": "both",
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": ["docs/"],
                    "forbidden_paths": [],
                },
                "expected_artifacts": [{"logical_name": "out", "kind": "handoff", "path": "docs/out.md"}],
            },
            {
                "id": "review",
                "type": "review",
                "repository": "consumer",
                "role_id": "reviewer",
                "lane_id": "codex",
                "reviewer_access_scope": "cross_repo_artifact_augmented",
                "write_scope": {
                    "mode": "review_only",
                    "repo_write": False,
                    "allowed_paths": ["docs/review/"],
                    "forbidden_paths": [],
                },
                "expected_artifacts": [{"logical_name": "review", "kind": "finding", "path": "docs/review/finding.md"}],
            },
        ],
        "edges": [
            {"from": "draft", "to": "review", "on": "completed"},
            {"from": "consumer", "to": "review", "on": "completed"},
        ],
        "cycles": [],
    }


def test_cross_repo_workflow_validates_with_repository_qualified_paths() -> None:
    validate_workflow(_workflow())


def test_cross_repo_requires_primary_repository_and_job_repository() -> None:
    workflow = _workflow()
    workflow.pop("primary_repository")
    with pytest.raises(WorkflowError, match="primary_repository"):
        validate_workflow(workflow)

    workflow = _workflow()
    jobs = workflow["jobs"]
    assert isinstance(jobs, list)
    assert isinstance(jobs[0], dict)
    jobs[0].pop("repository")
    with pytest.raises(WorkflowError, match="must declare repository"):
        validate_workflow(workflow)


def test_single_repo_workflow_rejects_job_repository() -> None:
    workflow = _workflow()
    workflow.pop("repositories")
    workflow.pop("primary_repository")
    with pytest.raises(WorkflowError, match="single-repo workflows must not declare job repository"):
        validate_workflow(workflow)


def test_cross_repo_rejects_duplicate_repo_ids_and_unknown_aliases() -> None:
    workflow = _workflow()
    repos = workflow["repositories"]
    assert isinstance(repos, dict)
    repos["consumer"] = {"repo_id": "repo_primary"}
    with pytest.raises(WorkflowError, match="share repo_id"):
        validate_workflow(workflow)

    workflow = _workflow()
    parallelism = workflow["parallelism"]
    assert isinstance(parallelism, dict)
    parallelism["per_repo_max_active_jobs"] = {"missing": 1}
    with pytest.raises(WorkflowError, match="unknown repository alias"):
        validate_workflow(workflow)


def test_cross_repo_cycle_requires_explicit_flag() -> None:
    workflow = _workflow()
    workflow["edges"] = [{"from": "draft", "to": "consumer", "on": "completed"}]
    workflow["cycles"] = [{"from": "consumer", "to": "draft", "on_verdict": "needs_revision", "max_iterations": 1}]
    with pytest.raises(WorkflowError, match="cross_repo_cycle=true"):
        validate_workflow(workflow)

    workflow["cycles"] = [
        {
            "from": "consumer",
            "to": "draft",
            "on_verdict": "needs_revision",
            "max_iterations": 1,
            "cross_repo_cycle": True,
        }
    ]
    validate_workflow(workflow)


def test_cross_repo_access_scope_rejected_for_single_repo_workflow() -> None:
    workflow = _workflow()
    workflow.pop("repositories")
    workflow.pop("primary_repository")
    jobs = workflow["jobs"]
    assert isinstance(jobs, list)
    for job in jobs:
        assert isinstance(job, dict)
        job.pop("repository", None)
    with pytest.raises(WorkflowError, match="only in cross-repo workflows"):
        validate_workflow(workflow)
