---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/issues/23/SPEC.md", "docs/issues/22/SPEC.md", "docs/ROADMAP.md", "docs/TODO.md", "docs/DECISION_LOG.md", "docs/POSTGRES_TRANSITION.md", "docs/SPEC.md", "docs/INDEX.md", "AGENTS.md", "src/striatum/daemon_runtime.py", "src/striatum/daemon_pg/client_admin.py", "src/striatum/cli/daemon.py", "go/cmd/striatumd/main.go", "go/pkg/supervisor/pointer.go"]
---

author: triager-unknown-model-001

# GH #23 -- Scope

Bound scope for GH #23, "daemon status reads `striatumd.pid` but no code path
writes it." The implementation job adds a Go-side pidfile lifecycle to the
daemon main so `striatum daemon status` returns truthful `pid`/`running`
values and `striatum daemon stop` (after #22 lands its no-PG fallback) has a
real pidfile to consult.

## Issues Covered

- GH #23 -- daemon status reads `striatumd.pid` but no code path writes it.

Related but not closed by this workflow:

- GH #22 -- daemon migration owner-role gap. Acceptance bullet 4 of #23
  ("`daemon stop` falls back to the pidfile even if it can't connect to
  Postgres") is shared with #22 acceptance bullet 2 ("`daemon stop` works
  even when migrations are pending"). This workflow ensures the pidfile
  exists with the correct PID; #22 owns the `daemon stop` rerouting to
  consume that pidfile without `connect_and_migrate`. The two paths must
  not depend on each other for liveness.

## Chosen Approach

**Option 1 from `docs/issues/23/SPEC.md` -- the Go daemon writes the pidfile
on startup and removes it on clean shutdown.** Confirmed correct after
auditing `src/striatum/cli/daemon.py:54-59`: `launch_daemon_start` calls
`run_go_daemon_foreground`, which builds an argv and ends with
`os.execv(str(binary), command)`. `execv` replaces the current Python
process in place; there is no fork and no parent Python process after the
exec succeeds. Net consequences for the three SPEC proposals:

- **Option 2 (Python launcher writes before `execv`)** is technically
  viable because the Python launcher's `os.getpid()` survives the
  `execv` and equals the Go daemon's PID afterward. It is rejected as
  the canonical path because (a) it leaks a pidfile on `execv` failure
  with no in-process cleanup hook, (b) it only covers daemons started
  through the Python CLI -- a Go binary launched directly (systemd,
  containers, ops scripts, `STRIATUMD_GO_BIN`-pointed binaries) would
  still ship without a pidfile, and (c) it splits pidfile lifecycle
  across two languages.
- **Option 3 (status falls back to socket-existence + RPC)** is not a
  replacement for option 1. The pidfile's job is to record the PID; a
  socket probe cannot recover that information. If anything, option 3
  is a possible future complement to option 1 for very-stale-pidfile
  situations; it is explicitly NOT in this scope.
- **Option 1 (Go daemon writes on startup)** is chosen because it (a)
  works regardless of launcher, (b) lets the existing
  `signal.NotifyContext(os.Interrupt, SIGTERM)` shutdown path in
  `go/cmd/striatumd/main.go:130-133` own pidfile removal via `defer`,
  (c) mirrors the atomic temp+rename pattern already present in
  `go/pkg/supervisor/pointer.go:46-60`, and (d) keeps the on-disk
  contract owned by the process that actually owns the PID.

The pidfile path itself MUST match the path Python clients already read:
`<runtime_dir>/striatumd.pid` where `runtime_dir` honors
`$STRIATUM_DAEMON_RUNTIME_DIR` → darwin
`~/Library/Caches/striatum/runtime` → `$XDG_RUNTIME_DIR/striatum` →
`~/.cache/striatum/runtime`, per `src/striatum/daemon_runtime.py:24-33`.
Because the Python launcher already passes `--socket
<runtime_dir>/striatumd.sock` (`src/striatum/cli/daemon.py:43-44`), the Go
side can simply derive the pidfile path as
`filepath.Join(filepath.Dir(socketPath), "striatumd.pid")`. A Go binary
launched directly with `--socket` will produce the same pidfile location
without needing a separate `--pid-file` flag.

## Files and Directories in Scope

The implementer should change only these files:

- `go/cmd/striatumd/main.go` -- after socket-dir is created and before the
  listener accepts connections, write the pidfile atomically (temp file +
  rename, mode 0600). The signal-driven `cancel()` path already handles
  shutdown; add a `defer os.Remove(pidPath)` (or equivalent placed after
  `cancel()` but before `main` returns) so SIGTERM/Interrupt always
  removes the pidfile. Refuse to start if an existing pidfile points to a
  *live* PID owned by another `striatumd` process (mirror Unix daemon
  convention; do not refuse on stale pidfile -- overwrite it).
- `go/pkg/striatumd/` (NEW package, optional) OR `go/cmd/striatumd/` --
  factor the pidfile read/write/remove helpers into a small Go function
  set so they are unit-testable without standing up a full daemon. The
  atomicity pattern (write `.tmp` then `os.Rename`) MUST match
  `go/pkg/supervisor/pointer.go:46-60`. New tests live alongside the
  helpers (e.g. `go/cmd/striatumd/pidfile_test.go` or a `go/pkg/...`
  variant).
- `go/cmd/striatumd/handler_coverage_test.go` -- extend ONLY if a new
  flag or describe field is added (none planned; skip if untouched).
- `src/striatum/cli/daemon.py` -- only if the implementer chooses to add
  a `--pid-file` argument to expose the path explicitly. Default
  behavior should not require any CLI change here; treat this file as
  read-only unless a flag is genuinely necessary.
- `src/striatum/daemon_runtime.py` -- no behavior change expected;
  `pid_path()` already returns the canonical path. The implementer MAY
  add a docstring note pointing at the Go-side writer for traceability.
- `src/striatum/daemon_pg/client_admin.py` -- no logic change required.
  `_read_pid`, `_pid_alive`, and `daemon_status_pg` already return the
  right shape once the pidfile is written. The implementer MAY add a
  short comment near `_read_pid` (line 326) crediting the Go-side
  writer.
- `tests/daemon_pg/` -- add a Python-level integration test that asserts
  `daemon_status_pg` returns `running:true, pid:<int>` when a pidfile
  exists with a live PID, and `running:false` when the PID is dead.
  Place under `tests/daemon_pg/test_pidfile_status.py` or extend an
  existing reads test.
- `tests/cli/` -- if a CLI flag is added, exercise it here; otherwise no
  change.
- `docs/issues/23/build/HANDOFF.md` -- required implementer handoff.
- `docs/issues/23/review/REVIEW.md` -- required verifier artifact.
- `docs/POSTGRES_TRANSITION.md` and `docs/CLI_REFERENCE.md` -- one-line
  edits ARE allowed if and only if existing operator wording references
  `daemon status` PID semantics; otherwise leave alone.

## Files and Directories Out of Scope

The implementer must not touch:

- `go/pkg/supervisor/` -- the supervisor pidfile pattern (per-scratch-dir
  `<scratch>/<supervisor_id>/pid`) is a DIFFERENT contract for the
  process-supervisor lifecycle. Do not unify these; only mirror the
  atomic-write idiom. Especially do not rename, refactor, or move
  `WritePidfile`/`ReadPidfile` in `go/pkg/supervisor/pointer.go`.
- Legacy SQLite paths under `src/striatum/service*.py` and
  `tests/test_service*.py`. They reference `striatumd.pid` for the
  retired Python daemon registry; they are not the active code path
  and must not be reanimated.
- Any other daemon RPC handler: do not modify `health_pg`, `daemon_stop`,
  `daemon_sweep_once`, `_require_pg_auth`, `daemon_doctor_records_pg`,
  the bootstrap admin token logic, audit chain code, or the GO `admin`,
  `apply`, `reads`, `mutations`, `recovery`, `repositories`, or `rpc`
  packages.
- `src/striatum/daemon_pg/connection.py` -- the migration/owner-role
  story is GH #22's territory, not this workflow's.
- `daemon stop` business logic in `src/striatum/daemon_pg/client_admin.py:346-361`.
  This workflow GUARANTEES the pidfile exists and is correct; #22 owns
  the actual `daemon stop` reroute that consumes the pidfile without
  routing through `connect_and_migrate`.
- `.striatum/` and any per-target-repo runner scratch directory.
- `docs/dogfood/`, `examples/`, `prompts/` historical fixtures.
- The Go daemon's blob/S3 setup (`loadBlobClient`) and the recovery
  scheduler.
- Schema migrations and `daemon_pg/sql/`.

## Acceptance Checklist

Each numbered item maps 1:1 to a bullet under "Acceptance / Definition of
done" in `docs/issues/23/SPEC.md`. The verifier should cite each with
file:line evidence and a runnable verification.

1. **GH23-1 (Pidfile present with daemon PID).** A running daemon causes
   `<runtime_dir>/striatumd.pid` to exist, contain a valid integer equal
   to the daemon process's PID, with file mode exactly `0600`, owned by
   the running user. Evidence: `ls -l` on the pidfile + numeric content
   match against `pgrep -f striatumd` or the launcher's reported PID.
2. **GH23-2 (`daemon status` is truthful).** Within ~1 second of
   `striatum daemon start` succeeding, `striatum daemon status` returns
   JSON with `running: true` and `pid: <int>` matching the daemon
   process. No manual `echo <pid> > striatumd.pid` workaround is
   required. Evidence: capture the JSON before any manual intervention.
3. **GH23-3 (Clean shutdown removes pidfile; crash leaves stale pidfile
   that status detects).** `kill -TERM <daemon-pid>` causes the daemon
   to exit and `striatumd.pid` to disappear within ~1 second. Killing
   via `kill -9` (or any unclean exit) leaves the pidfile behind, but
   `striatum daemon status` reports `running: false` because
   `_pid_alive` (`src/striatum/daemon_pg/client_admin.py:336-343`)
   rejects the dead PID. Evidence: two scripted scenarios with
   `ls`/`stat`/`status` captures.
4. **GH23-4 (`daemon stop` consumes the pidfile, composes with #22).**
   With this change alone, `_read_pid` returns a live PID, so
   `daemon_stop` (`src/striatum/daemon_pg/client_admin.py:346-361`)
   sends SIGTERM correctly. The composition with GH #22 (no
   `connect_and_migrate` requirement on stop) is verified at #22's
   merge; this workflow is responsible for ensuring the pidfile is
   present and correct. Evidence: a Python-level integration test
   asserting `daemon_stop` succeeds against a live daemon when no
   migration pressure is present, plus a note that the
   no-PG-stop path is GH #22's acceptance to demonstrate.
5. **GH23-5 (Tests cover startup-writes, shutdown-removes, stale-pid).**
   - Go-side unit test: `WritePidfile`-equivalent helper writes
     atomically (temp + rename), refuses to overwrite a pidfile that
     points to a live foreign PID, and overwrites a stale pidfile.
   - Go-side integration test (lightweight): start `striatumd` against
     a temp socket dir, observe pidfile materialization within a
     polling window, send SIGTERM, observe pidfile removal.
   - Python-side test in `tests/daemon_pg/`: synthesize a pidfile
     with `os.getpid()` and assert `daemon_status_pg` returns
     `running: true`; replace it with an unused PID and assert
     `running: false`.

## Verification Commands

The implementer should run at least:

```bash
# Go-side: build and unit tests for the new pidfile helpers.
make -C go test || (cd go && go test ./...)

# Python-side: targeted tests for daemon_status_pg + new pidfile test.
make test PYTEST_ARGS='tests/daemon_pg/test_pidfile_status.py tests/daemon_pg/handlers/reads/'

# Linters/typecheck for the Python touchpoints.
make lint
make typecheck

# Manual end-to-end (must be documented in HANDOFF.md):
#   1. striatum daemon start &           # or under a process manager
#   2. striatum daemon status            # expect running:true, pid:<N>
#   3. ls -l "$XDG_RUNTIME_DIR/striatum/striatumd.pid"  # mode 0600, owner=user
#   4. cat "$XDG_RUNTIME_DIR/striatum/striatumd.pid"    # matches the PID
#   5. kill -TERM <pid>                                 # clean shutdown
#   6. ls "$XDG_RUNTIME_DIR/striatum/striatumd.pid"     # must not exist
#   7. start daemon, kill -9, then daemon status        # running:false
```

If the local environment cannot bring up Postgres, the HANDOFF must
explicitly call out which manual steps were skipped and which were
substituted with a `--migrate=false` invocation or a unit-test analogue.

## Risks and Parallel-Workflow Conflicts

- **GH #22 composition risk.** Acceptance item 4 of #23 only fully
  resolves when #22's `daemon stop` reroute (no `connect_and_migrate`
  on stop) lands. If #22 has not yet shipped at fix-time, the
  implementer must still produce the pidfile and document in HANDOFF
  that #22 is the path that consumes it. The two changes are
  independently mergeable; this workflow does NOT block on #22.
- **Direct-launch parity.** If anyone launches the Go binary directly
  (no Python launcher), the pidfile path falls out of the `--socket`
  flag's parent directory. If the operator omits `--socket`, the Go
  default (`defaultSocketPath()` in `main.go:476-482`) is
  `$XDG_RUNTIME_DIR/striatum/daemon-go.sock` -- which differs from the
  Python-side `<runtime_dir>/striatumd.sock`. The implementer must
  decide whether to (a) align the Go default with the Python runtime
  dir, or (b) accept that the Python launcher always supplies an
  explicit `--socket`. The simplest path is (b); call this out in
  HANDOFF if the Go default is left alone.
- **Race window on startup.** Between daemon start and pidfile
  appearance, `daemon status` will briefly report `running: false`. The
  implementer should write the pidfile *before* the socket is bound so
  any client capable of reaching the daemon already sees a pidfile.
- **Stale-pidfile races on overwrite.** Two `striatumd` instances
  trying to start concurrently must not both win. The atomic
  rename ensures a single writer wins, but the live-foreign-PID check
  must precede the rename to avoid two daemons silently coexisting on
  one runtime dir. This is a startup-only check, not a runtime
  enforcement.
- **Permissions on `$XDG_RUNTIME_DIR/striatum`.** Already created with
  mode 0700 by `ensure_private_dir` in Python flows and by
  `os.MkdirAll(filepath.Dir(socketPath), 0o700)` in `main.go:217`. No
  new mkdir is required; the pidfile inherits the parent's privacy.
- **No interaction with the supervisor pidfile layer.** Reviewers
  should not flag duplication with `go/pkg/supervisor/pointer.go`; the
  two pidfile contracts cover different processes and different
  on-disk locations.
- **Reviewer-context risk.** GH #23 verify is fresh-context and runs
  the `compliance_license` posture. The implementer should keep
  changes free of third-party copy-paste so the posture passes
  trivially.
