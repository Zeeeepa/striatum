# Triage -- GH #23 scope

You are the triager for this issue workflow. Produce only the declared
scope artifact for this workflow. Do not implement source changes.

## Read

1. `docs/issues/23/SPEC.md`
2. `src/striatum/daemon_runtime.py` -- `pid_path()`, `runtime_dir()`,
   socket and token-file conventions.
3. `src/striatum/daemon_pg/client_admin.py` -- the read side
   (`_read_pid`, `_pid_alive`, the `daemon_status_pg` return shape).
4. `src/striatum/cli/daemon.py` -- `launch_daemon_start` →
   `run_go_daemon_foreground`; understand how the Python CLI hands off
   to the Go binary and whether it forks or directly execs.
5. `go/cmd/striatumd/` -- main entry, signal handling, socket bind
   sequence; this is where a Go-side pidfile write most naturally
   lives.
6. `go/pkg/supervisor/pointer.go` -- the existing pidfile pattern for
   *supervisor* scratch dirs; mirror its atomicity (temp file + rename)
   if practical.
7. `docs/issues/22/SPEC.md` -- the sibling owner-migration issue. The
   acceptance bullet "daemon stop falls back to the pidfile" composes
   across the two.
8. Any existing daemon test harness under `go/cmd/striatumd/`,
   `go/pkg/...`, or `tests/daemon_pg/` that exercises startup/shutdown.

## Output

Write `docs/issues/23/SCOPE.md` with `striatum.synthesis.v1` front matter
and the exact `author:` line from the work packet. Include:

- the exact files in scope for the fix (Go daemon main, Python
  launcher if a launcher-side write is preferred, daemon_status_pg
  helpers, tests);
- the exact files out of scope (do NOT touch supervisor pidfile
  handling, legacy SQLite, or unrelated daemon RPC handlers);
- an acceptance checklist with one numbered check per bullet under
  "Acceptance / Definition of done" in `docs/issues/23/SPEC.md`;
- the chosen approach among the proposals (Go-daemon writes /
  Python-launcher writes / fallback-in-status). The default
  recommendation in SPEC.md is "Go daemon writes on startup"; the
  triager must confirm this is correct given the launcher's exec
  shape, or justify a different choice;
- verification commands (`go test ./...`, the relevant Python test
  target, manual `striatum daemon start` + `daemon status` round-trip,
  `kill -TERM` cleanup, crashed-daemon stale-pid detection);
- risks and likely conflicts with parallel issue workflows
  (especially #22, since `daemon stop` recovery touches both).
