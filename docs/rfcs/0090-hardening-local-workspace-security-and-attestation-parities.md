# RFC 0090: Hardening Local Workspace Security and Attestation Parity

Status: proposed
Date: 2026-05-29
Context: STRIATUM_ARCHITECTURE_REVIEW_TEAMWORK_PREVIEW_2026-05-29.md, RFC 0033, RFC 0043, RFC 0072, RFC 0082, RFC 0089
Author: architect-antigravity-1

---

## Problem

A systems architecture review of the Striatum codebase highlighted critical security vulnerabilities, concurrency bottlenecks, and testing blind spots in the local-first coordinator design. As AI agents execute unverified, raw terminal operations in local workspace lanes, these issues represent significant risks to the operator's host system stability and daemon reliability.

Specifically, five primary gaps must be addressed:

1.  **Blocker: Symbolic Link Traversal Escape during Artifact Publication**
    During workspace adoption, the daemon checks if the root path is a symbolic link and rejects it. However, it does not recursively evaluate nested directories or enforce path jailing during artifact publishing. A supervised AI agent running under an active lane can easily create a symbolic link (e.g. `ln -s /etc/passwd ./passwd_symlink`) and request the daemon to publish an artifact at `passwd_symlink`. The daemon, running with local operator privileges, resolves the link on the host and writes arbitrary content straight into `/etc/passwd`.

2.  **Serious: Hardcoded Migration Advisory Lock Concurrency Collision**
    In `go/pkg/db/migrations.go:18`, the system advisory lock key is hardcoded to a static integer:
    ```go
    const MigrationLockKey = 332933
    ```
    If an operator runs multiple Striatum instances or registers multiple repositories pointing to the same Postgres database server under distinct schemas, startup routines will encounter deadlocks. The second daemon blocks indefinitely waiting for the first daemon to release the lock, even though they operate on completely isolated schemas.

3.  **Serious: Non-blocking FIFO Named-Pipe ENXIO Drops**
    To avoid blocking RPC handlers when sending packets to supervisor helpers, the daemon opens named pipes inside `.striatum/` in non-blocking write-only mode (`syscall.O_WRONLY|syscall.O_NONBLOCK`). On Linux, opening a named pipe in non-blocking write-only mode instantly returns an `ENXIO` error if no reader is actively attached. In degraded states (e.g., when a tmux pane detaches or a helper restarts), this open fails, causing the daemon to permanently drop incoming instruction packets and hang the lane.

4.  **Serious: Privilege Testing Blind Spot in DB Test Harness**
    To guarantee append-only logs and cryptographically durable provenance, Striatum uses native PL/pgSQL database triggers and `REVOKE UPDATE, DELETE` DML boundaries on events and artifacts tables. However, the database testing helper `go/pkg/pgtest/` connects to PostgreSQL and runs all package unit tests under the superuser (`postgres`) role. Since superusers bypass all table-level `REVOKE` privileges by design, the test suite never actually exercises the restricted database boundaries, introducing a critical test validation blind spot.

5.  **Serious: Missing Darwin Parity for System-Tick Process Attestation**
    Striatum implements process liveness attestation using Linux kernel boot ticks read from `/proc/<pid>/stat` field 22 to prevent PID recycling TOCTOU exploits. However, this implementation is hardcoded for Linux. When running on macOS (Darwin), the attestation parser is absent or fails to verify process identity, preventing secure attested bylines on macOS development environments.

---

## Goals

*   **Recursive Symlink Jailing**: Establish a secure, recursive path-jailing engine that validates every directory segment of an artifact publication target, asserting the canonical host destination is strictly inside the workspace boundary.
*   **Cryptographic Advisory Locking**: Refactor schema migration locking to derive the Postgres advisory lock key dynamically from the active schema name.
*   **Supervisor Named-Pipe Ring-Buffer**: Implement an in-memory, thread-safe daemon ring-buffer to buffer up to 10 packets during transient supervisor pipe detachments, preventing packet drops.
*   **Unprivileged Test Runner Pools**: Modify the integration test harness to run DML assertions under a dedicated unprivileged `striatumd_rw` connection pool.
*   **Darwin attestation parity**: Implement macOS-native process start-time validation using `proc_pidinfo` sysctl hooks.
*   **Dynamic Loopback Port Discovery**: Transition the loopback HTTP/MCP interface to dynamic port allocation to prevent port hijack attacks on shared workstations.

