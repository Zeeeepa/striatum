from __future__ import annotations

import sqlite3
from pathlib import Path

from test_cli_mvp import claim, prepare_started_run, register
from test_web_ui import _http_get_raw, _spawn_service, _stop_service


def test_doctor_renders_per_record_recovery_recipe(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    session_id = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, session_id)
    job_id = str(packet["job"]["job_id"])
    lease_id = str(packet["lease"]["lease_id"])
    with sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3") as conn:
        conn.execute("UPDATE leases SET state = 'expired' WHERE lease_id = ?", (lease_id,))
        conn.execute(
            """
            INSERT INTO process_executions(
              process_id, run_id, job_id, session_id, lease_id, packet_id, adapter,
              command_json, cwd, scratch_path, stdin_mode, stdio_mode, state,
              pid, started_at
            )
            VALUES ('proc_doctor_recipe', ?, ?, ?, ?, ?,
                    'subprocess', '[]', ?, '.striatum/scratch/proc_doctor_recipe',
                    'packet', 'suppressed', 'running', 999999,
                    '2026-05-14T00:00:00Z')
            """,
            (run_id, job_id, session_id, lease_id, str(packet["packet_id"]), str(tmp_path)),
        )
        conn.commit()

    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, "/doctor")
        assert status == 200
        assert b"process_running_with_expired_lease" in body
        assert b"striatum recovery process-reconcile" in body
    finally:
        _stop_service(proc)
