"""Command line interface for the agent_runner MVP."""

from __future__ import annotations

import argparse
import json
import sqlite3
import subprocess
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
    latest_verdict,
    new_id,
    record_review_verdict,
    repo_relative_path,
    row_by_id,
    sha256_bytes,
    transaction,
    utc_now,
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

    submit_review = sub.add_parser("submit-review")
    submit_review.add_argument("--session-id", required=True)
    submit_review.add_argument("--job-id", required=True)
    submit_review.add_argument("--lease-id", required=True)
    submit_review.add_argument("--path", required=True)
    submit_review.add_argument(
        "--verdict",
        choices=["accept", "accept_with_findings", "needs_revision", "reject"],
        required=True,
    )
    submit_review.add_argument("--logical-name", default="review")
    submit_review.add_argument("--kind", default="finding")
    submit_review.add_argument("--rationale")
    submit_review.add_argument("--json", action="store_true")

    evidence = sub.add_parser("evidence")
    evidence_sub = evidence.add_subparsers(dest="evidence_command", required=True)
    evidence_export = evidence_sub.add_parser("export")
    evidence_export.add_argument("--run-id", required=True)
    evidence_export.add_argument("--path", required=True)
    evidence_export.add_argument("--json", action="store_true")

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
            return branch_confirm(conn, repo=repo, run_id=args.run_id, branch=args.branch)
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
        if args.command == "submit-review":
            return submit_review(
                conn,
                repo=repo,
                session_id=args.session_id,
                job_id=args.job_id,
                lease_id=args.lease_id,
                path_text=args.path,
                verdict=args.verdict,
                logical_name=args.logical_name,
                kind=args.kind,
                rationale=args.rationale,
            )
        if args.command == "evidence" and args.evidence_command == "export":
            return evidence_export(conn, repo=repo, run_id=args.run_id, path_text=args.path)
        if args.command == "status":
            return status(conn, run_id=args.run_id)
        if args.command == "why":
            return why(conn, target_id=args.id)
        if args.command == "doctor":
            return doctor(conn, run_id=args.run_id)
    raise AgentRunnerError("unknown command", exit_code=2)


def branch_confirm(conn: sqlite3.Connection, *, repo: Path, run_id: str, branch: str) -> JsonObject:
    """Record branch confirmation."""
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
            (branch, now, run_id),
        )
        insert_event(conn, run_id=run_id, event_type="run.branch_confirmed", payload={"branch": branch})
        warning = None
        if current_branch is not None and current_branch != branch:
            warning = "current git branch differs from recorded branch confirmation"
        return {
            "run_id": run_id,
            "state": "ready",
            "branch": branch,
            "requested_branch": branch,
            "current_git_branch": current_branch,
            "records_only": True,
            "warning": warning,
        }


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
    open_blockers = blocker_summaries(conn, run_id=run_id, severity=None)
    human_checkpoints = blocker_summaries(conn, run_id=run_id, severity="human_checkpoint")
    non_accepting = latest_non_accepting_verdicts(conn, run_id=run_id)
    claimable = claimable_jobs_by_role_lane(conn, run_id=run_id)
    blocked_downstream = blocked_downstream_jobs(conn, run_id=run_id)
    return {
        "runs": [dict(row) for row in runs],
        "jobs": {str(row["state"]): int(row["count"]) for row in jobs},
        "open_blockers": open_blockers,
        "human_checkpoints": human_checkpoints,
        "latest_non_accepting_review_verdicts": non_accepting,
        "claimable_jobs": claimable,
        "blocked_downstream_jobs": blocked_downstream,
        "next_actions": next_actions(
            open_blockers=open_blockers,
            human_checkpoints=human_checkpoints,
            non_accepting_verdicts=non_accepting,
            claimable_jobs=claimable,
        ),
    }


