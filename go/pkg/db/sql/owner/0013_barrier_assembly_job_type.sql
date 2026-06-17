-- RFC 0135 P2 (D215/D216) — owner bundle 0013: 'barrier_assembly' job_type.
--
-- striatumd.jobs is an OWNER-HELD table in the two-role posture (owned by the
-- database owner, not the runtime role striatumd_rw). The job_type is persisted
-- under the inline jobs_job_type_check CHECK constraint first defined by owner
-- bundle 0005 (CREATE TABLE striatumd.jobs ... job_type text NOT NULL CHECK (...)).
-- A CHECK constraint cannot be widened in place, so adding a new permitted value
-- means DROP + re-ADD the constraint — owner-table DDL the runtime role cannot
-- perform (`must be owner of table jobs`; the RFC 0079 §5 / RFC 0081 owner-held
-- ALTER crash-loop). It therefore lives in this owner/admin bundle rather than a
-- regular runtime migration, and the runtime guard test
-- TestFutureRuntimeMigrationsDoNotCarryOwnerDDL keeps the CHECK widening out of
-- the runtime tree (RFC 0135 P2's runtime migration 0030 creates only the new
-- barrier_state table and carries NO owner-table DDL).
--
-- The new value `barrier_assembly` is the RFC 0135 P2 recoverable assembly job
-- type: the deferred post-completion join (RFC 0133 Slice 3) graduates from the
-- P1 opt-in plumbing into a first-class, recoverable job backed by a
-- barrier_state row (sealed -> assembling -> committed|failed) with two-phase
-- journaling. The job that folds every live-staged sibling contribution onto the
-- frozen tip and CAS-advances the run branch is a `barrier_assembly` job; without
-- this value the live jobs_job_type_check would refuse to persist it.
--
-- Owner-bundle number coordination (per RFC 0135 / RFC 0132 / D215): RFC 0132's
-- RESERVED optional `dissent_quarantine` run-state bundle becomes 0014. Per
-- D215 "whichever lands first takes 0013, the other 0014," RFC 0135 P2 lands
-- first and takes 0013; the RFC 0132 dissent-quarantine bundle is 0014.
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
     WHERE conname = 'jobs_job_type_check'
       AND conrelid = 'striatumd.jobs'::regclass
       AND pg_get_constraintdef(oid) NOT LIKE '%barrier_assembly%'
  ) THEN
    ALTER TABLE striatumd.jobs
      DROP CONSTRAINT jobs_job_type_check;
  END IF;

  IF NOT EXISTS (
    SELECT 1
      FROM pg_constraint
     WHERE conname = 'jobs_job_type_check'
       AND conrelid = 'striatumd.jobs'::regclass
  ) THEN
    ALTER TABLE striatumd.jobs
      ADD CONSTRAINT jobs_job_type_check
      CHECK (
        job_type IN (
          'draft','review','ledger','synthesis','build','test',
          'human_checkpoint','generic',
          'barrier_assembly'
        )
      );
  END IF;
END
$$;
