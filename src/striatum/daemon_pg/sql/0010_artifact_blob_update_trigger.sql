-- GH #27: refine `artifacts_no_update` so RFC 0072 blob-reference
-- updates are permitted without disabling the append-only trigger.
--
-- Migration 0005 installed a generic `refuse_repo_append_only_change`
-- trigger that refuses every UPDATE on `striatumd.artifacts`. RFC 0072
-- (migration 0009) added `(blob_key, blob_sha256, blob_content_type)`
-- columns that are not part of the artifact's immutable identity and
-- need to be writable by `artifact.backfill_blob`. This migration
-- replaces the artifacts-only trigger with a column-aware function that
-- still refuses any UPDATE touching a non-blob column, and grants
-- column-level UPDATE on the three blob-reference columns to
-- `striatumd_rw`.
--
-- The generic `refuse_repo_append_only_change()` function is unchanged
-- so the `events` triggers continue to refuse all updates and deletes.
-- The artifacts delete trigger is also unchanged: blob backfill never
-- needs to delete a row.

CREATE OR REPLACE FUNCTION striatumd.allow_artifact_blob_reference_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  blob_columns CONSTANT text[] := ARRAY['blob_key','blob_sha256','blob_content_type'];
  filtered_old jsonb;
  filtered_new jsonb;
BEGIN
  filtered_old := to_jsonb(OLD) - blob_columns;
  filtered_new := to_jsonb(NEW) - blob_columns;
  IF filtered_old IS DISTINCT FROM filtered_new THEN
    RAISE EXCEPTION 'repo-local append-only rows cannot be updated or deleted';
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS artifacts_no_update ON striatumd.artifacts;
CREATE TRIGGER artifacts_no_update
BEFORE UPDATE ON striatumd.artifacts
FOR EACH ROW EXECUTE FUNCTION striatumd.allow_artifact_blob_reference_update();

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT UPDATE (blob_key, blob_sha256, blob_content_type)
      ON striatumd.artifacts TO striatumd_rw;
  END IF;
END;
$$;
