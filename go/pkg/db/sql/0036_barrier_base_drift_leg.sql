-- RFC 0133 future-slice (#353) — base-drift-as-a-recoverable-leg at the fan-in join.
--
-- Background: today the fan-in staging gate (stageFaninContribution) REFUSES any
-- staged commit that does not descend from the frozen tip with
-- git_commit_apply_failed ("base drift / contaminated base"). That refusal
-- conflates two genuinely different situations the RFC 0133 Risks section
-- distinguishes:
--
--   * CONTAMINATED base (#352 smuggled-content): the staged chain folds in an
--     off-base FOREIGN root lineage — content the sibling never authored on top of
--     the frozen base. This must stay REFUSED (barrier_smuggled_content).
--   * RECOVERABLE base drift (#299/#306): the run branch legitimately EVOLVED under
--     a sibling's feet (e.g. another sibling integrated), so the sibling authored
--     its change on the EVOLVED tip rather than the frozen tip. The staged commit
--     shares a real merge-base with the frozen tip (same lineage tree, no foreign
--     root) and 3-way-merges cleanly. RFC 0133 Risks: "Base drift (#299/#306): the
--     frozen tip diverging from the live branch. Handle as a recorded, recoverable
--     extra commit-tree parent leg, not a CAS wedge."
--
-- This migration records the recoverable rebase/extra-parent leg on the existing
-- runtime-owned staging table. When a contribution is staged over an evolved base
-- (not a descendant of the frozen tip, but a clean-merge drift rather than
-- contamination), the staging row records the evolved base the assembly must fold
-- the contribution against as an EXTRA commit-tree parent — the recoverable leg
-- the RFC names. A direct-descendant contribution leaves these NULL.
--
-- Runtime migration, NOT an owner bundle: striatumd.barrier_staged_contributions
-- is runtime-owned (created in migration 0029, written by striatumd_rw on every
-- sibling completion / requeue tombstone) and carries NO foreign key into the
-- owner-held jobs table (these are scalar columns, no new FK), per D215's
-- runtime-vs-owner split. Like 0035's extension of job_recovery_state, this ADDs
-- columns to a runtime-owned table with ADD COLUMN IF NOT EXISTS; a deployment
-- behind on this migration falls back to today's behaviour (every drift refused),
-- which is degrade-safe — never silent corruption. Owner-applied at deploy +
-- substrate_version bumped to 36.

-- The evolved base the recoverable contribution was authored on top of (the run
-- branch tip a sibling forked from after the frozen tip advanced under its feet).
-- The assembly folds the contribution via a 3-way merge against the frozen tip and
-- records this evolved base as an EXTRA commit-tree parent, so the run-branch
-- provenance of the recovered leg is auditable. NULL for a direct-descendant
-- contribution (the common, non-drift case).
ALTER TABLE striatumd.barrier_staged_contributions
  ADD COLUMN IF NOT EXISTS base_drift_onto_sha text;

-- Why the contribution was staged as a recoverable drift leg rather than a direct
-- descendant (provenance legibility for recovery / the manifest), NULL otherwise.
ALTER TABLE striatumd.barrier_staged_contributions
  ADD COLUMN IF NOT EXISTS base_drift_reason text;

-- ADD COLUMN inherits the table's existing grants, so the runtime role already has
-- DML on the new columns. Re-assert the grant idempotently anyway (own GRANT per
-- D215) so the migration is self-contained and matches the grant pattern 0029
-- established for this table — harmless if the role or grant already exists.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.barrier_staged_contributions TO striatumd_rw;
  END IF;
END;
$$;
