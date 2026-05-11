author: reviewer-gemini-pro-001

# Threat-Model Review: RFC 0033 Substrate Design

This review evaluates the proposed PostgreSQL storage substrate (RFC 0033) against the Striatum threat model, specifically focusing on trust boundaries, privilege escalation, and data integrity during the V1→V2 transition.

## 1. Trust Boundaries and Attack Surfaces

### 1.1 Daemon-to-Postgres Trust
**Threat:** Leak of the `STRIATUM_DAEMON_DB_URL` (especially if it contains credentials) to an over-eager AI agent.
**Analysis:** RFC 0033 proposes storing the connection string in an environment variable, a config file, or a CLI flag. While "malicious-local-root" is out of scope, an AI agent with read access to the environment or configuration files could exfiltrate the DB URL. If the DB is accessible over the network (even if loopback), the agent could bypass the daemon's capability checks and mutate the registry directly.
**Mitigation:**
- **Requirement:** The daemon MUST refuse to start if the DB URL is provided via an environment variable that is accessible to child processes (lanes). 
- **Recommendation:** Use Unix domain sockets for Postgres connections where possible to leverage OS-level permission checks (similar to the daemon socket itself).
- **Recommendation:** If credentials must be used, prefer the OS keyring or a `0600` file, following the V1 token pattern.

### 1.2 Role-Privilege Escalation
**Threat:** The daemon role being used to bypass audit append-only constraints or escalate privileges within the database.
**Analysis:** RFC 0033 §5 specifies using Postgres roles to enforce append-only logic for audit tables (INSERT only). This is a strong mitigation. However, the daemon still requires `UPDATE`/`DELETE` for other state (sessions, cursors, etc.).
**Attack Surface:** An agent that compromises the daemon process (within the allowed OS user scope) inherits the daemon's DB role.
**Mitigation:**
- **Requirement:** The `audit_log` and `audit_segments` tables MUST be owned by a separate `striatum_admin` role. The `striatum_daemon` role MUST only be granted `SELECT` and `INSERT` on these tables.
- **Requirement:** `striatum daemon doctor` MUST verify that the `striatum_daemon` role lacks `UPDATE` and `DELETE` privileges on the audit tables.

## 2. Integrity and Persistence

### 2.1 Audit-Chain Integrity Across Migration
**Threat:** Manipulation of the audit chain during the V1 (SQLite) to V2 (Postgres) migration.
**Analysis:** The migration process (`daemon migrate`) is a critical window. If an agent can influence the migration, it could inject or omit audit records.
**Mitigation:**
- **Requirement:** `daemon migrate` MUST perform a byte-equivalent hash verification of the entire audit chain after import. 
- **Requirement:** The migration MUST be atomic (wrapped in a single Postgres transaction).

### 2.2 Schema and User-Version Tamper
**Threat:** An agent triggering a migration to an older or "poisoned" schema version.
**Analysis:** RFC 0033 §3 states migrations are forward-only and daemon-owned.
**Mitigation:**
- **Requirement:** Migration triggers MUST require the `admin` capability.
- **Requirement:** The daemon MUST refuse to connect to a schema that is newer than its binary (Exit Code 9).

### 2.3 Cross-Platform PG Version Drift
**Threat:** Subtle behavioral differences between Postgres versions (e.g., collation, JSONB handling) leading to corrupted state or incorrect audit hashes.
**Analysis:** `daemon doctor` checks for the major version.
**Mitigation:**
- **Recommendation:** Pin a specific minimum major version (e.g., PG 14) and use explicit types (e.g., `TIMESTAMPTZ` for UTC stability) in all schema definitions.

## 3. Operator Onboarding and Data-Loss Footguns

### 3.1 Migration Failure / Partial Cutover
**Threat:** A failed migration leaves the system in a "split-brain" state where some components think they are on V2 while others are on V1.
**Analysis:** RFC 0033 §4 uses a "checkpoint marker" in the V1 SQLite.
**Mitigation:**
- **Requirement:** The checkpoint marker MUST be written ONLY after the V2 Postgres transaction is successfully committed and verified.
- **Requirement:** `daemon migrate` MUST be idempotent; it should be safe to rerun if it fails before the checkpoint is written.

### 3.2 Wrong Postgres Setup
**Threat:** Operator sets up Postgres with a superuser role for the daemon, neutralizing the append-only mitigations.
**Mitigation:**
- **Requirement:** `daemon doctor` MUST explicitly check and warn if the daemon is connected as a superuser. It SHOULD refuse to run in `sealed_patch` mode if the audit tables are not properly protected by role-based INSERT-only constraints.

## Verdict: ACCEPTED WITH CONDITIONS

The substrate design significantly improves on the V1 SQLite model by leveraging real MVCC and robust Postgres role-based permissions. However, the connection string security and the strict enforcement of role-based audit protection are critical to meeting the RFC 0031 threat model.

### Conditions for Implementation:
1.  **DB URL Security:** DB URL must not be inherited by lane processes.
2.  **Role Verification:** `daemon doctor` must verify that the daemon role lacks `UPDATE`/`DELETE` on audit tables.
3.  **Atomic Migration:** Migration must be atomic and verified end-to-end.
4.  **Superuser Refusal:** `daemon doctor` must warn/refuse if connected as a superuser.
