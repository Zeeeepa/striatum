---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
verdict_intent: "needs_review"
tags: ["rfc-0039", "v1.6", "go-daemon", "design"]
---

author: designer-unknown-model-001

# Design — RFC 0039 V1.6 Go Daemon Hardening

## Scope

Close five named V1.6 findings against the Go daemon supervisor and CI:
`F-pty`, `F-pid-recycling`, `F-perms`, `F-store`, `F-ci`. The CHANGELOG
v1.39.0 "Known follow-ups (V1.6)" section is the authoritative spec for
each finding; this design lists touched files, key code sketches, and
acceptance verifiers.

Out of scope per the work packet: full Go mutation handler suite, apply
receipt cryptographic verification, envelope soft-version hardening,
`STRIATUM_DAEMON_CORE` operator-clarity warnings. Those defer to V1.7+.

## F-pty — `creack/pty` integration

**Files touched**

- `go/go.mod`, `go/go.sum` — add `github.com/creack/pty v1.1.21` (or
  current). The module's transitive footprint is zero (no further
  third-party deps); document this in the dogfood OPERATOR_REPORT.
- `go/pkg/supervisor/pty.go` — replace the sentinel branch with a real
  `pty.Start`-backed path. Keep the pipe-mode branch unchanged so
  `UsePTY=false` callers retain the current behavior.
- `go/pkg/supervisor/supervisor_test.go` — replace
  `TestLaunchPTYReturnsNotWired` with `TestLaunchPTYAllocatesMaster`.
- `tests/test_daemon_go_supervisor.py` — promote the Python parity
  scaffold to functional assertions (FIFO round-trip, heartbeat,
  SIGTERM no-orphan-PTY).

**Sketch**

```go
import "github.com/creack/pty"

if spec.UsePTY {
    cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
    cmd.Dir = spec.WorkingDir
    cmd.Env = append(os.Environ(), spec.Env...)
    master, err := pty.Start(cmd)
    if err != nil {
        return nil, fmt.Errorf("supervisor: pty.Start: %w", err)
    }
    if err := ensureFIFO(scratchDir, supervisorID, spec.StdinPipePath); err != nil {
        _ = master.Close()
        return nil, err
    }
    return &LaunchResult{
        PID:         cmd.Process.Pid,
        StdinWriter: master, // master is bidirectional; daemon writes packets here
        Cmd:         cmd,
    }, nil
}
```

The PTY master file handle becomes the daemon's `StdinWriter`; stdout
and stderr are drained from the same master by a goroutine that
forwards to `os.DevNull` (per D028, no transcript capture). The drain
goroutine is bounded by `cmd.Process.Wait()`.

**Acceptance verifiers**

- `go test ./pkg/supervisor -run TestLaunchPTYAllocatesMaster` — asserts
  `LaunchResult.StdinWriter != nil` and the child sees a TTY
  (`/bin/sh -c 'tty -s'` exits 0).
- `go test ./pkg/supervisor -run TestLaunchPTYDrainsStdout` — the
  goroutine pulls bytes from the master and the process exits cleanly.
- `tests/test_daemon_go_supervisor.py::test_pty_fifo_roundtrip` — boots
  the Go supervisor against `/bin/cat`, writes one JSON packet, asserts
  the packet appears at the child's stdin.

## F-pid-recycling — start-time validation

**Files touched**

- `go/pkg/supervisor/liveness.go` — pair `processAlive` with
  `processStartTime`; compare against `PointerRow.StartedAt` on each
  tick.
- `go/pkg/supervisor/start_time_linux.go` (new, `//go:build linux`) —
  `/proc/<pid>/stat` field 22 reader.
- `go/pkg/supervisor/start_time_darwin.go` (new, `//go:build darwin`) —
  `sysctl kern.proc.pid.<pid>` reader using
  `golang.org/x/sys/unix.SysctlRaw`.
- `go/pkg/supervisor/start_time_other.go` (new, `//go:build !linux && !darwin`)
  — returns `(time.Time{}, errUnsupported)`; callers degrade to
  signal-0 only with a logged warning.

**Sketch**

```go
func (l *Liveness) probeIdentity(row PointerRow) (alive bool, recycled bool) {
    if !processAlive(l.pid) {
        return false, false
    }
    started, err := processStartTime(l.pid)
    if err != nil || started.IsZero() {
        return true, false // best-effort: don't false-positive recycling
    }
    // Allow ±1s clock-resolution jitter against PointerRow.StartedAt.
    if started.Sub(row.StartedAt).Abs() > time.Second {
        return true, true
    }
    return true, false
}
```

On `recycled=true`, the controller calls
`store.MarkSupervisorLost(ctx, id, "pid_recycled")` and returns. This
closes gemini F1 without changing the pointer row schema; `StartedAt`
already exists in `PointerRow`.

**Acceptance verifiers**

- `go test ./pkg/supervisor -run TestStartTimeRoundTrip` — spawns a
  child, reads its start time, compares to `cmd.ProcessState.StartTime`
  (Linux) or a within-window `time.Now()` capture (darwin).
- `go test ./pkg/supervisor -run TestLivenessMarksRecycled` — uses a
  fake `processStartTime` provider injected via test-only var; supplies
  a start time 1h in the past against a `PointerRow.StartedAt` of
  `time.Now()`; asserts `lostCall == supID` and reason
  `pid_recycled`.

## F-perms — tighter scratch-dir perms

