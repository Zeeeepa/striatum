# GH #23 — daemon status reads striatumd.pid but no code path writes it

Source: https://github.com/halbritt/striatum/issues/23

## Summary

`striatum daemon status` reads a pidfile at `/run/user/<uid>/striatum/striatumd.pid` (`daemon_pg/client_admin.py:314,326`, `daemon_runtime.py:44`) and reports `running: <bool>` based on whether that PID is alive. **No code path writes this file.** Net effect: a perfectly healthy daemon is always reported as `running: false, pid: null`.

This looks like a regression in 1.55.0 — the previous Python-wrapped daemon start path (`/usr/bin/python3 .../striatum daemon start --json` as a long-lived process, observed on 2026-05-15) presumably wrote the pidfile and stayed parented. The new path (`launch_daemon_start` → `run_go_daemon_foreground`, `cli/daemon.py:54-59`) execs the Go binary directly in the foreground, and neither side touches the pidfile.

## Repro

1. `striatum daemon start --json &` (or under a process manager — anything that gets it up).
2. Confirm the daemon is up: `striatum daemon health` → `ok`, socket exists at `/run/user/<uid>/striatum/striatumd.sock`, the postgres connection is live.
3. `striatum daemon status` → `{"data":{...,"pid":null,"running":false,...}}`.
4. `ls /run/user/<uid>/striatum/` shows `striatumd.sock` and `client-token` but no `striatumd.pid`.
5. Manual fix: `echo <pid> > /run/user/<uid>/striatum/striatumd.pid` — now `daemon status` correctly reports `running:true, pid:<pid>`.

## Evidence

- `grep -rn 'striatumd.pid\|WritePidfile' go/` → only `go/pkg/supervisor/pointer.go` writes pidfiles, and only for *supervisor* scratch dirs, never for the daemon itself.
- `grep -rn 'pid_path\|striatumd.pid' src/striatum/cli/daemon.py` → no matches. `launch_daemon_start` does not write the pidfile before exec.
- `grep -rn 'pid_path' src/striatum/daemon_pg/client_admin.py` → only reads (lines 76, 314, 326).

Both sides assume the other writes the pidfile; neither does.

## Acceptance / Definition of done

A solution must satisfy each of:

1. **A running daemon causes `/run/user/<uid>/striatum/striatumd.pid` to exist** with the daemon's PID, mode 0600, owned by the running user.
2. **`striatum daemon status` returns `running:true` and `pid:<pid>`** within ~1s of `striatum daemon start` succeeding (no manual `echo` workaround).
3. **Clean shutdown removes the pidfile.** SIGTERM → daemon exits → pidfile gone. Crash leaves a stale pidfile, but `daemon status` correctly detects the dead PID and reports `running:false` (existing `_pid_alive` check).
4. **`daemon stop` falls back to the pidfile** even if it can't connect to Postgres (compose with #22). The two recovery paths must not depend on each other.
5. **Tests cover startup-writes-pidfile, shutdown-removes-pidfile, and stale-pidfile-detection.** Unit-level for the Go side; integration smoke if there's an existing daemon test harness.

## Suggested fix (proposals; pick one)

1. **Go daemon writes it on startup** (preferred — it's the process whose PID is being recorded). Add to the `striatumd` main: before serving the socket, write `os.Getpid()` to `<runtime_dir>/striatumd.pid` with mode 0600, and `os.Remove` it on clean shutdown / signal handler.
2. **Python launcher writes it before exec** — get the future pid via fork-then-exec rather than direct exec; `run_go_daemon_foreground` would write `os.getpid()` of the child before `os.execv`-ing. Slightly more fragile because crash-during-write leaves a stale pidfile.
3. **`daemon status` falls back to socket-existence + RPC liveness** when the pidfile is absent. Less truthful (pid is genuinely unknown), but stops reporting `running:false` on a healthy daemon. Probably belongs as a complement to (1), not a replacement.

(1) is the canonical Unix-daemon pattern and the smallest change.

## Why this matters

- Every `striatum daemon status` invocation is wrong on the live daemon, which makes operator runbooks misleading and breaks any monitor/watchdog that uses `daemon status` to detect a crashed daemon.
- It interacts badly with #22 (daemon stop unusable when migrations are pending): if you're stuck and the pidfile is also missing, you have no programmatic way to find the daemon to kill it — you have to `ps aux | grep striatumd`.

## Provenance

Hit while bringing the local daemon back up after the RFC 0072 / migration 0009 landing on 2026-05-18. The daemon was healthy by every direct measure (socket, health RPC, PG idle connection) but `status` insisted it wasn't running. See conversation transcript 2026-05-18.

Related:

- #22 — owner-role migration gap. `daemon stop` is broken via that flaw, so the missing pidfile compounds the recovery story.
