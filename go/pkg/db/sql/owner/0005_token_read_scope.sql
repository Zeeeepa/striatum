-- RFC 0113 R1 — token-secret read projection.
--
-- Applied OUT-OF-BAND as the database owner. This bundle is the first narrow
-- read-scope reduction for #164: striatumd_rw keeps direct SELECT on non-secret
-- client metadata, but token_hash/token_salt move behind owner-owned SECURITY
-- DEFINER functions that require daemon authority.

CREATE OR REPLACE FUNCTION striatumd.authorize_capability(
  p_daemon_secret text,
  p_required_capability text,
  p_repository_id text,
  p_token_id text,
  p_token_secret text,
  p_now timestamptz
)
RETURNS TABLE (
  client_id text,
  token_id text,
  repository_id text,
  session_id text,
  capability text,
  decision text,
  denial_reason text
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = striatumd, public, pg_temp
AS $$
DECLARE
  v_client_id text;
  v_token_hash text;
  v_token_salt text;
  v_token_revoked_at timestamptz;
  v_token_expires_at timestamptz;
  v_capability_id text;
  v_cap_repository_id text;
  v_cap_expires_at timestamptz;
  v_cap_session_id text;
  v_expected_hash text;
  v_scope_mismatch boolean;
BEGIN
  PERFORM set_config('striatum.daemon_auth', p_daemon_secret, true);
  PERFORM striatumd.assert_daemon_authority();

  token_id := p_token_id;
  repository_id := p_repository_id;
  capability := p_required_capability;
  decision := 'denied';
  session_id := NULL;

  SELECT c.client_id, c.token_hash, c.token_salt, c.revoked_at, c.expires_at
    INTO v_client_id, v_token_hash, v_token_salt, v_token_revoked_at, v_token_expires_at
    FROM striatumd.clients c
   WHERE c.token_id = p_token_id;
  IF NOT FOUND THEN
    denial_reason := 'token_invalid';
    RETURN NEXT;
    RETURN;
  END IF;

  client_id := v_client_id;
  v_expected_hash := encode(hmac(
    convert_to(p_token_secret, 'UTF8'),
    convert_to(v_token_salt, 'UTF8'),
    'sha256'), 'hex');
  IF v_expected_hash <> v_token_hash THEN
    denial_reason := 'token_invalid';
    RETURN NEXT;
    RETURN;
  END IF;
  IF v_token_revoked_at IS NOT NULL THEN
    denial_reason := 'token_revoked';
    RETURN NEXT;
    RETURN;
  END IF;
  IF v_token_expires_at IS NOT NULL AND v_token_expires_at <= p_now THEN
    denial_reason := 'token_expired';
    RETURN NEXT;
    RETURN;
  END IF;

  SELECT cc.capability_id, cc.repository_id, cc.expires_at, cc.session_id
    INTO v_capability_id, v_cap_repository_id, v_cap_expires_at, v_cap_session_id
    FROM striatumd.client_capabilities cc
   WHERE cc.client_id = v_client_id
     AND cc.capability = p_required_capability
     AND (cc.repository_id IS NULL OR cc.repository_id = p_repository_id)
     AND cc.revoked_at IS NULL
   ORDER BY cc.session_id IS NULL, cc.repository_id IS NULL
   LIMIT 1;
  IF NOT FOUND THEN
    IF p_repository_id <> '' THEN
      SELECT EXISTS (
        SELECT 1
          FROM striatumd.client_capabilities cc
         WHERE cc.client_id = v_client_id
           AND cc.capability = p_required_capability
           AND cc.repository_id IS NOT NULL
           AND cc.repository_id <> p_repository_id
           AND cc.revoked_at IS NULL
         LIMIT 1
      ) INTO v_scope_mismatch;
      IF v_scope_mismatch THEN
        denial_reason := 'capability_scope_mismatch';
        RETURN NEXT;
        RETURN;
      END IF;
    END IF;
    denial_reason := 'capability_missing';
    RETURN NEXT;
    RETURN;
  END IF;

  IF v_cap_expires_at IS NOT NULL AND v_cap_expires_at <= p_now THEN
    denial_reason := 'capability_expired';
    RETURN NEXT;
    RETURN;
  END IF;

  session_id := v_cap_session_id;
  decision := 'allowed';
  denial_reason := NULL;
  RETURN NEXT;
END
$$;

CREATE OR REPLACE FUNCTION striatumd.load_token_for_update(p_token_id text)
RETURNS TABLE (
  client_id text,
  client_kind text,
  display_name text,
  token_id text,
  token_hash text,
  token_salt text,
  expires_at timestamptz,
  revoked_at timestamptz
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = striatumd, public, pg_temp
AS $$
BEGIN
  PERFORM striatumd.assert_daemon_authority();
  RETURN QUERY
    SELECT c.client_id, c.client_kind, c.display_name, c.token_id,
           c.token_hash, c.token_salt, c.expires_at, c.revoked_at
      FROM striatumd.clients c
     WHERE c.token_id = p_token_id
     FOR UPDATE;
END
$$;

REVOKE ALL ON FUNCTION striatumd.authorize_capability(text, text, text, text, text, timestamptz) FROM PUBLIC;
REVOKE ALL ON FUNCTION striatumd.load_token_for_update(text) FROM PUBLIC;

GRANT EXECUTE ON FUNCTION striatumd.authorize_capability(text, text, text, text, text, timestamptz) TO striatumd_rw;
GRANT EXECUTE ON FUNCTION striatumd.load_token_for_update(text) TO striatumd_rw;

-- Remove table-level SELECT so token_hash/token_salt are no longer directly
-- readable through a leaked runtime DSN, then grant back only non-secret client
-- metadata needed by existing daemon operations.
REVOKE SELECT ON striatumd.clients FROM striatumd_rw;
GRANT SELECT (
  client_id, client_kind, display_name, token_id,
  created_at, expires_at, revoked_at, last_used_at
) ON striatumd.clients TO striatumd_rw;

INSERT INTO striatumd.schema_authority(capability, requires_daemon_auth, bundle_version)
VALUES ('auth_projection_read', true, 5)
ON CONFLICT (capability) DO NOTHING;
