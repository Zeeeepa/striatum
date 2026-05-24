"""PG handler for ``recovery.process_reconcile``.

Port of :func:`striatum.cli.recovery.process_reconcile` (lines 607-690).
Synthesis: docs/dogfood/057/DESIGN_SYNTHESIS.md L151-159.

The reconciler walks ``process_executions`` rows in state ``running`` for
``run_id``; for any whose PID is not live (``os.kill(pid, 0)`` raises
``ProcessLookupError``), it transitions the process to ``lost`` and
evaluates whether the job should be blocked because required outputs are
missing.
"""

from __future__ import annotations

import os
from datetime import UTC, datetime
from typing import Any, Mapping

from striatum.errors import NotFoundError

from ._shim import RepoHandlerContext, register_pg_handler
from ._sql import execute, fetch_all, parse_json, row_by_id, utc_now


def _pid_alive(pid: int) -> bool:
    """Return True when ``pid`` is reachable from this process.

    ``os.kill(pid, 0)`` raises:
      - ProcessLookupError (ESRCH) → process is gone.
      - PermissionError (EPERM)    → process exists but not ours.
      - returns None on success    → process is alive and reachable.

    The reconciler treats EPERM as "still running" (we can't act on it
    anyway), parity with the retired local-state path.
    """
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    return True


def _duration_seconds(started_at: str, ended_at: str) -> float:
    def parse(value: str) -> datetime:
        if value.endswith("Z"):
            value = value[:-1] + "+00:00"
        parsed = datetime.fromisoformat(value)
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=UTC)
        return parsed

    return max(0.0, (parse(ended_at) - parse(started_at)).total_seconds())


def _validate_outputs(
    ctx: RepoHandlerContext, *, job: dict[str, Any]
) -> tuple[list[str], bool]:
    """PG port of :func:`striatum.process_completion.validate_outputs`.

    Returns ``(missing_artifact_paths, review_verdict_missing)``.
    """
    expected = parse_json(job.get("expected_artifacts_json"), default=[])
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
    missing_paths: list[str] = []
    if required_paths:
        rows = fetch_all(
            ctx,
            sql=(
                "SELECT repo_path FROM striatumd.artifacts "
                "WHERE repository_id = %(repository_id)s "
                "  AND job_id = %(job_id)s"
            ),
            params={"job_id": job["job_id"]},
        )
        published = {str(r["repo_path"]) for r in rows}
        missing_paths = [p for p in required_paths if p not in published]
    verdict_missing = False
    if str(job.get("job_type")) == "review":
        verdict_rows = fetch_all(
            ctx,
            sql=(
                "SELECT verdict_id FROM striatumd.verdicts "
                "WHERE repository_id = %(repository_id)s "
                "  AND job_id = %(job_id)s LIMIT 1"
            ),
            params={"job_id": job["job_id"]},
        )
        verdict_missing = not verdict_rows
    return missing_paths, verdict_missing


def _evaluate_and_block(
    ctx: RepoHandlerContext,
    *,
    job: dict[str, Any],
    session_id: str,
    process_id: str,
    command: list[Any],
    duration_seconds: float,
) -> str | None:
    """Block a lost-process job when outputs are missing (PG-native).

    Mirrors :func:`striatum.process_completion.evaluate_and_block_after_reconcile`.
    Idempotent — if a blocker already exists for the job it returns ``None``.
    """
    missing_paths, verdict_missing = _validate_outputs(ctx, job=job)
    if not missing_paths and not verdict_missing:
        return None
    existing = fetch_all(
        ctx,
        sql=(
            "SELECT blocker_id FROM striatumd.blockers "
            "WHERE repository_id = %(repository_id)s "
            "  AND job_id = %(job_id)s "
            "  AND state = 'open' LIMIT 1"
        ),
        params={"job_id": job["job_id"]},
    )
    if existing:
        return None
    blocker_id = ctx.new_id("blk")
    blocker_kind = "process_lost_with_outputs_missing"
    now = utc_now()
    description = (
        f"process {process_id} was lost (external kill or runner exit); "
        f"required outputs missing: "
        f"{len(missing_paths)} artifact(s), verdict missing={verdict_missing}"
    )
    envelope = {
        "process_id": process_id,
        "command": command,
        "duration_seconds": duration_seconds,
        "missing_artifact_paths": missing_paths,
        "review_verdict_missing": verdict_missing,
    }
    execute(
        ctx,
        sql=(
            "INSERT INTO striatumd.blockers "
            "(repository_id, blocker_id, run_id, job_id, session_id, "
            " severity, blocker_kind, description, state, created_at, "
            " payload_json) "
            "VALUES (%(repository_id)s, %(blocker_id)s, %(run_id)s, "
            "        %(job_id)s, %(session_id)s, 'blocked', %(blocker_kind)s, "
            "        %(description)s, 'open', %(now)s, "
            "        %(payload_json)s::jsonb)"
        ),
        params={
            "blocker_id": blocker_id,
            "run_id": job["run_id"],
            "job_id": job["job_id"],
            "session_id": session_id,
            "blocker_kind": blocker_kind,
            "description": description,
            "now": now,
            "payload_json": __import__("json").dumps(envelope),
        },
    )
    execute(
        ctx,
        sql=(
            "UPDATE striatumd.jobs SET state = 'blocked' "
            "WHERE repository_id = %(repository_id)s "
            "  AND job_id = %(job_id)s"
        ),
        params={"job_id": job["job_id"]},
    )
    ctx.append_event(
        run_id=str(job["run_id"]),
        event_type="job.blocked",
        actor_session_id=session_id,
        job_id=str(job["job_id"]),
        payload={"blocker_id": blocker_id, "blocker_kind": blocker_kind},
    )
    return blocker_kind


