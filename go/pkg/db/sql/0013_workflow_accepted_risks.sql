CREATE TABLE IF NOT EXISTS striatumd.workflow_accepted_risks (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  accepted_risk_id text NOT NULL,
  workflow_snapshot_id text,
  workflow_fingerprint_sha256 text,
  workflow_id text,
  workflow_version text,
  source_path text,
  lint_rule text NOT NULL,
  finding_fingerprint_sha256 text NOT NULL,
  finding_json jsonb NOT NULL,
  decision_artifact_ref text NOT NULL,
  accepted_by text NOT NULL,
  accepted_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz,
  rationale text NOT NULL,
  PRIMARY KEY (repository_id, accepted_risk_id),
  FOREIGN KEY (repository_id, workflow_snapshot_id)
    REFERENCES striatumd.workflow_snapshots(repository_id, workflow_snapshot_id),
  CHECK (
    (workflow_snapshot_id IS NOT NULL AND workflow_fingerprint_sha256 IS NULL)
    OR
    (workflow_snapshot_id IS NULL AND workflow_fingerprint_sha256 IS NOT NULL)
  ),
  CHECK (length(trim(lint_rule)) > 0),
  CHECK (length(trim(finding_fingerprint_sha256)) > 0),
  CHECK (length(trim(decision_artifact_ref)) > 0),
  CHECK (length(trim(accepted_by)) > 0),
  CHECK (length(trim(rationale)) > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_workflow_accepted_risk_snapshot
  ON striatumd.workflow_accepted_risks(
    repository_id,
    workflow_snapshot_id,
    lint_rule,
    finding_fingerprint_sha256
  )
  WHERE workflow_snapshot_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_workflow_accepted_risk_fingerprint
  ON striatumd.workflow_accepted_risks(
    repository_id,
    workflow_fingerprint_sha256,
    lint_rule,
    finding_fingerprint_sha256
  )
  WHERE workflow_snapshot_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_workflow_accepted_risks_snapshot
  ON striatumd.workflow_accepted_risks(repository_id, workflow_snapshot_id);

CREATE INDEX IF NOT EXISTS idx_workflow_accepted_risks_fingerprint
  ON striatumd.workflow_accepted_risks(repository_id, workflow_fingerprint_sha256);

DROP TRIGGER IF EXISTS workflow_accepted_risks_no_update
  ON striatumd.workflow_accepted_risks;
CREATE TRIGGER workflow_accepted_risks_no_update
BEFORE UPDATE ON striatumd.workflow_accepted_risks
FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_repo_append_only_change();

DROP TRIGGER IF EXISTS workflow_accepted_risks_no_delete
  ON striatumd.workflow_accepted_risks;
CREATE TRIGGER workflow_accepted_risks_no_delete
BEFORE DELETE ON striatumd.workflow_accepted_risks
FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_repo_append_only_change();

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT ON striatumd.workflow_accepted_risks TO striatumd_rw;
    REVOKE UPDATE, DELETE ON striatumd.workflow_accepted_risks FROM striatumd_rw;
  END IF;
END;
$$;
