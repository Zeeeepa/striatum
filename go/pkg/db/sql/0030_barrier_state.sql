-- RFC 0135 P2 (D215/D216) — the recoverable barrier-assembly state table. The
-- deferred post-completion join (RFC 0133 Slice 3) graduates from the P1 opt-in
-- plumbing into a first-class, recoverable `barrier_assembly` job. That job's
-- progress is journaled in this table so a crash mid-assembly can recognize its
-- own intent and resume — never wedge, never double-commit.
--
-- Two-store consistency (inherited from RFC 0133, Risks): the PG state transition
-- and the git CAS that advances the run branch cannot share one transaction. This
-- table is the two-phase journal: the assembler writes the INTENT
-- (target_commit_sha + tree_sha, state='assembling') to PG BEFORE the git CAS,
-- then flips to 'committed' after the CAS lands. A crashed assembler that re-runs
-- reads the journaled intent and either recognizes its own already-applied commit
-- (the CAS is idempotent) or re-applies it — so the assembly is exactly-once at
-- the run-branch tip.
--
-- Ownership-safety (D215 / RFC 0079 §5 / the migration owner-table trap): like
-- 0023_principals.sql, 0027_spawn_authorization_grants.sql and the P1 migration
-- 0029, this migration ONLY creates a NEW table (which the runtime role
-- striatumd_rw owns) and NEVER alters or drops an owner-held table, so it applies
-- cleanly as striatumd_rw and cannot crash-loop a two-role production daemon
-- (D187 / #244). The `barrier_assembly` job_type CHECK widening — the part that
-- touches the owner-held jobs CHECK — ships as OWNER BUNDLE 0013, NOT in this
-- runtime migration, and the runtime guard TestFutureRuntimeMigrationsDoNotCarry-
-- OwnerDDL keeps it that way.
--
-- NO cross-role SQL referential key into the owner-held jobs table (the FK-to-
-- owner-table trap, D215): a runtime table carrying a real cross-role key into the
-- owner-held jobs table re-creates the cross-role ownership problem. The barrier /
-- seat identity (repository_id, run_id, barrier_id, downstream_workflow_job_id)
-- is carried as BARE COLUMNS; referential integrity is enforced in Go (the Go
-- assembler resolves the freeze record and the live-seal JOIN at assembly time,
-- not a stored key).
--
-- EXPLICIT GRANT is mandatory: the broad-DML grant in migration 0005 ran before
-- this table existed, and pgtest masks an omitted grant (the default single-role
-- harness runs runtime-migrations-only), so a missing grant passes tests and fails
-- in two-role production. The state table carries the full DML surface (the
-- assembler INSERTs the journal, UPDATEs the state through sealed -> assembling ->
-- committed|failed), guarded by IF EXISTS (pg_roles ... striatumd_rw).

-- The barrier-assembly journal: one row per barrier-assembly job. The state walks
-- sealed -> assembling -> committed | failed. The two-phase journal columns
-- (target_commit_sha + tree_sha) record the assembler's INTENT before the git CAS
-- so a crashed assembler can recognize its own commit (or compare to the journaled
-- intent) and resume idempotently.
CREATE TABLE IF NOT EXISTS striatumd.barrier_state (
  repository_id     text NOT NULL,
  barrier_id        text NOT NULL,
  run_id            text NOT NULL,
  -- The downstream job whose in-edges are the fan-in siblings (bare column, no
  -- cross-role key into the owner-held jobs table per D215).
  downstream_workflow_job_id text NOT NULL,
  -- The assembly job's stable workflow_job_id (the `barrier_assembly` seat). Bare
  -- column; integrity is enforced in Go (the assembler is the job's own handler).
  assembly_workflow_job_id text,
  -- The assembly job's job_id (the concrete attempt). Bare column.
  assembly_job_id   text,
  -- Lifecycle: 'sealed' once the barrier has fired and the assembly job is
  -- scheduled; 'assembling' once the assembler has journaled its target intent and
  -- is about to (or has just) run the git CAS; 'committed' once the run branch has
  -- been advanced to target_commit_sha; 'failed' on an unrecoverable assembly
  -- error (a real content conflict / corruption — surfaced, never silently retried
  -- forever).
  state             text NOT NULL DEFAULT 'sealed'
                      CHECK (state IN ('sealed','assembling','committed','failed')),
  -- The two-phase journal (RFC 0133 / RFC 0135 Risks): the assembler writes these
  -- with state='assembling' BEFORE the git CAS. target_commit_sha is the assembled
  -- commit the run branch will be CAS-advanced to; tree_sha is its tree (the
  -- assembled tree the equivalence fixture compares against D206). On resume the
  -- assembler recognizes target_commit_sha as its own intent.
  target_commit_sha text,
  tree_sha          text,
  -- The frozen run-branch tip the assembly folded onto (the assembly's expected
  -- base for the CAS), recorded for crash-recovery legibility. Bare column.
  base_commit_sha   text,
  -- A terminal failure reason for state='failed' (legible surfacing, not a retry
  -- loop).
  fail_reason       text,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  committed_at      timestamptz,
  PRIMARY KEY (repository_id, barrier_id)
);

CREATE INDEX IF NOT EXISTS idx_barrier_state_run
  ON striatumd.barrier_state (repository_id, run_id);

-- Explicit GRANT (mandatory; pgtest masks an omission). The state table carries
-- the full DML surface: the assembler INSERTs the journal and UPDATEs the state
-- through its lifecycle. Guarded by an IF EXISTS probe so the single-role pgtest
-- harness (where striatumd_rw is absent) is a clean no-op.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.barrier_state TO striatumd_rw;
  END IF;
END
$$;
