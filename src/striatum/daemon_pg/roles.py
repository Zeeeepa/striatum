"""Role provisioning helpers for the daemon PostgreSQL substrate."""

from __future__ import annotations

import re
from typing import Any

DEFAULT_DAEMON_RW_ROLE = "striatumd_rw"
APPEND_ONLY_TABLES = ("audit_log", "events", "artifacts")


def repair_role_grants(
    conn: Any,
    *,
    provision_role: bool = False,
    role: str = DEFAULT_DAEMON_RW_ROLE,
) -> dict[str, Any]:
    """Provision or repair the local daemon runtime role.

    The helper intentionally keeps the role name fixed and local. It does not
    change the connection URL; operators still need to reconnect the daemon as
    ``striatumd_rw`` after provisioning.
    """
    _validate_role(role)
    database = _current_database(conn)
    applied: list[str] = []
    try:
        if provision_role and not _role_exists(conn, role):
            _execute(conn, f"CREATE ROLE {_quote_ident(role)} WITH LOGIN")
            applied.append("create_role")
        if not _role_exists(conn, role):
            return {
                "ok": False,
                "status": "role_missing",
                "role": role,
                "database": database,
                "applied": applied,
                "manual_sql": role_repair_sql(database=database, role=role),
            }
        _execute(
            conn,
            f"GRANT CONNECT ON DATABASE {_quote_ident(database)} TO {_quote_ident(role)}",
        )
        applied.append("grant_database_connect")
        if not _schema_exists(conn, "striatumd"):
            conn.commit()
            return {
                "ok": True,
                "status": "schema_missing",
                "role": role,
                "database": database,
                "applied": applied,
                "manual_sql": role_repair_sql(database=database, role=role),
                "hint": "run daemon doctor with --apply-migrations, then --repair-grants",
            }
        _execute(conn, f"GRANT USAGE ON SCHEMA striatumd TO {_quote_ident(role)}")
        _execute(conn, f"GRANT CREATE ON DATABASE {_quote_ident(database)} TO {_quote_ident(role)}")
        _execute(conn, f"GRANT CREATE ON SCHEMA striatumd TO {_quote_ident(role)}")
        _execute(
            conn,
            "GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA striatumd "
            f"TO {_quote_ident(role)}",
        )
        _execute(
            conn,
            "GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA striatumd "
            f"TO {_quote_ident(role)}",
        )
        _execute(
            conn,
            "ALTER DEFAULT PRIVILEGES IN SCHEMA striatumd "
            f"GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO {_quote_ident(role)}",
        )
        _execute(
            conn,
            "ALTER DEFAULT PRIVILEGES IN SCHEMA striatumd "
            f"GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO {_quote_ident(role)}",
        )
        applied.append("grant_schema_runtime")
        for table in APPEND_ONLY_TABLES:
            if not _table_exists(conn, table):
                continue
            _execute(
                conn,
                f"REVOKE UPDATE, DELETE ON striatumd.{_quote_ident(table)} FROM {_quote_ident(role)}",
            )
            _execute(
                conn,
                f"GRANT SELECT, INSERT ON striatumd.{_quote_ident(table)} TO {_quote_ident(role)}",
            )
            applied.append(f"protect_{table}")
        if _table_exists(conn, "repo_event_chain_heads"):
            _execute(
                conn,
                "GRANT SELECT, INSERT, UPDATE ON striatumd.repo_event_chain_heads "
                f"TO {_quote_ident(role)}",
            )
            _execute(
                conn,
                f"REVOKE DELETE ON striatumd.repo_event_chain_heads FROM {_quote_ident(role)}",
            )
            applied.append("protect_repo_event_chain_heads")
        conn.commit()
    except Exception as exc:  # noqa: BLE001 - doctor should report repair failures.
        conn.rollback()
        return {
            "ok": False,
            "status": "repair_failed",
            "role": role,
            "database": database,
            "applied": applied,
            "error_type": type(exc).__name__,
            "error": str(exc),
            "manual_sql": role_repair_sql(database=database, role=role),
        }
    return {
        "ok": True,
        "status": "applied",
        "role": role,
        "database": database,
        "applied": applied,
        "manual_sql": role_repair_sql(database=database, role=role),
        "hint": "set STRIATUM_DAEMON_DB_URL to connect as striatumd_rw before starting the daemon",
    }


def role_repair_sql(
    *,
    database: str = "striatum_daemon",
    role: str = DEFAULT_DAEMON_RW_ROLE,
) -> list[str]:
    """Return operator-pasteable SQL for the daemon runtime role."""
    _validate_role(role)
    quoted_database = _quote_ident(database)
    quoted_role = _quote_ident(role)
    return [
        f"CREATE ROLE {quoted_role} WITH LOGIN PASSWORD '<yourpass>';",
        f"GRANT CONNECT ON DATABASE {quoted_database} TO {quoted_role};",
        "GRANT USAGE ON SCHEMA striatumd TO " + quoted_role + ";",
        f"GRANT CREATE ON DATABASE {quoted_database} TO {quoted_role};",
        "GRANT CREATE ON SCHEMA striatumd TO " + quoted_role + ";",
        "GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA striatumd "
        f"TO {quoted_role};",
        "GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA striatumd "
        f"TO {quoted_role};",
        "ALTER DEFAULT PRIVILEGES IN SCHEMA striatumd "
        f"GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO {quoted_role};",
        "ALTER DEFAULT PRIVILEGES IN SCHEMA striatumd "
        f"GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO {quoted_role};",
        f"REVOKE UPDATE, DELETE ON striatumd.audit_log FROM {quoted_role};",
        f"REVOKE UPDATE, DELETE ON striatumd.events FROM {quoted_role};",
        f"REVOKE UPDATE, DELETE ON striatumd.artifacts FROM {quoted_role};",
        f"GRANT SELECT, INSERT ON striatumd.audit_log TO {quoted_role};",
        f"GRANT SELECT, INSERT ON striatumd.events TO {quoted_role};",
        f"GRANT SELECT, INSERT ON striatumd.artifacts TO {quoted_role};",
        "GRANT SELECT, INSERT, UPDATE ON striatumd.repo_event_chain_heads "
        f"TO {quoted_role};",
        f"REVOKE DELETE ON striatumd.repo_event_chain_heads FROM {quoted_role};",
    ]


def _current_database(conn: Any) -> str:
    with conn.cursor() as cur:
        cur.execute("SELECT current_database()")
        row = cur.fetchone()
    return str(row[0])


def _role_exists(conn: Any, role: str) -> bool:
    with conn.cursor() as cur:
        cur.execute("SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = %s)", (role,))
        row = cur.fetchone()
    return bool(row[0])


def _schema_exists(conn: Any, schema: str) -> bool:
    with conn.cursor() as cur:
        cur.execute(
            "SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = %s)",
            (schema,),
        )
        row = cur.fetchone()
    return bool(row[0])


def _table_exists(conn: Any, table: str) -> bool:
    with conn.cursor() as cur:
        cur.execute("SELECT to_regclass(%s) IS NOT NULL", (f"striatumd.{table}",))
        row = cur.fetchone()
    return bool(row[0])


def _execute(conn: Any, sql: str) -> None:
    with conn.cursor() as cur:
        cur.execute(sql)


def _validate_role(role: str) -> None:
    if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", role):
        raise ValueError(f"unsafe PostgreSQL role name: {role!r}")


def _quote_ident(identifier: str) -> str:
    return '"' + identifier.replace('"', '""') + '"'
