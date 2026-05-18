"""RFC 0028 V1 local daemon registry and read-control helpers."""

from __future__ import annotations

import hashlib
import hmac
import os
import secrets
import signal
import socket
import sqlite3
import sys
import time
import uuid
from contextlib import contextmanager
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any, BinaryIO, Iterator, Mapping, Sequence, cast

from striatum.bootstrap import (
    init_operational_scratch as _bootstrap_init_operational_scratch,
    require_operational_scratch as _bootstrap_require_operational_scratch,
)
from striatum.cli.introspect import doctor as repo_doctor
from striatum.db import (
    connect as connect_repo,
    ensure_initialized,
    init_repo,
    insert_event,
    transaction,
)
from striatum.daemon_rpc.token_hash import hash_token
from striatum.daemon_runtime import (
    ENV_RUNTIME as _ENV_RUNTIME,
    DaemonRuntimeTokenError,
    ensure_private_dir as _ensure_private_dir,
    pid_path as _runtime_pid_path,
    read_runtime_token as _runtime_read_runtime_token,
    runtime_dir as _runtime_dir,
    socket_path as _runtime_socket_path,
    token_file as _runtime_token_file,
    write_runtime_token as _runtime_write_runtime_token,
)
from striatum.errors import (
    EXIT_DAEMON_AUTH,
    EXIT_DAEMON_CAPABILITY,
    EXIT_DAEMON_REGISTRY,
    EXIT_RPC_VERSION_INCOMPATIBLE,
    NotFoundError,
    SchemaVersionError,
    StriatumError,
)
from striatum.primitives import json_dumps, utc_now
from striatum.recovery.auto import run_auto_sweep
from striatum.repo_policy import db_path

REGISTRY_VERSION = 1
PROTOCOL_VERSION = 1
READ_CAPABILITY = "read"
ADMIN_CAPABILITY = "admin"
SUPPORTED_CAPABILITIES = {READ_CAPABILITY, ADMIN_CAPABILITY}
DEFAULT_SWEEP_TIMEOUT_SECONDS = 30.0

ENV_REGISTRY = "STRIATUM_DAEMON_REGISTRY"
ENV_RUNTIME = _ENV_RUNTIME
ENV_ALLOW_LEGACY_SQLITE_REGISTRY = "STRIATUM_ALLOW_LEGACY_SQLITE_REGISTRY"
ENV_SQLITE_CONNECT_TRIPWIRE = "STRIATUM_SQLITE_CONNECT_TRIPWIRE"


class DaemonUnreachableError(StriatumError):
    """V1 RFC 0028 registry-unreachable; predates the RFC 0043 socket error.

    Kept under the legacy ``daemon`` module name so RFC 0028 callers and
    tests keep working. The RFC 0043 §3 ``daemon_unreachable`` (exit 11)
    is :class:`striatum.errors.DaemonUnreachableError` and is raised at
    the CLI entry layer before any RFC 0028 path runs.
    """

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


@dataclass(frozen=True)
class AuthContext:
    client_id: str | None
    token_id: str | None
    capability: str | None
    repository_id: int | None
    authorization_result: str
    denial_reason: str | None = None


def registry_path() -> Path:
    override = os.environ.get(ENV_REGISTRY)
    if override:
        return Path(override).expanduser().resolve()
    if sys.platform == "darwin":
        root = Path.home() / "Library" / "Application Support" / "striatum"
    else:
        root = Path(os.environ.get("XDG_STATE_HOME", Path.home() / ".local" / "state")) / "striatum"
    return root / "striatumd.sqlite3"


def runtime_dir() -> Path:
    return _runtime_dir()


def token_file() -> Path:
    return _runtime_token_file()


def socket_path() -> Path:
    return _runtime_socket_path()


def pid_path() -> Path:
    return _runtime_pid_path()


def connect_registry() -> sqlite3.Connection:
    _require_legacy_sqlite_registry_allowed()
    if os.environ.get(ENV_SQLITE_CONNECT_TRIPWIRE) == "1":
        raise RuntimeError("sqlite_connect_tripwire: striatum.daemon.connect_registry() was called")
    path = registry_path()
    _ensure_private_dir(path.parent)
    existed = path.exists()
    conn = sqlite3.connect(path)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    conn.execute("PRAGMA journal_mode = WAL")
    conn.execute("PRAGMA busy_timeout = 5000")
    version = int(conn.execute("PRAGMA user_version").fetchone()[0])
    if version > REGISTRY_VERSION:
        conn.close()
        raise SchemaVersionError(
            f"daemon registry schema version {version} is newer than supported {REGISTRY_VERSION}"
        )
    if version < REGISTRY_VERSION:
        conn.execute("BEGIN IMMEDIATE")
        try:
            if version == 0:
                _apply_registry_v1(conn)
            conn.execute(f"PRAGMA user_version = {REGISTRY_VERSION}")
        except Exception:
            conn.rollback()
            conn.close()
            raise
        else:
            conn.commit()
    if not existed:
        try:
            os.chmod(path, 0o600)
        except PermissionError:
            pass
    cutover = conn.execute(
        "SELECT value FROM daemon_meta WHERE key = 'pg_cutover_completed_at'"
    ).fetchone()
    if cutover is not None:
        conn.close()
        raise DaemonRegistryError(
            "daemon SQLite registry has been cut over to PostgreSQL; configure STRIATUM_DAEMON_DB_URL or pass --postgres-url"
        )
    return conn


def _legacy_sqlite_registry_allowed() -> bool:
    return (
        os.environ.get("STRIATUM_TEST_HARNESS") == "1"
        and os.environ.get("STRIATUM_DAEMON_REQUIRED") == "0"
    )


def _require_legacy_sqlite_registry_allowed() -> None:
    if _legacy_sqlite_registry_allowed():
        return
    from striatum.daemon_pg.config import resolve_config

    cfg = resolve_config()
    if cfg.url is None:
        raise DaemonRegistryError(
            "legacy SQLite daemon registry is disabled in production; "
            "configure STRIATUM_DAEMON_DB_URL or pass --postgres-url"
        )
    raise DaemonRegistryError(
        "legacy SQLite daemon registry is disabled in production; "
        "use the configured PostgreSQL daemon registry"
    )


