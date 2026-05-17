"""Capability-token authorization helpers for daemon RPC."""

from __future__ import annotations

import hmac
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

from striatum.daemon_rpc.envelope import RpcError
from striatum.daemon_rpc.registry import CAPABILITIES
from striatum.daemon_rpc.token_hash import hash_token


@dataclass(frozen=True)
class RpcAuthContext:
    client_id: str | None
    token_id: str | None
    repository_id: str | None
    capability: str | None
    decision: str
    denial_reason: str | None = None


def authorize(
    conn: Any,
    *,
    required: str | None,
    repository_id: str | None,
    token: str | None,
) -> RpcAuthContext:
    if required is None:
        return RpcAuthContext(None, None, repository_id, None, "allowed")
    if required not in CAPABILITIES:
        raise RpcError("schema_invalid", f"unknown daemon RPC capability {required!r}")
    if token is None:
        return RpcAuthContext(None, None, repository_id, None, "denied", "token_missing")
    token_id, sep, secret = token.partition(".")
    if not sep or not token_id or not secret:
        return RpcAuthContext(None, token_id or None, repository_id, None, "denied", "token_malformed")
    with _dict_cursor(conn) as cur:
        cur.execute("SELECT * FROM striatumd.clients WHERE token_id = %s", (token_id,))
        row = cur.fetchone()
    if row is None:
        return RpcAuthContext(None, token_id, repository_id, None, "denied", "token_invalid")
    record = _row_dict(row)
    client_id = str(record["client_id"])
    expected = hash_token(secret=secret, salt=str(record["token_salt"]))
    if not hmac.compare_digest(expected, str(record["token_hash"])):
        return RpcAuthContext(client_id, token_id, repository_id, None, "denied", "token_invalid")
    if record.get("revoked_at") is not None:
        return RpcAuthContext(client_id, token_id, repository_id, None, "denied", "token_revoked")
    if _expired(record.get("expires_at")):
        return RpcAuthContext(client_id, token_id, repository_id, None, "denied", "token_expired")
    with _dict_cursor(conn) as cur:
        cur.execute(
            """
            SELECT * FROM striatumd.client_capabilities
            WHERE client_id = %s AND capability = %s
              AND (repository_id IS NULL OR repository_id = %s)
              AND revoked_at IS NULL
            ORDER BY repository_id IS NULL
            LIMIT 1
            """,
            (client_id, required, repository_id),
        )
        cap = cur.fetchone()
    if cap is None:
        if repository_id is not None:
            with _dict_cursor(conn) as cur:
                cur.execute(
                    """
                    SELECT repository_id FROM striatumd.client_capabilities
                    WHERE client_id = %s AND capability = %s
                      AND repository_id IS NOT NULL
                      AND repository_id <> %s
                      AND revoked_at IS NULL
                    LIMIT 1
                    """,
                    (client_id, required, repository_id),
                )
                scoped_elsewhere = cur.fetchone()
            if scoped_elsewhere is not None:
                return RpcAuthContext(
                    client_id,
                    token_id,
                    repository_id,
                    None,
                    "denied",
                    "capability_scope_mismatch",
                )
        return RpcAuthContext(client_id, token_id, repository_id, None, "denied", "capability_missing")
    cap_record = _row_dict(cap)
    if _expired(cap_record.get("expires_at")):
        return RpcAuthContext(client_id, token_id, repository_id, None, "denied", "capability_expired")
    with conn.cursor() as cur:
        cur.execute("UPDATE striatumd.clients SET last_used_at = now() WHERE client_id = %s", (client_id,))
    return RpcAuthContext(client_id, token_id, repository_id, required, "allowed")


def require_allowed(context: RpcAuthContext) -> None:
    if context.decision != "allowed":
        raise RpcError(context.denial_reason or "capability_missing", "daemon RPC authorization failed")


def _dict_cursor(conn: Any) -> Any:
    """Return a cursor that yields dict rows regardless of connection
    row_factory.

    Production daemon connections set ``row_factory = psycopg.rows.dict_row``
    at the connection level (see :mod:`striatum.daemon` start path), but
    test fixtures call :func:`striatum.daemon_pg.connection.connect`
    directly which defaults to tuple rows. Forcing dict rows per-cursor
    keeps both call sites correct without changing the global default
    (which would break positional access elsewhere in ``daemon_pg``).
    """
    try:
        from psycopg.rows import dict_row
    except ImportError:
        return conn.cursor()
    return conn.cursor(row_factory=dict_row)


def _row_dict(row: Any) -> dict[str, Any]:
    if isinstance(row, dict):
        return row
    if hasattr(row, "keys"):
        return {key: row[key] for key in row.keys()}
    raise TypeError("database row must expose mapping-like keys")


def _expired(value: Any) -> bool:
    if value is None:
        return False
    if isinstance(value, datetime):
        dt = value
    else:
        text = str(value).replace("Z", "+00:00")
        dt = datetime.fromisoformat(text)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=UTC)
    return dt <= datetime.now(UTC)
