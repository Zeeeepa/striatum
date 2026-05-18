from __future__ import annotations

import pytest

from _harness.multi_repo import MultiRepoHarness
from striatum.daemon_rpc.envelope import RpcEnvelope
from striatum.daemon_rpc.server import DaemonRpcRouter
from striatum.mcp import DaemonRpcServer

pytestmark = pytest.mark.multi_repo


def test_repo_scoped_write_token_authorizes_repo_a_write_and_audits_allowed(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["write"], repo_id=str(harness.repos[0].repository_id))

    result = harness.mcp_client(token, repo_index=0).call_tool(
        "work.send_message",
        repository_id=str(harness.repos[0].repository_id),
        arguments={"session_id": "sess_missing", "kind": "note", "body_json": "{}"},
    )

    assert result["isError"] is True
    row = harness.audit_rows(transport="mcp")[-1]
    assert row["decision"] == "allowed"
    assert row["denial_reason"] is None
    assert row["method"] == "work.send_message"


def test_repo_scoped_write_token_denied_against_repo_b_and_audits_scope_mismatch(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["write"], repo_id=str(harness.repos[0].repository_id))

    result = harness.mcp_client(token, repo_index=0).call_tool(
        "work.send_message",
        repository_id=str(harness.repos[1].repository_id),
        arguments={"session_id": "sess_missing", "kind": "note", "body_json": "{}"},
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
    assert "workflow.validate" not in names
    assert "publish_artifact" not in names
    assert "apply.reviewed_patch" not in names


def test_tools_list_hides_production_unsupported_methods_for_matching_tokens(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["read", "write", "surgical_recovery"], repo_id=str(harness.repos[0].repository_id))

    names = {tool["name"] for tool in harness.mcp_client(token).list_tools(repository_id=str(harness.repos[0].repository_id))}

    assert "workflow.generate.preview" in names
    assert "workflow.validate" not in names
    assert "workflow.generate" not in names
    assert "workflow.upgrade" not in names


def test_pg_daemon_mcp_resources_filter_repo_scope_without_sqlite(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    repo_a = str(harness.repos[0].repository_id)
    repo_b = str(harness.repos[1].repository_id)
    token = harness.issue_token(["read"], repo_id=repo_a)
    monkeypatch.setenv("STRIATUM_SQLITE_CONNECT_TRIPWIRE", "1")

    resources = harness.mcp_client(token).list_resources()

    uris = {str(item["uri"]) for item in resources}
    assert "striatum://daemon/repos" in uris
    assert "striatum://daemon/dashboard" in uris
    assert f"striatum://repo/{repo_a}/status" in uris
    assert f"striatum://repo/{repo_b}/status" not in uris
    row = harness.audit_rows(transport="mcp")[-1]
    assert row["method"] == "mcp.resources.list"
    assert row["decision"] == "allowed"


def test_pg_daemon_mcp_resources_missing_token_denied_without_sqlite(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    monkeypatch.setenv("STRIATUM_SQLITE_CONNECT_TRIPWIRE", "1")
    conn = harness.pg_conn()
    try:
        response = DaemonRpcServer(pg_conn=conn).handle_request(
            {
                "jsonrpc": "2.0",
                "id": "req-missing-token",
                "method": "resources/list",
                "params": {"request_id": "req-missing-token"},
            }
        )
    finally:
        conn.close()

    assert response is not None
    assert response["error"]["code"] == -32603
    assert "token_missing" in response["error"]["message"]
    row = harness.audit_rows(transport="mcp")[-1]
    assert row["method"] == "mcp.resources.list"
    assert row["decision"] == "denied"
    assert row["denial_reason"] == "token_missing"


def test_pg_daemon_mcp_resource_reads_preserve_shapes_without_sqlite(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    repo_id = str(harness.repos[0].repository_id)
    repo_root = str(harness.repos[0].path)
    harness.daemon_db_query(
        """
        INSERT INTO striatumd.workflow_snapshots(
          repository_id, workflow_snapshot_id, workflow_id,
          content_sha256, workflow_json, loaded_at
        )
        VALUES (%s, 'snap_mcp_resource', 'wf_mcp_resource',
                'sha', '{}'::jsonb, now())
        """,
        (repo_id,),
    )
    harness.daemon_db_query(
        """
        INSERT INTO striatumd.runs(
          repository_id, run_id, workflow_snapshot_id, repo_root,
          state, created_at
        )
        VALUES (%s, 'run_mcp_resource', 'snap_mcp_resource',
                %s, 'running', now())
        """,
        (repo_id, repo_root),
    )
    token = harness.issue_token(["read"], repo_id=repo_id)
    client = harness.mcp_client(token)
    monkeypatch.setenv("STRIATUM_SQLITE_CONNECT_TRIPWIRE", "1")

    repos = client.read_resource("striatum://daemon/repos")
    status = client.read_resource(f"striatum://repo/{repo_id}/status")
    runs = client.read_resource(f"striatum://repo/{repo_id}/runs")
    blockers = client.read_resource(f"striatum://repo/{repo_id}/blockers")
    stale = client.read_resource(f"striatum://repo/{repo_id}/stale-leases")
    run = client.read_resource(f"striatum://repo/{repo_id}/run/run_mcp_resource")
    why = client.read_resource(f"striatum://repo/{repo_id}/run/run_mcp_resource/why?id=run_mcp_resource")

    assert [row["repository_id"] for row in repos["repositories"]] == [repo_id]
    assert status["mode"] == "daemon"
    assert status["repository_id"] == repo_id
    assert status["runs"][0]["run_id"] == "run_mcp_resource"
    assert runs["runs"] == status["runs"]
    assert blockers == {
        "mode": "daemon",
        "repository_id": repo_id,
        "protocol_version": 1,
        "blockers": [],
    }
    assert stale["stale_leases"] == [
        {"run_id": "run_mcp_resource", "stale_count": 0, "stale_leases": [], "next_actions": []}
    ]
    assert run["runs"][0]["run_id"] == "run_mcp_resource"
    assert why["target_type"] == "run"
    assert why["run"]["run_id"] == "run_mcp_resource"


def test_pg_daemon_mcp_resource_read_denies_repo_scope_before_lookup(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    repo_a = str(harness.repos[0].repository_id)
    repo_b = str(harness.repos[1].repository_id)
    token = harness.issue_token(["read"], repo_id=repo_a)
    monkeypatch.setenv("STRIATUM_SQLITE_CONNECT_TRIPWIRE", "1")
    conn = harness.pg_conn()
    try:
        response = DaemonRpcServer(pg_conn=conn).handle_request(
            {
                "jsonrpc": "2.0",
                "id": "req-scope-denied",
                "method": "resources/read",
                "params": {
                    "uri": f"striatum://repo/{repo_b}/status",
                    "token": token,
                    "request_id": "req-scope-denied",
                },
            }
        )
    finally:
        conn.close()

    assert response is not None
    assert response["error"]["code"] == -32603
    assert "capability_scope_mismatch" in response["error"]["message"]
    row = harness.audit_rows(transport="mcp")[-1]
    assert row["method"] == "mcp.resources.read"
    assert row["decision"] == "denied"
    assert row["denial_reason"] == "capability_scope_mismatch"


def test_dashboard_all_daemon_route_uses_postgres_without_sqlite(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["read"])
    monkeypatch.setenv("STRIATUM_SQLITE_CONNECT_TRIPWIRE", "1")
    conn = harness.pg_conn()
    try:
        response = DaemonRpcRouter(pg_conn=conn).handle(
            RpcEnvelope(
                schema_version=1,
                request_id="req-dashboard-all-pg",
                method="dashboard.all",
                params={},
                capability_token=token,
            ),
            require_handshake=False,
        )
    finally:
        conn.close()

    assert response.ok is True
    assert response.data["mode"] == "daemon"
    assert {
        row["repository_id"]
        for row in response.data["repositories"]
    } == {str(repo.repository_id) for repo in harness.repos}
    assert all("run_progress" in row for row in response.data["repositories"])


def test_read_only_token_cannot_call_write_tool(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["read"], repo_id=str(harness.repos[0].repository_id))

    result = harness.mcp_client(token, repo_index=0).call_tool(
        "work.send_message",
        repository_id=str(harness.repos[0].repository_id),
        arguments={"session_id": "sess_missing", "kind": "note", "body_json": "{}"},
    )

    assert result["isError"] is True
    assert result["structuredContent"]["error"] == "capability_missing"
    assert harness.audit_rows(transport="mcp")[-1]["denial_reason"] == "capability_missing"


def test_retired_apply_reviewed_patch_denied_as_unknown_and_audited(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    token = harness.issue_token(["read"], repo_id=str(harness.repos[0].repository_id))

    result = harness.mcp_client(token).call_tool(
        "apply.reviewed_patch",
        repository_id=str(harness.repos[0].repository_id),
    )

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
        "work.send_message",
        repository_id=str(harness.repos[0].repository_id),
        arguments={"session_id": "sess_missing", "kind": "note", "body_json": "{}"},
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
        "work.send_message",
        repository_id=str(harness.repos[0].repository_id),
        arguments={"session_id": "sess_missing", "kind": "note", "body_json": "{}"},
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

    args: dict[str, object] = {"session_id": "sess_missing", "kind": "note", "body_json": "{}"}
    client.call_tool("work.send_message", repository_id=str(harness.repos[0].repository_id), arguments=args)
    client.call_tool("work.send_message", repository_id=str(harness.repos[1].repository_id), arguments=args)
    client.call_tool("not.a.method", repository_id=str(harness.repos[0].repository_id))

    harness.assert_audit_chain()
