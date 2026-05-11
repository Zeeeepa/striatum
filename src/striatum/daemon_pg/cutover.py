"""V1 SQLite daemon registry to V2 PostgreSQL cutover."""

from __future__ import annotations

import sqlite3
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from striatum import daemon as sqlite_daemon
from striatum.db import utc_now
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.migrations import apply_migrations, read_schema_version
from striatum.errors import StriatumError

CUTOVER_COMPLETED_KEY = "pg_cutover_completed_at"
CUTOVER_FINAL_HASH_KEY = "pg_cutover_final_hash"
CUTOVER_FINGERPRINT_KEY = "pg_cutover_db_fingerprint"


@dataclass(frozen=True)
class CutoverOptions:
    source_registry: Path
    postgres_url: str
    dry_run: bool = False
    keep_sqlite_readonly: bool = False


def migrate(options: CutoverOptions) -> dict[str, Any]:
    source = _open_source(options.source_registry)
    try:
        _ensure_not_already_cut_over(source)
        source_counts = _source_counts(source)
        source_final_hash = _source_final_hash(source)
        source_problems = sqlite_daemon._audit_chain_records(source)
        if source_problems:
            raise StriatumError("V1 daemon registry audit chain is not clean; refusing cutover", exit_code=13)
        pg = connect(options.postgres_url)
        try:
            schema_version = apply_migrations(pg)
            _refuse_non_empty_destination(pg)
            if options.dry_run:
                return {
                    "mode": "daemon_pg_cutover",
                    "dry_run": True,
                    "source_registry": str(options.source_registry),
                    "source_counts": source_counts,
                    "source_final_hash": source_final_hash,
                    "destination_schema_version": schema_version,
                    "destination_empty": True,
                }
            imported = _import_all(source, pg)
            _write_cutover_marker(
                source,
                fingerprint=_destination_fingerprint(pg),
                final_hash=source_final_hash,
                keep_sqlite_readonly=options.keep_sqlite_readonly,
            )
            return {
                "mode": "daemon_pg_cutover",
                "dry_run": False,
                "source_registry": str(options.source_registry),
                "source_counts": source_counts,
                "imported_counts": imported,
                "source_final_hash": source_final_hash,
                "destination_schema_version": read_schema_version(pg),
                "keep_sqlite_readonly": options.keep_sqlite_readonly,
            }
        finally:
            pg.close()
    finally:
        source.close()


def _open_source(path: Path) -> sqlite3.Connection:
    conn = sqlite3.connect(path)
    conn.row_factory = sqlite3.Row
    return conn


def _ensure_not_already_cut_over(conn: sqlite3.Connection) -> None:
    row = conn.execute("SELECT value FROM daemon_meta WHERE key = ?", (CUTOVER_COMPLETED_KEY,)).fetchone()
    if row is not None:
        raise StriatumError("V1 daemon registry has already been cut over to PostgreSQL", exit_code=13)


def _source_counts(conn: sqlite3.Connection) -> dict[str, int]:
    tables = [
        "repositories",
        "clients",
        "client_capabilities",
        "daemon_meta",
        "scheduler_cursors",
        "audit_segments",
        "audit_log",
    ]
    return {table: int(conn.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0]) for table in tables}


def _source_final_hash(conn: sqlite3.Connection) -> str | None:
    row = conn.execute("SELECT row_hash FROM audit_log ORDER BY audit_id DESC LIMIT 1").fetchone()
    return None if row is None else str(row["row_hash"])


def _refuse_non_empty_destination(conn: Any) -> None:
    tables = [
        "repositories",
        "clients",
        "client_capabilities",
        "scheduler_cursors",
        "audit_segments",
        "audit_log",
    ]
    with conn.cursor() as cur:
        for table in tables:
            cur.execute(f"SELECT COUNT(*) FROM striatumd.{table}")
            count = int(cur.fetchone()[0])
            if table == "audit_segments":
                cur.execute(
                    """
                    SELECT COUNT(*) FROM striatumd.audit_segments
                    WHERE first_audit_id IS NOT NULL OR last_audit_id IS NOT NULL
                    """
                )
                count = int(cur.fetchone()[0])
            if count > 0:
                raise StriatumError("destination PostgreSQL daemon tables are not empty", exit_code=13)


def _import_all(source: sqlite3.Connection, dest: Any) -> dict[str, int]:
    counts: dict[str, int] = {}
    try:
        with dest.cursor() as cur:
            cur.execute(
                """
                DELETE FROM striatumd.audit_segments
                WHERE first_audit_id IS NULL AND last_audit_id IS NULL
                """
            )
            counts["repositories"] = _copy_rows(source, cur, "repositories", _insert_repository)
            counts["clients"] = _copy_rows(source, cur, "clients", _insert_client)
            counts["client_capabilities"] = _copy_rows(source, cur, "client_capabilities", _insert_capability)
            counts["daemon_meta"] = _copy_rows(source, cur, "daemon_meta", _insert_meta)
            counts["scheduler_cursors"] = _copy_rows(source, cur, "scheduler_cursors", _insert_cursor)
            counts["audit_segments"] = _copy_rows(source, cur, "audit_segments", _insert_segment)
            counts["audit_log"] = _copy_rows(source, cur, "audit_log", _insert_audit)
        dest.commit()
    except Exception:
        dest.rollback()
        raise
    return counts


