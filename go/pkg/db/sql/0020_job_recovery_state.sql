-- RFC 0101 Phase 3 Slice 2: per-job autonomous-recovery budgets.
--
-- The crash-safe recovery sweep (recovery.RunScheduler ->
-- ActiveRunSweep.SweepOnce -> mutations.SweepRun -> HandleRecoveryAuto) gains an
-- autonomous decision tree that reclaims genuinely-stuck jobs (dead/stalled
-- owning session, lease released/expired, no recoverable artifact) on the SAME
-- attempt via requeueJobSameAttempt. To keep that loop convergent and bounded,
-- each job carries a recovery-budget row: how many times it has been requeued /
-- transferred / respawned by the daemon, the last action taken, and whether the
-- relevant budget has been exhausted (escalation_pending). Phase 4 consumes
-- escalation_pending to flip the run to needs_operator; this slice only records
-- it. The row is owner-applied at deploy + substrate_version bumped to 20.
CREATE TABLE IF NOT EXISTS striatumd.job_recovery_state (
  repository_id text NOT NULL,
  run_id        text NOT NULL,
  job_id        text NOT NULL,
  requeue_count   int NOT NULL DEFAULT 0,
  transfer_count  int NOT NULL DEFAULT 0,
  respawn_count   int NOT NULL DEFAULT 0,
  last_recovery_action text,
  last_recovery_at     timestamptz,
  last_stall_class     text,
  escalation_pending   boolean NOT NULL DEFAULT false,
  escalated_at         timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (repository_id, job_id)
);

-- The daemon connects as striatumd_rw; grant it DML on the new table (the
-- owner-applied table would otherwise be inaccessible to the runtime role and
-- the recovery sweep would fail permission-denied). Matches the grant pattern
-- every other table-creating migration uses (e.g. 0011 escalation_inbox).
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.job_recovery_state TO striatumd_rw;
  END IF;
END;
$$;
