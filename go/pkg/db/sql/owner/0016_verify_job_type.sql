-- RFC 0134 (D227) — owner bundle 0016: 'verify' job_type.
--
-- striatumd.jobs is an OWNER-HELD table in the two-role posture (owned by the
-- database owner, not the runtime role striatumd_rw). The job_type is persisted
-- under the inline jobs_job_type_check CHECK constraint first defined by runtime
-- migration 0005 and last widened by owner bundle 0013 (barrier_assembly). A
-- CHECK constraint cannot be widened in place, so adding a new permitted value
-- means DROP + re-ADD the constraint — owner-table DDL the runtime role cannot
-- perform (`must be owner of table jobs`; the RFC 0079 §5 / RFC 0081 owner-held
-- ALTER crash-loop). It therefore lives in this owner/admin bundle rather than a
-- regular runtime migration, and the runtime guard test
-- TestFutureRuntimeMigrationsDoNotCarryOwnerDDL keeps the CHECK widening out of
-- the runtime tree.
--
-- The new value `verify` is the RFC 0134 / D227 EXECUTABLE-VERIFICATION job type:
-- a disposable, sandboxed verifier LANE runs an operator-curated, content-
-- addressed check OFF the daemon's gate path and mints a tamper-evident
-- receipt.v1 (argv + resolved binary sha256 + exit code + stdout digest + cwd
-- tree-sha + seal). The daemon NEVER executes the check; the run-completion gate
-- only READS the sealed receipt to allow a claim's VERIFIED status (a missing or
-- wedged verify degrades the claim to ASSERTED, never blocks completion on engine
-- liveness). Persisting a verify job needs this value or the live
-- jobs_job_type_check would refuse it.
--
-- This bundle MUST carry every value the prior live constraint permitted (owner
-- bundle 0013's set) PLUS 'verify' — a DROP+re-ADD replaces the whole IN-list, so
-- omitting an existing value would orphan in-flight jobs of that type.
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
       AND pg_get_constraintdef(oid) NOT LIKE '%verify%'
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
          'barrier_assembly','verify'
        )
      );
  END IF;
END
$$;
