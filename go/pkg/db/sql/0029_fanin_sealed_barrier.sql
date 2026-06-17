-- RFC 0135 P1 (D216) — the first LIVE instance of the sealed expectation barrier:
-- fan-in with entity=job, seal=attempt (#345). This migration creates the two
-- runtime-owned tables the live-seal JOIN barrier reads: the immutable freeze
-- record (the frozen run-branch tip + the declared sibling seats) and the
-- attempt-addressed staging-contribution table.
--
-- Ownership-safety (D215 / RFC 0079 §5 / the migration owner-table trap): like
-- 0023_principals.sql and 0027_spawn_authorization_grants.sql, this migration ONLY
-- creates NEW tables (which the runtime role striatumd_rw owns) and NEVER alters or
-- drops an owner-held table, so it applies cleanly as striatumd_rw and cannot
-- crash-loop a two-role production daemon (D187 / #244).
--
-- The `barrier_assembly` job_type CHECK widening (P2) is the part that touches the
-- owner-held jobs CHECK; per D215 it ships as OWNER BUNDLE 0013/0014 and is NOT in
-- this runtime migration — P1 evaluates the barrier and emits the manifest without
-- a new job_type.
--
-- NO SQL foreign key to the owner-held jobs table (the FK-to-owner-table trap,
-- D215): a runtime table carrying a real cross-role FK into the owner-held jobs
-- table re-creates the cross-role ownership problem. The seat identity
-- (repository_id, run_id, workflow_job_id, attempt) is carried as BARE COLUMNS;
-- referential integrity is enforced in Go (the barrier evaluator resolves each
-- in-edge's live attempt by JOINing staged contributions against striatumd.jobs at
-- evaluation time — the live-seal JOIN, not a stored FK).
--
-- EXPLICIT GRANT per table is mandatory: the broad-DML grant in migration 0005 ran
-- before these tables existed, and pgtest masks an omitted grant (the default
-- single-role harness runs runtime-migrations-only), so a missing grant passes
-- tests and fails in two-role production. The freeze table grants SELECT, INSERT
-- ONLY (append-only) plus a BEFORE UPDATE OR DELETE refuse-trigger mirroring the
-- events/artifacts append-only triggers in 0005_repo_local_workflow_state.sql — the
-- frozen base is structurally immutable and auditable. The staging table carries
-- the full DML surface (a requeue tombstones a stale staging row, RFC 0133).

-- The immutable freeze record: written ONCE at fan-out, recording the frozen
-- run-branch tip the assembly folds onto and the declared set of sibling seats the
-- barrier waits on. Append-only: no UPDATE/DELETE grant + a refuse-trigger.
CREATE TABLE IF NOT EXISTS striatumd.fanin_freeze_points (
  repository_id           text NOT NULL,
  barrier_id              text NOT NULL,
  run_id                  text NOT NULL,
  -- The downstream job whose in-edges are the fan-in siblings (bare column, no FK
  -- to striatumd.jobs per D215).
  downstream_workflow_job_id text NOT NULL,
  -- The frozen run-branch tip commit the assembly folds onto (the merge-base
  -- contamination check asserts each staged ref descends from this).
  frozen_tip_sha          text NOT NULL,
  -- The frozen tip's tree sha, retained so a future per-commit tree-provenance
  -- check (RFC 0133 Risks child-idea) can close the smuggled-content hole.
  frozen_tip_tree_sha     text,
  -- The declared sibling seats this barrier waits on, as a JSON array of
  -- workflow_job_id strings. These are the in-edges; the live attempt of each is
  -- resolved at evaluation time against striatumd.jobs (the live-seal JOIN).
  declared_sibling_job_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at              timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (repository_id, barrier_id)
);

CREATE INDEX IF NOT EXISTS idx_fanin_freeze_points_run
  ON striatumd.fanin_freeze_points (repository_id, run_id);

-- The attempt-addressed staging contributions: one row per sibling completion,
-- carrying the SEAL (attempt) it was sealed at embedded as a bare column. The
-- staging write is co-transactional with the attempt/state update (RFC 0133). The
-- staging ref refs/striatum/staged/<run>/<job>/<attempt> is the git witness; this
-- row is the authoritative PG record the barrier JOINs against the live attempt.
--
-- A requeue/resume/complete-stalled bumps striatumd.jobs.attempt to a strictly-new
-- value (the UNIQUE (repository_id, run_id, workflow_job_id, attempt) key makes the
-- prior attempt a distinct historical row), so a stale-attempt staging row no
-- longer satisfies staged.attempt = jobs.attempt and is STRUCTURALLY invisible to
-- the barrier (RFC 0133 synthesis trap #1 kill, generalized to seal in RFC 0135).
CREATE TABLE IF NOT EXISTS striatumd.barrier_staged_contributions (
  repository_id     text NOT NULL,
  barrier_id        text NOT NULL,
  run_id            text NOT NULL,
  -- The stable seat identity (bare columns, no FK to striatumd.jobs per D215).
  workflow_job_id   text NOT NULL,
  -- The SEAL this contribution was sealed at (attempt, for the fan-in instance).
  attempt           integer NOT NULL,
  -- The job_id of the completing attempt (provenance / cross-validation; bare).
  job_id            text NOT NULL,
  -- The staged git ref and the commit it points at.
  staging_ref       text NOT NULL,
  commit_sha        text NOT NULL,
  -- Lifecycle: 'staged' is a live contribution; 'voided' is a requeue tombstone
  -- (RFC 0133) recording WHY a prior attempt's contribution was excluded. A voided
  -- row is never read by the barrier even if its attempt later collides.
  status            text NOT NULL DEFAULT 'staged'
                      CHECK (status IN ('staged', 'voided')),
  staged_at         timestamptz NOT NULL DEFAULT now(),
  voided_at         timestamptz,
  void_reason       text,
  PRIMARY KEY (repository_id, barrier_id, workflow_job_id, attempt)
);

CREATE INDEX IF NOT EXISTS idx_barrier_staged_contributions_run
  ON striatumd.barrier_staged_contributions (repository_id, run_id, barrier_id);

-- The append-only refuse-trigger for the freeze record, mirroring the
-- events/artifacts triggers in 0005_repo_local_workflow_state.sql. A freeze point,
-- once written, can never be updated or deleted — the base the assembly folds onto
-- is structurally immutable, so no code path (and no operator) can quietly re-point
-- the barrier's base after siblings have staged against it.
DROP TRIGGER IF EXISTS fanin_freeze_points_no_update ON striatumd.fanin_freeze_points;
CREATE TRIGGER fanin_freeze_points_no_update
BEFORE UPDATE ON striatumd.fanin_freeze_points
FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_repo_append_only_change();

DROP TRIGGER IF EXISTS fanin_freeze_points_no_delete ON striatumd.fanin_freeze_points;
CREATE TRIGGER fanin_freeze_points_no_delete
BEFORE DELETE ON striatumd.fanin_freeze_points
FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_repo_append_only_change();

-- Explicit GRANT per table (mandatory; pgtest masks an omission). The freeze table
-- is append-only: SELECT, INSERT ONLY (no UPDATE/DELETE), reinforcing the trigger
-- with a privilege boundary. The staging table carries full DML (a requeue
-- tombstones a stale row via UPDATE).
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT ON striatumd.fanin_freeze_points TO striatumd_rw;
    GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.barrier_staged_contributions TO striatumd_rw;
  END IF;
END
$$;
