author: reviewer-unknown-model-001

# Build Review: Go Daemon Core (RFC 0039 V1.6)

**Verdict:** `needs_revision`
**Posture:** `adversarial`
**Lane:** `gemini`

## Summary

The V1.6 hardening of the Go daemon core addresses several previous findings (PID-recycling on Linux, permission tightening, and CI hard-fails). However, the adversarial review identifies two high-severity architectural flaws in the supervisor's process management and database interaction that block acceptance. Specifically, the PTY implementation lacks an output drain, leading to guaranteed deadlocks for chatty processes, and the database UPSERT logic permits "session ghosting" where reaped supervisors can re-insert themselves into the authoritative store.

## Findings

### F1: PTY Master Deadlock (High)
The `launchPTY` implementation (`go/pkg/supervisor/pty.go:76`) allocates a pseudo-terminal via `pty.Start(cmd)` but fails to provide a reader for the master PTY file descriptor. If a supervised child process writes more data to `stdout` or `stderr` than the kernel's PTY buffer can hold (typically 4KB–16KB), the child will block on `write()`. This results in a permanent hang of the supervised lane. A malicious or chatty agent can effectively DoS its own supervisor by producing output.
*Cite:* `go/pkg/supervisor/pty.go:76`

### F2: Supervisor Row Re-insertion / "Session Ghosting" (High)
The `UpsertSupervisorPointer` implementation (`go/pkg/db/supervisor_pointers.go:56`) uses `INSERT ... ON CONFLICT (supervisor_id) DO UPDATE`. If the daemon reaps a stale supervisor by deleting its record from the `striatumd.process_supervisor_pointers` table, the supervisor's next heartbeat tick will automatically re-insert a new record instead of detecting the reaping. This breaks the daemon's authoritative control over session lifecycles and allows "ghost" supervisors to persist and re-register themselves indefinitely.
*Cite:* `go/pkg/db/supervisor_pointers.go:61`

### F3: Zombified Supervisor on Session Reaping (Medium)
The liveness heartbeat loop (`go/pkg/supervisor/liveness.go:107`) ignores all errors from `GetSupervisorPointer`. If a session is reaped by the daemon (record deleted), the supervisor continues running and heartbeating instead of terminating the child process and exiting. This is exacerbated by F2, as the heartbeat will succeed in re-creating the record.
*Cite:* `go/pkg/supervisor/liveness.go:111`

### F4: Path Traversal in Supervisor ID (Medium)
The `WritePidfile` (`go/pkg/supervisor/pointer.go:41`) and `ensureFIFO` (`go/pkg/supervisor/pty.go:68`) functions use `supervisorID` in a `filepath.Join` without validation. If an attacker can influence the `supervisorID` (e.g., via a compromised daemon or malicious RPC), they can trigger directory and file creation (with `0700/0600` permissions) outside the intended `.striatum/scratch` root via `../` sequences.
*Cite:* `go/pkg/supervisor/pointer.go:42`, `go/pkg/supervisor/pty.go:69`

### F5: Error Masking in `PointerStore.Get` (Medium)
The `GetSupervisorPointer` implementation (`go/pkg/db/supervisor_pointers.go:107`) swallows all database errors (including connection failures and pool exhaustion) and returns a generic `ErrSupervisorNotFound`. This masks infrastructure failures and prevents the supervisor from distinguishing between a legitimate session reaping and a transient database outage.
*Cite:* `go/pkg/db/supervisor_pointers.go:107`

### F6: macOS PID-Recycling Vulnerability (Low)
The start-time liveness check falls back to signal-0 only on non-Linux platforms (`go/pkg/supervisor/liveness.go:183`). This leaves macOS supervisors vulnerable to PID-recycling races where a dead PID is reused by an unrelated process before the supervisor detects the exit.
*Cite:* `go/pkg/supervisor/liveness.go:183`

### F7: Hardcoded `CLK_TCK` Assumption (Low)
The start-time reader assumes a constant `CLK_TCK = 100` (`go/pkg/supervisor/liveness.go:215`). While standard for x86_64/arm64 Linux kernels, this may produce incorrect results or false positives on kernels configured with different `USER_HZ` values.
*Cite:* `go/pkg/supervisor/liveness.go:215`

## Required Checks (Adversarial)

- [x] **F-pty:** PTY master is never drained; deadlock risk identified as F1.
- [x] **F-pid-recycling:** Start-time check implemented for Linux. macOS gap identified as F6.
- [x] **F-perms:** 0700/0600 used, but path traversal risk identified as F4.
- [x] **F-store:** Postgres-backed. Re-insertion risk identified as F2. Error masking as F5.
- [x] **F-ci:** CI check correctly fails if binary is missing.

## Verdict Rationale

While the implementation makes progress on the V1.5 findings, the introduces of F1 (Deadlock) and F2 (Ghosting) represent critical regressions in operational stability and security control. The supervisor must drain its PTY master to ensure process forward progress, and the database interaction must distinguish between updating an existing record and creating a new one to respect the daemon's reaping decisions.
