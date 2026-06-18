---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs:
  - "docs/issues/14/SPEC.md"
  - "docs/issues/14/SCOPE.md"
  - "docs/ROADMAP.md"
  - "docs/TODO.md"
  - "docs/SPEC.md"
  - "docs/CLI_REFERENCE.md"
  - "AGENTS.md"
---

author: implementer-unknown-model-001

# GH #14 — Implementer Handoff

This handoff covers the recovery path for an open `process_exit_nonzero`
blocker on a terminal/completed run when the job no longer has an active
lease. Scope is bounded by `docs/issues/14/SCOPE.md`: keep recovery on
the existing `recovery resume --blocker-id <id> --force` verb, do not
introduce a generic blocker-dismissal admin command, and pin the
behavior with a regression that matches the Engram bug report shape.

## Diagnosis

The v1.48.2 source already contains the GH #7 terminal no-op branch in
`src/striatum/cli/recovery.py::resume_blocker` (lines 422–461) and the
matching guard in
`src/striatum/process_completion.py::evaluate_and_block_inline`
(lines 291–293). The Engram bug report against v1.48.1 stems from a
release that predates those guards.

Two gaps remained after GH #7:

1. **No regression for the recovery side.** `test_gh7_terminal_blocker.py`
   only pins the inline-guard half; the recovery surface that operators
   actually rely on for legacy leaked blockers had no test, so future
   refactors could regress the no-op branch silently.
2. **`recovery auto` left terminal-state blockers reported as
   `still_stuck: blocker_recovery_eligible` indefinitely.** Even with
   `--autonomous-process-reconcile`, the autonomous sweep would never
   dismiss them — operators had to clear each blocker by hand.

## Changes

### `src/striatum/recovery/auto.py`

Extended Step 5 of `run_auto_sweep`:

- The `open_blockers` SELECT now joins `jobs.state` so the sweep can
  recognize terminal-state attachment without an extra round-trip.
- When the resolved policy has `autonomous_process_reconcile: true` and
  a process-adapter blocker is attached to a terminal job
  (`completed`/`failed`/`canceled`/`skipped`), the sweep autonomously
  calls `resume_blocker(..., force=True)` to dismiss the blocker as a
  no-op. Failures fall through to `still_stuck` with reason
  `terminal_blocker_dismiss_failed`.
- `--dry-run` produces a `terminal_blocker_dismiss_eligible` action so
  operators can preview the autonomous cleanup before enabling it.
- When the policy opt-in is absent, the today behavior is preserved
  byte-for-byte: the blocker is reported as `blocker_recovery_eligible`
  and `still_stuck` keeps its existing shape. No new autonomous
  mutation surface is introduced outside the explicit policy opt-in.

`resume_blocker` itself was not modified — the v1.48.2 source already
implements the required no-lease terminal-dismissal branch, and the new
test suite pins that behavior so future refactors cannot regress it.

### `tests/test_gh14_terminal_blocker_recovery.py` (new)

Seven focused regression tests covering every acceptance-checklist
clause in SCOPE.md:

1. `test_resume_force_dismisses_terminal_process_exit_blocker_without_lease`
   — the core scenario. Drive a one-job workflow to `completed`,
   release the lease (`current_lease_id IS NULL`), insert a legacy
   `process_exit_nonzero` blocker, then call
   `recovery resume --blocker-id <id> --force`. Asserts `status =
   resolved_terminal_no_op`, blocker resolved, job remains `completed`.
2. `test_resume_force_complete_against_terminal_job_is_terminal_no_op`
   — the exact `--force --complete --session-id` CLI shape the Engram
   operator filed. The terminal branch short-circuits before the
   active-lease check, so the request succeeds without trying to
   re-complete the already-completed job.
3. `test_resume_without_force_still_refuses_terminal_process_exit_blocker`
   — acceptance #5. The terminal no-op path is not a stealth bypass
   of the `--force` operator-inspection gate. Without `--force`, the
   CLI still refuses with exit code 4.