def why(conn: sqlite3.Connection, *, target_id: str) -> JsonObject:
    """Explain a state id by returning related rows and events."""
    run = conn.execute("SELECT * FROM runs WHERE run_id = ?", (target_id,)).fetchone()
    if run is not None:
        run_id = str(run["run_id"])
        return {
            "target_type": "run",
            "run": dict(run),
            "jobs": [dict(row) for row in jobs_for_run(conn, run_id=run_id)],
            "open_blockers": blocker_summaries(conn, run_id=run_id, severity=None),
            "events": events_for(conn, run_id=run_id),
            "next_actions": status(conn, run_id=run_id)["next_actions"],
        }

    job = conn.execute("SELECT * FROM jobs WHERE job_id = ? OR workflow_job_id = ?", (target_id, target_id)).fetchone()
    message = conn.execute("SELECT * FROM queue_messages WHERE message_id = ?", (target_id,)).fetchone()
    if job is not None or message is not None:
        job_id = str(job["job_id"] if job is not None else message["job_id"])
        return {
            "target_type": "job" if job is not None else "message",
            "job": dict(job) if job is not None else dict(row_by_id(conn, "jobs", "job_id", job_id)),
            "message": dict(message) if message is not None else None,
            "verdict": latest_verdict_row(conn, job_id=job_id),
            "blockers": blockers_for_job(conn, job_id=job_id),
            "downstream_jobs": downstream_jobs(conn, job_id=job_id),
            "events": events_for(conn, job_id=job_id),
        }

    blocker = conn.execute("SELECT * FROM blockers WHERE blocker_id = ?", (target_id,)).fetchone()
    if blocker is not None:
        job_id = str(blocker["job_id"]) if blocker["job_id"] is not None else None
        run_id = str(blocker["run_id"])
        return {
            "target_type": "blocker",
            "blocker": dict(blocker),
            "run": dict(row_by_id(conn, "runs", "run_id", run_id)),
            "job": dict(row_by_id(conn, "jobs", "job_id", job_id)) if job_id is not None else None,
            "session": dict(row_by_id(conn, "sessions", "session_id", str(blocker["session_id"])))
            if blocker["session_id"] is not None
            else None,
            "related_verdict": latest_verdict_row(conn, job_id=job_id) if job_id is not None else None,
            "blocked_downstream_jobs": downstream_jobs(conn, job_id=job_id) if job_id is not None else [],
            "next_actions": ["inspect_blocker", "resolve_human_checkpoint", "export_run_evidence"],
            "events": events_for(conn, job_id=job_id) if job_id is not None else events_for(conn, run_id=run_id),
        }

    artifact = conn.execute("SELECT * FROM artifacts WHERE artifact_id = ?", (target_id,)).fetchone()
    if artifact is not None:
        job_id = str(artifact["job_id"]) if artifact["job_id"] is not None else None
        return {
            "target_type": "artifact",
            "artifact": dict(artifact),
            "job": dict(row_by_id(conn, "jobs", "job_id", job_id)) if job_id is not None else None,
            "verdicts": verdicts_for_artifact(conn, artifact_id=target_id),
            "events": events_for(conn, artifact_id=target_id),
        }

    verdict = conn.execute("SELECT * FROM verdicts WHERE verdict_id = ?", (target_id,)).fetchone()
    if verdict is not None:
        artifact_id = verdict["findings_artifact_id"]
        return {
            "target_type": "verdict",
            "verdict": dict(verdict),
            "job": dict(row_by_id(conn, "jobs", "job_id", str(verdict["job_id"]))),
            "artifact": dict(row_by_id(conn, "artifacts", "artifact_id", str(artifact_id)))
            if artifact_id is not None
            else None,
            "blockers": blockers_for_job(conn, job_id=str(verdict["job_id"])),
            "events": events_for(conn, job_id=str(verdict["job_id"])),
        }

    session = conn.execute("SELECT * FROM sessions WHERE session_id = ? OR slug = ?", (target_id, target_id)).fetchone()
    if session is not None:
        return {
            "target_type": "session",
            "session": dict(session),
            "jobs": jobs_for_session(conn, session_id=str(session["session_id"])),
            "events": events_for(conn, session_id=str(session["session_id"])),
        }

    raise NotFoundError("target id is not a known run, job, message, blocker, artifact, verdict, or session")


