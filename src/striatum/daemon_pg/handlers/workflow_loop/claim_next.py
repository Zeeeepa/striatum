"""PG-backed ``work.claim_next`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from pathlib import Path
from typing import Any

from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    _jsonb,
    build_packet,
    is_repo_write,
    transaction,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.errors import BranchConfirmationError
from striatum.primitives import json_dumps, sha256_bytes


@register_pg_handler("work.claim_next", "claim_next")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = str(params["session_id"])
    lease_seconds = int(params.get("lease_seconds", 3600))
    with transaction(ctx):
        session = ctx.row_by_id("sessions", "session_id", session_id, for_update=True)
        run = ctx.row_by_id("runs", "run_id", str(session["run_id"]), for_update=True)
        _expire_leases(ctx, run_id=str(run["run_id"]))
        if run["state"] in ("needs_branch_confirmation", "ready"):
            raise BranchConfirmationError("branch confirmation and run start are required before claims")
        if run["state"] != "running":
            return {"status": "no_work"}
        if run.get("paused_at") is not None:
            return {"status": "no_work", "paused": True}
        with ctx.cursor() as cur:
            cur.execute(
                """
                SELECT qm.*
                FROM striatumd.queue_messages qm
                JOIN striatumd.jobs j
                  ON j.repository_id = qm.repository_id AND j.job_id = qm.job_id
                WHERE qm.repository_id = %s
                  AND qm.kind = 'work'
                  AND qm.state = 'pending'
                  AND qm.target_role_id = %s
                  AND (qm.target_lane_id IS NULL OR qm.target_lane_id = %s)
                  AND (
                    j.fresh_session_required = false
                    OR NOT EXISTS (
                      SELECT 1 FROM striatumd.work_packets wp
                      WHERE wp.repository_id = qm.repository_id
                        AND wp.run_id = qm.run_id
                        AND wp.session_id = %s
                    )
                  )
                  AND qm.run_id = %s
                ORDER BY qm.priority DESC, qm.created_at ASC
                LIMIT 1
                FOR UPDATE SKIP LOCKED
                """,
                (
                    ctx.repository_id,
                    session["role_id"],
                    session["lane_id"],
                    session_id,
                    run["run_id"],
                ),
            )
            chosen = cur.fetchone()
        if chosen is None:
            return {"status": "no_work"}
        job = ctx.row_by_id("jobs", "job_id", str(chosen["job_id"]), for_update=True)
        now = ctx.now()
        lease_id = ctx.new_id("lease")
        packet_id = ctx.new_id("wp")
        expires_at = ctx.expires_after(lease_seconds)
        with ctx.cursor() as cur:
            cur.execute(
                """
                INSERT INTO striatumd.leases (
                  repository_id, lease_id, run_id, resource_type, resource_id,
                  owner_session_id, state, acquired_at, expires_at, last_heartbeat_at
                )
                VALUES (%s, %s, %s, 'job', %s, %s, 'active', %s, %s, %s)
                """,
                (ctx.repository_id, lease_id, run["run_id"], job["job_id"], session_id, now, expires_at, now),
            )
            cur.execute(
                """
                UPDATE striatumd.queue_messages
                SET state = 'claimed', claimed_at = %s, updated_at = %s,
                    current_lease_id = %s, claim_count = claim_count + 1
                WHERE repository_id = %s AND message_id = %s
                """,
                (now, now, lease_id, ctx.repository_id, chosen["message_id"]),
            )
            cur.execute(
                """
                UPDATE striatumd.jobs
                SET state = 'claimed', current_lease_id = %s, started_at = %s
                WHERE repository_id = %s AND job_id = %s
                """,
                (lease_id, now, ctx.repository_id, job["job_id"]),
            )
        packet = build_packet(
            ctx,
            run=run,
            session=session,
            job=job,
            message_id=str(chosen["message_id"]),
            lease_id=lease_id,
            lease_expires_at=expires_at,
            packet_id=packet_id,
        )
        packet_json = json_dumps(packet)
        with ctx.cursor() as cur:
            cur.execute(
                """
                INSERT INTO striatumd.work_packets (
                  repository_id, packet_id, run_id, job_id, message_id, lease_id,
                  session_id, packet_json, packet_sha256, created_at
                )
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                """,
                (
                    ctx.repository_id,
                    packet_id,
                    run["run_id"],
                    job["job_id"],
                    chosen["message_id"],
                    lease_id,
                    session_id,
                    _jsonb(packet),
                    sha256_bytes(packet_json.encode("utf-8")),
                    now,
                ),
            )
        ctx.append_event(
            run_id=str(run["run_id"]),
            event_type="queue.claimed",
            actor_session_id=session_id,
            job_id=str(job["job_id"]),
            message_id=str(chosen["message_id"]),
            lease_id=lease_id,
        )
        delivery = _deliver_packet_to_attached_supervisor(
            ctx,
            run_id=str(run["run_id"]),
            session_id=session_id,
            packet_id=packet_id,
            packet_json=packet_json,
        )
        response: dict[str, Any] = {"status": "claimed", "packet": packet}
        if delivery is not None:
            response["supervisor_delivery"] = delivery
        return response


def _expire_leases(ctx: RepoHandlerContext, *, run_id: str) -> None:
    now = ctx.now()
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT * FROM striatumd.leases
            WHERE repository_id = %s AND run_id = %s AND state = 'active' AND expires_at < %s
            FOR UPDATE
            """,
            (ctx.repository_id, run_id, now),
        )
        leases = [dict(row) for row in cur.fetchall()]
    for lease in leases:
        job = ctx.row_by_id("jobs", "job_id", str(lease["resource_id"]), for_update=True)
        message_id = job["current_message_id"]
        job_state = "stale_lease" if is_repo_write(job) else "queued"
        message_state = "blocked" if is_repo_write(job) else "pending"
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.leases
                SET state = 'expired', released_at = %s, release_reason = 'expired'
                WHERE repository_id = %s AND lease_id = %s
                """,
                (now, ctx.repository_id, lease["lease_id"]),
            )
            cur.execute(
                """
                UPDATE striatumd.jobs
                SET state = %s, current_lease_id = NULL
                WHERE repository_id = %s AND job_id = %s
                """,
                (job_state, ctx.repository_id, job["job_id"]),
            )
            if message_id is not None:
                cur.execute(
                    """
                    UPDATE striatumd.queue_messages
                    SET state = %s, current_lease_id = NULL, updated_at = %s
                    WHERE repository_id = %s AND message_id = %s
                    """,
                    (message_state, now, ctx.repository_id, message_id),
                )
        ctx.append_event(
            run_id=run_id,
            event_type="lease.expired",
            job_id=str(job["job_id"]),
            message_id=message_id,
            lease_id=str(lease["lease_id"]),
            payload={"job_state": job_state, "message_state": message_state},
        )
        _mark_supervisor_lost_for_lease(
            ctx,
            run_id=run_id,
            session_id=str(lease["owner_session_id"]),
            lease_id=str(lease["lease_id"]),
        )
        _abandon_active_worktree_for_lease(ctx, run_id=run_id, job=job, lease_id=str(lease["lease_id"]))


def _deliver_packet_to_attached_supervisor(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    session_id: str,
    packet_id: str,
    packet_json: str,
) -> dict[str, Any] | None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT *
            FROM striatumd.process_supervisors
            WHERE repository_id = %s AND session_id = %s AND state = 'attached'
            ORDER BY started_at DESC
            LIMIT 1
            FOR UPDATE
            """,
            (ctx.repository_id, session_id),
        )
        row = cur.fetchone()
    if row is None:
        return None

    supervisor_id = str(row["supervisor_id"])
    pipe_text = row.get("stdin_pipe_path")
    pipe_path = Path(str(pipe_text)) if pipe_text else None
    payload = packet_json if packet_json.endswith("\n") else packet_json + "\n"
    payload_bytes = payload.encode("utf-8")

    error: str | None = None
    bytes_written = 0
    if pipe_path is None or not pipe_path.exists():
        error = "stdin_pipe_missing"
    else:
        try:
            from striatum.supervisor import _write_to_pipe

            bytes_written = int(_write_to_pipe(pipe_path, payload_bytes))
        except (BrokenPipeError, OSError, Exception):
            error = "stdin_pipe_missing"

    now = ctx.now()
    if error is not None:
        reason = "stdin pipe missing during auto-delivery"
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.process_supervisors
                SET state = 'lost', ended_at = %s, stop_reason = %s
                WHERE repository_id = %s AND supervisor_id = %s
                """,
                (now, reason, ctx.repository_id, supervisor_id),
            )
            cur.execute(
                """
                UPDATE striatumd.process_supervisor_pointers
                SET state = 'lost', updated_at = %s
                WHERE repository_id = %s AND supervisor_id = %s
                """,
                (now, ctx.repository_id, supervisor_id),
            )
        ctx.append_event(
            run_id=run_id,
            event_type="supervisor.lost",
            actor_session_id=session_id,
            payload={
                "supervisor_id": supervisor_id,
                "pid": row.get("pid"),
                "reason": reason,
                "phase": "claim_next_auto_delivery",
            },
        )
        return {"supervisor_id": supervisor_id, "error": error}

    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.process_supervisors
            SET heartbeat_at = %s
            WHERE repository_id = %s AND supervisor_id = %s
            """,
            (now, ctx.repository_id, supervisor_id),
        )
        cur.execute(
            """
            UPDATE striatumd.process_supervisor_pointers
            SET updated_at = %s
            WHERE repository_id = %s AND supervisor_id = %s
            """,
            (now, ctx.repository_id, supervisor_id),
        )
    ctx.append_event(
        run_id=run_id,
        event_type="supervisor.packet_delivered",
        actor_session_id=session_id,
        payload={
            "supervisor_id": supervisor_id,
            "packet_id": packet_id,
            "bytes_written": bytes_written,
            "via": "claim_next_auto_delivery",
        },
    )
    return {"supervisor_id": supervisor_id, "bytes_written": bytes_written}


