"""GH #14 regression: ``recovery resume --force`` must clear a stale
``process_exit_nonzero`` blocker attached to an already-terminal job
even when the job has no current lease.

This pins the second half of GH #7's fix that the bug report against
v1.48.1 surfaced as still broken:

* The post-process-completion guard in
  :mod:`striatum.process_completion` already skips when the job is
  terminal — covered by ``test_gh7_terminal_blocker.py``.
* The recovery side must dismiss a legacy open blocker on a terminal
  job without requiring an active lease, because the lease was
  released on ``job.completed`` before the trailing nonzero exit
  signal opened the blocker.

The fixture mirrors the Engram reproduction shape:

  artifact published → verdict recorded (for review jobs) →
  job.completed (lease released, ``current_lease_id = NULL``) →
  process.exited code 1 → legacy ``process_exit_nonzero`` blocker

The legacy blocker is inserted directly because the current inline
guard prevents normal creation of the same row from the adapter path
(intentionally — the guard fixes the source of the bug). The test
exercises the recovery path operators must rely on for runs that
still carry the v1.48.1 leak.
"""

from __future__ import annotations

import json
from pathlib import Path

from striatum.legacy_sqlite.db import connect
from test_process_adapter import (
    _build_workflow,
    _data,
    _job_state,
    _open_blocker,
    _publish_artifact,
    _run_cli,
    _start_with,
)


def _set_job_state(repo: Path, job_id: str, state: str) -> None:
    conn = connect(repo)
    try:
        conn.execute(
            "UPDATE jobs SET state = ?, current_lease_id = NULL WHERE job_id = ?",
            (state, job_id),
        )
        conn.commit()
    finally:
        conn.close()


def _insert_legacy_process_exit_blocker(
    repo: Path, *, run_id: str, job_id: str, session_id: str
) -> str:
    """Construct a pre-v1.42 leaked blocker row attached to a job.

    Returns the new blocker_id. Mirrors
    :func:`striatum.process_completion.block_job_with_envelope` but
    skips the ``UPDATE jobs SET state = 'blocked'`` step — the bug
    being reproduced is exactly that the job is already terminal.
    """
    envelope = {
        "envelope_version": "striatum.process_adapter.envelope.v1",
        "process_id": "proc_legacy",
        "command": ["/bin/sh", "-c", "exit 1"],
        "exit_code": 1,
        "duration_seconds": 0.1,
        "timeout_seconds": None,
        "missing_artifact_paths": [],
        "review_verdict_missing": False,
        "recovery_commands": [
            "striatum recovery resume --blocker-id blk_legacy --force",
        ],
    }
    blocker_id = "blk_gh14_legacy_test_000000000000"
    conn = connect(repo)
    try:
        conn.execute(
            """
            INSERT INTO blockers (
              blocker_id, run_id, job_id, session_id, severity, blocker_kind,
              description, state, created_at, payload_json
            )
            VALUES (?, ?, ?, ?, 'blocked', 'process_exit_nonzero',
                    'process exited with non-zero code 1', 'open',
                    '2026-05-13T00:00:00Z', ?)
            """,
            (
                blocker_id,
                run_id,
                job_id,
                session_id,
                json.dumps(envelope, sort_keys=True),
            ),
        )
        conn.commit()
    finally:
        conn.close()
    return blocker_id


def _drive_to_completed(
    repo: Path,
) -> tuple[str, str, str, str]:
    """Build, start, claim, ack, publish-artifact, complete a one-job run.

    Returns (run_id, session_id, job_id, lease_id). Job is left in the
    ``completed`` terminal state with ``current_lease_id = NULL``.
    """
    workflow = _build_workflow(command=["bash", "-c", "exit 0"])
    run_id, session_id, job_id, lease_id, _msg = _start_with(repo, workflow)
    _publish_artifact(repo, session_id, job_id, lease_id, "docs/out/OUT.md")
    _data(
        _run_cli(
            repo,
            "complete",
            "--session-id",
            session_id,
            "--job-id",
            job_id,
            "--lease-id",
            lease_id,
            "--summary",
            "happy completion before trailing nonzero exit",
        )
    )
    assert _job_state(repo, job_id) == "completed"
    # Confirm lease was released to NULL — this is the precondition
    # the bug report calls out as the reason `recovery resume` failed
    # in v1.48.1.
    conn = connect(repo)
    try:
        row = conn.execute(
            "SELECT current_lease_id FROM jobs WHERE job_id = ?",
            (job_id,),
        ).fetchone()
    finally:
        conn.close()
    assert row is not None and row[0] is None
    return run_id, session_id, job_id, lease_id


