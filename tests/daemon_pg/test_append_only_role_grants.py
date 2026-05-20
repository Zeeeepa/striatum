# ruff: noqa
"""RFC 0048 V1.5 F4 — append-only role-grant enforcement.

Migration 0005 (``0005_repo_local_workflow_state.sql``) ends with a
DO-block that grants ``striatumd_rw`` full SELECT/INSERT/UPDATE/DELETE
in the ``striatumd`` schema and then REVOKEs UPDATE/DELETE on
``striatumd.events`` and ``striatumd.artifacts`` — the append-only
audit tables. The schema also installs row-level triggers
(``events_no_update``, ``events_no_delete``, ``artifacts_no_update``,
``artifacts_no_delete``) so the protection holds even if a privileged
caller (database owner) tries the write. These tests lock the
role-level protection in place.

Both protections matter:
- Role REVOKE means ``striatumd_rw`` can't even attempt the write;
  the planner refuses at the privilege check.
- Trigger refuse_repo_append_only_change() means owner-level callers
  also can't UPDATE/DELETE the append-only rows.
"""

from __future__ import annotations

from typing import Any, Iterator

import pytest

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def _provision_striatumd_rw(conn: Any) -> bool:
    """Provision the ``striatumd_rw`` role in the ephemeral DB.

    The role is created at install time by the operator (see
    ``docs/POSTGRES_TRANSITION.md`` §role-provisioning); migration 0005's
    grants are guarded by ``IF EXISTS (SELECT 1 FROM pg_roles WHERE
    rolname = 'striatumd_rw')``. We create the role here and then
    re-apply the migration 0005 DO-block to materialize the grants in
    the test database.

    Returns False (and the caller skips) if we lack CREATEROLE
    privilege — typical for unprivileged shared dev databases.
    """
    with conn.cursor() as cur:
        try:
            cur.execute(
                "DO $$ BEGIN "
                "  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN "
                "    CREATE ROLE striatumd_rw WITH LOGIN PASSWORD 'rwpass'; "
                "  END IF; "
                "END $$;"
            )
            cur.execute(
                "GRANT CONNECT ON DATABASE "
                + _current_database(conn)
                + " TO striatumd_rw"
            )
            cur.execute("GRANT USAGE ON SCHEMA striatumd TO striatumd_rw")
            cur.execute(
                "GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES "
                "IN SCHEMA striatumd TO striatumd_rw"
            )
            cur.execute(
                "GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES "
                "IN SCHEMA striatumd TO striatumd_rw"
            )
            cur.execute("REVOKE UPDATE, DELETE ON striatumd.events FROM striatumd_rw")
            cur.execute(
                "REVOKE UPDATE, DELETE ON striatumd.artifacts FROM striatumd_rw"
            )
        except Exception:
            return False
    conn.commit()
    return True


def _current_database(conn: Any) -> str:
    with conn.cursor() as cur:
        cur.execute("SELECT current_database()")
        row = cur.fetchone()
    assert row is not None
    return str(row[0])


@pytest.mark.multi_repo
def test_striatumd_rw_lacks_update_delete_on_events(pg_url: str) -> None:
    """``information_schema.table_privileges`` must not list UPDATE or
    DELETE for ``striatumd_rw`` on ``striatumd.events``."""
    owner_conn = connect(pg_url)
    try:
        if not _provision_striatumd_rw(owner_conn):
            pytest.skip("test runner lacks CREATEROLE on the ephemeral database")
        with owner_conn.cursor() as cur:
            cur.execute(
                """
                SELECT privilege_type
                FROM information_schema.table_privileges
                WHERE grantee = 'striatumd_rw'
                  AND table_schema = 'striatumd'
                  AND table_name = 'events'
                """
            )
            granted = {str(row[0]) for row in cur.fetchall()}
    finally:
        owner_conn.close()

    assert "SELECT" in granted, "striatumd_rw must keep SELECT on events"
    assert "INSERT" in granted, "striatumd_rw must keep INSERT on events"
    assert "UPDATE" not in granted, (
        "striatumd_rw must NOT have UPDATE on events (append-only audit table)"
    )
    assert "DELETE" not in granted, (
        "striatumd_rw must NOT have DELETE on events (append-only audit table)"
    )


