# Design: RFC 0033 Storage Substrate Rewrite

**Author:** designer-gemini-pro-001  
**Date:** 2026-05-11  
**Status:** Draft  
**Context:** [RFC 0033](../../rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md)

## 1. Introduction

This document specifies the implementation design for migrating the Striatum daemon state from a local SQLite registry to a system-provided PostgreSQL substrate (RFC 0033). This shift enables real MVCC, concurrent daemon-mediated mutations, and robust event streaming (via `LISTEN/NOTIFY`) required for the V2 daemon-first product.

## 2. Cross-Platform Reality

Striatum operators run on a variety of platforms. The daemon will not manage the PostgreSQL lifecycle; instead, it will connect to a system PostgreSQL.

### 2.1 Supported Platforms & Onboarding

`striatum daemon doctor` will provide platform-specific hints for PostgreSQL installation:

| Platform | Recommended Installation | Doctor Hint |
| :--- | :--- | :--- |
| **macOS** | Homebrew | `brew install postgresql@14 && brew services start postgresql@14` |
| **Linux (Ubuntu/Debian)** | apt | `sudo apt install postgresql-14 && sudo systemctl enable --now postgresql` |
| **Linux (Arch)** | pacman | `sudo pacman -S postgresql && sudo -u postgres initdb -D /var/lib/postgres/data && sudo systemctl enable --now postgresql` |
| **Windows (WSL2)** | apt (within WSL) | Same as Linux (Ubuntu/Debian). |

### 2.2 Connection Configuration

The daemon will use a connection string (DSN) provided via:
1. Environment variable: `STRIATUM_DAEMON_DB_URL`
2. Config file: `~/.config/striatum/daemon.toml` (key: `database.url`)
3. CLI flag: `--postgres-url`

Default if not provided: `postgresql://localhost/striatumd` (connecting as current OS user).

## 3. PostgreSQL Schema & Domain Mapping

The PostgreSQL schema will incorporate the V1 registry tables plus new V2 structures.

### 3.1 Registry & Multi-Repo Management
*   **`repositories`**: Mirrors V1 `repositories`. Stores metadata for registered target repos.
*   **`clients`**: Mirrors V1 `clients`. Stores client identity and token metadata.
*   **`client_capabilities`**: Mirrors V1 `client_capabilities`. Binds clients to repo-scoped or global permissions.

### 3.2 Audit & Provenance (RFC 0033 §5)
PostgreSQL's role system allows for stronger append-only enforcement than SQLite triggers.

*   **`audit_log`**: Append-only log of every mutating daemon request.
    *   `previous_hash`: Link to the preceding row hash.
    *   `row_hash`: SHA-256 of row content + `previous_hash`.
*   **`audit_segments`**: Manifests for closed ranges of the audit log.
*   **Privilege Design**:
    *   The `striatum_daemon` role will have `SELECT` and `INSERT` on `audit_log`, but NO `UPDATE` or `DELETE` permissions.
    *   The `striatum_admin` role (used for migrations and rotation) will have full permissions.

### 3.3 Supervision & Liveness (RFC 0031)
*   **`supervisors`**: Records active agent processes, their PID, and command configuration.
*   **`supervisor_heartbeats`**: High-frequency table for liveness checkpoints.

### 3.4 Request Coordination & Mutability (RFC 0030, 0032)
*   **`rpc_request_log`**: Logs incoming RPC requests, version handshake results, and client session state.
*   **`mutation_queue`**: Outbox for daemon-mediated mutations to repo-local SQLite files.
*   **`scheduler_cursors`**: Centralized cursors for multi-repo recovery sweeps.

## 4. Operational Lifecycle

### 4.1 PostgreSQL Versioning
*   **Minimum Version:** PostgreSQL 14 (released 2021). Provides required features like `SEARCH`/`CYCLE` in CTEs and robust JSONB support.
*   **Drift Detection:** `daemon doctor` checks `server_version_num` and warns/refuses if below the floor.