def test_resume_force_dismisses_terminal_process_exit_blocker_without_lease(
    tmp_path: Path,
) -> None:
    """GH #14 core: terminal completed job, no lease, open
    ``process_exit_nonzero`` blocker → ``recovery resume --force`` resolves
    as terminal no-op and leaves the job ``completed``."""
    run_id, session_id, job_id, _lease_id = _drive_to_completed(tmp_path)
    blocker_id = _insert_legacy_process_exit_blocker(
        tmp_path, run_id=run_id, job_id=job_id, session_id=session_id
    )

    result = _data(
        _run_cli(
            tmp_path,
            "recovery",
            "resume",
            "--blocker-id",
            blocker_id,
            "--force",
        )
    )

    assert result["status"] == "resolved_terminal_no_op"
    assert result["blocker_kind"] == "process_exit_nonzero"
    assert result["completed_inline"] is False
    assert result["job_state"] == "completed"
    assert _job_state(tmp_path, job_id) == "completed"
    assert _open_blocker(tmp_path, job_id) is None


def test_resume_force_complete_against_terminal_job_is_terminal_no_op(
    tmp_path: Path,
) -> None:
    """GH #14 secondary: the exact CLI shape the Engram operator filed
    (``--force --complete --session-id <sid>``) must also short-circuit
    to the terminal no-op branch — the job is already completed, so
    completion has nothing more to do."""
    run_id, session_id, job_id, _lease_id = _drive_to_completed(tmp_path)
    blocker_id = _insert_legacy_process_exit_blocker(
        tmp_path, run_id=run_id, job_id=job_id, session_id=session_id
    )

    result = _data(
        _run_cli(
            tmp_path,
            "recovery",
            "resume",
            "--blocker-id",
            blocker_id,
            "--session-id",
            session_id,
            "--force",
            "--complete",
            "--summary",
            "engram-style retry against terminal completed run",
        )
    )

    assert result["status"] == "resolved_terminal_no_op"
    assert result["completed_inline"] is False
    assert _job_state(tmp_path, job_id) == "completed"
    assert _open_blocker(tmp_path, job_id) is None


def test_resume_without_force_still_refuses_terminal_process_exit_blocker(
    tmp_path: Path,
) -> None:
    """Acceptance #5: ``--force`` is still required for
    ``process_exit_nonzero`` even on terminal jobs — the no-op path
    must not be a stealth bypass of the operator-inspection gate."""
    run_id, session_id, job_id, _lease_id = _drive_to_completed(tmp_path)
    blocker_id = _insert_legacy_process_exit_blocker(
        tmp_path, run_id=run_id, job_id=job_id, session_id=session_id
    )

    refused = _run_cli(
        tmp_path,
        "recovery",
        "resume",
        "--blocker-id",
        blocker_id,
        check=False,
    )

    assert refused["returncode"] == 4
    # Either the active-lease check or the requires-force check fires
    # before the terminal branch; both are acceptable refusal shapes
    # as long as the operator-inspection gate stays closed.
    message = str(refused["error"]["message"])
    assert (
        "requires --force" in message
        or "must be blocked" in message
        or "no current lease" in message
    )
    assert _job_state(tmp_path, job_id) == "completed"
    # The blocker must remain open until the operator passes --force.
    assert _open_blocker(tmp_path, job_id) is not None


def test_recovery_auto_dismisses_terminal_process_blocker_when_policy_enabled(
    tmp_path: Path,
) -> None:
    """Acceptance #7: ``recovery auto --autonomous-process-reconcile``
    must no longer leave a terminal-state process-adapter blocker
    indefinitely stuck as ``blocker_recovery_eligible``."""
    run_id, session_id, job_id, _lease_id = _drive_to_completed(tmp_path)
    blocker_id = _insert_legacy_process_exit_blocker(
        tmp_path, run_id=run_id, job_id=job_id, session_id=session_id
    )

    result = _data(
        _run_cli(
            tmp_path,
            "recovery",
            "auto",
            "--run-id",
            run_id,
            "--autonomous-process-reconcile",
            "--eligible-after",
            "0",
        )
    )

    actions = result.get("actions", [])
    dismissals = [a for a in actions if a.get("kind") == "terminal_blocker_dismissed"]
    assert len(dismissals) == 1, (
        f"expected exactly one terminal-blocker dismissal action, got: {actions}"
    )
    assert dismissals[0]["blocker_id"] == blocker_id
    assert dismissals[0]["blocker_kind"] == "process_exit_nonzero"
    assert dismissals[0]["job_state"] == "completed"

    stuck = [
        s
        for s in result.get("still_stuck", [])
        if s.get("blocker_id") == blocker_id
    ]
    assert stuck == [], (
        f"terminal blocker must not also be reported as stuck: {stuck}"
    )
    assert _open_blocker(tmp_path, job_id) is None
    assert _job_state(tmp_path, job_id) == "completed"


