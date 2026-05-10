---
title: "RFC 0024 V3 build handoff (dogfood-026)"
date: 2026-05-09
---

# Build handoff: RFC 0024 V3 (cancel run + dirty-tree visibility + design-review F1/F3)

author: implementer-claude-opus-001

## Scope

V3 ships the run-mutation surface the operator asked for, plus the
dirty-tree visibility V2 had deferred:

1. **`cancel_run`** mutation — top-down cancellation in `db.py`,
   exposed via CLI (`striatum run cancel`), HTTP (`POST
   /run/<id>/cancel`), and a UI button on the run page.
2. **Run-now dirty-tree 409** — closes V2 design-review F3. When
   `git_create_or_checkout_branch` fails, `_handle_workflow_run_now`
   detects the pattern and returns 409 with `git status --short`
   in the body so the operator sees what's blocking.
3. **F1 disposition** — design-review F1 recommended dropping the
   redundant Re-run button. **Adopted.** The existing Run-now
   button is the re-run path; no second button.

## Files

### New

- `src/striatum/web/static/run_cancel.js` — ~30-LoC island for the
  Cancel-run button.
- `tests/test_cli_run_cancel.py` — 5 end-to-end CLI tests covering
  cancel from ready, cancel from running (jobs cleanup),
  idempotency, --reason flag, and InvalidTransition on completed.
- `tests/test_web_run_cancel.py` — 6 HTTP tests: 200 success, 200
  idempotent, 405 no-mutations, 404 missing run, 409 completed,
  and run page renders the button.

### Modified

- `src/striatum/db.py` — new `cancel_run(conn, *, run_id, reason)`
  function. Marks in-flight jobs (queued/running/blocked/ready/claimed)
  canceled, releases active leases (via `owner_session_id IN sessions
  WHERE run_id = ?`), updates the run row, emits `run.canceled`
  event, calls existing `close_remaining_sessions`. Idempotent on
  already-canceled. Refuses completed/failed via
  `InvalidTransitionError`.
- `src/striatum/cli/parser.py` — adds `striatum run cancel
  --run-id <id> [--reason]`.
- `src/striatum/cli/dispatch.py` — wires `run cancel` to
  `db.cancel_run`.
- `src/striatum/service.py`:
  - `_dispatch_post` adds `/run/<id>/cancel` branch.
  - New `_handle_run_cancel` — mutation gate, content-type check,
    body cap, calls `cancel_run`. 404 / 409 / 500 paths.
  - `_handle_workflow_run_now` — `WorkflowError` catch arm now
    detects "git checkout failed" messages and re-emits as 409
    `dirty_tree` with `git status --short` captured.
- `src/striatum/web/templates/run_detail.html` — Cancel button
  rendered when state is non-terminal; `<script defer
  src="/static/run_cancel.js">`.

### Docs

- `CHANGELOG.md` — `## 1.17.0 — 2026-05-09` section.
- `pyproject.toml` + `__init__.py` — bumped to `1.17.0`.
- `docs/DECISION_LOG.md` — D079 row.
- `docs/TODO.md` — F26 row.
- `docs/rfcs/0024-workflow-browser-and-builder.md` — status
  `accepted (V1+V1.5+V2+V3)`.
- `docs/rfcs/README.md` — index updated.

## Design-review disposition

| Finding | Severity | Disposition |
| --- | --- | --- |
| F1: Drop the redundant Re-run button | recommend | **Adopted**: did not ship the second button. Run-now is the re-run path. |
| F2: Verify `ack_work` refuses canceled runs | verify | **Confirmed transitive**: `cancel_run` marks jobs `canceled` (including state=`claimed`), so `ack_work`'s existing `job["state"] == "claimed"` check refuses with `InvalidTransitionError("work must be claimed before ack")`. No new code needed. |
| F3: Test cancel-during-ack race | note | **Covered**: `test_cancel_run_from_running` cancels mid-claim and asserts non-terminal job count is 0 afterward; `cancel_run` SQL includes `claimed` so the race state is handled. |

## Smoke

End-to-end against the local service after restart:

```
$ curl -X POST http://127.0.0.1:8088/run/<id>/cancel \
    -H "Content-Type: application/json" -d '{}'
→ 200 {"ok": true, "data": {"run_id": "...", "state": "canceled"}}

$ curl -X POST http://127.0.0.1:8088/run/<id>/cancel \
    -H "Content-Type: application/json" -d '{}'
→ 200 {"ok": true, "data": {... "status": "already_canceled"}}

$ curl -X POST http://127.0.0.1:8088/workflows/run/<dirty-tree-workflow> \
    -H "Content-Type: application/json" -d '{}'
→ 409 {"ok": false, "error": {"code": 409, "kind": "dirty_tree",
        "git_status": "M src/foo.py\n?? newfile.py"}}
```

## Test results

- `tests/test_cli_run_cancel.py`: 5 / 5 pass.
- `tests/test_web_run_cancel.py`: 6 / 6 pass.
- Full suite: **485 / 485 pass** (309.26s).
- `make lint`: clean.
- `make typecheck`: clean (77 source files).

## Out of scope (V4)

- Pause / resume runs.
- Auto-branch suffix (research showed multi-run-per-branch is
  by-design).
- Per-job mutation buttons (kill running job, retry).
- Programmatic re-run with parameter overrides.
- Recovery integration: cancel-run as an escalation hook target.

## Acceptance summary

| Gate | Verified |
| --- | --- |
| `cancel_run` from ready | `test_cancel_run_from_ready` |
| `cancel_run` from running cancels in-flight jobs | `test_cancel_run_from_running` |
| `cancel_run` idempotent on already-canceled | `test_cancel_run_idempotent` |
| `cancel_run` accepts --reason | `test_cancel_run_with_reason` |
| `cancel_run` refuses completed → exit 4 | `test_cancel_run_refuses_completed` |
| HTTP 200 cancel | `test_cancel_run_route_200` |
| HTTP idempotent | `test_cancel_run_idempotent` |
| HTTP 405 no-mutations | `test_cancel_without_mutations_returns_405` |
| HTTP 404 missing run | `test_cancel_missing_run_returns_404` |
| HTTP 409 completed | `test_cancel_completed_returns_409` |
| Run page renders Cancel button | `test_run_page_renders_cancel_button` |
