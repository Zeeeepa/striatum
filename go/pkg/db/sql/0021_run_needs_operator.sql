-- RFC 0101 Phase 4: loud structured escalation (a `needs_operator` run state).
--
-- Phase 3 (migration 0020) made the recovery sweep autonomously reclaim
-- genuinely-stuck jobs within per-job budgets and, when a budget is exhausted,
-- set striatumd.job_recovery_state.escalation_pending=true + escalated_at and
-- emit a recovery.budget_exhausted event. Nothing consumed that flag. Phase 4
-- consumes it: it turns an unrecoverable stuck job into a structured,
-- operator-actionable escalation (a blockers + escalation_inbox row, RFC 0062)
-- and flips the run to an explicit `needs_operator` state so a run never sits
-- silently `running`. The recovery sweep's `running`/`paused` active-run filter
-- excludes `needs_operator`, so an escalated run is paused (no re-sweep, no new
-- claims — claim.go gates new claims on run.state='running') until the operator
-- resolves the escalation, which flips the run back to `running`.
--
-- Two schema changes:
--   1. Add 'needs_operator' to the runs_state_check CHECK (worked example:
--      0007_decision_propagation.sql DROP+ADD pattern). The latest constraint
--      (0007) already allows ...,'compromised'; 0021 appends 'needs_operator'.
--   2. Add job_recovery_state.run_escalated_at: an idempotency guard so a
--      run-level escalation is raised once per escalation_pending budget row
--      (belt-and-suspenders with the sweep's running/paused filter). Added to
--      the 0020 table here via ADD COLUMN IF NOT EXISTS (0020 is immutable).
--
-- Owner-applied at deploy + substrate_version bumped to 21. No GRANT needed: no
-- new table is created and the runs / job_recovery_state grants already exist.

ALTER TABLE striatumd.runs
  DROP CONSTRAINT IF EXISTS runs_state_check;

ALTER TABLE striatumd.runs
  ADD CONSTRAINT runs_state_check CHECK (state IN (
    'needs_branch_confirmation','ready','running','blocked',
    'completed','failed','canceled','compromised','needs_operator'
  ));

ALTER TABLE striatumd.job_recovery_state
  ADD COLUMN IF NOT EXISTS run_escalated_at timestamptz;
