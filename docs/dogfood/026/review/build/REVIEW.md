---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
---

# Build review: RFC 0024 V3 (devils_advocate)

author: reviewer-claude-opus-002

## Posture

Devil's advocate. The build claims a tight V3: cancel_run mutation
+ HTTP/CLI/UI surfaces, dirty-tree 409 visibility, F1 adopted (no
re-run button). I will argue against each.

## Counter-claims

### C1: "cancel_run is correctly idempotent"

The handler short-circuits when `state == 'canceled'` and returns a
distinct `status: "already_canceled"` flag. Test
`test_cancel_run_idempotent` (CLI) and
`test_cancel_run_idempotent` (HTTP) both confirm. I tried to find a
case where a stale lease or queued message would still drain on the
second call — short-circuit returns before any UPDATE, so no.
**Survives.**

### C2: "Cancel from running drains in-flight jobs"

Test `test_cancel_run_from_running` asserts non-terminal job count
is 0 after cancel. The SQL covers `('queued','running','blocked','ready','claimed')`
— five states. I asked: what about `'paused'` (if such state existed)?
It doesn't (V4 deferral). What about `'completed'`/`'failed'`/
`'skipped'`/`'canceled'` jobs? They're terminal and correctly skipped.
**Survives.**

### C3: "Lease release uses correct ownership query"

The SQL `WHERE owner_session_id IN (SELECT session_id FROM sessions
WHERE run_id = ?)` is correct in striatum's model where every
session is registered to one run. I checked `register-session` —
yes, `--run-id` is required. **Survives.**

### C4: "F1 adopted: no redundant Re-run button"

I diffed `workflow_detail.html` — no new Re-run button was added.
The Run-now button (which V2 already shipped) handles re-runs by
creating a new run_id. F1 honored. **Survives.**

### C5: "F2 transitively covered"

The implementer claims `cancel_run` marking jobs `canceled`
including state=`claimed` means a racing `ack_work` will hit the
existing `job["state"] == "claimed"` check (which compares to
literal `"claimed"`, not `"canceled"`). Verified: `mutations.py:439`
reads `job["state"] != "claimed"` and rejects with
InvalidTransitionError. After `cancel_run`, the job is `canceled`,
so this check fires correctly. **Survives** without new code.

### C6: "F3 cancel-during-ack race tested"

`test_cancel_run_from_running` exercises the race indirectly: it
starts the run, calls cancel, and asserts no jobs remain in
non-terminal state. Direct race (claim + cancel + ack) isn't
explicitly tested. **Finding (F1, non-blocking):** Add an explicit
test where a session claims a job, cancel runs, then the session
attempts ack and asserts the InvalidTransition. Not blocking;
covered transitively.

### C7: "Dirty-tree 409 includes git_status"

Implementation uses `subprocess.run(["git", "status", "--short"], ...)`
with timeout=5s, capping at 80 lines. I asked: what if git binary
is missing? `OSError` is caught silently and `git_status=""` is
returned. The 409 still fires with the WorkflowError message. The
operator sees the failure but with empty git_status — graceful
degradation. **Survives.**

### C8: "CSP unchanged"

I diffed `run_detail.html` — only added a button + `<script
defer src="/static/run_cancel.js">` (separate file, CSP-clean). No
inline script blocks. **Survives.**

### C9: "Mutation gate is consistent"

Both `_handle_run_cancel` and `_handle_run_branch_confirm` (V2.1)
check `self.state.allow_mutations` first. Same pattern as
edit/run-now. Test `test_cancel_without_mutations_returns_405`
confirms. **Survives.**

### C10: "Test coverage adequate"

11 new tests + 485/485 full suite. Edge cases covered: idempotency,
each entry state, --reason flag, terminal-state refusal,
mutation-gate, missing-run, button rendering. The CLI
`exit_code=4` for InvalidTransition matches the contract. The race
test (F3) is the only soft spot. **Survives** with note.

## Findings

### F1 (note, non-blocking): Add an explicit cancel-during-ack race test

The current `test_cancel_run_from_running` covers the race
indirectly. A direct test that:
1. claims a job (state=claimed)
2. cancels the run
3. attempts ack with the same lease

…and asserts `InvalidTransitionError` would belt-and-suspenders the
F2/F3 verification asks. Not blocking — the SQL already covers
the `claimed` state.

## Verdict

**accept**

The build survives every devil's-advocate counterargument. F1 from
design review adopted; F2/F3 transitively covered. Single non-blocking
note about explicit race test. The dirty-tree 409 closes a real
operator pain point that V2 left open. cancel_run is the right
mutation surface for what was asked.