def evidence_export(conn: sqlite3.Connection, *, repo: Path, run_id: str, path_text: str) -> JsonObject:
    """Write a redacted Markdown snapshot of runner state."""
    run = row_by_id(conn, "runs", "run_id", run_id)
    target = repo_relative_path(repo, path_text)
    target.parent.mkdir(parents=True, exist_ok=True)
    status_payload = status(conn, run_id=run_id)
    doctor_payload = doctor(conn, run_id=run_id)
    snapshot = evidence_snapshot(conn, run_id=run_id)
    body = render_evidence_markdown(
        run=dict(run),
        status_payload=status_payload,
        doctor_payload=doctor_payload,
        snapshot=snapshot,
    )
    target.write_text(body, encoding="utf-8")
    digest = sha256_bytes(body.encode("utf-8"))
    insert_event(
        conn,
        run_id=run_id,
        event_type="evidence.exported",
        payload={"path": path_text, "sha256": digest},
    )
    return {"status": "exported", "run_id": run_id, "path": path_text, "sha256": digest}


def blocker_summaries(conn: sqlite3.Connection, *, run_id: str | None, severity: str | None) -> list[JsonObject]:
    """Return open blocker summaries."""
    rows = conn.execute(
        """
        SELECT b.blocker_id, b.run_id, b.job_id, b.session_id, b.severity,
               b.blocker_kind, b.description, b.state, j.workflow_job_id, j.state AS job_state
        FROM blockers b
        LEFT JOIN jobs j ON j.job_id = b.job_id
        WHERE b.state = 'open'
          AND (? IS NULL OR b.run_id = ?)
          AND (? IS NULL OR b.severity = ?)
        ORDER BY b.created_at
        """,
        (run_id, run_id, severity, severity),
    ).fetchall()
    return [dict(row) for row in rows]


def latest_non_accepting_verdicts(conn: sqlite3.Connection, *, run_id: str | None) -> list[JsonObject]:
    """Return latest non-accepting verdicts on waiting or failed review jobs."""
    rows = conn.execute(
        """
        SELECT v.verdict_id, v.run_id, v.job_id, j.workflow_job_id, j.state AS job_state,
               v.session_id, v.verdict, v.findings_artifact_id, v.rationale
        FROM verdicts v
        JOIN jobs j ON j.job_id = v.job_id
        WHERE j.job_type = 'review'
          AND j.state IN ('waiting_human','failed')
          AND v.verdict NOT IN ('accept','accept_with_findings')
          AND (? IS NULL OR v.run_id = ?)
          AND v.created_at = (
            SELECT MAX(v2.created_at) FROM verdicts v2 WHERE v2.job_id = v.job_id
          )
        ORDER BY v.created_at
        """,
        (run_id, run_id),
    ).fetchall()
    return [dict(row) for row in rows]


def claimable_jobs_by_role_lane(conn: sqlite3.Connection, *, run_id: str | None) -> list[JsonObject]:
    """Return pending work grouped by target role and lane."""
    rows = conn.execute(
        """
        SELECT qm.target_role_id AS role_id, qm.target_lane_id AS lane_id,
               COUNT(*) AS count, GROUP_CONCAT(j.workflow_job_id) AS workflow_job_ids
        FROM queue_messages qm
        JOIN jobs j ON j.job_id = qm.job_id
        WHERE qm.kind = 'work' AND qm.state = 'pending'
          AND (? IS NULL OR qm.run_id = ?)
        GROUP BY qm.target_role_id, qm.target_lane_id
        ORDER BY qm.target_role_id, qm.target_lane_id
        """,
        (run_id, run_id),
    ).fetchall()
    result: list[JsonObject] = []
    for row in rows:
        workflow_ids = str(row["workflow_job_ids"] or "").split(",") if row["workflow_job_ids"] else []
        result.append(
            {
                "role_id": row["role_id"],
                "lane_id": row["lane_id"],
                "count": int(row["count"]),
                "workflow_job_ids": workflow_ids,
            }
        )
    return result


def blocked_downstream_jobs(conn: sqlite3.Connection, *, run_id: str | None) -> list[JsonObject]:
    """Return blocked jobs with unsatisfied upstream dependency context."""
    jobs = conn.execute(
        """
        SELECT * FROM jobs
        WHERE state = 'blocked' AND (? IS NULL OR run_id = ?)
        ORDER BY workflow_job_id
        """,
        (run_id, run_id),
    ).fetchall()
    result: list[JsonObject] = []
    for job in jobs:
        dependencies = dependency_context(conn, job_id=str(job["job_id"]))
        if not dependencies:
            continue
        result.append(
            {
                "job_id": job["job_id"],
                "workflow_job_id": job["workflow_job_id"],
                "state": job["state"],
                "role_id": job["role_id"],
                "lane": json_loads(str(job["lane_selector_json"])).get("lane_id"),
                "blocked_by": dependencies,
            }
        )
    return result


