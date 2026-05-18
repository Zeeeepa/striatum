"""Tests for opt-in per-job git worktree isolation (RFC 0008)."""

from __future__ import annotations

import json
import os
import sqlite3
import subprocess
import sys
from copy import deepcopy
from pathlib import Path
from typing import Any, cast

ROOT = Path(__file__).resolve().parents[1]
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
        return {}
    payload = cast(JsonDict, json.loads(result.stdout))
    payload["returncode"] = result.returncode
    return payload


def data(payload: JsonDict) -> JsonDict:
    value = payload["data"]
    assert isinstance(value, dict)
    return cast(JsonDict, value)


def _git_init_repo(repo: Path, initial_branch: str = "main") -> None:
    """Initialize a git repo with one seed commit so checkouts are unambiguous."""
    subprocess.run(["git", "init"], cwd=repo, check=True, capture_output=True)
    subprocess.run(
        ["git", "checkout", "-b", initial_branch], cwd=repo, check=True, capture_output=True
    )
    subprocess.run(
        ["git", "config", "user.email", "test@example.com"],
        cwd=repo,
        check=True,
        capture_output=True,
    )
    subprocess.run(
        ["git", "config", "user.name", "Test"], cwd=repo, check=True, capture_output=True
    )
    seed = repo / ".gitseed"
    seed.write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", ".gitseed"], cwd=repo, check=True, capture_output=True)
    subprocess.run(
        ["git", "commit", "-m", "seed", "--no-gpg-sign"],
        cwd=repo,
        check=True,
        capture_output=True,
    )


def _docs_workflow_with_isolation() -> JsonDict:
    """Return a docs-review workflow with per-job worktree isolation on the lane."""
    workflow = cast(JsonDict, json.loads(DOCS_REVIEW_WORKFLOW.read_text(encoding="utf-8")))
    workflow = deepcopy(workflow)
    lanes = workflow["lanes"]
    assert isinstance(lanes, dict)
    lane = lanes["local"]
    assert isinstance(lane, dict)
    lane["worktree_isolation"] = "per_job"
    return workflow


def _docs_workflow_without_isolation() -> JsonDict:
    return cast(JsonDict, json.loads(DOCS_REVIEW_WORKFLOW.read_text(encoding="utf-8")))


def _write_workflow(path: Path, workflow: JsonDict) -> Path:
    path.write_text(json.dumps(workflow), encoding="utf-8")
    return path


def _prepare_run(repo: Path, workflow: JsonDict, *, branch: str = "feature/work") -> str:
    """Init the repo, prepare a docs-review run, confirm a branch, and start it."""
    _git_init_repo(repo, initial_branch="main")
    subprocess.run(
        ["git", "checkout", "-b", branch], cwd=repo, check=True, capture_output=True
    )
    from striatum.db import init_repo as legacy_init_repo

    legacy_init_repo(repo)
    workflow_path = _write_workflow(repo / "workflow.json", workflow)
    prepared = data(run_cli(repo, "run", "prepare", "--workflow", str(workflow_path)))
    run_id = str(prepared["run_id"])
    run_cli(
        repo,
        "branch",
        "confirm",
        "--run-id",
        run_id,
        "--branch",
        branch,
        "--use-current",
    )
    run_cli(repo, "run", "start", "--run-id", run_id)
    return run_id


def _register(repo: Path, run_id: str, role: str, lane: str) -> str:
    args: list[str] = [
        "register-session",
        "--run-id",
        run_id,
        "--role",
        role,
        "--lane",
        lane,
        "--capability",
        "write",
    ]
    # HARNESS-003: same-operator reviewer registration needs the
    # explicit override.
    if role == "reviewer":
        args += ["--force-non-fresh", "--reason", "test fixture"]
    payload = data(run_cli(repo, *args))
    return str(payload["session_id"])


def _claim(repo: Path, session_id: str) -> JsonDict:
    payload = data(run_cli(repo, "claim-next", "--session-id", session_id))
    assert payload["status"] == "claimed"
    packet = payload["packet"]
    assert isinstance(packet, dict)
    return cast(JsonDict, packet)


def _packet_ids(packet: JsonDict) -> tuple[str, str, str]:
    job = packet["job"]
    lease = packet["lease"]
    assert isinstance(job, dict)
    assert isinstance(lease, dict)
    return str(job["job_id"]), str(lease["message_id"]), str(lease["lease_id"])


