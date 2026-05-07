"""Read-only ``striatum list ...`` handlers.

Each handler is a thin SELECT over the corresponding SQLite table with a
small set of optional filters. The output is a stable JSON envelope of the
shape ``{"items": [...], "count": N}``. When a ``--run-id`` filter is
present the handler first applies the lazy lease-expiry sweep so the rows
reflect post-reap state, mirroring the convention used by ``status``.

These handlers intentionally stay small and avoid duplicating workflow
semantics or business logic. They expose existing rows so operators can
answer questions like "what sessions are registered?", "what jobs exist
and where are they at?", and "what artifacts have been published?"
without grepping events or shaping ad-hoc SQL.
"""

from __future__ import annotations

import json
import sqlite3

from striatum.db import (
    JsonObject,
    expire_leases,
    json_loads,
)
from striatum.errors import NotFoundError, StriatumError


# Stable state vocabularies, kept in lockstep with the CHECK constraints in
# ``schema.py``. ``--state`` rejects values outside these sets with a clear
# error citing the valid options for that table.
RUN_STATES: tuple[str, ...] = (
    "needs_branch_confirmation",
    "ready",
    "running",
    "blocked",
    "completed",
    "failed",
    "canceled",
)

SESSION_STATES: tuple[str, ...] = ("active", "expired", "stopped", "lost")

JOB_STATES: tuple[str, ...] = (
    "blocked",
    "queued",
    "claimed",
    "running",
    "stale_lease",
    "waiting_human",
    "completed",
    "failed",
    "canceled",
    "skipped",
)

ARTIFACT_KINDS: tuple[str, ...] = (
    "prompt",
    "finding",
    "findings_ledger",
    "synthesis",
    "marker",
    "handoff",
    "decision",
    "patch_summary",
    "test_report",
    "other",
)


def _validate_choice(value: str | None, *, allowed: tuple[str, ...], label: str) -> None:
    if value is None:
        return
    if value not in allowed:
        options = ", ".join(allowed)
        raise StriatumError(
            f"unknown {label}: {value!r} (valid options: {options})",
            exit_code=2,
        )


def _envelope(items: list[JsonObject]) -> JsonObject:
    return {"items": items, "count": len(items)}


def _ensure_run_exists(conn: sqlite3.Connection, *, run_id: str) -> None:
    row = conn.execute("SELECT 1 FROM runs WHERE run_id = ?", (run_id,)).fetchone()
    if row is None:
        raise NotFoundError(f"run not found: {run_id}")


def list_runs(
    conn: sqlite3.Connection,
    *,
    state: str | None,
    limit: int,
) -> JsonObject:
    """Return runs with workflow context, oldest first.

    The default ``limit`` of 100 mirrors the documented CLI default; runs is
    a top-level list so a cap protects long-running repos from unbounded
    output. A negative or zero limit is treated as ``no limit`` for symmetry
    with status output, but the parser already rejects non-positive values.
    """
    _validate_choice(state, allowed=RUN_STATES, label="run state")
    sql = """
        SELECT r.run_id, r.state, r.branch_name, r.created_at, r.started_at,
               r.completed_at, w.workflow_id
        FROM runs r
        LEFT JOIN workflow_snapshots w
          ON w.workflow_snapshot_id = r.workflow_snapshot_id
        WHERE (? IS NULL OR r.state = ?)
        ORDER BY r.created_at, r.run_id
        LIMIT ?
    """
    rows = conn.execute(sql, (state, state, int(limit))).fetchall()
    items: list[JsonObject] = [dict(row) for row in rows]
    return _envelope(items)


def list_sessions(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    state: str | None,
    role: str | None,
    lane: str | None,
) -> JsonObject:
    """Return sessions registered for a run, oldest first.

    ``capabilities_json`` is parsed back to a list so callers do not have
    to redo the decoding. No limit cap is applied because session lists are
    already scoped to a single run.
    """
    _validate_choice(state, allowed=SESSION_STATES, label="session state")
    expire_leases(conn, run_id=run_id)
    _ensure_run_exists(conn, run_id=run_id)
    sql = """
        SELECT session_id, role_id, lane_id, slug, ordinal, state,
               capabilities_json, registered_at, last_heartbeat_at
        FROM sessions
        WHERE run_id = ?
          AND (? IS NULL OR state = ?)
          AND (? IS NULL OR role_id = ?)
          AND (? IS NULL OR lane_id = ?)
        ORDER BY registered_at, session_id
    """
    rows = conn.execute(
        sql,
        (run_id, state, state, role, role, lane, lane),
    ).fetchall()
    items: list[JsonObject] = []
    for row in rows:
        item = dict(row)
        raw_caps = item.pop("capabilities_json", "[]")
        try:
            parsed = json.loads(str(raw_caps))
        except json.JSONDecodeError:
            parsed = []
        item["capabilities"] = parsed if isinstance(parsed, list) else []
        items.append(item)
    return _envelope(items)


