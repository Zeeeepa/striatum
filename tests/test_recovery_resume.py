# ruff: noqa
"""Recovery resume coverage for process-adapter blockers."""

from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import json
from pathlib import Path
from typing import Any, cast

from striatum.legacy_sqlite.db import connect
from test_process_adapter import (
    _build_workflow,
    _data,
    _job_state,
    _open_blocker,
    _publish_artifact,
    _run_cli,
    _start_with,
)

JsonDict = dict[str, Any]


def _lease_state(repo: Path, lease_id: str) -> str:
    with connect(repo) as conn:
        row = conn.execute(
            "SELECT state FROM leases WHERE lease_id = ?",
            (lease_id,),
        ).fetchone()
    assert row is not None
    return str(row[0])


def _event_payload(repo: Path, event_type: str) -> JsonDict:
    with connect(repo) as conn:
        row = conn.execute(
            """
            SELECT payload_json FROM events
            WHERE event_type = ?
            ORDER BY created_at DESC, event_id DESC
            LIMIT 1
            """,
            (event_type,),
        ).fetchone()
    assert row is not None
    payload = json.loads(str(row[0]))
    assert isinstance(payload, dict)
    return cast(JsonDict, payload)


def test_recovery_resume_refuses_until_required_artifact_present(
    tmp_path: Path,
) -> None:
    workflow = _build_workflow(command=["bash", "-c", "exit 0"])
    _run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    _data(
        _run_cli(
            tmp_path,
            "adapter",
            "run",
            "--session-id",
            session_id,
            "--lease-id",
            lease_id,
            "--stdin",
            "none",
        )
    )
    blocker = _open_blocker(tmp_path, job_id)
    assert blocker is not None

    failed = _run_cli(
        tmp_path,
        "recovery",
        "resume",
        "--blocker-id",
        str(blocker["blocker_id"]),
        check=False,
    )

    assert failed["returncode"] == 4
    assert "still has missing required artifacts" in failed["error"]["message"]
    assert _job_state(tmp_path, job_id) == "blocked"
    assert _open_blocker(tmp_path, job_id) is not None


def test_recovery_resume_resolves_outputs_missing_and_preserves_lease(
    tmp_path: Path,
) -> None:
    workflow = _build_workflow(command=["bash", "-c", "exit 0"])
    _run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    _data(
        _run_cli(
            tmp_path,
            "adapter",
            "run",
            "--session-id",
            session_id,
            "--lease-id",
            lease_id,
            "--stdin",
            "none",
        )
    )
    blocker = _open_blocker(tmp_path, job_id)
    assert blocker is not None
    _publish_artifact(tmp_path, session_id, job_id, lease_id, "docs/out/OUT.md")

    result = _data(
        _run_cli(
            tmp_path,
            "recovery",
            "resume",
            "--blocker-id",
            str(blocker["blocker_id"]),
        )
    )

    assert result["status"] == "resumed"
    assert result["job_id"] == job_id
    assert result["lease_id"] == lease_id
    assert result["completed_inline"] is False
    assert _job_state(tmp_path, job_id) == "running"
    assert _open_blocker(tmp_path, job_id) is None
    assert _lease_state(tmp_path, lease_id) == "active"
    event_payload = _event_payload(tmp_path, "recovery.process_blocker_resolved")
    assert event_payload["blocker_id"] == blocker["blocker_id"]
    assert event_payload["completed_inline"] is False


def test_recovery_resume_complete_finishes_remediated_job(tmp_path: Path) -> None:
    workflow = _build_workflow(command=["bash", "-c", "exit 0"])
    _run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    _data(
        _run_cli(
            tmp_path,
            "adapter",
            "run",
            "--session-id",
            session_id,
            "--lease-id",
            lease_id,
            "--stdin",
            "none",
        )
    )
    blocker = _open_blocker(tmp_path, job_id)
    assert blocker is not None
    _publish_artifact(tmp_path, session_id, job_id, lease_id, "docs/out/OUT.md")

    result = _data(
        _run_cli(
            tmp_path,
            "recovery",
            "resume",
            "--blocker-id",
            str(blocker["blocker_id"]),
            "--complete",
            "--session-id",
            session_id,
            "--summary",
            "remediated process blocker",
        )
    )

    assert result["status"] == "resumed_completed"
    assert result["completed_inline"] is True
    assert result["completion"]["status"] == "completed"
    assert _job_state(tmp_path, job_id) == "completed"
    assert _open_blocker(tmp_path, job_id) is None
    assert _lease_state(tmp_path, lease_id) == "released"


