-- RFC 0122: spawn-authorization grants for the daemon supervision.auto_spawn scheduler.
--
-- supervision.auto_spawn (#212) lets the daemon scheduler spawn a run's lanes
-- with no contemporaneous operator RPC. The authorizing principal is the RUN
-- OWNER, captured at run.start and REPLAYED by the scheduler — never a synthetic
-- principal (RFC 0122 §2). This table is that captured, durable, run-scoped,
-- revocable grant.
--
-- Ownership-safety (RFC 0079 §5 / the migration owner-table trap): like
-- 0023_principals.sql, this migration ONLY creates a NEW table (which the
-- runtime role owns) and NEVER alters an owner-held table, so it applies cleanly
-- as striatumd_rw and cannot crash-loop the daemon. It is deliberately a runtime
-- migration rather than an owner bundle: the terminal-finalization path
-- (freezeRunCompletionRecord) revokes a run's grant on EVERY terminal transition,
-- so the table must exist for every daemon from first boot — an owner-bundle-only
-- table would be absent until `owner-ddl apply` and abort those transactions.
--
-- owner_principal_id is a bare text column with NO foreign key to
-- striatumd.principals — mirroring principal_clients.client_id / audit_log.client_id
-- (referential integrity enforced in Go, and a FK REFERENCES on an owner-gated
-- table is exactly the trap). It holds the attribution identity the authority
-- prelude installs as striatum.principal_id (today, the run owner's client id), so
-- a scheduler spawn that replays it is byte-for-byte the attribution a manual
-- supervise.start would have produced (C1 attestation parity). run_as_spec and
-- capability_envelope are the run-as identity and the session-token capability set
-- the scheduler mint anchors on (RFC 0122 §3/§4), captured verbatim so the
-- scheduler reads, never chooses.
--
-- A grant is bounded by expires_at and revoked by revoked_at (run terminal /
-- run.stop / explicit revoke). After expiry/revocation the scheduler refuses to
-- spawn and escalates loud — never a silent fallback (C2 no authority invention).
--
-- The GRANT block makes the table writable by the runtime role (broad-DML
-- migration 0005 ran before this table existed, so a fresh table needs its own
-- grant); pgtest masks a missing GRANT, so it is mandatory.

CREATE TABLE IF NOT EXISTS striatumd.spawn_authorization_grants (
  repository_id       text NOT NULL,
  grant_id            text PRIMARY KEY,
  run_id              text NOT NULL,
  owner_principal_id  text NOT NULL,
  run_as_spec         jsonb NOT NULL,
  capability_envelope jsonb NOT NULL,
  created_at          timestamptz NOT NULL DEFAULT now(),
  expires_at          timestamptz,
  revoked_at          timestamptz,
  revoke_reason       text
);

-- At most one ACTIVE (un-revoked) grant per run: the scheduler resolves "the
-- grant for this run" unambiguously, and a re-capture cannot stack two live
-- grants.
CREATE UNIQUE INDEX IF NOT EXISTS uq_striatumd_spawn_grants_active_run
  ON striatumd.spawn_authorization_grants (repository_id, run_id)
  WHERE revoked_at IS NULL;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE ON striatumd.spawn_authorization_grants TO striatumd_rw;
  END IF;
END
$$;
