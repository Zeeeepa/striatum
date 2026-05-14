from __future__ import annotations

import json
import sqlite3
from pathlib import Path

from test_cli_mvp import claim, prepare_started_run, register, write_artifact
from test_web_ui import _http_get_raw, _spawn_service, _stop_service


def test_run_detail_renders_recovery_panel_with_blocker_recipe(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    session_id = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, session_id)
    job_id = str(packet["job"]["job_id"])
    lease_id = str(packet["lease"]["lease_id"])
    expected = packet["expected_artifacts"][0]
    write_artifact(tmp_path, str(expected["path"]), text=f"{expected['author_line']}\n\nstale\n")

    with sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3") as conn:
        conn.execute("UPDATE leases SET state = 'expired' WHERE lease_id = ?", (lease_id,))
        conn.execute("UPDATE jobs SET state = 'stale_lease' WHERE job_id = ?", (job_id,))
        conn.execute(
            """
            INSERT INTO blockers(blocker_id, run_id, job_id, session_id, severity,
                                 blocker_kind, description, state, created_at, payload_json)
            VALUES ('block_recovery_panel', ?, ?, ?, 'blocked',
                    'process_outputs_missing', 'missing output', 'open',
                    '2026-05-14T00:00:00Z', ?)
            """,
            (
                run_id,
                job_id,
                session_id,
                json.dumps({"recovery_commands": [f"striatum recovery process-reconcile --run-id {run_id}"]}),
            ),
        )
        conn.commit()

    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, f"/run/{run_id}")
        assert status == 200
        assert b"recovery-panel" in body
        assert b'id="island-recovery-panel"' in body
        assert b"/static/build/island-recovery-panel.js" in body
        assert b"process_outputs_missing" in body
        assert b"striatum recovery process-reconcile" in body
        assert b"data-copy=" in body
        assert b"recovery auto-publish" in body
    finally:
        _stop_service(proc)
