---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0024 V4 design

author: designer-codex-gpt-5.5-001

## Scope

Three additions on the run-mutation surface:

1. **Pause/resume** — `runs.paused_at` + `runs.paused_reason`
   columns (migration v11). `claim_next` gates on paused.
   Mutations: `pause_run`, `resume_run`. Idempotent.
2. **Per-job cancel via web** — wraps existing
   `recovery.cancel_job` (with `cascade=True` default) in a new
   HTTP route + UI button on the job detail page.
3. **Retry job** — new `retry_job` mutation for failed/canceled/
   blocked jobs. Resets terminal job state, re-enqueues, revives a
   canceled/failed *run* if needed.

## Migration v11

```python
def _apply_v11_runs_paused_columns(conn):
    cols = [row[1] for row in conn.execute("PRAGMA table_info(runs)").fetchall()]
    if "paused_at" not in cols:
        conn.execute("ALTER TABLE runs ADD COLUMN paused_at TEXT")
    if "paused_reason" not in cols:
        conn.execute("ALTER TABLE runs ADD COLUMN paused_reason TEXT")
```

`schema.py` baseline updated alongside (per RFC 0006 convention).

## Pause/resume contract

### `pause_run(conn, *, run_id, reason=None) -> {state, paused_at}`

Allowed source states: `running`, `ready`, `prepared`,
`needs_branch_confirmation`. Already-paused: no-op (returns current
`paused_at`). Terminal states (`completed`, `failed`, `canceled`):
`InvalidTransitionError`.

```sql
UPDATE runs SET paused_at = ?, paused_reason = ?
WHERE run_id = ? AND paused_at IS NULL
```

Emit `run.paused` event only when row was actually updated (use
`changes()`).

### `resume_run(conn, *, run_id) -> {state, paused_at: null}`

Reverse: clears both columns. Already-not-paused: no-op. Terminal
states: `InvalidTransitionError` (can't resume a completed run;
use `retry_job` to revive).

### Claim-next gate

`db.py:claim_next` line 798, after `state != "running"`:
```python
if run.get("paused_at") is not None:
    return {"status": "no_work", "paused": True}
```

Active leases tick normally; `expire_leases` runs at the top of
`claim_next` so paused-with-stale-leases is self-healing.

### CLI

```
striatum run pause --run-id <id> [--reason <text>] [--json]
striatum run resume --run-id <id> [--json]
```

### HTTP

```
POST /run/<id>/pause    Body: {"reason": "..."}
POST /run/<id>/resume   Body: {}
```

Mutation-gated. 200 / 405 / 404 / 409 (terminal) / 415.

### UI

Pause button (when not paused, state ∈ {ready, running,
prepared, needs_branch_confirmation}). Resume button (when
paused). Inline status pill `paused`. JS island
`/static/run_pause_resume.js`.

## Per-job cancel via web

```
POST /run/<id>/job/<jid>/cancel
Body: {"reason": "...", "cascade": true|false}
```

Calls `recovery.cancel_job(conn, run_id, job_id, reason,
cascade)`. Defaults: `cascade=true`,
`reason="operator_canceled_via_web"`.

UI: Cancel button on `/run/<id>/job/<jid>` (rendered when state
is non-terminal).

## Retry job

### `retry_job(conn, *, run_id, job_id) -> {job_id, previous_state, new_state, run_revived}`

Allowed source states: `failed`, `canceled`, `blocked`.

Steps (one transaction):
1. Verify state retriable; raise `InvalidTransitionError` otherwise.
2. Reset job: `state='queued'`, `started_at=NULL`,
   `completed_at=NULL`, `current_lease_id=NULL`,
   `attempt = attempt + 1`.
3. Clear unresolved blockers for that job:
   `DELETE FROM blockers WHERE job_id = ? AND resolved_at IS NULL`.
4. `enqueue_job(conn, job_id=...)` — inserts new pending
   queue_messages row.
5. If run is in `canceled`/`failed`, transition back to `running`,
   clear `completed_at`/`stop_reason`, emit `run.revived` event.
6. Emit `job.retried` event with `{previous_state, attempt}`.

### CLI / HTTP

```
striatum run retry-job --run-id <id> --job-id <jid>
POST /run/<id>/job/<jid>/retry
```

### UI

Retry button on job detail page when state ∈ {failed, canceled,
blocked}.

## Test plan

| File | Coverage |
| --- | --- |
| `tests/test_pause_resume.py` (new) | pause from each source state; resume reverses; idempotency; refuse terminal; claim_next returns no_work when paused |
| `tests/test_retry_job.py` (new) | retry from failed/canceled/blocked; refuse terminal; re-enqueues; revives canceled run |
| `tests/test_web_run_pause_resume.py` (new) | HTTP routes; mutation gate; status codes |
| `tests/test_web_job_actions.py` (new) | per-job cancel + retry HTTP; mutation gate; missing run/job |
| `tests/test_cli_run_cancel.py` (extend) | CLI pause/resume/retry-job |

## Backward compat

- `paused_at IS NULL` matches every pre-v11 row (column added with
  default NULL). No data migration needed.
- Existing claim_next tests still pass (untouched runs are not
  paused).
- Existing cancel_run still works as the heavy hammer; pause/retry
  are the lighter touch.

## Out of scope (V5 if needed)

- Pause-with-deadline (auto-resume at timestamp).
- Per-lane pause.
- "Auto-revive" run state when a retry-job succeeds.
- Recovery integration of pause as escalation target.
