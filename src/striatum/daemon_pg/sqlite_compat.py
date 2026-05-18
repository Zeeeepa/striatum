"""SQLite-era daemon helpers retained for one-way migration fixtures."""

from __future__ import annotations

import hashlib
import hmac
import sqlite3
from pathlib import Path
from typing import Any

from striatum.primitives import json_dumps
from striatum.repo_policy import db_path


def repo_identity(repo: Path) -> str:
    """Return the pre-D094 daemon registry identity for a repo-local SQLite DB."""
    repo_stat = repo.stat()
    state_stat = db_path(repo).stat()
    return (
        f"inode:{repo_stat.st_dev}:{repo_stat.st_ino}:"
        f"state:{state_stat.st_dev}:{state_stat.st_ino}"
    )


def audit_chain_records(conn: sqlite3.Connection) -> list[dict[str, Any]]:
    """Validate a V1 SQLite daemon registry audit chain."""
    records: list[dict[str, Any]] = []
    previous: str | None = None
    rows = conn.execute("SELECT * FROM audit_log ORDER BY audit_id").fetchall()
    for row in rows:
        if row["previous_hash"] != previous:
            records.append(
                {
                    "check": "daemon_audit_chain",
                    "id": str(row["audit_id"]),
                    "message": "daemon audit hash chain is broken",
                    "context": {
                        "expected_previous_hash": previous,
                        "actual_previous_hash": row["previous_hash"],
                    },
                }
            )
            break
        material = {
            "timestamp": row["timestamp"],
            "client_id": row["client_id"],
            "repository_id": row["repository_id"],
            "command": row["command"],
            "authorization_result": row["authorization_result"],
            "denial_reason": row["denial_reason"],
            "transport": row["transport"],
            "request_id": row["request_id"],
            "exit_code": row["exit_code"],
            "payload_sha256": row["payload_sha256"],
            "previous_hash": row["previous_hash"],
            "segment_id": row["segment_id"],
        }
        computed = hashlib.sha256(json_dumps(material).encode("utf-8")).hexdigest()
        if not hmac.compare_digest(computed, str(row["row_hash"])):
            records.append(
                {
                    "check": "daemon_audit_row_hash",
                    "id": str(row["audit_id"]),
                    "message": "daemon audit row hash is invalid",
                    "context": {},
                }
            )
            break
        previous = str(row["row_hash"])
    records.extend(audit_segment_records(conn))
    return records


def audit_segment_records(conn: sqlite3.Connection) -> list[dict[str, Any]]:
    """Validate V1 SQLite daemon registry audit segment manifests."""
    records: list[dict[str, Any]] = []
    segments = conn.execute("SELECT * FROM audit_segments ORDER BY segment_id").fetchall()
    previous_segment_last_hash: str | None = None
    for segment in segments:
        segment_id = int(segment["segment_id"])
        rows = conn.execute(
            "SELECT * FROM audit_log WHERE segment_id = ? ORDER BY audit_id",
            (segment_id,),
        ).fetchall()
        if segment["state"] == "purged":
            if rows:
                records.append(
                    {
                        "check": "daemon_audit_segment_retention",
                        "id": str(segment_id),
                        "message": "purged audit segment still has retained rows",
                        "context": {},
                    }
                )
            previous_segment_last_hash = (
                str(segment["last_hash"]) if segment["last_hash"] is not None else previous_segment_last_hash
            )
            continue
        if segment["previous_segment_last_hash"] != previous_segment_last_hash:
            records.append(
                {
                    "check": "daemon_audit_segment_chain",
                    "id": str(segment_id),
                    "message": "daemon audit segment chain is broken",
                    "context": {
                        "expected_previous_segment_last_hash": previous_segment_last_hash,
                        "actual_previous_segment_last_hash": segment["previous_segment_last_hash"],
                    },
                }
            )
            break
        if not rows:
            if segment["first_audit_id"] is not None or segment["last_audit_id"] is not None:
                records.append(
                    {
                        "check": "daemon_audit_segment_rows",
                        "id": str(segment_id),
                        "message": "daemon audit segment manifest references missing rows",
                        "context": {},
                    }
                )
                break
            previous_segment_last_hash = (
                str(segment["last_hash"]) if segment["last_hash"] is not None else previous_segment_last_hash
            )
            continue
        first = rows[0]
        last = rows[-1]
        if (
            segment["first_audit_id"] != first["audit_id"]
            or segment["last_audit_id"] != last["audit_id"]
            or segment["first_hash"] != first["row_hash"]
            or segment["last_hash"] != last["row_hash"]
        ):
            records.append(
                {
                    "check": "daemon_audit_segment_manifest",
                    "id": str(segment_id),
                    "message": "daemon audit segment manifest does not match retained rows",
                    "context": {},
                }
            )
            break
        if first["previous_hash"] != previous_segment_last_hash:
            records.append(
                {
                    "check": "daemon_audit_segment_boundary",
                    "id": str(segment_id),
                    "message": "daemon audit segment boundary hash is broken",
                    "context": {
                        "expected_first_previous_hash": previous_segment_last_hash,
                        "actual_first_previous_hash": first["previous_hash"],
                    },
                }
            )
            break
        previous_segment_last_hash = str(last["row_hash"])
    return records
