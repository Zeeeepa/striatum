---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0020 step 3 — `recovery watch` shape research

author: researcher-codex-gpt-5.5-001

Date: 2026-05-09

## What already exists

- `striatum.recovery.run_auto_sweep(conn, *, run_id, repo,
  policy, dry_run, hook_runner, now)` — pure orchestrator over a
  single sweep. Returns the structured envelope. Already covers
  the entire recovery workflow.
- `striatum.recovery.resolve_policy(workflow_payload,
  cli_overrides)` — merges defaults + workflow + CLI overrides.
- `cli/dispatch.py` already loads the workflow snapshot and
  resolves the policy for `recovery auto`.
- `.striatum/scratch/` is the canonical directory for per-run
  scratch state (used by RFC 0009 supervisor pipes).

## What step 3 adds

A daemon that wraps `run_auto_sweep` in a loop. Concretely:

```
striatum recovery watch
  --run-id <id>
  [--interval-seconds <n>]   # default 60
  [--exit-on-terminal]       # default true
  [--no-exit-on-terminal]    # opt out for paranoid CI
  [--max-sweeps <n>]         # cap; default unlimited
  [--autonomous-review-requeue]
  [--autonomous-process-reconcile]
  [--checkpoint-timeout <seconds>]
  [--max-requeue <n>]
  [--eligible-after <seconds>]
  [--json]                   # emit JSONL: one envelope per sweep
```

Operating loop:

1. Acquire single-instance pidfile at
   `.striatum/scratch/recovery-watch-<run_id>.pid`. If another
   process holds the pidfile *and* its PID is alive, refuse with
   exit 4. If the PID is gone (stale pidfile), overwrite it.
2. Install SIGTERM and SIGINT handlers that flip a `stop` event.
3. Loop:
   - Open SQLite connection (per sweep — short-lived, mirrors
     existing pattern).
   - Call `run_auto_sweep(...)`. Print the envelope as JSONL on
     stdout if `--json` is set.
   - If `--exit-on-terminal` is set (default), check the run's
     state. Exit 0 when terminal.
   - If `--max-sweeps` is set, decrement; exit 0 when zero.
   - Sleep `interval-seconds`, interruptible by `stop`.
4. On exit (clean or signal), remove the pidfile and emit a
   final summary line.

## Single-instance pidfile contract

- Path: `.striatum/scratch/recovery-watch-<run_id>.pid`.
- Contents: just the integer PID, one line.
- Lock pattern: `O_CREAT | O_EXCL` open. If that fails, read the
  existing PID. If the PID is alive (`os.kill(pid, 0)` succeeds),
  refuse with exit 4. If dead, overwrite. Race-safe enough for
  single-machine use; cross-machine isn't in scope (D020).

## Signal handling

- `SIGTERM` and `SIGINT` flip a `threading.Event`.
- The sleep between sweeps uses `event.wait(interval)` so a
  signal interrupts the sleep immediately.
- On exit, the pidfile is removed and a final
  `{"event": "watch_exit", "reason": "...", "swept_total": N}`
  line is emitted (when `--json` is set).

## JSONL output shape

When `--json` is set, stdout is one JSON object per line:

- One `run_auto_sweep` envelope per sweep (existing shape from
  step 1).
- One terminal `{"event": "watch_exit", ...}` line on shutdown.

Without `--json`, stdout prints a human-readable one-line
summary per sweep (timestamp + counts) and a final exit message.

## Test plan

`tests/test_recovery_watch.py`:

- `test_watch_runs_max_sweeps_then_exits` — `--max-sweeps 3
  --interval-seconds 0` runs 3 sweeps, exits 0.
- `test_watch_emits_jsonl_envelopes_when_json_set` — each line
  parses as JSON; one sweep envelope per iteration plus a final
  `watch_exit` line.
- `test_watch_pidfile_collision_refused` — start a watch, then
  start another against the same run; second exits 4 with a
  clear error message.
- `test_watch_stale_pidfile_overwritten` — write a pidfile
  pointing at a dead PID; watch starts cleanly.
- `test_watch_exits_on_run_terminal` — drive a run to terminal;
  watch exits without `--max-sweeps`.
- `test_watch_no_exit_on_terminal_keeps_looping` — `--no-exit-on-terminal --max-sweeps 2`
  keeps going past terminal, hits the cap.
- `test_watch_sigterm_clean_shutdown` — send SIGTERM mid-sleep;
  process exits 0, pidfile removed, final exit line present.

## Friction anticipated

- **Subprocess testing.** Tests need to spawn `striatum recovery
  watch` as a subprocess to exercise signal handling and pidfile
  semantics. The existing `tests/test_service.py` and
  `tests/test_web_ui.py` provide the spawn/wait pattern.
- **Time control.** `--interval-seconds 0` makes the loop
  effectively tight; useful for tests. For prod, default 60 is
  conservative.
- **Pidfile cleanup on crash.** The `O_EXCL` + alive-check
  pattern makes a crashed-watch pidfile self-healing on the
  next start. No daemon manager needed.
- **Stdout buffering.** Use `sys.stdout.flush()` (or
  `print(..., flush=True)`) after each line so JSONL consumers
  see envelopes as they happen, not buffered in 4 KB chunks.

## Recommended order

1. Add `striatum.recovery.watch` module with the loop body.
2. Wire `recovery watch` into the parser + dispatch.
3. Tests in `tests/test_recovery_watch.py`.
4. Doc updates (SPEC, CLI_REFERENCE, HOW_TO_HUMAN, CHANGELOG).
5. Bump v1.5.1 (patch — closes a deferred slice of an already
   accepted RFC) or v1.6.0 (minor — RFC 0020 → "accepted (V1)"
   as the suffix-free state).

V1.6.0 is the cleaner story since it lets RFC 0020 drop its
"step 3 deferred" qualifier.
