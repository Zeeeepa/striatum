from __future__ import annotations

import json
import sqlite3
from pathlib import Path

from test_web_run_posture_verdicts import _prepare_run
from test_web_ui import _git_init_repo, _http_get_raw, _spawn_service, _stop_service, _striatum_init


def test_posture_verdicts_show_override_provenance_and_attestation(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    run_id = _prepare_run(tmp_path)
    with sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3") as conn:
        job_id = str(conn.execute("SELECT job_id FROM jobs WHERE run_id = ? LIMIT 1", (run_id,)).fetchone()[0])
        conn.execute(
            """
            INSERT INTO sessions(session_id, run_id, role_id, lane_id, slug, ordinal,
                                 state, registered_at, capabilities_json,
                                 first_class, fresh_context)
            VALUES ('sess_override', ?, 'reviewer', 'codex', 'reviewer-codex-1',
                    1, 'closed', '2026-05-14T00:00:00Z', '[]', 1, 1)
            """,
            (run_id,),
        )
        conn.execute(
            """
            INSERT INTO verdicts(verdict_id, run_id, job_id, session_id, verdict,
                                 rationale, created_at, posture)
            VALUES ('verdict_override', ?, ?, 'sess_override', 'accept',
                    'operator accepted with rationale', '2026-05-14T00:00:00Z',
                    'threat_model')
            """,
            (run_id, job_id),
        )
        conn.execute(
            """
            INSERT INTO events(run_id, event_type, actor_session_id, job_id,
                               payload_json, created_at)
            VALUES (?, 'verdict.overridden', 'sess_override', ?, ?,
                    '2026-05-14T00:00:00Z')
            """,
            (run_id, job_id, json.dumps({"verdict_id": "verdict_override", "verdict": "accept", "rationale": "operator accepted with rationale"})),
        )
        conn.commit()

    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _, body = _http_get_raw(port, f"/run/{run_id}/posture/threat_model")
        assert status == 200
        assert b"Provenance" in body
        assert b"operator-override" in body
        assert b"operator accepted with rationale" in body
        assert b"unattested" in body
    finally:
        _stop_service(proc)
