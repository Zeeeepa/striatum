from __future__ import annotations

import pytest

from _harness.multi_repo import MultiRepoHarness, two_repo_workflow

pytestmark = pytest.mark.multi_repo


def test_restart_after_crash_mid_prepare_aborts_or_completes_without_orphans(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    xrun = "xrun_mid_prepare"
    harness.daemon_db_query(
        """
        INSERT INTO striatumd.cross_repo_runs(
          cross_repo_run_id, workflow_id, workflow_version, workflow_snapshot_hash,
          primary_repository_id, state, created_at, reconcile_after
        )
        VALUES (%s, 'wf', '1', repeat('0', 64), %s, 'preparing', now(), now())
        """,
        (xrun, harness.repos[0].repository_id),
    )
    for alias, repo in (("primary", harness.repos[0]), ("consumer", harness.repos[1])):
        harness.daemon_db_query(
            """
            INSERT INTO striatumd.cross_repo_run_repositories(
              cross_repo_run_id, repository_alias, repository_id, state, last_observed_at
            )
            VALUES (%s, %s, %s, 'preparing', now())
            """,
            (xrun, alias, repo.repository_id),
        )

    harness.kill_daemon()
    harness.restart_daemon()
    reconciled = harness.reconcile_preparing()

    assert reconciled == {"prepared": [], "aborted": [xrun]}
    assert harness.describe_cross_repo_run(xrun)["state"] == "aborted"


def test_restart_after_crash_mid_start_completes_or_blocks_structured(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    xrun = str(harness.prepare_cross_repo_run(two_repo_workflow(harness))["cross_repo_run_id"])
    harness._runner().fail_start_alias = "consumer"

    harness.kill_daemon()
    harness.restart_daemon()
    result = harness.start_cross_repo_run(xrun)

    assert result["state"] == "blocked"
    assert "primary" in result["started"]
    assert harness._runner().checkpoints


def test_restart_after_crash_mid_cancel_finishes_or_blocks(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    xrun = str(harness.prepare_cross_repo_run(two_repo_workflow(harness))["cross_repo_run_id"])
    harness.start_cross_repo_run(xrun)
    harness._runner().fail_cancel_alias = "consumer"

    harness.kill_daemon()
    harness.restart_daemon()
    result = harness.cancel_cross_repo_run(xrun)

    assert result["state"] == "blocked"
    assert result["blocked"] == ["consumer"]


def test_unreachable_participant_mid_run_creates_primary_checkpoint(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    xrun = str(harness.prepare_cross_repo_run(two_repo_workflow(harness))["cross_repo_run_id"])

    with harness.simulate_repo_unreachable(1):
        result = harness.start_cross_repo_run(xrun)

    assert result["state"] == "blocked"
    assert harness._runner().checkpoints
    rows = harness.repo_sqlite_query(0, "SELECT severity, blocker_kind FROM blockers")
    assert rows == [{"severity": "human_checkpoint", "blocker_kind": "human_checkpoint"}]
