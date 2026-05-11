---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["security", "threat-model", "daemon", "rfc-0028"]
---

# Threat-Model Review: RFC 0028 Long-Running Daemon

author: reviewer-gemini-pro-001
date: 2026-05-11
status: completed

## Executive Summary

The RFC 0028 daemon design introduces a resident control plane (`striatumd`) that significantly expands Striatum's capabilities but also concentrates authority and broadens the attack surface. The design correctly identifies the primary trust boundaries—OS user identity and loopback network—and proposes a capability-based authorization model for clients.

This review **Accepts with Findings**. The design is architecturally sound for its stated "local-first" goal, but several residual risks require explicit acknowledgment or mitigation before moving from prototype to production, particularly regarding same-UID peer process isolation and registry-DB tampering.

---

## 1. Trust Boundaries and Attack Surfaces

The daemon introduces four primary boundaries:
1.  **OS User / UID Boundary**: The primary defense. `striatumd` communicates via a Unix-domain socket with `0o600` permissions.
2.  **Loopback Boundary**: HTTP and WebSocket/SSE services are pinned to `127.0.0.1`.
3.  **Client Capability Boundary**: Connected clients (CLI, MCP, Web UI) are granted specific read/mutation permissions.
4.  **Repository Boundary**: The daemon manages multiple target repositories and must enforce isolation between them (e.g., preventing path traversal).

---

## 2. Adversary Model Analysis

### 2.1 Hostile Local Client
*   **Threat**: A rogue process (e.g., malicious browser extension, untrusted script) running as the same OS user attempts to interact with the daemon.
*   **Assessment**: **Mitigated**. The Unix socket permissions and loopback-only HTTP with constant-time token comparison provide strong baseline defenses.
*   **Finding**: The design must specify a secure storage mechanism for client capability tokens (e.g., OS keyring) to prevent "token-theft-by-grep" by peer processes.

### 2.2 Prompt-Injected MCP Agent
*   **Threat**: An LLM agent is manipulated via prompt injection to call daemon tools it wasn't intended to use.
*   **Assessment**: **Partially Mitigated**. The design defaults MCP tools to read-only.
*   **Finding**: The risk increases substantially if the operator grants mutation capabilities (e.g., `run_start`, `publish_artifact`) to an MCP client. This capability should be "opt-in per tool" rather than "one global flag," and the UI should warn when a client is granted high-risk capabilities.

### 2.3 Malicious Peer Process (Same UID)
*   **Threat**: Another process running under the same UID (e.g., a developer's browser, another CLI tool) directly manipulates the daemon's SQLite registry or the repository-local `.striatum/` state.
*   **Assessment**: **Unmitigated (Inherent)**. This is the fundamental trade-off of the "local-first" single-user model. A malicious peer process can read/write the Unix socket and the state files directly.
*   **Finding**: While this is "out of scope" for local-first security, the daemon should implement **integrity checks** on startup (e.g., validating DB hashes or signing the registry) to detect offline tampering by peer processes.

### 2.4 Registry/Registry-DB Tamper
*   **Threat**: Direct SQL manipulation of the central `striatumd.sqlite3` to elevate capabilities, register malicious repositories, or spoof lane attestation (by editing PIDs).
*   **Assessment**: **High Risk**. The registry concentrates all multi-repo authority.
*   **Finding**: The central registry should be the **most protected file** in the Striatum ecosystem. Access should be mediated exclusively through the daemon process, and the schema should include internal consistency checks (e.g., triggers that prevent manual identity changes).

### 2.5 Upgrade-Time Downgrade Attacks
*   **Threat**: An attacker replaces the `striatum` binary with an older version that lacks security enforcement (e.g., pre-RFC 0026/0027).
*   **Assessment**: **Mitigated**. Striatum's existing `PRAGMA user_version` and migration-registry system (D047) prevent older binaries from opening newer databases (Exit Code 9).

### 2.6 Cross-Platform Containment Differences
*   **Threat**: Security guarantees (like `sealed_patch`) are broken on platforms with weaker isolation primitives (macOS/Windows vs. Linux).
*   **Assessment**: **Mitigated**. RFC 0028 and the design synthesis (Dogfood-030) explicitly state that `sealed_patch` runs must refuse to start on unsupported platforms.
*   **Finding**: The "Authority Probe" mentioned in the synthesis must be robust and fail-closed.

### 2.7 Crashes that Strand Supervised Processes
*   **Threat**: A daemon crash orphans agent processes, potentially leaving them in an inconsistent or vulnerable state.
*   **Assessment**: **Operational Risk**.
*   **Mitigation**: The proposed "Supervisor Liveness Probes" and `striatum doctor` orphan-detection (D049) are sufficient.

---

## 3. Findings and Recommendations

### Finding 1: Capability Token Storage
The RFC mentions "Constant-time token compare" but not where tokens are stored.
*   **Recommendation**: Implement OS-keyring integration for client tokens. Do not store plain-text tokens in `.striatum/` or environment variables.

### Finding 2: Identity Forgery via PID Collision
Lane attestation (RFC 0026) relies on PID liveness. On systems with fast PID wraparound, a malicious process could attempt to capture a "blessed" PID.
*   **Recommendation**: Use `pid_start_time` (recorded in Migration v12) as part of the attestation token on all supported platforms to ensure PID uniqueness across reboots and wraparounds.

### Finding 3: Central Registry as a High-Value Target
The central registry stores repository paths, capabilities, and potentially signing keys (for `sealed_patch`).
*   **Recommendation**: If `sealed_patch` signing keys are resident in the daemon, they must be stored in an encrypted vault/keyring, not directly in the SQLite registry.

### Finding 4: Operator Identity Ambiguity
The RFC mentions "Operator identity mapping" for multi-user local mode.
*   **Recommendation**: Clarify how "Operator Tenants" are mapped to OS identities. For V1, a "One-Daemon-Per-UID" model is safer than a multi-user daemon.

---

## 4. Verdict

**Accept with Findings.**

The architectural direction is consistent with Striatum's local-first, deterministic principles. The move to a daemon improves the product's orchestration power without fundamentally weakening the provenance model, provided the recommended mitigations for token storage and registry integrity are implemented.
