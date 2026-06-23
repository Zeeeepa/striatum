# FALSIFIER - RFC 0165 Claude Provider Credential Freshness (Falsifier 2)

author: falsifier-reviewer-004

## Revision check: Path, Symlink, and Ownership Security Boundaries

I have reviewed the Holder's proposal for spawn-time Claude credential hydration to address GH #583. While the proposal correctly seeks to prevent launching lanes with stale credentials and isolates credential material, it exposes a critical Unix security vulnerability: a Time-of-Check to Time-of-Use (TOCTOU) symlink race during lane user home directory hydration. This race allows a compromised or malicious lane process to escalate privileges and compromise the operator host.

## Challenge: TOCTOU Symlink Race and Path Escape during Lane User Hydration

### Claim challenged

The Holder claims that the proposed hydration mechanism is secure and does not act as a privilege bridge:
- "Resolution does not become a privilege bridge. Source selectors are daemon/operator-owned; destination selectors are lane-home or daemon-configured; workflow CLAUDE_CONFIG_DIR escape tests fail closed without copying bytes." (Table Claim 393).
- "The destination file is written through a temp file in the destination directory, chmodded `0600`, chowned to the lane uid/gid, fsynced where practical, and atomically renamed. Existing destination symlinks, non-regular files, wrong owner, or wrong mode are refusal conditions unless the hydrator overwrote them through the safe temp file path and the final file verifies as regular lane-owned `0600`." (§2, paragraph 4).

### Concrete refutation

Consider the following scenario where the lane user is a distinct OS user (e.g., `striatum-lane`), and the daemon runs as the operator user (e.g., `halbritt`):

1. The daemon starts `supervise.start` for a new Claude lane.
2. The hydrator resolves the destination credential path to `/home/striatum-lane/.claude/.credentials.json`. It evaluates symlinks on the path and verifies that `.claude/` is a normal directory and no symlink exists.
3. Because the daemon user `halbritt` does not have write access to `/home/striatum-lane/.claude/` (which is restricted to `0700` and owned by `striatum-lane`), the hydrator writes the credentials via `sudo`:
   ```bash
   sudo -n -u striatum-lane -- sh -c 'cat > /home/striatum-lane/.claude/.credentials.json.tmp'
   ```
4. A concurrent or previously running process under the `striatum-lane` user (which could be spawned from an active workspace run or left over from a previous execution) monitors the `/home/striatum-lane/.claude/` directory using `inotify` or a loop.
5. As soon as the temp file `.credentials.json.tmp` is created by the `sudo` command, the lane process deletes the temp file and replaces it with a symlink pointing to an operator-owned sensitive file, such as `/home/halbritt/.ssh/authorized_keys` or `/home/halbritt/.bashrc`.
6. The `sudo` command finishes writing the OAuth credential bytes into the target of the symlink.
7. To enforce ownership and permission correctness, the daemon (running as `halbritt`) calls standard file API functions on the path:
   ```go
   os.Chown("/home/striatum-lane/.claude/.credentials.json.tmp", laneUid, laneGid)
   os.Chmod("/home/striatum-lane/.claude/.credentials.json.tmp", 0600)
   ```
8. Because `os.Chown` and `os.Chmod` follow symlinks by default in Go and standard POSIX system calls, they operate on the target of the symlink: `/home/halbritt/.ssh/authorized_keys`.
9. The owner of `/home/halbritt/.ssh/authorized_keys` is changed to `striatum-lane`, and its permissions are adjusted.
10. The `sudo` command (or daemon helper) performs the rename:
    ```go
    os.Rename("/home/striatum-lane/.claude/.credentials.json.tmp", "/home/striatum-lane/.claude/.credentials.json")
    ```
    This replaces the symlink with the final file, hiding the trace.
11. The lane user `striatum-lane` now has write permissions on `/home/halbritt/.ssh/authorized_keys`. The lane process appends its own public key to the file and logs into the operator host via SSH.
12. The lane user gains full passwordless shell access as the operator user, completely compromising the daemon, all target repositories, and the host machine.

This scenario proves that the proposed hydration mechanism introduces a classic privilege escalation bridge from the restricted lane OS user to the operator user via TOCTOU symlink races.

### Tests that do not close it

- `TestClaudeHydrationDestinationOwnerMode` and `TestClaudeHydrationSymlinkEscape` as proposed only assert that a pre-existing symlink is detected and refused. They do not simulate a concurrent file-swapping race during the window between creation and permission adjustments.
- `TestClaudeHydrationResolverRejectsWorkflowPath` only verifies prefix-matching on path resolution, not physical symlink evaluation races during copy operations.

### Strongest rebuttal on the Holder's behalf

The Holder may argue that the hydrator can perform `os.Lstat` checks on the temporary file before invoking `Chown` or `Chmod` to verify it is not a symlink.

This does not close the gap because:
1. It does not resolve the TOCTOU window: the lane process can swap the temp file with a symlink immediately *after* the `Lstat` check but *before* the `Chown` or `Chmod` call executes.
2. The lane user has write access to the parent directory (`/home/striatum-lane/.claude/`) and can modify its contents at any time.

### Required repair

To prevent symlink races and close the privilege bridge:

1. **Write and Modify Credentials strictly inside the Lane Sandbox**: The daemon (running as `halbritt`) must never perform `os.Chown`, `os.Chmod`, or `os.Rename` directly on files inside the lane home directory. All copy, permission setting, and renaming operations must be delegated to commands running strictly under `sudo -u <lane>`, ensuring that the execution context cannot touch or change ownership of files that the lane user does not already own and have permission to modify.
2. **Use of Lchown**: If the daemon must adjust path ownership, it must use `os.Lchown` (which does not follow symlinks) instead of `os.Chown`.
3. **O_NOFOLLOW / Safe File Descriptors**: The file copy helper must open the destination temp file with `O_NOFOLLOW` and `O_CLOEXEC` to prevent writing to symlinks.

### Verdict

Real gap remains. The proposal fails to guard against concurrent symlink swapping during the multi-step hydration process, which exposes the operator to privilege escalation by the lane user.
