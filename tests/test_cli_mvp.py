from __future__ import annotations

import json
import os
import sqlite3
import subprocess
import sys
from copy import deepcopy
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


def temporary_workflow(tmp_path: Path, workflow: dict[str, object]) -> Path:
    path = tmp_path / "workflow.json"
    path.write_text(json.dumps(workflow), encoding="utf-8")
    return path


def example_workflow() -> dict[str, object]:
    loaded = json.loads(WORKFLOW.read_text(encoding="utf-8"))
    assert isinstance(loaded, dict)
    return loaded


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


def verdict_claimed_review(
    repo: Path,
    session_id: str,
    packet: dict[str, object],
    *,
    verdict: str,
    logical_name: str = "review",
    kind: str = "finding",
    path: str,
) -> dict[str, object]:
    job_id, message_id, lease_id = packet_ids(packet)
    run_cli(repo, "ack", "--session-id", session_id, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(repo, path, text=f"{verdict}\n")
    artifact = data(
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
    )
    return data(
        run_cli(
            repo,
            "verdict",
            "--session-id",
            session_id,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--verdict",
            verdict,
            "--findings-artifact-id",
            str(artifact["artifact_id"]),
        )
    )


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


def test_verdict_reject_fails_run_and_does_not_enqueue_downstream(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    verdict = verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="reject",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    assert verdict["status"] == "failed"
    status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert status["runs"][0]["state"] == "failed"
    assert status["jobs"]["failed"] == 1
    ledger = register(tmp_path, run_id, "ledger", "codex")
    no_work = data(run_cli(tmp_path, "claim-next", "--session-id", ledger))
    assert no_work["status"] == "no_work"


def test_accepting_review_verdict_unblocks_downstream(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    codex = register(tmp_path, run_id, "reviewer", "codex")
    gemini = register(tmp_path, run_id, "reviewer", "gemini")
    verdict_claimed_review(
        tmp_path,
        codex,
        claim(tmp_path, codex),
        verdict="accept_with_findings",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    ledger = register(tmp_path, run_id, "ledger", "codex")
    assert data(run_cli(tmp_path, "claim-next", "--session-id", ledger))["status"] == "no_work"
    verdict_claimed_review(
        tmp_path,
        gemini,
        claim(tmp_path, gemini),
        verdict="accept",
        path="docs/reviews/rfc-ledger/gemini/RFC_LEDGER_REVIEW.md",
    )
    packet = claim(tmp_path, ledger)
    assert packet["job"]["workflow_job_id"] == "findings_ledger"


def test_verdict_needs_revision_uses_declared_cycle(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    codex = register(tmp_path, run_id, "reviewer", "codex")
    gemini = register(tmp_path, run_id, "reviewer", "gemini")
    verdict_claimed_review(
        tmp_path,
        codex,
        claim(tmp_path, codex),
        verdict="accept",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    verdict_claimed_review(
        tmp_path,
        gemini,
        claim(tmp_path, gemini),
        verdict="accept",
        path="docs/reviews/rfc-ledger/gemini/RFC_LEDGER_REVIEW.md",
    )
    ledger = register(tmp_path, run_id, "ledger", "codex")
    complete_claimed_job(
        tmp_path,
        ledger,
        claim(tmp_path, ledger),
        logical_name="ledger",
        kind="findings_ledger",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_FINDINGS_LEDGER.md",
    )
    synth = register(tmp_path, run_id, "synthesizer", "claude")
    complete_claimed_job(
        tmp_path,
        synth,
        claim(tmp_path, synth),
        logical_name="synthesis",
        kind="synthesis",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_SYNTHESIS.md",
    )
    final = register(tmp_path, run_id, "reviewer", "claude")
    verdict = verdict_claimed_review(
        tmp_path,
        final,
        claim(tmp_path, final),
        verdict="needs_revision",
        path="docs/reviews/rfc-ledger/final/RFC_LEDGER_FINAL_REVIEW.md",
    )
    assert verdict["status"] == "revision_requested"
    next_synth = register(tmp_path, run_id, "synthesizer", "claude")
    packet = claim(tmp_path, next_synth)
    assert packet["job"]["workflow_job_id"] == "synthesis"
    assert packet["job"]["attempt"] == 2
    status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert status["runs"][0]["state"] == "running"


def test_verdict_needs_revision_without_cycle_waits_human(tmp_path: Path) -> None:
    workflow = example_workflow()
    workflow["cycles"] = []
    workflow_path = temporary_workflow(tmp_path, workflow)
    init_repo(tmp_path)
    run_id = str(data(run_cli(tmp_path, "run", "prepare", "--workflow", str(workflow_path)))["run_id"])
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "agent-runner/v1-test")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    verdict = verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="needs_revision",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )
    assert verdict["status"] == "waiting_human"
    conn = sqlite3.connect(tmp_path / ".agent_runner" / "state.sqlite3")
    try:
        blocker = conn.execute("SELECT state FROM blockers WHERE run_id = ?", (run_id,)).fetchone()
        assert blocker[0] == "open"
    finally:
        conn.close()
    ledger = register(tmp_path, run_id, "ledger", "codex")
    assert data(run_cli(tmp_path, "claim-next", "--session-id", ledger))["status"] == "no_work"


def test_edges_materialize_dependencies_without_needs(tmp_path: Path) -> None:
    workflow = example_workflow()
    for job in workflow["jobs"]:
        job.pop("needs", None)
    workflow_path = temporary_workflow(tmp_path, workflow)
    init_repo(tmp_path)
    run_id = str(data(run_cli(tmp_path, "run", "prepare", "--workflow", str(workflow_path)))["run_id"])
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "agent-runner/v1-test")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)
    author = register(tmp_path, run_id, "author", "codex")
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    assert claim(tmp_path, author)["job"]["workflow_job_id"] == "draft"
    assert data(run_cli(tmp_path, "claim-next", "--session-id", reviewer))["status"] == "no_work"


def test_workflow_rejects_needs_edges_mismatch(tmp_path: Path) -> None:
    init_repo(tmp_path)
    workflow = example_workflow()
    jobs = workflow["jobs"]
    assert isinstance(jobs, list)
    mismatched = deepcopy(workflow)
    mismatched["jobs"][1]["needs"] = []
    rejected = run_cli(tmp_path, "workflow", "validate", str(temporary_workflow(tmp_path, mismatched)), check=False)
    assert rejected["returncode"] == 8


def test_complete_requires_expected_artifact_path_and_kind(tmp_path: Path) -> None:
    bad_repo = tmp_path / "bad"
    bad_repo.mkdir()
    run_id = prepare_started_run(bad_repo)
    author = register(bad_repo, run_id, "author", "codex")
    packet = claim(bad_repo, author)
    job_id, message_id, lease_id = packet_ids(packet)
    run_cli(bad_repo, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(bad_repo, "docs/reviews/rfc-ledger/WRONG.md")
    run_cli(
        bad_repo,
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
        "docs/reviews/rfc-ledger/WRONG.md",
    )
    rejected = run_cli(
        bad_repo,
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

    good_repo = tmp_path / "good"
    good_repo.mkdir()
    run_id = prepare_started_run(good_repo)
    author = register(good_repo, run_id, "author", "codex")
    complete_claimed_job(
        good_repo,
        author,
        claim(good_repo, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )


def test_verdict_requires_expected_artifact_path_and_kind(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    packet = claim(tmp_path, reviewer)
    job_id, message_id, lease_id = packet_ids(packet)
    run_cli(tmp_path, "ack", "--session-id", reviewer, "--message-id", message_id, "--lease-id", lease_id)
    write_artifact(tmp_path, "docs/reviews/rfc-ledger/codex/WRONG.md")
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
            "docs/reviews/rfc-ledger/codex/WRONG.md",
        )
    )
    rejected = run_cli(
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
        check=False,
    )
    assert rejected["returncode"] == 4


def test_doctor_reports_bad_review_gate_state(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    conn = sqlite3.connect(tmp_path / ".agent_runner" / "state.sqlite3")
    try:
        review = conn.execute(
            "SELECT job_id FROM jobs WHERE run_id = ? AND workflow_job_id = 'review_codex'",
            (run_id,),
        ).fetchone()
        conn.execute("UPDATE jobs SET state = 'completed' WHERE job_id = ?", (review[0],))
        conn.commit()
    finally:
        conn.close()
    doctor = data(run_cli(tmp_path, "doctor", "--run-id", run_id))
    assert doctor["ok"] is False
    assert any("lacks accepting verdict" in problem for problem in doctor["problems"])
