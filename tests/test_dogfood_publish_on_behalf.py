from __future__ import annotations

import json
import os
import sqlite3
import subprocess
import sys
from pathlib import Path
from typing import Any, cast

from striatum.db import connect
from striatum.dogfood import publish_on_behalf
from striatum.errors import InvalidTransitionError

ROOT = Path(__file__).resolve().parents[1]
JsonDict = dict[str, Any]


def _run_cli(repo: Path, *args: str, check: bool = True) -> JsonDict:
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
    if check and result.returncode != 0:
        raise AssertionError(
            f"command failed: {result.args}\nstdout={result.stdout}\nstderr={result.stderr}"
        )
    payload: JsonDict = {"returncode": result.returncode}
    if result.stdout.strip():
        loaded = json.loads(result.stdout)
        assert isinstance(loaded, dict)
        payload.update(cast(JsonDict, loaded))
    return payload


def _data(payload: JsonDict) -> JsonDict:
    value = payload["data"]
    assert isinstance(value, dict)
    return cast(JsonDict, value)


def _workflow(*, job_type: str = "generic", kind: str = "handoff") -> JsonDict:
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "test-dogfood-publish-on-behalf",
        "workflow_version": "2026-05-12",
        "name": "Test dogfood publish on behalf",
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
                    {
                        "logical_name": "out",
                        "kind": kind,
                        "path": "docs/out/OUT.md",
                        "required": True,
                    }
                ],
            }
        ],
        "edges": [],
        "cycles": [],
    }


