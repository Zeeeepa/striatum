author: designer-gemini-pro-003

# Track C Design: Repo-local State to Postgres (RFC 0042)

This document outlines the design for migrating repository-local state (currently in `.striatum/state.sqlite3`) into the centralized PostgreSQL substrate managed by the `striatumd` daemon. This transition is a foundational step for the "daemon-first" product shape (D082), enabling cross-repository observability and unified coordination without abandoning the local-first ethos.

## 1. Architectural Strategy

The authoritative state for a repository will move from a local SQLite file to the daemon-owned PostgreSQL database. This migration preserves the single-tenant trust boundary (D083) while providing the concurrency and MVCC benefits of PostgreSQL.

- **Storage Substrate:** The `striatumd` daemon (Go-based per RFC 0039) remains the only process that connects directly to the PostgreSQL database.
- **Client Access:** The `striatum` CLI and other clients interact with repository-local state exclusively through the daemon's RPC interface (envelope-v1, RFC 0030).
- **Isolation Model:** Isolation is achieved at the schema level within PostgreSQL. A new `striatum` schema will house all repository-local tables, with rows keyed by `repository_id` to enable multi-tenancy.

## 2. Schema Evolution

The PostgreSQL `striatum` schema will replicate the tables defined in the V1 SQLite schema (`src/striatum/schema.py`) with the following enhancements:

### 2.1 Multi-tenant Identity
- Every table in the `striatum` schema MUST include a `repository_id TEXT` column.
- `repository_id` is a foreign key referencing `striatumd.repositories(repository_id)`.
- Existing primary keys (UUID-based strings like `run_id`, `job_id`, `session_id`) remain unique across all repositories, but `repository_id` is added to indices to optimize per-repository queries.

### 2.2 Data Type Mapping
| SQLite Type | PostgreSQL Type | Notes |
| :--- | :--- | :--- |
| `TEXT` (UUID) | `text` | Preservation of existing UUID strings. |
| `TEXT` (Timestamp) | `timestamptz` | Native Postgres timezone-aware timestamps. |
| `TEXT` (JSON) | `jsonb` | Efficient storage and queryability of structured data. |
| `INTEGER` (Boolean) | `boolean` | Migration from 0/1 integers to native booleans. |
| `INTEGER` (ID) | `bigint` | Used for `event_id` and other auto-incrementing fields. |

### 2.3 Specific Table Adjustments
- **`events`**: `event_id` moves from `INTEGER PRIMARY KEY AUTOINCREMENT` to `bigserial`.
- **`queue_messages`**: `visible_after` and other timing fields use `timestamptz`.
- **`artifacts`**: `size_bytes` uses `bigint`.

## 3. Migration Workflow

The migration is an operator-initiated, one-way cutover for a specific repository.

### 3.1 Migration Command
`striatum daemon migrate-repo-local [--repo-root <path>]`

### 3.2 Sequence of Operations
1. **Discovery:** The CLI identifies the repository and verifies it is registered with the running `striatumd` daemon.
2. **Locking:** The daemon acquires an advisory lock (`pg_advisory_lock`) keyed by the `repository_id` to ensure no other process (or daemon instance) attempts to migrate or mutate this repo's state during the cutover.
3. **Snapshot:** The daemon instructs the CLI (or a helper) to perform a final `VACUUM INTO` on `.striatum/state.sqlite3` to create a clean snapshot.
4. **Rollback Backup:** `.striatum/state.sqlite3` is renamed to `.striatum/state.sqlite3.rollback` and marked read-only (chmod `444`).
5. **Data Transfer:**
   - The daemon reads the SQLite snapshot and batch-inserts rows into the PostgreSQL `striatum` schema.
   - Every row is enriched with the `repository_id`.
   - The transfer occurs within a single PostgreSQL transaction.
6. **Finalization:**
   - The `striatumd.repositories` entry for the repo is updated with `state_substrate = 'pg'`.
   - The local `.striatum/config` (or a new marker file `.striatum/substrate.pg`) is updated to inform the CLI that SQLite is no longer authoritative.

## 4. Operational Semantics & Failure Modes

### 4.1 Daemon-Mandatory Operation
Once migrated, the repository state is inaccessible without a running daemon.
- **Daemon Down:** The CLI will refuse to execute stateful commands (e.g., `run start`, `job claim`), emitting a clear error message: *"Repository state has been migrated to Postgres. Please start the 'striatumd' daemon to continue."*
- **PG Unavailable:** The daemon will return an RPC error (e.g., exit code 10 or a specific envelope error) when Postgres is unreachable. The CLI surfaces this to the operator.

### 4.2 Rollback Story
The `.striatum/state.sqlite3.rollback` file is preserved indefinitely.
- **Emergency Inspection:** An operator can run `striatum --local-only --read-only status` to inspect the state at the moment of migration.
- **Full Revert:** Reverting is a manual "break-glass" operation involving:
  1. Stopping the daemon.
  2. Renaming `.rollback` back to `state.sqlite3`.
  3. Updating the daemon registry and local config to point back to SQLite.
  *Note: Any work performed in PG after migration will be lost upon revert.*

### 4.3 Concurrent Operators
Single-tenant trust (D083) remains the primary security model. Multiple agents or human operators on the same OS account can safely interact with the same repository via the daemon. PostgreSQL's MVCC ensures that heartbeats, queue claims, and event logging do not deadlock.

### 4.4 Partial Migration
If the migration process is interrupted (e.g., SIGKILL, PG crash), the PostgreSQL transaction is rolled back by the database. The `.striatum/state.sqlite3` file remains the authoritative store, and the operator can re-attempt the migration.

## 5. Design Decisions (Supersession)

- **Supersedes D006/D007:** The authoritative state moves from the repository worktree (SQLite) to the daemon substrate (Postgres). The `.striatum/` directory remains as the identity root and a location for operational scratch data, but not for the live message bus.
- **Aligns with D082/D084/D086:** Fulfills the "daemon-first" and "non-SQLite substrate" mandates by unifying repo-local and daemon-global state in a single robust database.
- **Preserves D083:** Multi-user isolation is still deferred. The `repository_id` provides logical isolation, not security isolation between different OS users.
