---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
---

# Design review: RFC 0024 V3 (devils_advocate)

author: reviewer-claude-opus-001

## Posture

Devil's advocate. The synthesis claims a tight scope with one
mutation (cancel_run), one bug fix (dirty-tree 409), and one piece
of UI sugar (re-run). I will argue against each.

## Counter-claims

### C1: "cancel_run is idempotent on already-canceled"

The synthesis says re-cancelling a canceled run is a no-op. Concern:
in a multi-tab UI the user clicks Cancel twice, both POSTs land,
both call `cancel_run`. Result: first cancels, second sees terminal
canceled state and returns success. **Counter:** the synthesis is
explicit. No data corruption. **Survives.**

### C2: "Cancel cleanup is in one transaction"

Concern: if `close_remaining_sessions` fails mid-way (e.g. SQL
constraint), the lease release + jobs cancel + run state could be
already committed. **Counter:** synthesis says "single transaction"
explicitly; the existing `maybe_complete_run` uses the same
pattern with no observed corruption. SQLite WAL mode rollback covers
partial-failure. **Survives.**

### C3: "Cancel from `prepared` state is correct"

Concern: a `prepared` run has no enqueued jobs and no leases. Why
allow cancel from there? **Counter:** operator might have run
prepare via the editor and decided not to proceed. Cancelling a
`prepared` run cleans up the row + emits `run.canceled` so the
dashboard shows it as terminated rather than stuck. Cheap to
support. **Survives.**

### C4: "Dirty-tree 409 with git_status is helpful"

Concern: capturing `git status --short` adds a subprocess call to
the run-now hot path even when the tree is clean. **Counter:** the
status capture happens *only* in the error branch (when
`git_create_or_checkout_branch` fails). Clean-tree path is unchanged.
**Survives.**

### C5: "Re-run button is just sugar"

Concern: the synthesis admits this is "functionally identical" to
the existing Run-now button. Why ship it at all? **Finding (F1,
non-blocking):** It's redundant. The existing Run-now button
already handles re-runs. Ship a single button labeled appropriately
(e.g. "Run" or "Run this workflow now") and skip the duplicate. The
UI noise (two buttons doing the same thing) is worse than the
clarity benefit.

### C6: "InvalidTransition for completed runs is correct"

Concern: an operator might want to "rerun-from-failed" — cancelling
a failed run could be a precursor. **Counter:** that's V4 scope (per
the synthesis "out of scope" list). For V3, refusing to cancel an
already-terminal run keeps state machine semantics tight. The
operator can still re-run the workflow file (creates a new run_id).
**Survives.**

### C7: "Lease release uses owner_session_id IN (sessions)"

Concern: this assumes session ownership maps to run ownership. What
if a lease is held by an external session that's also working on
this run via a different role? **Counter:** In striatum's
single-operator local-first model, every session is registered to
exactly one run via `register-session --run-id`. Sessions don't span
runs. The query is correct. **Survives.**

### C8: "Cancel button in UI uses window.confirm"

Concern: window.confirm is not stylable and dated. **Counter:**
matches the V1.5 cancel-edit-draft confirm pattern; consistent with
existing islands. Custom-modal is a V4 concern. **Survives.**

### C9: "Cancel during job ack/lease race"

Race scenario: cancel_run starts, snapshots active leases. Job at
the same instant calls `ack_work` (acquires lease) — does the new
lease survive cancel? **Counter:** the cancel happens inside `with
transaction(conn)`, so SQLite's exclusive-lock semantics serialize
the operations. The other-direction race (ack starts during cancel)
sees the run already canceled and `ack_work` should detect that
and refuse. Need to verify that ack_work checks run state. If it
doesn't, this is a **Finding (F2, non-blocking):** ack_work could
silently grant a lease for a job in a canceled run; the implementer
should check.

### C10: "Test plan covers cascade integrity"

The plan lists `test_cancel_run.py` covering "releases leases",
"cancels in-flight jobs", "emits event". Looks complete. I'd add:
"cancel from each source state succeeds" as a parameterized test
to enumerate the state machine entry points. **Note for the
implementer.**

## Findings

### F1 (recommend, non-blocking): Drop the redundant Re-run button

The existing Run-now button is the re-run button when called
twice. Adding a separately-labeled "Re-run" button is UI noise.
Recommend either:
- Skip it entirely (keep Run-now).
- Keep one button, change the label to "Run this workflow" (drop
  "now").

### F2 (verify, non-blocking): Confirm ack_work refuses canceled runs

The implementer should check that `ack_work` in
`cli/mutations.py:431` consults the run state and returns an error
when the run has been canceled mid-claim. If it doesn't today, add
that check during V3 implementation.

### F3 (note, non-blocking): Test the cancel-during-ack race

Add a test that explicitly exercises the race: register a session,
claim a job, then cancel the run, then attempt to ack. Verify the
ack is refused with a useful error.

## Verdict

**accept**

Three findings, all non-blocking. The scope is tight and
defensible. F1 is a UI judgment call I'd defer to the implementer;
F2/F3 are verification asks, not new work.