@register_pg_handler("recovery.process_reconcile")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    """Reconcile ``process_executions`` rows in state ``running`` for ``run_id``.

    Each non-live PID gets transitioned to ``lost`` in its own
    serializable transaction (synthesis L157); when required outputs are
    missing, the job is blocked with kind ``process_lost_with_outputs_missing``.
    """
    run_id = str(params.get("run_id") or "")
    if not run_id:
        raise NotFoundError("recovery.process_reconcile requires run_id")
    run = row_by_id(ctx, table="runs", id_column="run_id", value=run_id)
    if not run:
        raise NotFoundError(f"run {run_id!r} not found")

    still_running: list[dict[str, Any]] = []
    transitioned_to_lost: list[dict[str, Any]] = []
    rows = fetch_all(
        ctx,
        sql=(
            "SELECT pe.* FROM striatumd.process_executions pe "
            "WHERE pe.repository_id = %(repository_id)s "
            "  AND pe.run_id = %(run_id)s "
            "  AND pe.state = 'running' "
            "ORDER BY pe.started_at"
        ),
        params={"run_id": run_id},
    )
    for row in rows:
        pid = row.get("pid")
        if pid is None:
            alive = False
        else:
            alive = _pid_alive(int(pid))
        if alive:
            still_running.append(
                {
                    "process_id": str(row["process_id"]),
                    "job_id": str(row["job_id"]),
                    "pid": int(pid) if pid is not None else None,
                    "started_at": str(row["started_at"]),
                }
            )
            continue
        process_id = str(row["process_id"])
        ended_at = utc_now()
        execute(
            ctx,
            sql=(
                "UPDATE striatumd.process_executions "
                "SET state = 'lost', ended_at = %(ended_at)s "
                "WHERE repository_id = %(repository_id)s "
                "  AND process_id = %(process_id)s"
            ),
            params={"ended_at": ended_at, "process_id": process_id},
        )
        ctx.append_event(
            run_id=run_id,
            event_type="process.lost",
            actor_session_id=str(row["session_id"]),
            job_id=str(row["job_id"]),
            lease_id=str(row["lease_id"]),
            payload={"process_id": process_id, "pid": pid},
        )
        process = row_by_id(
            ctx,
            table="process_executions",
            id_column="process_id",
            value=process_id,
        )
        job = row_by_id(
            ctx, table="jobs", id_column="job_id", value=str(process["job_id"])
        )
        command = parse_json(process.get("command_json"), default=[])
        if not isinstance(command, list):
            command = []
        started = str(process["started_at"])
        ended = str(process.get("ended_at") or utc_now())
        duration = _duration_seconds(started, ended)
        blocker_kind = _evaluate_and_block(
            ctx,
            job=job,
            session_id=str(process["session_id"]),
            process_id=process_id,
            command=command,
            duration_seconds=duration,
        )
        transitioned_to_lost.append(
            {
                "process_id": process_id,
                "job_id": str(row["job_id"]),
                "pid": int(pid) if pid is not None else None,
                "blocker_kind": blocker_kind,
            }
        )
    next_actions: list[str] = []
    if transitioned_to_lost:
        next_actions.append("inspect_blockers")
        next_actions.append("decide_recovery_path")
    return {
        "run_id": run_id,
        "still_running": still_running,
        "transitioned_to_lost": transitioned_to_lost,
        "next_actions": next_actions,
    }


__all__ = ["handle"]
