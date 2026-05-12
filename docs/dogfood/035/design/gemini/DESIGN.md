# RFC 0032 Implementation Design: Cross-Repo and MCP Mutation

**Author:** designer-gemini-pro-001
**Status:** draft
**Date:** 2026-05-12
**Context:** [`RFC 0032`](../../../rfcs/0032-cross-repo-workflows-and-mcp-mutation-capabilities.md), [`RFC 0028`](../../../rfcs/0028-long-running-daemon-and-multi-repository-control-plane.md), [`RFC 0030`](../../../rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md), [`RFC 0031`](../../../rfcs/0031-daemon-owned-supervision-and-sealed-apply-boundary.md)

This document details the implementation design for RFC 0032, focusing on the daemon V2 substrate (PostgreSQL, RPC) and the expansion of MCP capabilities to support multi-repository mutation.

## 1. Cross-Platform Reality

RFC 0032 requires "cross-platform reality," acknowledging that an operator may interact with the daemon from different environments (macOS, Linux, Windows/WSL) where absolute paths to the same physical repository differ.

### 1.1 Multi-Path Repository Mapping

The daemon registry (PostgreSQL) `repositories` table currently stores a single `repo_root` absolute path. We will extend this to support per-client or per-platform path overrides.

**Schema Change (`daemon_pg`):**

```sql
-- New table to support per-platform repository roots
CREATE TABLE repository_paths (
    repository_id TEXT REFERENCES repositories(repository_id),
    platform_id TEXT, -- e.g., 'linux', 'darwin', 'wsl', 'windows'
    repo_root TEXT NOT NULL,
    PRIMARY KEY (repository_id, platform_id)
);
```

The daemon will use the `platform_id` (determined from the client's RPC handshake or environment) to resolve the correct `repo_root`. If no platform-specific path exists, it falls back to the default `repo_root` in the `repositories` table.

### 1.3 WSL and Windows Interop

Windows-via-WSL presents a unique challenge where the same file system is accessed via different root paths (e.g., `C:\Users\...` vs `/mnt/c/Users/...`).

- **Automatic Mapping**: The daemon will attempt to auto-resolve `C:` to `/mnt/c` (and vice-versa) when a client platform is identified as `wsl`.
- **Identity Marker**: A `.striatum/identity` file containing a UUID will be used as a secondary identity check when inodes are unreliable (e.g., across the 9p mount in WSL).

### 1.4 Identity Persistence

Repository identity remains anchored in the `repo_identity` (inode-based string: `inode:<dev>:<ino>:state:<dev>:<ino>`). While inodes are not stable across different platforms (e.g., if the repo is on a network mount or shared between macOS and WSL), the `repository_id` serves as the stable logical identifier within the Striatum control plane.

When an operator adds a repository from a new platform, the daemon will:
1. Canonicalize the new path.
2. Check if the `repository_id` is already known via the `.striatum/identity` marker.
3. If the path differs but the logical identity is the same, it adds a row to `repository_paths`.

## 2. Per-Repo Failure Isolation

In a cross-repo workflow, one repository becoming unavailable (e.g., disk unmounted, network disconnected) must not crash the entire run or corrupt the state of other repositories.

### 2.1 The `waiting_repo_unavailable` State

When the daemon attempts to access a repository (e.g., to claim a job or publish an artifact) and finds the path missing or the `repo_identity` changed:

1. The affected job transitions to `state = 'blocked'`.
2. A blocker of kind `repo_unavailable` is inserted.
3. The run transitions to a new intermediate state `paused` (if not already there).
4. The daemon emits a `repo.unavailable` event.

### 2.2 Recovery Flow

1. The operator remediates the repository access (e.g., remounting).
2. The operator runs `striatum repo refresh --repo-id <id>`.
3. The daemon re-validates the path and identity.
4. If valid, it resolves the `repo_unavailable` blockers and returns the affected jobs to `queued` or `running` (if a lease was held).

## 3. MCP Capability Tokens with Short Expiries

Mutation capabilities (`write`, `apply`, `admin`) are high-risk. We will implement short-lived tokens to minimize the window of exposure.

### 3.1 Token Expiry Mechanism

**Schema Change (`daemon_pg`):**

```sql
ALTER TABLE client_capabilities ADD COLUMN expires_at TIMESTAMP WITH TIME ZONE;
```

**Implementation Details:**

- `striatum capability grant` will accept an optional `--ttl <seconds>` flag (defaulting to 3600/1 hour for mutation caps).
- The `RpcAuthContext` in `src/striatum/daemon_rpc/capability.py` will verify `expires_at > now()`.
- Expired tokens are treated as if the capability was never granted (403 Forbidden).

### 3.2 Automated Revocation

The daemon's background sweep process will periodically delete expired capability rows to keep the registry clean.

## 4. MCP `tools/call` Audit Pattern

Every tool invocation via the MCP wrapper must be recorded in the daemon audit log, ensuring a transparent record of all AI-driven mutations.

### 4.1 The Audit Envelope

Each `tools/call` request that routes through the daemon RPC will produce an audit row containing:

- `request_id`: Client-supplied ID.
- `client_id`: Identified from the token.
- `method`: `tools/call`.
- `params_hash`: HMAC-SHA256 of the tool name and arguments.
- `repository_id`: Scoped repository (if applicable).
- `decision`: `authorized` or `denied`.
- `timestamp`: UTC.

**Implementation:**
The `DaemonRpcRouter` will wrap the tool execution in a `BEGIN ... COMMIT` block that includes the audit insertion. We will NOT store the full tool arguments in the audit log to avoid leaking sensitive data (per D028), but the `params_hash` allows for later verification against the request log (which is operator-only and separate).

## 5. Operator UX for Capability Management

Operators need clear, ergonomic commands to manage the trust boundary.

### 5.1 Granting Capabilities

```bash
# Grant write capability to a client for a specific repository
striatum capability grant \
    --client-id <id> \
    --capability write \
    --repo-id <id> \
    --ttl 3600

# Grant global read capability
striatum capability grant \
    --client-id <id> \
    --capability read
```

### 5.2 Revoking Capabilities

```bash
# Revoke a specific grant
striatum capability revoke --grant-id <id>

# Revoke all capabilities for a client
striatum capability revoke --client-id <id> --all
```

### 5.3 Inspection

```bash
# List active capabilities
striatum capability list [--client-id <id>] [--repo-id <id>]
```

### 5.4 UX for Cross-Repo Workflows

When a workflow requires capabilities that the current session lacks:

1. `run prepare` or `claim-next` will surface a "Missing Capability" error.
2. The error message will provide the exact `striatum capability grant` command needed to proceed.
3. For `apply` tokens, the UI/CLI will emphasize the repo-scoped nature of the grant:
   *"This job requires 'apply' authority for repository 'striatum-core'. Run the following to grant it for 15 minutes:"*

## 6. Security Considerations

- **HMAC for Token Storage**: Tokens are never stored in plaintext.
- **Fail-Closed Authorization**: Lack of an explicit, unexpired grant always results in denial.
- **Audit Immutability**: Use PostgreSQL triggers or daemon-level hash chaining to detect tampering with audit rows.
- **Identity Honesty**: The `repo_identity` check is the primary defense against path-spoofing attacks.
