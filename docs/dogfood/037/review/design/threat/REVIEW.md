---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["threat-model", "rfc-0035", "multi-repo", "test-harness", "security-boundary"]
---

author: reviewer-gemini-pro-001

# Threat-Model Review: RFC 0035 Multi-Repo Test Harness

## Summary

The synthesized multi-repo test harness plan for RFC 0035 is **ACCEPTED**. The design provides comprehensive coverage of the trust boundaries and attack surfaces introduced by RFC 0032 and RFC 0031. It prioritizes integrated exercise of production code paths (Daemon RPC, PostgreSQL substrate, per-repo SQLite coordination) over mocked isolation, ensuring that security-critical enforcement logic is verified in its final integrated form.

## Threat Surface Analysis

### 1. Capability Gating and MCP Authorization
The harness explicitly exercises the `default-deny` posture and capability scope enforcement defined in RFC 0032:
- **`test_mcp_capability_scope_e2e.py`** verifies that tokens scoped to Repo A are refused with `capability_missing` when used against Repo B.
- **Unknown Method Handling** is covered, ensuring the daemon fails closed and records a `method_unknown` audit row.
- **Audit Integrity** is verified by asserting the sha256-chained `prev_hash` continuity across both allowed and denied paths, preventing "silent" security violations or log tampering via documented interfaces.

### 2. Cross-Repo Coordination and Reconciliation
The harness addresses the consistency risks inherent in the "best-effort" coordination between the daemon's PostgreSQL DB and repo-local SQLite DBs:
- **`test_cross_repo_crash_recovery_e2e.py`** uses SIGKILL to simulate crashes during the critical `preparing` and `starting` windows. This exercises the startup reconciliation logic to ensure runs are correctly transitioned to `aborted` or `started` without leaving orphan local state.
- **One-Repo-Unreachable Simulation** (via `chmod 000`) verifies the daemon's ability to pause runs and record human-checkpoint blockers in both the daemon DB and remaining reachable repos, mitigating the risk of state desynchronization.

### 3. Per-Repo Write-Scope Enforcement
The harness exercises the runtime enforcement of job-level repository targets:
- **`test_per_repo_write_scope_e2e.py`** confirms that a job targeting Repo B is refused with `write_scope_violation` if it attempts to publish artifacts or write to paths belonging to Repo A. This mitigates the "over-eager AI agent" threat where an agent might attempt to escalate access across repository boundaries.

### 4. Operational Integrity and CI Hygiene
The harness avoids common pitfalls in distributed systems testing:
- **Deterministic Cleanup**: `MultiRepoHarness` ensures ephemeral PostgreSQL databases and scratch directories (including Unix sockets) are dropped on teardown.
- **No Port Collisions**: The use of Unix domain sockets for the daemon RPC server avoids flakiness in CI environments.
- **Production Alignment**: The harness boots a real daemon instance and registers real repositories, ensuring that the tests exercise the actual production boundary code rather than a parallel mock implementation.

## Trust Boundaries Evaluated

| Boundary | Enforcement Mechanism | Harness Verification |
|---|---|---|
| **Client ↔ Daemon** | Capability Token / RPC Auth | `test_mcp_capability_scope_e2e.py` |
| **Daemon ↔ Repository** | RPC Handshake / SQLite Back-ref | `test_cross_repo_prepare_e2e.py` |
| **Job ↔ Worktree** | Write-Scope Path Resolution | `test_per_repo_write_scope_e2e.py` |
| **Run ↔ Audit Log** | Sha256 Audit Chain | `test_mcp_capability_scope_e2e.py` |

## Out of Scope Acknowledgement

As per RFC 0031, this review acknowledges that resistance to a **malicious local-root attacker** remains OUT OF SCOPE. The harness is designed to defend against over-eager AI agents and operator-mistake "footguns" acting through documented interfaces.

## Verdict Rationale

The plan is accepted because it does not just test "happy paths"; it proactively simulates failures (SIGKILL, unreachable filesystems, scope violations) that match the high-risk surfaces of the cross-repo architecture. The inclusion of audit chain verification ensures that the security provenance is as durable as the functional state.
