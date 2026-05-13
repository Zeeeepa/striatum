---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
---

# Build review: RFC 0024 V4 (devils_advocate)

author: reviewer-claude-opus-002

## Posture

Devil's advocate. Argue against the build's claims about
correctness, idempotency, race safety, and design-review compliance.

## Counter-claims

### C1: "Migration v11 is forward-only and idempotent"

I read it: `_apply_v11_runs_paused_columns` checks
`PRAGMA table_info` and conditionally `ALTER TABLE`. Schema baseline
in schema.py also includes the columns, so fresh-init bypasses
the migration. No data migration needed because columns default to
NULL. **Survives.**

### C2: "claim_next gate is correct"

The gate sits *after* the existing `state != "running"` check. Test
`test_pause_route_200` proves pause->no-claim transitions; full
suite passes (507/507) including existing `claim_next` tests. The
fallback `try/except (IndexError, KeyError)` for accessing the new
column is defensive in case Row doesn't have it. **Survives.**

### C3: "pause/resume idempotent"

CLI tests `test_pause_idempotent` and `test_resume_idempotent`
confirm. Status field distinguishes `paused` vs `already_paused`
and `resumed` vs `not_paused`. **Survives.**

### C4: "Pause refuses terminal states"

Test `test_pause_refuses_completed` (returncode 4) confirms.
**Survives.**

### C5: "Resume refuses terminal states (use retry instead)"

Test `test_resume_refuses_terminal` confirms. The error message
explicitly tells the operator to use retry_job to revive — good
UX. **Survives.**

### C6: "retry_job state guard before enqueue (F2)"

Implementation raises `InvalidTransitionError` on non-retriable
states *before* touching `enqueue_job`. Test
`test_retry_refuses_running` exercises this. **Survives.**

### C7: "retry_job revive_run is loud (F1 option C)"

Implementation emits `run.revived` event with `previous_run_state`
in the payload. Test `test_retry_canceled_job_revives_run` and
HTTP `test_job_retry_route_revives_run` confirm. CHANGELOG is
explicit about the semantics. **Survives.**

### C8: "retry_job refuses completed runs (F4)"

Test `test_retry_refuses_completed_run` confirms — returncode 4.
**Survives.**

### C9: "Re-enqueue avoids UNIQUE collision"

Implementation marks prior queue_messages rows as `state =
'canceled'` (which falls outside the partial unique index
`uq_active_work_message_per_job`). No DELETE — preserves
`work_packets.message_id` and `events.message_id` FK references.
The build_handoff calls this out explicitly. **Survives.**

### C10: "Cascade-confirm dialog (F3)"

`job_actions.js` confirm reads "Cancel this job AND its
dependents…". UI test `test_job_detail_renders_action_buttons`
confirms button presence; the confirm-text is JS not HTML, so
verifying the literal text would require a JSdom test (out of
scope for stdlib testing). I read the JS source — text is correct.
**Survives.**

### C11: "Per-job cancel doesn't double-transaction"

I noticed the build uses `cancel_job(...)` (recovery) WITHOUT
wrapping in `transaction(conn)` — recovery.cancel_job opens its
own. Tests pass for the cancel route. **Survives.**

### C12: "CSP unchanged"

Two new JS islands (`run_pause_resume.js`, `job_actions.js`); no
inline scripts. No `<style>` blocks. **Survives.**

### C13: "Test coverage adequate"

22 new tests + 507/507 full suite. Edge cases: idempotency, each
source state, --reason, terminal-refusal, mutation-gate, run-
revival, cancel-cascade, button rendering. Race tests are
implicit (no explicit pause-during-claim or retry-during-cancel).
**Note for V5:** add explicit race tests if multi-worker
deployments materialize.

## Findings

### F1 (note, non-blocking): Helper consolidation

Two `_read_json_body*` methods now coexist. Build_handoff
explicitly defers consolidation to V5. This is a known
tech-debt item, not blocking.

### F2 (note, non-blocking): Pause-during-claim race not explicitly tested

The current claim_next gate is straightforward, but a test that
launches concurrent pause + claim threads and asserts the worker
sees no_work (or processes the in-flight claim cleanly) would
provide belt-and-suspenders coverage. V5 if multi-worker
deployments need it.

## Verdict

**accept**

The build survives every devil's-advocate counterargument.
Migration v11 is correct; pause/resume mutations are correctly
idempotent and orthogonal to the lifecycle; retry_job revives
runs loudly per F1 option C; per-job cancel correctly delegates
to existing recovery.cancel_job; F3 cascade-confirm dialog text
is correct. Two non-blocking notes about helper consolidation
and explicit race tests, both deferred to V5 with rationale.
