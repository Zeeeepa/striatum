# RFC 0006: SQLite Schema Migration System

Status: superseded / historical
Date: 2026-05-07

Superseded by: RFC 0033, RFC 0043, D094, and D113. This RFC is retained as
the historical SQLite migration design and fixture reference. It is not
current production authority; live workflow state is daemon-owned
PostgreSQL.

## Problem

Striatum currently uses a static `SCHEMA_SQL` in `src/striatum/schema.py` which is executed during `striatum init`. There is no mechanism to migrate existing `.striatum/state.sqlite3` databases when the schema changes. As the tool evolves beyond V1, users will be forced to delete their local state directory and lose run history whenever a schema update is required.

## Goals

- Provide a reliable, versioned path for schema updates.
- Allow the codebase to evolve its data model without breaking existing local runs.
- Prevent `striatum` from operating on an incompatible database version.

## Non-Goals

- Do not introduce a heavy external migration framework (like Alembic).
- Do not support "downward" migrations (rollbacks) unless explicitly required by a future decision.

## Proposal

1.  **Schema Versioning:** Add a `user_version` PRAGMA to the SQLite database to track the current migration level.
2.  **Migration Registry:** Create a registry of migration functions or SQL scripts in `src/striatum/migrations.py`.
3.  **Automatic Upgrade:** Update `striatum init` and connection logic to check the current version against the code's required version. If the database is behind, apply migrations in a single transaction.
4.  **CLI Guard:** Any command attempting to use an outdated or "future" database (from a newer Striatum version) should exit with a clear error message.

## Acceptance Criteria

- `striatum init` correctly sets the initial version.
- Adding a "dummy" migration increments the version and preserves existing data.
- The system correctly detects and refuses to run on a database version higher than the installed software supports.

## Open Questions

- Should migrations be pure SQL strings or Python functions that can perform complex data transformations?
- Should we provide a `striatum db status` command to inspect the current version?

## Implementation Notes

The first implementation lives in `src/striatum/migrations.py`:

- Schema version is tracked through SQLite's built-in `PRAGMA user_version`.
  The existing `schema_meta` table is left untouched and remains available for
  unrelated metadata. The current schema is recorded as `user_version = 1`.
- `MIGRATIONS` is a sorted `list[Migration]` registry. Each `Migration` has an
  integer `version`, a short human `label`, and an `apply(conn)` callable.
  Version 1 applies the existing `SCHEMA_SQL`; future schema changes append a
  new entry with the next version. `LATEST_VERSION` is derived from the
  registry tail.
- `apply_migrations(conn)` reads the current `PRAGMA user_version`, applies
  every pending migration in strict version order inside a single
  `BEGIN IMMEDIATE` transaction, and sets `PRAGMA user_version` to
  `LATEST_VERSION` before commit. Re-running it on an already-current
  database is a no-op.
- `db.init_repo()` calls `apply_migrations(conn)` instead of executing
  `SCHEMA_SQL` directly. `db.connect()` calls `apply_migrations(conn)` for
  any existing database, so forward upgrades are silent and automatic on the
  next CLI invocation.
- A database whose `user_version` is higher than `LATEST_VERSION` raises
  `SchemaVersionError`, the new dedicated `StriatumError` subclass that exits
  with code `9`. The CLI surfaces the standard `{"ok": false, "error": ...}`
  envelope so older Striatum installs do not silently operate on a newer
  database.
- Future schema additions (for example, a `process_executions` table) attach
  through the migration registry instead of through ad-hoc `IF NOT EXISTS`
  schema shims, so a single forward path covers every table.

The `db status` introspection command remains an open question and is not
required by this RFC.