def _apply_registry_v1(conn: sqlite3.Connection) -> None:
    conn.executescript(
        """
        CREATE TABLE daemon_meta (
          key TEXT PRIMARY KEY,
          value TEXT NOT NULL
        );
        CREATE TABLE repositories (
          repository_id INTEGER PRIMARY KEY AUTOINCREMENT,
          repo_identity TEXT NOT NULL,
          repo_root TEXT NOT NULL,
          state_db_path TEXT NOT NULL,
          display_name TEXT NOT NULL,
          registered_at TEXT NOT NULL,
          removed_at TEXT,
          last_seen_at TEXT,
          last_schema_version INTEGER NOT NULL,
          state TEXT NOT NULL CHECK (state IN ('active','missing','disabled','removed')),
          settings_json TEXT NOT NULL
        );
        CREATE UNIQUE INDEX uq_active_daemon_repo_identity
          ON repositories(repo_identity)
          WHERE state IN ('active','missing','disabled');
        CREATE UNIQUE INDEX uq_active_daemon_repo_root
          ON repositories(repo_root)
          WHERE state IN ('active','missing','disabled');
        CREATE TABLE clients (
          client_id TEXT PRIMARY KEY,
          client_kind TEXT NOT NULL,
          display_name TEXT NOT NULL,
          token_id TEXT NOT NULL UNIQUE,
          token_hash TEXT NOT NULL,
          token_salt TEXT NOT NULL,
          created_at TEXT NOT NULL,
          expires_at TEXT,
          revoked_at TEXT,
          last_used_at TEXT
        );
        CREATE TABLE client_capabilities (
          capability_id TEXT PRIMARY KEY,
          client_id TEXT NOT NULL REFERENCES clients(client_id),
          repository_id INTEGER REFERENCES repositories(repository_id),
          capability TEXT NOT NULL CHECK (capability IN ('read','admin')),
          granted_at TEXT NOT NULL,
          expires_at TEXT,
          revoked_at TEXT,
          revoked_reason TEXT
        );
        CREATE TABLE audit_log (
          audit_id INTEGER PRIMARY KEY AUTOINCREMENT,
          timestamp TEXT NOT NULL,
          client_id TEXT,
          repository_id INTEGER,
          command TEXT NOT NULL,
          authorization_result TEXT NOT NULL,
          denial_reason TEXT,
          transport TEXT NOT NULL,
          request_id TEXT,
          exit_code INTEGER,
          payload_sha256 TEXT NOT NULL,
          previous_hash TEXT,
          row_hash TEXT NOT NULL,
          segment_id INTEGER NOT NULL
        );
        CREATE TABLE audit_segments (
          segment_id INTEGER PRIMARY KEY AUTOINCREMENT,
          opened_at TEXT NOT NULL,
          closed_at TEXT,
          first_audit_id INTEGER,
          last_audit_id INTEGER,
          first_hash TEXT,
          last_hash TEXT,
          previous_segment_id INTEGER,
          previous_segment_last_hash TEXT,
          next_segment_first_previous_hash TEXT,
          retention_state TEXT NOT NULL DEFAULT 'active',
          state TEXT NOT NULL CHECK (state IN ('open','closed','purged'))
        );
        CREATE TABLE scheduler_cursors (
          repository_id INTEGER NOT NULL REFERENCES repositories(repository_id),
          run_id TEXT NOT NULL,
          cursor_kind TEXT NOT NULL,
          last_sweep_at TEXT,
          next_sweep_after TEXT,
          last_result_json TEXT,
          state TEXT NOT NULL CHECK (state IN ('active','in_progress','sweep_degraded','removed')),
          PRIMARY KEY(repository_id, run_id, cursor_kind)
        );
        CREATE TRIGGER audit_no_update BEFORE UPDATE ON audit_log BEGIN
          SELECT RAISE(ABORT, 'daemon audit rows are append-only');
        END;
        CREATE TRIGGER audit_no_delete BEFORE DELETE ON audit_log BEGIN
          SELECT RAISE(ABORT, 'daemon audit rows are append-only');
        END;
        CREATE TRIGGER audit_segments_closed_no_update
        BEFORE UPDATE ON audit_segments
        WHEN OLD.state != 'open'
        BEGIN
          SELECT RAISE(ABORT, 'closed daemon audit segments are append-only');
        END;
        CREATE TRIGGER audit_segments_no_delete BEFORE DELETE ON audit_segments BEGIN
          SELECT RAISE(ABORT, 'daemon audit segments are append-only');
        END;
        """
    )
    instance_id = uuid.uuid4().hex[:12]
    conn.execute(
        "INSERT INTO daemon_meta(key, value) VALUES('instance_id', ?)",
        (instance_id,),
    )
    conn.execute(
        """
        INSERT INTO audit_segments(opened_at, state, retention_state)
        VALUES (?, 'open', 'active')
        """,
        (utc_now(),),
    )


@contextmanager
def registry_transaction(conn: sqlite3.Connection) -> Iterator[sqlite3.Connection]:
    conn.execute("BEGIN IMMEDIATE")
    try:
        yield conn
    except Exception:
        conn.rollback()
        raise
    else:
        conn.commit()


def _now_dt() -> datetime:
    return datetime.now(UTC).replace(microsecond=0)


def _parse_time(value: str | None) -> datetime | None:
    if not value:
        return None
    text = value[:-1] + "+00:00" if value.endswith("Z") else value
    try:
        return datetime.fromisoformat(text)
    except ValueError:
        return None


def _canonical_repo(path: Path) -> Path:
    raw = Path(path).expanduser()
    if ".." in raw.parts:
        raise DaemonCapabilityError("repo path traversal is not allowed")
    lexical = raw if raw.is_absolute() else Path.cwd() / raw
    if _has_symlink_component(lexical):
        raise DaemonCapabilityError("repo registration refuses symlink paths")
    resolved = raw.resolve(strict=True)
    state = resolved / ".striatum" / "state.sqlite3"
    if _has_symlink_component(state.parent) or state.is_symlink():
        raise DaemonCapabilityError("repo state database symlink is not allowed")
    return resolved


def _has_symlink_component(path: Path) -> bool:
    current = Path(path.anchor) if path.is_absolute() else Path()
    for part in path.parts:
        if part == path.anchor:
            continue
        current = current / part
        try:
            if current.is_symlink():
                return True
        except OSError:
            return True
    return False


def _repo_identity(repo: Path) -> str:
    repo_stat = repo.stat()
    state_stat = db_path(repo).stat()
    return (
        f"inode:{repo_stat.st_dev}:{repo_stat.st_ino}:"
        f"state:{state_stat.st_dev}:{state_stat.st_ino}"
    )


def _repo_user_version(repo_conn: sqlite3.Connection) -> int:
    return int(repo_conn.execute("PRAGMA user_version").fetchone()[0])


def _would_repo_migrate(repo: Path) -> bool:
    from striatum.migrations import LATEST_VERSION

    conn = sqlite3.connect(db_path(repo))
    try:
        version = int(conn.execute("PRAGMA user_version").fetchone()[0])
    finally:
        conn.close()
    return version < LATEST_VERSION


def _pg_connection_configured() -> bool:
    from striatum.daemon_pg.config import resolve_config

    return resolve_config().url is not None


def _pg_dict_cursor(conn: Any) -> Any:
    try:
        from psycopg.rows import dict_row
    except ImportError:
        return conn.cursor()
    return conn.cursor(row_factory=dict_row)


def _pg_repo_identity(repo: Path) -> str:
    repo_stat = repo.stat()
    return f"inode:{repo_stat.st_dev}:{repo_stat.st_ino}:root:{repo.resolve()}"


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


def _init_operational_scratch(repo: Path) -> Path:
    return _bootstrap_init_operational_scratch(
        repo,
        error_factory=DaemonCapabilityError,
    )


def _require_operational_scratch(repo: Path) -> Path:
    return _bootstrap_require_operational_scratch(
        repo,
        error_factory=DaemonCapabilityError,
    )


def _bootstrap_admin_if_needed(
    conn: sqlite3.Connection,
    *,
    token: str | None = None,
) -> dict[str, str] | None:
    row = conn.execute("SELECT COUNT(*) AS c FROM clients").fetchone()
    if row is not None and int(row["c"]) > 0:
        return None
    if token is not None:
        token_id, sep, secret = token.partition(".")
        if not sep or not token_id or not secret:
            raise DaemonAuthError("daemon runtime token is malformed")
        salt = secrets.token_hex(16)
        token_hash = _hash_token(secret=secret, salt=salt)
        client_id = f"dclient_{uuid.uuid4().hex}"
        now = utc_now()
        conn.execute(
            """
            INSERT INTO clients(client_id, client_kind, display_name, token_id,
              token_hash, token_salt, created_at, expires_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
            """,
            (
                client_id,
                "cli",
                "bootstrap-admin",
                token_id,
                token_hash,
                salt,
                now,
            ),
        )
        for capability in (ADMIN_CAPABILITY, READ_CAPABILITY):
            conn.execute(
                """
                INSERT INTO client_capabilities(capability_id, client_id,
                  repository_id, capability, granted_at, expires_at)
                VALUES (?, ?, NULL, ?, ?, NULL)
                """,
                (f"dcap_{uuid.uuid4().hex}", client_id, capability, now),
            )
        write_runtime_token(token)
        return {"client_id": client_id, "token_id": token_id, "token": token}
    return create_client(
        conn,
        display_name="bootstrap-admin",
        client_kind="cli",
        capabilities=[ADMIN_CAPABILITY, READ_CAPABILITY],
        repository_id=None,
        persist_runtime_token=True,
    )


