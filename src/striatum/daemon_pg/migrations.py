"""Forward-only PostgreSQL migrations for daemon-owned state."""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from importlib import resources
from typing import Any

from striatum.errors import SchemaVersionError, StriatumError

LATEST_DAEMON_DB_VERSION = 8
MIGRATION_LOCK_KEY = 332933


@dataclass(frozen=True)
class PgMigration:
    version: int
    label: str
    resource: str

    @property
    def sql(self) -> str:
        return resources.files("striatum.daemon_pg.sql").joinpath(self.resource).read_text(encoding="utf-8")

    @property
    def sha256(self) -> str:
        return hashlib.sha256(self.sql.encode("utf-8")).hexdigest()


MIGRATIONS: tuple[PgMigration, ...] = (
    PgMigration(1, "baseline daemon postgres substrate", "0001_baseline.sql"),
    PgMigration(2, "daemon rpc supervision and apply receipts", "0002_rpc_supervision_apply.sql"),
    PgMigration(3, "cross-repo workflows and MCP mutation scope", "0003_cross_repo_mcp.sql"),
    PgMigration(4, "dogfood surgical recovery capability", "0004_dogfood_surgical_recovery.sql"),
    PgMigration(5, "repo-local workflow state", "0005_repo_local_workflow_state.sql"),
    PgMigration(6, "events chain anchors + repo_event_chain_heads", "0006_events_chain_anchors.sql"),
    PgMigration(7, "decision propagation projections", "0007_decision_propagation.sql"),
    PgMigration(8, "lane evidence publish guard", "0008_lane_evidence_publish_guard.sql"),
)


def apply_migrations(conn: Any, *, daemon_version: str = "unknown") -> int:
    """Apply pending daemon PostgreSQL migrations and return the schema version.

    ``conn`` is intentionally a DB-API-like object so tests can use light
    fakes and production can use psycopg without importing it at module load.
    """
    with conn.cursor() as cur:
        cur.execute("SELECT pg_advisory_lock(%s)", (MIGRATION_LOCK_KEY,))
    try:
        _ensure_meta_table(conn)
        current = read_schema_version(conn)
        if current > LATEST_DAEMON_DB_VERSION:
            raise SchemaVersionError(
                f"daemon PostgreSQL schema version {current} is newer than supported {LATEST_DAEMON_DB_VERSION}"
            )
        for migration in MIGRATIONS:
            if migration.version <= current:
                _verify_recorded_hash(conn, migration)
                continue
            _apply_one(conn, migration, daemon_version=daemon_version)
            current = migration.version
        conn.commit()
        return current
    except Exception:
        conn.rollback()
        raise
    finally:
        with conn.cursor() as cur:
            cur.execute("SELECT pg_advisory_unlock(%s)", (MIGRATION_LOCK_KEY,))


def read_schema_version(conn: Any) -> int:
    with conn.cursor() as cur:
        cur.execute("SELECT value FROM striatumd.schema_meta WHERE key = 'substrate_version'")
        row = cur.fetchone()
    if row is None:
        return 0
    return int(row[0])


def _ensure_meta_table(conn: Any) -> None:
    with conn.cursor() as cur:
        cur.execute("CREATE SCHEMA IF NOT EXISTS striatumd")
        cur.execute(
            """
            CREATE TABLE IF NOT EXISTS striatumd.schema_meta (
              key text PRIMARY KEY,
              value text NOT NULL
            )
            """
        )


def _verify_recorded_hash(conn: Any, migration: PgMigration) -> None:
    with conn.cursor() as cur:
        cur.execute(
            "SELECT sha256 FROM striatumd.schema_migrations WHERE version = %s",
            (migration.version,),
        )
        row = cur.fetchone()
    if row is not None and row[0] != migration.sha256:
        raise StriatumError(
            f"daemon PostgreSQL migration {migration.version} hash mismatch",
            exit_code=9,
        )


def _apply_one(conn: Any, migration: PgMigration, *, daemon_version: str) -> None:
    with conn.cursor() as cur:
        cur.execute(migration.sql)
        cur.execute(
            """
            INSERT INTO striatumd.schema_migrations(version, label, sha256, daemon_version)
            VALUES (%s, %s, %s, %s)
            ON CONFLICT (version) DO NOTHING
            """,
            (migration.version, migration.label, migration.sha256, daemon_version),
        )
        cur.execute(
            """
            INSERT INTO striatumd.schema_meta(key, value)
            VALUES ('substrate_version', %s)
            ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
            """,
            (str(migration.version),),
        )