def test_packet_includes_worktree_required_for_isolated_lane(tmp_path: Path) -> None:
    workflow = _docs_workflow_with_isolation()
    run_id = _prepare_run(tmp_path, workflow)
    author = _register(tmp_path, run_id, "author", "local")
    packet = _claim(tmp_path, author)

    assert packet["worktree_isolation"] == "per_job"
    assert packet["worktree_required"] is True
    commands = packet["commands"]
    assert isinstance(commands, dict)
    job_id, _, lease_id = _packet_ids(packet)
    expected_cmd = (
        f"striatum worktree create --session-id {author} "
        f"--job-id {job_id} --lease-id {lease_id}"
    )
    assert commands["worktree_create"] == expected_cmd


def test_default_lane_packet_does_not_require_worktree(tmp_path: Path) -> None:
    workflow = _docs_workflow_without_isolation()
    run_id = _prepare_run(tmp_path, workflow)
    author = _register(tmp_path, run_id, "author", "local")
    packet = _claim(tmp_path, author)

    assert packet["worktree_isolation"] == "off"
    assert packet["worktree_required"] is False
    commands = packet["commands"]
    assert isinstance(commands, dict)
    assert "worktree_create" not in commands


def test_worktree_create_records_row_and_creates_directory(tmp_path: Path) -> None:
    workflow = _docs_workflow_with_isolation()
    run_id = _prepare_run(tmp_path, workflow)
    author = _register(tmp_path, run_id, "author", "local")
    packet = _claim(tmp_path, author)
    job_id, message_id, lease_id = _packet_ids(packet)
    run_cli(
        tmp_path, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id
    )

    created = data(
        run_cli(
            tmp_path,
            "worktree",
            "create",
            "--session-id",
            author,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
        )
    )
    worktree_id = str(created["worktree_id"])
    relative = str(created["worktree_path"])
    assert relative == f".striatum/worktrees/{worktree_id}"
    assert created["base_branch"] == "feature/work"
    target = tmp_path / relative
    assert target.is_dir()
    assert (target / ".gitseed").exists()

    # Row should be in active state with the lease and run id from the claim.
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT run_id, job_id, lease_id, base_branch, worktree_path, state FROM job_worktrees WHERE worktree_id = ?",
            (worktree_id,),
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    assert row[0] == run_id
    assert row[1] == job_id
    assert row[2] == lease_id
    assert row[3] == "feature/work"
    assert row[4] == relative
    assert row[5] == "active"


def test_publish_artifact_uses_worktree_path_and_records_logical(tmp_path: Path) -> None:
    workflow = _docs_workflow_with_isolation()
    run_id = _prepare_run(tmp_path, workflow)
    author = _register(tmp_path, run_id, "author", "local")
    packet = _claim(tmp_path, author)
    job_id, message_id, lease_id = _packet_ids(packet)
    run_cli(
        tmp_path, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id
    )

    created = data(
        run_cli(
            tmp_path,
            "worktree",
            "create",
            "--session-id",
            author,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
        )
    )
    worktree_path = tmp_path / str(created["worktree_path"])
    logical_path = "docs/reviews/docs-review-flow/DOCS_DRAFT.md"
    file_in_worktree = worktree_path / logical_path
    file_in_worktree.parent.mkdir(parents=True, exist_ok=True)
    file_in_worktree.write_text("draft body\n", encoding="utf-8")

    # The same logical path does NOT exist in the main worktree on purpose:
    # the artifact publisher must read from the per-job worktree.
    assert not (tmp_path / logical_path).exists()

    artifact = data(
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
            logical_path,
        )
    )

    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT repo_path, content_sha256 FROM artifacts WHERE artifact_id = ?",
            (str(artifact["artifact_id"]),),
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    # Artifact records the LOGICAL path (not the worktree-relative path), so
    # it lines up with the run's main-branch provenance.
    assert row[0] == logical_path


def test_worktree_release_removes_directory_and_marks_state(tmp_path: Path) -> None:
    workflow = _docs_workflow_with_isolation()
    run_id = _prepare_run(tmp_path, workflow)
    author = _register(tmp_path, run_id, "author", "local")
    packet = _claim(tmp_path, author)
    job_id, message_id, lease_id = _packet_ids(packet)
    run_cli(
        tmp_path, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id
    )
    created = data(
        run_cli(
            tmp_path,
            "worktree",
            "create",
            "--session-id",
            author,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
        )
    )
    worktree_id = str(created["worktree_id"])
    target = tmp_path / str(created["worktree_path"])
    assert target.is_dir()

    released = data(run_cli(tmp_path, "worktree", "release", "--worktree-id", worktree_id))
    assert released["status"] == "released"
    assert released["state"] == "removed"
    assert not target.exists()

    # Idempotent: a second release returns success without erroring.
    again = data(run_cli(tmp_path, "worktree", "release", "--worktree-id", worktree_id))
    assert again["status"] == "already_released"
    assert again["state"] == "removed"

    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT state, removed_at FROM job_worktrees WHERE worktree_id = ?",
            (worktree_id,),
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    assert row[0] == "removed"
    assert row[1] is not None
    assert run_id