4. `test_recovery_auto_dismisses_terminal_process_blocker_when_policy_enabled`
   — acceptance #7. `recovery auto --autonomous-process-reconcile
   --eligible-after 0` autonomously dismisses the blocker (reports a
   `terminal_blocker_dismissed` action) instead of returning it as
   `blocker_recovery_eligible`.
5. `test_recovery_auto_without_policy_still_reports_terminal_blocker_eligible`
   — acceptance #7 inverse. Without the policy opt-in, `recovery auto`
   does not mutate the blocker; it surfaces it as
   `blocker_recovery_eligible` exactly as before.
6. `test_recovery_auto_dry_run_reports_terminal_dismiss_eligible` —
   `--dry-run` preview action shape; on-disk state untouched.
7. `test_resume_force_records_terminal_dismiss_event` — acceptance #4.
   The dismissal must emit the structured
   `recovery.blocker_dismissed_terminal` event with `blocker_id`,
   `blocker_kind`, and `job_state` so the audit trail survives future
   refactors.

Legacy blockers are inserted directly because the current inline guard
(GH #7) prevents the adapter path from creating them in fresh runs.
The test helper documents this in a docstring.

## Tests

### Run

- `PYTHONPATH=src python3 -m pytest tests/test_gh14_terminal_blocker_recovery.py
  tests/test_gh7_terminal_blocker.py tests/test_recovery_resume.py
  tests/test_recovery_auto.py tests/test_recovery_extended.py
  tests/test_recovery_panel_dry_run.py tests/test_recovery_watch.py`
  → **48 passed**.
- `make lint` → `ruff` clean.
- `make typecheck` → `mypy` clean (213 source files).

### Not run

- `make test` in full. The full suite reports 7 pre-existing failures
  on this branch unrelated to recovery work (`test_daemon.py`'s daemon
  mode token tests, `test_daemon_rpc.py`'s daemon-pg migration name
  test, `test_doc_links.py` word-budget check, `test_override_modal_payload.py`,
  `test_web_ui.py::test_static_assets_no_external_urls`). Verified
  pre-existing by stashing the GH #14 changes — the same tests fail
  on `f235cc5` without my edits. None of those tests touch
  `src/striatum/cli/recovery.py`, `src/striatum/recovery/auto.py`,
  or any blocker-related path.
- Daemon-RPC / daemon-PG suites (`tests/daemon_rpc/`, `tests/daemon_pg/`)
  were skipped — they require separate Postgres infra and are
  orthogonal to the synchronous SQLite recovery path GH #14 lives on.

## Files changed

- `src/striatum/recovery/auto.py` — Step 5 of `run_auto_sweep` now
  autonomously dismisses terminal-state process-adapter blockers when
  `autonomous_process_reconcile` policy is on; behavior unchanged when
  the policy is off.
- `tests/test_gh14_terminal_blocker_recovery.py` — new regression
  suite (7 tests).
- `docs/issues/14/build/HANDOFF.md` — this file.

## Acceptance Checklist Mapping

| Item | Status | Evidence |
| ---- | ------ | -------- |
| 1 | met | `test_resume_force_dismisses_terminal_process_exit_blocker_without_lease` asserts the exact shape with `current_lease_id IS NULL`. |
| 2 | met | All four regression tests assert `_job_state(...) == "completed"` after dismissal. The terminal branch in `resume_blocker` only updates `blockers`, never `jobs`. |
| 3 | met | `test_resume_force_dismisses_terminal_process_exit_blocker_without_lease` calls the CLI without `--session-id`. |
| 4 | met | `test_resume_force_records_terminal_dismiss_event` reads the `events` table and confirms `recovery.blocker_dismissed_terminal` payload contents. |
| 5 | met | `test_resume_without_force_still_refuses_terminal_process_exit_blocker` asserts exit code 4 + open blocker preserved. |
| 6 | met | Pinned by the existing `tests/test_gh7_terminal_blocker.py` suite (`evaluate_and_block_inline` skips for every terminal state). No new code path can re-open a blocker after `job.completed`. |
| 7 | met | Three new auto-sweep tests: dismissal under policy, eligibility-only without policy, dry-run preview. |
| 8 | met | `tests/test_recovery_resume.py` (5 tests, still passing) keep the non-terminal recovery rails intact: missing artifacts refuse recovery (#56–88), `--complete` refuses when review verdict missing (#136–177), active-lease validation still applies when the job transitions back to `running` (#92–134). |

## Residual Risk

- **Autonomous dismissal under unusual blocker rows.** The new auto path
  trusts the terminal-state check + the existing `resume_blocker` no-op
  branch. If a future code change opens a process-adapter blocker on a
  terminal job with `missing_artifact_paths != []` (which the GH #7
  inline guard explicitly disallows), the autonomous sweep would still
  attempt dismissal. The `resume_blocker` branch only checks
  `force and terminal_state`, not the envelope contents — but the
  branch is no-op (it does not touch jobs/artifacts), so this is bounded
  to "marking a legitimate stale blocker resolved without surfacing the
  missing-output problem first". The risk is acceptable because the
  pre-condition (inline guard violation) would itself be the real bug.
- **Policy default is off.** Operators must opt in via
  `recovery_policy.autonomous_process_reconcile: true` (or the
  matching CLI flag) to get the autonomous cleanup. That keeps the
  default behavior conservative; the manual `recovery resume
  --blocker-id <id> --force` remains the primary recovery path.
- **Engram clean-up is downstream.** The Engram operator's specific
  blocker (`blk_6dd92e18a3da4cc5ac2c4f1445755b99`) is still open in
  their `/home/halbritt/git/engram/.striatum/state.sqlite3` until they
  upgrade to v1.48.2 (or later) and re-run `recovery resume --force`
  / `recovery auto --autonomous-process-reconcile`. Engram repository
  edits are out of scope per SCOPE.md.
- **Pre-existing branch-level test drift** (`test_web_ui.py`,
  `test_daemon.py` daemon-mode auth, doc word-budget, etc.) is not
  addressed and is unrelated to this work.
