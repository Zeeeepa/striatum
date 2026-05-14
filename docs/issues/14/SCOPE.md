---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/issues/14/SPEC.md", "docs/ROADMAP.md", "docs/TODO.md", "docs/SPEC.md", "docs/CLI_REFERENCE.md", "AGENTS.md"]
---

# GH #14 Scope

author: triager-unknown-model-001
date: 2026-05-14
status: scoped

## Issues Covered

- GH #14: Recovery cannot clear terminal-run `process_exit_nonzero` blocker without lease.

## Scope Decision

Keep the public recovery path on the existing `striatum recovery resume --blocker-id <id> --force` verb. Do not add a broad `dismiss-blocker` command for this issue. The narrow product behavior is: when an open process-adapter blocker is attached to a job that is already terminal, `recovery resume --force` may resolve the blocker without requiring a current active lease, without changing the terminal job state, and without reopening or completing the job.

The fix job should treat the Engram report as a regression of the GH #7 terminal-blocker path: source already has a terminal-state branch in `src/striatum/cli/recovery.py`, but the test file only pins the process-completion guard. The implementation should add a real regression for the observed no-lease recovery failure and then repair any code path that still reaches the active-lease check for terminal jobs.

## In Scope

- `src/striatum/cli/recovery.py`
  - Verify and, if needed, repair `resume_blocker` so terminal jobs with open process-adapter blockers resolve before active-lease validation.
  - Preserve the `--force` requirement for `process_exit_nonzero` and `process_timeout_exceeded`.
  - Preserve terminal job state, existing artifact records, and existing verdict records.
- `src/striatum/process_completion.py`
  - Only if needed to preserve or tighten the existing guard that skips post-exit blockers once a job is terminal.
- `src/striatum/recovery/auto.py`
  - Only if needed so `recovery auto --autonomous-process-reconcile` reports or invokes the same terminal-blocker cleanup instead of returning `still_stuck: blocker_recovery_eligible`.
- `src/striatum/cli/introspect.py`
  - Only if needed for `status`, `doctor`, or `why` messaging after the recovery behavior changes.
- `tests/test_gh7_terminal_blocker.py` or a focused new `tests/test_gh14_terminal_blocker_recovery.py`
  - Add a regression that constructs an already-completed review job with a published artifact, recorded accept verdict, open `process_exit_nonzero` blocker, and no current lease; then asserts `recovery resume --force` resolves the blocker and leaves the job completed.
  - Add or preserve coverage that post-process completion does not open a new blocker when the job is already terminal.
- `docs/issues/14/build/`
  - Implementer handoff artifact only.
- `docs/issues/14/review/`
  - Verification artifact only.

## Out Of Scope

- New generic `recovery dismiss-blocker` or admin-only blocker deletion surface.
- Making `checkpoint resolve` handle non-human-checkpoint process-adapter blockers.
- Manual SQLite mutation, migration surgery, or direct `.striatum/` edits.
- Changing artifact publishing, verdict semantics, or review completion behavior beyond this terminal-blocker recovery case.
- Changing daemon/PostgreSQL transition work, daemon RPC capability shape, or multi-repository recovery behavior.
- Engram repository edits or replaying the affected Engram run inside this workflow.
- Closing GH #15, GH #17, or active runway security issues #9-#13.

## Acceptance Checklist

1. `recovery resume --blocker-id <id> --force --json` resolves an open `process_exit_nonzero` blocker attached to a terminal `completed` job even when `jobs.current_lease_id` is null.
2. The same recovery leaves the job state terminal, specifically `completed` for the GH #14 shape, and does not transition the job back to `running`.
3. The same recovery does not require `--session-id` unless `--complete` is used for the ordinary non-terminal resume path; terminal no-op dismissal should not need a lease owner.
4. The same recovery records a structured event, preferably the existing `recovery.blocker_dismissed_terminal`, with the blocker id, blocker kind, and terminal job state.
5. `recovery resume` without `--force` still refuses `process_exit_nonzero` and `process_timeout_exceeded` blockers.
6. Post-exit process completion validation cannot create a new process-adapter blocker after the job has already reached a terminal state.
7. `recovery auto --autonomous-process-reconcile --eligible-after 0 --json` no longer leaves this exact terminal/no-missing-outputs shape indefinitely stuck as only `blocker_recovery_eligible`, or the implementer documents why auto cleanup is intentionally deferred while the public manual recovery path is fixed.
8. Existing recovery protections remain intact for non-terminal blocked jobs: missing artifacts still refuse recovery, missing review verdicts still refuse `--complete`, and active-lease validation still applies when the job must return to `running`.

## Risks And Conflicts

- The current source appears to contain the intended GH #7 terminal no-op branch, so the reported v1.48.1 behavior may reflect a packaging/version skew, an untested alternate path, or an option combination that bypasses the branch. The fix should start with a failing regression that matches the Engram command shape before changing behavior.
- `recovery auto` may only diagnose eligible blockers today. Folding terminal cleanup into it could expand autonomous mutation behavior; if that is too broad for this issue, keep auto changes minimal and document the remaining manual step.
- A broad blocker-dismissal command would create an operator footgun and conflict with the product boundary that live state advances through typed recovery verbs, not arbitrary database cleanup.
- Parallel issue workflows #15 and #17 are docs-focused and should not overlap. The active runway for #9-#13 touches recovery dry-run tests and web UI security; avoid broad recovery refactors that would collide with that work.

## Verification Commands

```bash
make test
```

```bash
PYTHONPATH=src python3 -m pytest tests/test_gh7_terminal_blocker.py tests/test_gh14_terminal_blocker_recovery.py
```

```bash
PYTHONPATH=src python3 -m striatum.cli recovery resume --blocker-id <legacy_terminal_blocker_id> --force --json
```

```bash
PYTHONPATH=src python3 -m striatum.cli recovery auto --run-id <run_id> --autonomous-process-reconcile --eligible-after 0 --json
```
