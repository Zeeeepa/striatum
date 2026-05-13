---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["rfc", "threat-model", "postgres", "migration", "concurrency"]
---
author: reviewer-gemini-pro-002

# Threat Model Review: RFC 0042 (Repo-Local State to Postgres)

## Trust Boundaries
RFC 0042 shifts the primary authoritative state from the repository filesystem (`.striatum/state.sqlite3`) to the daemon-owned Postgres database (`striatumd` schema). 

1. **OS User Boundary (D083):** The daemon continues to assume a single OS user. Any process reaching the daemon Unix socket is treated as authorized by that user.
2. **Repository Isolation Boundary:** Logical isolation between repositories is enforced by `repository_id` in Postgres. This is a "multi-tenant" model where tenants are repositories, not users.
3. **Migration Window:** The cutover from SQLite to PG creates a transient state where two potential "authoritative" sources exist.

## Attack Surface Analysis

### 1. Repository Isolation & Capability Gating
While D083 assumes a single user, the shift to a shared Postgres schema for multiple repositories increases the risk of cross-repo data leaks.
- **Threat:** Manipulating `repository_id` in RPC calls or exploiting a bug in the daemon's query generation to access Repo B data using a Repo A capability token.
- **Mitigation:** The RFC mandates composite primary keys and foreign keys that include `repository_id`. This is a strong defense: a query that forgets the repo filter will fail on the join or primary key lookup rather than returning an unrelated row.
- **Finding:** The daemon's RPC dispatcher MUST verify the `repository_id` in every request against the scoped capability token *before* it reaches the DB layer.

### 2. Partial Migration & "Phantom" State
The migration process is multi-transactional (one transaction per table).
- **Threat:** If the daemon or Postgres fails mid-migration, the Postgres DB may contain a partial set of repository rows.
- **Risk:** RFC 0042 defends against this with Error 15 (`repo_unmigrated`) and Error 17 (`pg_repo_drift_detected`). If the cutover sentinel isn't in SQLite, the daemon refuses to use Postgres for that repo.
- **Finding:** To further harden this, the `striatumd.repo_state_migrations` row should be inserted in the *final* transaction along with the summary audit row. The presence of this row should be the definitive "Postgres is authoritative" signal for the daemon, and it must only be present if all tables migrated successfully.

### 3. Audit Chain Continuity (v1 to v2 transition)
The audit log hash format version bumps to 2.
- **Threat:** An adversary with direct DB access could attempt to inject or modify rows at the format transition point.
- **Mitigation:** Continuous linkage via `previous_hash` is preserved. `daemon doctor` performs an integrity walk.
- **Finding:** The `previous_hash` check in `daemon doctor` must be verified to handle the format-version switch correctly, ensuring the payload for version 1 rows is recomputed using version 1 logic and vice-versa.

### 4. Rollback & Desynchronization
Rollback is "break-glass" and manual.
- **Threat:** Rolling back to SQLite after performing work in Postgres results in state loss (provenance gap).
- **Risk:** This is accepted by the RFC but creates a risk for "State Manipulation" if an operator reverts to an older SQLite state to "undo" a recorded verdict or artifact in Postgres.
- **Finding:** Since SQLite remains on disk (optionally as a tombstone), the system must ensure that once a cutover sentinel is written, no Python CLI version (even pre-RFC-0042 versions) can accidentally mutate it. The sentinel row in `schema_meta` is the correct mechanism, but it assumes all CLI versions respect that table.

## Verdict Summary
The shift to Postgres is architecturally sound and necessary for cross-repo coordination. The primary adversarial concern—cross-repo data leakage—is well-mitigated by the use of composite keys. The migration logic is robust, though the multi-transactional nature of the import requires the daemon to be strictly "Postgres-authoritative" only after the final migration row is committed.

**Verdict Intent: accept_with_findings**

**Findings:**
1. **Atomics:** Ensure the `repo_state_migrations` row and the final audit summary are committed in the same transaction as the last table import to prevent "authoritative but incomplete" states.
2. **Scoping:** Explicitly validate that all RPC methods derived from repo-local verbs (e.g., `job.claim_next`) enforce the `repository_id` from the capability token and do not rely on a user-supplied `repo_id` in the payload.
3. **Drift Detection:** `daemon doctor --check-migration` should be the standard "pre-flight" for any manual recovery or rollback action.
