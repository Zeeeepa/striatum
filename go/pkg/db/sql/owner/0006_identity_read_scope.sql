-- RFC 0114 (D173) — runtime read scope R1 step 2: principal/session identity.
--
-- Applied OUT-OF-BAND as the database owner. This bundle closes the three
-- remaining runtime-owned R1 surfaces from #164: striatumd_rw loses direct
-- SELECT on principals (identity prose), principal_clients.principal_id (the
-- attribution graph), and client_sessions (session↔credential linkage). The
-- runtime read paths that genuinely need these rows go through owner-owned
-- SECURITY DEFINER projections gated by striatumd.assert_daemon_authority().
--
-- Ownership transfer is the load-bearing step (D173): these tables were
-- created by runtime-role migrations (0001_baseline.sql, 0023_principals.sql)
-- and are owned by striatumd_rw in production, so a plain REVOKE SELECT is NOT
-- a boundary — the owning role can re-grant itself SELECT in one statement
-- with the same leaked credential. After ALTER TABLE ... OWNER TO CURRENT_USER
-- the runtime role can no longer self-re-grant. In pgtest the pool user
-- already owns the tables, so the transfer is an idempotent no-op there; the
-- mechanism is pinned executable by TestOwnerTransferClosesSelfRegrant.
--
-- RFC 0079 §5 consequence, accepted explicitly: once owner-held, any future
-- schema change to principals / principal_clients / client_sessions must ship
-- as an owner bundle (or owner-applied `daemon migrate-db --admin-url` DDL),
-- exactly as clients / client_capabilities work today.

