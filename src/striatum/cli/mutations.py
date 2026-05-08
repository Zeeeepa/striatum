"""State-mutation handlers for the Striatum CLI."""

from __future__ import annotations

import json
import sqlite3
import subprocess
from datetime import UTC, datetime, timedelta
from pathlib import Path

from striatum.artifacts import publish_artifact
from striatum.db import (
    JsonObject,
    active_lease_for,
    enqueue_job,
    insert_event,
    json_loads,
    maybe_complete_run,
    new_id,
    record_review_verdict,
    repo_relative_path,
    row_by_id,
    sha256_bytes,
    transaction,
    utc_now,
)
from striatum.errors import (
    ArtifactError,
    InvalidTransitionError,
    LeaseError,
    NotFoundError,
    WorkflowError,
)

from striatum.cli.introspect import downstream_jobs


def branch_confirm(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    run_id: str,
    branch: str,
    create: bool = False,
    use_current: bool = False,
    strict: bool = False,
) -> JsonObject:
    """Record branch confirmation.

    Default mode is records-only: write the row, run an advisory git check,
    and emit a warning if the current branch disagrees. The opt-in modes
    actually drive git or refuse to record:

    - ``create``: run ``git checkout -b <branch>`` (idempotent fallback to
      ``git checkout <branch>`` if the branch already exists). Surface git
      errors and exit with ``WorkflowError`` (code 8).
    - ``use_current``: ignore the requested branch as a target and record the
      current git branch instead. Conflicts with a non-matching ``--branch``.
    - ``strict``: refuse to record unless the current git branch already
      matches ``--branch`` exactly.
    """
    if create and use_current:
        raise WorkflowError("--create and --use-current are mutually exclusive")
    if strict and (create or use_current):
        raise WorkflowError("--strict is incompatible with --create and --use-current")

    requested_branch = branch
    created = False
    mode = "records_only"

    if use_current:
        mode = "use_current"
        current = current_git_branch(repo)
        if current is None:
            raise WorkflowError(
                "--use-current requires a detectable current git branch in the target repo"
            )
        if branch != current:
            raise WorkflowError(
                f"--use-current was given but --branch={branch!r} does not match current git branch {current!r}"
            )
        target_branch = current
    elif create:
        mode = "create"
        target_branch, created = git_create_or_checkout_branch(repo, branch)
    elif strict:
        mode = "strict"
        current = current_git_branch(repo)
        if current != branch:
            raise WorkflowError(
                f"--strict requires current git branch to match --branch={branch!r}; "
                f"current branch is {current!r}"
            )
        target_branch = branch
    else:
        target_branch = branch

    with transaction(conn):
        run = row_by_id(conn, "runs", "run_id", run_id)
        if run["state"] not in ("needs_branch_confirmation", "ready"):
            raise InvalidTransitionError("run is not waiting for branch confirmation")
        current_branch = current_git_branch(repo)
        now = utc_now()
        conn.execute(
            """
            UPDATE runs
            SET branch_name = ?, branch_confirmed_at = ?, branch_confirmed_by = 'human',
                state = 'ready'
            WHERE run_id = ?
            """,
            (target_branch, now, run_id),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="run.branch_confirmed",
            payload={"branch": target_branch, "mode": mode, "created": created},
        )
        warning = None
        if current_branch is not None and current_branch != target_branch:
            warning = "current git branch differs from recorded branch confirmation"
        return {
            "run_id": run_id,
            "state": "ready",
            "branch": target_branch,
            "requested_branch": requested_branch,
            "current_git_branch": current_branch,
            "records_only": True,
            "warning": warning,
            "created": created,
            "mode": mode,
        }


