"""Repository-scoped SQL helpers shared across recovery + evidence handlers.

Synthesis L19 mandates that every SQL statement include
``repository_id = %(repository_id)s`` in both reads and writes. The helpers
here centralize that predicate so handlers cannot accidentally forget it —
each row-by-id read, lazy-expire, and audit append goes through one of the
functions below.

All queries use psycopg's named-parameter style (``%(name)s``) to keep call
sites readable when many predicates share names with column names.
"""

from __future__ import annotations

import json
from datetime import UTC, datetime, timedelta
from typing import Any, Mapping

from ._shim import RepoHandlerContextProtocol


def utc_now() -> str:
    return datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def utc_in_seconds(seconds: int) -> str:
    return (
        (datetime.now(UTC) + timedelta(seconds=seconds))
        .replace(microsecond=0)
        .isoformat()
        .replace("+00:00", "Z")
    )


def row_by_id(
    ctx: RepoHandlerContextProtocol,
    *,
    table: str,
    id_column: str,
    value: str,
) -> dict[str, Any]:
    """Return a single repo-scoped row by primary key, or raise.

    The handler raises ``NotFoundError`` semantics by returning ``None`` and
    callers must check. Keeping this thin means handlers stay close to the
    SQL they execute.
    """
    with ctx.pg_conn.cursor() as cur:
        cur.execute(
            f"SELECT * FROM striatumd.{table} "
            f"WHERE repository_id = %(repository_id)s AND {id_column} = %(value)s",
            {"repository_id": ctx.repository_id, "value": value},
        )
        row = cur.fetchone()
        if row is None:
            return {}
        columns = [desc[0] for desc in cur.description]
        return dict(zip(columns, row, strict=True))


def fetch_all(
    ctx: RepoHandlerContextProtocol,
    *,
    sql: str,
    params: Mapping[str, Any],
) -> list[dict[str, Any]]:
    """Run a SELECT scoped to ``repository_id`` and return dict rows."""
    scoped = {"repository_id": ctx.repository_id, **params}
    with ctx.pg_conn.cursor() as cur:
        cur.execute(sql, scoped)
        rows = cur.fetchall()
        columns = [desc[0] for desc in cur.description]
        return [dict(zip(columns, row, strict=True)) for row in rows]


def execute(
    ctx: RepoHandlerContextProtocol,
    *,
    sql: str,
    params: Mapping[str, Any],
) -> None:
    scoped = {"repository_id": ctx.repository_id, **params}
    with ctx.pg_conn.cursor() as cur:
        cur.execute(sql, scoped)


def lock_run(ctx: RepoHandlerContextProtocol, *, run_id: str) -> dict[str, Any]:
    """``SELECT ... FOR UPDATE`` the run row before transitions."""
    with ctx.pg_conn.cursor() as cur:
        cur.execute(
            "SELECT * FROM striatumd.runs "
            "WHERE repository_id = %(repository_id)s AND run_id = %(run_id)s "
            "FOR UPDATE",
            {"repository_id": ctx.repository_id, "run_id": run_id},
        )
        row = cur.fetchone()
        if row is None:
            return {}
        columns = [desc[0] for desc in cur.description]
        return dict(zip(columns, row, strict=True))


def is_repo_write_scope(write_scope_json: Any) -> bool:
    """Evaluate repo-write scope from parsed write-scope JSON.

    A job is repo-write when its write scope is either ``mode=repo_write`` or
    its ``repo_write`` flag is truthy. This drives the lease-expiry policy
    (repo-write jobs go to ``stale_lease`` so operators inspect; others
    requeue as ``queued`` for automatic reclaim).
    """
    scope: Any
    if isinstance(write_scope_json, (bytes, str)):
        try:
            scope = json.loads(write_scope_json)
        except (json.JSONDecodeError, TypeError):
            return False
    elif isinstance(write_scope_json, Mapping):
        scope = dict(write_scope_json)
    else:
        return False
    if not isinstance(scope, dict):
        return False
    mode = scope.get("mode")
    if isinstance(mode, str) and mode == "repo_write":
        return True
    flag = scope.get("repo_write")
    return bool(flag)