def _bootstrap_pg_admin_if_needed(pg_conn: Any) -> dict[str, str] | None:
    """RFC 0048 Phase C: ensure striatumd.clients has an admin row + write
    the runtime token file so CLI verbs can authenticate over RPC.

    The Postgres-side client table is distinct from the daemon's SQLite
    registry (`clients`). Phase C makes the CLI talk to the daemon over
    Unix socket; authorization uses Postgres-stored clients.
    """
    with pg_conn.cursor() as cur:
        cur.execute("SELECT COUNT(*) AS c FROM striatumd.clients")
        row = cur.fetchone()
    if row is not None:
        # row may be a tuple, dict, or a psycopg row mapping depending on
        # the connection's row_factory. Handle both shapes.
        count = row["c"] if isinstance(row, dict) else row[0]
        if int(count) > 0:
            return None
    token_id = f"dtok_{secrets.token_urlsafe(12)}"
    secret = secrets.token_urlsafe(32)
    salt = secrets.token_hex(16)
    token_hash = _hash_token(secret=secret, salt=salt)
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


def create_client(
    conn: sqlite3.Connection,
    *,
    display_name: str,
    client_kind: str,
    capabilities: Sequence[str],
    repository_id: int | None,
    expires_at: str | None = None,
    persist_runtime_token: bool = False,
) -> dict[str, str]:
    for capability in capabilities:
        if capability not in SUPPORTED_CAPABILITIES:
            raise DaemonCapabilityError(f"unsupported daemon capability {capability!r}")
    token_id = f"dtok_{secrets.token_urlsafe(12)}"
    secret = secrets.token_urlsafe(32)
    salt = secrets.token_hex(16)
    token_hash = _hash_token(secret=secret, salt=salt)
    client_id = f"dclient_{uuid.uuid4().hex}"
    now = utc_now()
    conn.execute(
        """
        INSERT INTO clients(client_id, client_kind, display_name, token_id,
          token_hash, token_salt, created_at, expires_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        """,
        (client_id, client_kind, display_name, token_id, token_hash, salt, now, expires_at),
    )
    for capability in capabilities:
        conn.execute(
            """
            INSERT INTO client_capabilities(capability_id, client_id,
              repository_id, capability, granted_at, expires_at)
            VALUES (?, ?, ?, ?, ?, ?)
            """,
            (f"dcap_{uuid.uuid4().hex}", client_id, repository_id, capability, now, expires_at),
        )
    token = f"{token_id}.{secret}"
    if persist_runtime_token:
        write_runtime_token(token)
    return {"client_id": client_id, "token_id": token_id, "token": token}


def _hash_token(*, secret: str, salt: str) -> str:
    return hash_token(secret=secret, salt=salt)


def read_runtime_token() -> str | None:
    try:
        return _runtime_read_runtime_token()
    except DaemonRuntimeTokenError as exc:
        raise DaemonAuthError(str(exc)) from exc


def write_runtime_token(token: str) -> None:
    _runtime_write_runtime_token(token)


def _authorize(
    conn: sqlite3.Connection,
    *,
    required: str,
    repository_id: int | None,
    token: str | None,
) -> AuthContext:
    if token is None:
        return AuthContext(None, None, None, repository_id, "denied", "token_missing")
    token_id, sep, secret = token.partition(".")
    if not sep or not token_id or not secret:
        return AuthContext(None, token_id or None, None, repository_id, "denied", "token_malformed")
    row = conn.execute(
        "SELECT * FROM clients WHERE token_id = ?",
        (token_id,),
    ).fetchone()
    if row is None:
        return AuthContext(None, token_id, None, repository_id, "denied", "token_invalid")
    expected = _hash_token(secret=secret, salt=str(row["token_salt"]))
    if not hmac.compare_digest(expected, str(row["token_hash"])):
        return AuthContext(str(row["client_id"]), token_id, None, repository_id, "denied", "token_invalid")
    if row["revoked_at"] is not None:
        return AuthContext(str(row["client_id"]), token_id, None, repository_id, "denied", "token_revoked")
    expires = _parse_time(row["expires_at"])
    if expires is not None and expires <= _now_dt():
        return AuthContext(str(row["client_id"]), token_id, None, repository_id, "denied", "token_expired")
    cap = conn.execute(
        """
        SELECT * FROM client_capabilities
        WHERE client_id = ? AND capability = ?
          AND (repository_id IS NULL OR repository_id = ?)
          AND revoked_at IS NULL
        ORDER BY repository_id IS NULL
        LIMIT 1
        """,
        (row["client_id"], required, repository_id),
    ).fetchone()
    if cap is None:
        return AuthContext(str(row["client_id"]), token_id, None, repository_id, "denied", "capability_denied")
    cap_expires = _parse_time(cap["expires_at"])
    if cap_expires is not None and cap_expires <= _now_dt():
        return AuthContext(str(row["client_id"]), token_id, None, repository_id, "denied", "capability_expired")
    conn.execute(
        "UPDATE clients SET last_used_at = ? WHERE client_id = ?",
        (utc_now(), row["client_id"]),
    )
    return AuthContext(str(row["client_id"]), token_id, required, repository_id, "allowed")


def audit_request(
    conn: sqlite3.Connection,
    *,
    command: str,
    auth: AuthContext,
    transport: str,
    payload: Mapping[str, Any] | None = None,
    request_id: str | None = None,
    exit_code: int | None = None,
) -> None:
    segment = conn.execute(
        "SELECT * FROM audit_segments WHERE state = 'open' ORDER BY segment_id DESC LIMIT 1"
    ).fetchone()
    if segment is None:
        conn.execute(
            "INSERT INTO audit_segments(opened_at, state, retention_state) VALUES(?, 'open', 'active')",
            (utc_now(),),
        )
        segment_id = int(conn.execute("SELECT last_insert_rowid()").fetchone()[0])
    else:
        segment_id = int(segment["segment_id"])
    previous = conn.execute(
        "SELECT row_hash FROM audit_log ORDER BY audit_id DESC LIMIT 1"
    ).fetchone()
    previous_hash = str(previous["row_hash"]) if previous is not None else None
    timestamp = utc_now()
    payload_sha = hashlib.sha256(json_dumps(payload or {}).encode("utf-8")).hexdigest()
    row_material = {
        "timestamp": timestamp,
        "client_id": auth.client_id,
        "repository_id": auth.repository_id,
        "command": command,
        "authorization_result": auth.authorization_result,
        "denial_reason": auth.denial_reason,
        "transport": transport,
        "request_id": request_id,
        "exit_code": exit_code,
        "payload_sha256": payload_sha,
        "previous_hash": previous_hash,
        "segment_id": segment_id,
    }
    row_hash = hashlib.sha256(json_dumps(row_material).encode("utf-8")).hexdigest()
    cur = conn.execute(
        """
        INSERT INTO audit_log(timestamp, client_id, repository_id, command,
          authorization_result, denial_reason, transport, request_id,
          exit_code, payload_sha256, previous_hash, row_hash, segment_id)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """,
        (
            timestamp,
            auth.client_id,
            auth.repository_id,
            command,
            auth.authorization_result,
            auth.denial_reason,
            transport,
            request_id,
            exit_code,
            payload_sha,
            previous_hash,
            row_hash,
            segment_id,
        ),
    )
    audit_id = int(cur.lastrowid or 0)
    conn.execute(
        """
        UPDATE audit_segments
        SET first_audit_id = COALESCE(first_audit_id, ?),
            last_audit_id = ?,
            first_hash = COALESCE(first_hash, ?),
            last_hash = ?
        WHERE segment_id = ?
        """,
        (audit_id, audit_id, row_hash, row_hash, segment_id),
    )