def git_create_or_checkout_branch(repo: Path, branch: str) -> tuple[str, bool]:
    """Create ``branch`` via ``git checkout -b`` or fall back to checkout.

    Returns ``(branch, created)`` where ``created`` is True only when the
    branch did not exist beforehand. Raises ``WorkflowError`` if both git
    invocations fail; the latter stderr is included (truncated) so the user
    can diagnose dirty working trees and similar problems.
    """
    create_result = subprocess.run(
        ["git", "checkout", "-b", branch],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    if create_result.returncode == 0:
        return branch, True
    checkout_result = subprocess.run(
        ["git", "checkout", branch],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    if checkout_result.returncode == 0:
        return branch, False
    stderr = (checkout_result.stderr or create_result.stderr or "").strip()
    if len(stderr) > 200:
        stderr = stderr[:200] + "..."
    raise WorkflowError(
        f"git checkout failed for branch {branch!r}: {stderr}" if stderr
        else f"git checkout failed for branch {branch!r}"
    )


def current_git_branch(repo: Path) -> str | None:
    """Return the current Git branch when detectable."""
    result = subprocess.run(
        ["git", "branch", "--show-current"],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    branch = result.stdout.strip()
    if result.returncode != 0 or branch == "":
        return None
    return branch


def run_start(conn: sqlite3.Connection, *, run_id: str) -> JsonObject:
    """Start a prepared run and enqueue root jobs."""
    with transaction(conn):
        run = row_by_id(conn, "runs", "run_id", run_id)
        if run["state"] == "needs_branch_confirmation":
            raise WorkflowError("branch confirmation is required before run start")
        if run["state"] not in ("ready", "running"):
            raise InvalidTransitionError("run cannot be started from its current state")
        if run["state"] == "ready":
            now = utc_now()
            conn.execute("UPDATE runs SET state = 'running', started_at = ? WHERE run_id = ?", (now, run_id))
            roots = conn.execute(
                """
                SELECT j.job_id
                FROM jobs j
                WHERE j.run_id = ?
                  AND NOT EXISTS (
                    SELECT 1 FROM job_dependencies dep WHERE dep.job_id = j.job_id
                  )
                ORDER BY j.created_at
                """,
                (run_id,),
            ).fetchall()
            from striatum.db import enqueue_job

            for root in roots:
                enqueue_job(conn, job_id=str(root["job_id"]))
            insert_event(conn, run_id=run_id, event_type="run.started")
        return {"run_id": run_id, "state": "running"}


def register_session(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    role: str,
    lane: str,
    capabilities: list[str],
    fresh: bool,
    parent_session_id: str | None,
    force_non_fresh: bool = False,
    non_fresh_reason: str | None = None,
) -> JsonObject:
    """Register an agent session.

    HARNESS-003 policy: when the workflow declares any review job with
    ``reviewer_context_policy: fresh`` and an active author session
    already exists on the run, refuse a reviewer-role registration
    unless ``force_non_fresh=True`` is passed with a non-empty
    ``non_fresh_reason``. The reason is stored on the session row so
    evidence exports record the explicit breach. The runner cannot tell
    whether the operator is the same human driving both lanes — this
    advisory refusal at least forces an explicit override.
    """
    with transaction(conn):
        run = row_by_id(conn, "runs", "run_id", run_id)
        snapshot = row_by_id(
            conn,
            "workflow_snapshots",
            "workflow_snapshot_id",
            str(run["workflow_snapshot_id"]),
        )
        workflow = json_loads(str(snapshot["workflow_json"]))
        roles = workflow.get("roles", {})
        lanes = workflow.get("lanes", {})
        if not isinstance(roles, dict) or role not in roles:
            raise InvalidTransitionError(f"unknown role {role!r} for run")
        if not isinstance(lanes, dict) or lane not in lanes:
            raise InvalidTransitionError(f"unknown lane {lane!r} for run")
        recorded_non_fresh_reason: str | None = None
        if role == "reviewer" and _workflow_declares_fresh_reviewer(workflow):
            other_author_active = conn.execute(
                """
                SELECT 1 FROM sessions
                WHERE run_id = ? AND role_id = 'author' AND state = 'active'
                LIMIT 1
                """,
                (run_id,),
            ).fetchone()
            if other_author_active is not None:
                if not force_non_fresh:
                    raise InvalidTransitionError(
                        "workflow declares reviewer_context_policy: fresh and an "
                        "active author session already exists on this run; pass "
                        "--force-non-fresh --reason \"...\" to register a non-fresh "
                        "reviewer explicitly"
                    )
                if non_fresh_reason is None or not non_fresh_reason.strip():
                    raise InvalidTransitionError(
                        "--force-non-fresh requires a non-empty --reason"
                    )
                recorded_non_fresh_reason = non_fresh_reason.strip()
        ordinal_row = conn.execute(
            """
            SELECT COALESCE(MAX(ordinal), 0) + 1 AS next_ordinal
            FROM sessions WHERE run_id = ? AND role_id = ? AND lane_id = ?
            """,
            (run_id, role, lane),
        ).fetchone()
        ordinal = int(ordinal_row["next_ordinal"])
        session_id = new_id("sess")
        slug = f"{role}-{lane}-{ordinal}"
        now = utc_now()
        conn.execute(
            """
            INSERT INTO sessions (
              session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, parent_session_id, fresh_context, state,
              registered_at, last_heartbeat_at, non_fresh_reason
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?)
            """,
            (
                session_id,
                run_id,
                role,
                lane,
                slug,
                ordinal,
                json.dumps(capabilities),
                parent_session_id,
                1 if fresh else 0,
                now,
                now,
                recorded_non_fresh_reason,
            ),
        )
        payload: JsonObject = {"role": role, "lane": lane, "slug": slug}
        if recorded_non_fresh_reason is not None:
            payload["non_fresh_reason"] = recorded_non_fresh_reason
        insert_event(
            conn,
            run_id=run_id,
            event_type="session.registered",
            actor_session_id=session_id,
            payload=payload,
        )
        return {"session_id": session_id, "slug": slug}


def _workflow_declares_fresh_reviewer(workflow: JsonObject) -> bool:
    """Return True when any review job declares ``reviewer_context_policy: fresh``."""
    jobs = workflow.get("jobs")
    if not isinstance(jobs, list):
        return False
    for job in jobs:
        if not isinstance(job, dict):
            continue
        if job.get("type") != "review":
            continue
        if job.get("reviewer_context_policy") == "fresh":
            return True
        if job.get("fresh_session_required") is True:
            return True
    return False


# RFC 0011 — session close + run-terminal auto-close.
#
# Both flows transition a session from ``active`` to ``closed`` and emit a
# ``session.closed`` event. The explicit ``striatum session close`` command
# surfaces ``close_session``; the auto-close helper
# ``close_remaining_sessions`` is invoked from every run-terminal transition
# point (currently only ``maybe_complete_run`` covers ``failed`` and
# ``completed``; future ``canceled`` transitions should call the helper too).

_SESSION_TERMINAL_STATES = ("expired", "stopped", "lost", "closed")


def close_session(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    reason: str,
) -> JsonObject:
    """Explicitly close a session.

    Idempotent against an already-terminal row: returns the existing
    state plus a ``note`` describing the prior state. Refuses with
    :class:`LeaseError` when the session is ``active`` and holds an
    active lease — closing the session out from under a lease would
    orphan the packet. The error message points the operator at
    ``striatum release``.

    Emits a ``session.closed`` event whose payload includes
    ``source: "explicit"`` so evidence consumers can distinguish the
    explicit-close case from auto-close.
    """
    cleaned_reason = (reason or "").strip()
    if cleaned_reason == "":
        raise InvalidTransitionError("session close reason must not be empty")
    with transaction(conn):
        session = row_by_id(conn, "sessions", "session_id", session_id)
        if session["state"] in _SESSION_TERMINAL_STATES:
            return {
                "session_id": str(session["session_id"]),
                "run_id": str(session["run_id"]),
                "role_id": str(session["role_id"]),
                "lane_id": str(session["lane_id"]),
                "state": str(session["state"]),
                "closed_at": session["closed_at"],
                "close_reason": session["close_reason"],
                "note": f"session was already {session['state']}",
            }
        active_lease = conn.execute(
            "SELECT lease_id FROM leases WHERE owner_session_id = ? AND state = 'active' LIMIT 1",
            (session_id,),
        ).fetchone()
        if active_lease is not None:
            raise LeaseError(
                f"session has an active lease ({active_lease['lease_id']}); "
                "release the lease (striatum release) before closing the session"
            )
        now = utc_now()
        conn.execute(
            """
            UPDATE sessions
            SET state = 'closed', closed_at = ?, close_reason = ?
            WHERE session_id = ?
            """,
            (now, cleaned_reason, session_id),
        )
        insert_event(
            conn,
            run_id=str(session["run_id"]),
            event_type="session.closed",
            actor_session_id=session_id,
            payload={
                "session_id": session_id,
                "role_id": session["role_id"],
                "lane_id": session["lane_id"],
                "reason": cleaned_reason,
                "source": "explicit",
            },
        )
        return {
            "session_id": session_id,
            "run_id": str(session["run_id"]),
            "role_id": str(session["role_id"]),
            "lane_id": str(session["lane_id"]),
            "state": "closed",
            "closed_at": now,
            "close_reason": cleaned_reason,
        }


def ack_work(conn: sqlite3.Connection, *, session_id: str, message_id: str, lease_id: str) -> JsonObject:
    """Acknowledge claimed work and mark it running."""
    with transaction(conn):
        message = row_by_id(conn, "queue_messages", "message_id", message_id)
        job = row_by_id(conn, "jobs", "job_id", str(message["job_id"]))
        active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=str(job["job_id"]))
        if message["state"] == "acked":
            return {"status": "acked", "job_id": job["job_id"]}
        if message["state"] != "claimed" or job["state"] != "claimed":
            raise InvalidTransitionError("work must be claimed before ack")
        now = utc_now()
        conn.execute(
            "UPDATE queue_messages SET state = 'acked', acked_at = ?, updated_at = ? WHERE message_id = ?",
            (now, now, message_id),
        )
        conn.execute("UPDATE jobs SET state = 'running', started_at = ? WHERE job_id = ?", (now, job["job_id"]))
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="queue.acked",
            actor_session_id=session_id,
            job_id=str(job["job_id"]),
            message_id=message_id,
            lease_id=lease_id,
        )
        return {"status": "acked", "job_id": job["job_id"]}


def heartbeat(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    lease_id: str,
    extend_seconds: int,
) -> JsonObject:
    """Refresh session and lease liveness."""
    with transaction(conn):
        lease = active_lease_for(conn, lease_id=lease_id, session_id=session_id)
        now = utc_now()
        expires_at = (
            datetime.now(UTC) + timedelta(seconds=extend_seconds)
        ).replace(microsecond=0).isoformat().replace("+00:00", "Z")
        conn.execute(
            "UPDATE sessions SET last_heartbeat_at = ? WHERE session_id = ?",
            (now, session_id),
        )
        conn.execute(
            "UPDATE leases SET last_heartbeat_at = ?, expires_at = ? WHERE lease_id = ?",
            (now, expires_at, lease_id),
        )
        insert_event(
            conn,
            run_id=str(lease["run_id"]),
            event_type="lease.heartbeat",
            actor_session_id=session_id,
            job_id=str(lease["resource_id"]),
            lease_id=lease_id,
            payload={"expires_at": expires_at},
        )
        return {"status": "heartbeat", "expires_at": expires_at}


def release_work(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    message_id: str,
    lease_id: str,
    reason: str,
    requeue: bool,
) -> JsonObject:
    """Release claimed work."""
    with transaction(conn):
        message = row_by_id(conn, "queue_messages", "message_id", message_id)
        job = row_by_id(conn, "jobs", "job_id", str(message["job_id"]))
        active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=str(job["job_id"]))
        from striatum.db import is_repo_write

        now = utc_now()
        if requeue and not is_repo_write(job):
            job_state = "queued"
            msg_state = "pending"
        else:
            job_state = "blocked"
            msg_state = "blocked"
        conn.execute(
            "UPDATE leases SET state = 'released', released_at = ?, release_reason = ? WHERE lease_id = ?",
            (now, reason, lease_id),
        )
        conn.execute(
            "UPDATE jobs SET state = ?, current_lease_id = NULL WHERE job_id = ?",
            (job_state, job["job_id"]),
        )
        conn.execute(
            """
            UPDATE queue_messages
            SET state = ?, current_lease_id = NULL, updated_at = ?
            WHERE message_id = ?
            """,
            (msg_state, now, message_id),
        )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="lease.released",
            actor_session_id=session_id,
            job_id=str(job["job_id"]),
            message_id=message_id,
            lease_id=lease_id,
            payload={"reason": reason, "job_state": job_state},
        )
        return {"status": "released", "job_state": job_state}


