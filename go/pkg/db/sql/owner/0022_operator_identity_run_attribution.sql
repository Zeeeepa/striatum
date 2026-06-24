-- RFC 0167 P0 (D260) — operator identity & run attribution.
--
-- Applied OUT-OF-BAND as the database owner (`striatum daemon owner-ddl apply`),
-- THEN restart onto the binary that requires this watermark (RFC 0142 Layer 2).
--
-- Ordinal note: owner ordinal 0021 is RESERVED by the RFC 0142 P4 C3 DDL-revoke
-- (DDLRevokeOwnerBundleVersion = 21, STAGED outside ownerBundleFS under Option B /
-- D7'). RFC 0167 P0 therefore takes the next free EMBEDDED ordinal, 0022 — a normal
-- `owner-ddl apply`-eligible bundle that sits ABOVE the staged revoke. The owner.go
-- frontier predicates are retargeted from ">= the revoke frontier" to "== the exact
-- revoke version" so this normal bundle applies and the revoke stays excluded.
--
-- This bundle is the C2" composed-read-scope closure (the decisive blocker of the
-- v4 design): both operator-routed composed routes that would reconstruct
-- client_id -> principal_id over the full ACL graph are closed at the column layer,
-- and the identity reads ride owner-owned SECURITY DEFINER projections gated by
-- striatumd.assert_daemon_authority() (the RFC 0114 / bundle 0006 pattern).
--
-- The two new tables (operator_handles, operator_sessions) and the runs origin
-- columns are owner-held: they are created/altered here as CURRENT_USER (the owner
-- DSN role). runs is already owner-held on a two-role deploy (NOT in bundle 0018's
-- transfer cohort), so the runs SELECT REVOKE below is an irreversible boundary with
-- NO ownership transfer step (unlike bundle 0006's principal_clients, which was
-- runtime-owned). RFC 0079 §5 consequence, accepted: any future schema change to
-- these surfaces ships as an owner bundle.

