author: designer-unknown-model-001

# Design Proposal: RFC 0039 V1.6 Go Daemon Hardening

This document outlines the implementation design to address the five V1.6 known follow-ups for the Go daemon core, establishing parity with the Python supervisor boundaries and closing known gaps.

## 1. F-pty: PTY Integration

**Objective:** Replace the pipe-based fallback with a full pseudo-terminal integration for supervised lanes.

**Files to Touch:**
- `go.mod` / `go.sum`
- `go/pkg/supervisor/pty.go`
- `go/pkg/supervisor/supervisor_test.go`

**Design Sketch:**
We will introduce `github.com/creack/pty` as a targeted dependency to allocate PTYs cross-platform. This represents a known but acceptable supply-chain footprint increase to achieve terminal parity.

In `pty.go`, we branch on `spec.UsePTY` inside `Launch`:

```go
if spec.UsePTY {
    ptmx, err := pty.Start(cmd)
    if err != nil {
        return nil, fmt.Errorf("supervisor: pty start: %w", err)
    }
    // ptmx is a single handle representing both the stdin writer (for packet delivery)
    // and stdout/stderr reader (for log drain/discard).
    return &LaunchResult{
        PID:         cmd.Process.Pid,
        StdinWriter: ptmx,
        Cmd:         cmd,
    }, nil
}
```

**Acceptance Verifiers:**
- A Go-level test invoking `Launch` with `UsePTY=true` that asserts successful child spawn without encountering the "not yet wired" sentinel error.
- Verify that `StdinWriter` in the result is correctly assigned the PTY master.

## 2. F-pid-recycling: Liveness Probe Start-Time Validation

**Objective:** Close the PID-recycling race condition by pairing the `signal-0` probe with process start-time verification.

**Files to Touch:**
- `go/pkg/supervisor/liveness.go`
- `go/pkg/supervisor/liveness_test.go` (new or existing)

**Design Sketch:**
We introduce a helper to fetch a process's start time and compare it to the `PointerRow.StartedAt` timestamp to ensure we are monitoring the original process.

```go
func processAliveAndVerified(pid int, expectedStart time.Time) bool {
    if !processAlive(pid) {
        return false
    }
    actualStart, err := getProcessStartTime(pid)
    if err != nil {
        return false // Fail-safe if stats cannot be read
    }
    // Allow a small delta to account for recording delays
    return actualStart.Sub(expectedStart).Abs() < 5*time.Second
}
```
For Linux, `getProcessStartTime` parses `/proc/<pid>/stat` (field 22 `starttime`, relative to boot time from `/proc/stat`). For macOS (darwin), it will fall back to using `ps -p <pid> -o lstart=` or a targeted `sysctl kern.proc.pid.<pid>` wrapper.

**Acceptance Verifiers:**
- A Go-level test that simulates PID reuse (by launching a new short-lived process with a different start time) and asserts the liveness check returns false, leading to a `MarkSupervisorLost` call.

## 3. F-perms: Scratch Directory Permissions

**Objective:** Enforce the principle of least privilege on transient operational state directories.

**Files to Touch:**
- `go/pkg/supervisor/pointer.go`
- `go/pkg/supervisor/pty.go`

**Design Sketch:**
Tighten directory creation masks from `0755` to `0700` and file creation masks from `0644` to `0600`.

In `pointer.go` (`WritePidfile`):
```go
if err := os.MkdirAll(dir, 0o700); err != nil { ... }
...
if err := os.WriteFile(tmp, []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil { ... }
```

In `pty.go` (`ensureFIFO`):
```go
func ensureFIFO(scratchDir string, supervisorID string, fifoPath string) error {
    dir := filepath.Join(scratchDir, supervisorID)
    return os.MkdirAll(dir, 0o700)
}
```

**Acceptance Verifiers:**
- A Go-level test verifying that the `MkdirAll` and `WritePidfile` outputs match `0700` and `0600` modes precisely.

## 4. F-store: Concrete Postgres-backed PointerStore

**Objective:** Implement the `PointerStore` interface backed by the Postgres DB.

**Files to Touch:**
- `go/pkg/db/supervisor_pointers.go` (new)
- `go/pkg/db/supervisor_pointers_test.go` (new)
- `go/cmd/striatumd/main.go` (to wire the store)

**Design Sketch:**
Create a struct implementing `supervisor.PointerStore` using the existing `db.Runner` interface.

```go
type PgPointerStore struct {
    Runner Runner
}

func (s *PgPointerStore) UpsertSupervisorPointer(ctx context.Context, row supervisor.PointerRow) error {
    query := `
        INSERT INTO striatumd.process_supervisor_pointers
        (supervisor_id, repository_id, session_id, pid, started_at, last_heartbeat_at, stdin_pipe_path, state, lost_reason)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        ON CONFLICT (supervisor_id) DO UPDATE SET
            pid = EXCLUDED.pid,
            last_heartbeat_at = EXCLUDED.last_heartbeat_at,
            state = EXCLUDED.state,
            lost_reason = EXCLUDED.lost_reason
    `
    _, err := s.Runner.Exec(ctx, query, row.SupervisorID, row.RepositoryID, ...)
    return err
}
```

**Acceptance Verifiers:**
- An integration test running against `STRIATUM_PG_TEST_URL` verifying insert, update, retrieval, and deletion/mark-lost behavior mapping to the correct schema.

## 5. F-ci: Hard-fail on Missing Go Binary

**Objective:** Prevent the CI matrix job from silently passing via skips if the Go binary is missing.

**Files to Touch:**
- `tests/test_daemon_go_supervisor.py`
- `tests/conftest.py`

**Design Sketch:**
Ensure that when `STRIATUM_MULTI_REPO_DAEMON_CORE=go` is set, a missing Go binary translates to a failed test rather than a skipped one.

Modify `test_daemon_go_supervisor.py`:
```python
import os
import pytest
from striatum.cli.daemon import resolve_go_binary

def test_go_binary_present_if_core_go():
    if os.environ.get("STRIATUM_MULTI_REPO_DAEMON_CORE") == "go":
        try:
            resolve_go_binary()
        except Exception as e:
            pytest.fail(f"Go binary is missing, but CORE=go is set: {e}")
```

**Acceptance Verifiers:**
- CI behavior (or a local test mimicking it) confirming that `make test-multi-repo CORE=go` yields an error exit code if the binary is deliberately removed.
