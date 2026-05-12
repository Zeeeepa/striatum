"""Audit assertion helpers for daemon PG tests."""

from __future__ import annotations

from datetime import datetime
from typing import Any

from striatum.daemon_pg.audit import v2_row_hash


def audit_rows(conn: Any, *, transport: str | None = None) -> list[dict[str, object]]:
    sql = "SELECT * FROM striatumd.audit_log"
    args: tuple[object, ...] = ()
    if transport is not None:
        sql += " WHERE transport = %s"
        args = (transport,)
    sql += " ORDER BY audit_id"
    with conn.cursor() as cur:
        cur.execute(sql, args)
        rows = cur.fetchall()
        columns = [desc[0] for desc in cur.description]
    return [dict(zip(columns, row, strict=True)) for row in rows]


def assert_audit_chain(conn: Any) -> None:
    previous: str | None = None
    for row in audit_rows(conn):
        assert row["previous_hash"] == previous
        raw_ts = row["ts"]
        ts = (
            raw_ts.replace(microsecond=0).isoformat().replace("+00:00", "Z")
            if isinstance(raw_ts, datetime)
            else raw_ts
        )
        payload = {
            "ts": ts,
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
        assert row["row_hash"] == v2_row_hash(payload)
        previous = str(row["row_hash"])
