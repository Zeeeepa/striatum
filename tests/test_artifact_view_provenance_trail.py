# ruff: noqa
from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import json
from pathlib import Path

from striatum.legacy_sqlite.db import connect
from test_cli_mvp import claim, complete_claimed_job, prepare_started_run, register
from test_web_ui import _http_get_raw, _spawn_service, _stop_service


def test_artifact_view_renders_byline_integrity_and_provenance_trail(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    session_id = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, session_id)
    complete_claimed_job(
        tmp_path,
        session_id,
        packet,
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    with connect(tmp_path) as conn:
        artifact_id = str(conn.execute("SELECT artifact_id FROM artifacts WHERE run_id = ?", (run_id,)).fetchone()[0])
        conn.execute(
            """
            INSERT INTO events(run_id, event_type, actor_session_id, job_id, artifact_id,
                               payload_json, created_at)
            VALUES (?, 'provenance.publish_without_process_execution', ?, ?, ?, ?,
                    '2026-05-14T00:00:00Z')
            """,
            (run_id, session_id, str(packet["job"]["job_id"]), artifact_id, json.dumps({"artifact_id": artifact_id, "rationale": "test on behalf"})),
        )
        conn.commit()

    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, f"/run/{run_id}/artifact/{artifact_id}")
        assert status == 200
        assert b"Byline integrity" in body
        assert b"not yet correlated" in body
        assert b"provenance.publish_without_process_execution" in body
        assert b"test on behalf" in body
    finally:
        _stop_service(proc)