def parse_json(value: Any, *, default: Any = None) -> Any:
    """Tolerant JSON parsing for jsonb / text columns returned by psycopg.

    psycopg returns ``jsonb`` columns as parsed Python (dict/list); ``text``
    columns hold the raw string. Both call sites converge here.
    """
    if value is None:
        return default
    if isinstance(value, (dict, list)):
        return value
    if isinstance(value, (bytes, str)):
        try:
            return json.loads(value)
        except (json.JSONDecodeError, TypeError):
            return default
    return default


def expire_leases(
    ctx: RepoHandlerContextProtocol,
    *,
    run_id: str,
) -> list[dict[str, Any]]:
    """PostgreSQL lazy lease expiry.

    Behavior:
      - Find ``leases`` rows in state ``active`` whose ``expires_at`` is in
        the past.
      - Transition each lease to ``expired`` with ``release_reason='expired'``.
      - Transition the lease's job to ``stale_lease`` (repo-write) or
        ``queued`` (non-repo-write), clearing ``current_lease_id``.
      - Transition the job's current ``queue_messages`` row to ``blocked``
        (repo-write) or ``pending`` (non-repo-write).
      - Mark any active worktree owned by the expired lease as ``abandoned``.
      - Append a ``lease.expired`` audit event per expiry; append a
        ``worktree.abandoned`` event when a worktree is reclaimed.

    Returns the list of expired-lease summaries so callers can chain
    further events from the same transaction (``recovery.stale_requeued``,
    ``recovery.auto_published``, etc.).
    """
    now = utc_now()
    expired_rows = fetch_all(
        ctx,
        sql=(
            "SELECT * FROM striatumd.leases "
            "WHERE repository_id = %(repository_id)s "
            "  AND run_id = %(run_id)s "
            "  AND state = 'active' "
            "  AND expires_at < %(now)s "
            "FOR UPDATE"
        ),
        params={"run_id": run_id, "now": now},
    )
    summaries: list[dict[str, Any]] = []
    for lease in expired_rows:
        job = row_by_id(ctx, table="jobs", id_column="job_id", value=str(lease["resource_id"]))
        if not job:
            continue
        message_id = job.get("current_message_id")
        repo_write = is_repo_write_scope(job.get("write_scope_json"))
        job_state = "stale_lease" if repo_write else "queued"
        message_state = "blocked" if repo_write else "pending"
        execute(
            ctx,
            sql=(
                "UPDATE striatumd.leases "
                "SET state = 'expired', released_at = %(now)s, "
                "    release_reason = 'expired' "
                "WHERE repository_id = %(repository_id)s "
                "  AND lease_id = %(lease_id)s"
            ),
            params={"now": now, "lease_id": lease["lease_id"]},
        )
        execute(
            ctx,
            sql=(
                "UPDATE striatumd.jobs "
                "SET state = %(state)s, current_lease_id = NULL "
                "WHERE repository_id = %(repository_id)s "
                "  AND job_id = %(job_id)s"
            ),
            params={"state": job_state, "job_id": job["job_id"]},
        )
        if message_id is not None:
            execute(
                ctx,
                sql=(
                    "UPDATE striatumd.queue_messages "
                    "SET state = %(state)s, current_lease_id = NULL, "
                    "    updated_at = %(now)s "
                    "WHERE repository_id = %(repository_id)s "
                    "  AND message_id = %(message_id)s"
                ),
                params={"state": message_state, "now": now, "message_id": message_id},
            )
        ctx.append_event(
            run_id=run_id,
            event_type="lease.expired",
            job_id=str(job["job_id"]),
            message_id=str(message_id) if message_id is not None else None,
            lease_id=str(lease["lease_id"]),
            payload={"job_state": job_state, "message_state": message_state},
        )
        worktree_rows = fetch_all(
            ctx,
            sql=(
                "SELECT worktree_id, lease_id, base_branch FROM striatumd.job_worktrees "
                "WHERE repository_id = %(repository_id)s "
                "  AND job_id = %(job_id)s "
                "  AND state = 'active' "
                "FOR UPDATE"
            ),
            params={"job_id": job["job_id"]},
        )
        for worktree in worktree_rows:
            if str(worktree.get("lease_id")) != str(lease["lease_id"]):
                continue
            execute(
                ctx,
                sql=(
                    "UPDATE striatumd.job_worktrees "
                    "SET state = 'abandoned' "
                    "WHERE repository_id = %(repository_id)s "
                    "  AND worktree_id = %(worktree_id)s"
                ),
                params={"worktree_id": worktree["worktree_id"]},
            )
            ctx.append_event(
                run_id=run_id,
                event_type="worktree.abandoned",
                job_id=str(job["job_id"]),
                lease_id=str(lease["lease_id"]),
                payload={
                    "worktree_id": str(worktree["worktree_id"]),
                    "base_branch": worktree.get("base_branch"),
                },
            )
        summaries.append(
            {
                "lease_id": str(lease["lease_id"]),
                "job_id": str(job["job_id"]),
                "message_id": str(message_id) if message_id is not None else None,
                "job_state": job_state,
                "message_state": message_state,
                "repo_write": repo_write,
            }
        )
    return summaries


