---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
---

author: reviewer-unknown-model-001

# GH #23 Verification Review

Final verdict: `accept_with_findings`.

No license, attribution, external persistence, telemetry, or compliance
issue was found in the reviewed artifact set. The Go pidfile helper uses
only the Go standard library and mirrors an in-repo atomic-write idiom
(`go/pkg/supervisor/pointer.go:43-60`); no third-party code was copied or
re-licensed. The runtime change correctly closes GH23-1, GH23-2, and
GH23-3 with file:line evidence below. Two test-coverage gaps remain
against the SCOPE acceptance checklist (GH23-4 live-daemon
`daemon_stop` integration test and GH23-5b Go-side daemon
materialization+SIGTERM integration test). Both gaps are documented in
the implementer HANDOFF and do not block correctness; they are filed as
findings for follow-up rather than rejection.

## Acceptance Verification

1. **GH23-1 (Pidfile present with daemon PID, mode 0600, owned by user):
   accepted.** The Go daemon resolves the pidfile path from the socket
   directory at `go/cmd/striatumd/main.go:220` (`pidPath :=
   daemonPidfilePath(socketPath)`) and writes the current PID atomically
   at `go/cmd/striatumd/main.go:221` (`writeDaemonPidfile(pidPath,
   os.Getpid())`). The helper at `go/cmd/striatumd/pidfile.go:34-42`
   writes the tmp file with mode `0o600` via `os.WriteFile(tmp,
   []byte(strconv.Itoa(pid)+"\n"), 0o600)` and then `os.Rename(tmp,
   pidPath)`. Path derivation matches the Python reader: `pid_path()` in
   `src/striatum/daemon_runtime.py:44` returns
   `runtime_dir()/striatumd.pid`; `daemonPidfilePath` returns
   `filepath.Dir(socketPath)+"/striatumd.pid"`, and the Python launcher
   passes `--socket <runtime_dir>/striatumd.sock`
   (`src/striatum/cli/daemon.py:43-44`). File ownership defaults to the
   running user because the file is created by the daemon process and no
   chown is performed. Test coverage at
   `go/cmd/striatumd/pidfile_test.go:18-40` asserts the exact byte
   content (`"12345\n"`), permission mode (`0o600`), and that the tmp
   file is removed.

2. **GH23-2 (`daemon status` is truthful, no manual workaround):
   accepted.** The pidfile write occurs at
   `go/cmd/striatumd/main.go:221` *before* the socket bind at
   `go/cmd/striatumd/main.go:229` (`listener, err :=
   rpc.ListenUnix(socketPath)`). Any client that can reach the daemon
   via the socket therefore observes the pidfile already on disk; the
   "Race" adversarial probe is closed by construction. The Python
   read path (`_read_pid` at `src/striatum/daemon_pg/client_admin.py:329`
   → `daemon_status_pg` at
   `src/striatum/daemon_pg/client_admin.py:310-326`) returns `pid` and
   `running: True` whenever the pidfile exists with a live PID. The
   Python integration test at
   `tests/daemon_pg/test_pidfile_status.py:11-26` synthesizes the
   pidfile shape the Go writer produces and asserts the JSON shape.

3. **GH23-3 (Clean shutdown removes pidfile; crash leaves stale that
   status detects as not-running): accepted.** The daemon defers
   pidfile removal at `go/cmd/striatumd/main.go:224-228`:

   ```go
   defer func() {
       if err := os.Remove(pidPath); err != nil && !errors.Is(err, os.ErrNotExist) {
           log.Printf("remove daemon pidfile %s: %v", pidPath, err)
       }
   }()
   ```

   The signal-driven shutdown path at
   `go/cmd/striatumd/main.go:130-133` (`signal.NotifyContext(
   context.Background(), os.Interrupt, syscall.SIGTERM)`) cancels the
   serve context, `server.Serve` returns, and the deferred removal
   runs. For unclean exit (e.g. `kill -9`), the deferred function does
   not run and the stale pidfile remains; the Python `_pid_alive`
   check at `src/striatum/daemon_pg/client_admin.py:339-346` correctly
   reports `running: False` because `os.kill(pid, 0)` raises
   `OSError`. Stale detection is exercised by
   `tests/daemon_pg/test_pidfile_status.py:29-46`, which seeds the
   pidfile with a known-dead PID (from a completed `subprocess.Popen`)
   and asserts `status["running"] is False`.

