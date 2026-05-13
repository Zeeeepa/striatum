CREATE TABLE IF NOT EXISTS striatumd.workflow_snapshots (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  workflow_snapshot_id text NOT NULL,
  workflow_id text NOT NULL,
  workflow_version text,
  source_path text,
  content_sha256 text NOT NULL,
  workflow_json jsonb NOT NULL,
  loaded_at timestamptz NOT NULL,
  PRIMARY KEY (repository_id, workflow_snapshot_id)
);

CREATE TABLE IF NOT EXISTS striatumd.runs (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  run_id text NOT NULL,
  workflow_snapshot_id text NOT NULL,
  repo_root text NOT NULL,
  state text NOT NULL CHECK (state IN (
    'needs_branch_confirmation','ready','running','blocked',
    'completed','failed','canceled'
  )),
  branch_name text,
  branch_base text,
  branch_confirmed_at timestamptz,
  branch_confirmed_by text,
  created_at timestamptz NOT NULL,
  started_at timestamptz,
  completed_at timestamptz,
  stop_reason text,
  paused_at timestamptz,
  paused_reason text,
  cross_repo_run_id text,
  PRIMARY KEY (repository_id, run_id),
  FOREIGN KEY (repository_id, workflow_snapshot_id)
    REFERENCES striatumd.workflow_snapshots(repository_id, workflow_snapshot_id)
);

CREATE INDEX IF NOT EXISTS idx_runs_state
  ON striatumd.runs(repository_id, state);
CREATE INDEX IF NOT EXISTS idx_runs_cross_repo_run_id
  ON striatumd.runs(repository_id, cross_repo_run_id)
  WHERE cross_repo_run_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS striatumd.sessions (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  session_id text NOT NULL,
  run_id text NOT NULL,
  role_id text NOT NULL,
  lane_id text NOT NULL,
  slug text NOT NULL,
  ordinal integer NOT NULL,
  capabilities_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  parent_session_id text,
  first_class boolean NOT NULL DEFAULT true,
  fresh_context boolean NOT NULL DEFAULT false,
  state text NOT NULL CHECK (state IN ('active','expired','stopped','lost','closed')),
  registered_at timestamptz NOT NULL,
  last_heartbeat_at timestamptz,
  expires_at timestamptz,
  non_fresh_reason text,
  closed_at timestamptz,
  close_reason text,
  operator_label text,
  PRIMARY KEY (repository_id, session_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, parent_session_id)
    REFERENCES striatumd.sessions(repository_id, session_id),
  UNIQUE (repository_id, run_id, slug),
  UNIQUE (repository_id, run_id, role_id, lane_id, ordinal)
);

CREATE INDEX IF NOT EXISTS idx_sessions_run_state
  ON striatumd.sessions(repository_id, run_id, state);