## Non-Goals

*   Introducing cloud-hosted databases, SaaS authentication, or off-machine telemetry.
*   Prescribing containerized Linux kernel namespaces or Docker runtime requirements for base developer setups.
*   Deprecating PostgreSQL as the sole live-state persistence engine.

---

## Proposal

### 1. Scoped Symlink Path-Jail Resolver

We will implement a recursive path-jailing routine inside `go/pkg/mutations/artifact.go` that is executed prior to writing any file payload to disk.

The resolver will perform the following steps:
1.  Take the requested workspace-relative path and compute its absolute target location.
2.  Recursively resolve every directory segment from the repository root downward using `filepath.EvalSymlinks`.
3.  If any directory segment resolves to a physical location outside the repository root's canonical directory tree, reject the write closed immediately, returning a `symlink_traversal_blocked` error.
4.  Assert that `filepath.HasPrefix` or equivalent clean-path checks are satisfied against the resolved path.

```go
func ValidateSandboxJail(repoRoot, targetPath string) (string, error) {
    cleanRoot := filepath.Clean(repoRoot)
    resolvedRoot, err := filepath.EvalSymlinks(cleanRoot)
    if err != nil {
        return "", fmt.Errorf("failed to resolve repo root: %w", err)
    }

    cleanTarget := filepath.Clean(targetPath)
    // If target doesn't exist yet, evaluate its parent directory
    parentDir := filepath.Dir(cleanTarget)
    resolvedParent, err := filepath.EvalSymlinks(parentDir)
    if err != nil {
        return "", fmt.Errorf("failed to resolve parent jail: %w", err)
    }

    relative, err := filepath.Rel(resolvedRoot, resolvedParent)
    if err != nil || strings.HasPrefix(relative, "..") {
        return "", ErrSymlinkJailEscape
    }

    return filepath.Join(resolvedRoot, filepath.Base(cleanTarget)), nil
}
```

### 2. Cryptographic Schema-Based Advisory Locking

We will refactor `go/pkg/db/migrations.go` to eliminate the static `332933` key. The advisory lock key will be derived dynamically at bootstrap time by hashing the active PostgreSQL database name and schema.

Using SHA-256, we will hash the formatted connection target `"<db_name>.<schema>"` and extract the first 63 bits of the resulting bytes as a safe 64-bit signed integer compatible with PostgreSQL `pg_advisory_xact_lock` functions.

```go
func DeriveAdvisoryLockKey(dbName, schemaName string) int64 {
    h := sha256.New()
    h.Write([]byte(fmt.Sprintf("%s.%s", dbName, schemaName)))
    sum := h.Sum(nil)
    // Cast first 8 bytes to int64, zeroing the sign bit for compatibility
    val := binary.BigEndian.Uint64(sum[:8])
    return int64(val & 0x7fffffffffffffff)
}
```

### 3. Supervisor Named-Pipe Ring-Buffer

To defeat FIFO `ENXIO` errors on Linux during supervisor detachment, the daemon will manage a small, thread-safe queue inside `go/pkg/mutations/supervision_control.go`.

*   When the daemon attempts to open the FIFO and encounters `syscall.ENXIO` (meaning no reader is attached), it will queue the instruction packet in a bounded, thread-safe queue of size 10.
*   The supervisor helper, upon reattaching or spawning a new PTY lane, will issue a `flush` packet down the control channel.
*   Upon receipt of the flush trigger, the daemon will drain the in-memory queue and pump all buffered packets down the FIFO, maintaining zero packet loss.
*   If the queue overflows (exceeding 10 packets), the lane is marked degraded and fails closed to prevent infinite memory growth.

### 4. Privilege Validation Test Harness Pools

We will enhance `go/pkg/pgtest/pgtest.go` to explicitly validate our PostgreSQL security boundaries.

*   During setup, the test harness will create the target schemas using the administrative superuser (`postgres`).
*   It will then dynamically create an unprivileged role `striatumd_rw_test` and grant it standard read-write permissions, while executing the identical DML revokes (`REVOKE UPDATE, DELETE ON striatumd.events, striatumd.artifacts`) executed in production.
*   The test harness will return two connection pools: a privileged pool for schema migrations/cleanup, and a restricted pool for running standard RPC mutations under test.
*   A new suite of integration tests will be added to assert that any attempt by the unprivileged pool to mutate or delete existing event records or artifact registries actively fails with a `42501 (insufficient_privilege)` error.

