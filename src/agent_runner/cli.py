"""Command line interface for the agent_runner MVP."""

from __future__ import annotations

import argparse
import json
import sqlite3
import sys
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Sequence

from agent_runner.artifacts import publish_artifact
from agent_runner.db import (
    AgentRunnerError,
    JsonObject,
    active_lease_for,
    claim_next,
    complete_job,
    connect,
    db_path,
    ensure_initialized,
    expire_leases,
    init_repo,
    insert_event,
    json_dumps,
    json_loads,
    maybe_complete_run,
    maybe_enqueue_downstream,
    new_id,
    row_by_id,
    transaction,
    utc_now,
    verify_required_artifacts,
)
from agent_runner.errors import InvalidTransitionError, LeaseError, NotFoundError, WorkflowError
from agent_runner.workflow import create_run, load_workflow


def main(argv: Sequence[str] | None = None) -> int:
    """Run the CLI."""
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        result = dispatch(args)
    except AgentRunnerError as exc:
        if getattr(args, "json", False):
            print(json_dumps({"ok": False, "error": {"message": str(exc), "code": exc.exit_code}}))
        else:
            print(str(exc), file=sys.stderr)
        return exc.exit_code
    except sqlite3.Error as exc:
        if getattr(args, "json", False):
            print(json_dumps({"ok": False, "error": {"message": str(exc), "code": 1}}))
        else:
            print(str(exc), file=sys.stderr)
        return 1
    if result is not None:
        if getattr(args, "json", False) or isinstance(result, dict):
            print(json_dumps({"ok": True, "data": result}))
        else:
            print(result)
    return 0


