"""Audit-chain verification for daemon PostgreSQL rows."""

from __future__ import annotations

import hashlib
import hmac
from typing import Any

from striatum.db import json_dumps


def v1_row_hash(row: dict[str, Any]) -> str:
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
    return hashlib.sha256(json_dumps(material).encode("utf-8")).hexdigest()


def v2_row_hash(row: dict[str, Any]) -> str:
    material = {
        "ts": row["ts"],
        "schema_version": row["schema_version"],
        "hash_format_version": row["hash_format_version"],
        "daemon_version": row["daemon_version"],
        "client_id": row["client_id"],
        "repository_id": row["repository_id"],
        "method": row["method"],
        "decision": row["decision"],
        "denial_reason": row["denial_reason"],
        "transport": row["transport"],
        "request_id": row["request_id"],
        "exit_code": row["exit_code"],
        "params_sha256": row["params_sha256"],
        "previous_hash": row["previous_hash"],
        "segment_id": row["segment_id"],
    }
    return hashlib.sha256(json_dumps(material).encode("utf-8")).hexdigest()


def verify_rows(rows: list[dict[str, Any]]) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    previous: str | None = None
    for row in rows:
        if row["previous_hash"] != previous:
            records.append(
                {
                    "check": "daemon_pg_audit_chain",
                    "id": str(row["audit_id"]),
                    "message": "daemon PostgreSQL audit hash chain is broken",
                    "context": {"expected_previous_hash": previous, "actual_previous_hash": row["previous_hash"]},
                }
            )
            break
        if int(row["hash_format_version"]) == 1:
            computed = _imported_v1_hash(row)
        else:
            computed = v2_row_hash(row)
        if not hmac.compare_digest(computed, str(row["row_hash"])):
            records.append(
                {
                    "check": "daemon_pg_audit_row_hash",
                    "id": str(row["audit_id"]),
                    "message": "daemon PostgreSQL audit row hash is invalid",
                    "context": {"hash_format_version": row["hash_format_version"]},
                }
            )
            break
        previous = str(row["row_hash"])
    return records


def _imported_v1_hash(row: dict[str, Any]) -> str:
    return v1_row_hash(
        {
            "timestamp": row["ts"],
            "client_id": row["client_id"],
            "repository_id": int(row["repository_id"]) if row["repository_id"] is not None else None,
            "command": row["method"],
            "authorization_result": row["decision"],
            "denial_reason": row["denial_reason"],
            "transport": row["transport"],
            "request_id": row["request_id"],
            "exit_code": row["exit_code"],
            "payload_sha256": row["params_sha256"],
            "previous_hash": row["previous_hash"],
            "segment_id": int(row["segment_id"]),
        }
    )