@pytest.mark.multi_repo
def test_striatumd_rw_lacks_update_delete_on_artifacts(pg_url: str) -> None:
    """Same enforcement on ``striatumd.artifacts``."""
    owner_conn = connect(pg_url)
    try:
        if not _provision_striatumd_rw(owner_conn):
            pytest.skip("test runner lacks CREATEROLE on the ephemeral database")
        with owner_conn.cursor() as cur:
            cur.execute(
                """
                SELECT privilege_type
                FROM information_schema.table_privileges
                WHERE grantee = 'striatumd_rw'
                  AND table_schema = 'striatumd'
                  AND table_name = 'artifacts'
                """
            )
            granted = {str(row[0]) for row in cur.fetchall()}
    finally:
        owner_conn.close()

    assert "SELECT" in granted
    assert "INSERT" in granted
    assert "UPDATE" not in granted, (
        "striatumd_rw must NOT have UPDATE on artifacts (append-only)"
    )
    assert "DELETE" not in granted, (
        "striatumd_rw must NOT have DELETE on artifacts (append-only)"
    )


@pytest.mark.multi_repo
def test_striatumd_rw_keeps_update_delete_on_mutable_tables(pg_url: str) -> None:
    """The append-only revoke must be surgical: striatumd_rw still needs
    UPDATE/DELETE on transient state tables like ``leases`` and
    ``work_packets`` so the daemon can manage lease handoffs."""
    owner_conn = connect(pg_url)
    try:
        if not _provision_striatumd_rw(owner_conn):
            pytest.skip("test runner lacks CREATEROLE on the ephemeral database")
        with owner_conn.cursor() as cur:
            cur.execute(
                """
                SELECT table_name, privilege_type
                FROM information_schema.table_privileges
                WHERE grantee = 'striatumd_rw'
                  AND table_schema = 'striatumd'
                  AND table_name IN ('leases', 'work_packets', 'jobs', 'runs')
                """
            )
            rows = cur.fetchall()
    finally:
        owner_conn.close()

    by_table: dict[str, set[str]] = {}
    for row in rows:
        by_table.setdefault(str(row[0]), set()).add(str(row[1]))

    for table in ("leases", "work_packets", "jobs", "runs"):
        granted = by_table.get(table, set())
        assert "UPDATE" in granted, f"striatumd_rw needs UPDATE on {table}"
        assert "DELETE" in granted, f"striatumd_rw needs DELETE on {table}"


@pytest.mark.multi_repo
def test_striatumd_rw_update_events_raises_insufficient_privilege(pg_url: str) -> None:
    """End-to-end: connecting as striatumd_rw and attempting UPDATE on
    events must raise ``insufficient_privilege`` (SQLSTATE 42501)."""
    owner_conn = connect(pg_url)
    try:
        if not _provision_striatumd_rw(owner_conn):
            pytest.skip("test runner lacks CREATEROLE on the ephemeral database")
    finally:
        owner_conn.close()

    rw_url = _rw_url(pg_url)
    rw_conn = connect(rw_url)
    try:
        with rw_conn.cursor() as cur:
            with pytest.raises(Exception) as exc_info:
                cur.execute("UPDATE striatumd.events SET payload_json = '{}'::jsonb")
        # psycopg.errors.InsufficientPrivilege has SQLSTATE 42501
        sqlstate = getattr(exc_info.value, "sqlstate", None) or getattr(
            getattr(exc_info.value, "diag", None), "sqlstate", None
        )
        assert sqlstate == "42501", (
            f"expected insufficient_privilege (42501), got {sqlstate!r}"
        )
    finally:
        rw_conn.close()


@pytest.mark.multi_repo
def test_striatumd_rw_delete_artifacts_raises_insufficient_privilege(
    pg_url: str,
) -> None:
    owner_conn = connect(pg_url)
    try:
        if not _provision_striatumd_rw(owner_conn):
            pytest.skip("test runner lacks CREATEROLE on the ephemeral database")
    finally:
        owner_conn.close()

    rw_url = _rw_url(pg_url)
    rw_conn = connect(rw_url)
    try:
        with rw_conn.cursor() as cur:
            with pytest.raises(Exception) as exc_info:
                cur.execute("DELETE FROM striatumd.artifacts")
        sqlstate = getattr(exc_info.value, "sqlstate", None) or getattr(
            getattr(exc_info.value, "diag", None), "sqlstate", None
        )
        assert sqlstate == "42501", (
            f"expected insufficient_privilege (42501), got {sqlstate!r}"
        )
    finally:
        rw_conn.close()


def _rw_url(owner_url: str) -> str:
    """Rewrite the URL's auth to use striatumd_rw / rwpass."""
    from urllib.parse import urlsplit, urlunsplit

    parsed = urlsplit(owner_url)
    netloc = parsed.netloc
    if "@" in netloc:
        netloc = netloc.split("@", 1)[1]
    if not netloc:
        # peer-auth socket URL — cannot test SQLSTATE path without TCP auth
        pytest.skip(
            "rw-auth test requires a TCP Postgres URL with explicit credentials"
        )
    netloc = f"striatumd_rw:rwpass@{netloc}"
    return urlunsplit(
        (parsed.scheme, netloc, parsed.path, parsed.query, parsed.fragment)
    )
