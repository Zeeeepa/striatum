"""Connection and doctor helpers for daemon PostgreSQL."""

from __future__ import annotations

from typing import Any

from striatum.daemon_pg.config import PostgresConfig, onboarding_hints, resolve_config
from striatum.daemon_pg.migrations import LATEST_DAEMON_DB_VERSION, apply_migrations, read_schema_version
from striatum.daemon_pg.roles import DEFAULT_DAEMON_RW_ROLE, repair_role_grants, role_repair_sql
from striatum.errors import SchemaVersionError, StriatumError

MIN_POSTGRES_VERSION_NUM = 140000


def connect(url: str) -> Any:
    try:
        import psycopg
    except ImportError as exc:
        raise StriatumError(
            "daemon PostgreSQL support requires the optional psycopg package; install striatum-orchestrator[daemon-pg]",
            exit_code=13,
        ) from exc
    return psycopg.connect(url)


def connect_and_migrate(*, postgres_url: str | None = None, daemon_version: str = "unknown") -> Any:
    config = resolve_config(postgres_url=postgres_url)
    if config.url is None:
        raise StriatumError("daemon PostgreSQL URL is not configured", exit_code=13)
    conn = connect(config.url)
    _check_supported_version(conn)
    apply_migrations(conn, daemon_version=daemon_version)
    return conn


def doctor(
    *,
    postgres_url: str | None = None,
    apply: bool = False,
    provision_rw_role: bool = False,
    repair_grants: bool = False,
) -> dict[str, Any]:
    config = resolve_config(postgres_url=postgres_url)
    result: dict[str, Any] = {
        "configured": config.url is not None,
        "url_source": config.source,
        "redacted_url": config.redacted_url,
        "config_path": str(config.config_path),
        "supported_floor": MIN_POSTGRES_VERSION_NUM,
        "latest_supported_schema": LATEST_DAEMON_DB_VERSION,
        "onboarding_hints": onboarding_hints() if config.url is None else [],
        "runtime_role": DEFAULT_DAEMON_RW_ROLE,
        "ok": False,
    }
    if config.url is None:
        result["status"] = "missing_url"
        return result
    try:
        conn = connect(config.url)
    except StriatumError as exc:
        result["status"] = "driver_missing"
        result["error"] = str(exc)
        return result
    except Exception as exc:  # noqa: BLE001 - doctor must report connection failures.
        result["status"] = "unreachable"
        result["error_type"] = type(exc).__name__
        result["error"] = _scrub_error(str(exc), config)
        result["onboarding_hints"] = onboarding_hints()
        return result
    try:
        server_version_num = _server_version_num(conn)
        result["server_version_num"] = server_version_num
        if server_version_num < MIN_POSTGRES_VERSION_NUM:
            result["status"] = "unsupported_version"
            return result
        if provision_rw_role:
            result["role_provisioning"] = repair_role_grants(conn, provision_role=True)
        if apply:
            schema_version = apply_migrations(conn)
        else:
            schema_version = _read_existing_schema_version(conn)
        result["schema_version"] = schema_version
        if schema_version > LATEST_DAEMON_DB_VERSION:
            raise SchemaVersionError(
                f"daemon PostgreSQL schema version {schema_version} is newer than supported {LATEST_DAEMON_DB_VERSION}"
            )
        if repair_grants or (provision_rw_role and schema_version > 0):
            result["grant_repair"] = repair_role_grants(conn, provision_role=False)
        result["privileges"] = _privilege_summary(conn)
        if result["privileges"]["audit_log_exists"] and not result["privileges"]["append_only_role_ok"]:
            # Test harnesses run the daemon as the ephemeral-DB owner — owners
            # have implicit UPDATE/DELETE which the daemon doctor would
            # ordinarily refuse with `unsafe_privileges`. Behind an explicit
            # env opt-in, demote the refusal to a recorded warning so the
            # multi-repo / cross-repo / MCP test suites can start a daemon
            # without provisioning a dedicated low-privilege role per
            # ephemeral DB. Production deployments must NEVER set this.
            import os
            if os.environ.get("STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK") == "1":
                result["audit_status"] = "not_initialized" if schema_version == 0 else "available"
                result["status"] = "ok"
                result["ok"] = True
                result["warning"] = (
                    "daemon role has UPDATE/DELETE on audit tables — allowed because "
                    "STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK=1 is set"
                )
                return result
            result["status"] = "unsafe_privileges"
            result["error"] = "daemon role must not have UPDATE or DELETE on striatumd.audit_log"
            result["repair_sql"] = role_repair_sql(
                database=str(result["privileges"]["database"]),
                role=DEFAULT_DAEMON_RW_ROLE,
            )
            result["repair_hint"] = (
                "run `striatum daemon doctor --provision-rw-role --repair-grants "
                "--apply-migrations` as a database owner/admin, then configure "
                "the daemon to connect as striatumd_rw"
            )
            return result
        result["audit_status"] = "not_initialized" if schema_version == 0 else "available"
        result["status"] = "ok"
        result["ok"] = True
        return result
    except SchemaVersionError:
        raise
    except Exception as exc:  # noqa: BLE001
        result["status"] = "doctor_failed"
        result["error_type"] = type(exc).__name__
        result["error"] = _scrub_error(str(exc), config)
        return result
    finally:
        conn.close()


def _server_version_num(conn: Any) -> int:
    with conn.cursor() as cur:
        cur.execute("SHOW server_version_num")
        row = cur.fetchone()
    return int(row[0])


def _check_supported_version(conn: Any) -> None:
    version = _server_version_num(conn)
    if version < MIN_POSTGRES_VERSION_NUM:
        raise StriatumError(
            f"PostgreSQL server_version_num {version} is below supported floor {MIN_POSTGRES_VERSION_NUM}",
            exit_code=13,
        )


def _read_existing_schema_version(conn: Any) -> int:
    try:
        return read_schema_version(conn)
    except Exception:  # noqa: BLE001 - absent schema means not initialized.
        return 0


def _privilege_summary(conn: Any) -> dict[str, Any]:
    with conn.cursor() as cur:
        cur.execute("SELECT current_user, current_database()")
        user_row = cur.fetchone()
        cur.execute("SELECT to_regclass('striatumd.audit_log') IS NOT NULL")
        audit_exists = bool(cur.fetchone()[0])
        can_update_audit = False
        can_delete_audit = False
        if audit_exists:
            cur.execute("SELECT has_table_privilege(current_user, 'striatumd.audit_log', 'UPDATE')")
            can_update_audit = bool(cur.fetchone()[0])
            cur.execute("SELECT has_table_privilege(current_user, 'striatumd.audit_log', 'DELETE')")
            can_delete_audit = bool(cur.fetchone()[0])
    return {
        "user": user_row[0],
        "database": user_row[1],
        "audit_log_exists": audit_exists,
        "audit_update_allowed": can_update_audit,
        "audit_delete_allowed": can_delete_audit,
        "append_only_role_ok": audit_exists and not can_update_audit and not can_delete_audit,
    }


def _scrub_error(message: str, config: PostgresConfig) -> str:
    if config.url and config.redacted_url:
        return message.replace(config.url, config.redacted_url)
    return message
