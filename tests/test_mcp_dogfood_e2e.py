from __future__ import annotations

import pytest

from _harness.multi_repo import MultiRepoHarness

pytestmark = pytest.mark.multi_repo


def test_mcp_publish_on_behalf_is_removed_from_production_contract(
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
    assert structured["error"] == "method_unknown"
    row = harness.audit_rows(transport="mcp")[-1]
    assert row["decision"] == "denied"
    assert row["denial_reason"] == "method_unknown"
    assert row["method"] == "dogfood.publish_on_behalf"


def test_mcp_surgical_recovery_is_removed_from_production_contract(
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
    assert structured["error"] == "method_unknown"
    row = harness.audit_rows(transport="mcp")[-1]
    assert row["decision"] == "denied"
    assert row["denial_reason"] == "method_unknown"
    assert row["method"] == "dogfood.surgical_recovery"
