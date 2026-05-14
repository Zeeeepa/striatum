"""PostgreSQL request-log and audit append helpers for daemon RPC."""

from __future__ import annotations

import hashlib
from datetime import UTC, datetime, timedelta
from typing import Any, Mapping

from striatum import __version__
from striatum.daemon_pg.audit import v2_row_hash
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.daemon_rpc.envelope import SUPPORTED_ENVELOPE_VERSION, RpcError
from striatum.db import json_dumps

DEFAULT_RETENTION_DAYS = 7


def canonical_hash(payload: Mapping[str, Any]) -> str:
    return hashlib.sha256(json_dumps(payload).encode("utf-8")).hexdigest()


def append_request_log(
    conn: Any,
    *,
    request_id: str,
    method: str,
    params: Mapping[str, Any],
    auth: RpcAuthContext,
    decision: str,
    response: Mapping[str, Any] | None = None,
    audit_id: int | None = None,
    retention_days: int = DEFAULT_RETENTION_DAYS,
) -> None:
    params_hash = canonical_hash(params)
    response_hash = canonical_hash(response or {})
    started_at = datetime.now(UTC)
    expires_at = started_at + timedelta(days=retention_days)
    with conn.cursor() as cur:
        if request_id_seen(conn, request_id=request_id):
            raise RpcError("duplicate_request", "daemon RPC request_id was already used")
        cur.execute(
            """
            INSERT INTO striatumd.rpc_request_log (
              request_id, client_id, repository_id, method, params_hash,
              response_hash, decision, audit_id, envelope_version, daemon_version,
              schema_version, started_at, finished_at, expires_at
            )
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
            """,
            (
                request_id,
                auth.client_id,
                auth.repository_id,
                method,
                params_hash,
                response_hash,
                decision,
                audit_id,
                str(SUPPORTED_ENVELOPE_VERSION),
                __version__,
                1,
                started_at,
                started_at,
                expires_at,
            ),
        )


def request_id_seen(conn: Any, *, request_id: str) -> bool:
    with conn.cursor() as cur:
        cur.execute(
            "SELECT request_id FROM striatumd.rpc_request_log WHERE request_id = %s",
            (request_id,),
        )
        return cur.fetchone() is not None


def append_audit_row(
    conn: Any,
    *,
    auth: RpcAuthContext,
    method: str,
    transport: str,
    request_id: str,
    params: Mapping[str, Any],
    exit_code: int | None = None,
) -> int | None:
    with conn.cursor() as cur:
        cur.execute("SELECT last_hash FROM striatumd.audit_chain_head WHERE singleton = true")
        head = cur.fetchone()
        # Support both tuple_row (default) and dict_row factories.
        if head is None:
            previous_hash = None
        elif isinstance(head, dict):
            previous_hash = head["last_hash"]
        else:
            previous_hash = head[0]
        cur.execute("SELECT segment_id FROM striatumd.audit_segments WHERE state = 'open' ORDER BY segment_id DESC LIMIT 1")
        segment = cur.fetchone()
        if segment is None:
            segment_id = 1
        elif isinstance(segment, dict):
            segment_id = segment["segment_id"]
        else:
            segment_id = segment[0]
    row = {
        "ts": datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z"),
        "schema_version": 1,
        "hash_format_version": 2,
        "daemon_version": __version__,
        "client_id": auth.client_id,
        "repository_id": auth.repository_id,
        "method": method,
        "decision": auth.decision,
        "denial_reason": auth.denial_reason,
        "transport": transport,
        "request_id": request_id,
        "exit_code": exit_code,
        "params_sha256": canonical_hash(params),
        "previous_hash": previous_hash,
        "segment_id": segment_id,
    }
    row_hash = v2_row_hash(row)
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.audit_log (
              ts, schema_version, hash_format_version, daemon_version,
              client_id, repository_id, method, decision, denial_reason,
              transport, request_id, exit_code, params_sha256, previous_hash,
              row_hash, segment_id
            )
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
            RETURNING audit_id
            """,
            (
                row["ts"],
                row["schema_version"],
                row["hash_format_version"],
                row["daemon_version"],
                row["client_id"],
                row["repository_id"],
                row["method"],
                row["decision"],
                row["denial_reason"],
                row["transport"],
                row["request_id"],
                row["exit_code"],
                row["params_sha256"],
                row["previous_hash"],
                row_hash,
                row["segment_id"],
            ),
        )
        inserted = cur.fetchone()
        if inserted is None:
            audit_id = None
        elif isinstance(inserted, dict):
            audit_id = int(inserted.get("audit_id") or list(inserted.values())[0])
        else:
            audit_id = int(inserted[0])
        cur.execute(
            """
            UPDATE striatumd.audit_chain_head
            SET last_audit_id = %s, last_hash = %s, updated_at = now()
            WHERE singleton = true
            """,
            (audit_id, row_hash),
        )
        cur.execute(
            """
            UPDATE striatumd.audit_segments
            SET first_audit_id = COALESCE(first_audit_id, %s),
                last_audit_id = %s,
                first_hash = COALESCE(first_hash, %s),
                last_hash = %s
            WHERE segment_id = %s
            """,
            (audit_id, audit_id, row_hash, row_hash, row["segment_id"]),
        )
    return audit_id