def _git_init_repo(repo: Path) -> None:
    subprocess.run(["git", "init"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "checkout", "-b", "main"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.name", "Test"], cwd=repo, check=True, capture_output=True)
    seed = repo / ".gitseed"
    seed.write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", ".gitseed"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "commit", "-m", "seed", "--no-gpg-sign"], cwd=repo, check=True, capture_output=True)


def _start_and_claim(
    repo: Path, *, job_type: str = "generic", kind: str = "handoff"
) -> tuple[str, str, JsonDict]:
    _git_init_repo(repo)
    workflow_path = repo / "workflow.json"
    workflow_path.write_text(json.dumps(_workflow(job_type=job_type, kind=kind)), encoding="utf-8")
    _run_cli(repo, "init")
    prepared = _data(_run_cli(repo, "run", "prepare", "--workflow", str(workflow_path)))
    run_id = str(prepared["run_id"])
    _run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/test")
    _run_cli(repo, "run", "start", "--run-id", run_id)
    session = _data(
        _run_cli(
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
    )
    session_id = str(session["session_id"])
    claimed = _data(_run_cli(repo, "claim-next", "--session-id", session_id))
    packet = claimed["packet"]
    assert isinstance(packet, dict)
    return run_id, session_id, cast(JsonDict, packet)


def _packet_ids(packet: JsonDict) -> tuple[str, str, str]:
    job = cast(JsonDict, packet["job"])
    lease = cast(JsonDict, packet["lease"])
    return str(job["job_id"]), str(lease["message_id"]), str(lease["lease_id"])


def _write_artifact(repo: Path, text: str = "artifact\n") -> None:
    target = repo / "docs/out/OUT.md"
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text, encoding="utf-8")


def _job_state(repo: Path, job_id: str) -> str:
    db = sqlite3.connect(repo / ".striatum" / "state.sqlite3")
    try:
        row = db.execute("SELECT state FROM jobs WHERE job_id = ?", (job_id,)).fetchone()
        assert row is not None
        return str(row[0])
    finally:
        db.close()


def _row_state(repo: Path, table: str, id_column: str, row_id: str) -> str:
    db = sqlite3.connect(repo / ".striatum" / "state.sqlite3")
    try:
        row = db.execute(f"SELECT state FROM {table} WHERE {id_column} = ?", (row_id,)).fetchone()
        assert row is not None
        return str(row[0])
    finally:
        db.close()


def _count(repo: Path, table: str, where: str, args: tuple[str, ...]) -> int:
    db = sqlite3.connect(repo / ".striatum" / "state.sqlite3")
    try:
        row = db.execute(f"SELECT COUNT(*) FROM {table} WHERE {where}", args).fetchone()
        assert row is not None
        return int(row[0])
    finally:
        db.close()


def test_publish_on_behalf_acks_publishes_and_completes_claimed_work(tmp_path: Path) -> None:
    _run_id, session_id, packet = _start_and_claim(tmp_path)
    job_id, message_id, lease_id = _packet_ids(packet)
    _write_artifact(tmp_path)

    with connect(tmp_path) as conn:
        result = publish_on_behalf(
            conn,
            repo=tmp_path,
            session_id=session_id,
            artifact_path="docs/out/OUT.md",
            artifact_kind="handoff",
            logical_name="out",
            reason="agent denied ack from supervised wrapper",
        )

    assert result["ok"] is True
    assert result["job_id"] == job_id
    assert result["message_id"] == message_id
    assert result["lease_id"] == lease_id
    assert result["composition_steps"] == [
        {"step": "ack", "status": "acked"},
        {"step": "publish_artifact", "status": "published"},
        {"step": "complete", "status": "completed"},
    ]
    assert _job_state(tmp_path, job_id) == "completed"

    with connect(tmp_path) as conn:
        event = conn.execute(
            "SELECT payload_json FROM events WHERE event_type = 'dogfood.publish_on_behalf'"
        ).fetchone()
    assert event is not None
    payload = json.loads(str(event["payload_json"]))
    assert payload["operator_reason"] == "agent denied ack from supervised wrapper"
    assert payload["operation"] == "publish_on_behalf"


def test_publish_on_behalf_accepts_already_acked_work(tmp_path: Path) -> None:
    _run_id, session_id, packet = _start_and_claim(tmp_path)
    job_id, message_id, lease_id = _packet_ids(packet)
    _run_cli(
        tmp_path,
        "ack",
        "--session-id",
        session_id,
        "--message-id",
        message_id,
        "--lease-id",
        lease_id,
    )
    _write_artifact(tmp_path)

    with connect(tmp_path) as conn:
        result = publish_on_behalf(
            conn,
            repo=tmp_path,
            session_id=session_id,
            artifact_path="docs/out/OUT.md",
            artifact_kind="handoff",
            logical_name="out",
            reason="agent produced artifact but operator is publishing",
        )

    assert result["composition_steps"][0] == {"step": "ack", "status": "already_acked"}
    assert _job_state(tmp_path, job_id) == "completed"


def test_publish_on_behalf_records_review_verdict(tmp_path: Path) -> None:
    _run_id, session_id, packet = _start_and_claim(tmp_path, job_type="review", kind="finding")
    job_id, _message_id, _lease_id = _packet_ids(packet)
    _write_artifact(tmp_path, "accept with findings\n")

    with connect(tmp_path) as conn:
        result = publish_on_behalf(
            conn,
            repo=tmp_path,
            session_id=session_id,
            artifact_path="docs/out/OUT.md",
            artifact_kind="finding",
            logical_name="out",
            verdict="accept_with_findings",
            verdict_rationale="operator confirmed generated findings artifact",
            reason="review wrapper denied ack",
        )

    assert "verdict_id" in result
    assert result["composition_steps"][-1] == {"step": "verdict", "status": "completed"}
    assert _job_state(tmp_path, job_id) == "completed"
    with connect(tmp_path) as conn:
        verdict_row = conn.execute(
            "SELECT verdict, findings_artifact_id FROM verdicts WHERE job_id = ?",
            (job_id,),
        ).fetchone()
    assert verdict_row is not None
    assert verdict_row["verdict"] == "accept_with_findings"
    assert verdict_row["findings_artifact_id"] == result["artifact_id"]


def test_publish_on_behalf_rolls_back_when_completion_fails(tmp_path: Path, monkeypatch) -> None:  # type: ignore[no-untyped-def]
    _run_id, session_id, packet = _start_and_claim(tmp_path)
    job_id, message_id, lease_id = _packet_ids(packet)
    _write_artifact(tmp_path)

    def fail_complete(*args: object, **kwargs: object) -> JsonDict:
        raise InvalidTransitionError("forced complete failure")

    monkeypatch.setattr("striatum.dogfood.operator_tools._complete_locked", fail_complete)

    with connect(tmp_path) as conn:
        result = publish_on_behalf(
            conn,
            repo=tmp_path,
            session_id=session_id,
            artifact_path="docs/out/OUT.md",
            artifact_kind="handoff",
            logical_name="out",
            reason="agent denied ack from supervised wrapper",
        )

    assert result["ok"] is False
    assert result["error"]["code"] == "composite_failed"
    assert _job_state(tmp_path, job_id) == "claimed"
    assert _row_state(tmp_path, "queue_messages", "message_id", message_id) == "claimed"
    assert _row_state(tmp_path, "leases", "lease_id", lease_id) == "active"
    assert _count(tmp_path, "artifacts", "job_id = ?", (job_id,)) == 0
    assert _count(tmp_path, "verdicts", "job_id = ?", (job_id,)) == 0
    assert _count(tmp_path, "events", "job_id = ? AND event_type = 'job.completed'", (job_id,)) == 0
    assert _count(tmp_path, "events", "job_id = ? AND event_type = 'dogfood.publish_on_behalf_failed'", (job_id,)) == 1


def test_publish_on_behalf_denies_without_active_lease(tmp_path: Path) -> None:
    run_id, session_id, _packet = _start_and_claim(tmp_path)
    other_session = _data(
        _run_cli(
            tmp_path,
            "register-session",
            "--run-id",
            run_id,
            "--role",
            "author",
            "--lane",
            "stub",
            "--capability",
            "write",
        )
    )

    with connect(tmp_path) as conn:
        result = publish_on_behalf(
            conn,
            repo=tmp_path,
            session_id=str(other_session["session_id"]),
            artifact_path="docs/out/OUT.md",
            artifact_kind="handoff",
            logical_name="out",
            reason="no claim exists",
        )

    assert result["ok"] is False
    assert result["error"]["code"] == "no_active_lease"
    assert session_id != str(other_session["session_id"])


def test_publish_on_behalf_denies_busy_lease_shape(tmp_path: Path) -> None:
    _run_id, session_id, packet = _start_and_claim(tmp_path)
    job_id, _message_id, _lease_id = _packet_ids(packet)
    with connect(tmp_path) as conn:
        conn.execute("UPDATE jobs SET state = 'running' WHERE job_id = ?", (job_id,))
        conn.commit()

    with connect(tmp_path) as conn:
        result = publish_on_behalf(
            conn,
            repo=tmp_path,
            session_id=session_id,
            artifact_path="docs/out/OUT.md",
            artifact_kind="handoff",
            logical_name="out",
            reason="state shape is inconsistent",
        )

    assert result["ok"] is False
    assert result["error"]["code"] == "lease_busy"


def test_publish_on_behalf_rejects_undeclared_artifact_tuple(tmp_path: Path) -> None:
    _run_id, session_id, _packet = _start_and_claim(tmp_path)
    _write_artifact(tmp_path)

    with connect(tmp_path) as conn:
        result = publish_on_behalf(
            conn,
            repo=tmp_path,
            session_id=session_id,
            artifact_path="docs/out/OUT.md",
            artifact_kind="handoff",
            logical_name="wrong",
            reason="operator selected wrong logical name",
        )

    assert result["ok"] is False
    assert result["error"]["code"] == "artifact_not_expected"
