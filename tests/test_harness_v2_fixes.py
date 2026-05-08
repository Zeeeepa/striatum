"""Coverage for the dogfood-001 v2 HARNESS-001/002/003/004 fixes."""

from __future__ import annotations

import json
import os
import re
import sqlite3
import subprocess
import sys
from pathlib import Path
from typing import Any, cast


ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"


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


def prepare_started_run(repo: Path, *, workflow: Path = WORKFLOW) -> str:
    init_repo(repo)
    prepared = data(run_cli(repo, "run", "prepare", "--workflow", str(workflow)))
    run_id = str(prepared["run_id"])
    run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/v2-test")
    run_cli(repo, "run", "start", "--run-id", run_id)
    return run_id


def register_author(repo: Path, run_id: str) -> str:
    payload = data(
        run_cli(
            repo,
            "register-session",
            "--run-id",
            run_id,
            "--role",
            "author",
            "--lane",
            "codex",
            "--capability",
            "write",
        )
    )
    return str(payload["session_id"])


def fabricate_lost_supervisor_with_active_lease(
    repo: Path, *, run_id: str, session_id: str
) -> tuple[str, str]:
    """Insert a process_supervisors row in state ``lost`` plus an active lease.

    Returns ``(supervisor_id, lease_id)``. Used by the HARNESS-001 doctor
    and status-next-action tests; avoids the cost of spawning a real
    process and killing it.
    """
    conn = sqlite3.connect(repo / ".striatum" / "state.sqlite3")
    try:
        supervisor_id = "sup_test_lost"
        lease_id = "lease_test_active"
        # Pick the first job for this run to satisfy lease.resource_id.
        job_row = conn.execute(
            "SELECT job_id FROM jobs WHERE run_id = ? LIMIT 1",
            (run_id,),
        ).fetchone()
        assert job_row is not None
        job_id = str(job_row[0])
        conn.execute(
            """
            INSERT INTO process_supervisors (
              supervisor_id, run_id, session_id, adapter, command_json,
              cwd, scratch_path, stdin_pipe_path, pid, state, started_at,
              ended_at, stop_reason
            )
            VALUES (?, ?, ?, 'process', '[]', ?, '/tmp', NULL, NULL, 'lost',
                    '2026-05-08T00:00:00Z', '2026-05-08T00:00:01Z',
                    'pid is gone observed by status')
            """,
            (supervisor_id, run_id, session_id, str(repo)),
        )
        conn.execute(
            """
            INSERT INTO leases (
              lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at
            )
            VALUES (?, ?, 'job', ?, ?, 'active', '2026-05-08T00:00:00Z',
                    '2099-01-01T00:00:00Z')
            """,
            (lease_id, run_id, job_id, session_id),
        )
        conn.commit()
    finally:
        conn.close()
    return supervisor_id, lease_id


# ---------------------------------------------------------------------------
# HARNESS-001 — supervised-lane diagnostics + idempotent stop
# ---------------------------------------------------------------------------


