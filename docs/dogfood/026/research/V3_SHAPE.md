---
title: "V3 shape research: cancel run + re-run + dirty-tree visibility"
date: 2026-05-09
---

# V3 shape research

author: researcher-codex-gpt-5.5-001

## (1) Run states

| state | source/destination |
| --- | --- |
| `prepared` | initial after `create_run`. Transitions: → `ready` (via branch_confirm), → `needs_branch_confirmation` |
| `needs_branch_confirmation` | `branch.mode != auto` and no branch matched. → `ready` (via branch_confirm) |
| `ready` | branch confirmed; queue not yet enqueued. → `running` (via run_start) |
| `running` | enqueued; jobs claimable. → `completed`/`failed`/`canceled` (via maybe_complete_run) |
| `completed` | terminal. all non-cancel/skip jobs done. |
| `failed` | terminal. any failed job. |
| `canceled` | **already exists** as terminal — currently reached only when every job is `canceled`/`skipped`. |

**Key finding:** the `canceled` state is already in the codebase; we
need a top-down `cancel_run` that drives it directly, not via the
all-jobs-canceled rollup.

## (2) Cancel-run mutation

### Required steps (all in one transaction)

1. Cancel all in-flight jobs:
   ```sql
   UPDATE jobs
   SET state = 'canceled'
   WHERE run_id = ? AND state IN ('queued', 'running', 'blocked', 'ready')
   ```
2. Release all active leases for that run:
   ```sql
   UPDATE leases
   SET state = 'released', released_at = ?, release_reason = 'run_canceled'
   WHERE owner_session_id IN (SELECT session_id FROM sessions WHERE run_id = ?)
     AND state = 'active'
   ```
3. Update the run state itself:
   ```sql
   UPDATE runs
   SET state = 'canceled', completed_at = ?, stop_reason = 'operator_canceled'
   WHERE run_id = ?
   ```
4. Emit `run.canceled` event (payload: `{"reason": "operator_canceled"}`).
5. Call `close_remaining_sessions(conn, run_id, source="run_canceled", ...)` — already exists in db.py.

### Allowed source states

`prepared`, `needs_branch_confirmation`, `ready`, `running` →
`canceled`. Already-terminal states (`completed`, `failed`,
`canceled`) → `InvalidTransitionError`.

### Idempotency

Cancelling an already-canceled run returns success
(idempotent) — re-issuing from the UI on accidental double-click is
benign. The implementation should detect terminal `canceled` state and
return without re-running the cleanup.

## (3) Auto-branch + dirty-tree path

### What `branch_confirm(create=True)` does

`git_create_or_checkout_branch` (mutations.py:135) tries `git
checkout -b <branch>`; on failure (branch exists or dirty tree)
falls back to `git checkout <branch>`. So an existing branch alone
does *not* error — the function silently switches to it and
records the new run on the same branch.

### What actually causes 409 on Run-now

Dirty working tree: `git checkout` refuses to switch when there are
uncommitted changes. `git_create_or_checkout_branch` raises
`WorkflowError` with the stderr captured. My V2 handler catches
`BranchConfirmationError` → 409, but `git_create_or_checkout_branch`
raises `WorkflowError` (exit_code=8), not
`BranchConfirmationError` (exit_code=7). So in practice today,
auto-mode dirty-tree returns 422 from V2, not 409. **F3 from the V2
design review** wanted dirty-tree to land in 409 with `git status`;
the implementer-V2 left this for V3. **Now is the time.**

### Recommendation

1. Catch the specific dirty-tree case in run-now: when
   `git_create_or_checkout_branch` raises a `WorkflowError` whose
   message starts with `"git checkout failed"`, re-raise as
   `BranchConfirmationError` so V2's 409 path catches it. Better:
   add a stable `git_status` field by running
   `git status --short` in the handler and including the output.
2. **No auto-suffix in V3.** The same workflow.json used twice
   currently shares a branch (existing behavior). That's fine —
   multiple runs on one branch is by-design (per DDD: branches are
   workspace metadata, not run identity). The reusability friction
   is actually dirty-tree, not "branch in use." Document this in
   the synthesis.

## (4) Re-run button

`POST /workflows/run/<path>` is already a re-run when called
twice. V3 just renders the button on `/workflows/<path>` for every
valid workflow (V2 already does this, but with the label "Run this
workflow now"). For re-runs, label clarity matters less than the
cancel-and-rerun loop, which we now support end-to-end.

## V3 scope summary

| Surface | Action |
| --- | --- |
| `src/striatum/db.py` | New `cancel_run(conn, *, run_id, reason=None)` — drives state machine top-down |
| `src/striatum/cli/mutations.py` | Wrapper that exports `cancel_run`; CLI adds `striatum run cancel --run-id <id>` |
| `src/striatum/cli/dispatch.py` | Wires the new subcommand |
| `src/striatum/cli/parser.py` | Adds the parser stanza |
| `src/striatum/service.py` | POST `/run/<id>/cancel` (mutation-gated); existing `_handle_workflow_run_now` enriches 409 with `git_status` |
| `src/striatum/web/templates/run_detail.html` | Cancel button (rendered when state ∈ {ready, running, prepared, needs_branch_confirmation}) |
| `src/striatum/web/static/run_cancel.js` | Cancel-button JS island |
| `tests/test_cancel_run.py` (new) | Unit tests for `cancel_run` cascade |
| `tests/test_web_run_cancel.py` (new) | HTTP test for the route |
| `tests/test_web_workflow_run.py` (extend) | Dirty-tree case returns 409 with `git_status` field |

## Out of scope (V4)

- Pause / resume.
- Auto-branch suffix.
- Per-job mutation buttons (kill running job, retry).
- Programmatic re-run with overrides.

## Test precedent

- `tests/test_db.py` for `maybe_complete_run` covers the all-jobs-canceled
  rollup path. The new top-down `cancel_run` tests should mirror that
  shape (build an in-memory DB, insert run + jobs + leases, call
  cancel_run, assert states).
- `tests/test_web_workflow_run.py` is the closest analog for the
  HTTP test.
