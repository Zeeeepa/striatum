"""Tests for RFC 0018 step 3 (V1.5): verdicts.posture column +
introspection surfacing.

V1.5 ships migration v10 (`verdicts.posture` column with
`'neutral'` default + index), wires `record_review_verdict` to
read posture from the workflow snapshot, and surfaces posture in
six introspection paths: status, run summary, evidence export,
run graph json, dashboard, web UI.
"""

from __future__ import annotations

import sqlite3
from pathlib import Path

from striatum.dashboard import render_frame

from test_cli_mvp import (
    JsonDict,
    data,
    init_repo,
    run_cli,
    temporary_workflow,
)


def _base_workflow(*, posture: str | None = None) -> JsonDict:
    """Return a minimal valid workflow with a single review job.

    When ``posture`` is None, no `review_posture` is declared (the
    zero-regression baseline). Otherwise the review job carries the
    posture string verbatim.
    """
    review_job: JsonDict = {
        "id": "review_only",
        "type": "review",
        "title": "Review",
        "role_id": "reviewer",
        "lane_id": "lane_a",
        "fresh_session_required": True,
        "objective": "Review",
        "task_prompt": {"path": "prompts/review.md"},
        "write_scope": {
            "mode": "review_only_artifact",
            "repo_write": False,
            "allowed_paths": ["docs/reviews/"],
            "forbidden_paths": [".striatum/"],
        },
        "expected_artifacts": [
            {"logical_name": "review", "kind": "finding",
             "path": "docs/reviews/REVIEW.md", "required": True},
        ],
    }
    if posture is not None:
        review_job["review_posture"] = posture
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "wf-postures-introspection",
        "workflow_version": "1",
        "name": "Posture introspection",
        "branch": {"mode": "confirm", "suggested_name": "wf/posture-i", "allow_dirty": False},
        "coordinator": {"role_id": "reviewer", "lane_id": "lane_a"},
        "lanes": {
            "lane_a": {
                "adapter": "process",
                "display_model": "X",
                "command": ["echo"],
                "capabilities": ["review"],
            },
        },
        "roles": {"reviewer": {"definition_path": "roles/reviewer.md"}},
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 1,
            "require_disjoint_write_scopes": True,
        },
        "jobs": [review_job],
        "edges": [],
        "cycles": [],
    }


def _drive_to_verdict(
    tmp_path: Path, *, posture: str | None
) -> tuple[str, str]:
    """Drive a tiny review-only run to the point where a verdict is
    recorded. Returns (run_id, verdict_id)."""
    workflow = _base_workflow(posture=posture)
    workflow_path = temporary_workflow(tmp_path, workflow)
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(workflow_path)))
    run_id = str(prepared["run_id"])
    run_cli(tmp_path, "branch", "confirm", "--run-id", run_id, "--branch", "wf/posture-test")
    run_cli(tmp_path, "run", "start", "--run-id", run_id)
    sess = data(run_cli(
        tmp_path, "register-session",
        "--run-id", run_id, "--role", "reviewer", "--lane", "lane_a", "--fresh",
    ))
    session_id = str(sess["session_id"])
    claim = data(run_cli(tmp_path, "claim-next", "--session-id", session_id))
    packet = claim["packet"]
    job_id = packet["job"]["job_id"]
    lease_id = packet["lease"]["lease_id"]
    msg_id = packet["lease"]["message_id"]
    run_cli(tmp_path, "ack", "--session-id", session_id, "--message-id", msg_id, "--lease-id", lease_id)
    review_path = tmp_path / "docs" / "reviews" / "REVIEW.md"
    review_path.parent.mkdir(parents=True, exist_ok=True)
    expected_byline = packet["job"]["author"]["line"]
    review_path.write_text(
        f"# Review\n\n{expected_byline}\ndate: 2026-05-09\nverdict: accept\n\nbody\n",
        encoding="utf-8",
    )
    submit = data(run_cli(
        tmp_path, "submit-review",
        "--session-id", session_id,
        "--job-id", job_id,
        "--lease-id", lease_id,
        "--kind", "finding",
        "--logical-name", "review",
        "--path", "docs/reviews/REVIEW.md",
        "--verdict", "accept",
    ))
    return run_id, str(submit["verdict"]["verdict_id"])