def dependency_context(conn: sqlite3.Connection, *, job_id: str) -> list[JsonObject]:
    """Return dependency rows with upstream state and verdict context."""
    dependencies = conn.execute(
        """
        SELECT dep.depends_on_job_id, dep.gate_json, up.workflow_job_id, up.state, up.job_type
        FROM job_dependencies dep
        JOIN jobs up ON up.job_id = dep.depends_on_job_id
        WHERE dep.job_id = ?
        ORDER BY up.workflow_job_id
        """,
        (job_id,),
    ).fetchall()
    result: list[JsonObject] = []
    for dependency in dependencies:
        gate = json_loads(str(dependency["gate_json"]))
        verdict = latest_verdict(conn, job_id=str(dependency["depends_on_job_id"]))
        satisfied = dependency["state"] == "completed"
        required = gate.get("requires_verdict")
        if isinstance(required, list):
            satisfied = satisfied and verdict in set(required)
        if satisfied:
            continue
        result.append(
            {
                "depends_on_job_id": dependency["depends_on_job_id"],
                "workflow_job_id": dependency["workflow_job_id"],
                "state": dependency["state"],
                "required_verdicts": required,
                "latest_verdict": verdict,
            }
        )
    return result


def next_actions(
    *,
    open_blockers: list[JsonObject],
    human_checkpoints: list[JsonObject],
    non_accepting_verdicts: list[JsonObject],
    claimable_jobs: list[JsonObject],
) -> list[str]:
    """Return deterministic coordinator next-action names."""
    actions: list[str] = []
    if claimable_jobs:
        actions.append("claim_available_work")
    if open_blockers:
        actions.extend(["inspect_blocker", "export_run_evidence"])
    if human_checkpoints:
        actions.append("resolve_human_checkpoint")
    if non_accepting_verdicts:
        actions.append("revise_workflow_cycle")
    return list(dict.fromkeys(actions))


def downstream_jobs(conn: sqlite3.Connection, *, job_id: str) -> list[JsonObject]:
    """Return immediate downstream jobs and their dependency context."""
    rows = conn.execute(
        """
        SELECT j.* FROM job_dependencies dep
        JOIN jobs j ON j.job_id = dep.job_id
        WHERE dep.depends_on_job_id = ?
        ORDER BY j.workflow_job_id
        """,
        (job_id,),
    ).fetchall()
    return [
        {
            "job_id": row["job_id"],
            "workflow_job_id": row["workflow_job_id"],
            "state": row["state"],
            "blocked_by": dependency_context(conn, job_id=str(row["job_id"])),
        }
        for row in rows
    ]


def latest_verdict_row(conn: sqlite3.Connection, *, job_id: str | None) -> JsonObject | None:
    """Return the latest verdict row for a job."""
    if job_id is None:
        return None
    row = conn.execute(
        "SELECT * FROM verdicts WHERE job_id = ? ORDER BY created_at DESC, verdict_id DESC LIMIT 1",
        (job_id,),
    ).fetchone()
    return dict(row) if row is not None else None


def blockers_for_job(conn: sqlite3.Connection, *, job_id: str) -> list[JsonObject]:
    """Return blockers for a job."""
    rows = conn.execute("SELECT * FROM blockers WHERE job_id = ? ORDER BY created_at", (job_id,)).fetchall()
    return [dict(row) for row in rows]


def verdicts_for_artifact(conn: sqlite3.Connection, *, artifact_id: str) -> list[JsonObject]:
    """Return verdicts that cite an artifact."""
    rows = conn.execute(
        "SELECT * FROM verdicts WHERE findings_artifact_id = ? ORDER BY created_at",
        (artifact_id,),
    ).fetchall()
    return [dict(row) for row in rows]


