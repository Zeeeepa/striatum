# RFC 0165 Falsifying Challenge: Hydration Path Security and Same-User Token Mutation
author: falsifier-reviewer-003

## Challenge 1: Symlink TOCTOU Privilege Bridge in Lane-Owned Destination

### Claim Challenged
The proposal claims that spawn-time hydration can securely copy credentials into the lane user's directory (e.g., `~striatum-lane/.claude/.credentials.json`) by checking that destination parent components stay under the trusted lane directory and verifying owner/mode after the copy, without creating a privilege bridge or path escape.

### Concrete Counterexample (TOCTOU Symlink Attack)
1. The daemon (running as the operator user `halbritt`) initiates a launch preflight.
2. The daemon resolves and verifies `/home/striatum-lane/.claude` as a legitimate directory owned by `striatum-lane`.
3. Between the time of verification and the time the daemon writes the temp file or completes the rename, a background process running as the untrusted `striatum-lane` user (e.g., orphaned from a prior run or running concurrently) renames `/home/striatum-lane/.claude` and replaces it with a symlink pointing to `/home/halbritt/.ssh/` or another sensitive operator-owned path.
4. Because the daemon runs with the operator's privileges, the file write/rename traverses the symlink. This allows the untrusted lane user to force the daemon to write or overwrite files anywhere the operator has write access, bridging privileges.
5. Standard POSIX `O_NOFOLLOW` flags on file creation do not prevent parent directories from containing symlinks.

### Strongest Rebuttal
One could argue that the daemon can recursively check path components or use lockfiles. However, POSIX filesystems do not allow directory trees to be locked against rename by their owner (`striatum-lane`). The only standard way to write safely into a directory owned by an untrusted user is to drop privileges to that user during path resolution and file writing.

### Unanswered Gap
The proposal does not resolve this catch-22:
- If the daemon writes the credential directly with operator/daemon privileges, it is vulnerable to parent-directory symlink TOCTOU attacks.
- If the daemon drops privileges to `striatum-lane` to resolve and write safely, it cannot read the operator's private source credential `/home/halbritt/.claude/.credentials.json`.
The design needs a secure path-traversal primitive (e.g., opening parent directories with `O_PATH` and `O_NOFOLLOW` step-by-step using `openat` and checking ownership before traversal) or must delegate the write via a secure IPC boundary.

---

## Challenge 2: Same-User Collapse Mutates Operator Refresh Token

### Claim Challenged
The proposal claims that `RunAsUser == ""` (same-user collapse) can safely treat hydration as a verify-only no-op because the destination and source are the same file.

### Concrete Counterexample (OAuth Refresh Token Desynchronization)
1. A lane is spawned with same-user collapse, meaning the provider CLI (e.g. `claude`) runs as the operator user and accesses `/home/halbritt/.claude/.credentials.json` directly.
2. During the run, the provider access token expires. The provider CLI automatically performs a token refresh.
3. This refresh call succeeds, but rotates the single-use OAuth refresh token, writing the new credential back to `/home/halbritt/.claude/.credentials.json`.
4. If the operator runs a CLI command or another lane starts concurrently, it will try to use the old refresh token (which has already been used and invalidated), causing auth to fail.
5. This directly violates the non-goal: *"Do not ask a lane to refresh the operator's Claude OAuth credential."*

### Strongest Rebuttal
One might argue that same-user collapse is only a fallback for local testing where concurrency is low. However, even in a single-user setup, the running lane and the operator's own terminal shell are concurrent actors. Any token refresh by the lane immediately invalidates the operator's active CLI session refresh state, causing unexpected auth failures.

### Unanswered Gap
The proposal fails to define how same-user collapse prevents the lane's provider CLI from performing automatic token refreshes that mutate the operator's active credentials file and invalidate rotating refresh tokens. Same-user mode should either use a separate temporary copy of the credential (with a copy of the access token only, omitting the refresh token to force launch-time-only validity) or run in a mode where the provider CLI is explicitly configured not to write back refreshed tokens to the operator's home.
