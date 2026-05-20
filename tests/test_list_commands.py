# ruff: noqa
from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any, cast

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"
DOCS_REVIEW_WORKFLOW = ROOT / "examples" / "docs-review-flow" / "workflow.json"


JsonDict = dict[str, Any]


def run_cli(repo: Path, *args: str, check: bool = True) -> JsonDict:
    """Invoke the CLI in ``repo`` with ``--json`` and parse the envelope."""
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
    payload: JsonDict = {}
    if result.stdout.strip() != "":
        loaded = json.loads(result.stdout)
        assert isinstance(loaded, dict)
        payload = cast(JsonDict, loaded)
    payload["returncode"] = result.returncode
    payload["stderr"] = result.stderr
    return payload


def data(payload: JsonDict) -> JsonDict:
    value = payload["data"]
    assert isinstance(value, dict)
    return cast(JsonDict, value)


def items(payload: JsonDict) -> list[JsonDict]:
    inner = data(payload)
    assert "items" in inner and "count" in inner
    rows = inner["items"]
    assert isinstance(rows, list)
    assert inner["count"] == len(rows)
    return cast(list[JsonDict], rows)


def init_repo(repo: Path) -> None:
    from striatum.legacy_sqlite.db import init_repo as legacy_init_repo

    legacy_init_repo(repo)


def prepare_started_run(repo: Path, *, workflow: Path = WORKFLOW) -> str:
    """Initialize, prepare, branch-confirm, and start a run."""
    init_repo(repo)
    prepared = data(run_cli(repo, "run", "prepare", "--workflow", str(workflow)))
    run_id = str(prepared["run_id"])
    run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/list-test")
    run_cli(repo, "run", "start", "--run-id", run_id)
    return run_id


def register(repo: Path, run_id: str, role: str, lane: str) -> str:
    args: list[str] = [
        "register-session",
        "--run-id",
        run_id,
        "--role",
        role,
        "--lane",
        lane,
        "--capability",
        "review",
    ]
    # HARNESS-003: tests that drive both lanes from the same operator
    # use the explicit override.
    if role == "reviewer":
        args += ["--force-non-fresh", "--reason", "test fixture"]
    payload = data(run_cli(repo, *args))
    return str(payload["session_id"])


def claim(repo: Path, session_id: str) -> JsonDict:
    payload = data(run_cli(repo, "claim-next", "--session-id", session_id))
    assert payload["status"] == "claimed"
    packet = payload["packet"]
    assert isinstance(packet, dict)
    return cast(JsonDict, packet)


def packet_ids(packet: JsonDict) -> tuple[str, str, str]:
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
    packet: JsonDict,
    *,
    logical_name: str,
    kind: str,
    path: str,
) -> None:
    job_id, message_id, lease_id = packet_ids(packet)
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
    run_cli(
        repo,
        "complete",
        "--session-id",
        session_id,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
    )


def verdict_claimed_review(
    repo: Path,
    session_id: str,
    packet: JsonDict,
    *,
    verdict: str,
    path: str,
) -> JsonDict:
    job_id, message_id, lease_id = packet_ids(packet)
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
            "finding",
            "--logical-name",
            "review",
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


# --------------------------------------------------------------------------- #
# tests                                                                        #
# --------------------------------------------------------------------------- #