def list_jobs(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    state: str | None,
    workflow_job_id: str | None,
) -> JsonObject:
    """Return jobs for a run with the latest review verdict in line.

    ``lane_id`` is parsed out of the stored ``lane_selector_json`` so callers
    do not have to redo the decoding. For review jobs the most recent
    verdict (if any) is attached as ``verdict``; non-review jobs and review
    jobs without a verdict report ``None``.
    """
    _validate_choice(state, allowed=JOB_STATES, label="job state")
    expire_leases(conn, run_id=run_id)
    _ensure_run_exists(conn, run_id=run_id)
    sql = """
        SELECT j.job_id, j.workflow_job_id, j.state, j.role_id,
               j.lane_selector_json, j.attempt, j.max_attempts, j.job_type,
               j.created_at, j.started_at, j.completed_at,
               (
                 SELECT v.verdict FROM verdicts v
                 WHERE v.job_id = j.job_id
                 ORDER BY v.created_at DESC, v.verdict_id DESC
                 LIMIT 1
               ) AS verdict
        FROM jobs j
        WHERE j.run_id = ?
          AND (? IS NULL OR j.state = ?)
          AND (? IS NULL OR j.workflow_job_id = ?)
        ORDER BY j.created_at, j.workflow_job_id, j.attempt
    """
    rows = conn.execute(
        sql,
        (run_id, state, state, workflow_job_id, workflow_job_id),
    ).fetchall()
    items: list[JsonObject] = []
    for row in rows:
        item = dict(row)
        lane_selector = item.pop("lane_selector_json", "{}")
        try:
            selector = json_loads(str(lane_selector))
        except Exception:  # pragma: no cover - defensive
            selector = {}
        lane_value = selector.get("lane_id") if isinstance(selector, dict) else None
        item["lane_id"] = lane_value if isinstance(lane_value, str) else None
        # Drop the verdict key for non-review jobs to avoid implying it is
        # meaningful there. Review jobs without a verdict still report None.
        if item.get("job_type") != "review":
            item["verdict"] = None
        items.append(item)
    return _envelope(items)


def list_artifacts(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    kind: str | None,
) -> JsonObject:
    """Return published artifacts for a run with author identity.

    The author identity matches the structured byline produced by
    ``evidence_artifact_summaries`` (role/lane/ordinal/workflow_job_id),
    making the list self-describing without forcing callers to also export
    evidence. See ``striatum.cli.evidence`` for the canonical implementation.
    """
    _validate_choice(kind, allowed=ARTIFACT_KINDS, label="artifact kind")
    expire_leases(conn, run_id=run_id)
    _ensure_run_exists(conn, run_id=run_id)
    # Local import: ``evidence`` imports from ``introspect``, and pulling
    # ``evidence_artifact_summaries`` at module import time would create a
    # cross-module dependency that the test suite verifies stays minimal.
    from striatum.cli.evidence import evidence_artifact_summaries
    from striatum.db import workflow_for_run

    workflow = workflow_for_run(conn, run_id=run_id)
    summaries = evidence_artifact_summaries(conn, run_id=run_id, workflow=workflow)
    authors_by_artifact = {summary["artifact_id"]: summary.get("author") for summary in summaries}

    sql = """
        SELECT artifact_id, job_id, session_id, logical_name, artifact_kind,
               repo_path, content_sha256, size_bytes, created_at
        FROM artifacts
        WHERE run_id = ?
          AND (? IS NULL OR artifact_kind = ?)
        ORDER BY created_at, artifact_id
    """
    rows = conn.execute(sql, (run_id, kind, kind)).fetchall()
    items: list[JsonObject] = []
    for row in rows:
        item = dict(row)
        item["author"] = authors_by_artifact.get(item["artifact_id"])
        items.append(item)
    return _envelope(items)


def list_workflows(
    conn: sqlite3.Connection,
    *,
    limit: int,
) -> JsonObject:
    """Return workflow snapshots loaded into this repo, oldest first.

    The default ``limit`` of 100 caps the response on long-running repos
    that have loaded many workflows over time.
    """
    sql = """
        SELECT workflow_snapshot_id, workflow_id, workflow_version,
               source_path, content_sha256, loaded_at
        FROM workflow_snapshots
        ORDER BY loaded_at, workflow_snapshot_id
        LIMIT ?
    """
    rows = conn.execute(sql, (int(limit),)).fetchall()
    items: list[JsonObject] = [dict(row) for row in rows]
    return _envelope(items)
