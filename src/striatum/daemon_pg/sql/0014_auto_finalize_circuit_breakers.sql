CREATE TABLE IF NOT EXISTS striatumd.auto_finalize_circuit_breakers (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id) ON DELETE CASCADE,
  run_id text NOT NULL,
  workflow_job_id text NOT NULL,
  cause text NOT NULL,
  consecutive_failures integer NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
  opened_at timestamptz,
  open_until timestamptz,
  last_failure_at timestamptz,
  last_error text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (repository_id, run_id, workflow_job_id, cause),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id)
);

CREATE INDEX IF NOT EXISTS auto_finalize_circuit_breakers_open_idx
  ON striatumd.auto_finalize_circuit_breakers (repository_id, run_id, open_until)
  WHERE open_until IS NOT NULL;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE
      ON striatumd.auto_finalize_circuit_breakers TO striatumd_rw;
  END IF;
END;
$$;
