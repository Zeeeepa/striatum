---
title: "V4 shape research: pause/resume + per-job cancel + retry"
date: 2026-05-09
---

# V4 shape research

author: researcher-codex-gpt-5.5-001

## (1) Pause/resume — column vs state

### Recommended: column on `runs`

Add `paused_at TEXT` (nullable, ISO 8601) and `paused_reason TEXT`
(nullable) via migration v11. Pause sets both; resume clears both.
The run's `state` column stays `running` throughout — pause is
*orthogonal* to the run lifecycle.

### Why not a new state

If `paused` were a separate state, every state-transition site
(`maybe_complete_run`, `run_start`, `cancel_run`, etc.) would need
to reason about it. The orthogonal-column model leaves the
existing state machine untouched: terminal-state checks keep
working; only `claim_next` consults the new column.

### Claim-next gate

In `db.py:claim_next` at line 798, after the `state != "running"`
check, add:

```python
if run["paused_at"] is not None:
    return {"status": "no_work", "paused": True}
```

Active leases keep ticking; if a paused run's lease expires, the
existing `expire_leases` (called at the top of `claim_next`) handles
it. Recovery flows continue to work.

### Idempotency

`pause_run` on an already-paused run: no-op (returns current state).
`resume_run` on a non-paused run: no-op. Both emit corresponding
events (`run.paused`, `run.resumed`) only on actual transition.

## (2) Per-job cancel — wrap existing function

`recovery.cancel_job` (cli/recovery.py:256) already does the right
thing:
- Refuses terminal-state jobs.
- Releases leases via `expire_leases`.
- Cancels job + (with `cascade=True`) blocked dependents.
- Calls `maybe_complete_run`.

The web wrapper just calls it via a new HTTP route `POST
/run/<id>/job/<jid>/cancel` with default `cascade=True` and a
default `reason="operator_canceled_via_web"`.

## (3) Retry — new mutation

### Retriable states

| State | Retriable? |
| --- | --- |
| `failed` | yes |
| `canceled` | yes |
| `blocked` | yes — clear blockers, re-evaluate dependencies |
| `skipped` | no — terminal by design |
| `completed` | no — already done |
| `queued`/`running`/`claimed`/`ready` | no — already in flight |

### What `retry_job` does

In one transaction:
1. Verify job is in a retriable state; raise `InvalidTransitionError`
   otherwise.
2. Clear job: `state = 'queued'`, `started_at = NULL`, `completed_at
   = NULL`, `current_lease_id = NULL`, `attempt = attempt + 1`.
3. Cancel any orphaned blockers tied to the job:
   `DELETE FROM blockers WHERE job_id = ? AND resolved_at IS NULL`.
4. Re-enqueue via `enqueue_job(conn, job_id=...)` (idempotent — it
   inserts a new `queue_messages` row).
5. If the run state is `canceled`/`failed`, transition back to
   `running` (operator chose to revive). Emit `run.retried`.
6. Emit `job.retried` event.

### Why allow retry to revive a canceled run

If the operator canceled the run hastily and now wants to retry one
job, refusing because the run is canceled is annoying. Reviving to
`running` keeps the existing run_id (no new row, no branch
re-confirmation). Documented in BUILD_HANDOFF.

## V4 scope summary

| Surface | Action |
| --- | --- |
| `src/striatum/migrations.py` | Migration v11: ALTER TABLE runs ADD paused_at, paused_reason |
| `src/striatum/schema.py` | V1 baseline: include the new columns |
| `src/striatum/db.py` | `pause_run`, `resume_run`, `retry_job` mutations + claim_next gate |
| `src/striatum/cli/parser.py` | `striatum run pause/resume/retry-job` |
| `src/striatum/cli/dispatch.py` | Wire new subcommands |
| `src/striatum/service.py` | POST /run/<id>/(pause|resume), POST /run/<id>/job/<jid>/(cancel|retry) |
| `src/striatum/web/templates/run_detail.html` | Pause/Resume buttons |
| `src/striatum/web/static/run_pause_resume.js` | Pause/Resume island |
| `src/striatum/web/templates/job_detail.html` | Cancel/Retry buttons |
| `src/striatum/web/static/job_actions.js` | Job-button island |
| `tests/test_pause_resume.py` (new) | Pause/resume mutations + claim-next gate |
| `tests/test_retry_job.py` (new) | Retry mutation |
| `tests/test_web_run_pause_resume.py` (new) | HTTP surfaces |
| `tests/test_web_job_actions.py` (new) | Cancel/retry web wrappers |

## Migration v11 SQL

```python
def _apply_v11_runs_paused_columns(conn: sqlite3.Connection) -> None:
    """RFC 0024 V4: add paused_at + paused_reason to runs."""
    cols = [row[1] for row in conn.execute("PRAGMA table_info(runs)").fetchall()]
    if "paused_at" not in cols:
        conn.execute("ALTER TABLE runs ADD COLUMN paused_at TEXT")
    if "paused_reason" not in cols:
        conn.execute("ALTER TABLE runs ADD COLUMN paused_reason TEXT")
```

Forward-only; idempotent against fresh DB whose `schema.py` already
includes the column.

## Test precedent

Existing `tests/test_recovery_extended.py` covers `cancel_job`
flows. New tests follow the same pattern — `_run_state` /
`_job_state` SQL helpers, then assert state transitions.

## Out of scope (V5 if needed)

- Running a partially-canceled run "auto-resume" (operator must
  retry job individually).
- Pause-with-deadline (auto-resume at timestamp).
- Recovery integration: pause as escalation hook target.
- Per-lane pause (pause only `claude_code` work, leave `codex`
  running).
