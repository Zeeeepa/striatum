"""RFC 0047 V1 — decision-record propagation regression tests.

Pin the propagation behavior added to ``decision_record``:

- ``decision record --outcome rejected`` against a completed run
  → ``runs.state = 'compromised'``, accepting verdicts get
  ``superseded_by_decision_id`` set, ``run.compromised`` event emitted.
- ``decision record --outcome accepted`` against a compromised run
  → ``runs.state = 'completed'``, ``run.reopened_after_compromised``
  event emitted, supersession trail preserved.
- ``rejected`` against an already-compromised run is a no-op.
- ``accepted_with_follow_up`` and ``accepted`` against a non-
  compromised run do not change runs.state.

Schema invariants:

- Migration v16 admits ``compromised`` in the runs.state CHECK.
- Migration v16 adds ``superseded_by_decision_id`` + ``superseded_at``
  to ``verdicts``.
"""

from __future__ import annotations

import sqlite3

from striatum.cli.mutations import _propagate_decision_outcome
from striatum.legacy_sqlite.migrations import apply_migrations


def _scaffold(conn: sqlite3.Connection, *, run_state: str = "completed") -> str:
    """Insert minimal run + workflow_snapshot rows and return the run_id."""
    conn.execute(
        "INSERT INTO workflow_snapshots (workflow_snapshot_id, workflow_id, "
        "workflow_json, content_sha256, loaded_at) "
        "VALUES ('wfs_t', 'wf_t', '{}', 'sha_t', '2026-05-13T20:00:00Z')",
    )
    run_id = "run_t"
    conn.execute(
        "INSERT INTO runs (run_id, workflow_snapshot_id, repo_root, state, created_at) "
        "VALUES (?, 'wfs_t', '.', ?, '2026-05-13T20:00:00Z')",
        (run_id, run_state),
    )
    return run_id


def _scaffold_verdict(
    conn: sqlite3.Connection, *, run_id: str, verdict_id: str, verdict: str
) -> None:
    """Insert a job + session + verdict row for supersession testing."""
    conn.execute(
        "INSERT OR IGNORE INTO jobs (job_id, run_id, workflow_job_id, role_id, "
        "lane_selector_json, write_scope_json, expected_artifacts_json, state, "
        "attempt, max_attempts, idempotency_key, capability_requirements_json, "
        "created_at, ready_at) "
        "VALUES ('job_t', ?, 'j', 'reviewer', '{}', '{}', '[]', 'completed', 1, 1, "
        "'ide_t', '{}', '2026-05-13T20:00:00Z', '2026-05-13T20:00:00Z')",
        (run_id,),
    )
    conn.execute(
        "INSERT OR IGNORE INTO sessions (session_id, run_id, role_id, lane_id, "
        "ordinal, slug, capabilities_json, state, first_class, fresh_context, "
        "registered_at) "
        "VALUES ('sess_t', ?, 'reviewer', 'codex', 1, 'r-codex-1', '[]', "
        "'closed', 1, 1, '2026-05-13T20:00:00Z')",
        (run_id,),
    )
    conn.execute(
        "INSERT INTO verdicts (verdict_id, run_id, job_id, session_id, "
        "verdict, created_at) "
        "VALUES (?, ?, 'job_t', 'sess_t', ?, '2026-05-13T20:00:00Z')",
        (verdict_id, run_id, verdict),
    )


def _make_conn() -> sqlite3.Connection:
    conn = sqlite3.connect(":memory:")
    conn.row_factory = sqlite3.Row
    apply_migrations(conn)
    conn.execute("PRAGMA foreign_keys = OFF")
    return conn


def test_migration_v16_admits_compromised_state() -> None:
    """Sanity: the rebuilt runs CHECK constraint accepts compromised."""
    conn = _make_conn()
    _scaffold(conn)
    # No exception → CHECK admits the value.
    conn.execute("UPDATE runs SET state = 'compromised' WHERE run_id = 'run_t'")
    row = conn.execute("SELECT state FROM runs WHERE run_id = 'run_t'").fetchone()
    assert row["state"] == "compromised"


def test_migration_v16_adds_supersession_columns() -> None:
    conn = _make_conn()
    cols = {row[1] for row in conn.execute("PRAGMA table_info(verdicts)").fetchall()}
    assert "superseded_by_decision_id" in cols
    assert "superseded_at" in cols


