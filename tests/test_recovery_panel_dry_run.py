# ruff: noqa
from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import json
from pathlib import Path

from striatum.legacy_sqlite.db import connect
from test_cli_mvp import claim, prepare_started_run, register, write_artifact
from test_web_ui import _http_get_raw, _http_post_json, _spawn_service, _stop_service

ROOT = Path(__file__).resolve().parents[1]


def _seed_stale_publishable_artifact(repo: Path) -> tuple[str, str]:
    run_id = prepare_started_run(repo)
    session_id = register(repo, run_id, "author", "codex")
    packet = claim(repo, session_id)
    job_id = str(packet["job"]["job_id"])
    lease_id = str(packet["lease"]["lease_id"])
    expected = packet["expected_artifacts"][0]
    artifact_path = str(expected["path"])
    write_artifact(
        repo,
        artifact_path,
        text=f"{expected['author_line']}\n\npublishable stale artifact\n",
    )
    with connect(repo) as conn:
        conn.execute("UPDATE leases SET state = 'expired' WHERE lease_id = ?", (lease_id,))
        conn.execute("UPDATE jobs SET state = 'stale_lease' WHERE job_id = ?", (job_id,))
        conn.commit()
    return run_id, artifact_path


def test_recovery_panel_dry_run_endpoint_returns_would_publish_rows(tmp_path: Path) -> None:
    run_id, artifact_path = _seed_stale_publishable_artifact(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _http_post_json(
            port,
            "/v1/invoke",
            {
                "argv": [
                    "recovery",
                    "auto-publish",
                    "--run-id",
                    run_id,
                    "--dry-run",
                ]
            },
        )
        envelope = json.loads(body)
        assert status == 200, envelope
        assert envelope["ok"] is True
        data = envelope["data"]
        assert data["dry_run"] is True
        assert data["published_count"] == 1
        assert data["published"][0]["would_publish"][0]["path"] == artifact_path
    finally:
        _stop_service(proc)


def test_run_detail_serves_recovery_recipe_and_recovery_island_asset(tmp_path: Path) -> None:
    run_id, _artifact_path = _seed_stale_publishable_artifact(tmp_path)
    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, _headers, body = _http_get_raw(port, f"/run/{run_id}")
        assert status == 200
        assert b"striatum recovery auto-publish" in body
        assert b"--dry-run" in body
        assert b'id="island-recovery-panel"' in body
        assert b"/static/build/island-recovery-panel.js" in body
        assert b"data-copy=" in body

        vite_config = (ROOT / "src/striatum/web/frontend/vite.config.ts").read_text(
            encoding="utf-8"
        )
        assert '"island-recovery-panel"' in vite_config
        assert "islands/recovery-panel/main.tsx" in vite_config
    finally:
        _stop_service(proc)