def test_recovery_resume_exit_blocker_requires_force(tmp_path: Path) -> None:
    workflow = _build_workflow(
        command=["bash", "-c", "exit 1"],
        expected_artifact_required=False,
    )
    _run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    _data(
        _run_cli(
            tmp_path,
            "adapter",
            "run",
            "--session-id",
            session_id,
            "--lease-id",
            lease_id,
            "--stdin",
            "none",
        )
    )
    blocker = _open_blocker(tmp_path, job_id)
    assert blocker is not None
    assert blocker["blocker_kind"] == "process_exit_nonzero"
    assert any(
        cmd.endswith("--force")
        for cmd in blocker["payload"]["recovery_commands"]
    )

    refused = _run_cli(
        tmp_path,
        "recovery",
        "resume",
        "--blocker-id",
        str(blocker["blocker_id"]),
        check=False,
    )
    assert refused["returncode"] == 4
    assert "requires --force" in refused["error"]["message"]

    resumed = _data(
        _run_cli(
            tmp_path,
            "recovery",
            "resume",
            "--blocker-id",
            str(blocker["blocker_id"]),
            "--force",
        )
    )
    assert resumed["status"] == "resumed"
    assert resumed["force"] is True
    assert _job_state(tmp_path, job_id) == "running"


def test_recovery_resume_review_blocker_allows_accept_with_findings(
    tmp_path: Path,
) -> None:
    workflow = _build_workflow(command=["bash", "-c", "exit 0"], job_type="review")
    _run_id, session_id, job_id, lease_id, _msg = _start_with(tmp_path, workflow)
    _data(
        _run_cli(
            tmp_path,
            "adapter",
            "run",
            "--session-id",
            session_id,
            "--lease-id",
            lease_id,
            "--stdin",
            "none",
        )
    )
    blocker = _open_blocker(tmp_path, job_id)
    assert blocker is not None
    assert blocker["blocker_kind"] == "process_review_verdict_missing"

    still_missing_artifact = _run_cli(
        tmp_path,
        "recovery",
        "resume",
        "--blocker-id",
        str(blocker["blocker_id"]),
        check=False,
    )
    assert still_missing_artifact["returncode"] == 4
    _publish_artifact(tmp_path, session_id, job_id, lease_id, "docs/out/OUT.md")
    verdict_while_blocked = _run_cli(
        tmp_path,
        "verdict",
        "--session-id",
        session_id,
        "--job-id",
        job_id,
        "--lease-id",
        lease_id,
        "--verdict",
        "accept_with_findings",
        check=False,
    )
    assert verdict_while_blocked["returncode"] == 4
    assert "must be running" in verdict_while_blocked["error"]["message"]

    resumed = _data(
        _run_cli(
            tmp_path,
            "recovery",
            "resume",
            "--blocker-id",
            str(blocker["blocker_id"]),
        )
    )
    assert resumed["status"] == "resumed"
    assert resumed["review_verdict_missing"] is True
    assert resumed["next_actions"] == ["record_review_verdict"]
    assert _job_state(tmp_path, job_id) == "running"

    verdict = _data(
        _run_cli(
            tmp_path,
            "verdict",
            "--session-id",
            session_id,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--verdict",
            "accept_with_findings",
            "--rationale",
            "Accepted with recorded findings.",
        )
    )
    assert verdict["status"] == "completed"
    assert verdict["verdict"] == "accept_with_findings"
    assert _job_state(tmp_path, job_id) == "completed"
