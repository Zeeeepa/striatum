# Build review: `striatum recovery watch` daemon

author: reviewer-claude-opus-002
date: 2026-05-09
verdict: accept

Fresh-context review against the V1 contract pinned in
`DESIGN_SYNTHESIS.md` and `decisions/V1_ACCEPTANCE.md`.

## Verdict

**accept** — V1 acceptance gate satisfied; no blocking findings.

## Sweep matrix

| Acceptance gate | How V1 satisfies it | Verified |
| --- | --- | --- |
| `recovery watch` reuses `run_auto_sweep` rather than duplicating sweep logic | `run_watch` opens a fresh connection per iteration and calls `run_auto_sweep`; no policy or recovery branching is reimplemented inside the daemon | Source read of `src/striatum/recovery/watch.py` |
| Single-instance enforcement at `<repo>/.striatum/scratch/recovery-watch-<run_id>.pid` | `pidfile_path` returns the expected path; `_acquire_pidfile` opens with `O_CREAT|O_EXCL|O_WRONLY`, alive-checks on `EEXIST`, retries once after stale cleanup | `test_watch_pidfile_path_under_scratch`, `test_watch_pidfile_collision_refused`, `test_watch_stale_pidfile_overwritten` |
| `SIGTERM`/`SIGINT` graceful shutdown via `threading.Event` | Handlers set the event; `event.wait(timeout=interval)` returns immediately on signal; final `watch_exit` reports `reason: "signal"` | `test_watch_sigterm_clean_shutdown` (SIGTERM mid-30s sleep, exit 0, pidfile removed) |
| JSONL emission per sweep + final `watch_exit` envelope | `_emit_envelope` writes a JSON line per sweep when `--json`; the loop emits a terminal envelope with `reason ∈ {signal, max_sweeps_reached, terminal_run_state, error}` | `test_watch_runs_max_sweeps_then_exits`, `test_watch_emits_jsonl_envelopes_when_json_set` |
| Exit on terminal run state by default; `--no-exit-on-terminal` keeps looping | Loop checks run state when `exit_on_terminal=True`; default is `True` in CLI | `test_watch_no_exit_on_terminal_continues` (keeps looping until `--max-sweeps` wins) |
| `--max-sweeps N` caps loop count | Counter check inside the loop; exits with `reason: "max_sweeps_reached"` | `test_watch_runs_max_sweeps_then_exits` (3 sweep envelopes + 1 exit) |
| CLI override layering same as `recovery auto` | Same flag set wired through `cli_overrides` and threaded into `resolve_policy` once at startup | Source read; CLI parser shows `--autonomous-review-requeue`, `--autonomous-process-reconcile`, `--max-requeue`, `--checkpoint-timeout`, `--eligible-after` |
| Pidfile collision returns exit 4 with documented message | Dispatch raises `InvalidTransitionError("another recovery watch is active (pid <N>)")`; CLI exits 4 | `test_watch_pidfile_collision_refused` |
| Pidfile cleaned on every clean exit path | `try/finally` around the loop unlinks; SIGTERM test asserts pidfile is gone | `test_watch_sigterm_clean_shutdown`, `test_watch_runs_max_sweeps_then_exits` (implicit) |
| Test plan completeness | 8 cases across max-sweeps, JSONL, pidfile collision, stale-pidfile cleanup, path probe, exit-on-terminal mode, signal shutdown, module-level smoke | `tests/test_recovery_watch.py` reviewed |
| No regression to existing `recovery auto` | `recovery auto` orchestrator untouched; `watch.py` only imports from it; full suite (318 passed) | `make test` summary |

## Quality observations (non-blocking)

1. `install_signal_handlers=False` test hook is correct — pytest
   would otherwise have its own handlers stomped. The module-level
   smoke test (`test_run_watch_returns_zero_on_max_sweeps`)
   exercises the orchestrator without subprocess overhead and
   complements the subprocess tests.
2. The per-run pidfile naming
   (`recovery-watch-<run_id>.pid`) keeps multi-run watchers
   non-conflicting — operators can run one watcher per active run
   without coordination.
3. The handoff calls out the missing "watch every active run"
   operator pattern as a follow-up; agreed it is non-blocking and
   belongs in `HOW_TO_HUMAN.md`.

## Risks reviewed and rejected

- **Pidfile race between alive-check and re-create**: a third
  process could grab the pid in the gap between unlinking and
  re-opening. The retry-once policy bounds this; collision in the
  retry returns exit 4. Acceptable for V1 — operators do not
  start two watchers concurrently in practice.
- **Long-sleep ignoring signals**: avoided by `event.wait()` —
  validated by the SIGTERM-mid-30s-sleep test exiting in well
  under the 10-second `communicate` timeout.
- **Stale pidfile from a crashed watcher**: covered by the alive
  check in `_acquire_pidfile`.

## Follow-ups carried to TODO

- Document multi-run watch operator pattern in `HOW_TO_HUMAN.md`
  (note from build handoff §"Follow-ups").

## Decision

Accept V1. Land the change, bump to 1.6.0, transition RFC 0020 to
`accepted (V1)` (drop the "step 3 deferred" qualifier).
