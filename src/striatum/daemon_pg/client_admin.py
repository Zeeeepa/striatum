"""PostgreSQL-backed daemon CLI/admin helpers.

This module owns the production CLI/admin client path that used to live in the
deleted Python daemon module. It deliberately has no SQLite registry fallback.
"""

from __future__ import annotations

import os
import secrets
import signal
import time
import uuid
from datetime import UTC, datetime
from pathlib import Path
from typing import Any, Mapping

from striatum.daemon_pg.config import resolve_config
from striatum.daemon_pg.repositories import repo_add_pg, repo_list_pg, repo_remove_pg, repo_resolve_pg
from striatum.daemon_rpc.token_hash import hash_token
from striatum.daemon_runtime import (
    ENV_RUNTIME as _ENV_RUNTIME,
    pid_path as _runtime_pid_path,
    read_runtime_token as _read_runtime_token,
    runtime_dir as _runtime_dir,
    token_file as _runtime_token_file,
    write_runtime_token as _write_runtime_token,
)
from striatum.errors import (
    EXIT_DAEMON_AUTH,
    EXIT_DAEMON_CAPABILITY,
    EXIT_DAEMON_REGISTRY,
    EXIT_RPC_VERSION_INCOMPATIBLE,
    NotFoundError,
    StriatumError,
)

PROTOCOL_VERSION = 1
READ_CAPABILITY = "read"
ADMIN_CAPABILITY = "admin"
DEFAULT_SWEEP_TIMEOUT_SECONDS = 30.0

ENV_RUNTIME = _ENV_RUNTIME
ENV_REGISTRY = "STRIATUM_DAEMON_REGISTRY"
ENV_SQLITE_CONNECT_TRIPWIRE = "STRIATUM_SQLITE_CONNECT_TRIPWIRE"


class DaemonUnreachableError(StriatumError):
    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_RPC_VERSION_INCOMPATIBLE)


class DaemonAuthError(StriatumError):
    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_DAEMON_AUTH)


class DaemonCapabilityError(StriatumError):
    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_DAEMON_CAPABILITY)


class DaemonRegistryError(StriatumError):
    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_DAEMON_REGISTRY)


def runtime_dir() -> Path:
    return _runtime_dir()


def token_file() -> Path:
    return _runtime_token_file()


def pid_path() -> Path:
    return _runtime_pid_path()


def read_runtime_token() -> str | None:
    return _read_runtime_token()


def write_runtime_token(token: str) -> None:
    _write_runtime_token(token)


def _pg_connection_configured(*, postgres_url: str | None = None) -> bool:
    return resolve_config(postgres_url=postgres_url).url is not None


def _connect_pg(*, postgres_url: str | None = None) -> Any:
    if not _pg_connection_configured(postgres_url=postgres_url):
        raise DaemonRegistryError(
            "daemon PostgreSQL URL is not configured; configure STRIATUM_DAEMON_DB_URL"
        )
    from striatum.daemon_pg.connection import connect_and_migrate

    return connect_and_migrate(postgres_url=postgres_url)


