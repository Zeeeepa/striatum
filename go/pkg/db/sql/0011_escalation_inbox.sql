CREATE TABLE striatumd.escalation_inbox (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  escalation_id text NOT NULL,
  run_id text NOT NULL,
  job_id text,
  session_id text,
  blocker_id text NOT NULL,
  blocker_kind text NOT NULL,
  severity text NOT NULL,
  state text NOT NULL CHECK (state IN ('pending', 'viewed', 'resolved')),
  created_at timestamptz NOT NULL,
  viewed_at timestamptz,
  resolved_at timestamptz,
  decision_artifact_id text,
  resolution_note text,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY (repository_id, escalation_id),
  FOREIGN KEY (repository_id, blocker_id) REFERENCES striatumd.blockers(repository_id, blocker_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id),
  FOREIGN KEY (repository_id, job_id) REFERENCES striatumd.jobs(repository_id, job_id),
  FOREIGN KEY (repository_id, session_id) REFERENCES striatumd.sessions(repository_id, session_id),
  FOREIGN KEY (repository_id, decision_artifact_id) REFERENCES striatumd.artifacts(repository_id, artifact_id)
);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE ON striatumd.escalation_inbox TO striatumd_rw;
  END IF;
END;
$$;