4. **GH23-4 (`daemon stop` consumes the pidfile, composes with #22):
   accepted_with_findings.** The pidfile produced by the Go daemon is
   the same path consumed by `daemon_stop()` at
   `src/striatum/daemon_pg/client_admin.py:349-360`, which `_read_pid`s
   and sends `SIGTERM` to the recorded PID. The GH #22 reroute (the
   no-`connect_and_migrate` stop path) already lives at
   `src/striatum/daemon_pg/client_admin.py:350-360` per the comment
   "GH #22: stop must succeed when migrations are pending." Composition
   therefore works in code today. **Gap:** the SCOPE asks for "a
   Python-level integration test asserting `daemon_stop` succeeds
   against a live daemon when no migration pressure is present"
   (`docs/issues/23/SCOPE.md:181-184`); no such test was added, and the
   HANDOFF explicitly notes the manual end-to-end was not run because
   `STRIATUM_DAEMON_DB_URL` is not configured
   (`docs/issues/23/build/HANDOFF.md:59-62`). This is reported as a
   `low`-severity finding, not a rejection — the runtime contract is
   satisfied and the test is shared with GH #22's verification surface.

5. **GH23-5 (Tests cover startup-writes, shutdown-removes, stale-pid):
   accepted_with_findings.**
   - Go unit coverage is strong:
     `TestDaemonPidfilePathUsesSocketDirectory`
     (`go/cmd/striatumd/pidfile_test.go:10-16`) pins the path
     derivation; `TestWriteDaemonPidfileWritesPIDModeAndNoTmp`
     (`pidfile_test.go:18-40`) asserts atomic content, `0o600` mode,
     and tmp cleanup; `TestWriteDaemonPidfileRefusesLiveForeignStriatumd`
     (`pidfile_test.go:42-61`) asserts refusal-to-overwrite of a live
     foreign `striatumd`; `TestWriteDaemonPidfileOverwritesStalePidfile`
     (`pidfile_test.go:63-81`) asserts stale-PID overwrite;
     `TestWriteDaemonPidfileOverwritesLiveNonStriatumdPidfile`
     (`pidfile_test.go:83-101`) asserts live-non-striatumd overwrite.
   - Python coverage (`tests/daemon_pg/test_pidfile_status.py`) is
     adequate for the read path: live-PID-running and stale-PID-
     not-running.
   - **Gap:** SCOPE bullet 5b
     (`docs/issues/23/SCOPE.md:188-191`) requested a "Go-side
     integration test (lightweight): start `striatumd` against a temp
     socket dir, observe pidfile materialization within a polling
     window, send SIGTERM, observe pidfile removal." The
     `TestCleanShutdownRemovesDaemonPidfile` test at
     `pidfile_test.go:103-114` is a degenerate substitute — it calls
     `os.Remove` directly and never starts the daemon, so the
     `defer`/signal-handler path itself is not exercised. Reported as
     a `low`-severity finding for follow-up.

## Adversarial Probes

- **Race (pidfile write vs. socket bind):** closed. The pidfile is
  written at `go/cmd/striatumd/main.go:221`, then the listener is
  created at `go/cmd/striatumd/main.go:229`. Any client that can
  connect to the daemon sees the pidfile already on disk; there is no
  window where a connectable daemon lacks a pidfile.

- **Permissions / ownership:** closed.
  `os.WriteFile(tmp, ..., 0o600)` at `go/cmd/striatumd/pidfile.go:35`
  sets mode `0o600` on the tmp file before rename, and `os.Rename`
  preserves the file's mode bits, so the post-rename pidfile is
  exactly `0o600`. The parent runtime directory is already created
  with mode `0o700` at `go/cmd/striatumd/main.go:217`. Ownership is
  the running user because the daemon process performs the write and
  there is no `chown` anywhere in the new code path. Mode is asserted
  by `pidfile_test.go:30-36`. The umask of `runtime_dir()` is
  irrelevant because `os.WriteFile` honors the explicit `perm`
  argument when creating the file with `O_CREATE|O_WRONLY|O_TRUNC`.

- **Atomicity:** closed. `pidfile.go:34-41` writes
  `<path>.tmp` first, then `os.Rename(tmp, pidPath)`. Rename is
  atomic on the same filesystem (POSIX `rename(2)`). Readers either
  see no file or see the fully written file; no half-written or
  empty window is observable.

- **Cleanup-on-crash:** closed. The write helper
  (`pidfile.go:24-42`) explicitly reads any existing pidfile, calls
  `inspectPid` to decide whether the existing PID is a live foreign
  `striatumd`, and either refuses (live foreign striatumd) or
  overwrites via the atomic temp+rename (stale dead PID, or live PID
  not named `striatumd*`). Append-to-existing is impossible because
  the tmp file is created fresh with `O_TRUNC` semantics and then
  renamed; the new content always replaces the old.

- **Test coverage realism:** mostly closed. The Go unit tests use
  `t.TempDir()` and real `os.WriteFile`/`os.ReadFile`, exercising the
  file's presence on disk (not a mock); the only stubbed piece is the
  process-inspector function `inspectPid`, which is the correct seam
  to fake for hermetic unit tests. The Python test similarly writes a
  real pidfile under `tmp_path` and reads it back through
  `daemon_status_pg`. The remaining gap is the missing live-daemon
  Go integration test (see finding F2).

## Test & Verification Assessment

