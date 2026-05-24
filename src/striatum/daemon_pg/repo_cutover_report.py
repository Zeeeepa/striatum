"""Repository cutover verification reports."""

from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import canonical_event_hash


@dataclass(frozen=True)
class RepoCutoverReportOptions:
    repo: Path
    postgres_url: str


REPO_LOCAL_TABLE_NAMES: tuple[str, ...] = (
    "workflow_snapshots",
    "runs",
    "sessions",
    "jobs",
    "job_dependencies",
    "queue_messages",
    "leases",
    "work_packets",
    "artifacts",
    "verdicts",
    "blockers",
    "command_requests",
    "process_executions",
    "events",
    "job_worktrees",
    "process_supervisors",
    "process_supervisor_pointers",
)


def verify_repo_cutover(options: RepoCutoverReportOptions) -> dict[str, Any]:
    """Report repository registration and Postgres cutover health."""
    repo = options.repo.resolve()
    conn = connect(options.postgres_url)
    try:
        repository_id = _lookup_registered(conn, repo)
        registration = _registration_report(conn, repository_id)
        checkpoint = _existing_checkpoint(conn, repository_id)
        destination_counts = (
            _destination_counts(conn, repository_id) if repository_id is not None else {}
        )
        count_report = _count_cutover_report(destination_counts, checkpoint)
        file_report = _retired_state_file_cutover_report(checkpoint=checkpoint)
        event_chain = (
            _event_chain_report(conn, repository_id)
            if repository_id is not None
            else _event_chain_unregistered_report()
        )
    finally:
        conn.close()

    ok = (
        bool(registration["registered"])
        and checkpoint is not None
        and bool(count_report["ok"])
        and bool(file_report["ok"])
        and bool(event_chain["ok"])
    )
    recommendations: list[str] = []
    if not registration["registered"]:
        recommendations.append("register or migrate the target repository into daemon PostgreSQL")
    if checkpoint is None:
        recommendations.append(
            "import windows are closed; archive/remove legacy local state "
            "files and register the target repository with adopt or repo add --init"
        )
    recommendations.extend(str(item) for item in count_report["recommendations"])
    recommendations.extend(str(item) for item in file_report["recommendations"])
    recommendations.extend(str(item) for item in event_chain["recommendations"])
    return {
        "schema_version": "striatum.repo_cutover_report.v1",
        "ok": ok,
        "mode": "repo_cutover_verification",
        "verify_only": True,
        "repo": str(repo),
        "repository": registration,
        "checkpoint": {"present": checkpoint is not None, "record": checkpoint},
        "destination_counts": count_report,
        "retired_state_finalization": file_report,
        "event_chain": event_chain,
        "retired_state_exceptions": _retired_state_exception_notes(),
        "recommendations": recommendations,
    }


def _lookup_registered(conn: Any, repo: Path) -> str | None:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT repository_id
            FROM striatumd.repositories
            WHERE state != 'removed' AND repo_root = %s
            ORDER BY repository_id
            LIMIT 1
            """,
            (str(repo),),
        )
        row = cur.fetchone()
    return None if row is None else str(row[0])


def _existing_checkpoint(conn: Any, repository_id: str | None) -> dict[str, Any] | None:
    if repository_id is None:
        return None
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT source_user_version, source_event_manifest_sha256,
              source_artifact_manifest_sha256, source_state_db_sha256,
              migrated_at, tombstone_path, row_counts
            FROM striatumd.repo_migrations
            WHERE repository_id = %s AND source_substrate = 'sqlite'
              AND target_substrate = 'postgres'
            """,
            (repository_id,),
        )
        row = cur.fetchone()
    if row is None:
        return None
    return {
        "source_user_version": int(row[0]),
        "source_event_manifest_sha256": row[1],
        "source_artifact_manifest_sha256": row[2],
        "source_state_db_sha256": row[3],
        "migrated_at": _normalize_value(row[4]),
        "tombstone_path": row[5],
        "row_counts": row[6],
    }