def jobs_for_run(conn: sqlite3.Connection, *, run_id: str) -> list[sqlite3.Row]:
    """Return jobs for a run."""
    return conn.execute(
        "SELECT * FROM jobs WHERE run_id = ? ORDER BY workflow_job_id, attempt",
        (run_id,),
    ).fetchall()


def jobs_for_session(conn: sqlite3.Connection, *, session_id: str) -> list[JsonObject]:
    """Return jobs touched by a session."""
    rows = conn.execute(
        """
        SELECT DISTINCT j.*
        FROM jobs j
        LEFT JOIN leases l ON l.resource_id = j.job_id
        LEFT JOIN verdicts v ON v.job_id = j.job_id
        WHERE l.owner_session_id = ? OR v.session_id = ?
        ORDER BY j.workflow_job_id
        """,
        (session_id, session_id),
    ).fetchall()
    return [dict(row) for row in rows]


def events_for(
    conn: sqlite3.Connection,
    *,
    run_id: str | None = None,
    job_id: str | None = None,
    session_id: str | None = None,
    artifact_id: str | None = None,
) -> list[JsonObject]:
    """Return matching append-only events."""
    clauses: list[str] = []
    values: list[str] = []
    if run_id is not None:
        clauses.append("run_id = ?")
        values.append(run_id)
    if job_id is not None:
        clauses.append("job_id = ?")
        values.append(job_id)
    if session_id is not None:
        clauses.append("actor_session_id = ?")
        values.append(session_id)
    if artifact_id is not None:
        clauses.append("artifact_id = ?")
        values.append(artifact_id)
    where = " AND ".join(clauses) if clauses else "1 = 1"
    rows = conn.execute(
        f"SELECT event_id, event_type, payload_json FROM events WHERE {where} ORDER BY event_id",
        values,
    ).fetchall()
    return [dict(row) for row in rows]


def evidence_snapshot(conn: sqlite3.Connection, *, run_id: str) -> JsonObject:
    """Return redacted run state for evidence export."""
    run = row_by_id(conn, "runs", "run_id", run_id)
    snapshot = row_by_id(conn, "workflow_snapshots", "workflow_snapshot_id", str(run["workflow_snapshot_id"]))
    jobs = evidence_job_summaries(conn, run_id=run_id)
    artifacts = conn.execute(
        """
        SELECT artifact_id, job_id, logical_name, artifact_kind, repo_path, content_sha256
        FROM artifacts WHERE run_id = ? ORDER BY repo_path
        """,
        (run_id,),
    ).fetchall()
    verdicts = conn.execute(
        """
        SELECT verdict_id, job_id, session_id, verdict, findings_artifact_id, rationale
        FROM verdicts WHERE run_id = ? ORDER BY created_at
        """,
        (run_id,),
    ).fetchall()
    blockers = conn.execute(
        """
        SELECT blocker_id, job_id, session_id, severity, blocker_kind, description, state
        FROM blockers WHERE run_id = ? ORDER BY created_at
        """,
        (run_id,),
    ).fetchall()
    return {
        "schema_version": "agent-runner.evidence.v1",
        "exported_at": utc_now(),
        "workflow": {
            "workflow_id": snapshot["workflow_id"],
            "workflow_version": snapshot["workflow_version"],
        },
        "run": {
            "run_id": run["run_id"],
            "branch_name": run["branch_name"],
            "state": run["state"],
        },
        "jobs": jobs,
        "artifacts": [dict(row) for row in artifacts],
        "verdicts": [dict(row) for row in verdicts],
        "blockers": [dict(row) for row in blockers],
        "blocked_downstream_jobs": blocked_downstream_jobs(conn, run_id=run_id),
    }


def evidence_job_summaries(conn: sqlite3.Connection, *, run_id: str) -> list[JsonObject]:
    """Return redacted job summaries for evidence export."""
    summaries: list[JsonObject] = []
    for job in jobs_for_run(conn, run_id=run_id):
        lane = json_loads(str(job["lane_selector_json"])).get("lane_id")
        summaries.append(
            {
                "job_id": job["job_id"],
                "workflow_job_id": job["workflow_job_id"],
                "title": job["title"],
                "job_type": job["job_type"],
                "role_id": job["role_id"],
                "lane": lane,
                "state": job["state"],
                "attempt": job["attempt"],
                "max_attempts": job["max_attempts"],
                "fresh_session_required": bool(job["fresh_session_required"]),
                "dependencies": dependency_summary(conn, job_id=str(job["job_id"])),
            }
        )
    return summaries


