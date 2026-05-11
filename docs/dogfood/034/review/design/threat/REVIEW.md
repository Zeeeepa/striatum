---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["threat_model", "rfc-0030", "rfc-0031", "daemon", "design"]
---

author: reviewer-gemini-pro-001

# Threat-Model Review: RFC 0030 + RFC 0031 (Daemon V2)

The design synthesis for the paired RFC 0030 (Daemon RPC) and RFC 0031 (Daemon-owned Supervision & Sealed-Apply) architecture is accepted. The plan successfully enumerates the trust boundaries introduced by a long-running daemon and provides a robust set of mitigations centered on explicit capability gating, audit-chain integrity, and process-identity verification.

## Trust Boundary Analysis

### 1. Daemon Trust Boundary & Version Skew
The synthesis correctly identifies the daemon as the mediator for all mutating requests. The `daemon.hello` / `daemon.welcome` handshake establishes a clear version-skew protocol that refuses incompatible clients (Exit Code 10) and prevents silent downgrades to direct repo-local mode. This closes the "frictionless bypass" surface where a client might otherwise evade daemon-enforced policies.

### 2. Capability Gating & Audit Integrity
All RPC routes are mapped to a granular capability vocabulary (`read`, `write`, `review`, `claim`, `apply`, `admin`). The use of PostgreSQL (RFC 0033) with serializable isolation and role-based append-only enforcement ensures that every request—including denied attempts—is recorded in the immutable audit chain. The closed vocabulary for `denial_reason` provides high-signal observability for security events.

### 3. Sealed-Apply Authority
The synthesis maintains the critical "AI guardrail" framing required by the RFC 0031 threat model. It correctly acknowledges that while the daemon owns the signing key and apply gate, it does not defend against a malicious local operator with root/DB access. 
- **Mitigations**: The plan specifies hard refusal paths for base-tree drift, patch digest mismatches, and missing reviewer verdicts.
- **Custody**: Signing keys are stored in OS keyrings or `0600` files, and the daemon refuses sealed-mode operations if a key cannot be securely loaded.

### 4. Supervisor Ownership & PID Reuse
Moving supervisor ownership into the daemon process is a significant security improvement. The use of `pid` + `pid_start_time` (derived from `/proc/<pid>/stat`) for reattaching supervisors after a daemon crash effectively mitigates PID-reuse attacks and ensures lane attestation (RFC 0026) remains honest across daemon restarts.

### 5. MCP Mutation Safety
The synthesis correctly rejects a global `--allow-mutations` flag for MCP. Instead, it treats MCP clients as untrusted entities that must hold explicit capability tokens. The prompt-injection mitigation is built on the principle of "tokens, not trust claims," where the `tools/list` surface is dynamically filtered based on the token's effective capability set.

## Findings

The following observations are recorded as non-blocking architectural notes:

- **Local Trust Root**: As noted in the synthesis, the daemon remains a local process under operator control. The provenance guarantees are "honest" and "attested" within the local environment, but they are not cryptographic proofs for third-party consumption.
- **Windows Support**: The honest deferral of Windows-native daemon support (specifically Unix socket/Named Pipe parity) is accepted, as the current focus is on securing the Linux/macOS architecture first.

## Verdict: Accept

The design is threat-model complete for the V2 scope. It establishes the necessary enforcement points to support sealed-patch provenance while avoiding the trap of overclaiming security against out-of-scope adversaries.