CREATE TABLE IF NOT EXISTS striatumd.jobs (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  job_id text NOT NULL,
  run_id text NOT NULL,
  workflow_job_id text NOT NULL,
  title text NOT NULL,
  job_type text NOT NULL CHECK (job_type IN (
    'draft','review','ledger','synthesis','build','test',
    'human_checkpoint','generic'
  )),
  role_id text NOT NULL,
  lane_selector_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  capability_requirements_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  state text NOT NULL CHECK (state IN (
    'blocked','queued','claimed','running','stale_lease',
    'waiting_human','completed','failed','canceled','skipped'
  )),
  attempt integer NOT NULL DEFAULT 1,
  max_attempts integer NOT NULL DEFAULT 1,
  fresh_session_required boolean NOT NULL DEFAULT false,
  write_scope_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  expected_artifacts_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL,
  ready_at timestamptz,
  started_at timestamptz,
  completed_at timestamptz,
  current_message_id text,
  current_lease_id text,
  PRIMARY KEY (repository_id, job_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  UNIQUE (repository_id, run_id, workflow_job_id, attempt),
  UNIQUE (repository_id, run_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_jobs_run_state
  ON striatumd.jobs(repository_id, run_id, state, role_id);

CREATE TABLE IF NOT EXISTS striatumd.job_dependencies (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  job_id text NOT NULL,
  depends_on_job_id text NOT NULL,
  gate_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY (repository_id, job_id, depends_on_job_id),
  FOREIGN KEY (repository_id, job_id) REFERENCES striatumd.jobs(repository_id, job_id),
  FOREIGN KEY (repository_id, depends_on_job_id) REFERENCES striatumd.jobs(repository_id, job_id)
);

CREATE TABLE IF NOT EXISTS striatumd.queue_messages (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  message_id text NOT NULL,
  run_id text NOT NULL,
  job_id text,
  kind text NOT NULL CHECK (kind IN (
    'work','coordinator_message','agent_message','blocker',
    'human_checkpoint','commit_request'
  )),
  state text NOT NULL CHECK (state IN (
    'pending','claimed','acked','blocked','completed','released',
    'expired','canceled','dead'
  )),
  priority integer NOT NULL DEFAULT 0,
  target_session_id text,
  target_role_id text,
  target_lane_id text,
  dedupe_key text,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  visible_after timestamptz,
  claim_count integer NOT NULL DEFAULT 0,
  max_claims integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  claimed_at timestamptz,
  acked_at timestamptz,
  completed_at timestamptz,
  current_lease_id text,
  PRIMARY KEY (repository_id, message_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, job_id) REFERENCES striatumd.jobs(repository_id, job_id),
  FOREIGN KEY (repository_id, target_session_id) REFERENCES striatumd.sessions(repository_id, session_id)
);

CREATE INDEX IF NOT EXISTS idx_queue_claimable
  ON striatumd.queue_messages(
    repository_id, run_id, state, target_session_id, target_role_id,
    target_lane_id, visible_after, priority, created_at
  );
CREATE UNIQUE INDEX IF NOT EXISTS uq_active_work_message_per_job
  ON striatumd.queue_messages(repository_id, job_id)
  WHERE kind = 'work' AND state IN ('pending','claimed','acked');

CREATE TABLE IF NOT EXISTS striatumd.leases (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  lease_id text NOT NULL,
  run_id text NOT NULL,
  resource_type text NOT NULL CHECK (resource_type IN ('job')),
  resource_id text NOT NULL,
  owner_session_id text NOT NULL,
  state text NOT NULL CHECK (state IN ('active','released','expired')),
  acquired_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  last_heartbeat_at timestamptz,
  released_at timestamptz,
  release_reason text,
  PRIMARY KEY (repository_id, lease_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, owner_session_id) REFERENCES striatumd.sessions(repository_id, session_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_active_resource_lease
  ON striatumd.leases(repository_id, resource_type, resource_id)
  WHERE state = 'active';

CREATE TABLE IF NOT EXISTS striatumd.work_packets (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  packet_id text NOT NULL,
  run_id text NOT NULL,
  job_id text NOT NULL,
  message_id text NOT NULL,
  lease_id text NOT NULL,
  session_id text NOT NULL,
  packet_json jsonb NOT NULL,
  packet_sha256 text NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (repository_id, packet_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, job_id) REFERENCES striatumd.jobs(repository_id, job_id),
  FOREIGN KEY (repository_id, message_id) REFERENCES striatumd.queue_messages(repository_id, message_id),
  FOREIGN KEY (repository_id, lease_id) REFERENCES striatumd.leases(repository_id, lease_id),
  FOREIGN KEY (repository_id, session_id) REFERENCES striatumd.sessions(repository_id, session_id),
  UNIQUE (repository_id, message_id, lease_id)
);

CREATE INDEX IF NOT EXISTS idx_work_packets_run_session
  ON striatumd.work_packets(repository_id, run_id, session_id);

CREATE TABLE IF NOT EXISTS striatumd.artifacts (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  artifact_id text NOT NULL,
  run_id text NOT NULL,
  job_id text,
  session_id text,
  logical_name text NOT NULL,
  artifact_kind text NOT NULL,
  repo_path text NOT NULL,
  content_sha256 text NOT NULL,
  size_bytes bigint NOT NULL,
  publish_mode text NOT NULL CHECK (publish_mode IN (
    'create','replace_same_job','append_version'
  )),
  created_at timestamptz NOT NULL,
  author_line text,
  PRIMARY KEY (repository_id, artifact_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, job_id) REFERENCES striatumd.jobs(repository_id, job_id),
  FOREIGN KEY (repository_id, session_id) REFERENCES striatumd.sessions(repository_id, session_id),
  UNIQUE (repository_id, run_id, job_id, logical_name),
  UNIQUE (repository_id, run_id, repo_path, content_sha256)
);

CREATE TABLE IF NOT EXISTS striatumd.verdicts (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  verdict_id text NOT NULL,
  run_id text NOT NULL,
  job_id text NOT NULL,
  session_id text NOT NULL,
  verdict text NOT NULL CHECK (verdict IN (
    'accept','accept_with_findings','needs_revision','reject'
  )),
  rationale text,
  findings_artifact_id text,
  created_at timestamptz NOT NULL,
  posture text NOT NULL DEFAULT 'neutral',
  PRIMARY KEY (repository_id, verdict_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, job_id) REFERENCES striatumd.jobs(repository_id, job_id),
  FOREIGN KEY (repository_id, session_id) REFERENCES striatumd.sessions(repository_id, session_id),
  FOREIGN KEY (repository_id, findings_artifact_id) REFERENCES striatumd.artifacts(repository_id, artifact_id),
  UNIQUE (repository_id, job_id, session_id)
);

CREATE INDEX IF NOT EXISTS idx_verdicts_posture
  ON striatumd.verdicts(repository_id, posture);

CREATE TABLE IF NOT EXISTS striatumd.blockers (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  blocker_id text NOT NULL,
  run_id text NOT NULL,
  job_id text,
  session_id text,
  severity text NOT NULL CHECK (severity IN ('info','warning','blocked','human_checkpoint')),
  blocker_kind text NOT NULL,
  description text NOT NULL,
  state text NOT NULL CHECK (state IN ('open','resolved','canceled')),
  created_at timestamptz NOT NULL,
  resolved_at timestamptz,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY (repository_id, blocker_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, job_id) REFERENCES striatumd.jobs(repository_id, job_id),
  FOREIGN KEY (repository_id, session_id) REFERENCES striatumd.sessions(repository_id, session_id)
);

CREATE TABLE IF NOT EXISTS striatumd.command_requests (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  request_id text NOT NULL,
  run_id text,
  session_id text,
  command_name text NOT NULL,
  payload_sha256 text NOT NULL,
  response_json jsonb,
  state text NOT NULL CHECK (state IN ('started','completed','failed')),
  created_at timestamptz NOT NULL,
  completed_at timestamptz,
  PRIMARY KEY (repository_id, request_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, session_id) REFERENCES striatumd.sessions(repository_id, session_id)
);

CREATE TABLE IF NOT EXISTS striatumd.process_executions (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  process_id text NOT NULL,
  run_id text NOT NULL,
  job_id text NOT NULL,
  session_id text NOT NULL,
  lease_id text NOT NULL,
  packet_id text NOT NULL,
  adapter text NOT NULL,
  command_json jsonb NOT NULL,
  cwd text NOT NULL,
  scratch_path text NOT NULL,
  stdin_mode text NOT NULL CHECK (stdin_mode IN ('packet','none')),
  stdio_mode text NOT NULL CHECK (stdio_mode IN ('suppressed','inherit')),
  pid integer,
  state text NOT NULL CHECK (state IN ('starting','running','exited','failed','timed_out','lost')),
  exit_code integer,
  started_at timestamptz NOT NULL,
  ended_at timestamptz,
  PRIMARY KEY (repository_id, process_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, job_id) REFERENCES striatumd.jobs(repository_id, job_id),
  FOREIGN KEY (repository_id, session_id) REFERENCES striatumd.sessions(repository_id, session_id),
  FOREIGN KEY (repository_id, lease_id) REFERENCES striatumd.leases(repository_id, lease_id),
  FOREIGN KEY (repository_id, packet_id) REFERENCES striatumd.work_packets(repository_id, packet_id)
);

CREATE INDEX IF NOT EXISTS idx_process_executions_run_job
  ON striatumd.process_executions(repository_id, run_id, job_id, started_at);

CREATE TABLE IF NOT EXISTS striatumd.events (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  event_id bigint GENERATED BY DEFAULT AS IDENTITY,
  run_id text,
  event_type text NOT NULL,
  actor_session_id text,
  job_id text,
  message_id text,
  artifact_id text,
  lease_id text,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (repository_id, event_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, actor_session_id) REFERENCES striatumd.sessions(repository_id, session_id),
  FOREIGN KEY (repository_id, job_id) REFERENCES striatumd.jobs(repository_id, job_id),
  FOREIGN KEY (repository_id, message_id) REFERENCES striatumd.queue_messages(repository_id, message_id),
  FOREIGN KEY (repository_id, artifact_id) REFERENCES striatumd.artifacts(repository_id, artifact_id),
  FOREIGN KEY (repository_id, lease_id) REFERENCES striatumd.leases(repository_id, lease_id)
);

CREATE INDEX IF NOT EXISTS idx_events_run_time
  ON striatumd.events(repository_id, run_id, event_id);
CREATE INDEX IF NOT EXISTS idx_events_job
  ON striatumd.events(repository_id, job_id, event_id);

CREATE TABLE IF NOT EXISTS striatumd.job_worktrees (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  worktree_id text NOT NULL,
  run_id text NOT NULL,
  job_id text NOT NULL,
  lease_id text NOT NULL,
  base_branch text NOT NULL,
  worktree_path text NOT NULL,
  state text NOT NULL CHECK (state IN ('active','released','removed','abandoned')),
  created_at timestamptz NOT NULL,
  released_at timestamptz,
  removed_at timestamptz,
  PRIMARY KEY (repository_id, worktree_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, job_id) REFERENCES striatumd.jobs(repository_id, job_id),
  FOREIGN KEY (repository_id, lease_id) REFERENCES striatumd.leases(repository_id, lease_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_active_job_worktree
  ON striatumd.job_worktrees(repository_id, job_id)
  WHERE state = 'active';
CREATE INDEX IF NOT EXISTS idx_job_worktrees_run
  ON striatumd.job_worktrees(repository_id, run_id, state);

CREATE TABLE IF NOT EXISTS striatumd.process_supervisors (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  supervisor_id text NOT NULL,
  run_id text NOT NULL,
  session_id text NOT NULL,
  adapter text NOT NULL,
  command_json jsonb NOT NULL,
  cwd text NOT NULL,
  scratch_path text NOT NULL,
  stdin_pipe_path text,
  pid integer,
  state text NOT NULL CHECK (state IN ('starting','attached','detached','lost','stopped')),
  started_at timestamptz NOT NULL,
  heartbeat_at timestamptz,
  ended_at timestamptz,
  stop_reason text,
  pid_start_time text,
  PRIMARY KEY (repository_id, supervisor_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, session_id) REFERENCES striatumd.sessions(repository_id, session_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_active_supervisor_per_session
  ON striatumd.process_supervisors(repository_id, session_id)
  WHERE state IN ('starting','attached','detached');
CREATE INDEX IF NOT EXISTS idx_process_supervisors_run
  ON striatumd.process_supervisors(repository_id, run_id, state);

CREATE TABLE IF NOT EXISTS striatumd.process_supervisor_pointers (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  supervisor_id text NOT NULL,
  daemon_supervisor_id text NOT NULL,
  run_id text NOT NULL,
  session_id text NOT NULL,
  pid integer,
  pid_start_time text,
  state text NOT NULL CHECK (state IN ('starting','attached','detached','lost','stopped')),
  updated_at timestamptz NOT NULL,
  metadata_json jsonb NOT NULL,
  PRIMARY KEY (repository_id, supervisor_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, session_id) REFERENCES striatumd.sessions(repository_id, session_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_active_daemon_supervisor_pointer_per_session
  ON striatumd.process_supervisor_pointers(repository_id, session_id)
  WHERE state IN ('starting','attached','detached');
CREATE INDEX IF NOT EXISTS idx_process_supervisor_pointers_run
  ON striatumd.process_supervisor_pointers(repository_id, run_id, state);

CREATE TABLE IF NOT EXISTS striatumd.repo_migrations (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  source_substrate text NOT NULL CHECK (source_substrate = 'sqlite'),
  target_substrate text NOT NULL CHECK (target_substrate = 'postgres'),
  source_user_version integer NOT NULL,
  source_event_manifest_sha256 text NOT NULL,
  source_artifact_manifest_sha256 text NOT NULL,
  source_state_db_sha256 text NOT NULL,
  migrated_at timestamptz NOT NULL DEFAULT now(),
  tombstone_path text,
  row_counts jsonb NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY (repository_id, source_substrate, target_substrate)
);

CREATE OR REPLACE FUNCTION striatumd.refuse_repo_append_only_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'repo-local append-only rows cannot be updated or deleted';
END;
$$;

DROP TRIGGER IF EXISTS events_no_update ON striatumd.events;
CREATE TRIGGER events_no_update
BEFORE UPDATE ON striatumd.events
FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_repo_append_only_change();

DROP TRIGGER IF EXISTS events_no_delete ON striatumd.events;
CREATE TRIGGER events_no_delete
BEFORE DELETE ON striatumd.events
FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_repo_append_only_change();

DROP TRIGGER IF EXISTS artifacts_no_update ON striatumd.artifacts;
CREATE TRIGGER artifacts_no_update
BEFORE UPDATE ON striatumd.artifacts
FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_repo_append_only_change();

DROP TRIGGER IF EXISTS artifacts_no_delete ON striatumd.artifacts;
CREATE TRIGGER artifacts_no_delete
BEFORE DELETE ON striatumd.artifacts
FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_repo_append_only_change();

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA striatumd TO striatumd_rw;
    REVOKE UPDATE, DELETE ON striatumd.events FROM striatumd_rw;
    REVOKE UPDATE, DELETE ON striatumd.artifacts FROM striatumd_rw;
  END IF;
END;
$$;
