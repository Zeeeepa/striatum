from __future__ import annotations

from typing import Any

import pytest

from striatum.daemon_pg.roles import repair_role_grants, role_repair_sql


class FakeCursor:
    def __init__(self, conn: "FakeConn") -> None:
        self.conn = conn
        self.result: tuple[Any, ...] | None = None

    def __enter__(self) -> "FakeCursor":
        return self

    def __exit__(self, *_exc: object) -> None:
        return None

    def execute(self, sql: str, params: tuple[Any, ...] | None = None) -> None:
        self.conn.statements.append((sql, params))
        if sql.startswith("CREATE ROLE"):
            self.conn.role_exists = True
            self.result = None
        elif "SELECT current_database()" in sql:
            self.result = ("striatum_daemon",)
        elif "FROM pg_roles" in sql:
            self.result = (self.conn.role_exists,)
        elif "information_schema.schemata" in sql:
            self.result = (self.conn.schema_exists,)
        elif "to_regclass" in sql:
            table = str(params[0]).split(".")[-1] if params else ""
            self.result = (table in self.conn.tables,)
        else:
            self.result = None

    def fetchone(self) -> tuple[Any, ...] | None:
        return self.result


class FakeConn:
    def __init__(
        self,
        *,
        role_exists: bool = True,
        schema_exists: bool = True,
        tables: set[str] | None = None,
    ) -> None:
        self.role_exists = role_exists
        self.schema_exists = schema_exists
        self.tables = tables if tables is not None else {
            "audit_log",
            "events",
            "artifacts",
            "repo_event_chain_heads",
        }
        self.statements: list[tuple[str, tuple[Any, ...] | None]] = []
        self.commits = 0
        self.rollbacks = 0

    def cursor(self) -> FakeCursor:
        return FakeCursor(self)

    def commit(self) -> None:
        self.commits += 1

    def rollback(self) -> None:
        self.rollbacks += 1


def test_repair_role_grants_reports_missing_role_without_creating_it() -> None:
    conn = FakeConn(role_exists=False)

    result = repair_role_grants(conn, provision_role=False)

    assert result["ok"] is False
    assert result["status"] == "role_missing"
    assert result["manual_sql"] == role_repair_sql(database="striatum_daemon")
    assert not any(statement[0].startswith("CREATE ROLE") for statement in conn.statements)


def test_repair_role_grants_provisions_and_repairs_runtime_grants() -> None:
    conn = FakeConn(role_exists=False)

    result = repair_role_grants(conn, provision_role=True)

    sql = "\n".join(statement for statement, _params in conn.statements)
    assert result["ok"] is True
    assert "create_role" in result["applied"]
    assert "protect_audit_log" in result["applied"]
    assert "CREATE ROLE \"striatumd_rw\" WITH LOGIN" in sql
    assert "REVOKE UPDATE, DELETE ON striatumd.\"audit_log\" FROM \"striatumd_rw\"" in sql
    assert "REVOKE DELETE ON striatumd.repo_event_chain_heads FROM \"striatumd_rw\"" in sql
    assert conn.commits == 1


def test_repair_role_grants_handles_schema_missing_after_role_exists() -> None:
    conn = FakeConn(schema_exists=False)

    result = repair_role_grants(conn, provision_role=False)

    assert result["ok"] is True
    assert result["status"] == "schema_missing"
    assert result["hint"] == "run daemon doctor with --apply-migrations, then --repair-grants"
    assert conn.commits == 1


def test_role_repair_sql_rejects_unsafe_role_names() -> None:
    with pytest.raises(ValueError, match="unsafe PostgreSQL role name"):
        role_repair_sql(role="striatumd_rw; DROP DATABASE postgres")
