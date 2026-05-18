-- RFC 0072: Blob-backed artifact storage.
--
-- Adds per-artifact blob reference columns and per-repository bucket
-- registration. The artifact body lives in S3-compatible storage at
-- `blob_key`; the daemon-owned PG row keeps the authoritative
-- reference plus the content sha256 anchor for integrity verification
-- on every read. Each registered repository gets its own bucket
-- (recorded on `striatumd.repositories.blob_bucket`); two repositories
-- may not share a bucket.
--
-- Idempotent: `IF NOT EXISTS` guards on every column and index.

ALTER TABLE striatumd.artifacts
  ADD COLUMN IF NOT EXISTS blob_key          text,
  ADD COLUMN IF NOT EXISTS blob_sha256       text,
  ADD COLUMN IF NOT EXISTS blob_content_type text;

CREATE INDEX IF NOT EXISTS idx_artifacts_blob_key
  ON striatumd.artifacts(blob_key)
  WHERE blob_key IS NOT NULL;

ALTER TABLE striatumd.repositories
  ADD COLUMN IF NOT EXISTS blob_bucket     text,
  ADD COLUMN IF NOT EXISTS blob_created_at timestamptz;

-- Per-repo bucket isolation: two repositories cannot accidentally share
-- a bucket. NULL is permitted (a registered repo may pre-date this
-- migration or be pending an explicit `striatum adopt --apply-blob-creation`).
CREATE UNIQUE INDEX IF NOT EXISTS uq_repositories_blob_bucket
  ON striatumd.repositories(blob_bucket)
  WHERE blob_bucket IS NOT NULL;
