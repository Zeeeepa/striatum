# ruff: noqa
from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import json
from pathlib import Path

from striatum.legacy_sqlite.db import connect
from test_cli_mvp import claim, packet_ids, prepare_started_run, register, run_cli
from test_web_ui import _http_get_raw, _spawn_service, _stop_service


def test_job_detail_expected_artifacts_and_process_evidence(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    session_id = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, session_id)
    job_id, _, lease_id = packet_ids(packet)
    packet_id = "packet_process_evidence"
    with connect(tmp_path) as conn:
        conn.execute("UPDATE work_packets SET packet_id = ? WHERE job_id = ?", (packet_id, job_id))
        conn.execute(
            """
            INSERT INTO process_executions(
              process_id, run_id, job_id, session_id, lease_id, packet_id, adapter,
              command_json, cwd, scratch_path, stdin_mode, stdio_mode, state,
              exit_code, started_at, ended_at
            )
            VALUES ('proc_job_detail', ?, ?, ?, ?, ?, 'subprocess',
                    ?, ?, '.striatum/scratch/proc_job_detail', 'packet',
                    'suppressed', 'failed', 1,
                    '2026-05-14T00:00:00Z', '2026-05-14T00:00:01Z')
            """,
            (run_id, job_id, session_id, lease_id, packet_id, json.dumps(["false"]), str(tmp_path)),
        )
        conn.commit()

    run_cli(tmp_path, "ack", "--session-id", session_id, "--message-id", str(packet["lease"]["message_id"]), "--lease-id", lease_id)

    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, f"/run/{run_id}/job/{job_id}")
        assert status == 200
        assert b"expected-artifacts-table" in body
        assert b"missing" in body
        assert b"striatum publish-artifact" in body
        assert b"Process evidence" in body
        assert b"proc_job_detail" in body
        assert b"Override verdict" in body
    finally:
        _stop_service(proc)
