---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "medium"
tags: ["threat_model", "rfc-0043", "v1", "build"]
---

author: reviewer-unknown-model-001

# Adversarial Threat Model Review: RFC 0043 Postgres Substrate Migration

- **Status:** Finding
- **Date:** 2026-05-13
- **Reviewer:** gemini (adversarial lane)
- **Artifacts:** `src/striatum/daemon_pg/repo_local_migration.py`, `src/striatum/cli/daemon_required.py`, `src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py`

## Executive Summary

The migration of repo-local workflow state to PostgreSQL (RFC 0043) establishes a critical trust boundary: the daemon becomes the single authoritative writer. While the core migration logic utilizes strong `SERIALIZABLE` isolation and audit-chain re-anchoring, this review identified a persistence gap during crash recovery and a significant "escape path" in the CLI enforcement logic that could allow silent SQLite fallback. Furthermore, the `migrate-repo-local` command is implemented in the backend but not yet wired into the CLI surface.

## 1. Trust Boundaries & Attack Surfaces

### 1.1 The Single-Writer Invariant
RFC 0043 mandates that the daemon is the only process allowed to mutate workflow state.
- **Surface:** Any path that allows the CLI to write directly to SQLite bypasses the daemon's capability checks and audit logging.
- **Trust Boundary:** The transition from CLI-local SQLite to Daemon-remote Postgres.

### 1.2 `migrate-repo-local` Idempotency
- **Boundary:** Registration of a repository in the daemon's global namespace and cutover of its local state.
- **Attack Surface:** Concurrent invocations of `migrate-repo-local` against the same repository.

## 2. Adversarial Findings

### 2.1 [CRITICAL] Partial-Migrate Crash Recovery (Tombstone Persistence Gap)
The `_migrate_full` function in `src/striatum/daemon_pg/repo_local_migration.py` performs the following sequence:
1. Commit PG transaction (includes data copy and checkpoint insertion).
2. Tombstone/Delete SQLite file.

**Threat:** If the process crashes between step 1 and step 2, the data exists in Postgres, but the `state.sqlite3` file remains on disk and is not read-only.
- On re-run, `migrate_repo_local` detects the checkpoint and returns `already_migrated: True` (line 204), but it **does not re-attempt the tombstone/delete step**.
- The repository is left with an active SQLite file. An older client (pre-RFC 0043) or a misconfigured new client will continue to use/mutate this SQLite file, leading to "split-brain" where mutations are invisible to the daemon.
- **Recommendation:** `migrate_repo_local` must ensure the tombstone/delete state is applied if a checkpoint is found but the local file state is inconsistent.

### 2.2 [MAJOR] `STRIATUM_DAEMON_REQUIRED` Escape Path
In `src/striatum/cli/daemon_required.py`, the `resolve_requirement` function (line 72) only returns a enforcement requirement if `STRIATUM_DAEMON_REQUIRED == "1"`.

**Threat:** If an operator fails to set this environment variable, the CLI silently falls back to the legacy SQLite-backed path for all operations. This contradicts the "Daemon as hard prerequisite" mandate of RFC 0043 §3. It creates a massive escape path where the security benefits of the daemon (centralized audit, capability gating) are bypassed by simply not setting an environment variable.
- **Recommendation:** Retirement of the SQLite fallback path should be the default. The environment variable should ideally be for *opting out* (if even allowed) or removed entirely once the migration is mandatory.

### 2.3 [MODERATE] `migrate-repo-local` CLI Wiring Gap
The `migrate-repo-local` subcommand is missing from `src/striatum/cli/parser.py` and its dispatch is not wired in `src/striatum/cli/dispatch.py::_dispatch_daemon`.

**Finding:** While the logic is implemented in `repo_local_migration.py` and `daemon.py`, it is currently unreachable via the CLI.
- **Impact:** An operator attempting to follow the remediation hint for exit code 12 (repo not migrated) will find the command doesn't exist.

### 2.4 [INFO] Idempotency under Concurrent Invocations
The use of `BEGIN ISOLATION LEVEL SERIALIZABLE` and unique constraints on `striatumd.repositories` (repo_root, repo_identity) and `striatumd.repo_migrations` (repository_id, substrate pair) correctly mitigates race conditions during concurrent migration attempts. The second attempt will either find the existing checkpoint or fail with a serialization error/unique constraint violation.

### 2.5 [INFO] `--confirm-delete` + `--keep-sqlite-readonly` Flag Conflict
The logic in `_verify_delete_options` and `_tombstone_or_delete_state_db` correctly prioritizes the safe `--keep-sqlite-readonly` default. If an operator provides conflicting flags, the safe path (rename to `.tombstone`) is taken.

## 3. Method Registry Exhaustiveness
A review of `src/striatum/cli/mutations.py` vs `src/striatum/daemon_rpc/registry.py` confirms that the RFC 0043 method expansion is exhaustive. Every mutation function in the CLI has a corresponding registered method with appropriate capability requirements.

## 4. Verdict & Recommendations

The implementation of the migration logic is robust in its transactional integrity, but the lifecycle management of the local SQLite file and the CLI enforcement posture have gaps.

**Mandatory Remediation:**
1. **Fix Crash Recovery:** Update `migrate_repo_local` to always attempt the tombstone/delete step if a checkpoint is found but the local file still exists.
2. **Wire Parser:** Add the `migrate-repo-local` subparser to `parser.py` and delegate `_dispatch_daemon` to `striatum.cli.daemon:dispatch_daemon`.
3. **Tighten Enforcement:** Evaluate making `STRIATUM_DAEMON_REQUIRED=1` the default or removing the fallback logic in `daemon_required.py`.
