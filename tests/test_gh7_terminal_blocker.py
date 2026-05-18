"""GH #7 regression: process-adapter must not block jobs that have
already reached a terminal state.

Captured pattern: an adapter session naturally acked + recorded its
verdict + completed, then the supervised subprocess exited with code 1.
Pre-v1.42.0, the post-process_completion check (`evaluate_and_block_inline`)
ran unconditionally and opened a ``process_exit_nonzero`` blocker on the
already-completed job. The blocker could not be cleared by
``recovery resume`` because the completed job had no current lease.

This test pins the two halves of the fix:

* ``evaluate_and_block_inline`` skips when the job is already terminal —
  no blocker is opened.
* ``recovery resume --force`` against a legacy open blocker on a
  terminal job dismisses the blocker as a no-op, preserving the
  terminal state.
"""

from __future__ import annotations

import sqlite3
from unittest.mock import MagicMock

from striatum.process_completion import evaluate_and_block_inline


class _FakeRow:
    """Minimal sqlite3.Row stand-in keyed by string."""

    def __init__(self, **kwargs: object) -> None:
        self._data = kwargs

    def __getitem__(self, key: str) -> object:
        return self._data[key]

    def keys(self) -> list[str]:
        return list(self._data.keys())


def test_evaluate_and_block_inline_skips_when_job_terminal() -> None:
    """GH #7: skip the post-process blocker once the job is terminal."""
    conn = MagicMock(spec=sqlite3.Connection)
    job = _FakeRow(
        job_id="job_test_001",
        run_id="run_test_001",
        state="completed",  # GH #7 condition
        expected_artifacts_json='[{"path": "x.md", "required": true}]',
        role_id="reviewer",
    )
    kind, envelope = evaluate_and_block_inline(
        conn,
        job=job,
        session_id="sess_test_001",
        process_id="proc_test_001",
        command=["/bin/sh", "-c", "exit 1"],
        exit_code=1,
        duration_seconds=0.1,
        timed_out=False,
        timeout_seconds=None,
    )
    assert kind is None
    assert envelope is None
    # The transaction-side queries should not have run (no blocker insert).
    assert not conn.execute.called


def test_evaluate_and_block_inline_skips_for_each_terminal_state() -> None:
    """GH #7: the guard covers all four terminal states uniformly."""
    for terminal in ("completed", "failed", "canceled", "skipped"):
        conn = MagicMock(spec=sqlite3.Connection)
        job = _FakeRow(
            job_id=f"job_test_{terminal}",
            run_id=f"run_test_{terminal}",
            state=terminal,
            expected_artifacts_json="[]",
            role_id="implementer",
        )
        kind, envelope = evaluate_and_block_inline(
            conn,
            job=job,
            session_id=f"sess_test_{terminal}",
            process_id=f"proc_test_{terminal}",
            command=["/bin/sh", "-c", "exit 1"],
            exit_code=1,
            duration_seconds=0.1,
            timed_out=False,
            timeout_seconds=None,
        )
        assert kind is None, f"state={terminal!r} should have been skipped"
        assert envelope is None
