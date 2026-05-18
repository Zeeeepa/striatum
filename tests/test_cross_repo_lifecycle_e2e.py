from __future__ import annotations

import pytest

from _harness.multi_repo import MultiRepoHarness, cycle_workflow, two_repo_workflow

pytestmark = pytest.mark.multi_repo


def test_prepare_start_summary_cancel_two_repo_run(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    result = harness.prepare_cross_repo_run(two_repo_workflow(harness))
    xrun = str(result["cross_repo_run_id"])

    started = harness.start_cross_repo_run(xrun)
    described = harness.describe_cross_repo_run(xrun)
    canceled = harness.cancel_cross_repo_run(xrun)

    assert started["state"] == "running"
    assert described["state"] == "running"
    assert {row["state"] for row in described["participants"]} == {"running"}
    assert canceled["state"] == "canceled"
    for index in range(2):
        assert harness.participant_runs(index)[0]["state"] == "canceled"


def test_mixed_participant_state_is_reflected_in_describe_and_dashboard(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    result = harness.prepare_cross_repo_run(two_repo_workflow(harness))
    xrun = str(result["cross_repo_run_id"])
    harness.start_cross_repo_run(xrun)
    consumer = next(row for row in harness.describe_cross_repo_run(xrun)["participants"] if row["repository_alias"] == "consumer")
    harness.daemon_db_query(
        "UPDATE striatumd.cross_repo_run_repositories SET state = 'completed' WHERE cross_repo_run_id = %s AND repository_alias = 'consumer'",
        (xrun,),
    )

    described = harness.describe_cross_repo_run(xrun)

    states = {row["repository_alias"]: row["state"] for row in described["participants"]}
    assert states["consumer"] == "completed"
    assert states["primary"] == "running"
    assert consumer["repository_id"] in {row["repository_id"] for row in described["participants"]}


def test_cancel_mid_run_cascades_or_blocks_reachable_participants(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    result = harness.prepare_cross_repo_run(two_repo_workflow(harness))
    xrun = str(result["cross_repo_run_id"])
    harness.start_cross_repo_run(xrun)
    harness._runner().fail_cancel_alias = "consumer"

    canceled = harness.cancel_cross_repo_run(xrun)

    assert canceled["state"] == "blocked"
    assert canceled["blocked"] == ["consumer"]
    states = {row["repository_alias"]: row["state"] for row in harness.describe_cross_repo_run(xrun)["participants"]}
    assert states == {"consumer": "blocked", "primary": "canceled"}


def test_cross_repo_dashboard_includes_all_participants(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    xrun = str(harness.prepare_cross_repo_run(two_repo_workflow(harness))["cross_repo_run_id"])

    described = harness.describe_cross_repo_run(xrun)

    assert {row["repository_id"] for row in described["participants"]} == {
        harness.repos[0].repository_id,
        harness.repos[1].repository_id,
    }


def test_cross_repo_cycle_counter_increments_globally_once(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    workflow = cycle_workflow(harness)
    xrun = str(harness.prepare_cross_repo_run(workflow)["cross_repo_run_id"])

    harness.daemon_db_query(
        """
        INSERT INTO striatumd.cross_repo_cycle_counters(
          cross_repo_run_id, cycle_key, iterations, max_iterations
        )
        VALUES (%s, 'draft_consumer->draft_primary', 0, 2)
        """,
        (xrun,),
    )
    harness.daemon_db_query(
        """
        UPDATE striatumd.cross_repo_cycle_counters
        SET iterations = iterations + 1
        WHERE cross_repo_run_id = %s AND cycle_key = 'draft_consumer->draft_primary'
        """,
        (xrun,),
    )

    rows = harness.daemon_db_query("SELECT iterations FROM striatumd.cross_repo_cycle_counters WHERE cross_repo_run_id = %s", (xrun,))
    assert rows == [{"iterations": 1}]
