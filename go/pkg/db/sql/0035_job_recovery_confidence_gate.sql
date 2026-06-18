-- RFC 0131 Layer 3 (131-C): confidence-gated pipe-lane escalation state.
--
-- 131-A (#334, migration-free) made transport + a typed probe_basis
-- (deadline_elapsed_only vs pty_confirmed_dead) first-class OUTPUTS of the
-- liveness classifier and carried them onto the recovery.* event payloads.
-- 131-C (#336) is the load-bearing core: a pipe / agent-loop lane
-- (agy/Gemini, no pty_helper oracle) whose stall basis is deadline_elapsed_only
-- must NOT escalate the run on a single sweep's elapsed deadline. It must clear a
-- higher bar — corroboration across consecutive silent sweeps and/or a cross-lane
-- cohort liveness signal — before it may flip the run toward needs_operator. A
-- pty_confirmed_dead basis (the PTY oracle fired) escalates immediately, exactly
-- as it does today.
--
-- This adds the three confidence-gate columns the RFC names to the EXISTING
-- runtime table striatumd.job_recovery_state (migration 0020, extended again here
-- the way 0021 added run_escalated_at via ADD COLUMN IF NOT EXISTS — 0020 is
-- immutable). None of these columns exist today:
--
--   misfire_evidence_score     int  — a monotone-compounding confidence signal
--                                     that a deadline_elapsed_only escalation is a
--                                     misfire rather than a genuine hang. It rises
--                                     each silent sweep and is reset to 0 only by a
--                                     FORGERY-RESISTANT progress signal (a sealed
--                                     artifact anchor / sealed verdict advanced —
--                                     daemon-recorded sealed-work progress a
--                                     dead-but-spinning loop cannot manufacture).
--   consecutive_silent_sweeps  int  — the monotone silent-sweep counter that is
--                                     the Layer-4 escape valve. It increments every
--                                     deadline_elapsed_only sweep with no
--                                     forgery-resistant progress and resets to 0
--                                     ONLY on such progress. Once it reaches the cap
--                                     (default (max_requeues*2)+3) escalation fires
--                                     regardless of misfire_evidence_score: silence
--                                     itself is the oracle a pipe lane otherwise
--                                     lacks, so the lane is escalatable in bounded
--                                     time by construction (never un-escalatable,
--                                     Goal 3).
--   last_probe_basis           text — the probe_basis the previous sweep recorded
--                                     for this job (deadline_elapsed_only |
--                                     pty_confirmed_dead). Lets the gate require the
--                                     evidence STRICTLY INCREASED across two
--                                     consecutive deadline_elapsed_only sweeps
--                                     before committing escalation (a single sweep
--                                     never escalates a pipe lane).
--
-- Runtime migration, NOT an owner bundle: job_recovery_state is runtime-owned
-- (striatumd_rw writes it on every recovery sweep) and carries NO foreign key into
-- the owner-held jobs table (these are scalar columns, no new FK), per D215's
-- runtime-vs-owner split. A deployment behind on this migration falls back to
-- today's ungated single-sweep escalation (degrade-safe, like P0's bundle-0012
-- guard): the gate's reads tolerate the absent columns and behave as the
-- pre-131-C code path. Owner-applied at deploy + substrate_version bumped to 35.

ALTER TABLE striatumd.job_recovery_state
  ADD COLUMN IF NOT EXISTS misfire_evidence_score int NOT NULL DEFAULT 0;

ALTER TABLE striatumd.job_recovery_state
  ADD COLUMN IF NOT EXISTS consecutive_silent_sweeps int NOT NULL DEFAULT 0;

ALTER TABLE striatumd.job_recovery_state
  ADD COLUMN IF NOT EXISTS last_probe_basis text;

-- ADD COLUMN inherits the table's existing grants, so the runtime role already
-- has DML on the new columns. Re-assert the grant idempotently anyway (own GRANT
-- per D215) so the migration is self-contained and matches the grant pattern
-- 0020 established for this table — harmless if the role or grant already exists.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.job_recovery_state TO striatumd_rw;
  END IF;
END;
$$;