def test_worktree_list_returns_rows(tmp_path: Path) -> None:
    workflow = _docs_workflow_with_isolation()
    run_id = _prepare_run(tmp_path, workflow)
    author = _register(tmp_path, run_id, "author", "local")
    packet = _claim(tmp_path, author)
    job_id, message_id, lease_id = _packet_ids(packet)
    run_cli(
        tmp_path, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id
    )
    created = data(
        run_cli(
            tmp_path,
            "worktree",
            "create",
            "--session-id",
            author,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
        )
    )

    listing = data(run_cli(tmp_path, "worktree", "list", "--run-id", run_id))
    worktrees = listing["worktrees"]
    assert isinstance(worktrees, list)
    assert len(worktrees) == 1
    row = worktrees[0]
    assert row["worktree_id"] == created["worktree_id"]
    assert row["state"] == "active"
    assert row["workflow_job_id"] == "draft_docs"
    assert row["base_branch"] == "feature/work"


def test_lease_expiry_marks_worktree_abandoned(tmp_path: Path) -> None:
    workflow = _docs_workflow_with_isolation()
    run_id = _prepare_run(tmp_path, workflow)
    author = _register(tmp_path, run_id, "author", "local")
    packet = _claim(tmp_path, author)
    job_id, message_id, lease_id = _packet_ids(packet)
    run_cli(
        tmp_path, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id
    )
    created = data(
        run_cli(
            tmp_path,
            "worktree",
            "create",
            "--session-id",
            author,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
        )
    )
    worktree_id = str(created["worktree_id"])

    # Force the lease to look expired and trigger lazy expiry through any
    # mutation that calls expire_leases (recovery stale-leases is the most
    # explicit one).
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        conn.execute(
            "UPDATE leases SET expires_at = ? WHERE lease_id = ?",
            ("2000-01-01T00:00:00Z", lease_id),
        )
        conn.commit()
    finally:
        conn.close()
    run_cli(tmp_path, "recovery", "stale-leases", "--run-id", run_id)

    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT state FROM job_worktrees WHERE worktree_id = ?", (worktree_id,)
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    assert row[0] == "abandoned"

    # The directory itself must remain on disk for operator inspection.
    assert (tmp_path / str(created["worktree_path"])).is_dir()


def test_worktree_create_rejects_when_lane_not_isolated(tmp_path: Path) -> None:
    workflow = _docs_workflow_without_isolation()
    run_id = _prepare_run(tmp_path, workflow)
    author = _register(tmp_path, run_id, "author", "local")
    packet = _claim(tmp_path, author)
    job_id, message_id, lease_id = _packet_ids(packet)
    run_cli(
        tmp_path, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id
    )

    rejected = run_cli(
        tmp_path,
        "worktree",
        "create",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        check=False,
    )
    assert rejected["returncode"] == 4
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "worktree_isolation" in str(error["message"])


def test_worktree_create_rejects_non_repo_write_job(tmp_path: Path) -> None:
    workflow = _docs_workflow_with_isolation()
    run_id = _prepare_run(tmp_path, workflow)

    # Run the draft (repo_write) job to completion using the main worktree
    # so the review (review_only) job becomes claimable.
    author = _register(tmp_path, run_id, "author", "local")
    draft_packet = _claim(tmp_path, author)
    draft_job_id, draft_message_id, draft_lease_id = _packet_ids(draft_packet)
    run_cli(
        tmp_path,
        "ack",
        "--session-id",
        author,
        "--message-id",
        draft_message_id,
        "--lease-id",
        draft_lease_id,
    )
    draft_wt = data(
        run_cli(
            tmp_path,
            "worktree",
            "create",
            "--session-id",
            author,
            "--job-id",
            draft_job_id,
            "--lease-id",
            draft_lease_id,
        )
    )
    draft_path = "docs/reviews/docs-review-flow/DOCS_DRAFT.md"
    target_in_wt = tmp_path / str(draft_wt["worktree_path"]) / draft_path
    target_in_wt.parent.mkdir(parents=True, exist_ok=True)
    target_in_wt.write_text("draft\n", encoding="utf-8")
    run_cli(
        tmp_path,
        "publish-artifact",
        "--session-id",
        author,
        "--job-id",
        draft_job_id,
        "--lease-id",
        draft_lease_id,
        "--kind",
        "handoff",
        "--logical-name",
        "draft",
        "--path",
        draft_path,
    )
    run_cli(
        tmp_path,
        "complete",
        "--session-id",
        author,
        "--job-id",
        draft_job_id,
        "--lease-id",
        draft_lease_id,
    )
    run_cli(tmp_path, "worktree", "release", "--worktree-id", str(draft_wt["worktree_id"]))

    # Review job should now be claimable and is review_only (not repo_write).
    reviewer = _register(tmp_path, run_id, "reviewer", "local")
    review_packet = _claim(tmp_path, reviewer)
    job_id, message_id, lease_id = _packet_ids(review_packet)
    assert review_packet["worktree_required"] is False
    run_cli(
        tmp_path,
        "ack",
        "--session-id",
        reviewer,
        "--message-id",
        message_id,
        "--lease-id",
        lease_id,
    )

    rejected = run_cli(
        tmp_path,
        "worktree",
        "create",
        "--session-id",
        reviewer,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        check=False,
    )
    assert rejected["returncode"] == 4
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "repo-write" in str(error["message"])