The HANDOFF lists `go test ./cmd/striatumd`, `go test ./...`, `pytest
tests/daemon_pg/test_pidfile_status.py`, `make lint`, and `make
typecheck` as the run set (`docs/issues/23/build/HANDOFF.md:52-58`).
The Go test file
(`go/cmd/striatumd/pidfile_test.go`) compiles against the package's
existing test bag (`pidfile_test.go:1` `package main`). The Python
integration test stubs `pid_path`, `runtime_dir`,
`_require_pg_auth`, and `_pg_instance_id` to avoid requiring a live
Postgres instance — appropriate for unit-style coverage of the read
path. The implementer correctly disclosed that manual end-to-end
startup was skipped because the shell lacks `STRIATUM_DAEMON_DB_URL`
(`docs/issues/23/build/HANDOFF.md:59-62`).

## Findings

### F1: Missing live-daemon `daemon stop` integration test (GH23-4)
- **Severity:** low.
- **Where:** `tests/daemon_pg/`, no test file covers
  `daemon_stop()` against a live daemon whose pidfile was produced by
  the Go writer.
- **What the SCOPE asked for:**
  `docs/issues/23/SCOPE.md:181-184` — "Evidence: a Python-level
  integration test asserting `daemon_stop` succeeds against a live
  daemon when no migration pressure is present."
- **Why it is `low`:** the runtime contract is verifiably correct
  (Go writes the pidfile, Python reads it, `_pid_alive` validates the
  PID; all three steps have unit coverage). The composition with
  GH #22 is already wired in code at
  `src/striatum/daemon_pg/client_admin.py:349-360`.
- **Remediation:** add a `tests/daemon_pg/test_daemon_stop_live.py`
  that uses the existing daemon test fixture (or, if absent, a
  minimal `striatumd` subprocess launched with a temp socket dir and
  the test PG URL) to assert `daemon_stop()` returns `{"stopped":
  True, "pid": <int>}` and that the daemon process is gone within
  ~1s. This is shared verification surface with GH #22 and can land
  in either workflow.

### F2: Missing Go-side end-to-end pidfile lifecycle test (GH23-5b)
- **Severity:** low.
- **Where:** `go/cmd/striatumd/pidfile_test.go:103-114`
  (`TestCleanShutdownRemovesDaemonPidfile`) currently calls
  `os.Remove(pidPath)` directly instead of exercising the daemon's
  `defer` shutdown path.
- **What the SCOPE asked for:**
  `docs/issues/23/SCOPE.md:188-191` — "Go-side integration test
  (lightweight): start `striatumd` against a temp socket dir,
  observe pidfile materialization within a polling window, send
  SIGTERM, observe pidfile removal."
- **Why it is `low`:** the deferred removal at
  `go/cmd/striatumd/main.go:224-228` is a small, well-understood
  `defer`+`os.Remove` pair, and the signal context wiring at
  `go/cmd/striatumd/main.go:130-133` is shared with all other
  shutdown logic in the daemon (which already has end-to-end
  smoke coverage elsewhere). The risk of a regression slipping
  past the unit tests is bounded.
- **Remediation:** add a `go/cmd/striatumd/pidfile_integration_test.go`
  build-tagged for the integration test set that builds the daemon
  binary (or invokes `main` in a goroutine with a controllable
  context), points it at a `t.TempDir()` socket path, polls until
  the pidfile exists, sends SIGTERM (or cancels the context), and
  asserts the pidfile disappears within ~1s. Skip if the test
  environment lacks the test Postgres URL, mirroring the HANDOFF's
  disclosure.

## Compliance / License Check

- All new Go code (`go/cmd/striatumd/pidfile.go`,
  `go/cmd/striatumd/pidfile_test.go`) imports only Go standard library
  packages (`fmt`, `os`, `os/exec`, `path/filepath`, `strconv`,
  `strings`, `syscall`). No vendored or third-party code.
- The atomic temp+rename idiom mirrors the in-repo pattern at
  `go/pkg/supervisor/pointer.go:46-60`, which is the project's own
  code; no external attribution required.
- New Python test (`tests/daemon_pg/test_pidfile_status.py`) uses
  only stdlib (`os`, `subprocess`, `pathlib`, `typing`) plus
  `striatum.daemon_pg.client_admin`.
- No transcripts, telemetry, hosted-service hooks, or external
  persistence introduced.
- No copy-paste from third-party sources detected.

No unresolved license, attribution, or compliance issue.

## Verdict Rationale

The implementation correctly closes the runtime gap that motivated
GH #23 — a healthy daemon now produces a valid `striatumd.pid` with
the right path, mode, and contents, the pidfile precedes the socket
bind so `daemon status` cannot race the daemon's startup, and
clean shutdown removes it. All five adversarial probes pass.
The two test-coverage gaps (F1, F2) are documented in the HANDOFF
and shared with GH #22's verification surface; neither
contradicts the SPEC's behavioral acceptance bullets in code. I
record `accept_with_findings`.
