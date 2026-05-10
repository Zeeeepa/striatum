---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0024 V3 design

author: designer-claude-opus-001

## Scope

Three additions, mirroring the same conservative cadence as V2:

1. **`cancel_run` mutation** — top-down cancellation that drives the
   already-existing `runs.state = 'canceled'` terminal state without
   waiting for every job to be cancelled individually. Exposed via
   CLI (`striatum run cancel`), HTTP (`POST /run/<id>/cancel`), and
   a UI button on `/run/<id>`.
2. **Run-now dirty-tree visibility** — close the V2 design-review F3
   deferral. When `git_create_or_checkout_branch` fails with a dirty
   tree, the 409 response now carries `git status --short` output so
   the operator sees what's blocking without context-switching to a
   terminal.
3. **Re-run button** — sugar on the workflow detail page. Same POST
   `/workflows/run/<path>` endpoint as V2; new label and placement.

Auto-branch suffix is **not in V3.** Research clarified that
multiple runs sharing one branch is by-design (branches are
workspace metadata, not run identity). The reusability friction
operators feel is dirty-tree, which #2 fixes directly.

## Cancel-run contract

### Function signature

```python
def cancel_run(conn: sqlite3.Connection, *, run_id: str, reason: str | None = None) -> JsonObject:
    """Top-down cancel: voids active leases, marks queued/running/blocked
    jobs as canceled, transitions the run to 'canceled', emits run.canceled
    event, calls close_remaining_sessions. Idempotent on already-canceled."""
```

### Allowed source states

`prepared`, `needs_branch_confirmation`, `ready`, `running` →
`canceled`. Already-`canceled` → no-op (return current state).
`completed`/`failed` → `InvalidTransitionError` (exit 4).

### Cleanup steps (single transaction)

1. Mark in-flight jobs canceled:
   ```sql
   UPDATE jobs SET state = 'canceled'
   WHERE run_id = ? AND state IN ('queued','running','blocked','ready')
   ```
2. Release active leases held by this run's sessions:
   ```sql
   UPDATE leases
   SET state = 'released', released_at = ?, release_reason = 'run_canceled'
   WHERE owner_session_id IN (SELECT session_id FROM sessions WHERE run_id = ?)
     AND state = 'active'
   ```
3. Run row:
   ```sql
   UPDATE runs SET state = 'canceled', completed_at = ?, stop_reason = ?
   WHERE run_id = ?
   ```
   `stop_reason = "operator_canceled"` (or whatever the caller passed).
4. `insert_event(conn, run_id, event_type="run.canceled", payload={"reason": reason or "operator_canceled"})`.
5. `close_remaining_sessions(conn, run_id, source="run_canceled", reason="run_canceled")`.

### CLI surface

```
striatum run cancel --run-id <id> [--reason <text>] [--json]
```

Returns `{run_id, state: "canceled"}` on success. Idempotent on
already-canceled (returns the same payload). 409 on terminal
non-canceled.

### HTTP surface

```
POST /run/<id>/cancel
Body: {"reason": "<optional text>"}
```

| Code | Body |
| --- | --- |
| 200 | `{ok: true, data: {run_id, state: "canceled"}}` |
| 405 | mutation gate off |
| 404 | run not found |
| 409 | run is in a terminal non-canceled state |
| 415 | wrong content-type |

### UI button

Rendered in `run_detail.html` when `run.state ∈ {prepared,
needs_branch_confirmation, ready, running}`. Confirm dialog
(`window.confirm("Cancel this run? In-flight work will be marked canceled.")`)
before POST. Reload on 200.

## Dirty-tree visibility

### Current behavior (V2)

`_handle_workflow_run_now` calls `branch_confirm(create=True)`
which calls `git_create_or_checkout_branch`. On dirty tree,
`git checkout` fails and the function raises `WorkflowError` with
the captured stderr. V2 catches `BranchConfirmationError` for 409 —
but `WorkflowError` falls through to the 422 catch arm, surfacing as
"workflow validation failed" (misleading).

### Fix

Detect the specific git-checkout failure pattern in the run-now
handler and return 409 with the structured payload:

```json
{
  "ok": false,
  "error": {
    "code": 409,
    "message": "git checkout failed for branch 'striatum/foo': ...",
    "kind": "dirty_tree",
    "git_status": "M src/foo.py\n?? newfile.py\n"
  }
}
```

The handler runs `git status --short` (capped at ~80 lines) and
includes it in the body. JS-side, the alert prompt or banner can
display this verbatim so the operator can `git stash` / commit
before retrying.

### Implementation note

Catch `WorkflowError` whose message matches `git checkout failed`,
re-emit as 409 with the `git_status` payload. `BranchConfirmationError`
keeps its current handling (covers other 409 cases like
needs_branch_confirmation transitions).

## Re-run button

```html
<button id="rerun-btn" data-rel-path="{{ workflow.path }}">Re-run</button>
```

Bound to the same JS as the V2 "Run this workflow now" button —
just relabeled. Rendered alongside the existing Run-now button so
operators see both options. Functionally identical.

## Test plan

| Surface | Test |
| --- | --- |
| `tests/test_cancel_run.py` (new) | `cancel_run` cancels running, ready, prepared. Idempotent on already-canceled. Releases leases. Cancels in-flight jobs. Emits run.canceled. |
| `tests/test_cancel_run.py` (new) | InvalidTransition on completed/failed. |
| `tests/test_cli_run_cancel.py` (new) | CLI smoke: prepare run, cancel, status reflects canceled. |
| `tests/test_web_run_cancel.py` (new) | POST /run/<id>/cancel: 200 success, 405 no-mutations, 404 missing, 409 already-completed. |
| `tests/test_web_workflow_run.py` (extend) | Dirty-tree case returns 409 with `git_status` field. |
| `tests/test_web_ui.py` or detail-page test | Cancel button renders only when state is non-terminal. |

## Out of scope (V4)

- Pause / resume.
- Auto-branch suffix.
- Per-job mutation buttons (kill running job, retry).
- Programmatic re-run with parameter overrides.
- Recovery integration: cancel-run as a recovery escalation hook.
