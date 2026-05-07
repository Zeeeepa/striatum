# RFC 0006: SQLite Schema Migration System

Status: proposed
Date: 2026-05-07

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
