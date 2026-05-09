---
title: "RFC 0020 step 3 build handoff (dogfood-015)"
author: implementer-codex-gpt-5.5-001
date: 2026-05-09
---

# Build handoff: `striatum recovery watch` daemon

## Scope

RFC 0020 step 3 V1 (per `decisions/V1_ACCEPTANCE.md`):

- Add `striatum recovery watch --run-id <id>` long-lived sweeper.
- Wrap the existing `run_auto_sweep` orchestrator in a sleep loop;
  do not duplicate sweep logic.
- Single-instance enforcement via pidfile under
  `.striatum/scratch/recovery-watch-<run_id>.pid` (O_EXCL +
  alive-check; stale → overwrite, alive → exit 4).
- `SIGTERM` / `SIGINT` graceful shutdown via
  `threading.Event.wait()` interruptible sleep.
- JSONL emission per sweep + a final `watch_exit` envelope; flush
  every line.
- Exit on terminal run state by default; `--no-exit-on-terminal`
  keeps looping. `--max-sweeps N` caps for tests / probes.

## Files

### New

- `src/striatum/recovery/watch.py` — orchestrator (~150 lines).
  Public exports: `PIDFILE_COLLISION_EXIT_CODE`, `pidfile_path`,
  `run_watch`. Pidfile + signal handling helpers are private
  (`_acquire_pidfile`, `_pid_alive`, `_emit_envelope`, `_emit_line`).
- `tests/test_recovery_watch.py` — 8 cases (subprocess + module-
  level). Includes a `SIGTERM`-mid-30s-sleep clean-shutdown test
  and a stale-pidfile cleanup test.

### Modified

- `src/striatum/recovery/__init__.py` — re-export the new public
  surface.
- `src/striatum/cli/parser.py` — add the `recovery watch`
  subparser (CLI shape pinned by `DESIGN_SYNTHESIS.md`).
- `src/striatum/cli/dispatch.py` — wire the dispatch path; raises
  `InvalidTransitionError` (exit 4) on pidfile collision so the
  CLI exit code matches the documented contract.

## CLI shape

```
striatum recovery watch --run-id <id>
  [--interval-seconds N]                 # default 60
  [--exit-on-terminal | --no-exit-on-terminal]   # default exit
  [--max-sweeps N]                       # default unbounded
  [--autonomous-review-requeue {true,false}]
  [--autonomous-process-reconcile {true,false}]
  [--max-requeue N] [--checkpoint-timeout S] [--eligible-after S]
  [--json]                               # JSONL on stdout
```

`recovery_policy` resolution mirrors `recovery auto`: the workflow
block is the floor, CLI flags layer on top, runner defaults fill
gaps. Resolution happens once at watcher startup.

## Pidfile semantics

- Path: `<repo>/.striatum/scratch/recovery-watch-<run_id>.pid`.
- Open with `O_CREAT | O_EXCL | O_WRONLY`. On `EEXIST`, read the
  pid; if alive, exit 4 with `another recovery watch is active
  (pid <N>)`. If not alive, unlink and retry once.
- On clean exit (any reason — terminal, max-sweeps, signal,
  exception), unlink the pidfile.

## Signal handling

- `SIGTERM` and `SIGINT` set a `threading.Event`. The main loop
  uses `event.wait(timeout=interval_seconds)` — the wait returns
  `True` immediately when the event is set, so a 60-second sleep
  collapses to milliseconds on signal.
- Tests pass `install_signal_handlers=False` so they can run
  inside the test process without hijacking pytest's handlers.
- The final `watch_exit` envelope reports `reason` ∈ {`signal`,
  `max_sweeps_reached`, `terminal_run_state`, `error`}.

## JSONL envelope shapes

Per-sweep (one line per loop iteration):

```json
{"event": "sweep", "run_id": "...", "swept_at": "...",
 "actions_taken": [...], "swept_total": N}
```

Final line:

```json
{"event": "watch_exit", "run_id": "...", "reason": "...",
 "swept_total": N, "exited_at": "..."}
```

Human (non-`--json`) mode prints a one-line summary per sweep
plus a `watch exited (reason)` final line.

## Test coverage (`tests/test_recovery_watch.py`)

1. `test_watch_runs_max_sweeps_then_exits` — 3 sweeps + 1 exit
   envelope; `reason == "max_sweeps_reached"`.
2. `test_watch_emits_jsonl_envelopes_when_json_set` — every line
   parses as JSON.
3. `test_watch_pidfile_collision_refused` — current pid in
   pidfile → exit 4 + documented message.
4. `test_watch_stale_pidfile_overwritten` — dead pid → cleaned,
   watch starts, exits cleanly.
5. `test_watch_pidfile_path_under_scratch` — path matches
   spec.
6. `test_watch_no_exit_on_terminal_continues` — keeps looping
   past terminal until `--max-sweeps` wins.
7. `test_watch_sigterm_clean_shutdown` — `SIGTERM` mid-30s sleep
   → exit 0, pidfile removed, `reason == "signal"`.
8. `test_run_watch_returns_zero_on_max_sweeps` — direct module
   call (no subprocess) with signal handlers off.

All 8 pass; full suite green (318 passed).

## Smoke

```
$ striatum --repo . recovery watch \
    --run-id run_2cbcdb5fdba246c79538965e52143333 \
    --max-sweeps 2 --interval-seconds 0 --json
{"event":"sweep", ...}
{"event":"sweep", ...}
{"event":"watch_exit","reason":"max_sweeps_reached","swept_total":2}
```

## Out of scope (V1)

- Multi-run watch (one watcher per run; the pidfile name is
  per-run by design — operators run one watcher per run).
- systemd unit / launchd plist generation.
- Web-UI control.
- Telemetry / metrics export beyond the JSONL stream.

## Follow-ups (carried to TODO)

- Document operator pattern for "watch every active run" (e.g.,
  shell loop over `striatum list runs --state running`) in
  `HOW_TO_HUMAN.md`. Not blocking — the per-run watcher is
  already useful in isolation.