### 4.2 Role Privileges & Security
The daemon should ideally connect with a non-superuser role.
```sql
-- Example setup for operator
CREATE USER striatum_daemon WITH PASSWORD '...';
CREATE DATABASE striatumd OWNER striatum_daemon;
-- striatum_admin role for migrations (can be same as owner or more restricted)
```
**Append-only Enforcement:** The `audit_log` and `audit_segments` tables will use `GRANT INSERT, SELECT ON ... TO striatum_daemon` to enforce that the daemon can record history but not rewrite it, even if the daemon process is compromised.

### 4.3 Connection Pooling & Performance
The daemon is a long-lived process. It MUST use connection pooling to handle high-frequency supervisor heartbeats and concurrent CLI/Web clients.
*   **Implementation:** Use `psycopg_pool` (Python) or `pgxpool` (Go).
*   **Settings:** Default pool size 10-20; configurable for high-concurrency environments.

## 5. Packaging & Distribution Implications

*   **No PG Bundling:** The Striatum Python wheel will NOT include PostgreSQL binaries. This keeps the distribution small and avoids the complexity of managing a child PostgreSQL process lifecycle.
*   **Dependencies:** `psycopg` (v3) will be added to `pyproject.toml`. We will depend on the pure-python version with the `[c]` extra recommended for performance, requiring the operator to have `libpq` installed (usually bundled with PostgreSQL client tools).
*   **Binary Size:** By requiring system PG, the `striatum` binary (or wheel) footprint remains unchanged (~2-5 MB) rather than growing by 50-100 MB for a bundled PG.

## 6. Migration from V1 (SQLite) to V2 (PG)

### 5.1 The `migrate` Command
`striatum daemon migrate --from sqlite --to pg`

1.  **Dry Run:** `striatum daemon migrate --dry-run` reports row counts and integrity checks.
2.  **Schema Init:** Applies PG migrations to the target database.
3.  **Data Export:** Reads V1 registry SQLite and inserts into PG.
4.  **Audit Verification:** Re-computes the entire audit hash chain in PG to ensure parity with SQLite.
5.  **Checkpoint:** Writes a `migration_completed` marker in the SQLite registry. Future V1 registry writes are blocked.

### 5.2 Repository Continuity
Since `.striatum/state.sqlite3` remains in SQLite, the migration only affects how the daemon *knows* about these repos. The `repositories` table in PG will point to the same absolute paths on disk.

## 6. Development & Testing

### 6.1 Test Harness
To ensure deterministic testing without requiring a pre-configured PG instance:
1.  Use `initdb` to create a temporary cluster in a unique directory.
2.  Use `pg_ctl` to start/stop the instance on a random port.
3.  Set `STRIATUM_DAEMON_DB_URL` for the test session.
4.  Teardown: `pg_ctl stop` and delete the directory.

### 6.2 CI Integration
CI (GitHub Actions) will use the `postgres` service container and run migrations against it for integration tests.

## 8. Acceptance Criteria

*   **Connectivity:** The daemon successfully connects to a system PostgreSQL 14+ via `STRIATUM_DAEMON_DB_URL`.
*   **Onboarding:** `daemon doctor` correctly identifies a missing or old PostgreSQL installation and provides platform-specific `brew`/`apt` hints.
*   **Migration:** `striatum daemon migrate` accurately transfers V1 registry data to PostgreSQL, maintaining audit hash chain integrity.
*   **Concurrency:** Supervisor heartbeats and concurrent CLI commands do not cause deadlocks or connection exhaustion (verified via a high-concurrency test script).
*   **Security:** The `striatum_daemon` role is verified to be unable to `UPDATE` or `DELETE` rows in the `audit_log` table.
*   **Performance:** Connection pooling is active and respects the configured pool limits.
*   **Testing:** The `pg_ctl` test harness allows the full test suite to pass without a persistent PostgreSQL service.