def build_parser() -> argparse.ArgumentParser:
    """Build the top-level argument parser."""
    parser = argparse.ArgumentParser(prog="agent_runner")
    parser.add_argument("--repo", default=".", help="repository root")
    sub = parser.add_subparsers(dest="command", required=True)

    init = sub.add_parser("init")
    init.add_argument("--json", action="store_true")

    workflow = sub.add_parser("workflow")
    workflow_sub = workflow.add_subparsers(dest="workflow_command", required=True)
    validate = workflow_sub.add_parser("validate")
    validate.add_argument("path")
    validate.add_argument("--json", action="store_true")

    run = sub.add_parser("run")
    run_sub = run.add_subparsers(dest="run_command", required=True)
    prepare = run_sub.add_parser("prepare")
    prepare.add_argument("--workflow", required=True)
    prepare.add_argument("--json", action="store_true")
    start = run_sub.add_parser("start")
    start.add_argument("--run-id", required=True)
    start.add_argument("--json", action="store_true")

    branch = sub.add_parser("branch")
    branch_sub = branch.add_subparsers(dest="branch_command", required=True)
    confirm = branch_sub.add_parser("confirm")
    confirm.add_argument("--run-id", required=True)
    confirm.add_argument("--branch", required=True)
    confirm.add_argument("--create", action="store_true")
    confirm.add_argument("--use-current", action="store_true")
    confirm.add_argument("--json", action="store_true")

    register = sub.add_parser("register-session")
    register.add_argument("--run-id", required=True)
    register.add_argument("--role", required=True)
    register.add_argument("--lane", required=True)
    register.add_argument("--capability", action="append", default=[])
    register.add_argument("--fresh", action="store_true")
    register.add_argument("--parent-session-id")
    register.add_argument("--json", action="store_true")

    claim = sub.add_parser("claim-next")
    claim.add_argument("--session-id", required=True)
    claim.add_argument("--lease-seconds", type=int, default=1800)
    claim.add_argument("--json", action="store_true")

    ack = sub.add_parser("ack")
    add_work_identity(ack)

    heartbeat = sub.add_parser("heartbeat")
    heartbeat.add_argument("--session-id", required=True)
    heartbeat.add_argument("--lease-id", required=True)
    heartbeat.add_argument("--extend-seconds", type=int, default=1800)
    heartbeat.add_argument("--json", action="store_true")

    release = sub.add_parser("release")
    add_work_identity(release)
    release.add_argument("--reason", required=True)
    release.add_argument("--requeue", action="store_true")

    send = sub.add_parser("send")
    send.add_argument("--session-id", required=True)
    send.add_argument("--kind", required=True)
    send.add_argument("--body-json", default="{}")
    send.add_argument("--json", action="store_true")

    block = sub.add_parser("block")
    block.add_argument("--session-id", required=True)
    block.add_argument("--job-id", required=True)
    block.add_argument("--lease-id", required=True)
    block.add_argument("--kind", required=True)
    block.add_argument("--severity", choices=["blocked", "human_checkpoint"], required=True)
    block.add_argument("--description", required=True)
    block.add_argument("--json", action="store_true")

    publish = sub.add_parser("publish-artifact")
    publish.add_argument("--session-id", required=True)
    publish.add_argument("--job-id", required=True)
    publish.add_argument("--lease-id", required=True)
    publish.add_argument("--kind", required=True)
    publish.add_argument("--logical-name", required=True)
    publish.add_argument("--path", required=True)
    publish.add_argument("--json", action="store_true")

    complete = sub.add_parser("complete")
    complete.add_argument("--session-id", required=True)
    complete.add_argument("--job-id", required=True)
    complete.add_argument("--lease-id", required=True)
    complete.add_argument("--summary")
    complete.add_argument("--json", action="store_true")

    verdict = sub.add_parser("verdict")
    verdict.add_argument("--session-id", required=True)
    verdict.add_argument("--job-id", required=True)
    verdict.add_argument("--lease-id", required=True)
    verdict.add_argument(
        "--verdict",
        choices=["accept", "accept_with_findings", "needs_revision", "reject"],
        required=True,
    )
    verdict.add_argument("--findings-artifact-id")
    verdict.add_argument("--rationale")
    verdict.add_argument("--json", action="store_true")

    status = sub.add_parser("status")
    status.add_argument("--run-id")
    status.add_argument("--json", action="store_true")

    why = sub.add_parser("why")
    why.add_argument("id")
    why.add_argument("--json", action="store_true")

    doctor = sub.add_parser("doctor")
    doctor.add_argument("--run-id")
    doctor.add_argument("--json", action="store_true")

    return parser


def add_work_identity(parser: argparse.ArgumentParser) -> None:
    """Add standard work ownership arguments."""
    parser.add_argument("--session-id", required=True)
    parser.add_argument("--message-id", required=True)
    parser.add_argument("--lease-id", required=True)
    parser.add_argument("--json", action="store_true")


