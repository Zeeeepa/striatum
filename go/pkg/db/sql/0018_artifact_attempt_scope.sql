-- RFC 0095 §1 / GH #84: attempt-scoped artifacts.
--
-- A revision cycle re-opens a job and bumps `jobs.attempt`. The artifacts
-- table previously enforced UNIQUE (repository_id, run_id, job_id, logical_name)
-- and UNIQUE (repository_id, run_id, repo_path, content_sha256) with NO attempt
-- dimension, so a re-opened attempt republishing the SAME logical_name (or the
-- same path with new content) collided on the existing row — the #84
-- `artifact logical name already exists with different content` wedge.
--
-- This migration records the attempt that produced each artifact and widens
-- BOTH unique keys with `attempt`, so a fresh attempt may republish the same
-- logical_name/path under its own attempt while the prior attempt's row is
-- retained for provenance. The gate readers (verifyRequiredArtifacts,
-- recovery auto-finalize) only honor artifacts whose attempt == jobs.attempt.
--
-- OWNER-APPLIED (RFC 0079 §5): this ALTERs the owner-held striatumd.artifacts
-- table, so it is applied by the table owner via
-- `striatum daemon migrate-db --admin-url <owner-dsn>`, NOT by the runtime
-- role (which would crash-loop on an owner-table ALTER). Existing rows default
-- to attempt 1; the column add is metadata-only (no row UPDATE), so the
-- append-only artifacts_no_update trigger is not involved.

ALTER TABLE striatumd.artifacts
  ADD COLUMN IF NOT EXISTS attempt integer NOT NULL DEFAULT 1;

ALTER TABLE striatumd.artifacts
  DROP CONSTRAINT IF EXISTS artifacts_repository_id_run_id_job_id_logical_name_key;
ALTER TABLE striatumd.artifacts
  ADD CONSTRAINT artifacts_run_job_logical_attempt_key
  UNIQUE (repository_id, run_id, job_id, logical_name, attempt);

ALTER TABLE striatumd.artifacts
  DROP CONSTRAINT IF EXISTS artifacts_repository_id_run_id_repo_path_content_sha256_key;
ALTER TABLE striatumd.artifacts
  ADD CONSTRAINT artifacts_run_path_content_attempt_key
  UNIQUE (repository_id, run_id, repo_path, content_sha256, attempt);
