from __future__ import annotations

import json
import sqlite3
from pathlib import Path

from test_cli_mvp import claim, prepare_started_run, register, write_artifact
from test_web_ui import _http_get_raw, _spawn_service, _stop_service
from striatum.service import _jinja_env


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


def test_recovery_panel_template_renders_auto_finalize_projection() -> None:
    template = _jinja_env().from_string(
        "{% import '_recovery_panel.html' as recovery_ui %}"
        "{{ recovery_ui.recovery_panel(panel) }}"
    )
    html = template.render(
        panel={
            "run_id": "run_123",
            "blockers": [],
            "human_checkpoints": [],
            "blocked": [],
            "recipes": [],
            "auto_publish_recipe": None,
            "auto_finalize_dry_run": {
                "eligible_count": 1,
                "eligible": [
                    {
                        "workflow_job_id": "review_docs",
                        "artifacts": [{"path": "docs/review.md"}],
                    }
                ],
                "skipped_count": 1,
                "skipped": [
                    {
                        "workflow_job_id": "review_api",
                        "reason": "expected_artifact validation refused",
                        "artifacts": [
                            {
                                "path": "docs/api.md",
                                "reason": "finding artifact front matter is required for auto-finalize",
                            }
                        ],
                    }
                ],
                "policy": {"live_allowed": False},
            },
        }
    )

    assert "Auto-finalize dry run" in html
    assert "1 eligible" in html
    assert "1 refused" in html
    assert "live allowed: no" in html
    assert "review_docs" in html
    assert "docs/review.md" in html
    assert "finding artifact front matter is required for auto-finalize" in html
