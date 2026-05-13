---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0039", "v1-6", "build"]
---

author: reviewer-unknown-model-002

# Build Review — RFC 0039 V1.6 (claude, ergonomics_dx)

Fresh-context ergonomics/DX review of the V1.6 Go daemon
hardening build. Reads were limited to the artifacts named in
`docs/dogfood/051/prompts/review_build.md`: `DESIGN_SYNTHESIS.md`,
`build/HANDOFF.md`, and the cited Go + CI source files.

## Scope

Required-check coverage from a first-time-user perspective:

- **F-pty:** wired. `go/pkg/supervisor/pty.go:108` calls
  `pty.Start(cmd)`; `launchPTY` returns the master `*os.File` as
  `StdinWriter`. Sentinel-not-wired path is gone.
  `supervisor_test.go::TestLaunchPTYWired` spawns `/bin/true`
  under PTY and asserts positive PID + non-nil writer
  (`supervisor_test.go:161`).
- **F-pid-recycling:** real source.
  `liveness.go::readProcessStartTime` reads `/proc/<pid>/stat`
  field 22 and converts via `/proc/stat` `btime` + 100Hz CLK_TCK
  (`liveness.go:192`). 2-second tolerance constant is local and
  named (`liveness.go:178`). Non-Linux returns `(_, false)` and
  falls back to signal-0.
- **F-perms:** `pointer.go:48` mkdir `0o700`; `pointer.go:53`
  pidfile `0o600`. `pty.go:96` scratch dir `0o700`;
  `pty.go:125` stdout/stderr fallback `0o600`.
- **F-store:** SQL round-trip is present.
  `go/pkg/db/supervisor_pointers.go` defines `UPSERT … ON
  CONFLICT (supervisor_id)`, a typed `ErrSupervisorNotFound`, and
  `nullable()` helper for the optional string columns. See F1
  below for an interface-satisfaction gap that blocks the
  package doc comment's advertised wire-up.
- **F-ci:** `.github/workflows/ci.yml:40` adds a hard
  `test -x go/bin/striatumd` step under
  `matrix.daemon-core == 'go'` that fails with an
  `::error::` annotation. dogfood-049 gemini F6 is cited
  in the comment.

## Findings

### F1 (medium ergonomics) — `db.SupervisorPointerStore` does not satisfy `supervisor.PointerStore`

`go/pkg/db/supervisor_pointers.go:21-23` advertises:

```go
store := db.NewSupervisorPointerStore(pool)
sup := supervisor.NewLiveness(cfg, store, supervisorID, pid)
```

But `supervisor.PointerStore` (`pointer.go:23-27`) requires
methods that take `supervisor.PointerRow`, and the db type's
methods take `db.PointerRow` (`supervisor_pointers.go:54`,
`:90`, `:105`). The two `PointerRow` structs are byte-identical
but distinct named types under Go's nominal type system, so the
advertised one-liner will fail to compile with
`cannot use store … as supervisor.PointerStore`.

First-time wirer hits a wall the doc comment did not warn
about. Three plausible affordances:

- Extract `PointerRow` to a shared types package (no cycle).
- Make `db.PointerRow` a type alias: `type PointerRow = supervisor.PointerRow` — but this would re-introduce the cycle the file deliberately avoids.
- Ship a thin adapter (`type adapter struct { *SupervisorPointerStore }`) that converts row shapes at the boundary.

`build/HANDOFF.md:69-71` already flags wire-up as V1.7. The
finding here is the doc-vs-compile drift in the package
comment, not the deferral itself. Either correct the comment
to call out the conversion shim or land an adapter now.

### F2 (low) — Heartbeat ignores `ErrSupervisorNotFound`

The package comment for `ErrSupervisorNotFound`
(`supervisor_pointers.go:30-33`) says "Liveness treats this as
a hard signal: the daemon has already reaped the supervisor
record, so further heartbeats are dropped." But
`liveness.go:114-117` silently `continue`s on any error:

```go
row, err := l.store.GetSupervisorPointer(ctx, l.supervisorID)
if err != nil {
    continue
}
```