def _bootstrap_pg_admin_if_needed(pg_conn: Any) -> dict[str, str] | None:
    with pg_conn.cursor() as cur:
        cur.execute("SELECT COUNT(*) AS c FROM striatumd.clients")
        row = cur.fetchone()
    if row is not None:
        count = row["c"] if isinstance(row, dict) else row[0]
        if int(count) > 0:
            return None
    token_id = f"dtok_{secrets.token_urlsafe(12)}"
    secret = secrets.token_urlsafe(32)
    salt = secrets.token_hex(16)
    token_hash = hash_token(secret=secret, salt=salt)
    client_id = f"dclient_{uuid.uuid4().hex}"
    capabilities = (
        "admin",
        "read",
        "write",
        "claim",
        "review",
        "apply",
        "recovery",
        "surgical_recovery",
    )
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.clients(client_id, client_kind, display_name,
              token_id, token_hash, token_salt, created_at)
            VALUES (%s, %s, %s, %s, %s, %s, now())
            """,
            (client_id, "cli", "bootstrap-admin", token_id, token_hash, salt),
        )
        for capability in capabilities:
            cur.execute(
                """
                INSERT INTO striatumd.client_capabilities(capability_id, client_id,
                  repository_id, capability, granted_at)
                VALUES (%s, %s, NULL, %s, now())
                """,
                (f"dcap_{uuid.uuid4().hex}", client_id, capability),
            )
    token = f"{token_id}.{secret}"
    write_runtime_token(token)
    return {"client_id": client_id, "token_id": token_id, "token": token}


def _pg_instance_id(pg_conn: Any) -> str:
    with pg_conn.cursor() as cur:
        cur.execute("SELECT value FROM striatumd.daemon_meta WHERE key = 'instance_id'")
        row = cur.fetchone()
        if row is not None:
            return str(row["value"] if isinstance(row, dict) else row[0])
        value = uuid.uuid4().hex[:12]
        cur.execute(
            """
            INSERT INTO striatumd.daemon_meta(key, value, updated_at)
            VALUES (%s, %s, now())
            ON CONFLICT (key) DO UPDATE
            SET value = EXCLUDED.value, updated_at = now()
            """,
            ("instance_id", value),
        )
    try:
        pg_conn.commit()
    except Exception:  # noqa: BLE001 - autocommit connections do not need an explicit commit.
        pass
    return value


def _require_pg_auth(
    pg_conn: Any,
    *,
    command: str,
    required: str,
    repository_id: str | None = None,
    token: str | None = None,
    payload: Mapping[str, Any] | None = None,
) -> Any:
    from striatum.daemon_rpc.capability import authorize
    from striatum.daemon_rpc.request_log import append_audit_row

    auth = authorize(pg_conn, required=required, repository_id=repository_id, token=token)
    append_audit_row(
        pg_conn,
        auth=auth,
        method=command,
        transport="cli",
        request_id=f"cli_{uuid.uuid4().hex}",
        params=payload or {},
        exit_code=None if auth.decision == "allowed" else EXIT_DAEMON_CAPABILITY,
    )
    try:
        pg_conn.commit()
    except Exception:  # noqa: BLE001 - autocommit connections do not need an explicit commit.
        pass
    if auth.decision != "allowed":
        if auth.denial_reason and auth.denial_reason.startswith("token_"):
            raise DaemonAuthError(f"daemon authorization failed: {auth.denial_reason}")
        raise DaemonCapabilityError(f"daemon authorization failed: {auth.denial_reason}")
    return auth


def repo_add(
    path: Path,
    *,
    display_name: str | None = None,
    no_migrate: bool = False,
    init: bool = False,
) -> dict[str, Any]:
    pg_conn = _connect_pg()
    try:
        bootstrap = _bootstrap_pg_admin_if_needed(pg_conn)
        if bootstrap is not None:
            pg_conn.commit()
        token = read_runtime_token()
        _require_pg_auth(
            pg_conn,
            command="repo.add",
            required=ADMIN_CAPABILITY,
            token=token,
            payload={"path": str(path), "no_migrate": no_migrate, "init": init},
        )
        result = repo_add_pg(
            pg_conn,
            path,
            display_name=display_name,
            no_migrate=no_migrate,
            init=init,
        )
        if bootstrap is not None:
            result["bootstrap_admin"] = bootstrap
        return result
    finally:
        pg_conn.close()


def repo_list() -> dict[str, Any]:
    pg_conn = _connect_pg()
    try:
        _bootstrap_pg_admin_if_needed(pg_conn)
        pg_conn.commit()
        _require_pg_auth(
            pg_conn,
            command="repo.list",
            required=READ_CAPABILITY,
            token=read_runtime_token(),
        )
        return repo_list_pg(pg_conn)
    finally:
        pg_conn.close()


def repo_remove(identifier: str) -> dict[str, Any]:
    pg_conn = _connect_pg()
    try:
        _bootstrap_pg_admin_if_needed(pg_conn)
        pg_conn.commit()
        _require_pg_auth(
            pg_conn,
            command="repo.remove",
            required=ADMIN_CAPABILITY,
            token=read_runtime_token(),
            payload={"id": identifier},
        )
        return repo_remove_pg(pg_conn, identifier)
    finally:
        pg_conn.close()


def daemon_status() -> dict[str, Any]:
    try:
        pg_conn = _connect_pg()
    except StriatumError:
        raise
    except Exception as exc:  # noqa: BLE001 - status should not leak tracebacks.
        raise _daemon_status_pg_unavailable(exc) from exc
    try:
        return daemon_status_pg(pg_conn, token=read_runtime_token())
    finally:
        pg_conn.close()


def _daemon_status_pg_unavailable(exc: BaseException) -> StriatumError:
    sqlstate = getattr(exc, "sqlstate", None) or getattr(
        getattr(exc, "diag", None),
        "sqlstate",
        None,
    )
    if sqlstate == "42501":
        message = (
            "daemon PostgreSQL status unavailable: pending migrations require "
            f"database owner/admin privileges; runtime role was refused: {exc}"
        )
    else:
        message = f"daemon PostgreSQL status unavailable: {exc}"
    error = StriatumError(message, exit_code=EXIT_DAEMON_REGISTRY)
    setattr(
        error,
        "hint",
        "run `striatum daemon doctor --apply-migrations --as-owner "
        "<owner-url> --repair-grants` (GH #22) so the owner role applies "
        "the pending migrations, then retry `striatum daemon status`; the "
        "owner URL may be a local peer-auth socket such as "
        "`postgresql:///striatum_daemon`",
    )
    return error


def daemon_status_pg(pg_conn: Any, *, token: str | None = None) -> dict[str, Any]:
    _require_pg_auth(
        pg_conn,
        command="daemon.status",
        required=READ_CAPABILITY,
        token=token,
    )
    pid = _read_pid()
    return {
        "mode": "daemon",
        "protocol_version": PROTOCOL_VERSION,
        "substrate": "postgres",
        "runtime_dir": str(runtime_dir()),
        "pid": pid,
        "running": _pid_alive(pid) if pid is not None else False,
        "instance_id": _pg_instance_id(pg_conn),
    }


def _read_pid() -> int | None:
    path = pid_path()
    if not path.exists():
        return None
    try:
        return int(path.read_text(encoding="utf-8").strip())
    except ValueError:
        return None


def _pid_alive(pid: int | None) -> bool:
    if pid is None:
        return False
    try:
        os.kill(pid, 0)
    except OSError:
        return False
    return True


def daemon_stop() -> dict[str, Any]:
    # GH #22: stop must succeed when migrations are pending. Read the pidfile
    # and SIGTERM the daemon first; PG authorization/audit happens after the
    # signal as best-effort so a stale schema cannot block recovery.
    pid = _read_pid()
    if pid is None or not _pid_alive(pid):
        result: dict[str, Any] = {"stopped": False, "reason": "not_running"}
    else:
        os.kill(pid, signal.SIGTERM)
        result = {"stopped": True, "pid": pid}
    _best_effort_audit_stop(result)
    return result


def _best_effort_audit_stop(result: Mapping[str, Any]) -> None:
    """Append a `daemon.stop` audit row when PG is reachable without migrating.

    Connects with `connect()` directly so pending migrations never block stop
    (GH #22). Authorization and audit append are wrapped so a failure here
    never prevents the SIGTERM result from being returned to the caller.
    """
    if not _pg_connection_configured():
        return
    try:
        from striatum.daemon_pg.connection import connect

        config = resolve_config()
        if config.url is None:
            return
        pg_conn = connect(config.url)
    except Exception:  # noqa: BLE001 - audit is advisory; SIGTERM is authoritative.
        return
    try:
        _require_pg_auth(
            pg_conn,
            command="daemon.stop",
            required=ADMIN_CAPABILITY,
            token=read_runtime_token(),
            payload=dict(result),
        )
    except Exception:  # noqa: BLE001 - audit is advisory; SIGTERM is authoritative.
        try:
            pg_conn.rollback()
        except Exception:  # noqa: BLE001
            pass
    finally:
        try:
            pg_conn.close()
        except Exception:  # noqa: BLE001
            pass


def read_doctor(
    repo: Path | None = None,
    *,
    run_id: str | None = None,
    verbose: bool = False,
    token: str | None = None,
    postgres_url: str | None = None,
) -> dict[str, Any]:
    pg_conn = _connect_pg(postgres_url=postgres_url)
    try:
        _bootstrap_pg_admin_if_needed(pg_conn)
        try:
            pg_conn.commit()
        except Exception:  # noqa: BLE001 - autocommit connections do not need an explicit commit.
            pass
        return read_doctor_pg(pg_conn, repo=repo, run_id=run_id, verbose=verbose, token=token)
    finally:
        pg_conn.close()


def read_doctor_pg(
    pg_conn: Any,
    *,
    repo: Path | None = None,
    run_id: str | None = None,
    verbose: bool = False,
    token: str | None = None,
) -> dict[str, Any]:
    from striatum.daemon_pg.blob_doctor import fetch_blob_doctor_block
    from striatum.daemon_pg.handlers.context import RepoHandlerContext
    from striatum.daemon_pg.handlers.reads.doctor import doctor_payload

    token = read_runtime_token() if token is None else token
    if repo is None:
        _require_pg_auth(
            pg_conn,
            command="doctor",
            required=READ_CAPABILITY,
            token=token,
            payload={"run_id": run_id, "verbose": verbose},
        )
        problems = daemon_doctor_records_pg(pg_conn)
        data: dict[str, Any] = {"problems": [str(p["message"]) for p in problems]}
        if verbose:
            data["problem_records"] = problems
        data["blob"] = fetch_blob_doctor_block("")
        return {"mode": "daemon", "protocol_version": PROTOCOL_VERSION, **data}

    try:
        repo_row = repo_resolve_pg(pg_conn, repo)
    except NotFoundError as exc:
        raise DaemonCapabilityError("repository is not registered with daemon") from exc
    repository_id = str(repo_row["repository_id"])
    auth = _require_pg_auth(
        pg_conn,
        command="doctor",
        required=READ_CAPABILITY,
        repository_id=repository_id,
        token=token,
        payload={"run_id": run_id, "verbose": verbose},
    )
    ctx = RepoHandlerContext(
        pg_conn=pg_conn,
        repository_id=repository_id,
        repo_root=Path(str(repo_row["repo_root"])),
        auth=auth,
    )
    data = doctor_payload(ctx, run_id=run_id, verbose=verbose)
    data["blob"] = fetch_blob_doctor_block(repository_id)
    daemon_problems = daemon_doctor_records_pg(pg_conn)
    data["daemon_problems"] = [str(p["message"]) for p in daemon_problems]
    if verbose:
        data["daemon_problem_records"] = daemon_problems
    return {"mode": "daemon", "repository_id": repository_id, "protocol_version": PROTOCOL_VERSION, **data}


def daemon_doctor_records_pg(pg_conn: Any) -> list[dict[str, Any]]:
    from striatum.daemon_pg.audit import verify_rows

    records: list[dict[str, Any]] = []
    rt = runtime_dir()
    if rt.exists() and rt.stat().st_mode & 0o077:
        records.append({"check": "daemon_runtime_permissions", "id": str(rt), "message": "daemon runtime directory is not owner-only", "context": {"mode": oct(rt.stat().st_mode & 0o777)}})
    if token_file().exists() and token_file().stat().st_mode & 0o077:
        records.append({"check": "daemon_token_permissions", "id": str(token_file()), "message": "daemon token fallback file is not owner-only", "context": {"mode": oct(token_file().stat().st_mode & 0o777)}})
    with _pg_dict_cursor(pg_conn) as cur:
        cur.execute("SELECT * FROM striatumd.audit_log ORDER BY audit_id")
        audit_rows = [_pg_json_ready(_pg_row_dict(row)) for row in cur.fetchall()]
        records.extend(verify_rows(audit_rows))
        cur.execute(
            """
            SELECT repository_id, repo_root, state_db_path
            FROM striatumd.repositories
            WHERE state = 'active'
            ORDER BY repository_id
            """
        )
        active_repos = [_pg_json_ready(_pg_row_dict(row)) for row in cur.fetchall()]
        cur.execute(
            """
            SELECT COUNT(*) AS count
            FROM striatumd.audit_log a
            JOIN striatumd.repositories r ON r.repository_id = a.repository_id
            WHERE r.state = 'removed'
            """
        )
        removed_refs = _pg_row_dict(cur.fetchone())
        cur.execute(
            """
            SELECT repository_id, run_id, last_result_json
            FROM striatumd.scheduler_cursors
            WHERE state = 'sweep_degraded'
            ORDER BY repository_id, run_id
            """
        )
        degraded = [_pg_json_ready(_pg_row_dict(row)) for row in cur.fetchall()]
    for row in active_repos:
        state_path = Path(str(row["state_db_path"]))
        if not state_path.exists():
            records.append({"check": "daemon_repo_state_missing", "id": str(row["repository_id"]), "message": "registered repository operational scratch is missing", "context": {"repo_root": row["repo_root"], "state_db_path": str(state_path)}})
    if int(removed_refs["count"]) > 0:
        records.append({"check": "daemon_removed_repo_audit_refs", "id": "audit_log", "message": "removed repository ids remain in retained audit rows", "context": {"count": int(removed_refs["count"])}})
    for row in degraded:
        records.append({"check": "daemon_sweep_degraded", "id": f"{row['repository_id']}:{row['run_id']}", "message": "daemon recovery sweep is degraded", "context": {"last_result_json": row["last_result_json"]}})
    return records


def dashboard_all(*, token: str | None = None) -> dict[str, Any]:
    from striatum.daemon_pg.mcp_resources import dashboard_all_resource_pg

    pg_conn = _connect_pg()
    try:
        return dashboard_all_resource_pg(
            pg_conn,
            token=read_runtime_token() if token is None else token,
        )
    finally:
        pg_conn.close()


def daemon_sweep_once(
    *,
    per_run_timeout_seconds: float = DEFAULT_SWEEP_TIMEOUT_SECONDS,
    require_client_auth: bool = False,
    pg_conn: Any | None = None,
) -> dict[str, Any]:
    owns_pg_conn = pg_conn is None
    if pg_conn is None:
        pg_conn = _connect_pg()
    try:
        return _daemon_sweep_once_pg(
            pg_conn,
            per_run_timeout_seconds=per_run_timeout_seconds,
            require_client_auth=require_client_auth,
        )
    finally:
        if owns_pg_conn and pg_conn is not None:
            pg_conn.close()


def _daemon_sweep_once_pg(
    pg_conn: Any,
    *,
    per_run_timeout_seconds: float,
    require_client_auth: bool,
) -> dict[str, Any]:
    from psycopg.types.json import Jsonb

    from striatum.daemon_pg.handlers.context import RepoHandlerContext
    from striatum.daemon_pg.handlers.recovery_evidence.sweep import handle as sweep_handle
    from striatum.daemon_rpc.capability import RpcAuthContext
    from striatum.daemon_rpc.request_log import append_audit_row

    token = read_runtime_token()
    if require_client_auth:
        _require_pg_auth(
            pg_conn,
            command="daemon.sweep",
            required=ADMIN_CAPABILITY,
            token=token,
        )
    else:
        append_audit_row(
            pg_conn,
            auth=RpcAuthContext(None, None, None, ADMIN_CAPABILITY, "allowed"),
            method="daemon.sweep",
            transport="daemon-internal",
            request_id=f"daemon_sweep_{uuid.uuid4().hex}",
            params={},
        )
    try:
        pg_conn.commit()
    except Exception:  # noqa: BLE001 - autocommit connections do not need an explicit commit.
        pass

    with _pg_dict_cursor(pg_conn) as cur:
        cur.execute(
            """
            SELECT r.repository_id, r.repo_root, runs.run_id
            FROM striatumd.repositories r
            JOIN striatumd.runs runs
              ON runs.repository_id = r.repository_id
            WHERE r.state = 'active'
              AND runs.state IN ('running', 'paused')
            ORDER BY r.repository_id, runs.created_at, runs.run_id
            """
        )
        rows = [_pg_row_dict(row) for row in cur.fetchall()]

    sweeps: list[dict[str, Any]] = []
    for row in rows:
        repository_id = str(row["repository_id"])
        run_id = str(row["run_id"])
        started = time.monotonic()
        try:
            ctx = RepoHandlerContext(
                pg_conn=pg_conn,
                repository_id=repository_id,
                repo_root=Path(str(row["repo_root"])),
                auth=RpcAuthContext(None, None, repository_id, ADMIN_CAPABILITY, "allowed"),
            )
            result = sweep_handle(ctx, {"run_id": run_id})
            elapsed = time.monotonic() - started
            state = "active"
            if elapsed > per_run_timeout_seconds:
                state = "sweep_degraded"
                result = {
                    **result,
                    "degraded": True,
                    "degraded_reason": "sweep_timeout",
                    "elapsed_seconds": elapsed,
                }
            _upsert_pg_scheduler_cursor(
                pg_conn,
                repository_id=repository_id,
                run_id=run_id,
                result=result,
                state=state,
                jsonb=Jsonb,
            )
            sweeps.append({"repository_id": repository_id, "run_id": run_id, "result": result})
        except Exception as exc:  # noqa: BLE001
            try:
                pg_conn.rollback()
            except Exception:  # noqa: BLE001
                pass
            result = {"error": str(exc)}
            _upsert_pg_scheduler_cursor(
                pg_conn,
                repository_id=repository_id,
                run_id=run_id,
                result=result,
                state="sweep_degraded",
                jsonb=Jsonb,
            )
            sweeps.append({"repository_id": repository_id, "run_id": run_id, "error": str(exc)})
    return {"mode": "daemon", "sweeps": sweeps}


def _upsert_pg_scheduler_cursor(
    pg_conn: Any,
    *,
    repository_id: str,
    run_id: str,
    result: Mapping[str, Any],
    state: str,
    jsonb: Any,
) -> None:
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.scheduler_cursors(repository_id, run_id,
              cursor_kind, last_sweep_at, next_sweep_after, last_result_json, state)
            VALUES (%s, %s, 'recovery', now(), NULL, %s, %s)
            ON CONFLICT (repository_id, run_id, cursor_kind)
            DO UPDATE SET last_sweep_at = now(),
                          last_result_json = EXCLUDED.last_result_json,
                          state = EXCLUDED.state
            """,
            (repository_id, run_id, jsonb(dict(result)), state),
        )
    try:
        pg_conn.commit()
    except Exception:  # noqa: BLE001 - autocommit connections do not need an explicit commit.
        pass