def dispatch(args: argparse.Namespace) -> object:
    """Dispatch a parsed command."""
    repo = Path(args.repo).resolve()
    if args.command == "init":
        init_repo(repo)
        return {"state_dir": str(repo / ".agent_runner"), "db": str(db_path(repo))}
    if args.command == "workflow" and args.workflow_command == "validate":
        workflow = load_workflow(Path(args.path))
        return {"workflow_id": workflow["workflow_id"], "valid": True}
    ensure_initialized(repo)
    with connect(repo) as conn:
        if args.command == "run" and args.run_command == "prepare":
            with transaction(conn):
                return create_run(conn, repo=repo, workflow_path=Path(args.workflow))
        if args.command == "branch" and args.branch_command == "confirm":
            return branch_confirm(conn, run_id=args.run_id, branch=args.branch)
        if args.command == "run" and args.run_command == "start":
            return run_start(conn, run_id=args.run_id)
        if args.command == "register-session":
            return register_session(
                conn,
                run_id=args.run_id,
                role=args.role,
                lane=args.lane,
                capabilities=args.capability,
                fresh=args.fresh,
                parent_session_id=args.parent_session_id,
            )
        if args.command == "claim-next":
            return claim_next(
                conn,
                repo=repo,
                session_id=args.session_id,
                lease_seconds=args.lease_seconds,
            )
        if args.command == "ack":
            return ack_work(conn, session_id=args.session_id, message_id=args.message_id, lease_id=args.lease_id)
        if args.command == "heartbeat":
            return heartbeat(conn, session_id=args.session_id, lease_id=args.lease_id, extend_seconds=args.extend_seconds)
        if args.command == "release":
            return release_work(
                conn,
                session_id=args.session_id,
                message_id=args.message_id,
                lease_id=args.lease_id,
                reason=args.reason,
                requeue=args.requeue,
            )
        if args.command == "send":
            return send_message(conn, session_id=args.session_id, kind=args.kind, body_json=args.body_json)
        if args.command == "block":
            return block_work(
                conn,
                session_id=args.session_id,
                job_id=args.job_id,
                lease_id=args.lease_id,
                kind=args.kind,
                severity=args.severity,
                description=args.description,
            )
        if args.command == "publish-artifact":
            return publish_artifact(
                conn,
                repo=repo,
                session_id=args.session_id,
                job_id=args.job_id,
                lease_id=args.lease_id,
                kind=args.kind,
                logical_name=args.logical_name,
                path_text=args.path,
            )
        if args.command == "complete":
            return complete_job(
                conn,
                session_id=args.session_id,
                job_id=args.job_id,
                lease_id=args.lease_id,
                summary=args.summary,
            )
        if args.command == "verdict":
            return verdict_work(
                conn,
                session_id=args.session_id,
                job_id=args.job_id,
                lease_id=args.lease_id,
                verdict=args.verdict,
                findings_artifact_id=args.findings_artifact_id,
                rationale=args.rationale,
            )
        if args.command == "status":
            return status(conn, run_id=args.run_id)
        if args.command == "why":
            return why(conn, target_id=args.id)
        if args.command == "doctor":
            return doctor(conn, run_id=args.run_id)
    raise AgentRunnerError("unknown command", exit_code=2)