def send_message(conn: sqlite3.Connection, *, session_id: str, kind: str, body_json: str) -> JsonObject:
    """Write a structured message event."""
    with transaction(conn):
        session = row_by_id(conn, "sessions", "session_id", session_id)
        body = json.loads(body_json)
        if not isinstance(body, dict):
            raise InvalidTransitionError("message body must be a JSON object")
        message_id = new_id("msg")
        now = utc_now()
        conn.execute(
            """
            INSERT INTO queue_messages (
              message_id, run_id, kind, state, payload_json, created_at, updated_at
            )
            VALUES (?, ?, 'agent_message', 'completed', ?, ?, ?)
            """,
            (message_id, session["run_id"], json.dumps({"kind": kind, "body": body}), now, now),
        )
        insert_event(
            conn,
            run_id=str(session["run_id"]),
            event_type="message.sent",
            actor_session_id=session_id,
            message_id=message_id,
            payload={"kind": kind},
        )
        return {"message_id": message_id}


def block_work(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    kind: str,
    severity: str,
    description: str,
) -> JsonObject:
    """Record a blocker and stop the job."""
    with transaction(conn):
        job = row_by_id(conn, "jobs", "job_id", job_id)
        active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=job_id)
        now = utc_now()
        blocker_id = new_id("blk")
        state = "waiting_human" if severity == "human_checkpoint" else "blocked"
        conn.execute(
            """
            INSERT INTO blockers (
              blocker_id, run_id, job_id, session_id, severity, blocker_kind,
              description, state, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, 'open', ?)
            """,
            (blocker_id, job["run_id"], job_id, session_id, severity, kind, description, now),
        )
        conn.execute("UPDATE jobs SET state = ?, current_lease_id = NULL WHERE job_id = ?", (state, job_id))
        conn.execute(
            "UPDATE leases SET state = 'released', released_at = ?, release_reason = 'blocked' WHERE lease_id = ?",
            (now, lease_id),
        )
        if job["current_message_id"] is not None:
            conn.execute(
                "UPDATE queue_messages SET state = 'blocked', current_lease_id = NULL WHERE message_id = ?",
                (job["current_message_id"],),
            )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="job.blocked",
            actor_session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            payload={"blocker_id": blocker_id, "severity": severity},
        )
        return {"status": "blocked", "blocker_id": blocker_id}