### 5. macOS (Darwin) Process Attestation Parity

We will implement macOS-specific process attestation in `go/pkg/supervisor/process_identity_darwin.go` to match the Linux implementation.

Using Darwin's `sysctl` interface with the `KERN_PROC` and `KERN_PROC_PID` selectors, the system will query the `kinfo_proc` struct for the target PID. We will extract the process start-time (`kp_eproc.e_paddr.to_struct.struct_timeval`) and derive a stable, numeric boot-tick token.

```go
// +build darwin

package supervisor

import (
    "golang.org/x/sys/unix"
    "unsafe"
)

func ProcessStartToken(pid int) (uint64, error) {
    mib := []int32{unix.CTL_KERN, unix.KERN_PROC, unix.KERN_PROC_PID, int32(pid)}
    var proc unix.KinfoProc
    length := uintptr(unsafe.Sizeof(proc))
    
    err := sysctl(mib, &proc, &length)
    if err != nil {
        return 0, err
    }
    
    // Convert e_paddr timeval structure to microseconds
    sec := uint64(proc.Eproc.Paddr.Sec)
    usec := uint64(proc.Eproc.Paddr.Usec)
    return (sec * 1000000) + usec, nil
}
```

### 6. Dynamic Loopback Port Discovery

To eliminate port collision and localhost hijacking on shared workstation/homelab systems:
*   The daemon `striatumd` will bind the loopback HTTP/MCP server to a dynamic, random free port (`127.0.0.1:0`).
*   Upon successful binding, the daemon will write the active port number along with the capability secret token to an owner-only connection metadata file under `/run/user/<uid>/striatum.conn` (or `~/.striatum/conn` on macOS) with `0o600` permissions.
*   The unprivileged CLI `striatum` will read this connection file to resolve the daemon's active endpoint, enabling zero-conf local routing with complete port-hijack protection.

---

## Recommended Changes

| Priority | Feature / Change Name | Rationale | Benefit | Effort |
| :--- | :--- | :--- | :--- | :--- |
| **Blocker** | Path-Jail Symlink Resolver | Protect host configurations from traversal escapes via AI-crafted symlinks in artifact publishing. | Closes directory traversal and system file overwrite security gaps. | 2 Days |
| **Serious** | Dynamic Schema Advisory Lock | Hashing the Postgres database and schema name to derive advisory lock keys instead of a static constant `332933`. | Prevents startup deadlocks when running multiple Striatum daemons on a shared database server. | 4 Hours |
| **Serious** | Named-Pipe Resilient Ring Buffer | Maintain an in-memory queue to absorb transit packets during named pipe `ENXIO` reader detachments. | Eliminates packet losses and lane freezes during tmux or supervisor restarts. | 1 Day |
| **Serious** | Unprivileged DB Test Connection Pool | Run DML tests under a restricted connection role rather than the Postgres superuser. | Automatically asserts append-only triggers and DML privilege blocks under integration testing. | 1 Day |
| **Serious** | macOS proc_pidinfo Sysctl Attestation | Retrieve process start-times on macOS using native Darwin sysctl selectors to establish attestation parity. | Guarantees attestation and PID recycling security on macOS workstations. | 3 Days |
| **Medium** | Dynamic Free Loopback Port Allocation | Bind MCP HTTP services dynamically, publishing coordinates via an owner-only, secure runtime socket file. | Prevents port collisions and port-hijacking attacks on shared environments. | 2 Days |

---

## Execution Roadmap

1.  **Phase 1 (Immediate)**: Implement the Scoped Symlink Resolver in `go/pkg/mutations/artifact.go` and add the sandbox jail check tests.
2.  **Phase 2 (Near-Term)**: Refactor advisory lock generation and implement the named pipe ring buffer.
3.  **Phase 3 (Medium-Term)**: Modify `pgtest.go` to execute unprivileged test pools, and build Darwin-native sysctl attestation helpers.
4.  **Phase 4 (Long-Term)**: Transition the daemon to dynamic port bindings and secure runtime discovery.
