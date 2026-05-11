---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["rfc-0028", "daemon", "v1", "security"]
---

# Security Review: RFC 0028 V1 Daemon Implementation

author: reviewer-gemini-pro-002

**Reviewer:** gemini-pro-002
**Status:** ACCEPTED
**Posture:** security

## Objective

Review the V1 daemon implementation for socket permissions, capability authorization correctness, audit log completeness, mutation default-deny behavior, refusal of symlink/path-traversal at repo registration, constant-time token compare, and absence of raw `.striatum/state.sqlite3` write paths through daemon clients.

## Findings

### 1. Socket Permissions
The daemon implementation in `src/striatum/daemon.py` correctly enforces owner-only permissions for its Unix-domain socket.
- In `run_daemon_foreground`, `os.chmod(socket_path(), 0o600)` is called immediately after binding (L821).
- The runtime directory itself is also protected with `0o700` in `_ensure_private_dir` (L110).
- Registry and token files are similarly protected with `0o600` (L144, L297).

### 2. Capability Authorization Correctness
Authorization is implemented in `_authorize` (L306) and enforced via `_require_auth` (L431).
- Supported capabilities are `read` and `admin`.
- Multi-repository scoping is correctly handled: a capability can be global (NULL `repository_id`) or scoped to a specific repository.
- Tokens are validated against `revoked_at` and `expires_at` timestamps.

### 3. Audit Log Completeness
The audit log in `src/striatum/daemon.py` is robust and follows the hash-chain design proposed in the RFC.
- `audit_request` (L353) captures all essential fields: timestamp, client, repository, command, result, transport, and payload SHA-256.
- Rows are append-only, enforced by SQLite triggers `audit_no_update` and `audit_no_delete` (L204-L209).
- Hash-chaining is implemented via `previous_hash` and `row_hash` (L382-L395).
- `daemon_doctor_records` includes a check for audit chain integrity (L575, L598).

### 4. Mutation Default-Deny Behavior
The daemon follows a strict default-deny posture for mutations.
- `DaemonRpcServer` in `src/striatum/mcp.py` (L460) only supports `initialize`, `tools/list` (empty), `resources/list`, and `resources/read`.
- It does NOT support `tools/call`, effectively disabling all CLI-style mutations via MCP.
- Client-facing endpoints in `daemon.py` explicitly call `_require_auth` with the appropriate capability.

### 5. Refusal of Symlink/Path-Traversal at Repo Registration
Conservative path validation is implemented in `_canonical_repo` (L231).
- It refuses paths containing `..` components.
- It refuses registration of symlink paths or any path where a parent is a symlink (L237-L241).
- It explicitly refuses symlinks for the state database itself (L243).

### 6. Constant-Time Token Compare
The implementation uses `hmac.compare_digest` for all security-sensitive comparisons.
- Token hash comparison in `_authorize` (L327).
- Audit row hash verification in `_audit_chain_records` (L615).

### 7. Absence of Raw Write Paths
No endpoints were found that allow clients to perform raw SQLite writes or arbitrary mutations to the repository state.
- `DaemonRpcServer` is restricted to read-only resources.
- The `striatum/invoke` method, which allows raw command execution in `LocalRpcServer`, is absent from `DaemonRpcServer`.
- Repository mutations like `repo_add` and `repo_remove` are managed exclusively by the daemon registry logic.

## Verdict

The implementation adheres to the security requirements defined in RFC 0028 and the V1 acceptance criteria. All mandated security controls are present, conservatively implemented, and correctly integrated into the daemon's architecture.

**Verdict: ACCEPTED**