def verdict_work(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    verdict: str,
    findings_artifact_id: str | None,
    rationale: str | None,
) -> JsonObject:
    """Record a review verdict and apply review-gate behavior."""
    return record_review_verdict(
        conn,
        session_id=session_id,
        job_id=job_id,
        lease_id=lease_id,
        verdict=verdict,
        findings_artifact_id=findings_artifact_id,
        rationale=rationale,
    )


def submit_review(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    session_id: str,
    job_id: str,
    lease_id: str,
    path_text: str,
    verdict: str,
    logical_name: str,
    kind: str,
    rationale: str | None,
) -> JsonObject:
    """Publish a review artifact and record its verdict in one command."""
    job = row_by_id(conn, "jobs", "job_id", job_id)
    prevalidate_submit_review(
        conn,
        job=job,
        session_id=session_id,
        lease_id=lease_id,
        logical_name=logical_name,
        kind=kind,
        path_text=path_text,
    )
    if job["state"] == "claimed" and job["current_message_id"] is not None:
        ack_work(
            conn,
            session_id=session_id,
            message_id=str(job["current_message_id"]),
            lease_id=lease_id,
        )
    artifact = publish_artifact(
        conn,
        repo=repo,
        session_id=session_id,
        job_id=job_id,
        lease_id=lease_id,
        kind=kind,
        logical_name=logical_name,
        path_text=path_text,
    )
    verdict_result = record_review_verdict(
        conn,
        session_id=session_id,
        job_id=job_id,
        lease_id=lease_id,
        verdict=verdict,
        findings_artifact_id=str(artifact["artifact_id"]),
        rationale=rationale,
    )
    job = row_by_id(conn, "jobs", "job_id", job_id)
    run = row_by_id(conn, "runs", "run_id", str(job["run_id"]))
    return {
        "artifact": artifact,
        "verdict": verdict_result,
        "job_state": job["state"],
        "run_state": run["state"],
        "blocker_id": verdict_result.get("blocker_id"),
        "downstream_jobs": downstream_jobs(conn, job_id=job_id),
    }


