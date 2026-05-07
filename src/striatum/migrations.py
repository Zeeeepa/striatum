"""SQLite schema migration registry for the local state store.

Schema version is tracked through SQLite's `PRAGMA user_version`. The current
schema is version 1, encoded as the first migration in :data:`MIGRATIONS`.
Newer schema changes append additional migrations with strictly increasing
version numbers.

`apply_migrations` is the only entry point. It opens a single
`BEGIN IMMEDIATE` transaction, applies every pending migration in order, sets
`PRAGMA user_version` to the new latest, and commits. Re-running it on an
already-current database is a no-op. Running it against a database whose
`user_version` is higher than :data:`LATEST_VERSION` raises a
:class:`SchemaVersionError` so an older Striatum install does not silently
operate on a newer database.
"""

from __future__ import annotations

import sqlite3
from dataclasses import dataclass
from typing import Callable

from striatum.errors import SchemaVersionError
from striatum.schema import SCHEMA_SQL


@dataclass(frozen=True)
class Migration:
    """A single forward schema migration."""

    version: int
    label: str
    apply: Callable[[sqlite3.Connection], None]


def _apply_v1(conn: sqlite3.Connection) -> None:
    """Apply the V1 baseline schema."""
    conn.executescript(SCHEMA_SQL)


MIGRATIONS: list[Migration] = sorted(
    [
        Migration(version=1, label="v1 baseline schema", apply=_apply_v1),
    ],
    key=lambda migration: migration.version,
)


LATEST_VERSION: int = MIGRATIONS[-1].version


def current_user_version(conn: sqlite3.Connection) -> int:
    """Return the database's current `PRAGMA user_version`."""
    row = conn.execute("PRAGMA user_version").fetchone()
    if row is None:
        return 0
    return int(row[0])


def apply_migrations(conn: sqlite3.Connection) -> None:
    """Apply every pending migration to the connection.

    Reads the database's `PRAGMA user_version`, applies every migration with
    a strictly higher version inside a single `BEGIN IMMEDIATE` transaction,
    then sets `PRAGMA user_version` to :data:`LATEST_VERSION`. Refuses to run
    when the database version is higher than the latest registered migration.
    """
    current = current_user_version(conn)
    if current > LATEST_VERSION:
        raise SchemaVersionError(
            "striatum state schema is newer than this installation supports: "
            f"database user_version={current}, runner supports={LATEST_VERSION}; "
            "upgrade striatum or use a matching install",
        )
    pending = [migration for migration in MIGRATIONS if migration.version > current]
    if not pending:
        return
    conn.execute("BEGIN IMMEDIATE")
    try:
        for migration in pending:
            migration.apply(conn)
        conn.execute(f"PRAGMA user_version = {LATEST_VERSION}")
    except Exception:
        conn.rollback()
        raise
    else:
        conn.commit()