def _require_auth(
    conn: sqlite3.Connection,
    *,
    command: str,
    required: str,
    repository_id: int | None = None,
    token: str | None = None,
    payload: Mapping[str, Any] | None = None,
    transport: str = "cli",
) -> AuthContext:
    auth = _authorize(conn, required=required, repository_id=repository_id, token=token)
    audit_request(conn, command=command, auth=auth, transport=transport, payload=payload)
    if auth.authorization_result != "allowed":
        # Denied requests are audit-significant. Persist the audit row even
        # though the caller's higher-level operation is about to fail.
        try:
            conn.commit()
        except sqlite3.Error:
            pass
        if auth.denial_reason and auth.denial_reason.startswith("token_"):
            raise DaemonAuthError(f"daemon authorization failed: {auth.denial_reason}")
        raise DaemonCapabilityError(f"daemon authorization failed: {auth.denial_reason}")
    return auth


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
    if _pg_connection_configured():
        from striatum.daemon_pg.connection import connect_and_migrate

        pg_conn = connect_and_migrate()
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
    return _repo_add_legacy_sqlite(
        path,
        display_name=display_name,
        no_migrate=no_migrate,
        init=init,
    )


def repo_add_pg(
    pg_conn: Any,
    path: Path,
    *,
    display_name: str | None = None,
    no_migrate: bool = False,
    init: bool = False,
) -> dict[str, Any]:
    from striatum.migrations import LATEST_VERSION

    repo = _canonical_repo(path)
    legacy_state = db_path(repo)
    if legacy_state.exists():
        raise SchemaVersionError(
            "repo-local SQLite state exists and SQLite import windows are closed; "
            "archive or remove .striatum/state.sqlite3 before registering with "
            "`striatum adopt` or `striatum repo add --init`"
        )
    state_dir = _init_operational_scratch(repo) if init else _require_operational_scratch(repo)
    identity = _pg_repo_identity(repo)
    with _pg_dict_cursor(pg_conn) as cur:
        cur.execute(
            """
            SELECT *
            FROM striatumd.repositories
            WHERE repo_identity = %s AND state != 'removed'
            ORDER BY repository_id
            LIMIT 1
            """,
            (identity,),
        )
        existing_identity = cur.fetchone()
        if existing_identity is not None:
            row = _pg_row_dict(existing_identity)
            return {
                "repository_id": str(row["repository_id"]),
                "repo_root": str(row["repo_root"]),
                "repo_identity": str(row["repo_identity"]),
                "state_db_path": str(row["state_db_path"]),
                "schema_version": int(row["last_schema_version"]),
                "state": str(row["state"]),
                "already_registered": True,
            }
        cur.execute(
            """
            SELECT *
            FROM striatumd.repositories
            WHERE repo_root = %s AND state != 'removed'
            ORDER BY repository_id
            LIMIT 1
            """,
            (str(repo),),
        )
        existing_path = cur.fetchone()
        if existing_path is not None:
            raise DaemonCapabilityError("active repository path is occupied by a different repo identity")
        repository_id = f"repo_{uuid.uuid4().hex}"
        now = utc_now()
        cur.execute(
            """
            INSERT INTO striatumd.repositories(repository_id, repo_identity, repo_root,
              state_db_path, display_name, registered_at, removed_at, last_seen_at,
              last_schema_version, state, settings_json)
            VALUES (%s, %s, %s, %s, %s, %s, NULL, %s, %s, 'active', '{}'::jsonb)
            """,
            (
                repository_id,
                identity,
                str(repo),
                str(state_dir.resolve()),
                display_name or repo.name,
                now,
                now,
                LATEST_VERSION,
            ),
        )
    pg_conn.commit()
    return {
        "repository_id": repository_id,
        "repo_root": str(repo),
        "repo_identity": identity,
        "state_db_path": str(state_dir.resolve()),
        "schema_version": LATEST_VERSION,
        "state": "active",
    }


def _repo_add_legacy_sqlite(
    path: Path,
    *,
    display_name: str | None = None,
    no_migrate: bool = False,
    init: bool = False,
) -> dict[str, Any]:
    conn = connect_registry()
    with conn:
        bootstrap = _bootstrap_admin_if_needed(conn)
    token = read_runtime_token()
    repo = _canonical_repo(path)
    with registry_transaction(conn):
        auth = _require_auth(
            conn,
            command="repo.add",
            required=ADMIN_CAPABILITY,
            token=token,
            payload={"path": str(repo), "no_migrate": no_migrate, "init": init},
        )
        _ = auth
    if not db_path(repo).exists():
        if not init:
            raise DaemonCapabilityError("repo is not initialized; rerun repo add with --init or run striatum init first")
        init_repo(repo)
    if no_migrate and _would_repo_migrate(repo):
        raise SchemaVersionError(
            "repo-local SQLite state exists and SQLite import windows are closed; "
            "archive or remove .striatum/state.sqlite3 before registering"
        )
    ensure_initialized(repo)
    with registry_transaction(conn):
        with connect_repo(repo) as repo_conn:
            identity = _repo_identity(repo)
            version = _repo_user_version(repo_conn)
        existing_identity = conn.execute(
            "SELECT * FROM repositories WHERE repo_identity = ? AND state != 'removed'",
            (identity,),
        ).fetchone()
        if existing_identity is not None:
            return {"repository_id": existing_identity["repository_id"], "state": existing_identity["state"], "already_registered": True}
        existing_path = conn.execute(
            "SELECT * FROM repositories WHERE repo_root = ? AND state != 'removed'",
            (str(repo),),
        ).fetchone()
        if existing_path is not None:
            raise DaemonCapabilityError("active repository path is occupied by a different repo identity")
        now = utc_now()
        cur = conn.execute(
            """
            INSERT INTO repositories(repo_identity, repo_root, state_db_path,
              display_name, registered_at, last_seen_at, last_schema_version,
              state, settings_json)
            VALUES (?, ?, ?, ?, ?, ?, ?, 'active', '{}')
            """,
            (
                identity,
                str(repo),
                str(db_path(repo).resolve()),
                display_name or repo.name,
                now,
                now,
                version,
            ),
        )
        repository_id = int(cur.lastrowid or 0)
        return {
            "repository_id": repository_id,
            "repo_root": str(repo),
            "repo_identity": identity,
            "state_db_path": str(db_path(repo).resolve()),
            "schema_version": version,
            "state": "active",
            "bootstrap_admin": bootstrap,
        }


def repo_list() -> dict[str, Any]:
    if _pg_connection_configured():
        from striatum.daemon_pg.connection import connect_and_migrate

        pg_conn = connect_and_migrate()
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
    return _repo_list_legacy_sqlite()


def repo_list_pg(pg_conn: Any) -> dict[str, Any]:
    with _pg_dict_cursor(pg_conn) as cur:
        cur.execute(
            """
            SELECT repository_id, repo_identity, repo_root, display_name,
                   registered_at, removed_at, last_seen_at,
                   last_schema_version, state, state_db_path
            FROM striatumd.repositories
            ORDER BY registered_at, repository_id
            """
        )
        rows = cur.fetchall()
    repositories = []
    for row in rows:
        item = _pg_json_ready(_pg_row_dict(row))
        item["repository_id"] = str(item["repository_id"])
        repositories.append(item)
    return {"repositories": repositories}


def repo_resolve_pg(pg_conn: Any, path: Path) -> dict[str, Any]:
    repo = _canonical_repo(path)
    row = _find_repo_pg(pg_conn, str(repo), include_removed=False)
    if row is None:
        raise NotFoundError(f"active repository path is not registered: {repo}")
    item = _pg_json_ready(row)
    return {
        "repository_id": str(item["repository_id"]),
        "repo_root": str(item["repo_root"]),
        "repo_identity": str(item["repo_identity"]),
        "state_db_path": str(item["state_db_path"]),
        "display_name": item.get("display_name"),
        "registered_at": item.get("registered_at"),
        "removed_at": item.get("removed_at"),
        "last_seen_at": item.get("last_seen_at"),
        "schema_version": int(item.get("last_schema_version") or 0),
        "state": str(item["state"]),
    }


def _repo_list_legacy_sqlite() -> dict[str, Any]:
    conn = connect_registry()
    token = read_runtime_token()
    with registry_transaction(conn):
        _require_auth(conn, command="repo.list", required=READ_CAPABILITY, token=token)
        rows = conn.execute(
            """
            SELECT repository_id, repo_identity, repo_root, display_name,
                   registered_at, removed_at, last_seen_at,
                   last_schema_version, state
            FROM repositories
            ORDER BY repository_id
            """
        ).fetchall()
        return {"repositories": [dict(row) for row in rows]}


