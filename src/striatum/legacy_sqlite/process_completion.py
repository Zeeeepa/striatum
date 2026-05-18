"""RFC 0014 V1 process-adapter completion guarantees.

This module owns the post-exit validation logic for the one-shot
``adapter run`` path: build the privacy-safe diagnostic envelope,
inspect required artifacts and review-job verdicts, insert a blocker
row when outputs are missing, and transition the job to ``blocked``.

The envelope **never** contains child stdout, stderr, or model output
(D028). It carries only metadata Striatum already collected (process
id, command, exit code, duration) plus output-validation deltas
(missing artifact paths, review verdict missing) plus operator-
copyable recovery commands.
"""

from __future__ import annotations

import json
import sqlite3

from striatum.legacy_sqlite.db import (
    insert_event,
)
from striatum.primitives import JsonObject, new_id, utc_now


ENVELOPE_VERSION = "striatum.process_adapter.envelope.v1"

# RFC 0014 V1: priority order when multiple failure conditions hold
# simultaneously. Higher index in this tuple wins. Reconciler-path
# blockers are not in the inline-path priority — the reconciler skips
# rows that already have an open blocker for the same job.
_INLINE_REASON_PRIORITY = (
    "process_outputs_missing",
    "process_review_verdict_missing",
    "process_timeout_exceeded",
    "process_exit_nonzero",
)


def build_diagnostic_envelope(
    *,
    process_id: str,
    command: list[str],
    exit_code: int | None,
    duration_seconds: float,
    timeout_seconds: int | None,
    missing_artifact_paths: list[str],
    review_verdict_missing: bool,
    recovery_commands: list[str],
) -> JsonObject:
    """Assemble the V1 diagnostic envelope.

    The envelope is the canonical privacy-safe failure record stored on
    every process-adapter blocker row (``blockers.payload_json``) and
    embedded in the corresponding ``process_adapter.outputs_missing``
    event payload.
    """
    return {
        "envelope_version": ENVELOPE_VERSION,
        "process_id": process_id,
        "command": list(command),
        "exit_code": exit_code,
        "duration_seconds": round(duration_seconds, 3),
        "timeout_seconds": timeout_seconds,
        "missing_artifact_paths": list(missing_artifact_paths),
        "review_verdict_missing": review_verdict_missing,
        "recovery_commands": list(recovery_commands),
    }


def validate_outputs(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
) -> tuple[list[str], bool]:
    """Return ``(missing_artifact_paths, review_verdict_missing)``.

    ``missing_artifact_paths`` is the set of repo-relative paths from
    ``expected_artifacts`` where ``required: true`` and the path is
    not present in the ``artifacts`` table for this ``job_id``.

    ``review_verdict_missing`` is ``True`` only when the job is a
    review job and no row exists in ``verdicts`` for this ``job_id``.
    """
    raw = job["expected_artifacts_json"]
    try:
        expected = json.loads(str(raw)) if raw else []
    except json.JSONDecodeError:
        expected = []
    if not isinstance(expected, list):
        expected = []
    required_paths: list[str] = []
    for item in expected:
        if not isinstance(item, dict):
            continue
        if not item.get("required", True):
            continue
        path = item.get("path")
        if isinstance(path, str) and path:
            required_paths.append(path)
    if required_paths:
        rows = conn.execute(
            "SELECT repo_path FROM artifacts WHERE job_id = ?",
            (str(job["job_id"]),),
        ).fetchall()
        published = {str(r["repo_path"]) for r in rows}
        missing_paths = [p for p in required_paths if p not in published]
    else:
        missing_paths = []
    if str(job["job_type"]) == "review":
        verdict_row = conn.execute(
            "SELECT verdict_id FROM verdicts WHERE job_id = ? LIMIT 1",
            (str(job["job_id"]),),
        ).fetchone()
        verdict_missing = verdict_row is None
    else:
        verdict_missing = False
    return missing_paths, verdict_missing


def pick_inline_blocker_kind(
    *,
    exit_code: int | None,
    timed_out: bool,
    missing_artifact_paths: list[str],
    review_verdict_missing: bool,
) -> str | None:
    """Return the highest-priority blocker kind for an inline failure.

    Returns ``None`` when no failure condition holds.
    """
    if exit_code is not None and exit_code != 0:
        return "process_exit_nonzero"
    if timed_out:
        return "process_timeout_exceeded"
    if review_verdict_missing:
        return "process_review_verdict_missing"
    if missing_artifact_paths:
        return "process_outputs_missing"
    return None


