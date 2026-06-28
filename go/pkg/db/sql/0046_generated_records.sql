-- RFC 0171 / D273: generated-record blob index.
--
-- This runtime-owned table indexes generated operator/run-shaped bodies whose
-- authoritative bytes live in blob storage while git keeps compact dockets or
-- pointer manifests. It is repository-scoped and deliberately carries run,
-- job, and artifact linkage as bare nullable columns: regular runtime
-- migrations are applied by striatumd_rw, so this migration must not depend on
-- owner-held workflow tables for relational enforcement.
--
-- The row is an index/projection, not workflow state authority. Later RFC 0171
-- slices can add dockets, resolver/materializer behavior, and import proofing
-- against this table without moving generated bodies back into tracked source.

CREATE TABLE IF NOT EXISTS striatumd.generated_records (
  repository_id   text NOT NULL,
  record_id       text NOT NULL,
  source_path     text NOT NULL,
  source_commit   text,
  record_class    text NOT NULL,
  run_id          text,
  job_id          text,
  artifact_id     text,
  content_sha256  text NOT NULL,
  blob_key        text NOT NULL,
  blob_sha256     text NOT NULL,
  content_type    text NOT NULL,
  size_bytes      bigint NOT NULL CHECK (size_bytes >= 0),
  retention_class text NOT NULL,
  bundle_id       text,
  import_batch_id text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  status          text NOT NULL DEFAULT 'indexed' CHECK (status IN (
    'indexed',
    'archived',
    'missing_blob',
    'tombstoned'
  )),
  PRIMARY KEY (repository_id, record_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_generated_records_repo_blob_key
  ON striatumd.generated_records(repository_id, blob_key);

CREATE UNIQUE INDEX IF NOT EXISTS uq_generated_records_source_commit_path
  ON striatumd.generated_records(repository_id, source_commit, source_path)
  WHERE source_commit IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_generated_records_artifact
  ON striatumd.generated_records(repository_id, artifact_id)
  WHERE artifact_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_generated_records_run_job
  ON striatumd.generated_records(repository_id, run_id, job_id)
  WHERE run_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_generated_records_source_path
  ON striatumd.generated_records(repository_id, source_path);

CREATE INDEX IF NOT EXISTS idx_generated_records_content_sha256
  ON striatumd.generated_records(repository_id, content_sha256);

CREATE INDEX IF NOT EXISTS idx_generated_records_status
  ON striatumd.generated_records(repository_id, status, retention_class, record_class);

CREATE INDEX IF NOT EXISTS idx_generated_records_bundle
  ON striatumd.generated_records(repository_id, bundle_id)
  WHERE bundle_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_generated_records_import_batch
  ON striatumd.generated_records(repository_id, import_batch_id)
  WHERE import_batch_id IS NOT NULL;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE ON striatumd.generated_records TO striatumd_rw;
    REVOKE DELETE ON striatumd.generated_records FROM striatumd_rw;
  END IF;
END
$$;
