from __future__ import annotations

import json
from pathlib import Path

from striatum.dashboard import render_frame
from striatum.legacy_sqlite.db import connect

from test_cli_mvp import (
    claim,
    complete_claimed_job,
    data,
    prepare_started_run,
    register,
    run_cli,
    verdict_claimed_review,
    write_artifact,
)
from test_web_ui import _http_get_raw, _spawn_service, _stop_service


def test_dashboard_and_run_page_surface_same_v1_chip_labels(tmp_path: Path) -> None:
    """RFC 0050 V1: dashboard text and run page share operator labels.

    The fixture deliberately includes:
    - unattested sessions with an attestation reason,
    - a non-accepting verdict,
    - a still-claimable packet for ``inspect_packet_with_inbox``, and
    - a stale lease with an on-disk artifact for ``recovery_auto_publish``.
    """

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

    codex_reviewer = register(tmp_path, run_id, "reviewer", "codex")
    verdict_claimed_review(
        tmp_path,
        codex_reviewer,
        claim(tmp_path, codex_reviewer),
        verdict="needs_revision",
        path="docs/reviews/rfc-ledger/codex/RFC_LEDGER_REVIEW.md",
    )

    # Keep the other reviewer packet pending so ``inspect_packet_with_inbox``
    # remains visible, and synthesize a separate stale lease on blocked work.
    register(tmp_path, run_id, "reviewer", "gemini")
    stale_lease_id = "lease_dashboard_web_parity_stale"
    with connect(tmp_path) as conn:
        stale_job = conn.execute(
            """
            SELECT job_id, expected_artifacts_json
              FROM jobs
             WHERE run_id = ?
               AND state = 'blocked'
               AND expected_artifacts_json IS NOT NULL
             ORDER BY workflow_job_id
             LIMIT 1
            """,
            (run_id,),
        ).fetchone()
        assert stale_job is not None
        expected_artifacts = json.loads(str(stale_job["expected_artifacts_json"]))
        stale_artifact_path = str(expected_artifacts[0]["path"])
        write_artifact(tmp_path, stale_artifact_path, text="author: operator\n\nstale work\n")
        conn.execute(
            """
            INSERT INTO leases (
              lease_id, run_id, resource_type, resource_id, owner_session_id,
              state, acquired_at, expires_at, last_heartbeat_at
            )
            VALUES (?, ?, 'job', ?, ?, 'expired',
                    '2026-05-07T00:00:00Z',
                    '2026-05-07T00:00:00Z',
                    '2026-05-07T00:00:00Z')
            """,
            (stale_lease_id, run_id, str(stale_job["job_id"]), author),
        )
        conn.execute(
            """
            UPDATE jobs
               SET state = 'stale_lease',
                   current_lease_id = ?
             WHERE job_id = ?
            """,
            (stale_lease_id, str(stale_job["job_id"])),
        )
        conn.execute(
            """
            UPDATE blockers
               SET state = 'resolved',
                   resolved_at = '2026-05-07T00:00:00Z'
             WHERE run_id = ?
            """,
            (run_id,),
        )
        conn.commit()

    status_payload = data(run_cli(tmp_path, "status", "--run-id", run_id))
    required_next_actions = {
        "inspect_packet_with_inbox",
        "derive_expected_byline",
        "recovery_auto_publish",
    }
    assert required_next_actions <= set(status_payload["next_actions"])

    expected_labels = _v1_chip_labels(status_payload)
    assert {"unattested", "no_attached_supervisor", "needs_revision"} <= expected_labels
    assert required_next_actions <= expected_labels

    dashboard_text = render_frame(
        {
            "run": {"run_id": run_id, "branch_name": "striatum/v1-test", "state": "running"},
            "status": status_payload,
            "events": [],
            "updated_at": "2026-05-18T00:00:00Z",
        },
        terminal_width=120,
    )
    payload_text = json.dumps(status_payload, sort_keys=True)

    proc, port = _spawn_service(tmp_path, "--web")
    try:
        status, headers, body = _http_get_raw(port, f"/run/{run_id}")
        assert status == 200
        assert "text/html" in headers.get("Content-Type", "")
        run_page_html = body.decode("utf-8")
    finally:
        _stop_service(proc)

    assert _labels_seen(payload_text, expected_labels) == expected_labels
    assert _labels_seen(dashboard_text, expected_labels) == expected_labels
    assert _labels_seen(run_page_html, expected_labels) == expected_labels


def _v1_chip_labels(status_payload: dict[str, object]) -> set[str]:
    labels: set[str] = set()
    jobs = status_payload.get("jobs")
    if isinstance(jobs, dict):
        labels.update(str(state) for state in jobs)

    actions = status_payload.get("next_actions")
    if isinstance(actions, list):
        for action in actions:
            labels.add(str(action))

    sessions = status_payload.get("sessions")
    if isinstance(sessions, list):
        for session in sessions:
            if not isinstance(session, dict):
                continue
            attestation = session.get("lane_attestation")
            reason = session.get("lane_attestation_reason")
            if attestation:
                labels.add(str(attestation))
            if reason:
                labels.add(str(reason))

    verdicts = status_payload.get("latest_non_accepting_review_verdicts")
    if isinstance(verdicts, list):
        for verdict in verdicts:
            if isinstance(verdict, dict) and verdict.get("verdict"):
                labels.add(str(verdict["verdict"]))

    return labels


def _labels_seen(surface: str, labels: set[str]) -> set[str]:
    return {label for label in labels if label in surface}