def _read_verdict_row(tmp_path: Path, verdict_id: str) -> dict[str, object]:
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    conn.row_factory = sqlite3.Row
    try:
        row = conn.execute(
            "SELECT * FROM verdicts WHERE verdict_id = ?",
            (verdict_id,),
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    return dict(row)


# --- Migration / column presence -----------------------------------


def test_verdicts_posture_column_present_after_init(tmp_path: Path) -> None:
    """A freshly-initialized DB has the `posture` column in `verdicts`
    via the updated baseline schema."""
    init_repo(tmp_path)
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        cols = [row[1] for row in conn.execute(
            "PRAGMA table_info(verdicts)"
        ).fetchall()]
    finally:
        conn.close()
    assert "posture" in cols


def test_idx_verdicts_posture_present_after_init(tmp_path: Path) -> None:
    """The covering index ships in the baseline schema."""
    init_repo(tmp_path)
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        names = [
            row[0] for row in conn.execute(
                "SELECT name FROM sqlite_master WHERE type='index'"
            ).fetchall()
        ]
    finally:
        conn.close()
    assert "idx_verdicts_posture" in names


def test_migration_v10_idempotent(tmp_path: Path) -> None:
    """Re-running the migrations doesn't error and doesn't duplicate
    the column or index."""
    from striatum.legacy_sqlite.migrations import apply_migrations
    init_repo(tmp_path)
    conn = sqlite3.connect(tmp_path / ".striatum" / "state.sqlite3")
    try:
        apply_migrations(conn)
        apply_migrations(conn)
        cols = [row[1] for row in conn.execute(
            "PRAGMA table_info(verdicts)"
        ).fetchall()]
        # Only one `posture` column.
        assert cols.count("posture") == 1
    finally:
        conn.close()


# --- record_review_verdict writes posture --------------------------


def test_submit_review_writes_declared_posture(tmp_path: Path) -> None:
    _, verdict_id = _drive_to_verdict(tmp_path, posture="security")
    row = _read_verdict_row(tmp_path, verdict_id)
    assert row["posture"] == "security"


def test_submit_review_defaults_to_neutral_for_undeclared(tmp_path: Path) -> None:
    _, verdict_id = _drive_to_verdict(tmp_path, posture=None)
    row = _read_verdict_row(tmp_path, verdict_id)
    assert row["posture"] == "neutral"


def test_submit_review_writes_custom_posture(tmp_path: Path) -> None:
    _, verdict_id = _drive_to_verdict(tmp_path, posture="custom:my_thing")
    row = _read_verdict_row(tmp_path, verdict_id)
    assert row["posture"] == "custom:my_thing"


# --- status verdicts_by_posture ------------------------------------


def test_status_emits_verdicts_by_posture(tmp_path: Path) -> None:
    run_id, _ = _drive_to_verdict(tmp_path, posture="security")
    status = data(run_cli(tmp_path, "status", "--run-id", run_id, "--json"))
    assert "verdicts_by_posture" in status
    assert status["verdicts_by_posture"] == {"security": 1}


def test_status_verdicts_by_posture_empty_when_no_verdicts(tmp_path: Path) -> None:
    """Always-emitted shape: a run with no verdicts has an empty dict."""
    workflow = _base_workflow()
    workflow_path = temporary_workflow(tmp_path, workflow)
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(workflow_path)))
    run_id = str(prepared["run_id"])
    status = data(run_cli(tmp_path, "status", "--run-id", run_id, "--json"))
    assert "verdicts_by_posture" in status
    assert status["verdicts_by_posture"] == {}