def prevalidate_submit_review(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
    session_id: str,
    lease_id: str,
    logical_name: str,
    kind: str,
    path_text: str,
) -> None:
    """Reject submit-review calls that would fail after artifact publication."""
    if job["job_type"] != "review":
        raise InvalidTransitionError("submit-review is valid only for review jobs")
    if job["state"] not in {"claimed", "running"}:
        raise InvalidTransitionError("review job must be claimed or running before submit-review")
    if job["state"] == "claimed" and job["current_message_id"] is None:
        raise InvalidTransitionError("claimed review job is missing its current message")
    active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=str(job["job_id"]))
    expected = json.loads(str(job["expected_artifacts_json"]))
    if not isinstance(expected, list):
        raise InvalidTransitionError("expected artifacts must be a list")
    for item in expected:
        if not isinstance(item, dict) or item.get("required") is not True:
            continue
        expected_logical_name = item.get("logical_name")
        expected_kind = item.get("kind")
        expected_path = item.get("path")
        if (expected_logical_name, expected_kind, expected_path) == (logical_name, kind, path_text):
            continue
        found = conn.execute(
            """
            SELECT 1 FROM artifacts
            WHERE job_id = ? AND logical_name = ? AND artifact_kind = ? AND repo_path = ?
            LIMIT 1
            """,
            (job["job_id"], expected_logical_name, expected_kind, expected_path),
        ).fetchone()
        if found is None:
            raise InvalidTransitionError(
                "required artifact would still be missing after submit-review: "
                f"logical_name={expected_logical_name!r}, kind={expected_kind!r}, path={expected_path!r}"
            )


