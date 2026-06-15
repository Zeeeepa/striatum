-- RFC 0127 P0 (D195): opt-in plain-dir job workspaces — the lane as a pure byte producer.
--
-- Phase P0 of retiring the lane git identity adds an OPT-IN `plain_dir` workspace
-- alongside the legacy per-job git worktree. When a job opts in (workspace.create
-- with workspace_kind = plain_dir) the daemon stages the run-branch base content
-- into a plain, .git-free directory under .striatum/workspaces/<id> and records
-- the base tree sha it staged BEFORE the lane starts. That durable "before" pin
-- lets a retrospective reconstruct the exact diff (base tree XOR published tree)
-- without trusting the lane. The legacy git-worktree path (job_worktrees) stays
-- the default and is untouched; this table only ever holds opt-in plain_dir rows,
-- so no in-flight per_job run is affected.
--
-- Ownership-safety (RFC 0079 §5 / D187 / the RFC 0081 owner-table crash-loop): the
-- daemon applies migrations as the runtime role `striatumd_rw`, which cannot alter
-- owner-held tables, and regular migrations >= 27 must carry no owner-table DDL
-- (ALTER/DROP) at all (TestFutureRuntimeMigrationsDoNotCarryOwnerDDL). So this
-- migration ONLY creates a NEW runtime-owned table and declares NO foreign keys
-- (referential integrity to repositories/runs/jobs/leases is enforced in Go in
-- the workspace.create handler, exactly like the interrogations (16) and
-- principals (23) migrations). The GRANT block makes the table usable when the
-- migration is owner-applied (pgtest masks a missing GRANT, so it is mandatory).

CREATE TABLE IF NOT EXISTS striatumd.job_workspaces (
  repository_id text NOT NULL,
  workspace_id text NOT NULL,
  run_id text NOT NULL,
  job_id text NOT NULL,
  lease_id text NOT NULL,
  workspace_kind text NOT NULL CHECK (workspace_kind IN ('plain_dir')),
  base_branch text NOT NULL,
  base_tree_sha text NOT NULL,
  workspace_path text NOT NULL,
  state text NOT NULL CHECK (state IN ('active','released','removed','abandoned')),
  created_at timestamptz NOT NULL,
  released_at timestamptz,
  removed_at timestamptz,
  PRIMARY KEY (repository_id, workspace_id)
);

-- At most one ACTIVE workspace per job (mirrors uq_active_job_worktree).
CREATE UNIQUE INDEX IF NOT EXISTS uq_active_job_workspace
  ON striatumd.job_workspaces(repository_id, job_id)
  WHERE state = 'active';
CREATE INDEX IF NOT EXISTS idx_job_workspaces_run
  ON striatumd.job_workspaces(repository_id, run_id, state);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.job_workspaces TO striatumd_rw;
  END IF;
END
$$;
