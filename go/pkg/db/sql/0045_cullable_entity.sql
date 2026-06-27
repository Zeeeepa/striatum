-- RFC 0170 P0 / D271: observe-only Tier-1 self-culling candidacy ledger.
--
-- This is a runtime-owned table, not an owner bundle. It creates only one new
-- striatumd-owned table, declares no foreign keys, and carries no owner-held
-- table DDL, so it stays inside the >=27 runtime-migration ownership guard.
--
-- P0 writes candidacy observations only. Nothing consumes this table to delete,
-- tombstone, page, affect doctor, or affect run admission. Withdrawal is an
-- UPDATE of candidacy_state; there is intentionally no DELETE grant.

CREATE TABLE IF NOT EXISTS striatumd.cullable_entity (
  kind                 text NOT NULL,
  ref                  text NOT NULL,
  last_reinforced_at   timestamptz,
  decay_score          double precision NOT NULL,
  reachable_from_root  boolean NOT NULL,
  candidacy_state      text NOT NULL,
  PRIMARY KEY (kind, ref),
  CHECK (kind IN ('code_symbol','file','package','branch','rfc','decision','doc','table')),
  CHECK (candidacy_state IN ('nominated','withdrawn'))
);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE ON striatumd.cullable_entity TO striatumd_rw;
  END IF;
END
$$;
