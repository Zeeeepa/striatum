author: reviewer-unknown-model-001
kind: finding
logical_name: verify_review

# Review for GH #14: Recovery cannot clear terminal-run process_exit_nonzero blocker without lease

## Objective
Verify that GH #14 is resolved, allowing operators to clear stale `process_exit_nonzero` blockers on terminal jobs without requiring a current lease.

## Verification Steps
1. **Requirement Analysis**: Reviewed `docs/issues/14/SPEC.md` to identify the core issue: v1.48.1 failed to clear blockers on terminal runs because it enforced a lease check even when the job was already completed.
2. **Code Review**:
    - Analyzed `src/striatum/cli/recovery.py`. Confirmed that the `resume_blocker` function now includes a short-circuit for terminal jobs (state in `completed`, `failed`, `canceled`, `skipped`) that bypasses the lease check if `--force` is provided.
    - Analyzed `src/striatum/recovery/auto.py`. Confirmed that `run_auto_sweep` now supports `autonomous_process_reconcile` policy to automatically dismiss these terminal blockers.
3. **Regression Testing**:
    - Executed `tests/test_gh14_terminal_blocker_recovery.py`: All 7 tests passed. This confirms:
        - `recovery resume --force` dismisses terminal blockers without a lease.
        - `recovery resume --force --complete` (Engram style) also works.
        - `--force` is still required (no stealth bypass).
        - `recovery auto --autonomous-process-reconcile` dismisses these blockers.
        - `recovery auto` without the policy still reports them as eligible but doesn't mutate.
        - `--dry-run` works correctly.
        - Structured events are recorded.
    - Executed `tests/test_gh7_terminal_blocker.py`: 2 passed (ensures no regression for the original fix).
    - Executed `tests/test_recovery_auto.py`: 21 passed (ensures broader recovery logic remains sound).

## Findings
- The fix correctly implements the "public recovery path" requested in the SPEC.
- The use of the `resolved_terminal_no_op` status provides clear feedback to the operator.
- The autonomous recovery integration addresses the "Current Impact" where `doctor` would report these blockers indefinitely.

## Verdict: accept
The implementation satisfies all requirements in the SPEC and is verified by targeted regression tests.