# --- run summary posture grouping ----------------------------------


def test_run_summary_includes_posture_when_non_neutral(tmp_path: Path) -> None:
    run_id, _ = _drive_to_verdict(tmp_path, posture="security")
    run_cli(tmp_path, "run", "summary", "--run-id", run_id, "--path", "RUN_SUMMARY.md")
    body = (tmp_path / "RUN_SUMMARY.md").read_text(encoding="utf-8")
    assert "[posture: `security`]" in body


def test_run_summary_omits_posture_for_neutral_only(tmp_path: Path) -> None:
    """Zero-regression: a posture-omitting run produces a summary with
    no posture suffix in the Verdicts section."""
    run_id, _ = _drive_to_verdict(tmp_path, posture=None)
    run_cli(tmp_path, "run", "summary", "--run-id", run_id, "--path", "RUN_SUMMARY.md")
    body = (tmp_path / "RUN_SUMMARY.md").read_text(encoding="utf-8")
    assert "[posture:" not in body


# --- evidence export ----------------------------------------------


def test_evidence_export_includes_posture_in_verdict(tmp_path: Path) -> None:
    """Evidence export is Markdown that embeds the redacted snapshot
    JSON in fenced code blocks; the posture field appears in that
    block once we've added it to the SELECT + redaction whitelist."""
    run_id, _ = _drive_to_verdict(tmp_path, posture="threat_model")
    run_cli(tmp_path, "evidence", "export", "--run-id", run_id, "--path", "EVIDENCE.md")
    body = (tmp_path / "EVIDENCE.md").read_text(encoding="utf-8")
    assert '"posture": "threat_model"' in body


def test_evidence_export_neutral_posture_present(tmp_path: Path) -> None:
    """Even posture-omitting verdicts surface as `neutral` in the
    evidence export — this is the intentional V1.5 format change."""
    run_id, _ = _drive_to_verdict(tmp_path, posture=None)
    run_cli(tmp_path, "evidence", "export", "--run-id", run_id, "--path", "EVIDENCE.md")
    body = (tmp_path / "EVIDENCE.md").read_text(encoding="utf-8")
    assert '"posture": "neutral"' in body


# --- run graph --format json ---------------------------------------


def test_run_graph_json_includes_posture_on_review_verdict(tmp_path: Path) -> None:
    run_id, _ = _drive_to_verdict(tmp_path, posture="security")
    out = data(run_cli(tmp_path, "run", "graph", "--run-id", run_id, "--format", "json"))
    nodes = out["graph"]["nodes"]
    review_nodes = [n for n in nodes if n.get("type") == "review"]
    assert review_nodes
    found = False
    for node in review_nodes:
        latest = node.get("latest_verdict")
        if latest is not None:
            assert "posture" in latest
            assert latest["posture"] == "security"
            found = True
    assert found, "no review node carried a verdict"


# --- dashboard panel ----------------------------------------------


def test_dashboard_renders_posture_summary_when_non_neutral(tmp_path: Path) -> None:
    text = _render_dashboard_with_postures({"security": 1})
    assert "Postures: security=1" in text


def test_dashboard_omits_posture_summary_when_only_neutral(tmp_path: Path) -> None:
    text = _render_dashboard_with_postures({"neutral": 1})
    assert "Postures:" not in text


def _render_dashboard_with_postures(posture_counts: dict[str, int]) -> str:
    return render_frame(
        {
            "run": {"run_id": "run_posture", "branch_name": "striatum/v1-test", "state": "running"},
            "status": {
                "jobs": {},
                "open_blockers": [],
                "human_checkpoints": [],
                "latest_non_accepting_review_verdicts": [],
                "claimable_jobs": [],
                "next_actions": [],
            },
            "events": [],
            "posture_counts": posture_counts,
            "updated_at": "2026-05-18T00:00:00Z",
        },
        terminal_width=100,
    )