def _copy_rows(source: sqlite3.Connection, cur: Any, table: str, inserter: Any) -> int:
    rows = source.execute(f"SELECT * FROM {table} ORDER BY 1").fetchall()
    for row in rows:
        inserter(cur, row)
    return len(rows)


def _insert_repository(cur: Any, row: sqlite3.Row) -> None:
    cur.execute(
        """
        INSERT INTO striatumd.repositories(repository_id, repo_identity, repo_root,
          state_db_path, display_name, registered_at, removed_at, last_seen_at,
          last_schema_version, state, settings_json)
        VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s::jsonb)
        """,
        tuple(str(row[key]) if key == "repository_id" else row[key] for key in row.keys()),
    )


def _insert_client(cur: Any, row: sqlite3.Row) -> None:
    cur.execute(
        """
        INSERT INTO striatumd.clients(client_id, client_kind, display_name, token_id,
          token_hash, token_salt, created_at, expires_at, revoked_at, last_used_at)
        VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
        """,
        tuple(row),
    )


def _insert_capability(cur: Any, row: sqlite3.Row) -> None:
    values = list(row)
    values[2] = None if values[2] is None else str(values[2])
    cur.execute(
        """
        INSERT INTO striatumd.client_capabilities(capability_id, client_id,
          repository_id, capability, granted_at, expires_at, revoked_at, revoked_reason)
        VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
        """,
        tuple(values),
    )


def _insert_meta(cur: Any, row: sqlite3.Row) -> None:
    cur.execute(
        """
        INSERT INTO striatumd.daemon_meta(key, value)
        VALUES (%s, %s)
        ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
        """,
        tuple(row),
    )


def _insert_cursor(cur: Any, row: sqlite3.Row) -> None:
    values = list(row)
    values[0] = str(values[0])
    cur.execute(
        """
        INSERT INTO striatumd.scheduler_cursors(repository_id, run_id, cursor_kind,
          last_sweep_at, next_sweep_after, last_result_json, state)
        VALUES (%s, %s, %s, %s, %s, %s::jsonb, %s)
        """,
        tuple(values),
    )


def _insert_segment(cur: Any, row: sqlite3.Row) -> None:
    cur.execute(
        """
        INSERT INTO striatumd.audit_segments(segment_id, opened_at, closed_at,
          first_audit_id, last_audit_id, first_hash, last_hash,
          previous_segment_id, previous_segment_last_hash,
          next_segment_first_previous_hash, retention_state, state)
        OVERRIDING SYSTEM VALUE
        VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
        """,
        tuple(row),
    )


def _insert_audit(cur: Any, row: sqlite3.Row) -> None:
    cur.execute(
        """
        INSERT INTO striatumd.audit_log(audit_id, ts, schema_version,
          hash_format_version, daemon_version, client_id, repository_id,
          method, decision, denial_reason, transport, request_id, exit_code,
          params_sha256, previous_hash, row_hash, segment_id)
        OVERRIDING SYSTEM VALUE
        VALUES (%s, %s, 1, 1, 'v1-import', %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
        """,
        (
            row["audit_id"],
            row["timestamp"],
            row["client_id"],
            None if row["repository_id"] is None else str(row["repository_id"]),
            row["command"],
            row["authorization_result"],
            row["denial_reason"],
            row["transport"],
            row["request_id"],
            row["exit_code"],
            row["payload_sha256"],
            row["previous_hash"],
            row["row_hash"],
            row["segment_id"],
        ),
    )


def _write_cutover_marker(
    conn: sqlite3.Connection,
    *,
    fingerprint: str,
    final_hash: str | None,
    keep_sqlite_readonly: bool,
) -> None:
    now = utc_now()
    conn.execute("BEGIN IMMEDIATE")
    try:
        conn.execute(
            "INSERT INTO daemon_meta(key, value) VALUES(?, ?)",
            (CUTOVER_COMPLETED_KEY, now),
        )
        conn.execute(
            "INSERT INTO daemon_meta(key, value) VALUES(?, ?)",
            (CUTOVER_FINGERPRINT_KEY, fingerprint),
        )
        conn.execute(
            "INSERT INTO daemon_meta(key, value) VALUES(?, ?)",
            (CUTOVER_FINAL_HASH_KEY, final_hash or ""),
        )
        conn.execute(
            "INSERT INTO daemon_meta(key, value) VALUES(?, ?)",
            ("pg_cutover_keep_sqlite_readonly", "true" if keep_sqlite_readonly else "false"),
        )
    except Exception:
        conn.rollback()
        raise
    else:
        conn.commit()


def _destination_fingerprint(conn: Any) -> str:
    with conn.cursor() as cur:
        cur.execute("SELECT current_database(), current_user")
        row = cur.fetchone()
    return f"postgres:{row[0]}:{row[1]}"