def _readable_repository_ids(conn: sqlite3.Connection, *, token: str | None) -> tuple[set[int], AuthContext]:
    global_auth = _authorize(conn, required=READ_CAPABILITY, repository_id=None, token=token)
    if global_auth.authorization_result == "allowed":
        rows = conn.execute(
            "SELECT repository_id FROM repositories WHERE state = 'active'"
        ).fetchall()
        return {int(row["repository_id"]) for row in rows}, global_auth
    if global_auth.denial_reason and global_auth.denial_reason.startswith("token_"):
        return set(), global_auth
    if global_auth.client_id is None:
        return set(), global_auth
    rows = conn.execute(
        """
        SELECT repository_id FROM client_capabilities
        WHERE client_id = ? AND capability = 'read'
          AND repository_id IS NOT NULL
          AND revoked_at IS NULL
          AND (expires_at IS NULL OR expires_at > ?)
        """,
        (global_auth.client_id, utc_now()),
    ).fetchall()
    if not rows:
        return set(), global_auth
    conn.execute(
        "UPDATE clients SET last_used_at = ? WHERE client_id = ?",
        (utc_now(), global_auth.client_id),
    )
    return (
        {int(row["repository_id"]) for row in rows},
        AuthContext(global_auth.client_id, global_auth.token_id, READ_CAPABILITY, None, "allowed"),
    )


def _raise_denied(auth: AuthContext) -> None:
    if auth.denial_reason and auth.denial_reason.startswith("token_"):
        raise DaemonAuthError(f"daemon authorization failed: {auth.denial_reason}")
    raise DaemonCapabilityError(f"daemon authorization failed: {auth.denial_reason}")


def repo_remove(identifier: str) -> dict[str, Any]:
    if _pg_connection_configured():
        from striatum.daemon_pg.connection import connect_and_migrate

        pg_conn = connect_and_migrate()
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
    return _repo_remove_legacy_sqlite(identifier)


