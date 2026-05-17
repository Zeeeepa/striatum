"""Capability-token helpers for daemon PG tests."""

from __future__ import annotations

import secrets
import uuid
from datetime import UTC, datetime, timedelta
from typing import Any

from striatum.daemon_rpc.token_hash import hash_token


def issue_token(
    conn: Any,
    *,
    capabilities: list[str],
    repo_id: str | None,
    expires_in: int | None = 3600,
) -> str:
    token_id = f"dtok_{secrets.token_urlsafe(12)}"
    secret = secrets.token_urlsafe(32)
    salt = secrets.token_hex(16)
    client_id = f"dclient_{uuid.uuid4().hex}"
    now = datetime.now(UTC)
    expires_at = None if expires_in is None else now + timedelta(seconds=expires_in)
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.clients(
              client_id, client_kind, display_name, token_id, token_hash,
              token_salt, created_at, expires_at
            )
            VALUES (%s, 'test', 'test client', %s, %s, %s, %s, %s)
            """,
            (client_id, token_id, hash_token(secret=secret, salt=salt), salt, now, expires_at),
        )
        for capability in capabilities:
            cur.execute(
                """
                INSERT INTO striatumd.client_capabilities(
                  capability_id, client_id, repository_id, capability, granted_at, expires_at
                )
                VALUES (%s, %s, %s, %s, %s, %s)
                """,
                (f"dcap_{uuid.uuid4().hex}", client_id, repo_id, capability, now, expires_at),
            )
    conn.commit()
    return f"{token_id}.{secret}"


def revoke_token(conn: Any, token: str) -> None:
    token_id = token.split(".", 1)[0]
    with conn.cursor() as cur:
        cur.execute("UPDATE striatumd.clients SET revoked_at = now() WHERE token_id = %s", (token_id,))
    conn.commit()


def expire_token(conn: Any, token: str) -> None:
    token_id = token.split(".", 1)[0]
    with conn.cursor() as cur:
        cur.execute("UPDATE striatumd.clients SET expires_at = now() - interval '1 second' WHERE token_id = %s", (token_id,))
    conn.commit()