def test_rejected_propagates_to_compromised_and_supersedes_verdicts() -> None:
    conn = _make_conn()
    run_id = _scaffold(conn, run_state="completed")
    _scaffold_verdict(conn, run_id=run_id, verdict_id="v_accept", verdict="accept_with_findings")
    result = _propagate_decision_outcome(
        conn,
        run_id=run_id,
        decision_id="dec_t",
        outcome="rejected",
        decision_path="docs/decisions/d_t.md",
        now="2026-05-13T20:00:01Z",
    )
    assert result["run_state_transition"] == {"from": "completed", "to": "compromised"}
    assert result["superseded_verdict_count"] == 1
    row = conn.execute("SELECT state FROM runs WHERE run_id = ?", (run_id,)).fetchone()
    assert row["state"] == "compromised"
    verdict_row = conn.execute(
        "SELECT superseded_by_decision_id, superseded_at FROM verdicts WHERE verdict_id = 'v_accept'"
    ).fetchone()
    assert verdict_row["superseded_by_decision_id"] == "dec_t"
    assert verdict_row["superseded_at"] == "2026-05-13T20:00:01Z"
    event = conn.execute(
        "SELECT payload_json FROM events WHERE event_type = 'run.compromised' AND run_id = ?",
        (run_id,),
    ).fetchone()
    assert event is not None


def test_rejected_against_already_compromised_is_noop() -> None:
    conn = _make_conn()
    run_id = _scaffold(conn, run_state="compromised")
    result = _propagate_decision_outcome(
        conn,
        run_id=run_id,
        decision_id="dec_t",
        outcome="rejected",
        decision_path="docs/decisions/d_t.md",
        now="2026-05-13T20:00:01Z",
    )
    assert result["run_state_transition"] is None
    events = conn.execute(
        "SELECT COUNT(*) AS n FROM events WHERE event_type = 'run.compromised'"
    ).fetchone()
    assert events["n"] == 0


def test_accepted_reopens_compromised_run() -> None:
    conn = _make_conn()
    run_id = _scaffold(conn, run_state="compromised")
    _scaffold_verdict(
        conn,
        run_id=run_id,
        verdict_id="v_super",
        verdict="accept_with_findings",
    )
    conn.execute(
        "UPDATE verdicts SET superseded_by_decision_id = 'dec_prev', "
        "superseded_at = '2026-05-13T19:00:00Z' "
        "WHERE verdict_id = 'v_super'"
    )
    result = _propagate_decision_outcome(
        conn,
        run_id=run_id,
        decision_id="dec_reopen",
        outcome="accepted",
        decision_path="docs/decisions/d_reopen.md",
        now="2026-05-13T20:00:01Z",
    )
    assert result["run_state_transition"] == {"from": "compromised", "to": "completed"}
    row = conn.execute("SELECT state FROM runs WHERE run_id = ?", (run_id,)).fetchone()
    assert row["state"] == "completed"
    # Supersession trail preserved (verdict still marked).
    verdict_row = conn.execute(
        "SELECT superseded_by_decision_id FROM verdicts WHERE verdict_id = 'v_super'"
    ).fetchone()
    assert verdict_row["superseded_by_decision_id"] == "dec_prev"
    event = conn.execute(
        "SELECT 1 FROM events WHERE event_type = 'run.reopened_after_compromised' AND run_id = ?",
        (run_id,),
    ).fetchone()
    assert event is not None


def test_accepted_against_completed_run_is_noop() -> None:
    conn = _make_conn()
    run_id = _scaffold(conn, run_state="completed")
    result = _propagate_decision_outcome(
        conn,
        run_id=run_id,
        decision_id="dec_t",
        outcome="accepted",
        decision_path="docs/decisions/d.md",
        now="2026-05-13T20:00:01Z",
    )
    assert result["run_state_transition"] is None
    row = conn.execute("SELECT state FROM runs WHERE run_id = ?", (run_id,)).fetchone()
    assert row["state"] == "completed"


def test_accepted_with_follow_up_does_not_change_state() -> None:
    conn = _make_conn()
    run_id = _scaffold(conn, run_state="completed")
    result = _propagate_decision_outcome(
        conn,
        run_id=run_id,
        decision_id="dec_t",
        outcome="accepted_with_follow_up",
        decision_path="docs/decisions/d.md",
        now="2026-05-13T20:00:01Z",
    )
    assert result["run_state_transition"] is None
