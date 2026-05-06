"""SQLite schema for the V1 local state store."""

from __future__ import annotations

SCHEMA_VERSION = "1"

SCHEMA_SQL = """
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS schema_meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_snapshots (
  workflow_snapshot_id TEXT PRIMARY KEY,
  workflow_id TEXT NOT NULL,
  workflow_version TEXT,
  source_path TEXT,
  content_sha256 TEXT NOT NULL,
  workflow_json TEXT NOT NULL,
  loaded_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS runs (
  run_id TEXT PRIMARY KEY,
  workflow_snapshot_id TEXT NOT NULL REFERENCES workflow_snapshots(workflow_snapshot_id),
  repo_root TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN (
    'needs_branch_confirmation','ready','running','blocked',
    'completed','failed','canceled'
  )),
  branch_name TEXT,
  branch_base TEXT,
  branch_confirmed_at TEXT,
  branch_confirmed_by TEXT,
  created_at TEXT NOT NULL,
  started_at TEXT,
  completed_at TEXT,
  stop_reason TEXT
);

CREATE TABLE IF NOT EXISTS sessions (
  session_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  role_id TEXT NOT NULL,
  lane_id TEXT NOT NULL,
  slug TEXT NOT NULL,
  ordinal INTEGER NOT NULL,
  capabilities_json TEXT NOT NULL DEFAULT '[]',
  parent_session_id TEXT REFERENCES sessions(session_id),
  first_class INTEGER NOT NULL DEFAULT 1 CHECK (first_class IN (0,1)),
  fresh_context INTEGER NOT NULL DEFAULT 0 CHECK (fresh_context IN (0,1)),
  state TEXT NOT NULL CHECK (state IN ('active','expired','stopped','lost')),
  registered_at TEXT NOT NULL,
  last_heartbeat_at TEXT,
  expires_at TEXT,
  UNIQUE (run_id, slug),
  UNIQUE (run_id, role_id, lane_id, ordinal)
);

CREATE TABLE IF NOT EXISTS jobs (
  job_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  workflow_job_id TEXT NOT NULL,
  title TEXT NOT NULL,
  job_type TEXT NOT NULL CHECK (job_type IN (
    'draft','review','ledger','synthesis','build','test',
    'human_checkpoint','generic'
  )),
  role_id TEXT NOT NULL,
  lane_selector_json TEXT NOT NULL DEFAULT '{}',
  capability_requirements_json TEXT NOT NULL DEFAULT '[]',
  state TEXT NOT NULL CHECK (state IN (
    'blocked','queued','claimed','running','stale_lease',
    'waiting_human','completed','failed','canceled','skipped'
  )),
  attempt INTEGER NOT NULL DEFAULT 1,
  max_attempts INTEGER NOT NULL DEFAULT 1,
  fresh_session_required INTEGER NOT NULL DEFAULT 0 CHECK (fresh_session_required IN (0,1)),
  write_scope_json TEXT NOT NULL DEFAULT '{}',
  expected_artifacts_json TEXT NOT NULL DEFAULT '[]',
  idempotency_key TEXT NOT NULL,
  created_at TEXT NOT NULL,
  ready_at TEXT,
  started_at TEXT,
  completed_at TEXT,
  current_message_id TEXT,
  current_lease_id TEXT,
  UNIQUE (run_id, workflow_job_id, attempt),
  UNIQUE (run_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS job_dependencies (
  job_id TEXT NOT NULL REFERENCES jobs(job_id),
  depends_on_job_id TEXT NOT NULL REFERENCES jobs(job_id),
  gate_json TEXT NOT NULL DEFAULT '{}',
  PRIMARY KEY (job_id, depends_on_job_id)
);

CREATE TABLE IF NOT EXISTS queue_messages (
  message_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT REFERENCES jobs(job_id),
  kind TEXT NOT NULL CHECK (kind IN (
    'work','coordinator_message','agent_message','blocker',
    'human_checkpoint','commit_request'
  )),
  state TEXT NOT NULL CHECK (state IN (
    'pending','claimed','acked','blocked','completed','released',
    'expired','canceled','dead'
  )),
  priority INTEGER NOT NULL DEFAULT 0,
  target_session_id TEXT REFERENCES sessions(session_id),
  target_role_id TEXT,
  target_lane_id TEXT,
  dedupe_key TEXT,
  payload_json TEXT NOT NULL DEFAULT '{}',
  visible_after TEXT,
  claim_count INTEGER NOT NULL DEFAULT 0,
  max_claims INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  claimed_at TEXT,
  acked_at TEXT,
  completed_at TEXT,
  current_lease_id TEXT
);

CREATE TABLE IF NOT EXISTS leases (
  lease_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  resource_type TEXT NOT NULL CHECK (resource_type IN ('job')),
  resource_id TEXT NOT NULL,
  owner_session_id TEXT NOT NULL REFERENCES sessions(session_id),
  state TEXT NOT NULL CHECK (state IN ('active','released','expired')),
  acquired_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  last_heartbeat_at TEXT,
  released_at TEXT,
  release_reason TEXT
);

CREATE TABLE IF NOT EXISTS work_packets (
  packet_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT NOT NULL REFERENCES jobs(job_id),
  message_id TEXT NOT NULL REFERENCES queue_messages(message_id),
  lease_id TEXT NOT NULL REFERENCES leases(lease_id),
  session_id TEXT NOT NULL REFERENCES sessions(session_id),
  packet_json TEXT NOT NULL,
  packet_sha256 TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE (message_id, lease_id)
);

CREATE TABLE IF NOT EXISTS artifacts (
  artifact_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT REFERENCES jobs(job_id),
  session_id TEXT REFERENCES sessions(session_id),
  logical_name TEXT NOT NULL,
  artifact_kind TEXT NOT NULL CHECK (artifact_kind IN (
    'prompt','finding','findings_ledger','synthesis','marker',
    'handoff','decision','patch_summary','test_report','other'
  )),
  repo_path TEXT NOT NULL,
  content_sha256 TEXT NOT NULL,
  size_bytes INTEGER NOT NULL,
  publish_mode TEXT NOT NULL CHECK (publish_mode IN (
    'create','replace_same_job','append_version'
  )),
  created_at TEXT NOT NULL,
  UNIQUE (run_id, job_id, logical_name),
  UNIQUE (run_id, repo_path, content_sha256)
);

CREATE TABLE IF NOT EXISTS verdicts (
  verdict_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT NOT NULL REFERENCES jobs(job_id),
  session_id TEXT NOT NULL REFERENCES sessions(session_id),
  verdict TEXT NOT NULL CHECK (verdict IN (
    'accept','accept_with_findings','needs_revision','reject'
  )),
  rationale TEXT,
  findings_artifact_id TEXT REFERENCES artifacts(artifact_id),
  created_at TEXT NOT NULL,
  UNIQUE (job_id, session_id)
);

CREATE TABLE IF NOT EXISTS blockers (
  blocker_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT REFERENCES jobs(job_id),
  session_id TEXT REFERENCES sessions(session_id),
  severity TEXT NOT NULL CHECK (severity IN ('info','warning','blocked','human_checkpoint')),
  blocker_kind TEXT NOT NULL,
  description TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('open','resolved','canceled')),
  created_at TEXT NOT NULL,
  resolved_at TEXT
);

CREATE TABLE IF NOT EXISTS command_requests (
  request_id TEXT PRIMARY KEY,
  run_id TEXT REFERENCES runs(run_id),
  session_id TEXT REFERENCES sessions(session_id),
  command_name TEXT NOT NULL,
  payload_sha256 TEXT NOT NULL,
  response_json TEXT,
  state TEXT NOT NULL CHECK (state IN ('started','completed','failed')),
  created_at TEXT NOT NULL,
  completed_at TEXT
);

CREATE TABLE IF NOT EXISTS events (
  event_id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT REFERENCES runs(run_id),
  event_type TEXT NOT NULL,
  actor_session_id TEXT REFERENCES sessions(session_id),
  job_id TEXT REFERENCES jobs(job_id),
  message_id TEXT REFERENCES queue_messages(message_id),
  artifact_id TEXT REFERENCES artifacts(artifact_id),
  lease_id TEXT REFERENCES leases(lease_id),
  payload_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_runs_state ON runs(state);
CREATE INDEX IF NOT EXISTS idx_sessions_run_state ON sessions(run_id, state);
CREATE INDEX IF NOT EXISTS idx_jobs_run_state ON jobs(run_id, state, role_id);
CREATE INDEX IF NOT EXISTS idx_queue_claimable ON queue_messages(
  run_id, state, target_session_id, target_role_id, target_lane_id,
  visible_after, priority, created_at
);
CREATE INDEX IF NOT EXISTS idx_events_run_time ON events(run_id, event_id);
CREATE INDEX IF NOT EXISTS idx_events_job ON events(job_id, event_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_active_resource_lease
  ON leases(resource_type, resource_id)
  WHERE state = 'active';

CREATE UNIQUE INDEX IF NOT EXISTS uq_active_work_message_per_job
  ON queue_messages(job_id)
  WHERE kind = 'work' AND state IN ('pending','claimed','acked');

CREATE TRIGGER IF NOT EXISTS events_no_update
BEFORE UPDATE ON events
BEGIN
  SELECT RAISE(ABORT, 'events are append-only');
END;

CREATE TRIGGER IF NOT EXISTS events_no_delete
BEFORE DELETE ON events
BEGIN
  SELECT RAISE(ABORT, 'events are append-only');
END;

CREATE TRIGGER IF NOT EXISTS artifacts_no_update
BEFORE UPDATE ON artifacts
BEGIN
  SELECT RAISE(ABORT, 'artifact records are append-only');
END;

CREATE TRIGGER IF NOT EXISTS artifacts_no_delete
BEFORE DELETE ON artifacts
BEGIN
  SELECT RAISE(ABORT, 'artifact records are append-only');
END;

INSERT OR REPLACE INTO schema_meta(key, value) VALUES ('schema_version', '1');
"""