def branch_confirm(conn: sqlite3.Connection, *, run_id: str, branch: str) -> JsonObject:
    """Record branch confirmation."""
    with transaction(conn):
        run = row_by_id(conn, "runs", "run_id", run_id)
        if run["state"] not in ("needs_branch_confirmation", "ready"):
            raise InvalidTransitionError("run is not waiting for branch confirmation")
        now = utc_now()
        conn.execute(
            """
            UPDATE runs
            SET branch_name = ?, branch_confirmed_at = ?, branch_confirmed_by = 'human',
                state = 'ready'
            WHERE run_id = ?
            """,
            (branch, now, run_id),
        )
        insert_event(conn, run_id=run_id, event_type="run.branch_confirmed", payload={"branch": branch})
        return {"run_id": run_id, "state": "ready", "branch": branch}


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
            from agent_runner.db import enqueue_job

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
) -> JsonObject:
    """Register an agent session."""
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
              registered_at, last_heartbeat_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)
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
            ),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="session.registered",
            actor_session_id=session_id,
            payload={"role": role, "lane": lane, "slug": slug},
        )
        return {"session_id": session_id, "slug": slug}


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
        from agent_runner.db import is_repo_write

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
    """Record a review verdict and complete the review job."""
    with transaction(conn):
        job = row_by_id(conn, "jobs", "job_id", job_id)
        if job["job_type"] != "review":
            raise InvalidTransitionError("verdict is valid only for review jobs")
        active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=job_id)
        if job["state"] != "running":
            raise InvalidTransitionError("review job must be running before verdict")
        verify_required_artifacts(conn, job_id=job_id)
        verdict_id = new_id("verdict")
        now = utc_now()
        conn.execute(
            """
            INSERT INTO verdicts (
              verdict_id, run_id, job_id, session_id, verdict, rationale,
              findings_artifact_id, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                verdict_id,
                job["run_id"],
                job_id,
                session_id,
                verdict,
                rationale,
                findings_artifact_id,
                now,
            ),
        )
        message_id = job["current_message_id"]
        conn.execute(
            "UPDATE jobs SET state = 'completed', completed_at = ?, current_lease_id = NULL WHERE job_id = ?",
            (now, job_id),
        )
        if message_id is not None:
            conn.execute(
                """
                UPDATE queue_messages
                SET state = 'completed', completed_at = ?, updated_at = ?,
                    current_lease_id = NULL
                WHERE message_id = ?
                """,
                (now, now, message_id),
            )
        conn.execute(
            """
            UPDATE leases
            SET state = 'released', released_at = ?, release_reason = 'verdict'
            WHERE lease_id = ?
            """,
            (now, lease_id),
        )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="verdict.recorded",
            actor_session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            payload={"verdict": verdict},
        )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="job.completed",
            actor_session_id=session_id,
            job_id=job_id,
            message_id=message_id,
            lease_id=lease_id,
            payload={"summary": verdict},
        )
        maybe_enqueue_downstream(conn, completed_job_id=job_id)
        maybe_complete_run(conn, run_id=str(job["run_id"]))
        return {"status": "completed", "job_id": job_id, "verdict": verdict}


def status(conn: sqlite3.Connection, *, run_id: str | None) -> JsonObject:
    """Return current state summary."""
    if run_id is not None:
        expire_leases(conn, run_id=run_id)
    runs = conn.execute(
        "SELECT run_id, state, branch_name FROM runs WHERE (? IS NULL OR run_id = ?) ORDER BY created_at",
        (run_id, run_id),
    ).fetchall()
    jobs = conn.execute(
        """
        SELECT state, COUNT(*) AS count FROM jobs
        WHERE (? IS NULL OR run_id = ?)
        GROUP BY state ORDER BY state
        """,
        (run_id, run_id),
    ).fetchall()
    return {
        "runs": [dict(row) for row in runs],
        "jobs": {str(row["state"]): int(row["count"]) for row in jobs},
    }


def why(conn: sqlite3.Connection, *, target_id: str) -> JsonObject:
    """Explain a job or message by returning related rows and events."""
    job = conn.execute("SELECT * FROM jobs WHERE job_id = ? OR workflow_job_id = ?", (target_id, target_id)).fetchone()
    message = conn.execute("SELECT * FROM queue_messages WHERE message_id = ?", (target_id,)).fetchone()
    if job is None and message is None:
        raise NotFoundError("target id is not a known job or message")
    job_id = job["job_id"] if job is not None else message["job_id"]
    events = conn.execute(
        "SELECT event_id, event_type, payload_json FROM events WHERE job_id = ? ORDER BY event_id",
        (job_id,),
    ).fetchall()
    return {
        "job": dict(job) if job is not None else None,
        "message": dict(message) if message is not None else None,
        "events": [dict(row) for row in events],
    }


def doctor(conn: sqlite3.Connection, *, run_id: str | None) -> JsonObject:
    """Return consistency checks for the state database."""
    problems: list[str] = []
    orphan_jobs = conn.execute(
        """
        SELECT j.job_id
        FROM jobs j
        LEFT JOIN leases l ON l.lease_id = j.current_lease_id AND l.state = 'active'
        WHERE j.state IN ('claimed','running') AND l.lease_id IS NULL
          AND (? IS NULL OR j.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in orphan_jobs:
        problems.append(f"active job without active lease: {row['job_id']}")
    schema_version = conn.execute(
        "SELECT value FROM schema_meta WHERE key = 'schema_version'"
    ).fetchone()
    return {
        "ok": len(problems) == 0,
        "schema_version": schema_version["value"] if schema_version is not None else None,
        "problems": problems,
    }


if __name__ == "__main__":
    raise SystemExit(main())