-- Projection functions. All use the authorize_capability secret-as-parameter
-- pattern (not load_token_for_update's ambient-GUC pattern) because the doctor
-- path calls on a bare db.Runner with no authorized-mutation prelude. Bodies
-- are verbatim transplants of the current go/pkg/admin handler SQL so the row
-- shapes — and therefore the Go decode paths — are bit-identical.

CREATE OR REPLACE FUNCTION striatumd.get_principal(
  p_daemon_secret text,
  p_principal_id text
)
RETURNS TABLE (
  principal_id text,
  principal_kind text,
  display_name text,
  created_at timestamptz,
  disabled_at timestamptz
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = striatumd, public, pg_temp
AS $$
BEGIN
  PERFORM set_config('striatum.daemon_auth', p_daemon_secret, true);
  PERFORM striatumd.assert_daemon_authority();
  RETURN QUERY
    SELECT p.principal_id, p.principal_kind, p.display_name,
           p.created_at, p.disabled_at
      FROM striatumd.principals p
     WHERE p.principal_id = p_principal_id;
END
$$;

CREATE OR REPLACE FUNCTION striatumd.resolve_principal_for_client(
  p_daemon_secret text,
  p_client_id text
)
RETURNS TABLE (
  principal_id text,
  principal_kind text,
  display_name text,
  disabled_at timestamptz
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = striatumd, public, pg_temp
AS $$
BEGIN
  PERFORM set_config('striatum.daemon_auth', p_daemon_secret, true);
  PERFORM striatumd.assert_daemon_authority();
  RETURN QUERY
    SELECT p.principal_id, p.principal_kind, p.display_name, p.disabled_at
      FROM striatumd.principal_clients pc
      JOIN striatumd.principals p ON p.principal_id = pc.principal_id
     WHERE pc.client_id = p_client_id AND pc.unlinked_at IS NULL;
END
$$;

CREATE OR REPLACE FUNCTION striatumd.list_principal_scopes(
  p_daemon_secret text
)
RETURNS text
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = striatumd, public, pg_temp
AS $$
DECLARE
  v_payload text;
BEGIN
  PERFORM set_config('striatum.daemon_auth', p_daemon_secret, true);
  PERFORM striatumd.assert_daemon_authority();
  WITH grants AS (
    SELECT pc.principal_id,
           cc.capability,
           cc.repository_id,
           (cc.session_id IS NOT NULL) AS session_bound
      FROM striatumd.principal_clients pc
      JOIN striatumd.client_capabilities cc ON cc.client_id = pc.client_id
     WHERE pc.unlinked_at IS NULL
       AND cc.revoked_at IS NULL
       AND (cc.expires_at IS NULL OR cc.expires_at > now())
  ),
  client_counts AS (
    SELECT pc.principal_id, COUNT(*) AS client_count
      FROM striatumd.principal_clients pc
     WHERE pc.unlinked_at IS NULL
     GROUP BY pc.principal_id
  )
  SELECT COALESCE(
    jsonb_agg(
      jsonb_build_object(
        'principal_id', p.principal_id,
        'kind', p.principal_kind,
        'display_name', p.display_name,
        'disabled', (p.disabled_at IS NOT NULL),
        'client_count', COALESCE(cc.client_count, 0),
        'repositories', COALESCE((
          SELECT jsonb_agg(DISTINCT g.repository_id ORDER BY g.repository_id)
            FROM grants g
           WHERE g.principal_id = p.principal_id AND g.repository_id IS NOT NULL
        ), '[]'::jsonb),
        'capabilities', COALESCE((
          SELECT jsonb_agg(
                   jsonb_build_object(
                     'capability', g.capability,
                     'repository_id', g.repository_id,
                     'session_bound', g.session_bound
                   )
                   ORDER BY g.capability, g.repository_id NULLS FIRST
                 )
            FROM (
              SELECT capability, repository_id, bool_or(session_bound) AS session_bound
                FROM grants
               WHERE grants.principal_id = p.principal_id
               GROUP BY capability, repository_id
            ) g
        ), '[]'::jsonb)
      )
      ORDER BY p.principal_id
    ),
    '[]'::jsonb
  )::text
    INTO v_payload
    FROM striatumd.principals p
    LEFT JOIN client_counts cc ON cc.principal_id = p.principal_id;
  RETURN v_payload;
END
$$;

-- link_client_to_principal: the RFC 0114 Open Question 4 contingency, taken.
-- PostgreSQL requires SELECT on the ON CONFLICT arbiter columns (verified
-- live: the upsert below fails 42501 without SELECT(principal_id), the exact
-- column this bundle gates), so the active-link upsert moves behind daemon
-- authority as an owner-owned write function. The body is the verbatim
-- statement from admin.LinkClientToPrincipal; the existence and active-link
-- checks stay in Go (through the read projections above), so external
-- behavior is unchanged.
CREATE OR REPLACE FUNCTION striatumd.link_client_to_principal(
  p_daemon_secret text,
  p_principal_id text,
  p_client_id text
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = striatumd, public, pg_temp
AS $$
BEGIN
  PERFORM set_config('striatum.daemon_auth', p_daemon_secret, true);
  PERFORM striatumd.assert_daemon_authority();
  INSERT INTO striatumd.principal_clients(principal_id, client_id, linked_at, unlinked_at)
  VALUES (p_principal_id, p_client_id, now(), NULL)
  ON CONFLICT (principal_id, client_id)
  DO UPDATE SET unlinked_at = NULL, linked_at = now();
END
$$;

REVOKE ALL ON FUNCTION striatumd.get_principal(text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION striatumd.resolve_principal_for_client(text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION striatumd.list_principal_scopes(text) FROM PUBLIC;
REVOKE ALL ON FUNCTION striatumd.link_client_to_principal(text, text, text) FROM PUBLIC;

GRANT EXECUTE ON FUNCTION striatumd.get_principal(text, text) TO striatumd_rw;
GRANT EXECUTE ON FUNCTION striatumd.resolve_principal_for_client(text, text) TO striatumd_rw;
GRANT EXECUTE ON FUNCTION striatumd.list_principal_scopes(text) TO striatumd_rw;
GRANT EXECUTE ON FUNCTION striatumd.link_client_to_principal(text, text, text) TO striatumd_rw;

-- Ownership transfer FIRST relative to the revokes: a REVOKE against a
-- runtime-owned table is reversible by the runtime role itself. CURRENT_USER
-- is the owner DSN role applying this bundle — the same role that owns the
-- 0001–0005 SD functions and the clients table. No projection is created for
-- client_sessions (zero runtime Go consumers; full SELECT denial).
ALTER TABLE striatumd.client_sessions   OWNER TO CURRENT_USER;
ALTER TABLE striatumd.principals        OWNER TO CURRENT_USER;
ALTER TABLE striatumd.principal_clients OWNER TO CURRENT_USER;

-- Revokes and grant-backs: read surface narrowed, write surface preserved
-- exactly. The DML grant-backs are restated explicitly (not inherited from
-- migration 0023's grants) so this bundle and ReassertReadRevokes fully
-- determine the post-close ACL.
--
-- principal_clients keeps a COLUMN gate rather than a full table revoke:
-- admin/tokens.go's unlinkClientFromPrincipals runs
-- `UPDATE ... WHERE client_id = $1 AND unlinked_at IS NULL`, and PostgreSQL
-- requires SELECT privilege on columns read by a write statement, so
-- client_id/unlinked_at (plus linked_at) stay runtime-selectable. Without
-- principal_id a leaked runtime credential sees client ids and timestamps,
-- not whose credentials they are. The link upsert, whose ON CONFLICT arbiter
-- reads principal_id, goes through link_client_to_principal above.
REVOKE SELECT ON striatumd.client_sessions FROM striatumd_rw;
GRANT INSERT, UPDATE, DELETE ON striatumd.client_sessions TO striatumd_rw;

REVOKE SELECT ON striatumd.principals FROM striatumd_rw;
GRANT INSERT, UPDATE, DELETE ON striatumd.principals TO striatumd_rw;

REVOKE SELECT ON striatumd.principal_clients FROM striatumd_rw;
GRANT SELECT (client_id, linked_at, unlinked_at)
  ON striatumd.principal_clients TO striatumd_rw;
GRANT INSERT, UPDATE, DELETE ON striatumd.principal_clients TO striatumd_rw;

INSERT INTO striatumd.schema_authority(capability, requires_daemon_auth, bundle_version)
VALUES ('identity_projection_read', true, 6)
ON CONFLICT (capability) DO NOTHING;
