from __future__ import annotations

import pytest

from striatum.errors import WorkflowError
from striatum.workflow import validate_workflow
from _harness.multi_repo import MultiRepoHarness, two_repo_workflow

pytestmark = pytest.mark.multi_repo


def test_prepare_valid_two_repo_workflow_creates_daemon_and_local_rows(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    workflow = two_repo_workflow(harness)

    result = harness.prepare_cross_repo_run(workflow)

    xrun = str(result["cross_repo_run_id"])
    assert harness.daemon_db_query("SELECT state FROM striatumd.cross_repo_runs WHERE cross_repo_run_id = %s", (xrun,))[0]["state"] == "prepared"
    assert len(harness.daemon_db_query("SELECT * FROM striatumd.cross_repo_run_repositories WHERE cross_repo_run_id = %s", (xrun,))) == 2
    for index in range(2):
        rows = [
            {"cross_repo_run_id": row["cross_repo_run_id"]}
            for row in harness.participant_runs(index)
        ]
        assert rows == [{"cross_repo_run_id": xrun}]


def test_prepare_unknown_repository_id_refuses_without_writes(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    workflow = two_repo_workflow(harness)
    repositories = workflow["repositories"]
    assert isinstance(repositories, dict)
    repositories["consumer"] = {"repo_id": "repo_missing"}

    with pytest.raises(Exception):
        harness.prepare_cross_repo_run(workflow)

    assert harness.daemon_db_query("SELECT state FROM striatumd.cross_repo_runs") == []
    assert harness.participant_runs(0) == []
    assert harness.participant_runs(1) == []


def test_prepare_repo_local_failure_aborts_without_false_prepared(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    workflow = two_repo_workflow(harness)
    harness._runner(workflow).fail_prepare_alias = "consumer"

    with pytest.raises(RuntimeError):
        harness.prepare_cross_repo_run(workflow)

    rows = harness.daemon_db_query("SELECT state, last_reconcile_error FROM striatumd.cross_repo_runs")
    assert rows[0]["state"] == "aborted"
    assert "consumer" in str(rows[0]["last_reconcile_error"])
    assert not any(row["state"] == "prepared" for row in rows)


def test_validate_catches_shape_errors_before_prepare(multi_repo_harness: MultiRepoHarness) -> None:
    workflow = two_repo_workflow(multi_repo_harness)
    jobs = workflow["jobs"]
    assert isinstance(jobs, list)

    broken = dict(workflow)
    broken.pop("primary_repository")
    with pytest.raises(WorkflowError, match="primary_repository"):
        validate_workflow(broken)

    broken = dict(workflow)
    broken["repositories"] = {"primary": {"repo_id": "same"}, "consumer": {"repo_id": "same"}}
    with pytest.raises(WorkflowError, match="share repo_id"):
        validate_workflow(broken)

    broken = dict(workflow)
    copied_jobs = [dict(job) for job in jobs]
    copied_jobs[0]["repository"] = "missing"
    broken["jobs"] = copied_jobs
    with pytest.raises(WorkflowError, match="unknown repository alias"):
        validate_workflow(broken)

    broken = dict(workflow)
    copied_jobs = [dict(job) for job in jobs]
    copied_jobs[0].pop("repository")
    broken["jobs"] = copied_jobs
    with pytest.raises(WorkflowError, match="must declare repository"):
        validate_workflow(broken)
