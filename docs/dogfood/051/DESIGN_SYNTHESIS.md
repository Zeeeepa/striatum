---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

author: designer-unknown-model-002

# DESIGN SYNTHESIS — RFC 0039 V1.6 Go daemon hardening

Reconciles `design/codex/DESIGN.md`, `design/claude_code/DESIGN.md`,
`design/gemini/DESIGN.md` for closure of dogfood-049 V1.6 follow-ups.

## Decisions

- **F-pty:** Use `github.com/creack/pty v1.1.24`. The single-function
  `pty.Start(cmd)` returns the master fd which becomes the daemon's
  `StdinWriter`. Slave-side is wired automatically by creack/pty as the
  child's stdin/stdout/stderr.
- **F-pid-recycling:** Linux path reads `/proc/<pid>/stat` field 22
  (clock-ticks-since-boot) and converts to absolute time via
  `/proc/stat`'s `btime` + assumed 100Hz `CLK_TCK`. Compare against
  `PointerRow.StartedAt` with 2-second tolerance. On non-Linux fall
  back to signal-0 only (acceptance gate is Linux explicitly per
  RFC 0039 §6).
- **F-perms:** scratch dir `0o700`, pidfile `0o600`, stdout/stderr
  fallback `0o600`.
- **F-store:** Concrete `*pgxpool.Pool`-backed implementation in
  `go/pkg/db/supervisor_pointers.go`. Defines a local `PointerRow`
  shape that mirrors `supervisor.PointerRow` to avoid an import cycle.
- **F-ci:** Add a "Verify Go binary present" step in
  `.github/workflows/ci.yml` immediately after `make daemon-go-build`
  when `daemon-core == 'go'`. The step `exit 1`s with a clear stderr
  message when `go/bin/striatumd` is missing.

## Implementation order

1. `go/pkg/supervisor/pointer.go` — perm tightening.
2. `go/pkg/supervisor/pty.go` — perm tightening + creack/pty import +
   PTY launch.
3. `go/pkg/supervisor/liveness.go` — split `processAlive` →
   `processAliveAtStartTime`, read `/proc/<pid>/stat`.
4. `go/pkg/supervisor/supervisor_test.go` — flip PTY not-wired test to
   wired functional test.
5. `go/pkg/db/supervisor_pointers.go` — Postgres-backed store.
6. `go/go.mod` / `go/go.sum` — `go get github.com/creack/pty`.
7. `.github/workflows/ci.yml` — verify-go-binary-present step.
8. `docs/rfcs/0039-go-daemon-core.md` — mark V1.6 closed.

## Acceptance

- `cd go && go build ./...` clean.
- `cd go && go test ./pkg/supervisor/...` green.
- `cd go && go test ./pkg/db/...` green (existing tests + new
  supervisor_pointers if a real PG is available).
- CI `daemon-core=go` axis fails fast when `go/bin/striatumd` is
  removed by hand between build + test steps.
