"""RFC 0046 V1 lane evidence guard — unit tests.

These pin the three internal helpers that drive the publish-time guard:

- ``_is_operator_byline`` — distinguishes operator-authored vs model-attested.
- ``_lane_evidence_present`` — looks up process_executions for a session.
- migration v15 — adds the ``attestation_override_rationale`` column.

Full integration tests against a live publish flow live in
``tests/test_cli_mvp.py`` (existing) and the dogfood-053 acceptance
suite. These unit tests just pin the building blocks so regressions in
the guard logic fail fast without spinning up the entire scaffold.
"""

from __future__ import annotations

import sqlite3


from striatum.artifacts import _is_operator_byline, _lane_evidence_present
from striatum.legacy_sqlite.migrations import apply_migrations


def test_is_operator_byline_recognises_canonical_form() -> None:
    assert _is_operator_byline("author: operator") is True
    assert _is_operator_byline("author: operator [self-declared: x]") is True


def test_is_operator_byline_rejects_model_bylines() -> None:
    assert _is_operator_byline("author: reviewer-codex-gpt-5.5-001") is False
    assert _is_operator_byline("author: implementer-claude-code-001") is False
    assert _is_operator_byline("author: designer-unknown-model-001") is False


def test_lane_evidence_present_returns_false_for_empty_db() -> None:
    """No process_executions row → no evidence → guard refuses."""
    conn = sqlite3.connect(":memory:")
    conn.row_factory = sqlite3.Row
    apply_migrations(conn)
    conn.execute("PRAGMA foreign_keys = OFF")
    assert (
        _lane_evidence_present(
            conn, session_id="sess_missing", path_text="x/HANDOFF.md"
        )
        is False
    )


def test_lane_evidence_present_returns_true_for_completed_run() -> None:
    """A completed exit-0 process_executions row counts as evidence."""
    conn = sqlite3.connect(":memory:")
    conn.row_factory = sqlite3.Row
    apply_migrations(conn)
    conn.execute("PRAGMA foreign_keys = OFF")
    conn.execute(
        """
        INSERT INTO process_executions (
          process_id, run_id, job_id, session_id, lease_id, packet_id,
          adapter, command_json, cwd, scratch_path, stdin_mode,
          stdio_mode, pid, state, exit_code, started_at, ended_at
        ) VALUES (
          'proc_t', 'run_t', 'job_t', 'sess_present', 'lease_t',
          'pkt_t', 'process', '[]', '/tmp', '/tmp/s', 'packet', 'suppressed',
          12345, 'exited', 0,
          '2026-05-13T20:00:00Z', '2026-05-13T20:00:01Z'
        )
        """
    )
    assert (
        _lane_evidence_present(
            conn,
            session_id="sess_present",
            path_text="x/HANDOFF.md",
        )
        is True
    )


def test_lane_evidence_present_rejects_failed_completion() -> None:
    """A completed but non-zero exit doesn't count as evidence."""
    conn = sqlite3.connect(":memory:")
    conn.row_factory = sqlite3.Row
    apply_migrations(conn)
    conn.execute("PRAGMA foreign_keys = OFF")
    conn.execute(
        """
        INSERT INTO process_executions (
          process_id, run_id, job_id, session_id, lease_id, packet_id,
          adapter, command_json, cwd, scratch_path, stdin_mode,
          stdio_mode, pid, state, exit_code, started_at, ended_at
        ) VALUES (
          'proc_t', 'run_t', 'job_t', 'sess_failed', 'lease_t',
          'pkt_t', 'process', '[]', '/tmp', '/tmp/s', 'packet', 'suppressed',
          12345, 'exited', 7,
          '2026-05-13T20:00:00Z', '2026-05-13T20:00:01Z'
        )
        """
    )
    assert (
        _lane_evidence_present(
            conn,
            session_id="sess_failed",
            path_text="x/HANDOFF.md",
        )
        is False
    )


def test_migration_v15_adds_override_rationale_column() -> None:
    """The v15 migration must add the attestation_override_rationale
    column to artifacts and leave it nullable."""
    conn = sqlite3.connect(":memory:")
    apply_migrations(conn)
    cols = {row[1]: row for row in conn.execute("PRAGMA table_info(artifacts)").fetchall()}
    assert "attestation_override_rationale" in cols
    # notnull flag should be 0 (nullable)
    assert cols["attestation_override_rationale"][3] == 0
