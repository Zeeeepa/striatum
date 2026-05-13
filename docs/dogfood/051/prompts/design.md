# Design — RFC 0039 V1.6 Go daemon hardening

Read in order:
- `docs/rfcs/0039-go-daemon-core.md`
- `docs/dogfood/049/review/build/codex/REVIEW.md` (F1-F3)
- `docs/dogfood/049/review/build/claude/REVIEW.md` (F1-F4)
- `docs/dogfood/049/review/build/gemini/REVIEW.md` (F1-F7)
- `CHANGELOG.md` v1.39.0 "Known follow-ups (V1.6)" section
- `go/pkg/supervisor/{pointer,liveness,pty}.go` to ground the changes

Design closure of these V1.6 findings. Each finding has a single
authoritative description in the v1.39.0 CHANGELOG entry; treat that
as the spec.

- **F-pty:** `creack/pty` integration. Replace the "PTY launch not yet
  wired" sentinel in `go/pkg/supervisor/pty.go`. Add `github.com/creack/pty`
  to `go.mod`/`go.sum`, wire `pty.Start` into the `UsePTY=true` path,
  thread the PTY master back to the supervisor for stdin packet delivery
  and stdout/stderr drain. Document the supply-chain footprint.
- **F-pid-recycling:** Pair the signal-0 liveness probe with start-time
  validation. On Linux read `/proc/<pid>/stat` field 22 (`starttime`),
  on darwin use `sysctl kern.proc.pid.<pid>`. Compare against
  `PointerRow.StartedAt` to detect PID recycling.
- **F-perms:** Tighten scratch-dir perms in `pointer.go` /
  `pty.go::ensureFIFO` from `0755`/`0644` to `0700`/`0600`. No external
  consumers exist for these paths.
- **F-store:** Concrete Postgres-backed `PointerStore` under
  `go/pkg/db/supervisor_pointers.go`. CRUD against
  `striatumd.process_supervisor_pointers` matching the Python supervisor.
- **F-ci:** When `CORE=go` and `go/bin/striatumd` is absent, hard-fail
  rather than skip-pass. Sentinel env var or explicit
  `make daemon-go-build` precondition.

Out of scope (deferred to V1.7+): full Go mutation handler suite
(closes codex F2), apply-receipt cryptographic verification (closes
gemini F2), envelope soft-version-check hardening (closes gemini F4),
`STRIATUM_DAEMON_CORE` operator-clarity warnings (closes gemini F5).

Output: design proposal listing files to touch, key code sketches
(short — not full implementations), and acceptance verifiers (Go-level
tests for each F). 600-1200 words.
