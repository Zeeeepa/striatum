"""PostgreSQL repository registry helpers.

These helpers are shared by the Python CLI client path and daemon RPC router.
They deliberately live outside the deleted Python daemon module so repository
registration can survive Python-daemon deletion.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from striatum.bootstrap import (
    init_operational_scratch as _bootstrap_init_operational_scratch,
    require_operational_scratch as _bootstrap_require_operational_scratch,
)
from striatum.errors import EXIT_DAEMON_CAPABILITY, NotFoundError, SchemaVersionError, StriatumError
from striatum.primitives import utc_now
from striatum.repo_local_schema import LATEST_REPO_LOCAL_SCHEMA_VERSION
from striatum.repo_policy import DB_NAME, db_path, state_dir


class RepoRegistrationError(StriatumError):
    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_DAEMON_CAPABILITY)


def repo_add_pg(
    pg_conn: Any,
    path: Path,
    *,
    display_name: str | None = None,
    no_migrate: bool = False,
    init: bool = False,
) -> dict[str, Any]:
    repo = _canonical_repo(path)
    legacy_state = db_path(repo)
    if legacy_state.exists():
        raise SchemaVersionError(
            "retired repo-local state exists and import windows are closed; "
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
            item = _repository_projection(row)
            return {
                "repository_id": str(item["repository_id"]),
                "repo_root": str(item["repo_root"]),
                "repo_identity": str(item["repo_identity"]),
                "state_db_path": str(item["state_db_path"]),
                "schema_version": int(item["last_schema_version"]),
                "state": str(item["state"]),
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
            raise RepoRegistrationError("active repository path is occupied by a different repo identity")
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
                LATEST_REPO_LOCAL_SCHEMA_VERSION,
            ),
        )
    pg_conn.commit()
    return {
        "repository_id": repository_id,
        "repo_root": str(repo),
        "repo_identity": identity,
        "state_db_path": str(state_dir.resolve()),
        "schema_version": LATEST_REPO_LOCAL_SCHEMA_VERSION,
        "state": "active",
    }


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
        item = _repository_projection(_pg_row_dict(row))
        item["repository_id"] = str(item["repository_id"])
        repositories.append(item)
    return {"repositories": repositories}


def repo_resolve_pg(pg_conn: Any, path: Path) -> dict[str, Any]:
    repo = _canonical_repo(path)
    row = find_repo_pg(pg_conn, str(repo), include_removed=False)
    if row is None:
        raise NotFoundError(f"active repository path is not registered: {repo}")
    item = _repository_projection(row)
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


def repo_remove_pg(pg_conn: Any, identifier: str) -> dict[str, Any]:
    row = find_repo_pg(pg_conn, identifier, include_removed=True)
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


def find_repo_pg(pg_conn: Any, identifier: str, *, include_removed: bool = False) -> dict[str, Any] | None:
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


def _canonical_repo(path: Path) -> Path:
    raw = Path(path).expanduser()
    if ".." in raw.parts:
        raise RepoRegistrationError("repo path traversal is not allowed")
    lexical = raw if raw.is_absolute() else Path.cwd() / raw
    if _has_symlink_component(lexical):
        raise RepoRegistrationError("repo registration refuses symlink paths")
    resolved = raw.resolve(strict=True)
    state = db_path(resolved)
    if _has_symlink_component(state.parent) or state.is_symlink():
        raise RepoRegistrationError("repo state database symlink is not allowed")
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


def _repository_projection(row: dict[str, Any]) -> dict[str, Any]:
    item = _pg_json_ready(row)
    repo_root = item.get("repo_root")
    if repo_root is None:
        return item
    scratch_path = state_dir(Path(str(repo_root)))
    stored_state_path = Path(str(item.get("state_db_path", "")))
    if stored_state_path.name == DB_NAME and stored_state_path.parent.name == scratch_path.name:
        item["state_db_path"] = str(scratch_path)
    return item


def _init_operational_scratch(repo: Path) -> Path:
    return _bootstrap_init_operational_scratch(
        repo,
        error_factory=RepoRegistrationError,
    )


def _require_operational_scratch(repo: Path) -> Path:
    return _bootstrap_require_operational_scratch(
        repo,
        error_factory=RepoRegistrationError,
    )
