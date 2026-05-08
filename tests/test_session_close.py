"""Acceptance coverage for RFC 0011 (session close + run-terminal auto-close)."""

from __future__ import annotations

import json
import os
import sqlite3
import subprocess
import sys
from pathlib import Path
from typing import Any, cast


ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"
DOCS_REVIEW_WORKFLOW = ROOT / "examples" / "docs-review-flow" / "workflow.json"


JsonDict = dict[str, Any]


def run_cli(repo: Path, *args: str, check: bool = True) -> JsonDict:
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
    if result.stdout.strip() == "":
        return {"returncode": result.returncode}
    payload = cast(JsonDict, json.loads(result.stdout))
    payload["returncode"] = result.returncode
    return payload


def data(payload: JsonDict) -> JsonDict:
    value = payload["data"]
    assert isinstance(value, dict)
    return cast(JsonDict, value)


def init_repo(repo: Path) -> None:
    run_cli(repo, "init")


def prepare_started_run(repo: Path, *, workflow: Path = DOCS_REVIEW_WORKFLOW) -> str:
    init_repo(repo)
    prepared = data(run_cli(repo, "run", "prepare", "--workflow", str(workflow)))
    run_id = str(prepared["run_id"])
    run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/rfc-0011-test")
    run_cli(repo, "run", "start", "--run-id", run_id)
    return run_id


def register_author(repo: Path, run_id: str, *, lane: str = "local") -> str:
    payload = data(
        run_cli(
            repo,
            "register-session",
            "--run-id",
            run_id,
            "--role",
            "author",
            "--lane",
            lane,
            "--capability",
            "write",
        )
    )
    return str(payload["session_id"])


def register_reviewer(repo: Path, run_id: str, *, lane: str = "local") -> str:
    payload = data(
        run_cli(
            repo,
            "register-session",
            "--run-id",
            run_id,
            "--role",
            "reviewer",
            "--lane",
            lane,
            "--capability",
            "review",
            "--force-non-fresh",
            "--reason",
            "test fixture",
        )
    )
    return str(payload["session_id"])


def session_row(repo: Path, session_id: str) -> sqlite3.Row | None:
    conn = sqlite3.connect(repo / ".striatum" / "state.sqlite3")
    try:
        conn.row_factory = sqlite3.Row
        row = conn.execute(
            "SELECT * FROM sessions WHERE session_id = ?", (session_id,)
        ).fetchone()
    finally:
        conn.close()
    return cast(sqlite3.Row | None, row)


def session_closed_events(repo: Path, *, session_id: str | None = None) -> list[JsonDict]:
    conn = sqlite3.connect(repo / ".striatum" / "state.sqlite3")
    try:
        rows = conn.execute(
            "SELECT payload_json FROM events WHERE event_type = 'session.closed' ORDER BY event_id"
        ).fetchall()
    finally:
        conn.close()
    out: list[JsonDict] = []
    for (payload_text,) in rows:
        payload = cast(JsonDict, json.loads(payload_text))
        if session_id is None or payload.get("session_id") == session_id:
            out.append(payload)
    return out


def write_artifact_with_byline(repo: Path, path: str, byline: str) -> None:
    target = repo / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(f"{byline}\n\nbody\n", encoding="utf-8")


def claim_ack(repo: Path, session_id: str) -> tuple[str, str, str, JsonDict]:
    payload = data(run_cli(repo, "claim-next", "--session-id", session_id))
    packet = payload["packet"]
    assert isinstance(packet, dict)
    job = packet["job"]
    lease = packet["lease"]
    job_id = str(job["job_id"])
    lease_id = str(lease["lease_id"])
    message_id = str(lease["message_id"])
    run_cli(
        repo,
        "ack",
        "--session-id",
        session_id,
        "--message-id",
        message_id,
        "--lease-id",
        lease_id,
    )
    return job_id, lease_id, message_id, packet


def packet_byline(packet: JsonDict, logical_name: str) -> str | None:
    expected = packet.get("expected_artifacts")
    if not isinstance(expected, list):
        return None
    for item in expected:
        if isinstance(item, dict) and item.get("logical_name") == logical_name:
            byline = item.get("author_line")
            if isinstance(byline, str) and byline.strip():
                return byline
    return None


# ---------------------------------------------------------------------------
# Test 1: explicit close happy path
# ---------------------------------------------------------------------------