def daemon_audit(limit: int = 100) -> dict[str, Any]:
    pg_conn = _connect_pg()
    try:
        _bootstrap_pg_admin_if_needed(pg_conn)
        try:
            pg_conn.commit()
        except Exception:  # noqa: BLE001 - autocommit connections do not need an explicit commit.
            pass
        return daemon_audit_pg(pg_conn, limit=limit, token=read_runtime_token())
    finally:
        pg_conn.close()


def daemon_audit_pg(pg_conn: Any, *, limit: int = 100, token: str | None = None) -> dict[str, Any]:
    token = read_runtime_token() if token is None else token
    _require_pg_auth(
        pg_conn,
        command="daemon.audit",
        required=ADMIN_CAPABILITY,
        token=token,
        payload={"limit": limit},
    )
    with _pg_dict_cursor(pg_conn) as cur:
        cur.execute(
            """
            SELECT audit_id,
                   ts AS timestamp,
                   client_id,
                   repository_id,
                   method AS command,
                   decision AS authorization_result,
                   denial_reason,
                   transport,
                   request_id,
                   exit_code,
                   params_sha256 AS payload_sha256,
                   previous_hash,
                   row_hash,
                   segment_id,
                   schema_version,
                   hash_format_version,
                   daemon_version
            FROM striatumd.audit_log
            ORDER BY audit_id DESC
            LIMIT %s
            """,
            (limit,),
        )
        rows = cur.fetchall()
    return {"mode": "daemon", "audit": [_pg_json_ready(_pg_row_dict(row)) for row in rows]}


