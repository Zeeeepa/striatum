from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path

import sqlite3

from striatum.dashboard import render_frame
from striatum.service import _byline_line, _jinja_env, _shape_artifact_rows

from test_cli_mvp import claim, data, prepare_started_run, register, run_cli
from test_web_ui import _http_get_raw, _spawn_service, _stop_service


MODEL_BYLINE = "author: author-codex-gpt-5.5-001"
OPERATOR_BYLINE = "author: operator"


def test_legacy_operator_helpers_retired_outside_test_harness(tmp_path: Path) -> None:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(Path(__file__).resolve().parents[1] / "src")
    env["STRIATUM_DAEMON_REQUIRED"] = "1"
    env["STRIATUM_SQLITE_CONNECT_TRIPWIRE"] = "1"
    env.pop("STRIATUM_TEST_HARNESS", None)

    commands = [
        ["byline", "--session-id", "session_1", "--job-id", "job_1"],
        ["inbox", "--session-id", "session_1"],
    ]
    for command in commands:
        result = subprocess.run(
            [
                sys.executable,
                "-m",
                "striatum.cli",
                "--repo",
                str(tmp_path),
                *command,
                "--json",
            ],
            env=env,
            text=True,
            capture_output=True,
            check=False,
        )

        assert result.returncode == 8
        payload = json.loads(result.stdout)
        assert payload["ok"] is False
        assert "legacy operator helpers are retired" in payload["error"]["message"]
        assert not (tmp_path / ".striatum" / "state.sqlite3").exists()


def _unattested_author_fixture(repo: Path) -> tuple[str, str, str]:
    run_id = prepare_started_run(repo)
    session_id = register(repo, run_id, "author", "codex")
    packet = claim(repo, session_id)

    assert packet["lane_attestation"]["state"] == "unattested"
    assert packet["lane_attestation"]["reason"] == "no_attached_supervisor"

    job = packet["job"]
    assert isinstance(job, dict)
    job_id = str(job["job_id"])

    byline = data(run_cli(repo, "byline", "--session-id", session_id, "--job-id", job_id))
    assert byline["byline"] == OPERATOR_BYLINE

    expected_artifacts = packet["expected_artifacts"]
    assert isinstance(expected_artifacts, list)
    assert expected_artifacts
    first_artifact = expected_artifacts[0]
    assert isinstance(first_artifact, dict)
    assert first_artifact["author_line"] == OPERATOR_BYLINE

    return run_id, session_id, job_id


def test_unattested_session_renders_operator_byline_on_dashboard_and_web(
    tmp_path: Path,
) -> None:
    run_id, _session_id, job_id = _unattested_author_fixture(tmp_path)

    status_payload = data(run_cli(tmp_path, "status", "--run-id", run_id))
    dashboard = render_frame(
        {
            "run": {"run_id": run_id, "branch_name": "striatum/v1-test", "state": "running"},
            "status": status_payload,
            "events": [],
            "updated_at": "2026-05-18T00:00:00Z",
        },
        terminal_width=120,
    )
    assert OPERATOR_BYLINE in dashboard
    assert MODEL_BYLINE not in dashboard

    proc, port = _spawn_service(tmp_path, "--web")
    try:
        web_bodies = []
        for path in (f"/run/{run_id}", f"/run/{run_id}/job/{job_id}"):
            status, headers, body = _http_get_raw(port, path)
            assert status == 200
            assert "text/html" in headers.get("Content-Type", "")
            text = body.decode("utf-8")
            web_bodies.append(text)
            assert OPERATOR_BYLINE in text
            assert MODEL_BYLINE not in text

        combined_web = "\n".join(web_bodies)
        assert OPERATOR_BYLINE in combined_web
        assert MODEL_BYLINE not in combined_web
    finally:
        _stop_service(proc)


def test_unattested_byline_renderers_hide_forged_model_author() -> None:
    shaped = _byline_line(MODEL_BYLINE, attested=False)
    assert shaped["display"] == OPERATOR_BYLINE
    assert MODEL_BYLINE not in shaped["display"]

    template = _jinja_env().from_string(
        "{% import '_components.html' as ui %}"
        "{{ ui.byline_line(model_byline, attested=false) }}"
    )
    rendered = template.render(model_byline=MODEL_BYLINE)
    assert OPERATOR_BYLINE in rendered
    assert MODEL_BYLINE not in rendered


def test_artifact_attestation_uses_recorded_author_line_not_live_session() -> None:
    conn = sqlite3.connect(":memory:")
    try:
        shaped = _shape_artifact_rows(
            conn,
            artifacts=[
                {
                    "artifact_id": "art_1",
                    "repo_path": "docs/out.md",
                    "session_id": "closed_or_lost_session",
                    "author_line": MODEL_BYLINE,
                }
            ],
            expected_rows=[
                {
                    "path": "docs/out.md",
                    "expected_author_line": MODEL_BYLINE,
                }
            ],
        )
    finally:
        conn.close()

    chip = shaped[0]["lane_attestation_chip"]
    assert chip["attested"] is True
    assert chip["state"] == "attested"
    assert shaped[0]["byline_line"]["display"] == MODEL_BYLINE


def test_artifact_override_with_model_byline_never_renders_attested() -> None:
    conn = sqlite3.connect(":memory:")
    try:
        shaped = _shape_artifact_rows(
            conn,
            artifacts=[
                {
                    "artifact_id": "art_1",
                    "repo_path": "docs/out.md",
                    "session_id": "sess_operator_override",
                    "author_line": MODEL_BYLINE,
                    "attestation_override_rationale": "operator published on behalf",
                }
            ],
            expected_rows=[
                {
                    "path": "docs/out.md",
                    "expected_author_line": MODEL_BYLINE,
                }
            ],
        )
    finally:
        conn.close()

    chip = shaped[0]["lane_attestation_chip"]
    assert chip["attested"] is False
    assert chip["state"] == "unattested"
    assert chip["reason"] == "operator_override"
    assert shaped[0]["byline_line"]["display"] == OPERATOR_BYLINE
    assert shaped[0]["lane_evidence_chip"]["state"] == "override"
    assert shaped[0]["lane_evidence_chip"]["rationale"] == "operator published on behalf"
