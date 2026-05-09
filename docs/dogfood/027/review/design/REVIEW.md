---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
---

# Design review: RFC 0024 V4 (devils_advocate)

author: reviewer-claude-opus-001

## Posture

Devil's advocate. Argue against scope, semantics, idempotency,
race safety, schema choice.

## Counter-claims

### C1: "Pause as column is more correct than pause as state"

The synthesis chooses the orthogonal-column model. Concern: this
splits the run lifecycle conceptually into "state" and "is paused" —
two truths to track instead of one. **Counter:** pause IS
orthogonal in spirit (we're suspending claim-eligibility, not
changing what work exists). Mixing it into `state` would force every
state-transition site to special-case it. The column model is the
narrower change. **Survives.**

### C2: "Idempotent pause/resume"

The synthesis says idempotent: re-pause = no-op, re-resume = no-op.
Concern: what if pause race lands (two operators pause at the same
instant)? **Counter:** SQL `UPDATE ... WHERE paused_at IS NULL`
naturally idempotent — second update finds nothing to change,
emits no event. **Survives.**

### C3: "Claim-next gate is correct"

`if run["paused_at"] is not None: return no_work` after the
existing state check. Concern: race between resume and claim-next.
Operator pauses; worker hits no_work; operator resumes; worker
might miss the resume (its prior no_work response already returned).
**Counter:** workers poll claim-next; they'll re-check on the next
poll. Worst case: one cycle of latency. Not a correctness issue.
**Survives.**

### C4: "Retry revives canceled runs"

Synthesis says retry on a canceled run transitions the run back to
running. **Concern:** this conflicts with the V3 state-machine
hardening that said terminal states are sacred. **Finding (F1,
non-blocking):** Reconsider. Reviving a canceled run quietly is
surprising; an explicit "uncancel" or "revive_run" mutation would
be cleaner. Either:
- (A) Refuse retry on canceled runs; operator must manually `run
  start` again (but `run_start` won't accept a canceled run).
- (B) Add `revive_run` as a separate mutation; retry never
  transitions run state.
- (C) Keep synthesis as-is but emit `run.revived` and document
  loudly.

I lean (B). But (C) is acceptable if BUILD_HANDOFF is honest about
the semantic gymnastics.

### C5: "retry_job re-enqueues idempotently"

`enqueue_job` is described as idempotent. Concern: if retry is
called on a job that's already queued, do we get duplicate
queue_messages rows? **Verify:** check `enqueue_job`
implementation. The synthesis says "inserts new pending
queue_messages row" — if this is *unconditional*, retrying a
queued job creates a duplicate. The synthesis already constrains
retry to `{failed, canceled, blocked}` source states, so a queued
job wouldn't be retried — but the implementation should
defensively check. **Finding (F2, non-blocking):** Add an explicit
state check before `enqueue_job` in `retry_job`.

### C6: "Cascade=True default for web cancel-job"

Concern: cascade silently cancels dependents. An operator clicking
Cancel on one job might unintentionally cancel many. **Finding (F3,
non-blocking):** UI should display dependents-to-be-canceled in
the confirm dialog. For V4, simpler: pass `cascade=true` but set
the alert text to "Cancel this job AND its dependents (jobs that
depend on it)?". Lower-risk than exposing a `cascade` toggle.

### C7: "Migration v11 is forward-only and idempotent"

Synthesis includes `if "paused_at" not in cols` guard. Matches V9
and V10 patterns. **Survives.**

### C8: "Schema baseline alongside migration"

Per RFC 0006 convention, fresh-init must include the column
without running v11. Synthesis says schema.py is updated. I'll
verify this in build review. **Survives.**

### C9: "Pause from `prepared`/`needs_branch_confirmation`"

Synthesis allows pause from these pre-running states. Concern: what
does it mean to pause a run that hasn't started? It still works
(claim_next already returns no_work for non-running states), so
pause adds nothing. **Counter:** harmless. Operator paused-then-
resumed semantics work intuitively. Could refuse but no benefit.
**Survives.**

### C10: "Test plan covers retry-revives-run"

The plan mentions "revives canceled run" — explicitly tests F4
behavior. Good. Add a test for "retry on a job whose run is
completed → InvalidTransition" — completed runs should not be
revived. **Note for implementer.**

## Findings

### F1 (recommend, non-blocking): Make run-revival explicit

Either separate `revive_run` mutation or loud documentation.
Implementer's choice.

### F2 (verify, non-blocking): retry_job state guard before enqueue

Defense-in-depth check that the job is in a retriable state before
calling `enqueue_job`.

### F3 (recommend, non-blocking): Cascade-confirm dialog text

UI confirm: "Cancel this job AND its dependents?" — make it
visible.

### F4 (note): Test retry-on-completed-run

Should refuse with InvalidTransition. Add to retry test plan.

## Verdict

**accept_with_findings**

Four findings, all non-blocking. Scope is tight. F1 is the
biggest semantic question; implementer should pick a path and
document it loudly.
