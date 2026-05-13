ALTER TABLE striatumd.rpc_methods
  DROP CONSTRAINT IF EXISTS rpc_methods_required_capability_check;

ALTER TABLE striatumd.rpc_methods
  ADD CONSTRAINT rpc_methods_required_capability_check
  CHECK (required_capability IN ('read','write','review','claim','apply','admin','recovery'));

ALTER TABLE striatumd.rpc_methods
  ADD COLUMN IF NOT EXISTS repository_scope_mode text NOT NULL DEFAULT 'daemon_global'
  CHECK (repository_scope_mode IN ('single_repo','cross_repo','daemon_global'));

UPDATE striatumd.rpc_methods
SET repository_scope_mode = CASE
  WHEN repository_scope THEN 'single_repo'
  ELSE 'daemon_global'
END
WHERE repository_scope_mode = 'daemon_global';

CREATE TABLE IF NOT EXISTS striatumd.cross_repo_runs (
  cross_repo_run_id text PRIMARY KEY,
  workflow_id text NOT NULL,
  workflow_version text,
  workflow_snapshot_hash text NOT NULL,
  primary_repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  state text NOT NULL CHECK (
    state IN ('preparing','prepared','running','blocked','canceling','canceled','completed','failed','aborted')
  ),
  created_at timestamptz NOT NULL,
  started_at timestamptz,
  completed_at timestamptz,
  canceled_at timestamptz,
  reconcile_after timestamptz,
  last_reconcile_error text
);

CREATE TABLE IF NOT EXISTS striatumd.cross_repo_run_repositories (
  cross_repo_run_id text NOT NULL REFERENCES striatumd.cross_repo_runs(cross_repo_run_id),
  repository_alias text NOT NULL,
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  local_run_id text,
  state text NOT NULL CHECK (
    state IN ('preparing','prepared','running','blocked','canceling','canceled','completed','failed','aborted','unavailable')
  ),
  prepared_at timestamptz,
  last_observed_at timestamptz,
  PRIMARY KEY (cross_repo_run_id, repository_alias),
  UNIQUE (cross_repo_run_id, repository_id)
);

CREATE TABLE IF NOT EXISTS striatumd.cross_repo_cycle_counters (
  cross_repo_run_id text NOT NULL REFERENCES striatumd.cross_repo_runs(cross_repo_run_id),
  cycle_key text NOT NULL,
  iterations integer NOT NULL DEFAULT 0 CHECK (iterations >= 0),
  max_iterations integer NOT NULL CHECK (max_iterations >= 1),
  PRIMARY KEY (cross_repo_run_id, cycle_key)
);

CREATE TABLE IF NOT EXISTS striatumd.audit_repositories (
  audit_id bigint NOT NULL REFERENCES striatumd.audit_log(audit_id),
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  PRIMARY KEY (audit_id, repository_id)
);
