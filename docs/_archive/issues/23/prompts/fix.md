# Implement -- GH #23

You are the implementer. Apply only the scoped changes for this workflow.

## Read

- `docs/issues/23/SPEC.md`
- `docs/issues/23/SCOPE.md`
- the source modules named in `SCOPE.md`
- `docs/issues/22/SPEC.md` only as far as is needed to satisfy the
  cross-issue acceptance bullet (daemon stop falls back to the pidfile).

## Deliverables

Per `docs/issues/23/SPEC.md` "Acceptance / Definition of done":

1. The Go daemon (or the Python launcher, per `SCOPE.md`) writes
   `<runtime_dir>/striatumd.pid` on successful startup, mode 0600,
   atomically (temp + rename) so a crash-mid-write doesn't leave a
   half-written PID. Mirror `go/pkg/supervisor/pointer.go`'s pattern.
2. Clean shutdown (SIGTERM or graceful exit) removes the pidfile.
3. Crash-leaves-stale-pidfile is detectable by `daemon status`'s
   existing `_pid_alive` check (no behavior change there, but verify
   it still works with the new write path).
4. `daemon status` on a healthy daemon returns `running:true` and
   `pid:<real pid>` within ~1s of start.
5. Unit tests for: startup-writes-pidfile, shutdown-removes-pidfile,
   stale-pidfile-detection. Integration smoke if existing daemon
   tests support it.

## Constraints

- Stay inside `write_scope.allowed_paths`.
- Atomic write only (temp file in same dir + `os.Rename` /
  `os.WriteFile` with care). Direct `os.WriteFile` is acceptable if the
  directory is already private (`runtime_dir()` is 0700).
- Do not change the pidfile path or its discovery (`daemon_runtime.py:44`)
  unless the triager has documented why.
- Use the exact `author:` line from the work packet in the handoff.

## Handoff

Write `docs/issues/23/build/HANDOFF.md` with the
`striatum.handoff.v1` front matter. Cite each definition-of-done bullet
closed, files changed, tests run / not run, and residual risk.
