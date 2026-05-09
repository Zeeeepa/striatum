---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0020-autonomous-stalled-run-recovery.md", "docs/dogfood/015/research/WATCH_SHAPE.md", "src/striatum/recovery/auto.py"]
---

# RFC 0020 step 3 Design Synthesis

author: designer-codex-gpt-5.5-001

Date: 2026-05-09
Target: RFC 0020 step 3 — `recovery watch` daemon. Closes the
RFC's last deferred slice; transitions RFC 0020 from
`accepted (V1; step 3 deferred)` to `accepted (V1)`.

## Locked Contracts

### CLI surface

```text
striatum recovery watch
  --run-id <id>
  [--interval-seconds <n>]            # default 60
  [--exit-on-terminal | --no-exit-on-terminal]   # default exit
  [--max-sweeps <n>]                  # cap; default unlimited
  [--autonomous-review-requeue]
  [--autonomous-process-reconcile]
  [--checkpoint-timeout <seconds>]
  [--max-requeue <n>]
  [--eligible-after <seconds>]
  [--json]
```

The CLI overrides match `recovery auto` so the two verbs are
substitutable from a workflow / cron perspective.

### Module + entry point

New `src/striatum/recovery/watch.py` exposes:

```python
def run_watch(
    repo: Path,
    *,
    run_id: str,
    interval_seconds: float = 60.0,
    exit_on_terminal: bool = True,
    max_sweeps: int | None = None,
    cli_overrides: Mapping[str, Any] | None = None,
    json_output: bool = False,
    stdout: TextIO = sys.stdout,
    now: Callable[[], str] = utc_now,
) -> int:
    """Run sweeps until terminal / max_sweeps / signal. Return
    exit code (0 = clean, 4 = pidfile collision)."""
```

Re-exported from `striatum.recovery.__init__`.

### Pidfile

Path: `.striatum/scratch/recovery-watch-<run_id>.pid`.

Acquisition: `os.open(path, O_CREAT | O_EXCL | O_WRONLY)`. On
`FileExistsError`:

1. Read the pid from the file.
2. `os.kill(pid, 0)` — if it raises `ProcessLookupError`, the
   pidfile is stale; `os.unlink` it and retry the `O_EXCL` open.
3. If `os.kill` succeeds (process is alive), return exit 4 with
   the message `another recovery watch is active for <run_id>
   (pid <pid>)`.

Cleanup: `try/finally` removes the pidfile on every exit path.

### Signal handling

```python
stop_event = threading.Event()
def _handler(signum, frame):
    stop_event.set()
signal.signal(signal.SIGTERM, _handler)
signal.signal(signal.SIGINT, _handler)
```

The sleep between sweeps is `stop_event.wait(interval_seconds)`
so SIGTERM/SIGINT interrupt immediately. The handler is
re-entrant-safe (just sets a bool).

### Loop body

```python
swept = 0
exit_reason = None
try:
    while not stop_event.is_set():
        with connect(repo) as conn:
            envelope = run_auto_sweep(
                conn, run_id=run_id, repo=repo, policy=policy
            )
        emit(envelope)
        swept += 1
        if exit_on_terminal:
            run_state = read_run_state(conn, run_id)
            if run_state in TERMINAL_RUN_STATES:
                exit_reason = "run_terminal"
                break
        if max_sweeps is not None and swept >= max_sweeps:
            exit_reason = "max_sweeps_reached"
            break
        # Interruptible sleep.
        if stop_event.wait(interval_seconds):
            exit_reason = "signal"
            break
finally:
    pidfile.unlink(missing_ok=True)
    emit_exit(exit_reason or "clean", swept)
return 0
```

`policy` is resolved once at the top of `run_watch` from the
workflow snapshot + CLI overrides (same flow as `recovery auto`).

### JSONL output

When `json_output=True`:

- Each sweep emits one line (the existing `run_auto_sweep`
  envelope).
- A terminal `{"event": "watch_exit", "reason": <reason>,
  "swept_total": N, "exited_at": <utc>}` line on shutdown.
- `print(json.dumps(envelope), flush=True)` — flush every
  line so JSONL consumers see envelopes immediately, not in
  4 KB-buffered chunks.

When `json_output=False`:

- Each sweep prints a single human-readable line:
  `swept_at=<ts> actions=<n> escalations=<n> still_stuck=<n>`.
- Final line: `recovery watch exit: reason=<reason>
  swept=<N>`.

### Doc updates (in same PR)

- `docs/SPEC.md` § "Recovery": add a paragraph naming the
  daemon alongside `recovery auto`.
- `docs/UBIQUITOUS_LANGUAGE.md`: glossary entries `recovery
  watch`, `watch pidfile`.
- `docs/CLI_REFERENCE.md`: add `striatum recovery watch`.
- `docs/HOW_TO_HUMAN.md` § "Inspect, watch, and export
  evidence": one-line pointer with a `cron` example.
- `docs/rfcs/0020-...md`: status flips to `accepted (V1)`.
- `docs/rfcs/README.md`: index reflects new status + D068.
- `docs/DECISION_LOG.md`: D068 (one sentence per cell, per the
  cleanup contract).
- `docs/TODO.md`: F15.
- `pyproject.toml` + `__version__`: 1.5.0 → 1.6.0.
- `CHANGELOG.md`: 1.6.0 section.

## Test Plan (pinned)

`tests/test_recovery_watch.py`:

| Test | Asserts |
|---|---|
| `test_watch_runs_max_sweeps_then_exits` | spawn subprocess with `--max-sweeps 3 --interval-seconds 0 --json`; expect 3 envelopes + 1 watch_exit line; exit 0 |
| `test_watch_emits_jsonl_envelopes` | every stdout line parses as JSON; envelopes match the `run_auto_sweep` shape |
| `test_watch_pidfile_collision_refused` | run-watch holding pidfile; second invocation returns exit 4 with the documented message |
| `test_watch_stale_pidfile_overwritten` | write `<pidfile>` with a dead PID; new watch starts cleanly |
| `test_watch_exits_on_run_terminal` | drive a run to terminal; watch exits with `reason: run_terminal` |
| `test_watch_no_exit_on_terminal_continues` | terminal run + `--no-exit-on-terminal --max-sweeps 2`; exits with `reason: max_sweeps_reached` |
| `test_watch_sigterm_clean_shutdown` | spawn watch with `--interval-seconds 30`; SIGTERM mid-sleep; expect exit 0 within 2s, `reason: signal`, pidfile removed |
| `test_watch_pidfile_path_under_scratch` | pidfile is under `.striatum/scratch/recovery-watch-<run_id>.pid` |

Total: 8 new cases. Suite count moves 310 → 318.

## Acceptance Criteria

- `striatum recovery watch --run-id <id> --max-sweeps 3
  --interval-seconds 0` exits 0 after 3 sweeps.
- `striatum recovery watch` against an unknown run exits 3
  (NotFoundError surfaces through `run_auto_sweep`).
- Concurrent watches against the same run: second exits 4.
- Stale pidfile (dead PID) is overwritten cleanly.
- `--exit-on-terminal` (default) exits when the run hits a
  terminal state.
- `--no-exit-on-terminal` keeps looping past terminal.
- SIGTERM/SIGINT shut down cleanly; pidfile removed; final
  `watch_exit` line emitted.
- `--json` produces parseable JSONL.
- Lint + typecheck + tests clean.
- `pyproject.toml` and `__version__` bump 1.5.0 → 1.6.0.
- RFC 0020 transitions to `accepted (V1)`.

## Acceptance Gate

Implementation job blocks until human acceptance recorded under
`docs/dogfood/015/decisions/`.