def _registration_report(conn: Any, repository_id: str | None) -> dict[str, Any]:
    if repository_id is None:
        return {"registered": False, "repository_id": None}
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT repository_id, repo_identity, repo_root, state_db_path,
              display_name, registered_at, last_seen_at, last_schema_version, state
            FROM striatumd.repositories
            WHERE repository_id = %s
            """,
            (repository_id,),
        )
        row = cur.fetchone()
    if row is None:
        return {"registered": False, "repository_id": repository_id}
    return {
        "registered": True,
        "repository_id": str(row[0]),
        "repo_identity": row[1],
        "repo_root": row[2],
        "state_db_path": row[3],
        "display_name": row[4],
        "registered_at": _normalize_value(row[5]),
        "last_seen_at": _normalize_value(row[6]),
        "last_schema_version": row[7],
        "state": row[8],
    }


def _destination_counts(conn: Any, repository_id: str) -> dict[str, int]:
    counts: dict[str, int] = {}
    with conn.cursor() as cur:
        for table in REPO_LOCAL_TABLE_NAMES:
            cur.execute(
                f"SELECT COUNT(*) FROM striatumd.{table} WHERE repository_id = %s",
                (repository_id,),
            )
            counts[table] = int(cur.fetchone()[0])
    return counts


def _checkpoint_row_counts(checkpoint: dict[str, Any] | None) -> dict[str, int]:
    if checkpoint is None:
        return {}
    raw_counts = checkpoint.get("row_counts") or {}
    if isinstance(raw_counts, str):
        try:
            raw_counts = json.loads(raw_counts)
        except json.JSONDecodeError:
            raw_counts = {}
    if not isinstance(raw_counts, dict):
        return {}
    counts: dict[str, int] = {}
    for table in REPO_LOCAL_TABLE_NAMES:
        value = raw_counts.get(table, 0)
        try:
            counts[table] = int(value)
        except (TypeError, ValueError):
            counts[table] = 0
    return counts


def _count_cutover_report(
    destination_counts: dict[str, int],
    checkpoint: dict[str, Any] | None,
) -> dict[str, Any]:
    checkpoint_counts = _checkpoint_row_counts(checkpoint)
    violations: list[dict[str, Any]] = []
    for table, expected in checkpoint_counts.items():
        actual = int(destination_counts.get(table, 0))
        if actual < expected:
            violations.append(
                {"table": table, "checkpoint_count": expected, "actual_count": actual}
            )
    recommendations: list[str] = []
    if checkpoint is None:
        recommendations.append(
            "no repo_migrations checkpoint is present for retired local state -> postgres"
        )
    if violations:
        recommendations.append(
            "destination Postgres row counts are below the migration checkpoint; "
            "inspect the daemon DB or restore from backup"
        )
    return {
        "ok": checkpoint is not None and not violations,
        "actual": destination_counts,
        "checkpoint_minimum": checkpoint_counts,
        "not_below_checkpoint": not violations if checkpoint is not None else False,
        "violations": violations,
        "recommendations": recommendations,
    }


def _retired_state_file_cutover_report(
    *,
    checkpoint: dict[str, Any] | None,
) -> dict[str, Any]:
    recommendations: list[str] = []
    diagnosis: list[str] = []
    if checkpoint is None:
        status = "checkpoint_missing"
        ok = False
        recommendations.append("register the target repository with adopt or repo add --init")
    else:
        status = "retired_local_state_removed"
        ok = True
        diagnosis.append("retired repo-local file finalization is no longer inspected")
    return {
        "ok": ok,
        "status": status,
        "expected_action": "none",
        "source_state_absent": True,
        "sentinel_absent": True,
        "diagnosis": diagnosis,
        "recommendations": recommendations,
    }


def _event_chain_unregistered_report() -> dict[str, Any]:
    return {
        "ok": False,
        "status": "repository_not_registered",
        "event_count": 0,
        "anchored_event_count": 0,
        "legacy_unanchored_event_count": 0,
        "payload_side_anchor_count": 0,
        "head": None,
        "problems": [],
        "recommendations": ["repository must be registered before event-chain health can be checked"],
    }


def _event_chain_report(conn: Any, repository_id: str) -> dict[str, Any]:
    rows = _event_chain_rows(conn, repository_id)
    head = _event_chain_head(conn, repository_id)
    problems: list[dict[str, Any]] = []
    payload_side_anchor_count = 0
    anchored_count = 0
    legacy_unanchored_count = 0
    previous_anchored_hash: str | None = None
    anchored_seen = False
    latest_anchored: dict[str, Any] | None = None
    for row in rows:
        event_id = int(row["event_id"])
        payload = row.get("payload_json")
        if isinstance(payload, dict) and "_event_chain" in payload:
            payload_side_anchor_count += 1
            problems.append({"event_id": event_id, "problem": "payload_side_event_chain_anchor"})
        row_hash = row.get("row_hash")
        previous_hash = row.get("previous_hash")
        if isinstance(row_hash, str) and row_hash:
            anchored_count += 1
            if anchored_seen and previous_hash != previous_anchored_hash:
                problems.append(
                    {
                        "event_id": event_id,
                        "problem": "previous_hash_mismatch",
                        "expected_previous_hash": previous_anchored_hash,
                        "actual_previous_hash": previous_hash,
                    }
                )
            if not anchored_seen and previous_hash is not None:
                problems.append(
                    {
                        "event_id": event_id,
                        "problem": "first_anchored_event_has_previous_hash",
                        "actual_previous_hash": previous_hash,
                    }
                )
            computed_hash = canonical_event_hash(row, previous_hash=previous_hash)
            if computed_hash != row_hash:
                problems.append(
                    {
                        "event_id": event_id,
                        "problem": "row_hash_mismatch",
                        "expected_row_hash": computed_hash,
                        "actual_row_hash": row_hash,
                    }
                )
            anchored_seen = True
            previous_anchored_hash = row_hash
            latest_anchored = {"event_id": event_id, "row_hash": row_hash}
        else:
            legacy_unanchored_count += 1
            if anchored_seen:
                problems.append({"event_id": event_id, "problem": "unanchored_after_anchor"})
    if anchored_count:
        if head is None:
            problems.append({"problem": "missing_event_chain_head"})
        elif latest_anchored is not None and (
            int(head["last_event_id"]) != latest_anchored["event_id"]
            or head["last_hash"] != latest_anchored["row_hash"]
        ):
            problems.append(
                {
                    "problem": "event_chain_head_mismatch",
                    "expected": latest_anchored,
                    "actual": head,
                }
            )
    elif head is not None:
        problems.append({"problem": "head_without_anchored_events", "actual": head})
    status = _event_chain_status(
        event_count=len(rows),
        anchored_count=anchored_count,
        legacy_unanchored_count=legacy_unanchored_count,
        problems=problems,
    )
    recommendations: list[str] = []
    if problems:
        recommendations.append("run daemon doctor and inspect striatumd.events chain anchors")
    return {
        "ok": not problems,
        "status": status,
        "event_count": len(rows),
        "anchored_event_count": anchored_count,
        "legacy_unanchored_event_count": legacy_unanchored_count,
        "payload_side_anchor_count": payload_side_anchor_count,
        "head": head,
        "latest_anchored_event": latest_anchored,
        "problems": problems,
        "recommendations": recommendations,
    }


def _event_chain_rows(conn: Any, repository_id: str) -> list[dict[str, Any]]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT repository_id, event_id, run_id, event_type, actor_session_id,
              job_id, message_id, artifact_id, lease_id, payload_json, created_at,
              previous_hash, row_hash
            FROM striatumd.events
            WHERE repository_id = %s
            ORDER BY event_id
            """,
            (repository_id,),
        )
        rows = cur.fetchall()
        names = [desc[0] for desc in cur.description]
    return [dict(zip(names, row, strict=True)) for row in rows]


