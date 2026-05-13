CREATE TABLE IF NOT EXISTS striatumd.rpc_methods (
  method text PRIMARY KEY,
  required_capability text CHECK (
    required_capability IN ('read','write','review','claim','apply','admin')
  ),
  repository_scope boolean NOT NULL,
  params_schema_version integer NOT NULL,
  params_schema_hash text NOT NULL,
  audit_class text NOT NULL,
  min_envelope integer NOT NULL,
  deprecated boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS striatumd.daemon_supervisors (
  daemon_supervisor_id text PRIMARY KEY,
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  run_id text NOT NULL,
  session_id text NOT NULL,
  repo_supervisor_id text NOT NULL,
  daemon_instance_id text NOT NULL,
  adapter text NOT NULL,
  command_json jsonb NOT NULL,
  command_sha256 text NOT NULL,
  cwd text NOT NULL,
  stdin_pipe_path text,
  pid integer,
  pid_start_time text,
  state text NOT NULL CHECK (state IN ('starting','attached','detached','lost','stopped')),
  started_at timestamptz NOT NULL,
  heartbeat_at timestamptz,
  ended_at timestamptz,
  stop_reason text
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_active_daemon_supervisor_session
  ON striatumd.daemon_supervisors(repository_id, session_id)
  WHERE state IN ('starting','attached','detached');

CREATE INDEX IF NOT EXISTS idx_daemon_supervisors_repo_run
  ON striatumd.daemon_supervisors(repository_id, run_id, state);

CREATE TABLE IF NOT EXISTS striatumd.apply_receipts (
  apply_receipt_id text PRIMARY KEY,
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  run_id text NOT NULL,
  job_id text NOT NULL,
  patch_artifact_id text NOT NULL,
  patch_digest_sha256 text NOT NULL,
  base_tree_sha256 text NOT NULL,
  applied_tree_sha256 text,
  signing_key_id text NOT NULL,
  signature text NOT NULL,
  evidence_artifact_path text NOT NULL,
  state text NOT NULL CHECK (state IN ('applied','refused','superseded')),
  denial_reason text,
  created_at timestamptz NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_apply_receipt_patch_digest
  ON striatumd.apply_receipts(repository_id, patch_digest_sha256)
  WHERE state = 'applied';
