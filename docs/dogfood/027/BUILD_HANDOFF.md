---
title: "RFC 0024 V4 build handoff (dogfood-027)"
date: 2026-05-09
---

# Build handoff: RFC 0024 V4 (pause/resume + per-job cancel + retry)

author: implementer-codex-gpt-5.5-001

## Scope

Three editor-side mutations layered on top of V3:

1. **Pause/resume run** via `runs.paused_at` + `runs.paused_reason`
   columns (migration v11). `claim_next` gates on `paused_at IS NOT NULL`
   without changing the run state machine. Idempotent. Orthogonal
   to the existing lifecycle.
2. **Per-job cancel via web** — wraps existing `recovery.cancel_job`
   with cascade=true default in a new HTTP route + UI button.
3. **`retry_job` mutation** — resets failed/canceled/blocked jobs to
   `queued`, re-enqueues, and (per design F1 option C) revives a
   canceled/failed *run* to `running` with a loud `run.revived` event.

## Files

### New

- `src/striatum/web/static/run_pause_resume.js` — pause/resume button
  island.
- `src/striatum/web/static/job_actions.js` — per-job cancel/retry
  button island. Cancel confirm dialog reads "Cancel this job AND
  its dependents…" per design-review F3.
- `tests/test_pause_resume.py` — 7 CLI tests (pause/resume from
  source states, idempotency, --reason, refuse terminal).
- `tests/test_retry_job.py` — 5 CLI tests (retry from
  failed/canceled/blocked, refuse running, refuse completed run per
  F4).
- `tests/test_web_run_pause_resume.py` — 10 HTTP tests (pause /
  resume / mutation gate / job-cancel / job-retry / button rendering).

### Modified

- `src/striatum/migrations.py` — migration v11 adds `paused_at` +
  `paused_reason` columns to `runs` (forward-only, idempotent).
- `src/striatum/schema.py` — V1 baseline updated alongside per RFC
  0006 convention.
- `src/striatum/db.py`:
  - New `pause_run(conn, *, run_id, reason)`.
  - New `resume_run(conn, *, run_id)`.
  - New `retry_job(conn, *, run_id, job_id)` — clears terminal
    job state, increments `attempt`, marks prior queue_messages
    rows as `canceled` (respects `uq_active_work_message_per_job`
    partial unique index), re-enqueues, revives canceled/failed
    run.
  - `claim_next` adds the `paused_at` gate after the
    `state != "running"` check.
- `src/striatum/cli/parser.py` — `striatum run pause/resume/retry-job`.
- `src/striatum/cli/dispatch.py` — wires the new subcommands.
- `src/striatum/service.py`:
  - `_dispatch_post` adds `/run/<id>/(pause|resume)` and
    `/run/<id>/job/<jid>/(cancel|retry)` branches; the existing
    `/run/<id>/cancel` branch is now guarded against `/job/`
    substring overlap.
  - New `_handle_run_pause`, `_handle_run_resume`,
    `_handle_job_action` handlers + `_read_json_body_strict`
    helper (V2's `_read_json_body` had different semantics; new
    name avoids redefinition).
- `src/striatum/web/templates/run_detail.html` — Pause/Resume
  buttons (when state non-terminal) + paused status pill.
- `src/striatum/web/templates/job_detail.html` — Cancel button
  (when non-terminal) + Retry button (when failed/canceled/blocked).

### Docs

- `CHANGELOG.md` — `## 1.18.0 — 2026-05-09` section.
- `pyproject.toml` + `__init__.py` — bumped to `1.18.0`.
- `docs/DECISION_LOG.md` — D080 row.
- `docs/TODO.md` — F27 row.
- `docs/rfcs/0024-workflow-browser-and-builder.md` — status
  `accepted (V1+V1.5+V2+V3+V4)`.
- `docs/rfcs/README.md` — index updated.

## Design-review disposition

| Finding | Severity | Disposition |
| --- | --- | --- |
| F1: Make run-revival explicit | recommend | **Adopted (option C)**: `retry_job` revives canceled/failed runs and emits a loud `run.revived` event with `previous_run_state` payload. Documented here and in CHANGELOG. |
| F2: retry_job state guard before enqueue | verify | **Confirmed**: `retry_job` raises `InvalidTransitionError` if the source state isn't in `{failed, canceled, blocked}` *before* touching `enqueue_job`. |
| F3: Cascade-confirm dialog text | recommend | **Adopted**: `job_actions.js` confirm reads "Cancel this job AND its dependents (jobs that depend on it)?". |
| F4: Test retry-on-completed-run | note | **Adopted**: `test_retry_refuses_completed_run` covers it; returns InvalidTransition (exit 4). |

## Implementation notes

### `paused_at` gate placement

The gate sits *after* the existing `state != "running"` check in
`claim_next`. This means terminal runs still return `no_work`
correctly; paused runs return `{"status": "no_work", "paused":
True}` so future tooling can distinguish.

### Re-enqueue without UNIQUE collision

`uq_active_work_message_per_job` is a partial unique index on
`queue_messages(job_id) WHERE state IN
('pending','claimed','acked')`. `retry_job` updates prior rows to
`state = 'canceled'` (which falls outside the partial predicate)
before calling `enqueue_job`, so the new pending row inserts
cleanly without violating the index. This also keeps the FK
references from `work_packets` and `events` intact (no DELETE).

### `_read_json_body_strict` helper

V2 already had a `_read_json_body()` that didn't validate
Content-Type or cap body size. To avoid re-defining it, V4 adds
`_read_json_body_strict(max_bytes)` for the new endpoints. Long-term
the two helpers should probably consolidate.

## Smoke

```
$ curl -X POST http://127.0.0.1:8088/run/<id>/pause \
    -H "Content-Type: application/json" -d '{"reason": "test"}'
→ 200 {"ok": true, "data": {"status": "paused", ...}}

$ curl -X POST http://127.0.0.1:8088/run/<id>/resume \
    -H "Content-Type: application/json" -d '{}'
→ 200 {"ok": true, "data": {"status": "resumed", ...}}

$ curl -X POST http://127.0.0.1:8088/run/<id>/job/<jid>/retry \
    -H "Content-Type: application/json" -d '{}'
→ 200 {"ok": true, "data": {"new_state": "queued", "run_revived": true, ...}}
```

## Test results

- `tests/test_pause_resume.py`: 7 / 7.
- `tests/test_retry_job.py`: 5 / 5.
- `tests/test_web_run_pause_resume.py`: 10 / 10.
- `make lint`: clean.
- `make typecheck`: clean (80 source files).
- Full suite: pending.

## Out of scope (V5 if needed)

- Pause-with-deadline (auto-resume at timestamp).
- Per-lane pause.
- Recovery integration of pause as escalation hook target.
- Consolidate `_read_json_body` / `_read_json_body_strict`.