def test_recovery_auto_without_policy_still_reports_terminal_blocker_eligible(
    tmp_path: Path,
) -> None:
    """Acceptance #7 inverse: when ``--autonomous-process-reconcile``
    is not set, ``recovery auto`` must not autonomously mutate
    blockers — it falls back to the today behavior of reporting the
    blocker as ``blocker_recovery_eligible``."""
    run_id, session_id, job_id, _lease_id = _drive_to_completed(tmp_path)
    blocker_id = _insert_legacy_process_exit_blocker(
        tmp_path, run_id=run_id, job_id=job_id, session_id=session_id
    )

    result = _data(
        _run_cli(
            tmp_path,
            "recovery",
            "auto",
            "--run-id",
            run_id,
            "--eligible-after",
            "0",
        )
    )

    actions = result.get("actions", [])
    assert not any(
        a.get("kind") == "terminal_blocker_dismissed" for a in actions
    ), f"autonomous dismissal must be gated on policy: {actions}"
    stuck = [
        s
        for s in result.get("still_stuck", [])
        if s.get("blocker_id") == blocker_id
    ]
    assert len(stuck) == 1
    assert stuck[0]["reason"] == "blocker_recovery_eligible"
    # Blocker is left open for the operator to clear manually.
    assert _open_blocker(tmp_path, job_id) is not None


def test_recovery_auto_dry_run_reports_terminal_dismiss_eligible(
    tmp_path: Path,
) -> None:
    """``--dry-run`` against ``--autonomous-process-reconcile`` must
    surface the planned terminal-blocker dismissal without mutating
    state, so operators can preview the autonomous cleanup before
    enabling it."""
    run_id, session_id, job_id, _lease_id = _drive_to_completed(tmp_path)
    blocker_id = _insert_legacy_process_exit_blocker(
        tmp_path, run_id=run_id, job_id=job_id, session_id=session_id
    )

    result = _data(
        _run_cli(
            tmp_path,
            "recovery",
            "auto",
            "--run-id",
            run_id,
            "--autonomous-process-reconcile",
            "--eligible-after",
            "0",
            "--dry-run",
        )
    )

    eligible = [
        a
        for a in result.get("actions", [])
        if a.get("kind") == "terminal_blocker_dismiss_eligible"
    ]
    assert len(eligible) == 1
    assert eligible[0]["blocker_id"] == blocker_id
    assert eligible[0]["dry_run"] is True
    # State must not have been mutated.
    assert _open_blocker(tmp_path, job_id) is not None


def test_resume_force_records_terminal_dismiss_event(tmp_path: Path) -> None:
    """Acceptance #4: the dismissal must emit a structured event
    operators can grep for. ``recovery.blocker_dismissed_terminal``
    is the existing GH #7 event; pin it here so future refactors
    keep the audit trail in place."""
    run_id, session_id, job_id, _lease_id = _drive_to_completed(tmp_path)
    blocker_id = _insert_legacy_process_exit_blocker(
        tmp_path, run_id=run_id, job_id=job_id, session_id=session_id
    )

    _data(
        _run_cli(
            tmp_path,
            "recovery",
            "resume",
            "--blocker-id",
            blocker_id,
            "--force",
        )
    )

    conn = connect(tmp_path)
    try:
        row = conn.execute(
            """
            SELECT payload_json FROM events
            WHERE event_type = 'recovery.blocker_dismissed_terminal'
              AND run_id = ?
            ORDER BY created_at DESC, event_id DESC
            LIMIT 1
            """,
            (run_id,),
        ).fetchone()
    finally:
        conn.close()
    assert row is not None, "terminal-dismissal event was not recorded"
    payload = json.loads(str(row[0]))
    assert payload["blocker_id"] == blocker_id
    assert payload["blocker_kind"] == "process_exit_nonzero"
    assert payload["job_state"] == "completed"