def maybe_complete_run(
    ctx: RepoHandlerContextProtocol,
    *,
    run_id: str,
) -> dict[str, Any]:
    """Maybe move a run into a terminal state after job transitions.

    A run reaches a terminal state when every job is in a terminal state
    (``completed``, ``failed``, ``canceled``, ``skipped``). The terminal
    state is ``completed`` when every job is ``completed`` or ``skipped``,
    ``canceled`` when at least one is ``canceled`` and the rest are
    terminal, otherwise ``failed``. Sessions whose lifecycle was scoped to
    the run close automatically.
    """
    run = lock_run(ctx, run_id=run_id)
    if not run or run.get("state") not in {"ready", "running"}:
        return run or {}
    job_rows = fetch_all(
        ctx,
        sql=(
            "SELECT state FROM striatumd.jobs "
            "WHERE repository_id = %(repository_id)s AND run_id = %(run_id)s"
        ),
        params={"run_id": run_id},
    )
    if not job_rows:
        return run
    terminal = {"completed", "failed", "canceled", "skipped"}
    states = [str(row["state"]) for row in job_rows]
    if not all(state in terminal for state in states):
        return run
    if any(state == "canceled" for state in states):
        new_state = "canceled"
        stop_reason = "job_canceled"
    elif any(state == "failed" for state in states):
        new_state = "failed"
        stop_reason = "job_failed"
    else:
        new_state = "completed"
        stop_reason = "all_jobs_terminal"
    now = utc_now()
    execute(
        ctx,
        sql=(
            "UPDATE striatumd.runs "
            "SET state = %(state)s, completed_at = %(now)s, "
            "    stop_reason = %(stop_reason)s "
            "WHERE repository_id = %(repository_id)s AND run_id = %(run_id)s"
        ),
        params={
            "state": new_state,
            "now": now,
            "stop_reason": stop_reason,
            "run_id": run_id,
        },
    )
    ctx.append_event(
        run_id=run_id,
        event_type=f"run.{new_state}",
        payload={"stop_reason": stop_reason},
    )
    session_rows = fetch_all(
        ctx,
        sql=(
            "SELECT session_id FROM striatumd.sessions "
            "WHERE repository_id = %(repository_id)s "
            "  AND run_id = %(run_id)s "
            "  AND state = 'active' "
            "FOR UPDATE"
        ),
        params={"run_id": run_id},
    )
    for session in session_rows:
        execute(
            ctx,
            sql=(
                "UPDATE striatumd.sessions "
                "SET state = 'closed', closed_at = %(now)s, "
                "    close_reason = 'run_terminal' "
                "WHERE repository_id = %(repository_id)s "
                "  AND session_id = %(session_id)s"
            ),
            params={"now": now, "session_id": session["session_id"]},
        )
        ctx.append_event(
            run_id=run_id,
            event_type="session.closed",
            actor_session_id=str(session["session_id"]),
            payload={"reason": "run_terminal"},
        )
    run["state"] = new_state
    run["completed_at"] = now
    run["stop_reason"] = stop_reason
    return run


__all__ = [
    "execute",
    "expire_leases",
    "fetch_all",
    "is_repo_write_scope",
    "lock_run",
    "maybe_complete_run",
    "parse_json",
    "row_by_id",
    "utc_in_seconds",
    "utc_now",
]