def test_worktree_create_rejects_when_already_active(tmp_path: Path) -> None:
    workflow = _docs_workflow_with_isolation()
    run_id = _prepare_run(tmp_path, workflow)
    author = _register(tmp_path, run_id, "author", "local")
    packet = _claim(tmp_path, author)
    job_id, message_id, lease_id = _packet_ids(packet)
    run_cli(
        tmp_path, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id
    )
    run_cli(
        tmp_path,
        "worktree",
        "create",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
    )
    rejected = run_cli(
        tmp_path,
        "worktree",
        "create",
        "--session-id",
        author,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        check=False,
    )
    assert rejected["returncode"] == 4
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "already" in str(error["message"]).lower()
    assert run_id


def test_doctor_flags_orphaned_and_missing_worktrees(tmp_path: Path) -> None:
    workflow = _docs_workflow_with_isolation()
    run_id = _prepare_run(tmp_path, workflow)
    author = _register(tmp_path, run_id, "author", "local")
    packet = _claim(tmp_path, author)
    job_id, message_id, lease_id = _packet_ids(packet)
    run_cli(
        tmp_path, "ack", "--session-id", author, "--message-id", message_id, "--lease-id", lease_id
    )
    created = data(
        run_cli(
            tmp_path,
            "worktree",
            "create",
            "--session-id",
            author,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
        )
    )
    worktree_id = str(created["worktree_id"])
    target = tmp_path / str(created["worktree_path"])

    # Force the lease to look released without touching the worktree row,
    # so doctor sees an active worktree whose lease is no longer active.
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        conn.execute(
            "UPDATE leases SET state = 'released', released_at = ?, release_reason = 'test' WHERE lease_id = ?",
            ("2026-05-07T00:00:00Z", lease_id),
        )
        conn.commit()
    finally:
        conn.close()
    doctor = data(run_cli(tmp_path, "doctor", "--run-id", run_id))
    problems = doctor["problems"]
    assert isinstance(problems, list)
    assert any(
        problem == f"active worktree without active lease: {worktree_id}"
        for problem in problems
    )

    # Now simulate filesystem drift: remove the directory but keep the row
    # active. This is a separate doctor check.
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        conn.execute(
            "UPDATE leases SET state = 'active', released_at = NULL, release_reason = NULL WHERE lease_id = ?",
            (lease_id,),
        )
        conn.commit()
    finally:
        conn.close()
    subprocess.run(
        ["git", "worktree", "remove", "--force", str(target)],
        cwd=tmp_path,
        check=True,
        capture_output=True,
    )
    assert not target.exists()
    doctor2 = data(run_cli(tmp_path, "doctor", "--run-id", run_id))
    problems2 = doctor2["problems"]
    assert isinstance(problems2, list)
    assert any(
        f"active worktree directory missing on disk: {worktree_id}" in problem
        for problem in problems2
    )


def test_workflow_validate_rejects_unknown_worktree_isolation_value(tmp_path: Path) -> None:
    workflow = _docs_workflow_with_isolation()
    lanes = workflow["lanes"]
    assert isinstance(lanes, dict)
    lane = lanes["local"]
    assert isinstance(lane, dict)
    lane["worktree_isolation"] = "shared"  # not a valid choice
    workflow_path = _write_workflow(tmp_path / "workflow.json", workflow)
    rejected = run_cli(tmp_path, "workflow", "validate", str(workflow_path), check=False)
    assert rejected["returncode"] == 8
    error = rejected["error"]
    assert isinstance(error, dict)
    assert "worktree_isolation" in str(error["message"])