def repo_remove_pg(pg_conn: Any, identifier: str) -> dict[str, Any]:
    row = _find_repo_pg(pg_conn, identifier, include_removed=True)
    if row is None:
        raise NotFoundError(f"unknown daemon repository {identifier!r}")
    repository_id = str(row["repository_id"])
    if row["state"] == "removed":
        return {"repository_id": repository_id, "state": "removed", "already_removed": True}
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.repositories
            SET state = 'removed', removed_at = now()
            WHERE repository_id = %s
            """,
            (repository_id,),
        )
        cur.execute(
            """
            UPDATE striatumd.client_capabilities
            SET revoked_at = now(), revoked_reason = 'repo_removed'
            WHERE repository_id = %s AND revoked_at IS NULL
            """,
            (repository_id,),
        )
        revoked = int(cur.rowcount or 0)
        cur.execute(
            "UPDATE striatumd.scheduler_cursors SET state = 'removed' WHERE repository_id = %s",
            (repository_id,),
        )
    pg_conn.commit()
    return {"repository_id": repository_id, "state": "removed", "revoked_capabilities": revoked}


def _find_repo_pg(pg_conn: Any, identifier: str, *, include_removed: bool = False) -> dict[str, Any] | None:
    where = "" if include_removed else "AND state != 'removed'"
    with _pg_dict_cursor(pg_conn) as cur:
        cur.execute(
            f"SELECT * FROM striatumd.repositories WHERE repository_id = %s {where}",
            (identifier,),
        )
        row = cur.fetchone()
        if row is not None:
            return _pg_row_dict(row)
        try:
            resolved = Path(identifier).expanduser().resolve(strict=False)
        except OSError:
            resolved = Path(identifier)
        cur.execute(
            f"SELECT * FROM striatumd.repositories WHERE repo_root = %s {where}",
            (str(resolved),),
        )
        row = cur.fetchone()
    return None if row is None else _pg_row_dict(row)


def _repo_remove_legacy_sqlite(identifier: str) -> dict[str, Any]:
    conn = connect_registry()
    token = read_runtime_token()
    with registry_transaction(conn):
        _require_auth(conn, command="repo.remove", required=ADMIN_CAPABILITY, token=token, payload={"id": identifier})
        row = _find_repo(conn, identifier, include_removed=True)
        if row is None:
            raise NotFoundError(f"unknown daemon repository {identifier!r}")
        if row["state"] == "removed":
            return {"repository_id": row["repository_id"], "state": "removed", "already_removed": True}
        now = utc_now()
        repository_id = int(row["repository_id"])
        conn.execute(
            "UPDATE repositories SET state = 'removed', removed_at = ? WHERE repository_id = ?",
            (now, repository_id),
        )
        caps = conn.execute(
            """
            SELECT capability_id, client_id FROM client_capabilities
            WHERE repository_id = ? AND revoked_at IS NULL
            """,
            (repository_id,),
        ).fetchall()
        revoked = 0
        for cap in caps:
            conn.execute(
                """
                UPDATE client_capabilities
                SET revoked_at = ?, revoked_reason = 'repo_removed'
                WHERE capability_id = ?
                """,
                (now, cap["capability_id"]),
            )
            audit_request(
                conn,
                command="capability.revoked",
                auth=AuthContext(str(cap["client_id"]), None, None, repository_id, "revoked", "repo_removed"),
                transport="cli",
            )
            revoked += 1
        conn.execute(
            "UPDATE scheduler_cursors SET state = 'removed' WHERE repository_id = ?",
            (repository_id,),
        )
        return {"repository_id": repository_id, "state": "removed", "revoked_capabilities": revoked}


def _find_repo(conn: sqlite3.Connection, identifier: str, *, include_removed: bool = False) -> sqlite3.Row | None:
    where = "" if include_removed else "AND state != 'removed'"
    if identifier.isdigit():
        row = conn.execute(
            f"SELECT * FROM repositories WHERE repository_id = ? {where}",
            (int(identifier),),
        ).fetchone()
        if row is not None:
            return cast(sqlite3.Row, row)
    try:
        resolved = Path(identifier).expanduser().resolve(strict=False)
    except OSError:
        resolved = Path(identifier)
    row = conn.execute(
        f"SELECT * FROM repositories WHERE repo_root = ? {where}",
        (str(resolved),),
    ).fetchone()
    return cast(sqlite3.Row | None, row)


def _registered_repo_for_path(conn: sqlite3.Connection, repo: Path) -> sqlite3.Row:
    resolved = repo.resolve()
    row = conn.execute(
        "SELECT * FROM repositories WHERE repo_root = ? AND state = 'active'",
        (str(resolved),),
    ).fetchone()
    if row is None:
        raise DaemonCapabilityError("repository is not registered with daemon")
    return cast(sqlite3.Row, row)


def daemon_status() -> dict[str, Any]:
    if _pg_connection_configured():
        from striatum.daemon_pg.connection import connect_and_migrate

        try:
            pg_conn = connect_and_migrate()
        except StriatumError:
            raise
        except Exception as exc:  # noqa: BLE001 - status should not leak tracebacks.
            raise _daemon_status_pg_unavailable(exc) from exc
        try:
            return daemon_status_pg(pg_conn, token=read_runtime_token())
        finally:
            pg_conn.close()

    conn = connect_registry()
    token = read_runtime_token()
    with registry_transaction(conn):
        _require_auth(conn, command="daemon.status", required=READ_CAPABILITY, token=token)
        pid = _read_pid()
        return {
            "mode": "daemon",
            "protocol_version": PROTOCOL_VERSION,
            "registry_path": str(registry_path()),
            "runtime_dir": str(runtime_dir()),
            "pid": pid,
            "running": _pid_alive(pid) if pid is not None else False,
            "instance_id": _instance_id(conn),
        }


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
        "run `striatum daemon doctor --apply-migrations --repair-grants` "
        "as a database owner/admin, then retry `striatum daemon status`",
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


def _instance_id(conn: sqlite3.Connection) -> str:
    row = conn.execute("SELECT value FROM daemon_meta WHERE key = 'instance_id'").fetchone()
    if row is None:
        value = uuid.uuid4().hex[:12]
        conn.execute("INSERT INTO daemon_meta(key, value) VALUES('instance_id', ?)", (value,))
        return value
    return str(row["value"])


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
    if _pg_connection_configured():
        from striatum.daemon_pg.connection import connect_and_migrate

        pg_conn = connect_and_migrate()
        try:
            _require_pg_auth(
                pg_conn,
                command="daemon.stop",
                required=ADMIN_CAPABILITY,
                token=read_runtime_token(),
            )
        finally:
            pg_conn.close()
        pid = _read_pid()
        if pid is None or not _pid_alive(pid):
            return {"stopped": False, "reason": "not_running"}
        os.kill(pid, signal.SIGTERM)
        return {"stopped": True, "pid": pid}

    conn = connect_registry()
    token = read_runtime_token()
    with registry_transaction(conn):
        _require_auth(conn, command="daemon.stop", required=ADMIN_CAPABILITY, token=token)
    pid = _read_pid()
    if pid is None or not _pid_alive(pid):
        return {"stopped": False, "reason": "not_running"}
    os.kill(pid, signal.SIGTERM)
    return {"stopped": True, "pid": pid}


def run_daemon_foreground(
    *,
    sweep_interval_seconds: float = 60.0,
    max_sweeps: int | None = None,
    postgres_url: str | None = None,
) -> dict[str, Any]:
    import threading
    import uuid

    pg_doctor: dict[str, Any] | None = None
    daemon_pg_conn: Any | None = None
    pg_bootstrap: dict[str, str] | None = None
    # RFC 0048 V1.5: resolve daemon.toml + env + flag in one shot so the
    # daemon picks up the configured PG URL regardless of how it was launched
    # (systemd unit, direct shell, etc.). Without this the systemd-launched
    # daemon stayed pg_conn=None and native PG handlers were unavailable.
    from striatum.daemon_pg.config import resolve_config

    _resolved_cfg = resolve_config(postgres_url=postgres_url)
    if _resolved_cfg.url is not None:
        from striatum.daemon_pg.connection import doctor as daemon_pg_doctor, connect as daemon_pg_connect

        pg_doctor = daemon_pg_doctor(postgres_url=_resolved_cfg.url, apply=True)
        if not pg_doctor.get("ok"):
            raise DaemonRegistryError("daemon PostgreSQL doctor failed; refusing daemon start")
        daemon_pg_conn = daemon_pg_connect(_resolved_cfg.url)
        daemon_pg_conn.autocommit = True
        # RFC 0048 V1.5: row_factory dict_row so authorize()'s _row_dict
        # (which expects mapping-like keys) works against per-cursor results.
        try:
            import psycopg
            daemon_pg_conn.row_factory = psycopg.rows.dict_row
        except Exception:  # noqa: BLE001
            pass
        # RFC 0048 Phase C: bootstrap an admin client into striatumd.clients
        # on first start so RPC verbs can authenticate. Mirrors the SQLite
        # registry's _bootstrap_admin_if_needed but targets Postgres. The
        # token file under runtime_dir() is overwritten so the CLI can read
        # it for capability_token on each request.
        pg_bootstrap = _bootstrap_pg_admin_if_needed(daemon_pg_conn)
    bootstrap: dict[str, str] | None
    if daemon_pg_conn is not None:
        bootstrap = pg_bootstrap
        daemon_instance_id = _pg_instance_id(daemon_pg_conn)
    else:
        conn = connect_registry()
        with conn:
            bootstrap = _bootstrap_admin_if_needed(conn)
            daemon_instance_id = _instance_id(conn)
    _ensure_private_dir(runtime_dir())
    pid = _read_pid()
    if pid is not None and _pid_alive(pid):
        raise DaemonUnreachableError(f"another striatumd is active (pid {pid})")
    if socket_path().exists():
        socket_path().unlink()
    pid_path().write_text(str(os.getpid()), encoding="utf-8")
    try:
        os.chmod(pid_path(), 0o600)
    except PermissionError:
        pass
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.bind(str(socket_path()))
    sock.listen(16)
    sock.settimeout(0.5)
    try:
        os.chmod(socket_path(), 0o600)
    except PermissionError:
        pass

    previous_daemon_instance_id = os.environ.get("STRIATUM_DAEMON_INSTANCE_ID")
    os.environ["STRIATUM_DAEMON_INSTANCE_ID"] = daemon_instance_id

    # RFC 0048 V1.5: accept-loop wiring. One accept thread polls sock.accept()
    # with a short timeout against `stop_event`; per-connection daemon threads
    # iterate NDJSON envelopes through DaemonRpcRouter.handle. Without this
    # loop the bound socket was a probe endpoint only (no accept()), and
    # daemon-required CLI verbs refused with exit 11 even when daemon was up.
    from striatum.daemon_rpc.framing import read_envelopes, write_response
    from striatum.daemon_rpc.server import DaemonRpcRouter

    substrate_schema = int(pg_doctor.get("schema_version", 0)) if pg_doctor else 0
    router = DaemonRpcRouter(
        pg_conn=daemon_pg_conn,
        repo_root=Path.cwd(),
        substrate_schema=substrate_schema,
    )
    stop_event = threading.Event()
    connection_threads: list[threading.Thread] = []

    def _serve_connection(conn_sock: "socket.socket", connection_id: str) -> None:
        import sys
        import traceback
        try:
            stream = cast(BinaryIO, conn_sock.makefile("rwb"))
            for envelope in read_envelopes(stream):
                if stop_event.is_set():
                    break
                response = router.handle(
                    envelope,
                    connection_id=connection_id,
                    transport="unix",
                    require_handshake=True,
                )
                write_response(stream, response)
        except Exception as exc:  # noqa: BLE001 — connection-thread failures must not kill the daemon
            print(f"daemon connection {connection_id[:8]} crashed: {type(exc).__name__}: {exc}", file=sys.stderr)
            traceback.print_exc(file=sys.stderr)
        finally:
            try:
                conn_sock.close()
            except Exception:  # noqa: BLE001
                pass

    def _accept_loop() -> None:
        while not stop_event.is_set():
            try:
                conn_sock, _addr = sock.accept()
            except socket.timeout:
                continue
            except OSError:
                # Listener closed during shutdown.
                break
            connection_id = uuid.uuid4().hex
            t = threading.Thread(
                target=_serve_connection,
                args=(conn_sock, connection_id),
                name=f"striatumd-conn-{connection_id[:8]}",
                daemon=True,
            )
            t.start()
            connection_threads.append(t)

    accept_thread = threading.Thread(target=_accept_loop, name="striatumd-accept", daemon=True)
    accept_thread.start()

    stopping = False

    def _stop(_signum: int, _frame: object) -> None:
        nonlocal stopping
        stopping = True
        stop_event.set()

    old_term = signal.signal(signal.SIGTERM, _stop)
    old_int = signal.signal(signal.SIGINT, _stop)
    sweeps = 0
    try:
        while not stopping:
            daemon_sweep_once(pg_conn=daemon_pg_conn)
            sweeps += 1
            if max_sweeps is not None and sweeps >= max_sweeps:
                stop_event.set()
                break
            # Poll stop_event with shorter intervals so SIGTERM responds quickly.
            stop_event.wait(timeout=max(0.1, sweep_interval_seconds))
    finally:
        signal.signal(signal.SIGTERM, old_term)
        signal.signal(signal.SIGINT, old_int)
        stop_event.set()
        try:
            sock.close()
        except Exception:  # noqa: BLE001
            pass
        accept_thread.join(timeout=2.0)
        for t in connection_threads:
            t.join(timeout=0.5)
        if daemon_pg_conn is not None:
            try:
                daemon_pg_conn.close()
            except Exception:  # noqa: BLE001
                pass
        if previous_daemon_instance_id is None:
            os.environ.pop("STRIATUM_DAEMON_INSTANCE_ID", None)
        else:
            os.environ["STRIATUM_DAEMON_INSTANCE_ID"] = previous_daemon_instance_id
        for path in (socket_path(), pid_path()):
            try:
                path.unlink()
            except FileNotFoundError:
                pass
    return {
        "mode": "daemon",
        "started": True,
        "sweeps": sweeps,
        "registry_path": str(registry_path()),
        "socket_path": str(socket_path()),
        "bootstrap_admin": bootstrap,
        "instance_id": daemon_instance_id,
        "postgres": pg_doctor,
        "rpc_accept_loop": "running",
    }


def read_doctor(
    repo: Path | None = None,
    *,
    run_id: str | None = None,
    verbose: bool = False,
    token: str | None = None,
    postgres_url: str | None = None,
) -> dict[str, Any]:
    if postgres_url is not None or _pg_connection_configured():
        from striatum.daemon_pg.connection import connect_and_migrate

        pg_conn = connect_and_migrate(postgres_url=postgres_url)
        try:
            _bootstrap_pg_admin_if_needed(pg_conn)
            try:
                pg_conn.commit()
            except Exception:  # noqa: BLE001 - autocommit connections do not need an explicit commit.
                pass
            return read_doctor_pg(pg_conn, repo=repo, run_id=run_id, verbose=verbose, token=token)
        finally:
            pg_conn.close()

    conn = connect_registry()
    token = read_runtime_token() if token is None else token
    with registry_transaction(conn):
        repository_id: int | None = None
        if repo is not None:
            repo_row = _registered_repo_for_path(conn, repo)
            repository_id = int(repo_row["repository_id"])
        _require_auth(conn, command="doctor", required=READ_CAPABILITY, repository_id=repository_id, token=token, payload={"run_id": run_id})
        problems = daemon_doctor_records(conn)
    if repo is None:
        data: dict[str, Any] = {"problems": [p["message"] for p in problems]}
        if verbose:
            data["problem_records"] = problems
        return {"mode": "daemon", "protocol_version": PROTOCOL_VERSION, **data}
    with connect_repo(repo) as repo_conn:
        data = repo_doctor(repo_conn, repo=repo, run_id=run_id, verbose=verbose)
    data["daemon_problems"] = [p["message"] for p in problems]
    if verbose:
        data["daemon_problem_records"] = problems
    return {"mode": "daemon", "repository_id": repository_id, "protocol_version": PROTOCOL_VERSION, **data}


def read_doctor_pg(
    pg_conn: Any,
    *,
    repo: Path | None = None,
    run_id: str | None = None,
    verbose: bool = False,
    token: str | None = None,
) -> dict[str, Any]:
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
        return {"mode": "daemon", "protocol_version": PROTOCOL_VERSION, **data}

    resolved = _canonical_repo(repo)
    repo_row = _find_repo_pg(pg_conn, str(resolved))
    if repo_row is None:
        raise DaemonCapabilityError("repository is not registered with daemon")
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


def daemon_doctor_records(conn: sqlite3.Connection) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    reg = registry_path()
    if reg.exists() and reg.stat().st_mode & 0o077:
        records.append({"check": "daemon_registry_permissions", "id": str(reg), "message": "daemon registry is not owner-only", "context": {"mode": oct(reg.stat().st_mode & 0o777)}})
    rt = runtime_dir()
    if rt.exists() and rt.stat().st_mode & 0o077:
        records.append({"check": "daemon_runtime_permissions", "id": str(rt), "message": "daemon runtime directory is not owner-only", "context": {"mode": oct(rt.stat().st_mode & 0o777)}})
    if token_file().exists() and token_file().stat().st_mode & 0o077:
        records.append({"check": "daemon_token_permissions", "id": str(token_file()), "message": "daemon token fallback file is not owner-only", "context": {"mode": oct(token_file().stat().st_mode & 0o777)}})
    records.extend(_audit_chain_records(conn))
    missing = conn.execute(
        "SELECT repository_id, repo_root, state_db_path FROM repositories WHERE state = 'active'"
    ).fetchall()
    for row in missing:
        if not Path(str(row["state_db_path"])).exists():
            records.append({"check": "daemon_repo_state_missing", "id": str(row["repository_id"]), "message": "registered repository state database is missing", "context": {"repo_root": row["repo_root"]}})
    removed_refs = conn.execute(
        """
        SELECT COUNT(*) AS count FROM audit_log a
        JOIN repositories r ON r.repository_id = a.repository_id
        WHERE r.state = 'removed'
        """
    ).fetchone()
    if removed_refs is not None and int(removed_refs["count"]) > 0:
        records.append({"check": "daemon_removed_repo_audit_refs", "id": "audit_log", "message": "removed repository ids remain in retained audit rows", "context": {"count": int(removed_refs["count"])}})
    degraded = conn.execute(
        """
        SELECT repository_id, run_id, last_result_json
        FROM scheduler_cursors
        WHERE state = 'sweep_degraded'
        ORDER BY repository_id, run_id
        """
    ).fetchall()
    for row in degraded:
        records.append({"check": "daemon_sweep_degraded", "id": f"{row['repository_id']}:{row['run_id']}", "message": "daemon recovery sweep is degraded", "context": {"last_result_json": row["last_result_json"]}})
    for row in missing:
        try:
            from striatum.recovery.watch import pidfile_path, _pid_alive, _read_pidfile

            with connect_repo(Path(str(row["repo_root"]))) as repo_conn:
                runs = repo_conn.execute(
                    "SELECT run_id FROM runs WHERE state IN ('running','paused')"
                ).fetchall()
            for run in runs:
                pidfile = pidfile_path(Path(str(row["repo_root"])), str(run["run_id"]))
                pid = _read_pidfile(pidfile)
                if pid is not None and _pid_alive(pid):
                    records.append({"check": "daemon_duplicate_recovery_scheduler", "id": f"{row['repository_id']}:{run['run_id']}", "message": "recovery watch is active for a daemon-swept run", "context": {"pid": pid, "pidfile": str(pidfile)}})
        except Exception:  # noqa: BLE001
            continue
    return records


def _audit_chain_records(conn: sqlite3.Connection) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    previous: str | None = None
    rows = conn.execute("SELECT * FROM audit_log ORDER BY audit_id").fetchall()
    for row in rows:
        if row["previous_hash"] != previous:
            records.append({"check": "daemon_audit_chain", "id": str(row["audit_id"]), "message": "daemon audit hash chain is broken", "context": {"expected_previous_hash": previous, "actual_previous_hash": row["previous_hash"]}})
            break
        material = {
            "timestamp": row["timestamp"],
            "client_id": row["client_id"],
            "repository_id": row["repository_id"],
            "command": row["command"],
            "authorization_result": row["authorization_result"],
            "denial_reason": row["denial_reason"],
            "transport": row["transport"],
            "request_id": row["request_id"],
            "exit_code": row["exit_code"],
            "payload_sha256": row["payload_sha256"],
            "previous_hash": row["previous_hash"],
            "segment_id": row["segment_id"],
        }
        computed = hashlib.sha256(json_dumps(material).encode("utf-8")).hexdigest()
        if not hmac.compare_digest(computed, str(row["row_hash"])):
            records.append({"check": "daemon_audit_row_hash", "id": str(row["audit_id"]), "message": "daemon audit row hash is invalid", "context": {}})
            break
        previous = str(row["row_hash"])
    records.extend(_audit_segment_records(conn))
    return records


def _audit_segment_records(conn: sqlite3.Connection) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    segments = conn.execute("SELECT * FROM audit_segments ORDER BY segment_id").fetchall()
    previous_segment_last_hash: str | None = None
    for segment in segments:
        segment_id = int(segment["segment_id"])
        rows = conn.execute(
            "SELECT * FROM audit_log WHERE segment_id = ? ORDER BY audit_id",
            (segment_id,),
        ).fetchall()
        if segment["state"] == "purged":
            if rows:
                records.append({"check": "daemon_audit_segment_retention", "id": str(segment_id), "message": "purged audit segment still has retained rows", "context": {}})
            previous_segment_last_hash = str(segment["last_hash"]) if segment["last_hash"] is not None else previous_segment_last_hash
            continue
        if segment["previous_segment_last_hash"] != previous_segment_last_hash:
            records.append({"check": "daemon_audit_segment_chain", "id": str(segment_id), "message": "daemon audit segment chain is broken", "context": {"expected_previous_segment_last_hash": previous_segment_last_hash, "actual_previous_segment_last_hash": segment["previous_segment_last_hash"]}})
            break
        if not rows:
            if segment["first_audit_id"] is not None or segment["last_audit_id"] is not None:
                records.append({"check": "daemon_audit_segment_rows", "id": str(segment_id), "message": "daemon audit segment manifest references missing rows", "context": {}})
                break
            previous_segment_last_hash = str(segment["last_hash"]) if segment["last_hash"] is not None else previous_segment_last_hash
            continue
        first = rows[0]
        last = rows[-1]
        if (
            segment["first_audit_id"] != first["audit_id"]
            or segment["last_audit_id"] != last["audit_id"]
            or segment["first_hash"] != first["row_hash"]
            or segment["last_hash"] != last["row_hash"]
        ):
            records.append({"check": "daemon_audit_segment_manifest", "id": str(segment_id), "message": "daemon audit segment manifest does not match retained rows", "context": {}})
            break
        if first["previous_hash"] != previous_segment_last_hash:
            records.append({"check": "daemon_audit_segment_boundary", "id": str(segment_id), "message": "daemon audit segment boundary hash is broken", "context": {"expected_first_previous_hash": previous_segment_last_hash, "actual_first_previous_hash": first["previous_hash"]}})
            break
        previous_segment_last_hash = str(last["row_hash"])
    return records


def daemon_sweep_once(
    *,
    per_run_timeout_seconds: float = DEFAULT_SWEEP_TIMEOUT_SECONDS,
    require_client_auth: bool = False,
    pg_conn: Any | None = None,
) -> dict[str, Any]:
    if pg_conn is not None or _pg_connection_configured():
        owns_pg_conn = pg_conn is None
        if pg_conn is None:
            from striatum.daemon_pg.connection import connect_and_migrate

            pg_conn = connect_and_migrate()
        try:
            return _daemon_sweep_once_pg(
                pg_conn,
                per_run_timeout_seconds=per_run_timeout_seconds,
                require_client_auth=require_client_auth,
            )
        finally:
            if owns_pg_conn and pg_conn is not None:
                pg_conn.close()
    conn = connect_registry()
    token = read_runtime_token()
    with registry_transaction(conn):
        if require_client_auth:
            _require_auth(conn, command="daemon.sweep", required=ADMIN_CAPABILITY, token=token)
        else:
            auth = AuthContext(None, None, "admin", None, "allowed")
            audit_request(conn, command="daemon.sweep", auth=auth, transport="daemon-internal")
        repos = conn.execute(
            """
            SELECT r.*
            FROM repositories r
            LEFT JOIN scheduler_cursors c
              ON c.repository_id = r.repository_id
             AND c.cursor_kind = 'recovery'
             AND c.state != 'removed'
            WHERE r.state = 'active'
            GROUP BY r.repository_id
            ORDER BY COALESCE(MIN(c.last_sweep_at), ''), r.repository_id
            """
        ).fetchall()
        instance_id = _instance_id(conn)
    author = f"striatumd-{instance_id}"
    sweeps: list[dict[str, Any]] = []
    for repo_row in repos:
        repo = Path(str(repo_row["repo_root"]))
        try:
            with connect_repo(repo) as repo_conn:
                runs = repo_conn.execute(
                    "SELECT run_id FROM runs WHERE state IN ('running','paused') ORDER BY created_at"
                ).fetchall()
                for run in runs:
                    run_id = str(run["run_id"])
                    with registry_transaction(conn):
                        conn.execute(
                            """
                            INSERT OR REPLACE INTO scheduler_cursors(repository_id, run_id,
                              cursor_kind, last_sweep_at, next_sweep_after,
                              last_result_json, state)
                            VALUES (?, ?, 'recovery', NULL, NULL, '{}', 'in_progress')
                            """,
                            (int(repo_row["repository_id"]), run_id),
                        )
                    started = time.monotonic()
                    result = run_auto_sweep(repo_conn, run_id=run_id, repo=repo, recovery_author=author)
                    elapsed = time.monotonic() - started
                    state = "active"
                    if elapsed > per_run_timeout_seconds:
                        state = "sweep_degraded"
                        result = {**result, "degraded": True, "degraded_reason": "sweep_timeout", "elapsed_seconds": elapsed}
                    with transaction(repo_conn):
                        insert_event(
                            repo_conn,
                            run_id=run_id,
                            event_type="daemon.recovery_sweep",
                            payload={
                                "author": author,
                                "repository_id": int(repo_row["repository_id"]),
                                "result": result,
                            },
                        )
                    with registry_transaction(conn):
                        conn.execute(
                            """
                            UPDATE scheduler_cursors
                            SET last_sweep_at = ?, last_result_json = ?, state = ?
                            WHERE repository_id = ? AND run_id = ? AND cursor_kind = 'recovery'
                            """,
                            (utc_now(), json_dumps(result), state, int(repo_row["repository_id"]), run_id),
                        )
                    sweeps.append({"repository_id": int(repo_row["repository_id"]), "run_id": run_id, "result": result})
        except Exception as exc:  # noqa: BLE001
            with registry_transaction(conn):
                conn.execute(
                    """
                    INSERT OR REPLACE INTO scheduler_cursors(repository_id, run_id,
                      cursor_kind, last_sweep_at, next_sweep_after,
                      last_result_json, state)
                    VALUES (?, '<repo>', 'recovery', ?, NULL, ?, 'sweep_degraded')
                    """,
                    (int(repo_row["repository_id"]), utc_now(), json_dumps({"error": str(exc)})),
                )
            sweeps.append({"repository_id": int(repo_row["repository_id"]), "error": str(exc)})
    _ = token
    return {"mode": "daemon", "sweeps": sweeps}


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
    if _pg_connection_configured():
        from striatum.daemon_pg.connection import connect_and_migrate

        pg_conn = connect_and_migrate()
        try:
            _bootstrap_pg_admin_if_needed(pg_conn)
            try:
                pg_conn.commit()
            except Exception:  # noqa: BLE001 - autocommit connections do not need an explicit commit.
                pass
            return daemon_audit_pg(pg_conn, limit=limit, token=read_runtime_token())
        finally:
            pg_conn.close()

    conn = connect_registry()
    token = read_runtime_token()
    with registry_transaction(conn):
        _require_auth(conn, command="daemon.audit", required=ADMIN_CAPABILITY, token=token, payload={"limit": limit})
        rows = conn.execute(
            """
            SELECT audit_id, timestamp, client_id, repository_id, command,
                   authorization_result, denial_reason, transport,
                   request_id, exit_code, payload_sha256, previous_hash,
                   row_hash, segment_id
            FROM audit_log
            ORDER BY audit_id DESC
            LIMIT ?
            """,
            (limit,),
        ).fetchall()
        return {"mode": "daemon", "audit": [dict(row) for row in rows]}


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
    if _pg_connection_configured():
        from striatum.daemon_pg.connection import connect_and_migrate

        pg_conn = connect_and_migrate()
        try:
            return health_pg(pg_conn)
        finally:
            pg_conn.close()

    conn = connect_registry()
    with registry_transaction(conn):
        audit_request(
            conn,
            command="health",
            auth=AuthContext(None, None, None, None, "allowed"),
            transport="cli",
        )
    return {"mode": "daemon", "ok": True, "protocol_version": PROTOCOL_VERSION}


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


def main(argv: Sequence[str] | None = None) -> int:
    """Console entry point for ``striatumd``."""
    from striatum.cli import main as cli_main

    return cli_main(["daemon", "start", *(list(argv) if argv is not None else sys.argv[1:])])