def decision_record(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    run_id: str,
    path_text: str,
    outcome: str,
    title: str,
    decision_id: str | None,
    rationale: str | None,
    follow_up: str | None,
) -> JsonObject:
    """Write and record an owner decision artifact without requiring a lease."""
    row_by_id(conn, "runs", "run_id", run_id)
    title = title.strip()
    if title == "":
        raise ArtifactError("decision title cannot be empty")
    if outcome == "accepted_with_follow_up" and (follow_up is None or follow_up.strip() == ""):
        raise ArtifactError("accepted_with_follow_up decisions require --follow-up")
    resolved_decision_id = decision_id.strip() if decision_id is not None else new_id("dec")
    if resolved_decision_id == "" or any(character.isspace() for character in resolved_decision_id):
        raise ArtifactError("decision id cannot be empty or contain whitespace")
    existing = conn.execute(
        """
        SELECT artifact_id FROM artifacts
        WHERE run_id = ? AND artifact_kind = 'decision'
          AND (logical_name = ? OR repo_path = ?)
        LIMIT 1
        """,
        (run_id, resolved_decision_id, path_text),
    ).fetchone()
    if existing is not None:
        raise ArtifactError("decision artifact already exists for this run id/path")
    target = repo_relative_path(repo, path_text)
    if target.exists():
        raise ArtifactError("decision artifact path already exists")
    target.parent.mkdir(parents=True, exist_ok=True)
    created_at = utc_now()
    body = render_decision_markdown(
        decision_id=resolved_decision_id,
        run_id=run_id,
        outcome=outcome,
        title=title,
        created_at=created_at,
        rationale=rationale,
        follow_up=follow_up,
    )
    target.write_text(body, encoding="utf-8")
    payload = body.encode("utf-8")
    digest = sha256_bytes(payload)
    artifact_id = new_id("art")
    with transaction(conn):
        conn.execute(
            """
            INSERT INTO artifacts (
              artifact_id, run_id, job_id, session_id, logical_name,
              artifact_kind, repo_path, content_sha256, size_bytes,
              publish_mode, created_at
            )
            VALUES (?, ?, NULL, NULL, ?, 'decision', ?, ?, ?, 'create', ?)
            """,
            (
                artifact_id,
                run_id,
                resolved_decision_id,
                path_text,
                digest,
                len(payload),
                created_at,
            ),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="decision.recorded",
            artifact_id=artifact_id,
            payload={
                "decision_id": resolved_decision_id,
                "outcome": outcome,
                "path": path_text,
                "sha256": digest,
            },
        )
    return {
        "status": "recorded",
        "run_id": run_id,
        "decision_id": resolved_decision_id,
        "artifact_id": artifact_id,
        "path": path_text,
        "outcome": outcome,
        "sha256": digest,
    }


