---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0039", "go-daemon", "adversarial", "race_condition", "supply_chain"]
---

author: reviewer-unknown-model-001

# Build Review Gemini Adversarial: Go Daemon V1.5 (dogfood-047)

This review evaluates the adversarial posture of the Go daemon implementation as of RFC 0039 V1.5, specifically focusing on race conditions, supply chain risks, and trust boundaries.

## Trust Boundaries & Attack Surfaces

The V1.5 deltas significantly reshape the daemon's trust boundaries compared to the initial Phase 1 build.

### TB-1: The External Supply Chain (New)
The transition to a pure-Go PostgreSQL driver (`pgx/v5`) introduces the first third-party runtime dependencies to the Go daemon core.
- **Attack Surface:** Upstream module compromise. The daemon now depends on `github.com/jackc/pgx/v5` and five indirect modules.
- **Mitigation:** `pgx` is a de-facto standard in the Go ecosystem. The build remains statically linked, preserving the single-binary distribution model.
- **Finding (Supply Chain):** The shift from `psql` (ambient system dependency) to `pgx` (build-time dependency) moves the risk from the operator's environment to the build pipeline. `go.sum` integrity is now a critical security control for the daemon.

### TB-2: Database Transactional Integrity (Race Conditions)
The audit chain and migration paths are the primary shared resources prone to race conditions.
- **Attack Surface:** Audit chain "forking" or "bifurcation" under concurrent RPC requests.
- **Mitigation:** F4 implements a `READ COMMITTED` transaction that uses `SELECT ... FOR UPDATE` on the singleton `striatumd.audit_chain_head` row. This serializes the "read previous hash -> insert new row -> update head" sequence.
- **Finding (Race Condition):** The row-level lock on the chain head effectively mitigates the race condition identified in previous reviews. However, the implementation must ensure that the `db.TxRunner` correctly manages the transaction lifecycle to prevent leaked locks or partial appends in case of driver-level failures.

### TB-3: Local Filesystem Migration Integrity
The daemon embeds SQL migrations but also allows verification against disk.
- **Attack Surface:** Migration drift or local SQL injection.
- **Mitigation:** F2 introduces `--migrations-sha-source`, which compares embedded migration hashes against the source SQL files on disk at startup.
- **Finding:** This provides a strong integrity signal for developers and operators, mitigating the risk of accidental drift or malicious tampering with the local source tree before the binary is built/run.

### TB-4: RPC Authorization Boundary
- **Attack Surface:** Unauthenticated RPC access (the previous "AllowAll" posture).
- **Mitigation:** F1 wires a PostgreSQL-backed authorizer that performs `subtle.ConstantTimeCompare` on Python-issued capability tokens.
- **Finding:** This closes the critical security hole where local socket access was equivalent to full daemon authority. The authorization logic now correctly mirrors the Python daemon's enforcement (including repository scope wildcards).

## Adversarial Findings

### [MEDIUM] Supply Chain: Dependency Growth
The introduction of `pgx` is justified for correctness (F4, F5), but it marks the end of the "standard library only" era for the Go daemon.
- **Risk:** Future steps (mutating verbs, PTY handling) may pull in more dependencies (e.g., `creack/pty`).
- **Recommendation:** Establish a "dependency budget" or a rigorous review process for any new entries in `go.mod`. Ensure that `go mod verify` is part of the CI pipeline for the `CORE=go` matrix.

### [LOW] Migration Advisory Lock Persistence
While F5 moves to a native driver, the RFC does not explicitly detail the fix for the "broken advisory lock" (previously broken because of `psql` subprocess exits).
- **Risk:** If the migration runner uses a connection pool without pinning the advisory lock to a single session, the lock may be released prematurely or leaked.
- **Recommendation:** Verify that the migration logic uses a dedicated, non-pooled connection for the duration of the migration lock, or that the advisory lock is held within the same `pgx` transaction that applies the DDL.

## Verdict

**accept_with_findings**

The V1.5 deltas (dogfood-047) successfully address the most critical adversarial risks identified in the initial Phase 1 reviews. The move to `pgx` and transactional audit logging effectively mitigates SQL injection and race condition vectors. The primary remaining risk is the newly introduced supply chain surface, which is managed through the use of established libraries and static linking.