def test_list_runs_returns_runs_with_workflow_id(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    rows = items(run_cli(tmp_path, "list", "runs"))
    assert len(rows) == 1
    row = rows[0]
    assert row["run_id"] == run_id
    assert row["workflow_id"] == "rfc-ledger-cleanup"
    assert row["workflow_version"] == "2026-05-06"
    assert row["workflow_snapshot_id"]
    assert row["workflow_identity"] == {
        "workflow_id": "rfc-ledger-cleanup",
        "workflow_version": "2026-05-06",
        "workflow_snapshot_id": row["workflow_snapshot_id"],
    }
    assert row["state"] == "running"
    assert row["branch_name"] == "striatum/list-test"
    assert "created_at" in row and "started_at" in row and "completed_at" in row


def test_list_sessions_filters_by_role_and_lane(tmp_path: Path) -> None:
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
    register(tmp_path, run_id, "reviewer", "codex")

    all_sessions = items(run_cli(tmp_path, "list", "sessions", "--run-id", run_id))
    roles = sorted({row["role_id"] for row in all_sessions})
    assert roles == ["author", "reviewer"]

    only_author = items(
        run_cli(tmp_path, "list", "sessions", "--run-id", run_id, "--role", "author")
    )
    assert len(only_author) == 1
    assert only_author[0]["session_id"] == author
    assert only_author[0]["role_id"] == "author"
    assert only_author[0]["lane_id"] == "codex"
    assert isinstance(only_author[0]["capabilities"], list)
    assert "review" in only_author[0]["capabilities"]
    # ``capabilities_json`` is parsed away; raw JSON should not leak through.
    assert "capabilities_json" not in only_author[0]

    only_codex_reviewer = items(
        run_cli(
            tmp_path,
            "list",
            "sessions",
            "--run-id",
            run_id,
            "--role",
            "reviewer",
            "--lane",
            "codex",
        )
    )
    assert len(only_codex_reviewer) == 1
    assert only_codex_reviewer[0]["role_id"] == "reviewer"
    assert only_codex_reviewer[0]["lane_id"] == "codex"


def test_list_jobs_includes_latest_verdict_for_review_jobs(tmp_path: Path) -> None:
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
    verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="accept_with_findings",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )

    rows = items(
        run_cli(
            tmp_path,
            "list",
            "jobs",
            "--run-id",
            run_id,
            "--workflow-job-id",
            "review_codex",
        )
    )
    assert len(rows) == 1
    job_row = rows[0]
    assert job_row["workflow_job_id"] == "review_codex"
    assert job_row["job_type"] == "review"
    assert job_row["verdict"] == "accept_with_findings"
    assert job_row["lane_id"] == "codex"
    # ``lane_selector_json`` should not bleed through.
    assert "lane_selector_json" not in job_row

    # Non-review job should have ``verdict`` = None.
    draft_rows = items(
        run_cli(
            tmp_path,
            "list",
            "jobs",
            "--run-id",
            run_id,
            "--workflow-job-id",
            "draft",
        )
    )
    assert len(draft_rows) == 1
    assert draft_rows[0]["job_type"] == "draft"
    assert draft_rows[0]["verdict"] is None


def test_list_artifacts_filters_by_kind(tmp_path: Path) -> None:
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
    verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="accept",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )

    all_rows = items(run_cli(tmp_path, "list", "artifacts", "--run-id", run_id))
    kinds = sorted({row["artifact_kind"] for row in all_rows})
    assert "handoff" in kinds and "finding" in kinds

    finding_rows = items(
        run_cli(tmp_path, "list", "artifacts", "--run-id", run_id, "--kind", "finding")
    )
    assert len(finding_rows) == 1
    found = finding_rows[0]
    assert found["artifact_kind"] == "finding"
    assert found["logical_name"] == "review"
    assert "content_sha256" in found and "size_bytes" in found
    # Author identity from the evidence helper should be embedded.
    author_block = found.get("author")
    assert isinstance(author_block, dict)
    assert author_block.get("role_id") == "reviewer"


def test_list_workflows_returns_loaded_snapshots(tmp_path: Path) -> None:
    init_repo(tmp_path)
    first = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    second = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(DOCS_REVIEW_WORKFLOW)))
    assert first["run_id"] != second["run_id"]

    rows = items(run_cli(tmp_path, "list", "workflows"))
    workflow_ids = {row["workflow_id"] for row in rows}
    assert workflow_ids == {"rfc-ledger-cleanup", "docs-review-flow"}
    for row in rows:
        assert isinstance(row.get("content_sha256"), str)
        assert len(row["content_sha256"]) == 64


def test_list_state_filter_rejects_unknown_value(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    payload = run_cli(
        tmp_path,
        "list",
        "jobs",
        "--run-id",
        run_id,
        "--state",
        "bogus",
        check=False,
    )
    assert payload["returncode"] == 2
    assert payload["ok"] is False
    error = payload["error"]
    assert isinstance(error, dict)
    assert "bogus" in error["message"]
    # The error should cite at least one valid job state.
    assert "queued" in error["message"]


def test_list_runs_limit_caps_results(tmp_path: Path) -> None:
    init_repo(tmp_path)
    run_ids: list[str] = []
    for _ in range(3):
        prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
        run_ids.append(str(prepared["run_id"]))
    assert len(set(run_ids)) == 3

    capped = items(run_cli(tmp_path, "list", "runs", "--limit", "2"))
    assert len(capped) == 2

    uncapped = items(run_cli(tmp_path, "list", "runs"))
    assert len(uncapped) == 3
