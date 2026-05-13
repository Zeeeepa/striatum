---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0043", "v1.6", "build", "adversarial"]
---

author: reviewer-unknown-model-001

# Build Review — RFC 0043 V1.6 substrate hardening

Adversarial threat-modeling review of the substrate hardening build (Phase V1.6).

## Trust Boundaries & Attack Surfaces

The build introduces or modifies the following boundaries:

1.  **Daemon-Required Gate:** The CLI now enforces daemon connectivity by default, with an opt-out mechanism tied to the pair of `STRIATUM_DAEMON_REQUIRED=0` and `STRIATUM_TEST_HARNESS=1`.
2.  **Migration Sentinel Boundary:** A filesystem-based sentinel (`.migrated`) and tombstone (`.tombstone`) are used to signal state transition from SQLite to Postgres.
3.  **Migration Lock Boundary:** A sidecar lock file (`.migrate.lock`) is used to prevent concurrent migration attempts.

## Findings

### A1: Silent Split-Brain during Crashed Migration [CRITICAL]
The hardening of `db.connect()` intended to prevent split-brain is bypassed if the `state.sqlite3` file still exists [src/striatum/db.py:146].

The check for the `.migrated` sentinel and `.tombstone` only occurs when `target.exists()` is False. If `migrate-repo-local` crashes *after* the Postgres commit and sentinel write, but *before* the SQLite finalization, the `state.sqlite3` file remains on disk. Subsequent CLI or daemon commands will connect to the stale SQLite file without error, even though the authoritative state has moved to Postgres.

**Impact:** Data divergence and silent split-brain between SQLite and Postgres substrates.
**Remediation:** `db.connect()` must check for the `.migrated` sentinel even if the SQLite file exists, and refuse connection if the sentinel is present (pointing the user to resume migration).

### A2: Inadequate Locking during Migration [HIGH]
The migration lock introduced in `_exclusive_migrate_lock` [src/striatum/daemon_pg/repo_local_migration.py:34] uses a sidecar lock file (`.migrate.lock`).

While this prevents concurrent `migrate-repo-local` commands, it does NOT prevent the daemon or other CLI instances from writing to the source `state.sqlite3` file while the migration is reading it. Since the migration opens the source in read-only mode [src/striatum/daemon_pg/repo_local_migration.py:355], concurrent writes are not blocked by the OS, leading to potential data loss for any events written to SQLite after the migration scan began.

**Impact:** Data loss during the migration cutover window.
**Remediation:** Use an exclusive lock on the actual `state.sqlite3` file (or a mechanism that the daemon/CLI also respects) to ensure quiescence during the copy.

### A3: Usability Regression — Migrated Repos effectively disabled [HIGH]
The build hardens the boundary by refusing to open tombstones, but the implementer explicitly deferred the daemon-side business logic port to V2.0 [docs/dogfood/052/build/HANDOFF.md:46].

Because `DaemonRpcRouter` still delegates to `striatum.api.invoke` (which uses `db.connect`), all single-repo operations (e.g., `status`, `summary`, `list artifacts`) on a migrated repository will fail with exit code 12. The remediation message "use the daemon socket directly" [src/striatum/db.py:155] is currently a dead end for these commands.

**Impact:** Migrating a repository effectively renders it unusable for standard CLI/Daemon operations until V2.0.
**Remediation:** Either ship enough of the V2.0 substrate flip to support read-only commands via PG, or relax the `db.connect` tombstone refusal for read-only connections if the daemon is the caller.

### A4: Trivial Bypass of Daemon-Required Mandate [MEDIUM]
The narrowing of the `STRIATUM_DAEMON_REQUIRED=0` escape hatch [src/striatum/cli/daemon_required.py:70] is trivially bypassed by an operator setting `STRIATUM_TEST_HARNESS=1`.

While intended for the test suite, this remains a functional production backdoor. If the goal is absolute enforcement, the test-harness marker should be more difficult to spoof or tied to a non-production build configuration.

### A5: Platform Incompatibility [LOW]
The `fcntl` module is imported at the top-level of `src/striatum/daemon_pg/repo_local_migration.py`. This will cause an `ImportError` on Windows systems during module discovery, even if the migration command is not executed.

## Verdict

**needs_revision**

The critical split-brain risk (A1) and high-severity locking inadequacy (A2) must be addressed before this hardening can be considered effective. The usability regression (A3) makes the migration path effectively "broken" for current users, which contradicts the "substrate completion" goal.