**Files touched**

- `go/pkg/supervisor/pointer.go::WritePidfile` — directory mode
  `0o755 → 0o700`, file mode `0o644 → 0o600`.
- `go/pkg/supervisor/pty.go::ensureFIFO` — directory mode
  `0o755 → 0o700`.
- `go/pkg/supervisor/pty.go::openDevNullOr` — `0o644 → 0o600` on
  created files.
- `go/pkg/supervisor/supervisor_test.go` — assert directory and pidfile
  permissions after `WritePidfile`.

**Sketch**

```go
if err := os.MkdirAll(dir, 0o700); err != nil { ... }
if err := os.WriteFile(tmp, []byte(...), 0o600); err != nil { ... }
```

No external consumer reads these paths today (the Python supervisor
writes its own pidfile elsewhere). The change is mechanical; the test
fixture pins the new mode so regressions are loud.

**Acceptance verifier**

- `go test ./pkg/supervisor -run TestPidfilePermissions` — stats the
  pidfile and the parent dir; asserts perms `0o700` / `0o600`.

## F-store — concrete Postgres `PointerStore`

**Files touched**

- `go/pkg/db/supervisor_pointers.go` (new) — `PgPointerStore` struct
  satisfying `supervisor.PointerStore`; CRUD against
  `striatumd.process_supervisor_pointers`.
- `go/pkg/db/supervisor_pointers_test.go` (new) — opt-in pgx-backed
  integration test gated on `STRIATUM_PG_TEST_URL` (matches
  `audit_race_test.go` shape).
- `go/cmd/striatumd/main.go` — wire `PgPointerStore` when a Postgres
  URL is configured; keep an in-memory fake for tests only.

**Sketch**

```go
type PgPointerStore struct{ R *Runner }

func (s *PgPointerStore) UpsertSupervisorPointer(ctx context.Context, row supervisor.PointerRow) error {
    _, err := s.R.Exec(ctx, `
        INSERT INTO striatumd.process_supervisor_pointers
            (supervisor_id, repository_id, session_id, pid, started_at,
             last_heartbeat_at, stdin_pipe_path, state, lost_reason)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
        ON CONFLICT (supervisor_id) DO UPDATE SET
            last_heartbeat_at = EXCLUDED.last_heartbeat_at,
            state             = EXCLUDED.state,
            lost_reason       = EXCLUDED.lost_reason
    `, row.SupervisorID, row.RepositoryID, row.SessionID, row.PID,
        row.StartedAt, row.LastHeartbeatAt, row.StdinPipePath,
        row.State, row.LostReason)
    return err
}
```

Column names and the partial unique constraint on
`(repository_id, session_id) WHERE state IN ('starting','attached','detached')`
are already in migration `0005_repo_local_workflow_state.sql`; no
schema change required.

**Acceptance verifiers**

- `go test ./pkg/db -run TestPgPointerStoreUpsertReadLost` — upserts a
  row, reads it back, marks lost, re-reads, asserts state transitions.
- `tests/test_daemon_go_supervisor.py::test_pointer_row_roundtrip` —
  Python-side cross-core regression: a Go-spawned supervisor produces
  rows readable by `src/striatum/daemon_supervisor/` Python code.

## F-ci — hard-fail on missing Go binary when `CORE=go`

**Files touched**

- `Makefile` — `test-multi-repo` target gains a precondition: when
  `CORE=go`, fail if `go/bin/striatumd` is absent and
  `STRIATUM_MULTI_REPO_REQUIRE_GO_BIN=1`.
- `.github/workflows/ci.yml` — set
  `STRIATUM_MULTI_REPO_REQUIRE_GO_BIN: '1'` on the `daemon-core: go`
  matrix job; ensure `make daemon-go-build` runs in the same job
  before tests.
- `tests/conftest.py` — when the sentinel is set and the binary path
  resolves to `None`, raise `pytest.UsageError` instead of skipping the
  Go-core class.

**Sketch (Makefile)**

```make
test-multi-repo:
ifeq ($(CORE),go)
ifeq ($(STRIATUM_MULTI_REPO_REQUIRE_GO_BIN),1)
	@test -x go/bin/striatumd \
	  || (echo "CORE=go requires go/bin/striatumd; run 'make daemon-go-build'"; exit 1)
endif
endif
	STRIATUM_MULTI_REPO_DAEMON_CORE=$(CORE) pytest tests/multi_repo
```

**Acceptance verifier**

- `tests/test_ci_sentinels.py::test_go_bin_required_when_core_go` —
  invokes `make test-multi-repo CORE=go STRIATUM_MULTI_REPO_REQUIRE_GO_BIN=1`
  in a temp tree with no `go/bin/striatumd`, asserts non-zero exit and
  the remediation string `make daemon-go-build` in stderr.

## Order & Risk

Land in this order so each step has working preconditions:

1. **F-perms** — mechanical, no behavior change beyond mode bits.
2. **F-store** — unblocks F-pid-recycling assertions against real rows.
3. **F-pid-recycling** — closes gemini F1.
4. **F-pty** — adds `creack/pty` dependency; promotes Python parity
   tests.
5. **F-ci** — pinned last so the preceding steps' green tests gate the
   matrix.

Primary supply-chain risk: `creack/pty` is well-trodden (Docker,
testcontainers, kubectl); add a one-line note to the OPERATOR_REPORT
naming the version and that it has no further transitive deps.
