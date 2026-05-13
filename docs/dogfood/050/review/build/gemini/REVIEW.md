---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0043", "v1.5", "build", "adversarial"]
---

author: reviewer-unknown-model-001

# Adversarial Build Review: RFC 0043 V1.5 (Postgres Substrate)

This review evaluates the V1.5 build against the adversarial objectives of RFC 0043, specifically focusing on the transition from repo-local SQLite to a centralized Postgres substrate.

## Executive Summary

The V1.5 build successfully implements the schema migrations, the `migrate-repo-local` tool, and the daemon-required enforcement gates. However, a critical architectural gap exists: **the business logic for single-repo operations has not been ported to the Postgres substrate.** The daemon currently delegates RPC requests back to SQLite-backed CLI logic. This creates a severe "substrate mismatch" where migrated repositories appear empty or cause split-brain conditions because the system continues to write to (and read from) fresh SQLite files instead of the migrated Postgres data.

**Verdict: NEEDS REVISION**

---

## Adversarial Findings

### A1: Server-side Silent SQLite Fallback (Substrate Mismatch)
**Severity: Critical**
**Objective:** "Any remaining silent SQLite fallback (search the cli/ tree)"

While the CLI correctly enforces the daemon requirement and checks for migration status, the daemon itself (both Python and Go cores) currently lacks the business logic to operate on the Postgres substrate for single-repo verbs. 

- **Mechanism:** `DaemonRpcRouter._route` in `src/striatum/daemon_rpc/server.py` delegates most single-repo routes (e.g., `work.block`, `run.start`) to `striatum.api.invoke`. 
- **Consequence:** `invoke` calls the standard CLI `dispatch` logic, which connects to SQLite via `striatum.db.connect`. Even if a repo has been migrated to Postgres, the daemon continues to execute logic against a repo-local SQLite file.
- **Adversarial Vector:** An operator who migrates their repo to Postgres and continues to use the daemon is actually still using SQLite. The "substrate flip" is a facade for single-repo operations.

### A2: Post-Migration Data Loss / Split-Brain
**Severity: High**
**Objective:** "Backward-compat tombstone semantics preserved"

There is a fatal interaction between the migration tombstone and the CLI's automatic database creation.

- **Mechanism:** After `migrate-repo-local` succeeds, the SQLite database is moved to `.tombstone` or deleted. The next daemon-mediated mutation calls the CLI logic, which sees the `state.sqlite3` file is missing. `striatum.db.connect` then creates a **fresh, empty SQLite database**.
- **Consequence:** All existing runs, jobs, and artifacts (now in Postgres) are ignored. The system silently starts over in a new SQLite file. The `repo_is_migrated` check in `daemon_required.py` incorrectly returns `True` if the `state.sqlite3` file is absent, allowing the CLI to proceed and create a new one.
- **Remediation:** `repo_is_migrated` must check for the existence of the repository record in the daemon's Postgres registry, and the CLI logic must be modified to use the Postgres substrate when a migration checkpoint exists.

### A3: Lack of Exclusive Locking During Migration
**Severity: Medium**
**Objective:** "Concurrent migrate-repo-local invocations"

The `migrate-repo-local` tool does not take an exclusive lock on the source SQLite file, nor does it disable the daemon's ability to write to it during the migration.

- **Mechanism:** `migrate_repo_local` opens the source database with `mode=ro`. Because `repo_is_migrated` returns `False` until the very end of the migration (when the tombstone is created), the daemon and other CLI instances can continue to perform writes to the SQLite file while it is being copied.
- **Consequence:** Any writes occurring between the start of the migration and the final tombstone/deletion are **silently lost**. While `reanchor` manifest checks provide some protection, they only detect mismatches for the specific rows copied; they do not prevent or detect concurrent writes to other parts of the database or writes that occur after the manifest calculation but before the tombstone.

---

## Objective Verification

| Objective | Status | Notes |
| :--- | :--- | :--- |
| Silent SQLite Fallback | **FAILED** | The daemon routes back to SQLite-backed logic (A1). |
| Concurrent `migrate-repo-local` | **PASSED** | Protected by `SERIALIZABLE` isolation and unique constraints on `striatumd.repositories`. |
| Rollback-on-crash atomicity | **PASSED** | The sentinel file (`.migrated`) and SHA verification in `_resume_sqlite_finalization_after_checkpoint` are robust. |
| Tombstone semantics | **DEGRADED** | Semantics are preserved (A4), but the implementation allows accidental SQLite re-init (A2). |

---

## Recommendations

1.  **Port Business Logic:** Accelerate the porting of `striatum.db` and `striatum.cli.mutations` to a substrate-agnostic layer that can target either SQLite or Postgres.
2.  **Fix Migration Gating:** Update `repo_is_migrated` to be authoritative by querying the daemon registry instead of relying on file existence. 
3.  **Implement Migration Locking:** `migrate-repo-local` should attempt to take an exclusive lock (e.g., `BEGIN EXCLUSIVE` or a file lock) on the SQLite database to prevent concurrent writes during the cutover.
4.  **Enforce Redirection:** The CLI `dispatch` logic should be updated to send all mutation requests to the daemon when `repo_is_migrated` is True, rather than attempting to execute them in-process.
