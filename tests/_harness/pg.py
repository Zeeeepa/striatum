# ruff: noqa
"""PostgreSQL setup helpers for the multi-repo test harness."""

from __future__ import annotations

import os
import re
import uuid
from dataclasses import dataclass
from importlib.util import find_spec
from typing import Any
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

import pytest

from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.config import ENV_DAEMON_DB_URL
from striatum.daemon_pg.migrations import apply_migrations

ENV_TEST_POSTGRES_URL = "STRIATUM_TEST_POSTGRES_URL"
ENV_REQUIRE_PG = "STRIATUM_MULTI_REPO_REQUIRE_PG"
DEFAULT_POSTGRES_URL = "postgresql:///postgres"


@dataclass(frozen=True)
class EphemeralPostgres:
    admin_url: str
    database_url: str
    database_name: str


def resolve_base_url() -> str:
    """Return a reachable Postgres URL or skip the current test.

    RFC 0039 V1.6 F3 (dogfood-047): when
    ``STRIATUM_MULTI_REPO_REQUIRE_PG=1`` is set, an unreachable Postgres
    fails the test instead of skipping. CI for the multi-repo / Go-core
    matrix sets this so a missing-Postgres environment can't masquerade
    as a green run.
    """
    require_pg = os.environ.get(ENV_REQUIRE_PG) == "1"
    if find_spec("psycopg") is None:
        msg = (
            "multi-repo harness requires psycopg; install the daemon-pg "
            "extra and run make pg-test"
        )
        if require_pg:
            pytest.fail(msg)
        pytest.skip(msg)
    candidates = [
        os.environ.get(ENV_TEST_POSTGRES_URL),
        os.environ.get(ENV_DAEMON_DB_URL),
        DEFAULT_POSTGRES_URL,
    ]
    for candidate in candidates:
        if not candidate:
            continue
        try:
            conn = connect(candidate)
            conn.close()
        except Exception:
            continue
        return candidate
    msg = (
        "multi-repo harness requires a reachable system PostgreSQL URL via "
        f"{ENV_TEST_POSTGRES_URL} or {ENV_DAEMON_DB_URL}; run make pg-test first"
    )
    if require_pg:
        pytest.fail(msg)
    pytest.skip(msg)


def create_ephemeral_database(base_url: str) -> EphemeralPostgres:
    require_pg = os.environ.get(ENV_REQUIRE_PG) == "1"
    db_name = f"striatum_test_{uuid.uuid4().hex}"
    admin_url = _database_url(base_url, "postgres")
    admin_conn = connect(admin_url)
    try:
        admin_conn.autocommit = True
        with admin_conn.cursor() as cur:
            try:
                cur.execute(f'CREATE DATABASE "{db_name}"')
            except Exception as exc:
                msg = f"multi-repo harness requires CREATE DATABASE privilege: {exc}"
                if require_pg:
                    pytest.fail(msg)
                pytest.skip(msg)
    finally:
        admin_conn.close()
    database_url = _database_url(base_url, db_name)
    conn = connect(database_url)
    try:
        apply_migrations(conn, daemon_version="test")
    finally:
        conn.close()
    return EphemeralPostgres(admin_url=admin_url, database_url=database_url, database_name=db_name)


def drop_ephemeral_database(ephemeral: EphemeralPostgres) -> None:
    admin_conn = connect(ephemeral.admin_url)
    try:
        admin_conn.autocommit = True
        with admin_conn.cursor() as cur:
            cur.execute(
                """
                SELECT pg_terminate_backend(pid)
                FROM pg_stat_activity
                WHERE datname = %s AND pid <> pg_backend_pid()
                """,
                (ephemeral.database_name,),
            )
            cur.execute(f'DROP DATABASE IF EXISTS "{ephemeral.database_name}"')
    finally:
        admin_conn.close()


def reset_daemon_db(conn: Any) -> None:
    """Clear daemon data tables while preserving schema and method metadata."""
    preserved = {
        "schema_meta",
        "schema_migrations",
        "rpc_methods",
    }
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT tablename
            FROM pg_tables
            WHERE schemaname = 'striatumd'
            ORDER BY tablename
            """
        )
        tables = [str(row[0]) for row in cur.fetchall() if str(row[0]) not in preserved]
        if tables:
            qualified = ", ".join(f'striatumd."{table}"' for table in tables)
            cur.execute(f"TRUNCATE {qualified} RESTART IDENTITY CASCADE")
        cur.execute(
            "INSERT INTO striatumd.audit_segments(opened_at, state, retention_state) "
            "VALUES (now(), 'open', 'active')"
        )
        cur.execute(
            "INSERT INTO striatumd.audit_chain_head(singleton, last_audit_id, last_hash) "
            "VALUES (true, NULL, NULL)"
        )
    conn.commit()


def _database_url(url: str, database: str) -> str:
    parsed = urlsplit(url)
    quoted = _quote_database_name(database)
    path = "/" + quoted
    if parsed.netloc == "" and parsed.scheme in {"postgresql", "postgres"}:
        suffix = ""
        if parsed.query:
            suffix += f"?{parsed.query}"
        if parsed.fragment:
            suffix += f"#{parsed.fragment}"
        return f"{parsed.scheme}:///{quoted}{suffix}"
    return urlunsplit((parsed.scheme, parsed.netloc, path, parsed.query, parsed.fragment))


def _quote_database_name(database: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9_]+", database):
        raise ValueError(f"unsafe database name {database!r}")
    return database


def url_without_database(url: str) -> str:
    parsed = urlsplit(url)
    query = urlencode(parse_qsl(parsed.query, keep_blank_values=True))
    return urlunsplit((parsed.scheme, parsed.netloc, "", query, parsed.fragment))