def _mark_supervisor_lost_for_lease(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    session_id: str,
    lease_id: str,
) -> None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT supervisor_id, pid
            FROM striatumd.process_supervisors
            WHERE repository_id = %s AND session_id = %s AND state = 'attached'
            ORDER BY started_at DESC
            LIMIT 1
            FOR UPDATE
            """,
            (ctx.repository_id, session_id),
        )
        row = cur.fetchone()
    if row is None:
        return
    now = ctx.now()
    supervisor_id = str(row["supervisor_id"])
    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.process_supervisors
            SET state = 'lost', ended_at = %s, stop_reason = 'lease expired while attached'
            WHERE repository_id = %s AND supervisor_id = %s
            """,
            (now, ctx.repository_id, supervisor_id),
        )
        cur.execute(
            """
            UPDATE striatumd.process_supervisor_pointers
            SET state = 'lost', updated_at = %s
            WHERE repository_id = %s AND supervisor_id = %s
            """,
            (now, ctx.repository_id, supervisor_id),
        )
    ctx.append_event(
        run_id=run_id,
        event_type="supervisor.lease_expired_with_supervisor",
        actor_session_id=session_id,
        lease_id=lease_id,
        payload={"supervisor_id": supervisor_id, "pid": row.get("pid")},
    )


def _abandon_active_worktree_for_lease(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    job: Mapping[str, Any],
    lease_id: str,
) -> None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT *
            FROM striatumd.job_worktrees
            WHERE repository_id = %s AND job_id = %s AND state = 'active'
            LIMIT 1
            FOR UPDATE
            """,
            (ctx.repository_id, job["job_id"]),
        )
        row = cur.fetchone()
    if row is None or str(row["lease_id"]) != lease_id:
        return
    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.job_worktrees
            SET state = 'abandoned'
            WHERE repository_id = %s AND worktree_id = %s
            """,
            (ctx.repository_id, row["worktree_id"]),
        )
    ctx.append_event(
        run_id=run_id,
        event_type="worktree.abandoned",
        job_id=str(job["job_id"]),
        lease_id=lease_id,
        payload={
            "worktree_id": str(row["worktree_id"]),
            "worktree_path": str(row["worktree_path"]),
            "reason": "lease_expired",
        },
    )
