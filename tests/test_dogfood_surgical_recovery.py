from __future__ import annotations

import json
import os
import subprocess
import sys
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any, cast

from striatum.legacy_sqlite.db import connect, expire_leases, transaction
from striatum.primitives import utc_now
from striatum.dogfood.operator_tools import surgical_recovery
from striatum.identity import process_start_time


ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"


def run_cli(repo: Path, *args: str) -> dict[str, Any]:
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
    assert result.returncode == 0, result.stderr
    return cast(dict[str, Any], json.loads(result.stdout)["data"])


def _running_author_packet(repo: Path) -> tuple[str, dict[str, Any]]:
    from striatum.legacy_sqlite.db import init_repo as legacy_init_repo

    legacy_init_repo(repo)
    prepared = run_cli(repo, "run", "prepare", "--workflow", str(WORKFLOW))
    run_id = str(prepared["run_id"])
    run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/test")
    run_cli(repo, "run", "start", "--run-id", run_id)
    session = run_cli(
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
    claimed = run_cli(repo, "claim-next", "--session-id", str(session["session_id"]))
    packet = cast(dict[str, Any], claimed["packet"])
    job = cast(dict[str, Any], packet["job"])
    lease = cast(dict[str, Any], packet["lease"])
    run_cli(
        repo,
        "ack",
        "--session-id",
        str(session["session_id"]),
        "--message-id",
        str(lease["message_id"]),
        "--lease-id",
        str(lease["lease_id"]),
    )
    packet["run_id"] = run_id
    packet["job_id"] = str(job["job_id"])
    packet["lease_id"] = str(lease["lease_id"])
    packet["message_id"] = str(lease["message_id"])
    return str(session["session_id"]), packet


def test_surgical_recovery_reactivates_expired_repo_write_job(tmp_path: Path) -> None:
    session_id, packet = _running_author_packet(tmp_path)
    expected = packet["expected_artifacts"][0]
    artifact_path = tmp_path / str(expected["path"])
    artifact_path.parent.mkdir(parents=True, exist_ok=True)
    artifact_path.write_text("# Draft\n\nready after inspection\n", encoding="utf-8")

    with connect(tmp_path) as conn:
        now = utc_now()
        conn.execute(
            """
            INSERT INTO process_supervisors(
              supervisor_id, run_id, session_id, adapter, command_json, cwd,
              scratch_path, stdin_pipe_path, pid, state, started_at, heartbeat_at,
              pid_start_time
            )
            VALUES (?, ?, ?, 'process', '[]', ?, ?, NULL, ?, 'attached', ?, ?, ?)
            """,
            (
                "sup_test",
                str(packet["run_id"]),
                session_id,
                str(tmp_path),
                str(tmp_path / ".striatum" / "scratch" / "sup_test"),
                os.getpid(),
                now,
                now,
                process_start_time(os.getpid()),
            ),
        )
        conn.commit()
        with transaction(conn):
            past = (datetime.now(UTC) - timedelta(seconds=5)).replace(microsecond=0).isoformat().replace("+00:00", "Z")
            conn.execute(
                "UPDATE leases SET expires_at = ? WHERE lease_id = ?",
                (past, str(packet["lease_id"])),
            )
            expire_leases(conn, run_id=str(packet["run_id"]))

        result = surgical_recovery(
            conn,
            job_id=str(packet["job_id"]),
            reason="artifact present and process still alive",
            extend_lease_seconds=900,
        )

    assert result["status"] == "recovered"
    with connect(tmp_path) as conn:
        lease = conn.execute(
            "SELECT state FROM leases WHERE lease_id = ?", (str(packet["lease_id"]),)
        ).fetchone()
        job = conn.execute(
            "SELECT state, current_lease_id FROM jobs WHERE job_id = ?",
            (str(packet["job_id"]),),
        ).fetchone()
        supervisor = conn.execute(
            "SELECT state FROM process_supervisors WHERE supervisor_id = 'sup_test'"
        ).fetchone()
    assert lease is not None
    assert job is not None
    assert supervisor is not None
    assert tuple(lease) == ("active",)
    assert tuple(job) == ("running", str(packet["lease_id"]))
    assert tuple(supervisor) == ("attached",)


def test_surgical_recovery_refuses_missing_expected_artifact(tmp_path: Path) -> None:
    _session_id, packet = _running_author_packet(tmp_path)
    with connect(tmp_path) as conn:
        with transaction(conn):
            past = (datetime.now(UTC) - timedelta(seconds=5)).replace(microsecond=0).isoformat().replace("+00:00", "Z")
            conn.execute(
                "UPDATE leases SET expires_at = ? WHERE lease_id = ?",
                (past, str(packet["lease_id"])),
            )
            expire_leases(conn, run_id=str(packet["run_id"]))
        result = surgical_recovery(
            conn,
            job_id=str(packet["job_id"]),
            reason="operator inspected",
        )

    assert result["ok"] is False
    assert result["error"]["code"] == "missing_expected_artifacts"


def test_surgical_recovery_requires_operator_reason(tmp_path: Path) -> None:
    _session_id, packet = _running_author_packet(tmp_path)
    with connect(tmp_path) as conn:
        result = surgical_recovery(
            conn,
            job_id=str(packet["job_id"]),
            reason=" ",
        )

    assert result["ok"] is False
    assert result["error"]["code"] == "reason_required"
