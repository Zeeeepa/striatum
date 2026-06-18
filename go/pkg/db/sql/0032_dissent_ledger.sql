-- RFC 0135 P4 (#339, D214/D215/D216) — the forward-written dissent ledger for the
-- panel-quorum instance of the sealed expectation barrier (entity=review_seat keyed
-- by stable workflow_job_id, seal=attempt). This migration creates ONE new
-- runtime-owned, append-only table: a seal-durable dissent witness that BLOCKS panel
-- finalize wherever recovery later moved the seat's lineage.
--
-- A live dissent row is the quorum analog of "a superseded-seal contribution is
-- invisible" applied to BLOCKING rather than SATISFYING contributions: it is keyed on
-- the STABLE workflow_job_id, so a recovered/transferred seat (whose job_id churns)
-- maps back to the same dissent row and the barrier stays blocked. The row is written
-- CO-TRANSACTIONALLY with the verdict insert in the review record/apply path, under
-- the existing per-run advisory lock — so a blocking verdict and its durable witness
-- can never disagree.
--
-- Ownership-safety (D215 / RFC 0079 §5 / the migration owner-table trap): like
-- 0029_fanin_sealed_barrier.sql and 0030_barrier_state.sql, this migration ONLY
-- creates a NEW table (which the runtime role striatumd_rw owns) and NEVER alters or
-- drops an owner-held table, so it applies cleanly as striatumd_rw and cannot
-- crash-loop a two-role production daemon (D187 / #244). The future-runtime-DDL guard
-- (TestFutureRuntimeMigrationsDoNotCarryOwnerDDL, floor 27) forbids ALTER/DROP of
-- striatumd.* outright for versions >= 27.
--
-- NO SQL foreign key to the owner-held jobs table (the FK-to-owner-table trap, D215):
-- a runtime table carrying a real cross-role FK into the owner-held jobs table
-- re-creates the cross-role ownership problem. The seat identity
-- (repository_id, run_id, workflow_job_id, attempt) is carried as BARE COLUMNS;
-- referential integrity is enforced in Go (the live-seal JOIN at evaluation time).
--
-- EXPLICIT GRANT is mandatory: the broad-DML grant in migration 0005 ran before this
-- table existed, and pgtest masks an omitted grant (the default single-role harness
-- runs runtime-migrations-only), so a missing grant passes tests and fails in two-role
-- production. The dissent ledger is APPEND-ONLY: SELECT, INSERT ONLY (UPDATE/DELETE
-- revoked) plus a BEFORE UPDATE OR DELETE refuse-trigger mirroring the freeze table in
-- 0029_fanin_sealed_barrier.sql — a recorded dissent is a structurally immutable,
-- auditable witness that no code path (and no operator) can quietly retract after the
-- fact. A dissent is "cleared" only by a NEW, current-seal accepting verdict that
-- raises the seat's live seal past the dissenting verdict's seal (the barrier reads
-- the live-seal accepting verdict, not a mutation of this row).

-- The dissent ledger: one append-only row per recorded blocking verdict
-- (needs_revision / reject), carrying the SEAL (attempt) it was sealed at as a bare
-- column. A dissent whose seal EQUALS the seat's live seal is a LIVE dissent and
-- blocks the barrier; a dissent at a superseded seal is structurally non-current (the
-- seat advanced past it) and no longer blocks — exactly the seal trap-killer property
-- applied to blocking contributions.
CREATE TABLE IF NOT EXISTS striatumd.dissent_ledger (
  repository_id    text NOT NULL,
  dissent_id       text NOT NULL,
  run_id           text NOT NULL,
  -- The STABLE seat identity (bare columns, no FK to striatumd.jobs per D215). The
  -- quorum keys on workflow_job_id, NEVER job_id (which recovery churns).
  workflow_job_id  text NOT NULL,
  -- The SEAL this dissent was sealed at (the seat's live attempt at record time).
  attempt          integer NOT NULL,
  -- The job_id + session_id + verdict_id that recorded the dissent (provenance /
  -- cross-validation; bare columns, no FK).
  job_id           text NOT NULL,
  session_id       text,
  verdict_id       text,
  -- The blocking verdict value (needs_revision | reject). NEVER an accepting verdict
  -- and NEVER NULL — a dissent row exists only for a recorded blocking verdict. (An
  -- abstention is the verdict-LESS stub, D214a, which is NOT recorded here: it holds a
  -- seat in the quorum denominator, it does not block via the ledger.)
  verdict          text NOT NULL
                     CHECK (verdict IN ('needs_revision', 'reject')),
  recorded_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (repository_id, dissent_id)
);

CREATE INDEX IF NOT EXISTS idx_dissent_ledger_seat
  ON striatumd.dissent_ledger (repository_id, run_id, workflow_job_id, attempt);

-- The append-only refuse-trigger, mirroring fanin_freeze_points in
-- 0029_fanin_sealed_barrier.sql and the events/artifacts triggers in
-- 0005_repo_local_workflow_state.sql. A dissent, once written, can never be updated or
-- deleted — the blocking witness is structurally immutable.
DROP TRIGGER IF EXISTS dissent_ledger_no_update ON striatumd.dissent_ledger;
CREATE TRIGGER dissent_ledger_no_update
BEFORE UPDATE ON striatumd.dissent_ledger
FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_repo_append_only_change();

DROP TRIGGER IF EXISTS dissent_ledger_no_delete ON striatumd.dissent_ledger;
CREATE TRIGGER dissent_ledger_no_delete
BEFORE DELETE ON striatumd.dissent_ledger
FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_repo_append_only_change();

-- Explicit GRANT (mandatory; pgtest masks an omission). Append-only: SELECT, INSERT
-- ONLY (no UPDATE/DELETE), reinforcing the refuse-trigger with a privilege boundary.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT ON striatumd.dissent_ledger TO striatumd_rw;
  END IF;
END
$$;