def health() -> dict[str, Any]:
    pg_conn = _connect_pg()
    try:
        return health_pg(pg_conn)
    finally:
        pg_conn.close()


def health_pg(pg_conn: Any) -> dict[str, Any]:
    from striatum.daemon_rpc.capability import RpcAuthContext
    from striatum.daemon_rpc.request_log import append_audit_row

    append_audit_row(
        pg_conn,
        auth=RpcAuthContext(None, None, None, None, "allowed"),
        method="health",
        transport="cli",
        request_id=f"cli_{uuid.uuid4().hex}",
        params={},
    )
    try:
        pg_conn.commit()
    except Exception:  # noqa: BLE001 - autocommit connections do not need an explicit commit.
        pass
    return {"mode": "daemon", "ok": True, "protocol_version": PROTOCOL_VERSION}


def _pg_dict_cursor(conn: Any) -> Any:
    try:
        from psycopg.rows import dict_row
    except ImportError:
        return conn.cursor()
    return conn.cursor(row_factory=dict_row)


def _pg_row_dict(row: Any) -> dict[str, Any]:
    if isinstance(row, dict):
        return dict(row)
    if hasattr(row, "keys"):
        return {str(key): row[key] for key in row.keys()}
    raise TypeError("database row must expose mapping-like keys")


def _pg_json_ready(row: dict[str, Any]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in row.items():
        if isinstance(value, datetime):
            result[key] = value.astimezone(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")
        else:
            result[key] = value
    return result
