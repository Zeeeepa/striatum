-- GH #311 P0 — owner bundle 0012: 'quarantined' job state.
--
-- striatumd.jobs is an OWNER-HELD table in the two-role posture (owned by the
-- database owner, not the runtime role striatumd_rw). The job state is persisted
-- under the inline jobs_state_check CHECK constraint first defined by owner
-- bundle 0005 (CREATE TABLE striatumd.jobs ... state text NOT NULL CHECK (...)).
-- A CHECK constraint cannot be widened in place, so adding a new permitted value
-- means DROP + re-ADD the constraint — owner-table DDL the runtime role cannot
-- perform (`must be owner of table jobs`; the RFC 0079 §5 / RFC 0081 owner-held
-- ALTER crash-loop). It therefore lives in this owner/admin bundle rather than a
-- regular runtime migration, and the runtime guard test
-- TestFutureRuntimeMigrationsDoNotCarryOwnerDDL keeps it out of the runtime tree.
--
-- The new value `quarantined` is the #311 finalize-the-majority state: when a
-- SINGLE job exhausts its autonomous-recovery budget but no unfinished job
-- depends on it and it is not a provenance-required reviewer the run-completion
-- gate would refuse, recovery moves ONLY that job to 'quarantined' (NOT
-- completed — never a false completion) and lets the run finalize on the
-- already-completed deliverables, recording a quarantine manifest. The
-- quarantined job is the one narrow thing surfaced to the operator, who
-- terminalizes it with `recovery accept-quarantined`. Without this value the
-- decision tree cannot persist the state and every UPDATE that tried to would
-- fail the CHECK.
--
-- Idempotent: the DROP is guarded by a pg_constraint definition probe and the
-- ADD by an existence probe, so a re-run (or applying onto a DB an operator has
-- already hand-patched) is a safe no-op. ApplyOwnerBundles runs the whole bundle
-- in one transaction and stamps owner_bundle_meta last, so the swap is atomic.

DO $$
BEGIN
  -- Only rebuild the constraint when it does not already permit the new value.
  -- pg_get_constraintdef renders the full CHECK text; probing it keeps the swap
  -- idempotent without parsing the IN-list.
  IF EXISTS (
    SELECT 1
      FROM pg_constraint
     WHERE conname = 'jobs_state_check'
       AND conrelid = 'striatumd.jobs'::regclass
       AND pg_get_constraintdef(oid) NOT LIKE '%quarantined%'
  ) THEN
    ALTER TABLE striatumd.jobs
      DROP CONSTRAINT jobs_state_check;
  END IF;

  IF NOT EXISTS (
    SELECT 1
      FROM pg_constraint
     WHERE conname = 'jobs_state_check'
       AND conrelid = 'striatumd.jobs'::regclass
  ) THEN
    ALTER TABLE striatumd.jobs
      ADD CONSTRAINT jobs_state_check
      CHECK (
        state IN (
          'blocked','queued','claimed','running','stale_lease',
          'waiting_human','completed','failed','canceled','skipped',
          'quarantined'
        )
      );
  END IF;
END
$$;
