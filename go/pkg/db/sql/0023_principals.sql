-- RFC 0107: multi-principal trust model — make multi-user explicit (self-hosted, not SaaS).
--
-- A "principal" is a named identity (a human, an AI operator, or a service)
-- that holds capability tokens. It sits ABOVE the existing `clients` /
-- `client_capabilities` substrate: a principal owns one or more clients, and
-- each client (with its per-repository, optionally session-bound capability
-- grants) is the wire credential. Token rotation mints a NEW client row, so the
-- principal->client link is what keeps a principal's identity stable across
-- rotation and what lets the daemon-global audit chain (audit_log.client_id)
-- attribute every mutation back to a principal.
--
-- This generalizes RFC 0053's single escalation-only human principal to SEVERAL
-- humans, and reuses RFC 0028 per-repository capability scoping +
-- RFC 0096 V2 session-binding unchanged as the cross-principal/cross-repo
-- isolation mechanism. NO hosted control plane, tenant provisioning, external
-- IdP, or telemetry is introduced — principals are local capability grants on
-- the operator-owned daemon + PostgreSQL.
--
-- Ownership-safety (RFC 0079 §5 / the migration owner-table trap): the daemon
-- applies migrations as the runtime role `striatumd_rw`, which CANNOT ALTER the
-- owner-held tables (`clients`, `client_capabilities`, ...). Therefore this
-- migration ONLY creates NEW tables (which the runtime role owns) and NEVER
-- alters an owner table. `principal_clients.client_id` is a bare text column
-- with NO foreign key to `striatumd.clients` — exactly like `audit_log.client_id`
-- — because a FK REFERENCES privilege on an owner table is the blocker the trap
-- describes; referential integrity is enforced in Go. The GRANT block makes the
-- tables usable when the migration is owner-applied (pgtest masks a missing
-- GRANT, so it is mandatory).

CREATE TABLE IF NOT EXISTS striatumd.principals (
  principal_id text PRIMARY KEY,
  principal_kind text NOT NULL CHECK (principal_kind IN ('human', 'ai_operator', 'service')),
  display_name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  disabled_at timestamptz
);

-- Links a principal to the clients it owns. A client_id appears in exactly one
-- ACTIVE link (unlinked_at IS NULL); rotation links the replacement client to
-- the same principal. No FK to striatumd.clients (owner-held) — see the header.
CREATE TABLE IF NOT EXISTS striatumd.principal_clients (
  principal_id text NOT NULL REFERENCES striatumd.principals(principal_id),
  client_id text NOT NULL,
  linked_at timestamptz NOT NULL DEFAULT now(),
  unlinked_at timestamptz,
  PRIMARY KEY (principal_id, client_id)
);

-- One principal per client while the link is active: a client cannot belong to
-- two principals at once, which keeps audit attribution unambiguous.
CREATE UNIQUE INDEX IF NOT EXISTS uq_striatumd_principal_clients_active_client
  ON striatumd.principal_clients (client_id)
  WHERE unlinked_at IS NULL;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.principals TO striatumd_rw;
    GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.principal_clients TO striatumd_rw;
  END IF;
END
$$;
