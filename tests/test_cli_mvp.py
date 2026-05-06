from __future__ import annotations

import json
import os
import sqlite3
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"


def run_cli(repo: Path, *args: str, check: bool = True) -> dict[str, object]:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    result = subprocess.run(
        [sys.executable, "-m", "agent_runner.cli", "--repo", str(repo), *args, "--json"],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if check and result.returncode != 0:
        raise AssertionError(f"command failed: {result.args}\nstdout={result.stdout}\nstderr={result.stderr}")
    if result.stdout.strip() == "":
        return {}
    payload = json.loads(result.stdout)
    payload["returncode"] = result.returncode
    return payload


def data(payload: dict[str, object]) -> dict[str, object]:
    value = payload["data"]
    assert isinstance(value, dict)
    return value


def init_repo(repo: Path) -> None:
    run_cli(repo, "init")


def prepare_started_run(repo: Path) -> str:
    init_repo(repo)
    prepared = data(run_cli(repo, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = str(prepared["run_id"])
    before = run_cli(repo, "claim-next", "--session-id", "missing", check=False)
    assert before["returncode"] == 3
    run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "agent-runner/v1-test")
    run_cli(repo, "run", "start", "--run-id", run_id)
    return run_id


def register(repo: Path, run_id: str, role: str, lane: str) -> str:
    payload = data(
        run_cli(
            repo,
            "register-session",
            "--run-id",
            run_id,
            "--role",
            role,
            "--lane",
            lane,
            "--capability",
            "review",
        )
    )
    return str(payload["session_id"])


def claim(repo: Path, session_id: str) -> dict[str, object]:
    payload = data(run_cli(repo, "claim-next", "--session-id", session_id))
    assert payload["status"] == "claimed"
    packet = payload["packet"]
    assert isinstance(packet, dict)
    return packet


def packet_ids(packet: dict[str, object]) -> tuple[str, str, str]:
    job = packet["job"]
    lease = packet["lease"]
    assert isinstance(job, dict)
    assert isinstance(lease, dict)
    return str(job["job_id"]), str(lease["message_id"]), str(lease["lease_id"])


def write_artifact(repo: Path, path: str, text: str = "artifact\n") -> None:
    target = repo / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text, encoding="utf-8")


def complete_claimed_job(
    repo: Path,
    session_id: str,
    packet: dict[str, object],
    *,
    logical_name: str,
    kind: str,
    path: str,
) -> None:
    job_id, message_id, lease_id = packet_ids(packet)
    run_cli(repo, "ack", "--session-id", session_id, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(repo, path)
    run_cli(
        repo,
        "publish-artifact",
        "--session-id",
        session_id,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--kind",
        kind,
        "--logical-name",
        logical_name,
        "--path",
        path,
    )
    run_cli(repo, "complete", "--session-id", session_id, "--job-id", job_id, "--lease-id", lease_id)


def test_init_status_and_doctor(tmp_path: Path) -> None:
    init_repo(tmp_path)
    assert (tmp_path / ".agent_runner" / "state.sqlite3").exists()
    assert ".agent_runner/" in (tmp_path / ".gitignore").read_text(encoding="utf-8")
    status = data(run_cli(tmp_path, "status"))
    assert status["runs"] == []
    doctor = data(run_cli(tmp_path, "doctor"))
    assert doctor["ok"] is True


def test_workflow_validate_accepts_json_and_rejects_yaml(tmp_path: Path) -> None:
    init_repo(tmp_path)
    valid = data(run_cli(tmp_path, "workflow", "validate", str(WORKFLOW)))
    assert valid["workflow_id"] == "rfc-ledger-cleanup"
    yaml_path = tmp_path / "workflow.yaml"
    yaml_path.write_text("schema_version: agent-runner.workflow.v1\n", encoding="utf-8")
    rejected = run_cli(tmp_path, "workflow", "validate", str(yaml_path), check=False)
    assert rejected["returncode"] == 8


def test_branch_confirmation_blocks_claims(tmp_path: Path) -> None:
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = str(prepared["run_id"])
    session_id = register(tmp_path, run_id, "author", "codex")
    blocked = run_cli(tmp_path, "claim-next", "--session-id", session_id, check=False)
    assert blocked["returncode"] == 7
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "agent-runner/v1-test")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)
    packet = claim(tmp_path, session_id)
    job = packet["job"]
    assert isinstance(job, dict)
    assert job["workflow_job_id"] == "draft"