def dependency_summary(conn: sqlite3.Connection, *, job_id: str) -> list[JsonObject]:
    """Return all upstream dependency states for export."""
    rows = conn.execute(
        """
        SELECT dep.depends_on_job_id, dep.gate_json, up.workflow_job_id, up.state
        FROM job_dependencies dep
        JOIN jobs up ON up.job_id = dep.depends_on_job_id
        WHERE dep.job_id = ?
        ORDER BY up.workflow_job_id
        """,
        (job_id,),
    ).fetchall()
    result: list[JsonObject] = []
    for row in rows:
        gate = json_loads(str(row["gate_json"]))
        result.append(
            {
                "depends_on_job_id": row["depends_on_job_id"],
                "workflow_job_id": row["workflow_job_id"],
                "state": row["state"],
                "required_verdicts": gate.get("requires_verdict"),
                "latest_verdict": latest_verdict(conn, job_id=str(row["depends_on_job_id"])),
            }
        )
    return result


def render_evidence_markdown(
    *,
    run: JsonObject,
    status_payload: JsonObject,
    doctor_payload: JsonObject,
    snapshot: JsonObject,
) -> str:
    """Render a redacted evidence snapshot as Markdown."""
    return "\n".join(
        [
            "# Agent Runner Evidence Export",
            "",
            f"Run ID: `{run['run_id']}`",
            f"Branch: `{run['branch_name']}`",
            f"Run state: `{run['state']}`",
            f"Exported at: `{snapshot['exported_at']}`",
            "",
            "Live SQLite state remains ignored under `.agent_runner/` and is not part of this export.",
            "",
            "## Status Output",
            "",
            "```json",
            json_dumps(status_payload),
            "```",
            "",
            "## Doctor Output",
            "",
            "```json",
            json_dumps(doctor_payload),
            "```",
            "",
            "## Snapshot",
            "",
            "```json",
            json.dumps(snapshot, indent=2, sort_keys=True),
            "```",
            "",
        ]
    )


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
    dependencies = conn.execute(
        """
        SELECT dep.job_id, dep.depends_on_job_id, dep.gate_json
        FROM job_dependencies dep
        JOIN jobs upstream ON upstream.job_id = dep.depends_on_job_id
        WHERE (? IS NULL OR upstream.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in dependencies:
        try:
            gate = json_loads(str(row["gate_json"]))
        except (json.JSONDecodeError, InvalidTransitionError):
            problems.append(f"dependency gate_json is invalid: {row['depends_on_job_id']} -> {row['job_id']}")
            continue
        if gate.get("requires_verdict") is None:
            continue
        upstream = row_by_id(conn, "jobs", "job_id", str(row["depends_on_job_id"]))
        if upstream["state"] == "completed" and latest_verdict(conn, job_id=str(upstream["job_id"])) not in {
            "accept",
            "accept_with_findings",
        }:
            problems.append(
                "completed review dependency lacks accepting verdict: "
                f"{upstream['workflow_job_id']} -> {row['job_id']}"
            )
    jobs = conn.execute(
        "SELECT job_id, expected_artifacts_json FROM jobs WHERE (? IS NULL OR run_id = ?)",
        (run_id, run_id),
    ).fetchall()
    for job in jobs:
        expected = json.loads(str(job["expected_artifacts_json"]))
        if not isinstance(expected, list):
            continue
        for item in expected:
            if not isinstance(item, dict) or item.get("required") is not True:
                continue
            logical_name = item.get("logical_name")
            existing = conn.execute(
                "SELECT artifact_kind, repo_path FROM artifacts WHERE job_id = ? AND logical_name = ?",
                (job["job_id"], logical_name),
            ).fetchone()
            if existing is None:
                continue
            if existing["artifact_kind"] != item.get("kind") or existing["repo_path"] != item.get("path"):
                problems.append(
                    "required artifact mismatch: "
                    f"job_id={job['job_id']}, logical_name={logical_name!r}, "
                    f"expected kind={item.get('kind')!r}, path={item.get('path')!r}"
                )
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