def test_session_close_explicit_transitions_active_to_closed(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register_author(tmp_path, run_id)

    payload = data(
        run_cli(
            tmp_path,
            "session",
            "close",
            "--session-id",
            author,
            "--reason",
            "test explicit close",
        )
    )
    assert payload["state"] == "closed"
    assert payload["close_reason"] == "test explicit close"
    assert "closed_at" in payload and payload["closed_at"]
    assert "note" not in payload

    row = session_row(tmp_path, author)
    assert row is not None
    assert row["state"] == "closed"
    assert row["closed_at"] == payload["closed_at"]
    assert row["close_reason"] == "test explicit close"

    events = session_closed_events(tmp_path, session_id=author)
    assert len(events) == 1
    assert events[0]["source"] == "explicit"
    assert events[0]["reason"] == "test explicit close"


# ---------------------------------------------------------------------------
# Test 2: idempotency
# ---------------------------------------------------------------------------


def test_session_close_idempotent_against_already_closed(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register_author(tmp_path, run_id)
    first = data(
        run_cli(
            tmp_path,
            "session",
            "close",
            "--session-id",
            author,
            "--reason",
            "first",
        )
    )

    second = data(
        run_cli(
            tmp_path,
            "session",
            "close",
            "--session-id",
            author,
            "--reason",
            "second",
        )
    )
    assert second["state"] == "closed"
    assert second.get("note") == "session was already closed"
    # Closed_at and reason from the first call are preserved.
    assert second["closed_at"] == first["closed_at"]
    assert second["close_reason"] == "first"

    events = session_closed_events(tmp_path, session_id=author)
    # Idempotent path does not double-emit the event.
    assert len(events) == 1


# ---------------------------------------------------------------------------
# Test 3: refusal when an active lease is held
# ---------------------------------------------------------------------------


def test_session_close_refuses_when_active_lease_held(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register_author(tmp_path, run_id)

    # Claim a packet — this acquires an active lease for the author session.
    data(run_cli(tmp_path, "claim-next", "--session-id", author))

    refused = run_cli(
        tmp_path,
        "session",
        "close",
        "--session-id",
        author,
        "--reason",
        "trying to close mid-claim",
        check=False,
    )
    assert refused["returncode"] != 0
    error = refused.get("error", {})
    assert isinstance(error, dict)
    message = str(error.get("message", ""))
    assert "active lease" in message
    assert "release" in message  # message points operator at the right command

    row = session_row(tmp_path, author)
    assert row is not None
    assert row["state"] == "active"  # session was not transitioned


# ---------------------------------------------------------------------------
# Test 4: auto-close on run-completed transition
# ---------------------------------------------------------------------------


def test_run_complete_auto_closes_active_sessions(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register_author(tmp_path, run_id)
    reviewer = register_reviewer(tmp_path, run_id)

    # Drive the docs-review-flow draft → review → apply lifecycle.
    # 1. Draft
    job_id, lease_id, _msg, packet = claim_ack(tmp_path, author)
    byline = packet_byline(packet, "draft")
    if byline is not None:
        write_artifact_with_byline(
            tmp_path, "docs/reviews/docs-review-flow/DOCS_DRAFT.md", byline
        )
    else:
        target = tmp_path / "docs/reviews/docs-review-flow/DOCS_DRAFT.md"
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text("body\n", encoding="utf-8")
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
        "docs/reviews/docs-review-flow/DOCS_DRAFT.md",
    )
    run_cli(tmp_path, "complete", "--session-id", author, "--job-id", job_id, "--lease-id", lease_id)

    # 2. Review
    review_job_id, review_lease_id, _msg, review_packet = claim_ack(tmp_path, reviewer)
    review_byline = packet_byline(review_packet, "review")
    review_path = "docs/reviews/docs-review-flow/review/DOCS_REVIEW.md"
    if review_byline is not None:
        write_artifact_with_byline(tmp_path, review_path, review_byline)
    else:
        target = tmp_path / review_path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text("body\n", encoding="utf-8")
    run_cli(
        tmp_path,
        "submit-review",
        "--session-id",
        reviewer,
        "--job-id",
        review_job_id,
        "--lease-id",
        review_lease_id,
        "--path",
        review_path,
        "--kind",
        "finding",
        "--logical-name",
        "review",
        "--verdict",
        "accept",
    )

    # 3. Apply
    apply_job_id, apply_lease_id, _msg, apply_packet = claim_ack(tmp_path, author)
    apply_byline = packet_byline(apply_packet, "summary")
    apply_path = "docs/reviews/docs-review-flow/DOCS_SUMMARY.md"
    if apply_byline is not None:
        write_artifact_with_byline(tmp_path, apply_path, apply_byline)
    else:
        target = tmp_path / apply_path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text("body\n", encoding="utf-8")
    run_cli(
        tmp_path,
        "publish-artifact",
        "--session-id",
        author,
        "--job-id",
        apply_job_id,
        "--lease-id",
        apply_lease_id,
        "--kind",
        "synthesis",
        "--logical-name",
        "summary",
        "--path",
        apply_path,
    )
    run_cli(tmp_path, "complete", "--session-id", author, "--job-id", apply_job_id, "--lease-id", apply_lease_id)

    # Run should be completed and both sessions auto-closed with run_completed source.
    status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert status["runs"][0]["state"] == "completed"

    author_row = session_row(tmp_path, author)
    reviewer_row = session_row(tmp_path, reviewer)
    assert author_row is not None and author_row["state"] == "closed"
    assert reviewer_row is not None and reviewer_row["state"] == "closed"
    assert author_row["close_reason"] == "run_completed"
    assert reviewer_row["close_reason"] == "run_completed"

    events = session_closed_events(tmp_path)
    sources = [e["source"] for e in events]
    assert sources.count("run_completed") == 2


# ---------------------------------------------------------------------------
# Test 5: auto-close on run-canceled (via recovery cancel-job cascade)
# ---------------------------------------------------------------------------


def test_run_canceled_auto_closes_active_sessions(tmp_path: Path) -> None:
    """When ``recovery cancel-job --cascade`` cancels every remaining job
    on a run, ``maybe_complete_run`` flips the run to ``completed`` (the
    schema does not yet have a separate canceled-run path) and auto-close
    fires. Verify auto-close runs end-to-end through the cancel path."""
    run_id = prepare_started_run(tmp_path)
    author = register_author(tmp_path, run_id)
    register_reviewer(tmp_path, run_id)

    # Find the queued draft job and cancel it with --cascade so all
    # downstream jobs get canceled too.
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT job_id FROM jobs WHERE run_id = ? AND state = 'queued' ORDER BY workflow_job_id LIMIT 1",
            (run_id,),
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    target_job_id = str(row[0])

    run_cli(
        tmp_path,
        "recovery",
        "cancel-job",
        "--run-id",
        run_id,
        "--job-id",
        target_job_id,
        "--reason",
        "test cancel",
        "--cascade",
    )

    # All jobs are now canceled; the run advances to ``completed`` because
    # ``maybe_complete_run`` treats canceled jobs as terminal. Auto-close
    # fires under source="run_completed" — that's the current behavior.
    # When a dedicated canceled-run path is added later, the source will
    # become "run_canceled"; the test will need updating then.
    status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert status["runs"][0]["state"] in {"completed", "canceled"}

    author_row = session_row(tmp_path, author)
    assert author_row is not None and author_row["state"] == "closed"


# ---------------------------------------------------------------------------
# Test 6: doctor cleared after auto-close
# ---------------------------------------------------------------------------


def test_doctor_no_longer_flags_terminal_run_after_auto_close(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register_author(tmp_path, run_id)
    register_reviewer(tmp_path, run_id)

    # Cancel everything (cheaper than driving the full lifecycle).
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT job_id FROM jobs WHERE run_id = ? AND state = 'queued' ORDER BY workflow_job_id LIMIT 1",
            (run_id,),
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    run_cli(
        tmp_path,
        "recovery",
        "cancel-job",
        "--run-id",
        run_id,
        "--job-id",
        str(row[0]),
        "--reason",
        "test cancel",
        "--cascade",
    )

    report = data(run_cli(tmp_path, "doctor", "--run-id", run_id, "--verbose"))
    problems = report.get("problems", [])
    assert all(
        "active session on terminal run" not in str(p) for p in problems
    ), f"unexpected active-session warning: {problems}"
    records = report.get("problem_records", [])
    assert all(
        r.get("check") != "active_session_on_terminal_run" for r in records
    )
    # The author session was closed by auto-close.
    author_row = session_row(tmp_path, author)
    assert author_row is not None and author_row["state"] == "closed"


# ---------------------------------------------------------------------------
# Test 7: run summary renders closed state and reason
# ---------------------------------------------------------------------------


def test_evidence_summary_renders_closed_state_and_reason(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register_author(tmp_path, run_id)
    register_reviewer(tmp_path, run_id)

    run_cli(
        tmp_path,
        "session",
        "close",
        "--session-id",
        author,
        "--reason",
        "explicit test close",
    )

    run_cli(
        tmp_path,
        "run",
        "summary",
        "--run-id",
        run_id,
        "--path",
        "docs/reviews/docs-review-flow/RUN_SUMMARY.md",
    )
    summary = (tmp_path / "docs/reviews/docs-review-flow/RUN_SUMMARY.md").read_text(
        encoding="utf-8"
    )
    assert "## Sessions" in summary
    assert "`closed`" in summary
    assert "explicit test close" in summary

    # Evidence export carries the new sessions block too.
    run_cli(
        tmp_path,
        "evidence",
        "export",
        "--run-id",
        run_id,
        "--path",
        "docs/reviews/docs-review-flow/EVIDENCE.md",
    )
    evidence = (tmp_path / "docs/reviews/docs-review-flow/EVIDENCE.md").read_text(
        encoding="utf-8"
    )
    assert "explicit test close" in evidence