def build_recovery_commands(
    *,
    run_id: str,
    job_id: str,
    session_id: str,
    blocker_kind: str,
    missing_artifact_paths: list[str],
    review_verdict_missing: bool,
    blocker_id: str | None = None,
) -> list[str]:
    """Return shell-string operator suggestions for the given failure mode.

    Process-adapter blockers keep the claimed lease in place. The recovery
    path therefore resolves the blocker back to ``running`` instead of trying
    the stale-lease requeue path, which intentionally refuses repo-write work.
    """
    cmds: list[str] = []
    for path in missing_artifact_paths:
        cmds.append(
            "striatum publish-artifact "
            f"--session-id {session_id} --job-id {job_id} "
            f"--lease-id <lease_id> --kind <kind> "
            f"--logical-name <logical_name> --path {path}"
        )
    if review_verdict_missing:
        cmds.append(
            "striatum verdict "
            f"--session-id {session_id} --job-id {job_id} "
            f"--lease-id <lease_id> --verdict <accept|accept_with_findings|needs_revision|reject>"
        )
    blocker_arg = blocker_id if blocker_id is not None else "<blocker_id>"
    force_arg = (
        " --force"
        if blocker_kind in {"process_exit_nonzero", "process_timeout_exceeded"}
        else ""
    )
    cmds.append(f"striatum recovery resume --blocker-id {blocker_arg}{force_arg}")
    if not review_verdict_missing:
        cmds.append(
            f"striatum recovery resume --blocker-id {blocker_arg}{force_arg} "
            f"--complete --session-id {session_id} --summary \"<summary>\""
        )
    if blocker_kind == "process_lost_with_outputs_missing":
        cmds.append(f"striatum recovery process-reconcile --run-id {run_id}")
    return cmds


def block_job_with_envelope(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
    session_id: str,
    blocker_kind: str,
    description: str,
    envelope: JsonObject,
) -> str:
    """Insert a blocker row, transition the job to ``blocked``, emit the event.

    Returns the new ``blocker_id``. Caller is responsible for the
    enclosing transaction. The lease is intentionally NOT released —
    RFC 0014 § "Job state transition" leaves the lease in place so
    operators can inspect via ``striatum recovery requeue-stale`` /
    ``recovery process-reconcile``.
    """
    now = utc_now()
    blocker_id = new_id("blk")
    missing_paths = [
        str(path)
        for path in envelope.get("missing_artifact_paths", [])
        if isinstance(path, str)
    ]
    review_verdict_missing = bool(envelope.get("review_verdict_missing"))
    final_envelope = dict(envelope)
    final_envelope["recovery_commands"] = build_recovery_commands(
        run_id=str(job["run_id"]),
        job_id=str(job["job_id"]),
        session_id=session_id,
        blocker_kind=blocker_kind,
        missing_artifact_paths=missing_paths,
        review_verdict_missing=review_verdict_missing,
        blocker_id=blocker_id,
    )
    envelope_json = json.dumps(final_envelope, sort_keys=True)
    conn.execute(
        """
        INSERT INTO blockers (
          blocker_id, run_id, job_id, session_id, severity, blocker_kind,
          description, state, created_at, payload_json
        )
        VALUES (?, ?, ?, ?, 'blocked', ?, ?, 'open', ?, ?)
        """,
        (
            blocker_id,
            str(job["run_id"]),
            str(job["job_id"]),
            session_id,
            blocker_kind,
            description,
            now,
            envelope_json,
        ),
    )
    conn.execute(
        "UPDATE jobs SET state = 'blocked' WHERE job_id = ?",
        (str(job["job_id"]),),
    )
    insert_event(
        conn,
        run_id=str(job["run_id"]),
        event_type="process_adapter.outputs_missing",
        actor_session_id=session_id,
        job_id=str(job["job_id"]),
        payload={"blocker_id": blocker_id, "envelope": final_envelope},
    )
    return blocker_id