def _event_chain_head(conn: Any, repository_id: str) -> dict[str, Any] | None:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT last_event_id, last_hash, updated_at
            FROM striatumd.repo_event_chain_heads
            WHERE repository_id = %s
            """,
            (repository_id,),
        )
        row = cur.fetchone()
    if row is None:
        return None
    return {
        "last_event_id": int(row[0]),
        "last_hash": row[1],
        "updated_at": _normalize_value(row[2]),
    }


def _event_chain_status(
    *,
    event_count: int,
    anchored_count: int,
    legacy_unanchored_count: int,
    problems: list[dict[str, Any]],
) -> str:
    if problems:
        return "broken"
    if event_count == 0:
        return "empty"
    if anchored_count == 0:
        return "legacy_unanchored"
    if legacy_unanchored_count:
        return "mixed_legacy_and_anchored"
    return "anchored"


def _retired_state_exception_notes() -> list[dict[str, str]]:
    return [
        {
            "scope": "migration_source_import",
            "note": "writable import windows are closed; retired local files are not opened",
        },
        {
            "scope": "tests_and_fixtures",
            "note": "legacy local-state fixtures are deleted from active tests",
        },
    ]


def _normalize_value(value: Any) -> Any:
    if isinstance(value, datetime):
        if value.tzinfo is None:
            value = value.replace(tzinfo=UTC)
        value = value.astimezone(UTC).replace(microsecond=0)
        return value.isoformat().replace("+00:00", "Z")
    return value