def render_decision_markdown(
    *,
    decision_id: str,
    run_id: str,
    outcome: str,
    title: str,
    created_at: str,
    rationale: str | None,
    follow_up: str | None,
) -> str:
    """Render a machine-checkable owner decision Markdown artifact."""
    follow_up_required = outcome == "accepted_with_follow_up"
    lines = [
        "---",
        "schema_version: striatum.decision.v1",
        f"decision_id: {json.dumps(decision_id)}",
        f"run_id: {json.dumps(run_id)}",
        "artifact_kind: decision",
        "owner: human",
        f"outcome: {outcome}",
        f"follow_up_required: {str(follow_up_required).lower()}",
        f"title: {json.dumps(title)}",
        f"created_at: {json.dumps(created_at)}",
        "---",
        "",
        f"# {title}",
        "",
        f"Decision ID: `{decision_id}`",
        f"Run ID: `{run_id}`",
        f"Outcome: `{outcome}`",
        "",
    ]
    if rationale is not None and rationale.strip() != "":
        lines.extend(["## Rationale", "", rationale.strip(), ""])
    if follow_up is not None and follow_up.strip() != "":
        lines.extend(["## Follow-Up", "", follow_up.strip(), ""])
    return "\n".join(lines)


def checkpoint_resolve(
    conn: sqlite3.Connection,
    *,
    blocker_id: str,
    action: str,
    decision_id: str | None,
) -> JsonObject:
    """Resolve an open ``human_checkpoint`` blocker.

    The ``continue`` action closes the blocker and returns the affected job to
    a claimable state (``queued`` with its existing message restored to
    ``pending``); the ``cancel`` action closes the blocker and transitions the
    affected job to ``canceled``, leaving downstream blocked jobs untouched
    (they remain blocked because their dependency was canceled). When
    ``decision_id`` is provided the corresponding run-level decision artifact
    is validated and its ``decision_id`` is recorded on the resolution event
    payload so audit can link the resolution back to the operator's decision
    artifact.
    """
    if action not in {"continue", "cancel"}:
        raise InvalidTransitionError(f"unknown checkpoint resolve action {action!r}")
    blocker = conn.execute(
        "SELECT * FROM blockers WHERE blocker_id = ?", (blocker_id,)
    ).fetchone()
    if blocker is None:
        raise NotFoundError(f"blocker {blocker_id!r} not found")
    if blocker["state"] != "open":
        raise InvalidTransitionError("blocker is not open")
    if blocker["severity"] != "human_checkpoint":
        raise InvalidTransitionError(
            "checkpoint resolve only applies to human_checkpoint blockers"
        )
    run_id = str(blocker["run_id"])
    blocker_job_id = (
        str(blocker["job_id"]) if blocker["job_id"] is not None else None
    )
    artifact_id: str | None = None
    if decision_id is not None:
        artifact_row = conn.execute(
            """
            SELECT artifact_id, run_id, job_id, session_id, logical_name
            FROM artifacts
            WHERE artifact_kind = 'decision'
              AND run_id = ?
              AND logical_name = ?
            LIMIT 1
            """,
            (run_id, decision_id),
        ).fetchone()
        if artifact_row is None:
            raise NotFoundError(
                f"decision artifact for decision_id={decision_id!r} not found in run"
            )
        if artifact_row["job_id"] is not None or artifact_row["session_id"] is not None:
            raise InvalidTransitionError(
                "decision artifact must be run-level (no job or session binding)"
            )
        artifact_id = str(artifact_row["artifact_id"])

    with transaction(conn):
        now = utc_now()
        downstream_payload: list[JsonObject] = []
        run_state: str | None = None
        if action == "continue":
            conn.execute(
                "UPDATE blockers SET state = 'resolved', resolved_at = ? WHERE blocker_id = ?",
                (now, blocker_id),
            )
            if blocker_job_id is not None:
                job = row_by_id(conn, "jobs", "job_id", blocker_job_id)
                if job["state"] != "waiting_human":
                    raise InvalidTransitionError(
                        f"checkpoint job is not in waiting_human (state={job['state']!r})"
                    )
                message_id = job["current_message_id"]
                if message_id is not None:
                    conn.execute(
                        """
                        UPDATE queue_messages
                        SET state = 'pending', current_lease_id = NULL, updated_at = ?
                        WHERE message_id = ?
                        """,
                        (now, message_id),
                    )
                    conn.execute(
                        """
                        UPDATE jobs
                        SET state = 'queued', current_lease_id = NULL, ready_at = ?
                        WHERE job_id = ?
                        """,
                        (now, blocker_job_id),
                    )
                else:
                    conn.execute(
                        "UPDATE jobs SET state = 'blocked', current_lease_id = NULL WHERE job_id = ?",
                        (blocker_job_id,),
                    )
                    enqueue_job(conn, job_id=blocker_job_id)
                downstream_payload = downstream_jobs(conn, job_id=blocker_job_id)
            event_payload: JsonObject = {
                "blocker_id": blocker_id,
                "action": "continue",
            }
            if decision_id is not None:
                event_payload["decision_id"] = decision_id
            if artifact_id is not None:
                event_payload["decision_artifact_id"] = artifact_id
            insert_event(
                conn,
                run_id=run_id,
                event_type="checkpoint.resolved",
                job_id=blocker_job_id,
                artifact_id=artifact_id,
                payload=event_payload,
            )
            next_actions_list = ["claim_available_work", "monitor_run_progress"]
        else:
            conn.execute(
                "UPDATE blockers SET state = 'resolved', resolved_at = ? WHERE blocker_id = ?",
                (now, blocker_id),
            )
            if blocker_job_id is not None:
                job = row_by_id(conn, "jobs", "job_id", blocker_job_id)
                if job["state"] != "waiting_human":
                    raise InvalidTransitionError(
                        f"checkpoint job is not in waiting_human (state={job['state']!r})"
                    )
                message_id = job["current_message_id"]
                conn.execute(
                    """
                    UPDATE jobs
                    SET state = 'canceled', current_lease_id = NULL,
                        current_message_id = NULL, completed_at = ?
                    WHERE job_id = ?
                    """,
                    (now, blocker_job_id),
                )
                if message_id is not None:
                    conn.execute(
                        """
                        UPDATE queue_messages
                        SET state = 'canceled', current_lease_id = NULL, updated_at = ?
                        WHERE message_id = ?
                        """,
                        (now, message_id),
                    )
                downstream_payload = downstream_jobs(conn, job_id=blocker_job_id)
            event_payload = {
                "blocker_id": blocker_id,
                "action": "cancel",
            }
            if decision_id is not None:
                event_payload["decision_id"] = decision_id
            if artifact_id is not None:
                event_payload["decision_artifact_id"] = artifact_id
            insert_event(
                conn,
                run_id=run_id,
                event_type="checkpoint.canceled",
                job_id=blocker_job_id,
                artifact_id=artifact_id,
                payload=event_payload,
            )
            maybe_complete_run(conn, run_id=run_id)
            next_actions_list = ["inspect_run_state", "export_run_evidence"]
        run_row = row_by_id(conn, "runs", "run_id", run_id)
        run_state = str(run_row["state"])
        return {
            "status": "resolved",
            "blocker_id": blocker_id,
            "job_id": blocker_job_id,
            "action": action,
            "decision_id": decision_id,
            "decision_artifact_id": artifact_id,
            "run_state": run_state,
            "downstream_jobs": downstream_payload,
            "next_actions": next_actions_list,
        }
