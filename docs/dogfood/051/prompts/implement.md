# Implement — RFC 0039 V1.6 Go daemon hardening

Blocked until `review_design` returns an accepting verdict.

Implement V1.6 deltas per `docs/dogfood/051/DESIGN_SYNTHESIS.md`.
Claude implementer (deliberately not codex — codex/codex anti-pattern
avoidance, prior count 5+).

**Your write scope:** `go/`, `tests/`, `Makefile`, `.github/workflows/`,
`docs/rfcs/0039-go-daemon-core.md`, `docs/dogfood/051/build/`. No
writes to `.striatum/`, `docs/rfcs/0043-*`, `docs/dogfood/049/`,
`docs/dogfood/050/`.

**F-by-F:**

- **F-pty:** Add `github.com/creack/pty` to `go/go.mod` + `go/go.sum`,
  wire `pty.Start(cmd)` in `go/pkg/supervisor/pty.go`'s `UsePTY=true`
  branch, return `LaunchResult{PID, StdinWriter: ptyMaster, Cmd: cmd}`.
  Replace the not-wired sentinel error.
- **F-pid-recycling:** In `go/pkg/supervisor/liveness.go`, after the
  signal-0 probe passes, check `/proc/<pid>/stat` field 22 against
  `PointerRow.StartedAt` (Linux) or `sysctl` (darwin). Return false if
  start-time disagrees by more than 2 seconds.
- **F-perms:** `go/pkg/supervisor/pointer.go::WritePidfile` writes pid
  at `0600`; mkdir scratch at `0700`. Same in `pty.go::ensureFIFO`.
- **F-store:** Create `go/pkg/db/supervisor_pointers.go` implementing
  `supervisor.PointerStore` against
  `striatumd.process_supervisor_pointers` via the existing `go/pkg/db`
  connection pool.
- **F-ci:** In `.github/workflows/ci.yml`, when `daemon-core == 'go'`,
  add a "Verify Go binary present" step that runs `test -f go/bin/striatumd`
  after `make daemon-go-build`. If missing, fail the job with a clear
  error. (Effectively: don't let `make daemon-go-build` silent-success
  on a no-op build masquerade as a skip.)

**Tests:**

- Extend `go/pkg/supervisor/supervisor_test.go` with PTY-path table
  tests against `/bin/cat` (echo back), PID-recycling table test with
  a synthetic mismatched StartedAt, perm table test asserting `0o700`
  / `0o600`.
- New `go/pkg/db/supervisor_pointers_test.go` against the existing
  test Postgres fixture (skip if no PG).

**HANDOFF:** `docs/dogfood/051/build/HANDOFF.md` with byline matching
the work packet's `expected_author_line` (typically
`implementer-unknown-model-001`). Summarize shipped scope, tests run,
deviations.

**Use sub-agents aggressively:** one per F (pty, pid, perms, store,
ci) dispatched in parallel.
