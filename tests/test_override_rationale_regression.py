from __future__ import annotations

import sqlite3
from pathlib import Path

from striatum.dashboard import _verdict_chip
from striatum.service import _shape_verdict_rows

from test_cli_mvp import (
    claim,
    complete_claimed_job,
    data,
    packet_ids,
    prepare_started_run,
    register,
    run_cli,
    run_cli_text,
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

    dashboard = run_cli_text(tmp_path, "dashboard", "--run-id", run_id, "--once")
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


def test_dashboard_verdict_chip_includes_truncated_override_rationale() -> None:
    rationale = "Operator accepted because the remaining issue is tracked elsewhere."
    rendered = _verdict_chip("accept_with_findings", override=True, rationale=rationale)
    assert "accept_with_findings (override):" in rendered
    assert rationale in rendered
