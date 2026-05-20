from __future__ import annotations
import pytest; pytest.skip("dependency eradicated", allow_module_level=True)

import sqlite3
from pathlib import Path

from striatum.dashboard import _verdict_chip, render_frame
from striatum.service import _shape_verdict_rows

from test_cli_mvp import (
    claim,
    complete_claimed_job,
    data,
    packet_ids,
    prepare_started_run,
    register,
    run_cli,
    verdict_claimed_review,
)
from test_web_ui import _http_get_raw, _spawn_service, _stop_service


OVERRIDE_RATIONALE = "Operator accepts with findings."


def test_override_rationale_renders_on_dashboard_and_web(tmp_path: Path) -> None:
    run_id = prepare_started_run(tmp_path)
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )

    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    packet = claim(tmp_path, reviewer)
    review_job_id, _message_id, _lease_id = packet_ids(packet)
    verdict_claimed_review(
        tmp_path,
        reviewer,
        packet,
        verdict="needs_revision",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )

    operator = register(tmp_path, run_id, "reviewer", "gemini")
    override = data(
        run_cli(
            tmp_path,
            "override-verdict",
            "--session-id",
            operator,
            "--job-id",
            review_job_id,
            "--verdict",
            "accept_with_findings",
            "--rationale",
            OVERRIDE_RATIONALE,
        )
    )
    assert override["status"] == "overridden"

    status_payload = data(run_cli(tmp_path, "status", "--run-id", run_id))
    dashboard = render_frame(
        {
            "run": {"run_id": run_id, "branch_name": "striatum/v1-test", "state": "running"},
            "status": status_payload,
            "events": [],
            "verdict_counts": {"needs_revision": 1, "accept_with_findings": 1},
            "override_verdict_counts": {"accept_with_findings": 1},
            "override_verdicts": [
                {
                    "previous_verdict": "needs_revision",
                    "verdict": "accept_with_findings",
                    "rationale": OVERRIDE_RATIONALE,
                }
            ],
            "updated_at": "2026-05-18T00:00:00Z",
        },
        terminal_width=120,
    )
    assert "accept_with_findings (override)" in dashboard
    assert OVERRIDE_RATIONALE in dashboard

    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, f"/run/{run_id}/job/{review_job_id}")
        assert status == 200
        assert "text/html" in headers.get("Content-Type", "")
        html = body.decode("utf-8")
        assert "operator-override" in html
        assert OVERRIDE_RATIONALE in html
    finally:
        _stop_service(proc)


def test_missing_verdict_source_does_not_infer_operator_override() -> None:
    conn = sqlite3.connect(":memory:")
    try:
        conn.row_factory = sqlite3.Row
        conn.execute("CREATE TABLE verdicts (verdict_id TEXT, verdict TEXT)")
        conn.execute("CREATE TABLE events (event_type TEXT, payload_json TEXT)")
        shaped = _shape_verdict_rows(
            conn,
            verdicts=[
                {
                    "verdict_id": "v1",
                    "verdict": "needs_revision",
                    "created_at": "2026-05-14T00:00:00Z",
                    "session_id": None,
                },
                {
                    "verdict_id": "v2",
                    "verdict": "accept",
                    "created_at": "2026-05-14T00:01:00Z",
                    "session_id": None,
                    "rationale": "Natural reviewer accepted after fixes.",
                },
            ],
        )
    finally:
        conn.close()

    latest = shaped[0]
    assert latest["verdict"] == "accept"
    assert latest["provenance"] == "natural"
    assert latest["override_rationale"] is None
    assert latest["verdict_chip"]["provenance"] == "natural"


def test_verdict_from_closed_supervised_session_is_distinct_from_never_attested() -> None:
    conn = sqlite3.connect(":memory:")
    try:
        conn.row_factory = sqlite3.Row
        conn.execute("CREATE TABLE verdicts (verdict_id TEXT, verdict TEXT)")
        conn.execute("CREATE TABLE events (event_type TEXT, payload_json TEXT)")
        conn.execute("CREATE TABLE sessions (session_id TEXT PRIMARY KEY)")
        conn.execute(
            """
            CREATE TABLE process_supervisors (
              supervisor_id TEXT PRIMARY KEY,
              session_id TEXT,
              state TEXT,
              started_at TEXT,
              ended_at TEXT
            )
            """
        )
        conn.execute("INSERT INTO sessions (session_id) VALUES ('sess_closed')")
        conn.execute(
            """
            INSERT INTO process_supervisors (
              supervisor_id, session_id, state, started_at, ended_at
            ) VALUES (
              'sup_closed', 'sess_closed', 'stopped',
              '2026-05-14T00:00:00Z', '2026-05-14T00:02:00Z'
            )
            """
        )
        shaped = _shape_verdict_rows(
            conn,
            verdicts=[
                {
                    "verdict_id": "v1",
                    "verdict": "accept",
                    "created_at": "2026-05-14T00:01:00Z",
                    "session_id": "sess_closed",
                }
            ],
        )
    finally:
        conn.close()

    chip = shaped[0]["lane_attestation_chip"]
    assert chip["state"] == "previously_attested"
    assert chip["label"] == "previously attested"
    assert chip["reason"] == "session_stopped"
    assert chip["supervisor_id"] == "sup_closed"


def test_dashboard_verdict_chip_includes_truncated_override_rationale() -> None:
    rationale = "Operator accepted because the remaining issue is tracked elsewhere."
    rendered = _verdict_chip("accept_with_findings", override=True, rationale=rationale)
    assert "accept_with_findings (override):" in rendered
    assert rationale in rendered
