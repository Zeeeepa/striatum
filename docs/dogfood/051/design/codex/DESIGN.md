author: designer-unknown-model-001

# RFC 0039 V1.6 Go Daemon Hardening Design

## Scope

V1.6 should close the five changelog-authoritative gaps without expanding
into the deferred Go mutation, apply-receipt signature, envelope hardening,
or `STRIATUM_DAEMON_CORE` operator-warning work. The implementation should
touch only the Go supervisor/database slice, the Go dependency files, and the
CI/test gates needed to prove that the Go axis cannot pass by skipping.

## Files To Touch

The primary code changes are:

- `go.mod` and `go.sum`: add `github.com/creack/pty` with a short dependency
  note in `docs/rfcs/0039-go-daemon-core.md` or the V1.6 build handoff. This
  is the only new supply-chain footprint: PTY allocation is delegated to the
  established cross-platform Go PTY package instead of custom termios/syscall
  code.
- `go/pkg/supervisor/pty.go`: replace the `UsePTY=true` sentinel with real
  `pty.Start(cmd)` launch, retain the non-PTY pipe path, and tighten
  supervisor scratch output permissions.
- `go/pkg/supervisor/pointer.go`: change pidfile directory/file modes to
  `0700` and `0600`, and extend `PointerRow` with the schema fields required
  by Postgres parity (`DaemonSupervisorID`, `RunID`, `PIDStartTime`,
  `UpdatedAt`, `MetadataJSON` or a typed map).
- `go/pkg/supervisor/liveness.go`: replace boolean signal-0 liveness with a
  process identity check that compares the current OS pid start time with the
  stored pointer row.
- new `go/pkg/supervisor/proc_identity_linux.go` and
  `go/pkg/supervisor/proc_identity_darwin.go`: platform implementations for
  reading process start identity. Linux reads `/proc/<pid>/stat` field 22;
  Darwin shells no external command and uses `sysctl kern.proc.pid.<pid>`.
- new `go/pkg/db/supervisor_pointers.go`: concrete Postgres-backed
  `PointerStore` against `striatumd.process_supervisor_pointers`.
- `.github/workflows/ci.yml`, `Makefile`, and `tests/test_daemon_go_supervisor.py`:
  make the Go-core axis hard-fail when `go/bin/striatumd` or a PATH binary is
  missing, and replace placeholder supervisor assertions with functional PTY,
  FIFO, heartbeat, and SIGTERM checks.

## Key Code Shape

`pty.go` should keep one `Launch` entry point. The PTY branch becomes:

```go
cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
cmd.Dir = spec.WorkingDir
cmd.Env = append(os.Environ(), spec.Env...)
master, err := pty.Start(cmd)
if err != nil { return nil, fmt.Errorf("supervisor: pty.Start: %w", err) }
go drainTo(master, openDevNullOr(spec.StdoutPath), openDevNullOr(spec.StderrPath))
return &LaunchResult{PID: cmd.Process.Pid, StdinWriter: master, Cmd: cmd}, nil
```

Because a PTY is bidirectional, packet delivery writes to the PTY master.
Drain must continue until command exit or context cancellation, and it must
not persist transcript text. If stdout/stderr paths are empty they stay
`os.DevNull`; when test paths are supplied they are opened `0600`, not `0644`.
`ensureFIFO` should still create the supervisor scratch directory, but with
`os.MkdirAll(dir, 0o700)`. `WritePidfile` should write the temporary pidfile
with `0600` before atomic rename.

Liveness should store and compare a stable process identity, not just a pid:

```go
func processIdentityMatches(pid int, expected string) (bool, error) {
    if pid <= 0 || expected == "" { return false, nil }
    current, err := pidStartTime(pid)
    if os.IsNotExist(err) { return false, nil }
    if err != nil { return false, err }
    return current == expected, nil
}
```

On Linux, `pidStartTime` parses `/proc/<pid>/stat` carefully because `comm`
may contain spaces inside parentheses; split only after the final `") "`, then
take field 22 from the full stat record, which is index 19 in the post-comm
slice. On Darwin, return the kernel process start timestamp normalized as a
string. `Liveness.run` should mark the pointer lost with
`pid_recycled_or_exited` when signal-0 succeeds but start identity mismatches.

`go/pkg/db/supervisor_pointers.go` should implement the existing interface
without changing supervisor package ownership:

```go
type SupervisorPointerStore struct { Runner db.Runner }

func (s SupervisorPointerStore) UpsertSupervisorPointer(ctx context.Context, r supervisor.PointerRow) error {
    return s.Runner.Exec(ctx, `INSERT INTO striatumd.process_supervisor_pointers (...)
        VALUES (...)
        ON CONFLICT (repository_id, supervisor_id) DO UPDATE SET
          daemon_supervisor_id = EXCLUDED.daemon_supervisor_id,
          run_id = EXCLUDED.run_id,
          session_id = EXCLUDED.session_id,
          pid = EXCLUDED.pid,
          pid_start_time = EXCLUDED.pid_start_time,
          state = EXCLUDED.state,
          updated_at = now(),
          metadata_json = EXCLUDED.metadata_json`, ...)
}
```

`MarkSupervisorLost` must scope by `(repository_id, supervisor_id)` if the
row object is available; if the current interface remains
`(supervisorID, reason)`, then the implementation must first resolve the row
and update only that primary key. `GetSupervisorPointer` should return
`pgx.ErrNoRows` wrapped distinctly enough for liveness to continue rather
than panic.

## Acceptance Verifiers

Go unit tests should cover `UsePTY=true` launching a small command that
requires a TTY, packet write through the returned `StdinWriter`, drain exit,
and `Stop(signalProcess=true)` leaving no live child. Permission tests should
assert supervisor directories are `0700` and pid/log/FIFO-created files are
`0600` on Unix platforms. Liveness tests should inject a fake
`pidStartTime` provider to prove live match, dead pid, and recycled pid paths;
a Linux parser test should include a `/proc/<pid>/stat` sample with spaces in
`comm`.

Postgres store tests should run under the daemon DB test harness and verify
upsert, get, update-to-lost, metadata JSON round-trip, and repository-scoped
primary-key behavior against `striatumd.process_supervisor_pointers`.
Python harness tests in `tests/test_daemon_go_supervisor.py` should stop being
placeholders: with `STRIATUM_MULTI_REPO_DAEMON_CORE=go`, missing
`go/bin/striatumd` must fail immediately, then the test should start a
supervised PTY process, deliver one JSON packet, observe heartbeat advancement
in Postgres, stop it, and assert no orphan process remains.

CI should add a small precondition before every Go-core Python harness step:
`test -x go/bin/striatumd || { echo "CORE=go requires make daemon-go-build";
exit 1; }`. Keep `make daemon-go-build` before `make daemon-go-test`; the
point is to prevent future local or matrix reshuffles from turning missing Go
binary into a skip-pass.