-- (1) operator_handles: the leased, memorable rendering layer over principal_id.
CREATE TABLE IF NOT EXISTS striatumd.operator_handles (
  handle_id         text PRIMARY KEY,
  repository_id     text NOT NULL REFERENCES striatumd.repositories(repository_id),
  principal_id      text NOT NULL REFERENCES striatumd.principals(principal_id),  -- SELECT NOT granted (C2" Route 1)
  handle            text NOT NULL,                                                -- lowercase, privacy-safe
  leased_session_id text NOT NULL,
  leased_until      timestamptz NOT NULL,
  released_at       timestamptz,
  last_heartbeat_at timestamptz
);
-- Live-uniqueness scoped to un-released rows on a repo: the same word on a different
-- repo is fine, and a dead operator's released handle cannot be impersonated.
CREATE UNIQUE INDEX IF NOT EXISTS operator_handles_live_uq
  ON striatumd.operator_handles (repository_id, lower(handle)) WHERE released_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS operator_handles_live_session_uq
  ON striatumd.operator_handles (repository_id, leased_session_id) WHERE released_at IS NULL;

-- (2) operator_sessions: the pre-run operator-session substrate + lifecycle.
CREATE TABLE IF NOT EXISTS striatumd.operator_sessions (
  repository_id       text NOT NULL REFERENCES striatumd.repositories(repository_id),
  operator_session_id text NOT NULL,
  principal_id        text NOT NULL REFERENCES striatumd.principals(principal_id),  -- SELECT NOT granted (C2'/C2")
  client_id           text NOT NULL,                                               -- SELECT NOT granted (C2'/C2")
  state               text NOT NULL CHECK (state IN ('active','closed','expired')),
  registered_at       timestamptz NOT NULL,
  last_heartbeat_at   timestamptz,
  expires_at          timestamptz NOT NULL,
  closed_at           timestamptz,
  close_reason        text,
  PRIMARY KEY (repository_id, operator_session_id)
);
CREATE INDEX IF NOT EXISTS operator_sessions_principal_live
  ON striatumd.operator_sessions (repository_id, principal_id) WHERE state = 'active';

-- (3) runs write-once origin stamp columns (owner-held table -> owner bundle, C-1).
ALTER TABLE striatumd.runs ADD COLUMN IF NOT EXISTS created_by_principal_id text REFERENCES striatumd.principals(principal_id);
ALTER TABLE striatumd.runs ADD COLUMN IF NOT EXISTS created_by_handle_id    text REFERENCES striatumd.operator_handles(handle_id);

-- (4) write-once at the DB: a BEFORE UPDATE trigger that raises when either origin
-- column changes (chosen over a column UPDATE-revoke because runs is actively
-- UPDATEd on ~six paths). The stamp happens at INSERT (no BEFORE UPDATE fire);
-- IS DISTINCT FROM also forbids a later NULL -> value UPDATE. The trigger reads
-- OLD/NEW row variables (no column SELECT privilege needed), so the SELECT revoke
-- in (6) does not affect it. Orthogonal to the SELECT revoke: this enforces
-- immutability (UPDATE), the revoke enforces read-scope (SELECT); both required.
CREATE OR REPLACE FUNCTION striatumd.refuse_run_origin_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.created_by_principal_id IS DISTINCT FROM OLD.created_by_principal_id
     OR NEW.created_by_handle_id  IS DISTINCT FROM OLD.created_by_handle_id THEN
    RAISE EXCEPTION 'runs.created_by_* origin stamp is write-once (set at run creation, immutable thereafter)';
  END IF;
  RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS runs_origin_write_once ON striatumd.runs;
CREATE TRIGGER runs_origin_write_once BEFORE UPDATE ON striatumd.runs
  FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_run_origin_change();

-- (5) SECURITY DEFINER identity projections (C2".2c). They read the identity
-- columns AS THE DEFINER (owner); striatumd_rw never selects them directly. Both
-- require p_daemon_secret and call assert_daemon_authority(), so a leaked runtime
-- credential ALONE cannot invoke them to map client -> principal. Bodies follow the
-- bundle 0006 resolve_principal_for_client secret-as-parameter shape.

-- whose <run-id>: the origin identity for one run.
CREATE OR REPLACE FUNCTION striatumd.run_origin_identity(
  p_daemon_secret text,
  p_repository_id text,
  p_run_id text
)
RETURNS TABLE (
  run_id text,
  state text,
  created_by_principal_id text,
  origin_handle text,
  principal_kind text,
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
    SELECT r.run_id, r.state, r.created_by_principal_id,
           oh.handle, p.principal_kind, p.disabled_at
      FROM striatumd.runs r
      LEFT JOIN striatumd.operator_handles oh ON oh.handle_id = r.created_by_handle_id
      LEFT JOIN striatumd.principals       p  ON p.principal_id = r.created_by_principal_id
     WHERE r.repository_id = p_repository_id AND r.run_id = p_run_id;
END
$$;

-- status --mine: the run-ids whose origin principal = the live caller's principal.
CREATE OR REPLACE FUNCTION striatumd.runs_for_origin_client(
  p_daemon_secret text,
  p_repository_id text,
  p_client_id text
)
RETURNS TABLE (
  run_id text,
  state text,
  created_by_handle_id text
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = striatumd, public, pg_temp
AS $$
BEGIN
  PERFORM set_config('striatum.daemon_auth', p_daemon_secret, true);
  PERFORM striatumd.assert_daemon_authority();
  RETURN QUERY
    SELECT r.run_id, r.state, r.created_by_handle_id
      FROM striatumd.runs r
      JOIN striatumd.principal_clients pc ON pc.principal_id = r.created_by_principal_id
     WHERE r.repository_id = p_repository_id
       AND pc.client_id = p_client_id
       AND pc.unlinked_at IS NULL;
END
$$;

-- doctor attribution_unknown (D7): non-terminal runs with NO origin principal.
-- created_by_principal_id is SELECT-revoked for striatumd_rw (Route 2), so the
-- advisory scan must ride this DEFINER projection, not a direct WHERE.
CREATE OR REPLACE FUNCTION striatumd.runs_missing_origin(
  p_daemon_secret text,
  p_repository_id text
)
RETURNS TABLE (
  run_id text,
  state text
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = striatumd, public, pg_temp
AS $$
BEGIN
  PERFORM set_config('striatum.daemon_auth', p_daemon_secret, true);
  PERFORM striatumd.assert_daemon_authority();
  RETURN QUERY
    SELECT r.run_id, r.state
      FROM striatumd.runs r
     WHERE r.repository_id = p_repository_id
       AND r.created_by_principal_id IS NULL
       AND r.state NOT IN ('completed', 'failed', 'canceled');
END
$$;

REVOKE ALL ON FUNCTION striatumd.run_origin_identity(text, text, text)    FROM PUBLIC;
REVOKE ALL ON FUNCTION striatumd.runs_for_origin_client(text, text, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION striatumd.runs_missing_origin(text, text)          FROM PUBLIC;

-- (6) runtime-role DML + COLUMN-SCOPED read grants (C2' + C2" composed-route
-- closure). Owner bundles apply only on two-role deploys (where striatumd_rw
-- exists); the guard keeps a hypothetical single-role apply a no-op.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    -- operator_handles: NEW table; column-scoped SELECT from creation EXCLUDING
    -- principal_id (C2" Route 1). Grant exactly the columns the lease walk /
    -- lazy-expiry / heartbeat / stamp subquery read; no DELETE (snapshots never
    -- dangle -- lazy expiry only sets released_at).
    GRANT SELECT (handle_id, leased_session_id, handle, repository_id,
                  released_at, leased_until, last_heartbeat_at)
      ON striatumd.operator_handles TO striatumd_rw;
    GRANT INSERT, UPDATE ON striatumd.operator_handles TO striatumd_rw;

    -- operator_sessions: NEW table; column-scoped SELECT EXCLUDING principal_id
    -- AND client_id (C2'); no DELETE.
    GRANT SELECT (operator_session_id, repository_id, state, registered_at,
                  last_heartbeat_at, expires_at, closed_at, close_reason)
      ON striatumd.operator_sessions TO striatumd_rw;
    GRANT INSERT, UPDATE ON striatumd.operator_sessions TO striatumd_rw;

    -- runs: EXISTING table with table-level runtime SELECT (0005). REVOKE, then
    -- re-GRANT every column EXCEPT created_by_principal_id (C2" Route 2). The
    -- column list is regenerated from the live catalog at authorship time (§C2".2b):
    -- the 0005 baseline 16 columns + completion_mode (migration 0025) +
    -- completion_record_json (migration 0026) + 0022's created_by_handle_id (the
    -- handle FK stays selectable -- it is no identity map on its own), MINUS
    -- created_by_principal_id. The 0025/0026 columns MUST be re-granted or the
    -- run.summary / evidence.export / run-completion-record runtime readers springs
    -- 42501. INSERT(created_by_principal_id) is independent of SELECT and rides the
    -- table-level INSERT (0005), so the run-origin stamp is unaffected; the
    -- write-once trigger reads OLD/NEW (no SELECT privilege needed).
    REVOKE SELECT ON striatumd.runs FROM striatumd_rw;
    GRANT SELECT (
      repository_id, run_id, workflow_snapshot_id, repo_root, state,
      branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at,
      started_at, completed_at, stop_reason, paused_at, paused_reason, cross_repo_run_id,
      completion_mode, completion_record_json,
      created_by_handle_id
    ) ON striatumd.runs TO striatumd_rw;
    -- runs INSERT/UPDATE/DELETE table-level DML is unchanged (0005).

    -- Identity reads ride the DEFINER projections (secret-gated EXECUTE).
    GRANT EXECUTE ON FUNCTION striatumd.run_origin_identity(text, text, text)    TO striatumd_rw;
    GRANT EXECUTE ON FUNCTION striatumd.runs_for_origin_client(text, text, text) TO striatumd_rw;
    GRANT EXECUTE ON FUNCTION striatumd.runs_missing_origin(text, text)          TO striatumd_rw;
  END IF;
END
$$;

-- (7) watermark/capability stamp (mirrors every read-projection bundle, e.g. 0006).
-- requires_daemon_auth = true: the projections are daemon-secret-gated. This stamp
-- keys readScopeReasserts["operator_identity_run_attribution"] (owner.go) so
-- ReassertReadRevokes re-closes all three column gates on drift.
INSERT INTO striatumd.schema_authority(capability, requires_daemon_auth, bundle_version)
VALUES ('operator_identity_run_attribution', true, 22)
ON CONFLICT (capability) DO NOTHING;