def evaluate_and_block_inline(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
    session_id: str,
    process_id: str,
    command: list[str],
    exit_code: int | None,
    duration_seconds: float,
    timed_out: bool,
    timeout_seconds: int | None,
) -> tuple[str | None, JsonObject | None]:
    """Run output validation and block the job if any failure condition holds.

    Returns ``(blocker_kind, envelope)`` when a blocker was inserted,
    or ``(None, None)`` when the process completed cleanly with all
    required outputs present.

    Caller is responsible for the enclosing transaction. The function
    is idempotent against an already-blocked job: if a blocker already
    exists for this job, it returns ``(None, None)`` without inserting
    a duplicate.
    """
    # GH #7: when the job has already reached a terminal state (the
    # adapter session naturally acked + verdict-recorded + completed
    # before exiting nonzero), skip the post-completion blocker. The
    # outputs are by definition present (job.completed required them),
    # and no recovery is needed — the nonzero exit is a benign trailing
    # signal from the supervised process, not a workflow failure.
    job_state = str(job["state"]) if "state" in job.keys() else ""
    if job_state in {"completed", "failed", "canceled", "skipped"}:
        return None, None
    missing_paths, verdict_missing = validate_outputs(conn, job=job)
    blocker_kind = pick_inline_blocker_kind(
        exit_code=exit_code,
        timed_out=timed_out,
        missing_artifact_paths=missing_paths,
        review_verdict_missing=verdict_missing,
    )
    if blocker_kind is None:
        return None, None
    existing = conn.execute(
        "SELECT blocker_id FROM blockers WHERE job_id = ? AND state = 'open' LIMIT 1",
        (str(job["job_id"]),),
    ).fetchone()
    if existing is not None:
        return None, None
    recovery = build_recovery_commands(
        run_id=str(job["run_id"]),
        job_id=str(job["job_id"]),
        session_id=session_id,
        blocker_kind=blocker_kind,
        missing_artifact_paths=missing_paths,
        review_verdict_missing=verdict_missing,
    )
    envelope = build_diagnostic_envelope(
        process_id=process_id,
        command=command,
        exit_code=exit_code,
        duration_seconds=duration_seconds,
        timeout_seconds=timeout_seconds,
        missing_artifact_paths=missing_paths,
        review_verdict_missing=verdict_missing,
        recovery_commands=recovery,
    )
    description = _describe(blocker_kind, missing_paths, verdict_missing, exit_code, timeout_seconds)
    block_job_with_envelope(
        conn,
        job=job,
        session_id=session_id,
        blocker_kind=blocker_kind,
        description=description,
        envelope=envelope,
    )
    return blocker_kind, envelope


def evaluate_and_block_after_reconcile(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
    session_id: str,
    process_id: str,
    command: list[str],
    duration_seconds: float,
) -> tuple[str | None, JsonObject | None]:
    """Run output validation for a row the reconciler just transitioned to ``lost``.

    Same return shape as :func:`evaluate_and_block_inline`. Uses the
    reconciler-specific blocker kind ``process_lost_with_outputs_missing``
    when outputs are missing; for a lost-but-cleanly-published case
    returns ``(None, None)``.

    Idempotent against an already-blocked job.
    """
    missing_paths, verdict_missing = validate_outputs(conn, job=job)
    if not missing_paths and not verdict_missing:
        return None, None
    existing = conn.execute(
        "SELECT blocker_id FROM blockers WHERE job_id = ? AND state = 'open' LIMIT 1",
        (str(job["job_id"]),),
    ).fetchone()
    if existing is not None:
        return None, None
    blocker_kind = "process_lost_with_outputs_missing"
    recovery = build_recovery_commands(
        run_id=str(job["run_id"]),
        job_id=str(job["job_id"]),
        session_id=session_id,
        blocker_kind=blocker_kind,
        missing_artifact_paths=missing_paths,
        review_verdict_missing=verdict_missing,
    )
    envelope = build_diagnostic_envelope(
        process_id=process_id,
        command=command,
        exit_code=None,
        duration_seconds=duration_seconds,
        timeout_seconds=None,
        missing_artifact_paths=missing_paths,
        review_verdict_missing=verdict_missing,
        recovery_commands=recovery,
    )
    description = (
        f"process {process_id} was lost (external kill or runner exit); "
        f"required outputs missing: "
        f"{len(missing_paths)} artifact(s), verdict missing={verdict_missing}"
    )
    block_job_with_envelope(
        conn,
        job=job,
        session_id=session_id,
        blocker_kind=blocker_kind,
        description=description,
        envelope=envelope,
    )
    return blocker_kind, envelope


def _describe(
    blocker_kind: str,
    missing_paths: list[str],
    verdict_missing: bool,
    exit_code: int | None,
    timeout_seconds: int | None,
) -> str:
    """Short human description for the blocker row."""
    if blocker_kind == "process_exit_nonzero":
        return f"process exited with non-zero code {exit_code}"
    if blocker_kind == "process_timeout_exceeded":
        return f"process exceeded {timeout_seconds}s timeout; SIGTERM sent"
    if blocker_kind == "process_review_verdict_missing":
        suffix = (
            f"; also missing {len(missing_paths)} artifact(s)"
            if missing_paths
            else ""
        )
        return f"review job exited cleanly without recording a verdict{suffix}"
    if blocker_kind == "process_outputs_missing":
        return (
            f"process exited cleanly without producing "
            f"{len(missing_paths)} required artifact(s)"
        )
    return blocker_kind
