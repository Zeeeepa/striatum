author: implementer-unknown-model-001

# Build Handoff — RFC 0039 V1.6 Go daemon hardening

Operator-driven implementer slot (per recurring 5+ instance
claude-no-explicit-publish anti-pattern, treated as harness gap).

## Shipped Scope

### F-pty — creack/pty integration

- Added `github.com/creack/pty v1.1.24` to `go/go.mod` + `go/go.sum`
  via `go get`.
- `go/pkg/supervisor/pty.go` import line + new `launchPTY` function
  using `pty.Start(cmd)` returning master fd as `StdinWriter`. The
  old "not wired" sentinel is gone.
- `go/pkg/supervisor/supervisor_test.go::TestLaunchPTYWired` replaces
  the not-wired test: spawns `/bin/true` under PTY, asserts positive
  PID and non-nil StdinWriter, then waits cleanly.

### F-pid-recycling — start-time pairing

- `go/pkg/supervisor/liveness.go::processAliveAtStartTime` pairs the
  signal-0 probe with `readProcessStartTime(pid)` which reads
  `/proc/<pid>/stat` field 22, converts via `btime` + 100Hz CLK_TCK,
  and compares with `PointerRow.StartedAt` (2-second tolerance).
- On non-Linux the start-time reader returns `(_, false)` and the
  caller falls back to signal-0 only.
- Liveness goroutine now passes `row.StartedAt` to the probe on each
  tick.

### F-perms — 0700 / 0600

- `go/pkg/supervisor/pointer.go::WritePidfile`: scratch dir `0o700`,
  pidfile `0o600`.
- `go/pkg/supervisor/pty.go::ensureFIFO`: dir `0o700`.
- `go/pkg/supervisor/pty.go::openDevNullOr`: file `0o600`.

### F-store — Postgres-backed PointerStore

- New `go/pkg/db/supervisor_pointers.go`:
  `SupervisorPointerStore{pool: *pgxpool.Pool}` implementing the
  three-method `supervisor.PointerStore` interface via UPSERT on
  `striatumd.process_supervisor_pointers`. Local `PointerRow` shape
  mirrors `supervisor.PointerRow` to avoid an import cycle. Typed
  `ErrSupervisorNotFound` returned from `Get` and `MarkLost` when no
  row matches.

### F-ci — verify go binary present

- `.github/workflows/ci.yml` adds a new step under
  `daemon-core == 'go'`: `test -x go/bin/striatumd` after
  `make daemon-go-build`. Fails with `::error::` annotation if
  absent. Closes gemini F6 (CI matrix bypass risk).

## Tests

- `cd go && go build ./...` clean.
- `cd go && go test ./pkg/supervisor/...` PASS (cached + fresh).
- `cd go && go test ./pkg/db/...` existing tests remain green.

## Deviations

- macOS start-time reader is deliberately not wired (returns
  `(_, false)` from `readProcessStartTime`); macOS falls back to
  signal-0 only. V1.7 follow-up will use `proc_pidinfo` /
  `sysctl kern.proc.pid.<pid>`.
- The Postgres-backed `SupervisorPointerStore` is not yet wired into
  `cmd/striatumd/main.go`'s boot path; the boot wire-up is a
  one-liner dep injection that lands when the daemon switches from
  in-memory fakes to PG-backed in V1.7.

## V1.7 follow-ups
- Wire Postgres-backed PointerStore into the daemon main.
- macOS process start-time reader.
