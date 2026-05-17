from __future__ import annotations

import pytest

from _harness.multi_repo import MultiRepoHarness

pytestmark = pytest.mark.multi_repo


def test_mcp_publish_on_behalf_is_retired_before_sqlite_composite(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["write"], repo_id=str(harness.repos[0].repository_id))

    result = harness.mcp_client(token, repo_index=0).call_tool(
        "dogfood.publish_on_behalf",
        repository_id=str(harness.repos[0].repository_id),
        arguments={
            "session_id": "sess_1",
            "artifact_path": "docs/out/OUT.md",
            "artifact_kind": "handoff",
            "logical_name": "out",
            "reason": "agent denied ack from supervised wrapper",
        },
    )

    assert result["isError"] is True
    structured = result["structuredContent"]
    assert structured["error"] == "dogfood_publish_on_behalf_retired"
    assert structured["error_codes"] == ["dogfood_publish_on_behalf_retired"]
    data = structured["data"]
    assert data["code"] == "dogfood_publish_on_behalf_retired"
    assert data["details"]["blocker"] == "legacy_python_composite_uses_repo_local_sqlite_connection"
    row = harness.audit_rows(transport="mcp")[-1]
    assert row["decision"] == "allowed"
    assert row["method"] == "dogfood.publish_on_behalf"


def test_mcp_surgical_recovery_is_retired_before_sqlite_composite(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(
        ["surgical_recovery"],
        repo_id=str(harness.repos[0].repository_id),
    )

    result = harness.mcp_client(token, repo_index=0).call_tool(
        "dogfood.surgical_recovery",
        repository_id=str(harness.repos[0].repository_id),
        arguments={
            "job_id": "job_1",
            "reason": "operator inspected the worktree and process",
            "extend_lease_seconds": 900,
            "confirm_write": True,
        },
    )

    assert result["isError"] is True
    structured = result["structuredContent"]
    assert structured["error"] == "dogfood_surgical_recovery_retired"
    assert structured["error_codes"] == ["dogfood_surgical_recovery_retired"]
    data = structured["data"]
    assert data["code"] == "dogfood_surgical_recovery_retired"
    assert (
        data["details"]["blocker"]
        == "legacy_python_composite_uses_repo_local_sqlite_connection_and_progress_lock_policy"
    )
    assert data["details"]["extend_lease_seconds"] == 900
    row = harness.audit_rows(transport="mcp")[-1]
    assert row["decision"] == "allowed"
    assert row["method"] == "dogfood.surgical_recovery"