def test_doctor_flags_supervisor_lost_with_held_lease(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register_author(tmp_path, run_id)
    supervisor_id, lease_id = fabricate_lost_supervisor_with_active_lease(
        tmp_path, run_id=run_id, session_id=author
    )

    report = data(run_cli(tmp_path, "doctor", "--verbose"))
    assert report["ok"] is False
    records = report["problem_records"]
    assert isinstance(records, list)
    matching = [r for r in records if r["check"] == "supervisor_lost_with_held_lease"]
    assert len(matching) == 1
    record = matching[0]
    assert record["id"] == supervisor_id
    assert record["context"]["lease_id"] == lease_id
    assert record["context"]["session_id"] == author


def test_status_surfaces_recover_orphan_supervisor_next_action(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register_author(tmp_path, run_id)
    fabricate_lost_supervisor_with_active_lease(
        tmp_path, run_id=run_id, session_id=author
    )

    status = data(run_cli(tmp_path, "status", "--run-id", run_id))
    actions = status["next_actions"]
    assert isinstance(actions, list)
    assert "recover_orphan_supervisor" in actions


def test_supervise_stop_is_idempotent_when_supervisor_already_lost(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register_author(tmp_path, run_id)
    fabricate_lost_supervisor_with_active_lease(
        tmp_path, run_id=run_id, session_id=author
    )

    payload = run_cli(
        tmp_path,
        "supervise",
        "stop",
        "--session-id",
        author,
        "--reason",
        "test stop",
    )
    assert payload["returncode"] == 0
    body = data(payload)
    assert body["state"] == "stopped"
    assert "supervisor was already" in str(body.get("note", ""))


# ---------------------------------------------------------------------------
# HARNESS-002 — editable-install guards
# ---------------------------------------------------------------------------


def test_doctor_flags_editable_install_outside_repo(tmp_path: Path) -> None:
    """``doctor`` warns when the repo arg is a Striatum source tree but the
    running install lives elsewhere.

    Simulated by creating a fake ``src/striatum/migrations.py`` inside
    ``tmp_path``. The CLI is still importing the canonical install at
    ``ROOT``; from the perspective of ``--repo tmp_path`` that install
    is outside the repo, so the new check fires.
    """
    fake_migrations = tmp_path / "src" / "striatum" / "migrations.py"
    fake_migrations.parent.mkdir(parents=True, exist_ok=True)
    fake_migrations.write_text("LATEST_VERSION = 1\n", encoding="utf-8")
    init_repo(tmp_path)

    report = data(run_cli(tmp_path, "doctor", "--verbose"))
    records = report["problem_records"]
    assert isinstance(records, list)
    matching = [r for r in records if r["check"] == "editable_install_outside_repo"]
    assert len(matching) == 1
    assert "install_path" in matching[0]["context"]
    assert "repo_path" in matching[0]["context"]


def test_init_refuses_when_install_lags_repo_migrations(tmp_path: Path) -> None:
    """``init`` against a fresh DB refuses when the repo's source tree has a
    higher ``LATEST_VERSION`` than the running install."""
    # Set the fake repo's source tree to LATEST_VERSION=999 so the running
    # install (currently 6) is guaranteed to be lower.
    fake_migrations = tmp_path / "src" / "striatum" / "migrations.py"
    fake_migrations.parent.mkdir(parents=True, exist_ok=True)
    fake_migrations.write_text(
        "Migration = lambda **kwargs: kwargs\n"
        "LATEST_VERSION = 999\n"
        "MIGRATIONS = [Migration(version=999)]\n",
        encoding="utf-8",
    )
    payload = run_cli(tmp_path, "init", check=False)
    assert payload["returncode"] != 0
    error = payload.get("error", {})
    assert isinstance(error, dict)
    message = str(error.get("message", ""))
    assert "older than the repo source tree" in message
    assert "pip install -e" in message


# ---------------------------------------------------------------------------
# HARNESS-003 — reviewer-independence advisory + register-session policy
# ---------------------------------------------------------------------------


def test_register_session_refuses_fresh_reviewer_without_force(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    register_author(tmp_path, run_id)

    refused = run_cli(
        tmp_path,
        "register-session",
        "--run-id",
        run_id,
        "--role",
        "reviewer",
        "--lane",
        "codex",
        "--capability",
        "review",
        check=False,
    )
    assert refused["returncode"] != 0
    error = refused.get("error", {})
    assert "fresh" in str(error.get("message", ""))

    # --force-non-fresh without --reason still refuses.
    refused2 = run_cli(
        tmp_path,
        "register-session",
        "--run-id",
        run_id,
        "--role",
        "reviewer",
        "--lane",
        "codex",
        "--capability",
        "review",
        "--force-non-fresh",
        check=False,
    )
    assert refused2["returncode"] != 0
    assert "non-empty --reason" in str(refused2.get("error", {}).get("message", ""))

    # With both --force-non-fresh and --reason, registration succeeds and
    # the reason is recorded on the session row.
    payload = data(
        run_cli(
            tmp_path,
            "register-session",
            "--run-id",
            run_id,
            "--role",
            "reviewer",
            "--lane",
            "codex",
            "--capability",
            "review",
            "--force-non-fresh",
            "--reason",
            "operator drove both lanes; HARNESS-001 not yet shipped",
        )
    )
    session_id = str(payload["session_id"])

    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT non_fresh_reason FROM sessions WHERE session_id = ?",
            (session_id,),
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    assert row[0] is not None
    assert "HARNESS-001" in str(row[0])


def test_publish_artifact_records_missing_author_line(tmp_path: Path) -> None:
    """An artifact published without an ``author:`` line records NULL in
    ``artifacts.author_line``; evidence renders ``author: <missing>``."""
    run_id = prepare_started_run(tmp_path)
    author = register_author(tmp_path, run_id)

    # Claim the first work packet for the author.
    claim_payload = data(run_cli(tmp_path, "claim-next", "--session-id", author))
    packet = claim_payload["packet"]
    assert isinstance(packet, dict)
    job = packet["job"]
    lease = packet["lease"]
    assert isinstance(job, dict)
    assert isinstance(lease, dict)
    job_id = str(job["job_id"])
    lease_id = str(lease["lease_id"])
    message_id = str(lease["message_id"])

    run_cli(
        tmp_path,
        "ack",
        "--session-id",
        author,
        "--message-id",
        message_id,
        "--lease-id",
        lease_id,
    )

    artifact_path = "docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md"
    target = tmp_path / artifact_path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text("plain body without an author line\n", encoding="utf-8")

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
        artifact_path,
    )

    # SQLite column is NULL.
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT author_line FROM artifacts WHERE job_id = ? AND logical_name = 'draft'",
            (job_id,),
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    assert row[0] is None

    # Run summary renders the missing case explicitly.
    run_cli(
        tmp_path,
        "run",
        "summary",
        "--run-id",
        run_id,
        "--path",
        "docs/reviews/rfc-ledger/RUN_SUMMARY.md",
    )
    summary = (tmp_path / "docs/reviews/rfc-ledger/RUN_SUMMARY.md").read_text(encoding="utf-8")
    assert "<missing>" in summary


# ---------------------------------------------------------------------------
# HARNESS-004 — reviewer role doc paths match write_scope
# ---------------------------------------------------------------------------


_FILE_UNDER_RE = re.compile(
    r"`(docs/dogfood/[^`]+/HARNESS-[^`]+\.md)`",
    re.IGNORECASE,
)


def test_reviewer_role_doc_paths_match_write_scope() -> None:
    """Every dogfood reviewer role doc that points at a HARNESS-NNN.md
    path must point inside the corresponding review job's
    ``write_scope.allowed_paths``.

    This is the regression test for HARNESS-004. The role doc and the
    workflow's review write_scope must agree about where reviewer
    harness proposals land.
    """
    dogfood_root = ROOT / "docs" / "dogfood"
    reviewer_docs = sorted(dogfood_root.glob("*/roles/reviewer.md"))
    assert reviewer_docs, "expected at least one dogfood reviewer role doc"
    for reviewer_doc in reviewer_docs:
        text = reviewer_doc.read_text(encoding="utf-8")
        # Find paths pointed at by the doc that look like reviewer-side
        # harness proposal targets (a HARNESS-NNN.md file under a
        # docs/dogfood/<id>/... path).
        matches = _FILE_UNDER_RE.findall(text)
        if not matches:
            continue
        workflow_path = reviewer_doc.parent.parent / "workflow.json"
        assert workflow_path.is_file(), (
            f"missing workflow.json sibling for {reviewer_doc}"
        )
        workflow = json.loads(workflow_path.read_text(encoding="utf-8"))
        review_scopes: list[list[str]] = []
        for job in workflow.get("jobs", []):
            if job.get("type") != "review":
                continue
            scope = job.get("write_scope", {})
            paths = scope.get("allowed_paths", [])
            if isinstance(paths, list):
                review_scopes.append([str(p) for p in paths])
        assert review_scopes, (
            f"{reviewer_doc} has reviewer role but no review job in {workflow_path}"
        )
        for path in matches:
            # The doc is allowed to mention the author-side findings
            # path explicitly when it is contrasting it with the
            # reviewer-side path; only require that any path mentioned
            # in a "file under" or "file at" instruction sentence is
            # inside at least one review scope.
            if "/findings/" in path and any("/review/" in p for p in path_paragraph(text, path)):
                # Author-side path mentioned only as a contrast — fine.
                continue
            assert any(
                path.startswith(allowed) for scope in review_scopes for allowed in scope
            ), f"{reviewer_doc} points at {path} which is outside any review write_scope: {review_scopes}"


def path_paragraph(text: str, needle: str) -> list[str]:
    """Return all single-paragraph slices of ``text`` containing ``needle``."""
    paragraphs = re.split(r"\n\s*\n", text)
    return [p for p in paragraphs if needle in p]
