from __future__ import annotations

import json
import os
import sqlite3
import subprocess
import sys
from pathlib import Path
from typing import Any, cast

import pytest

from _harness.multi_repo import MultiRepoHarness

pytestmark = pytest.mark.multi_repo
ROOT = Path(__file__).resolve().parents[1]
JsonDict = dict[str, Any]


def _run_cli(repo: Path, *args: str) -> JsonDict:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    result = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), *args, "--json"],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise AssertionError(f"command failed: {result.args}\nstdout={result.stdout}\nstderr={result.stderr}")
    loaded = json.loads(result.stdout)
    assert isinstance(loaded, dict)
    return cast(JsonDict, loaded["data"])


def _workflow(*, job_type: str = "generic", kind: str = "handoff") -> JsonDict:
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "test-mcp-dogfood-e2e",
        "workflow_version": "2026-05-12",
        "name": "Test MCP dogfood e2e",
        "branch": {"mode": "confirm", "suggested_name": "striatum/test"},
        "coordinator": {"role_id": "author", "lane_id": "stub"},
        "lanes": {"stub": {"adapter": "manual", "capabilities": ["write", "review"]}},
        "roles": {"author": {"definition_path": "roles/author.md"}},
        "context_docs": [],
        "parallelism": {"mode": "declared", "max_active_jobs": 1},
        "jobs": [
            {
                "id": "demo",
                "type": job_type,
                "title": "Demo",
                "role_id": "author",
                "lane_id": "stub",
                "objective": "demo",
                "task_prompt": {"path": "prompts/demo.md"},
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": ["docs/out/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {"logical_name": "out", "kind": kind, "path": "docs/out/OUT.md", "required": True}
                ],
            }
        ],
        "edges": [],
        "cycles": [],
    }


def _git_init_repo(repo: Path) -> None:
    if (repo / ".git").exists():
        return
    subprocess.run(["git", "init"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "checkout", "-b", "main"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.name", "Test"], cwd=repo, check=True, capture_output=True)
    seed = repo / ".gitseed"
    seed.write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", ".gitseed"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "commit", "-m", "seed", "--no-gpg-sign"], cwd=repo, check=True, capture_output=True)


def _start_and_claim(repo: Path, *, job_type: str = "generic", kind: str = "handoff") -> tuple[str, str, JsonDict]:
    _git_init_repo(repo)
    workflow_path = repo / "workflow.json"
    workflow_path.write_text(json.dumps(_workflow(job_type=job_type, kind=kind)), encoding="utf-8")
    _run_cli(repo, "init")
    prepared = _run_cli(repo, "run", "prepare", "--workflow", str(workflow_path))
    run_id = str(prepared["run_id"])
    _run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/test")
    _run_cli(repo, "run", "start", "--run-id", run_id)
    session = _run_cli(
        repo,
        "register-session",
        "--run-id",
        run_id,
        "--role",
        "author",
        "--lane",
        "stub",
        "--capability",
        "write",
        "--capability",
        "review",
    )
    claimed = _run_cli(repo, "claim-next", "--session-id", str(session["session_id"]))
    return run_id, str(session["session_id"]), cast(JsonDict, claimed["packet"])


def _packet_ids(packet: JsonDict) -> tuple[str, str, str]:
    job = cast(JsonDict, packet["job"])
    lease = cast(JsonDict, packet["lease"])
    return str(job["job_id"]), str(lease["message_id"]), str(lease["lease_id"])


def _write_artifact(repo: Path, text: str = "artifact\n") -> None:
    target = repo / "docs/out/OUT.md"
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text, encoding="utf-8")


def test_mcp_publish_on_behalf_dispatches_and_completes_job(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    repo = harness.repos[0].path
    _run_id, session_id, packet = _start_and_claim(repo)
    job_id, _message_id, _lease_id = _packet_ids(packet)
    _write_artifact(repo)
    harness.register_all()
    token = harness.issue_token(["write"], repo_id=str(harness.repos[0].repository_id))

    result = harness.mcp_client(token, repo_index=0).call_tool(
        "dogfood.publish_on_behalf",
        repository_id=str(harness.repos[0].repository_id),
        arguments={
            "session_id": session_id,
            "artifact_path": "docs/out/OUT.md",
            "artifact_kind": "handoff",
            "logical_name": "out",
            "reason": "agent denied ack from supervised wrapper",
        },
    )

    assert result["isError"] is False
    data = result["structuredContent"]["data"]
    assert data["status"] == "published_on_behalf_completed"
    with sqlite3.connect(repo / ".striatum" / "state.sqlite3") as conn:
        conn.row_factory = sqlite3.Row
        job = conn.execute("SELECT state FROM jobs WHERE job_id = ?", (job_id,)).fetchone()
        artifact = conn.execute("SELECT 1 FROM artifacts WHERE job_id = ?", (job_id,)).fetchone()
        event = conn.execute(
            "SELECT 1 FROM events WHERE event_type = 'dogfood.publish_on_behalf'"
        ).fetchone()
    assert job is not None and job["state"] == "completed"
    assert artifact is not None
    assert event is not None
    row = harness.audit_rows(transport="mcp")[-1]
    assert row["decision"] == "allowed"
    assert row["method"] == "dogfood.publish_on_behalf"


def test_mcp_publish_on_behalf_records_review_verdict(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
) -> None:
    harness = multi_repo_harness
    repo = harness.repos[0].path
    _run_id, session_id, packet = _start_and_claim(repo, job_type="review", kind="finding")
    job_id, _message_id, _lease_id = _packet_ids(packet)
    _write_artifact(repo, "accept with findings\n")
    harness.register_all()
    token = harness.issue_token(["write"], repo_id=str(harness.repos[0].repository_id))

    result = harness.mcp_client(token, repo_index=0).call_tool(
        "dogfood.publish_on_behalf",
        repository_id=str(harness.repos[0].repository_id),
        arguments={
            "session_id": session_id,
            "artifact_path": "docs/out/OUT.md",
            "artifact_kind": "finding",
            "logical_name": "out",
            "reason": "review wrapper denied ack",
            "verdict": "accept_with_findings",
            "verdict_rationale": "operator confirmed generated findings artifact",
        },
    )

    assert result["isError"] is False
    data = result["structuredContent"]["data"]
    assert data["status"] == "published_on_behalf_reviewed"
    assert data["verdict_id"]
    assert data["findings_artifact_id"] == data["artifact_id"]
    with sqlite3.connect(repo / ".striatum" / "state.sqlite3") as conn:
        verdict = conn.execute("SELECT verdict FROM verdicts WHERE job_id = ?", (job_id,)).fetchone()
    assert verdict is not None
    assert verdict[0] == "accept_with_findings"
