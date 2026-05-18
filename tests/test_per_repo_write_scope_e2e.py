from __future__ import annotations

import pytest

from striatum.errors import WorkflowError
from striatum.repo_policy import path_allowed
from striatum.workflow import validate_workflow
from _harness.multi_repo import MultiRepoHarness, two_repo_workflow

pytestmark = pytest.mark.multi_repo


def test_repo_b_job_publishes_artifact_inside_repo_b_scope(multi_repo_harness: MultiRepoHarness) -> None:
    repo_b = multi_repo_harness.repos[1].path
    target = repo_b / "docs" / "consumer.md"
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text("author: implementer-codex-gpt-5.5-001\n\n# Consumer\n", encoding="utf-8")

    assert path_allowed(
        repo_b,
        "docs/consumer.md",
        {"allowed_paths": ["docs/"], "forbidden_paths": [".striatum/"]},
    )


def test_expected_artifact_escape_into_repo_a_rejected_before_prepare(
    multi_repo_harness: MultiRepoHarness,
) -> None:
    workflow = two_repo_workflow(multi_repo_harness)
    jobs = workflow["jobs"]
    assert isinstance(jobs, list)
    consumer_job = jobs[1]
    assert isinstance(consumer_job, dict)
    consumer_job["expected_artifacts"] = [
        {"logical_name": "out", "kind": "handoff", "path": "../repo-0/docs/escape.md"}
    ]

    with pytest.raises(WorkflowError, match="invalid artifact path|outside"):
        validate_workflow(workflow)


def test_runtime_publish_from_repo_b_into_repo_a_refused(multi_repo_harness: MultiRepoHarness) -> None:
    repo_b = multi_repo_harness.repos[1].path

    with pytest.raises(Exception, match="stay inside the repository"):
        path_allowed(
            repo_b,
            "../repo-0/docs/escape.md",
            {"allowed_paths": ["docs/"], "forbidden_paths": [".striatum/"]},
        )


def test_job_targeting_undeclared_repository_alias_rejected_by_validator(
    multi_repo_harness: MultiRepoHarness,
) -> None:
    workflow = two_repo_workflow(multi_repo_harness)
    jobs = workflow["jobs"]
    assert isinstance(jobs, list)
    assert isinstance(jobs[0], dict)
    jobs[0]["repository"] = "missing"

    with pytest.raises(WorkflowError, match="unknown repository alias"):
        validate_workflow(workflow)
