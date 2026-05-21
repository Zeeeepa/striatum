from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import pytest

from _harness.mcp import McpClient
from _harness.multi_repo import MultiRepoHarness
from striatum.workflow import validate_workflow

pytestmark = pytest.mark.multi_repo


def test_fake_mcp_agent_completes_work_packet_loop(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    repo_id = _register_single_repo(harness)
    repo_root = harness.repos[0].path
    workflow_path = _write_fake_agent_workflow(repo_root, "fake-agent-workflow.json")
    token = harness.issue_token(["admin", "claim", "write", "read"], repo_id=repo_id)
    client = harness.mcp_client(token, repo_index=0)

    names = {str(tool["name"]) for tool in client.list_tools(repository_id=repo_id)}
    assert {
        "run.prepare",
        "run.start",
        "session.register",
        "work.await_packet",
        "work.ack",
        "work.heartbeat",
        "artifact.publish",
        "work.complete",
    } <= names

    prepared = _call_data(
        client,
        "run.prepare",
        repo_id=repo_id,
        arguments={"workflow": workflow_path.name},
    )
    run_id = str(prepared["run_id"])
    assert prepared["state"] == "ready"
    assert _call_data(client, "run.start", repo_id=repo_id, arguments={"run_id": run_id})[
        "state"
    ] == "running"

    session = _call_data(
        client,
        "session.register",
        repo_id=repo_id,
        arguments={"run_id": run_id, "role": "author", "lane": "codex"},
    )
    session_id = str(session["session_id"])
    claimed = _call_data(
        client,
        "work.await_packet",
        repo_id=repo_id,
        arguments={"session_id": session_id, "lease_seconds": 60},
    )
    assert claimed["status"] == "claimed"
    packet = _as_dict(claimed["packet"])
    lease = _as_dict(packet["lease"])
    job = _as_dict(packet["job"])
    expected_artifact = _as_dict(packet["expected_artifacts"][0])
    lease_id = str(lease["lease_id"])
    message_id = str(lease["message_id"])
    job_id = str(job["job_id"])

    assert _call_data(
        client,
        "work.ack",
        repo_id=repo_id,
        arguments={"session_id": session_id, "message_id": message_id, "lease_id": lease_id},
    )["status"] == "acked"
    assert _call_data(
        client,
        "work.heartbeat",
        repo_id=repo_id,
        arguments={"session_id": session_id, "lease_id": lease_id, "extend_seconds": 90},
    )["status"] == "heartbeat"

    artifact_path = str(expected_artifact["path"])
    author_line = str(expected_artifact["author_line"])
    _write_handoff(repo_root / artifact_path, author_line)
    publish = _call_data(
        client,
        "artifact.publish",
        repo_id=repo_id,
        arguments={
            "session_id": session_id,
            "job_id": job_id,
            "lease_id": lease_id,
            "kind": "handoff",
            "logical_name": str(expected_artifact["logical_name"]),
            "path": artifact_path,
        },
    )
    assert publish["status"] == "published"
    assert _call_data(
        client,
        "work.complete",
        repo_id=repo_id,
        arguments={
            "session_id": session_id,
            "job_id": job_id,
            "lease_id": lease_id,
            "summary": "fake MCP agent completed the packet",
        },
    )["status"] == "completed"

    rows = harness.daemon_db_query(
        """
        SELECT r.state AS run_state,
               j.state AS job_state,
               q.state AS queue_state,
               l.state AS lease_state,
               s.state AS session_state,
               a.logical_name AS artifact_logical_name
          FROM striatumd.runs r
          JOIN striatumd.jobs j
            ON j.repository_id = r.repository_id AND j.run_id = r.run_id
          JOIN striatumd.queue_messages q
            ON q.repository_id = j.repository_id AND q.job_id = j.job_id
          JOIN striatumd.leases l
            ON l.repository_id = j.repository_id AND l.resource_id = j.job_id
          JOIN striatumd.sessions s
            ON s.repository_id = r.repository_id AND s.run_id = r.run_id
          JOIN striatumd.artifacts a
            ON a.repository_id = j.repository_id AND a.job_id = j.job_id
         WHERE r.repository_id = %s AND r.run_id = %s
        """,
        (repo_id, run_id),
    )
    assert rows == [
        {
            "run_state": "completed",
            "job_state": "completed",
            "queue_state": "completed",
            "lease_state": "released",
            "session_state": "closed",
            "artifact_logical_name": "handoff",
        }
    ]
    methods = [str(row["method"]) for row in harness.audit_rows(transport="mcp")]
    for method in [
        "run.prepare",
        "run.start",
        "session.register",
        "work.await_packet",
        "work.ack",
        "work.heartbeat",
        "artifact.publish",
        "work.complete",
    ]:
        assert method in methods


def test_fake_mcp_agent_stale_lease_refuses_lifecycle_mutation(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    repo_id = _register_single_repo(harness)
    workflow_path = _write_fake_agent_workflow(
        harness.repos[0].path, "stale-lease-workflow.json"
    )
    token = harness.issue_token(["admin", "claim", "write", "read"], repo_id=repo_id)
    client = harness.mcp_client(token, repo_index=0)
    run_id = str(
        _call_data(
            client,
            "run.prepare",
            repo_id=repo_id,
            arguments={"workflow": workflow_path.name},
        )["run_id"]
    )
    _call_data(client, "run.start", repo_id=repo_id, arguments={"run_id": run_id})
    session_id = str(
        _call_data(
            client,
            "session.register",
            repo_id=repo_id,
            arguments={"run_id": run_id, "role": "author", "lane": "codex"},
        )["session_id"]
    )
    claimed = _call_data(
        client,
        "work.await_packet",
        repo_id=repo_id,
        arguments={"session_id": session_id, "lease_seconds": 60},
    )
    lease = _as_dict(_as_dict(claimed["packet"])["lease"])
    lease_id = str(lease["lease_id"])
    message_id = str(lease["message_id"])

    harness.daemon_db_query(
        """
        UPDATE striatumd.leases
           SET expires_at = now() - interval '1 second'
         WHERE repository_id = %s AND lease_id = %s
        """,
        (repo_id, lease_id),
    )

    denied = client.call_tool(
        "work.ack",
        repository_id=repo_id,
        arguments={"session_id": session_id, "message_id": message_id, "lease_id": lease_id},
    )

    assert denied["isError"] is True
    assert denied["structuredContent"]["error"] == "lease_error"
    assert "expired" in denied["structuredContent"]["error_message"]
    row = harness.audit_rows(transport="mcp")[-1]
    assert row["method"] == "work.ack"
    assert row["decision"] == "denied"
    assert row["denial_reason"] == "lease_error"


def _register_single_repo(harness: MultiRepoHarness) -> str:
    repo_id = harness.register_all()[0]
    return str(repo_id)


def _write_fake_agent_workflow(repo_root: Path, filename: str) -> Path:
    workflow = {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": f"test-{filename.removesuffix('.json')}",
        "workflow_version": "2026-05-21",
        "name": "Fake MCP Agent Loop",
        "branch": {"mode": "auto", "suggested_name": "test/fake-mcp-agent"},
        "coordinator": {"role_id": "author", "lane_id": "codex"},
        "lanes": {"codex": {"adapter": "manual", "display_model": "Fake MCP Agent"}},
        "roles": {"author": {}},
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 1,
            "require_disjoint_write_scopes": True,
        },
        "jobs": [
            {
                "id": "draft",
                "type": "draft",
                "title": "Draft fake agent handoff",
                "role_id": "author",
                "lane_id": "codex",
                "objective": "Write the fake-agent handoff artifact.",
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": ["docs/fake-agent/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "handoff",
                        "kind": "handoff",
                        "path": "docs/fake-agent/HANDOFF.md",
                        "required": True,
                    }
                ],
            }
        ],
        "edges": [],
        "cycles": [],
    }
    validate_workflow(workflow)
    path = repo_root / filename
    path.write_text(json.dumps(workflow, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return path


def _write_handoff(path: Path, author_line: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        (
            f"# Fake MCP Agent Handoff\n{author_line}\n\n"
            "The fake MCP agent completed the packet through `/mcp` tools/call.\n"
        ),
        encoding="utf-8",
    )


def _call_data(
    client: McpClient,
    method: str,
    *,
    repo_id: str,
    arguments: dict[str, object],
) -> dict[str, Any]:
    result = client.call_tool(method, repository_id=repo_id, arguments=arguments)
    assert result["isError"] is False, result
    data = result["structuredContent"]["data"]
    assert isinstance(data, dict)
    return data


def _as_dict(value: object) -> dict[str, Any]:
    assert isinstance(value, dict), value
    return value