def test_register_session_rejects_unknown_role_or_lane(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    bad_role = run_cli(
        tmp_path,
        "register-session",
        "--run-id",
        run_id,
        "--role",
        "ghost",
        "--lane",
        "codex",
        check=False,
    )
    assert bad_role["returncode"] == 4
    bad_lane = run_cli(
        tmp_path,
        "register-session",
        "--run-id",
        run_id,
        "--role",
        "author",
        "--lane",
        "ghost",
        check=False,
    )
    assert bad_lane["returncode"] == 4


def test_complete_requires_ack(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    job_id, _, lease_id = packet_ids(packet)
    write_artifact(tmp_path, "docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md")
    # Publishing is allowed for claimed work, but completion still requires ack.
    run_cli(
        tmp_path,
        "publish-artifact",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--kind",
        "handoff",
        "--logical-name",
        "draft",
        "--path",
        "docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    rejected = run_cli(
        tmp_path,
        "complete",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        check=False,
    )
    assert rejected["returncode"] == 4


def test_artifact_completion_and_verdict_flow(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    draft_packet = claim(tmp_path, author)
    complete_claimed_job(
        tmp_path,
        author,
        draft_packet,
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )

    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    review_packet = claim(tmp_path, reviewer)
    job_id, message_id, lease_id = packet_ids(review_packet)
    run_cli(tmp_path, "ack", "--session-id", reviewer, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(tmp_path, "docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md")
    artifact = data(
        run_cli(
            tmp_path,
            "publish-artifact",
            "--session-id",
            reviewer,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--kind",
            "finding",
            "--logical-name",
            "review",
            "--path",
            "docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
        )
    )
    verdict = data(
        run_cli(
            tmp_path,
            "verdict",
            "--session-id",
            reviewer,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--verdict",
            "accept",
            "--findings-artifact-id",
            str(artifact["artifact_id"]),
        )
    )
    assert verdict["status"] == "completed"
    why = data(run_cli(tmp_path, "why", job_id))
    events = why["events"]
    assert isinstance(events, list)
    assert any(event["event_type"] == "verdict.recorded" for event in events if isinstance(event, dict))


def test_release_requeues_fresh_review_for_new_session_only(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    draft_packet = claim(tmp_path, author)
    complete_claimed_job(
        tmp_path,
        author,
        draft_packet,
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    review_packet = claim(tmp_path, reviewer)
    _, message_id, lease_id = packet_ids(review_packet)
    run_cli(
        tmp_path,
        "release",
        "--session-id",
        reviewer,
        "--message-id",
        message_id,
        "--lease-id",
        lease_id,
        "--reason",
        "freshness test",
        "--requeue",
    )
    no_work = data(run_cli(tmp_path, "claim-next", "--session-id", reviewer))
    assert no_work["status"] == "no_work"
    replacement = register(tmp_path, run_id, "reviewer", "codex")
    packet = claim(tmp_path, replacement)
    job = packet["job"]
    assert isinstance(job, dict)
    assert job["workflow_job_id"] == "review_codex"


def test_publish_artifact_rejects_out_of_scope_paths(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, author)
    job_id, message_id, lease_id = packet_ids(packet)
    run_cli(tmp_path, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(tmp_path, "outside.md")
    rejected = run_cli(
        tmp_path,
        "publish-artifact",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--kind",
        "handoff",
        "--logical-name",
        "draft",
        "--path",
        "outside.md",
        check=False,
    )
    assert rejected["returncode"] == 6


def test_events_are_append_only(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    conn = sqlite3.connect(tmp_path / ".agent_runner" / "state.sqlite3")
    try:
        event_id = conn.execute("SELECT event_id FROM events WHERE run_id = ? LIMIT 1", (run_id,)).fetchone()[0]
        try:
            conn.execute("UPDATE events SET event_type = 'tampered' WHERE event_id = ?", (event_id,))
        except sqlite3.DatabaseError as exc:
            assert "append-only" in str(exc)
        else:
            raise AssertionError("events update unexpectedly succeeded")
    finally:
        conn.close()