A first-time reader of the package doc would expect the
goroutine to exit on `ErrSupervisorNotFound`; instead the loop
spins forever. Add an `errors.Is(err, db.ErrSupervisorNotFound)`
branch that returns from `run`, or update the package comment
to match current behavior.

### F3 (low) — `processAlive` is a dead public wrapper

`liveness.go:150-152` defines `processAlive(pid)` as
`processAliveAtStartTime(pid, time.Time{})`. The only caller in
tree is the wrapper itself; the heartbeat loop calls
`processAliveAtStartTime` directly (`liveness.go:118`). For a
first-time reader scanning the file, two functions doing the
same thing for the same call site is a discoverability tax with
no payoff. Delete `processAlive`, or document the back-compat
contract that keeps it alive.

### F4 (low) — `LaunchSpec` PTY semantics are silent

`pty.go:17-26` documents `StdinPipePath`, `StdoutPath`,
`StderrPath` as wrapper-protocol fields. `launchPTY`
(`pty.go:104-117`) ignores all three: there is no FIFO setup,
no stdout/stderr redirection, the master fd shoulders
everything. A first-time user reading `LaunchSpec` would assume
those fields applied uniformly. Either annotate each field with
"(pipe mode only)" / "(honored under UsePTY=false)" or move
pipe-only fields into a `PipeSpec` sub-struct.

### F5 (low) — `ensureFIFO` no longer ensures a FIFO

`pty.go:94-97`: the function name still says `ensureFIFO` but
the body only `os.MkdirAll`s the scratch dir; the doc string
admits "The FIFO itself is created by the daemon when
delivering packets." Rename to `ensureScratchDir` for honesty.
Minor, but a first-time grep for FIFO creation lands here and
mis-leads.

### F6 (low) — `clkTck = 100` is a silent assumption

`liveness.go:237` hard-codes `clkTck = 100`. The comment names
"standard sysconf(_SC_CLK_TCK) on Linux x86_64 / arm64" but
doesn't say what happens if a downstream kernel is built with
`CONFIG_HZ=250` (some ARM/embedded targets). The acceptance
gate is Linux x86_64/arm64 per RFC 0039 §6, so this is
deferrable, but a comment line like
`// TODO: cgo sysconf(_SC_CLK_TCK) for non-100Hz kernels` keeps
the assumption discoverable for the next reader.

## Positive ergonomics

- `LivenessConfig.withDefaults()` (`liveness.go:24-35`) is a clean
  zero-value-friendly pattern; a first-time caller can pass
  `LivenessConfig{}` and get sensible cadence.
- `pointer.go:1-9` package doc is a model for orientation: it
  states the authoritative-record contract (Postgres row,
  pidfile is a hint) in two sentences.
- `processAliveAtStartTime` doc (`liveness.go:154-184`)
  surfaces the V1.6 PID-recycling rationale in-line with the
  function; reader does not need to fetch the RFC.
- CI step's `::error::` annotation (`ci.yml:47`) surfaces in the
  GitHub Actions UI, not just the log scrollback. dogfood-049
  gemini F6 cited inline keeps the change reviewable.
- `TestLaunchPTYWired` (`supervisor_test.go:161-179`) name now
  matches reality. The old sentinel test is gone, not commented
  out.
- `HANDOFF.md` lists Deviations and V1.7 follow-ups explicitly
  rather than burying them; a first-time reviewer can decide
  whether each gap is in scope without grepping the RFC.

## Verdict

`accept_with_findings` (severity: low).

All five required checks land at the structural level
(F-pty wired, F-pid-recycling reads `/proc/<pid>/stat`, F-perms
0700/0600, F-store round-trips at the SQL level, F-ci
hard-fails). F1 is the only finding that touches the
advertised surface for first-time integrators, and per
HANDOFF the boot wire-up is explicitly V1.7 — so the
doc-vs-compile drift is deferrable as long as the package
comment is corrected or the adapter lands when V1.7 wires the
daemon. F2-F6 are minor tightening opportunities.
