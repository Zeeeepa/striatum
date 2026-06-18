-- GH #372 / GH #379 — owner bundle 0014: chain-head lock wait gauges.
--
-- The hot append-only chains serialize on their head rows:
--   * striatumd.repo_event_chain_heads per repository for durable events.
--   * striatumd.audit_chain_head globally for audit_log.
-- This bundle adds nullable lock_wait_us gauges and restates the SECURITY
-- DEFINER append functions so the measured value is written atomically with the
-- row that waited. The gauge is observability-only and is deliberately excluded
-- from the row-hash input, preserving the existing hash contracts.
--
-- No runtime migration: both tables/functions are owner-admin surfaces in the
-- two-role posture. No index: doctor samples bounded tails instead of adding hot
-- write-path maintenance.

ALTER TABLE striatumd.events
  ADD COLUMN IF NOT EXISTS lock_wait_us bigint;

ALTER TABLE striatumd.audit_log
  ADD COLUMN IF NOT EXISTS lock_wait_us bigint;

CREATE OR REPLACE FUNCTION striatumd.append_audit_row(
  p_daemon_version text,
  p_client_id text,
  p_repository_id text,
  p_method text,
  p_decision text,
  p_denial_reason text,
  p_transport text,
  p_request_id text,
  p_ok boolean,
  p_params_sha256 text
)
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = striatumd, public, pg_temp
AS $$
DECLARE
  v_last_hash text;
  v_segment_id bigint;
  v_ts timestamptz := date_trunc('second', now());
  v_exit_code integer := CASE WHEN p_ok THEN NULL ELSE 10 END;
  v_row_hash text;
  v_audit_id bigint;
  v_lock_wait_started_at timestamptz;
  v_lock_wait_us bigint := 0;
BEGIN
  PERFORM striatumd.assert_daemon_authority();

  v_lock_wait_started_at := clock_timestamp();
  SELECT last_hash INTO v_last_hash
    FROM striatumd.audit_chain_head
   WHERE singleton = true
     FOR UPDATE;
  v_lock_wait_us := GREATEST(
    0,
    floor(EXTRACT(EPOCH FROM (clock_timestamp() - v_lock_wait_started_at)) * 1000000)::bigint
  );

  SELECT segment_id INTO v_segment_id
    FROM striatumd.audit_segments
   WHERE state = 'open'
   ORDER BY segment_id DESC
   LIMIT 1;
  IF v_segment_id IS NULL THEN
    INSERT INTO striatumd.audit_segments(opened_at, state, retention_state)
    VALUES (now(), 'open', 'active')
    RETURNING segment_id INTO v_segment_id;
  END IF;

  v_row_hash := striatumd.audit_v3_row_hash(
    v_ts, 1, 3, p_daemon_version, p_client_id, p_repository_id, p_method,
    p_decision, p_denial_reason, p_transport, p_request_id, v_exit_code,
    p_params_sha256, v_last_hash, v_segment_id);

  INSERT INTO striatumd.audit_log (
    ts, schema_version, hash_format_version, daemon_version,
    client_id, repository_id, method, decision, denial_reason,
    transport, request_id, exit_code, params_sha256, previous_hash,
    row_hash, segment_id, lock_wait_us
  ) VALUES (
    v_ts, 1, 3, p_daemon_version,
    p_client_id, p_repository_id, p_method, p_decision, p_denial_reason,
    p_transport, p_request_id, v_exit_code, p_params_sha256, v_last_hash,
    v_row_hash, v_segment_id, v_lock_wait_us
  ) RETURNING audit_id INTO v_audit_id;

  UPDATE striatumd.audit_chain_head
     SET last_audit_id = v_audit_id, last_hash = v_row_hash, updated_at = now()
   WHERE singleton = true;

  RETURN v_audit_id;
END
$$;

CREATE OR REPLACE FUNCTION striatumd.append_event_row(
  p_repository_id text,
  p_run_id text,
  p_event_type text,
  p_actor_session_id text,
  p_job_id text,
  p_message_id text,
  p_artifact_id text,
  p_lease_id text,
  p_payload jsonb
)
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = striatumd, public, pg_temp
AS $$
DECLARE
  v_payload jsonb := COALESCE(p_payload, '{}'::jsonb);
  v_previous_hash text;
  v_event_id bigint;
  v_created_at timestamptz := date_trunc('second', now());
  v_row_hash text;
  v_lock_wait_started_at timestamptz;
  v_lock_wait_us bigint := 0;
BEGIN
  PERFORM striatumd.assert_daemon_authority();

  IF v_payload ?| ARRAY['stdout', 'stderr', 'transcript', 'raw_output', 'provider_output'] THEN
    RAISE EXCEPTION USING ERRCODE = '23514',
      MESSAGE = 'event payload rejected: durable events may not carry transcript keys (stdout/stderr/transcript/raw_output/provider_output)';
  END IF;
  IF octet_length(v_payload::text) > 262144 THEN
    RAISE EXCEPTION USING ERRCODE = '23514',
      MESSAGE = 'event payload rejected: payload exceeds the 256 KiB durable-event size cap (transcript-sized payloads are not durable state)';
  END IF;

  v_lock_wait_started_at := clock_timestamp();
  SELECT last_hash INTO v_previous_hash
    FROM striatumd.repo_event_chain_heads
   WHERE repository_id = p_repository_id
     FOR UPDATE;
  v_lock_wait_us := GREATEST(
    0,
    floor(EXTRACT(EPOCH FROM (clock_timestamp() - v_lock_wait_started_at)) * 1000000)::bigint
  );

  v_event_id := nextval(pg_get_serial_sequence('striatumd.events', 'event_id'));

  v_row_hash := striatumd.event_v3_row_hash(
    v_previous_hash, p_repository_id, v_event_id, p_run_id, p_event_type,
    p_actor_session_id, p_job_id, p_message_id, p_artifact_id, p_lease_id,
    v_payload, v_created_at);

  INSERT INTO striatumd.events (
    repository_id, event_id, run_id, event_type, actor_session_id, job_id,
    message_id, artifact_id, lease_id, payload_json, created_at,
    previous_hash, row_hash, lock_wait_us
  ) VALUES (
    p_repository_id, v_event_id, p_run_id, p_event_type, p_actor_session_id, p_job_id,
    p_message_id, p_artifact_id, p_lease_id, v_payload, v_created_at,
    v_previous_hash, v_row_hash, v_lock_wait_us
  );

  INSERT INTO striatumd.repo_event_chain_heads(
    repository_id, last_event_id, last_hash, updated_at
  ) VALUES (p_repository_id, v_event_id, v_row_hash, now())
  ON CONFLICT (repository_id)
  DO UPDATE SET last_event_id = EXCLUDED.last_event_id,
                last_hash = EXCLUDED.last_hash,
                updated_at = now();

  RETURN v_event_id;
END
$$;

REVOKE ALL ON FUNCTION striatumd.append_audit_row(text, text, text, text, text, text, text, text, boolean, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION striatumd.append_event_row(text, text, text, text, text, text, text, text, jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION striatumd.append_audit_row(text, text, text, text, text, text, text, text, boolean, text) TO striatumd_rw;
GRANT EXECUTE ON FUNCTION striatumd.append_event_row(text, text, text, text, text, text, text, text, jsonb) TO striatumd_rw;
