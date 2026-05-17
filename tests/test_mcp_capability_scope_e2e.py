from __future__ import annotations

import pytest

from _harness.multi_repo import MultiRepoHarness

pytestmark = pytest.mark.multi_repo


def test_repo_scoped_write_token_authorizes_repo_a_write_and_audits_allowed(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["write"], repo_id=str(harness.repos[0].repository_id))

    result = harness.mcp_client(token, repo_index=0).call_tool(
        "dogfood.publish_on_behalf",
        repository_id=str(harness.repos[0].repository_id),
        arguments={"reason": "capability scope smoke"},
    )

    assert result["isError"] is True
    assert result["structuredContent"]["error"] == "dogfood_publish_on_behalf_retired"
    row = harness.audit_rows(transport="mcp")[-1]
    assert row["decision"] == "allowed"
    assert row["denial_reason"] is None
    assert row["method"] == "dogfood.publish_on_behalf"


def test_repo_scoped_write_token_denied_against_repo_b_and_audits_scope_mismatch(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["write"], repo_id=str(harness.repos[0].repository_id))

    result = harness.mcp_client(token, repo_index=0).call_tool(
        "dogfood.publish_on_behalf",
        repository_id=str(harness.repos[1].repository_id),
    )

    assert result["isError"] is True
    assert result["structuredContent"]["error"] == "capability_scope_mismatch"
    row = harness.audit_rows(transport="mcp")[-1]
    assert row["decision"] == "denied"
    assert row["denial_reason"] == "capability_scope_mismatch"


def test_read_only_token_lists_only_read_tools(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["read"], repo_id=str(harness.repos[0].repository_id))

    names = {tool["name"] for tool in harness.mcp_client(token).list_tools(repository_id=str(harness.repos[0].repository_id))}

    assert "status" in names
    assert "publish_artifact" not in names
    assert "apply.reviewed_patch" not in names


def test_read_only_token_cannot_call_write_tool(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["read"], repo_id=str(harness.repos[0].repository_id))

    result = harness.mcp_client(token, repo_index=0).call_tool(
        "dogfood.publish_on_behalf",
        repository_id=str(harness.repos[0].repository_id),
    )

    assert result["isError"] is True
    assert result["structuredContent"]["error"] == "capability_missing"
    assert harness.audit_rows(transport="mcp")[-1]["denial_reason"] == "capability_missing"


def test_unknown_method_denied_and_audited(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["read"], repo_id=str(harness.repos[0].repository_id))

    result = harness.mcp_client(token).call_tool("not.a.method", repository_id=str(harness.repos[0].repository_id))

    assert result["isError"] is True
    assert result["structuredContent"]["error"] == "method_unknown"
    assert harness.audit_rows(transport="mcp")[-1]["denial_reason"] == "method_unknown"


def test_revoked_token_denied_and_audited(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["write"], repo_id=str(harness.repos[0].repository_id))
    harness.revoke_token(token)

    result = harness.mcp_client(token, repo_index=0).call_tool(
        "dogfood.publish_on_behalf",
        repository_id=str(harness.repos[0].repository_id),
    )

    assert result["structuredContent"]["error"] == "token_revoked"
    assert harness.audit_rows(transport="mcp")[-1]["denial_reason"] == "token_revoked"


def test_expired_token_denied_and_audited(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["write"], repo_id=str(harness.repos[0].repository_id))
    harness.expire_token(token)

    result = harness.mcp_client(token, repo_index=0).call_tool(
        "dogfood.publish_on_behalf",
        repository_id=str(harness.repos[0].repository_id),
    )

    assert result["structuredContent"]["error"] == "token_expired"
    assert harness.audit_rows(transport="mcp")[-1]["denial_reason"] == "token_expired"


def test_audit_chain_continuous_across_allowed_and_denied_calls(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["write"], repo_id=str(harness.repos[0].repository_id))
    client = harness.mcp_client(token, repo_index=0)

    client.call_tool("dogfood.publish_on_behalf", repository_id=str(harness.repos[0].repository_id))
    client.call_tool("dogfood.publish_on_behalf", repository_id=str(harness.repos[1].repository_id))
    client.call_tool("not.a.method", repository_id=str(harness.repos[0].repository_id))

    harness.assert_audit_chain()
