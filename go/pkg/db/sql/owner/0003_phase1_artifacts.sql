-- RFC 0110 Phase 1 (audit_artifacts) — owner bundle 0003: promote striatumd.artifacts
-- from runtime DML to a SECURITY-DEFINER-only write surface (§7).
--
-- Applied OUT-OF-BAND as the database owner (never the runtime role; RFC 0079 §5),
-- and only AFTER a binary that supports the artifact_sd_append capability is
-- deployed (capability parity, §8.2) and the operator sets --pg-write-boundary
-- audit_artifacts in lockstep. Additive + cumulative on bundle 0001/0002: a
-- daemon that has not adopted P1 still routes artifact writes through the direct
-- INSERT, so this bundle being applied without the matching binary fails closed
-- at the runtime role's first direct INSERT (42501), not silently.
--
-- The whole bundle applies in one transaction; ApplyOwnerBundles stamps
-- owner_bundle_meta last, so a partially-applied bundle cannot persist.

-- append_artifact_row: the SD-only write path for artifacts (RFC 0110 §7 P1).
-- Asserts daemon authority (the secret the prelude set on the caller's mutation
-- transaction) before the INSERT, so a leaked runtime credential cannot forge an
-- artifact. The INSERT runs in the caller's transaction (function call, not a new
-- tx) so the artifacts row commits atomically with the rest of the mutation and
-- its run/job/session foreign keys resolve. publish_mode/attempt fall back to the
-- column-default shape ('create' / 1) when the caller passes NULL.
CREATE OR REPLACE FUNCTION striatumd.append_artifact_row(
  p_repository_id text,
  p_artifact_id text,
  p_run_id text,
  p_job_id text,
  p_session_id text,
  p_logical_name text,
  p_artifact_kind text,
  p_repo_path text,
  p_content_sha256 text,
  p_size_bytes bigint,
  p_publish_mode text,
  p_created_at timestamptz,
  p_author_line text,
  p_blob_key text,
  p_blob_sha256 text,
  p_blob_content_type text,
  p_attempt integer
)
RETURNS text
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = striatumd, public, pg_temp
AS $$
BEGIN
  PERFORM striatumd.assert_daemon_authority();

  INSERT INTO striatumd.artifacts (
    repository_id, artifact_id, run_id, job_id, session_id, logical_name,
    artifact_kind, repo_path, content_sha256, size_bytes, publish_mode,
    created_at, author_line, blob_key, blob_sha256, blob_content_type, attempt
  ) VALUES (
    p_repository_id, p_artifact_id, p_run_id, p_job_id, p_session_id, p_logical_name,
    p_artifact_kind, p_repo_path, p_content_sha256, p_size_bytes,
    COALESCE(p_publish_mode, 'create'),
    p_created_at, p_author_line, p_blob_key, p_blob_sha256, p_blob_content_type,
    COALESCE(p_attempt, 1)
  );

  RETURN p_artifact_id;
END
$$;

-- SD-hardening (RFC 0110 §6, C-SD-HARDEN): no PUBLIC execute; the runtime role
-- gets EXECUTE only on the sanctioned write path.
REVOKE ALL ON FUNCTION striatumd.append_artifact_row(
  text, text, text, text, text, text, text, text, text, bigint, text,
  timestamptz, text, text, text, text, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION striatumd.append_artifact_row(
  text, text, text, text, text, text, text, text, text, bigint, text,
  timestamptz, text, text, text, text, integer) TO striatumd_rw;

-- Phase 1 (audit_artifacts): the runtime role may no longer INSERT artifacts
-- directly; the only write path is the SD function (T-42501-P1, T-EXEC-AUTH).
-- The append-only UPDATE/DELETE revokes/triggers on artifacts are unchanged.
REVOKE INSERT ON striatumd.artifacts FROM striatumd_rw;

-- Capability stamp the binary's startup parity check verifies (RFC 0110 §8.2):
-- a binary that does not support artifact_sd_append refuses to serve once this
-- is stamped (old-binary / authority-schema), and a binary configured for P1
-- refuses to serve until this is stamped (new-binary / old-schema).
INSERT INTO striatumd.schema_authority(capability, requires_daemon_auth, bundle_version)
VALUES ('artifact_sd_append', true, 3)
ON CONFLICT (capability) DO NOTHING;
