-- RFC 0110 Phase 2 (full) — owner bundle 0004: promote striatumd.events from
-- runtime DML to a SECURITY-DEFINER-only write surface (§7), the last L1 phase.
-- Only this phase licenses the sole-durable-write-path claim ("the daemon's
-- durable write paths are DB-enforced").
--
-- Applied OUT-OF-BAND as the database owner (never the runtime role; RFC 0079 §5),
-- and only AFTER a binary that supports the event_sd_append capability is deployed
-- (capability parity, §8.2) and the operator sets --pg-write-boundary full in
-- lockstep. Additive + cumulative on bundles 0001-0003: a daemon that has not
-- adopted P2 still routes event writes through the direct INSERT, so this bundle
-- applied without the matching binary fails closed at the runtime role's first
-- direct INSERT (42501), not silently.
--
-- The whole bundle applies in one transaction; ApplyOwnerBundles stamps
-- owner_bundle_meta last, so a partially-applied bundle cannot persist.

-- THE LOAD-BEARING P2 DECISION (decision-log + RFC 0110): the event chain hash is
-- computed ENTIRELY IN-DB, with NO Go counterpart. Unlike the audit chain (whose
-- doctor verifier recomputes row hashes in Go and therefore needs the byte-
-- identical V3RowHash builder), nothing in Go ever recomputes an event row hash:
-- canonicalEventHash is write-only and the only event-chain verifier
-- (assertEventChainLinear) checks chain *linkage* (previous_hash == prior
-- row_hash), not hash content. So the events v3 hash can be authoritative in-DB —
-- which satisfies G1 (the only write path computes the hash and holds the chain
-- lock in-DB) with ZERO porting hazard, because there is nothing to port to. The
-- v2 Go-computed event rows keep their hashes; the chain continues linearly across
-- the v2->v3 boundary (the SD fn reads the head's last_hash as previous_hash, so
-- linkage holds by construction). Trusted caller-supplied hashing was rejected for
-- the same reason audit rejected it: it would not be DB-enforced (G1).

-- event_v3_row_hash: the in-DB v3 event row hash. Length-prefixed canonical over
-- the event fields in fixed declared order, reusing the generic escaping-free,
-- key-order-free encoder from bundle 0001 (audit_v3_enc_text). payload_json is
-- folded in as its canonical jsonb text (jsonb normalizes key order / whitespace /
-- number form, so ::text is deterministic for a given logical value). This is a
-- fresh in-DB-only format; no Go builder mirrors it (see the decision note above).
CREATE OR REPLACE FUNCTION striatumd.event_v3_row_hash(
  p_previous_hash text,
  p_repository_id text,
  p_event_id bigint,
  p_run_id text,
  p_event_type text,
  p_actor_session_id text,
  p_job_id text,
  p_message_id text,
  p_artifact_id text,
  p_lease_id text,
  p_payload jsonb,
  p_created_at timestamptz
)
RETURNS text
LANGUAGE sql
IMMUTABLE
SET search_path = striatumd, public, pg_temp
AS $$
  SELECT encode(digest(
    striatumd.audit_v3_enc_text(p_previous_hash) ||
    striatumd.audit_v3_enc_text(p_repository_id) ||
    striatumd.audit_v3_enc_text(p_event_id::text) ||
    striatumd.audit_v3_enc_text(p_run_id) ||
    striatumd.audit_v3_enc_text(p_event_type) ||
    striatumd.audit_v3_enc_text(p_actor_session_id) ||
    striatumd.audit_v3_enc_text(p_job_id) ||
    striatumd.audit_v3_enc_text(p_message_id) ||
    striatumd.audit_v3_enc_text(p_artifact_id) ||
    striatumd.audit_v3_enc_text(p_lease_id) ||
    striatumd.audit_v3_enc_text(p_payload::text) ||
    striatumd.audit_v3_enc_text(to_char(p_created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')),
    'sha256'), 'hex')
$$;

-- append_event_row: the SD-only write path for events (RFC 0110 §7 P2). Asserts
-- daemon authority, enforces durable-event transcript exclusion (§12,
-- C-EVENT-NO-TRANSCRIPTS), locks the per-repository chain head, computes the v3
-- hash in-DB, appends the event, and advances the chain head — all atomic with
-- whatever transaction the caller is in (event appends ride the mutation tx, so
-- the event row commits or rolls back with the state transition that emitted it).
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
BEGIN
  PERFORM striatumd.assert_daemon_authority();

  -- Durable-event transcript exclusion (RFC 0110 §12, C-EVENT-NO-TRANSCRIPTS):
  -- the DB keeps a curated record, not a transcript store (AGENTS.md product
  -- boundary, D028). Reject any payload carrying a top-level transcript key, and
  -- reject transcript-sized payloads as a backstop. The forbidden-key check is
  -- the primary, low-false-positive mechanism (no curated event uses these
  -- top-level keys); the size cap (256 KiB) is far above any coordination event
  -- and below a real transcript. check_violation (23514) is distinct from the
  -- INSERT's FK code (23503), so the daemon and the gate can tell a rejected
  -- payload from a missing parent row.
  IF v_payload ?| ARRAY['stdout', 'stderr', 'transcript', 'raw_output', 'provider_output'] THEN
    RAISE EXCEPTION USING ERRCODE = '23514',
      MESSAGE = 'event payload rejected: durable events may not carry transcript keys (stdout/stderr/transcript/raw_output/provider_output)';
  END IF;
  IF octet_length(v_payload::text) > 262144 THEN
    RAISE EXCEPTION USING ERRCODE = '23514',
      MESSAGE = 'event payload rejected: payload exceeds the 256 KiB durable-event size cap (transcript-sized payloads are not durable state)';
  END IF;

  SELECT last_hash INTO v_previous_hash
    FROM striatumd.repo_event_chain_heads
   WHERE repository_id = p_repository_id
     FOR UPDATE;

  v_event_id := nextval(pg_get_serial_sequence('striatumd.events', 'event_id'));

  v_row_hash := striatumd.event_v3_row_hash(
    v_previous_hash, p_repository_id, v_event_id, p_run_id, p_event_type,
    p_actor_session_id, p_job_id, p_message_id, p_artifact_id, p_lease_id,
    v_payload, v_created_at);

  INSERT INTO striatumd.events (
    repository_id, event_id, run_id, event_type, actor_session_id, job_id,
    message_id, artifact_id, lease_id, payload_json, created_at,
    previous_hash, row_hash
  ) VALUES (
    p_repository_id, v_event_id, p_run_id, p_event_type, p_actor_session_id, p_job_id,
    p_message_id, p_artifact_id, p_lease_id, v_payload, v_created_at,
    v_previous_hash, v_row_hash
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

-- SD-hardening (RFC 0110 §6, C-SD-HARDEN): no PUBLIC execute; the runtime role
-- gets EXECUTE only on the sanctioned write path.
REVOKE ALL ON FUNCTION striatumd.event_v3_row_hash(
  text, text, bigint, text, text, text, text, text, text, text, jsonb, timestamptz) FROM PUBLIC;
REVOKE ALL ON FUNCTION striatumd.append_event_row(
  text, text, text, text, text, text, text, text, jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION striatumd.append_event_row(
  text, text, text, text, text, text, text, text, jsonb) TO striatumd_rw;

-- Phase 2 (full): the runtime role may no longer INSERT events directly; the only
-- write path is the SD function (T-42501-P2, T-EXEC-AUTH). The append-only
-- UPDATE/DELETE revokes/triggers on events are unchanged. repo_event_chain_heads
-- stays runtime-writable for parity with audit_chain_head (the SD fn advances it
-- as owner; it is a derived pointer, not the protected append-only surface).
REVOKE INSERT ON striatumd.events FROM striatumd_rw;

-- Capability stamp the binary's startup parity check verifies (RFC 0110 §8.2):
-- a binary that does not support event_sd_append refuses to serve once this is
-- stamped (old-binary / authority-schema), and a binary configured for P2 refuses
-- to serve until this is stamped (new-binary / old-schema).
INSERT INTO striatumd.schema_authority(capability, requires_daemon_auth, bundle_version)
VALUES ('event_sd_append', true, 4)
ON CONFLICT (capability) DO NOTHING;
