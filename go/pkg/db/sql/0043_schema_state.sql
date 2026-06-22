-- RFC 0142 Layer 3 part 1 (P3 / #570): schema-fingerprint drift gate — record the
-- last successfully-applied deploy plan so the serving daemon can compare what it
-- EXPECTS (the ordered runtime-migration + owner-bundle SHA set its binary embeds,
-- ExpectedFingerprint) against what is LIVE in the database (LiveFingerprint).
--
-- This is Layer 3 PART ONE only: it ships the loud, test-covered drift DETECTION
-- gate WITHOUT yet moving where DDL runs (the one-shot `striatum daemon deploy`
-- coordinator is P4). The serving daemon still auto-applies migrations on boot; on
-- a successful apply it self-records its ExpectedFingerprint into this table, so a
-- correctly-migrated daemon always reads Live == Expected and never false-halts.
-- Divergence is surfaced as a `schema_drift` condition — by default a LOUD WARNING
-- that boots anyway (shadow mode), and only a hard refuse-to-serve when the
-- operator opts in with STRIATUM_SCHEMA_DRIFT_REFUSE=1. The shadow-first default is
-- the codebase convention for a risky new boot gate (cf. STRIATUM_AUTO_SPAWN_-
-- SCHEDULER): a false-positive fingerprint must NOT auto-cause an outage on the
-- next restart simply by landing to main.
--
-- Ownership-safety (RFC 0079 §5 / D187 / D248 / the migration owner-table trap):
-- the daemon applies runtime migrations as the runtime role `striatumd_rw`, which
-- cannot ALTER/DROP owner-held tables, and regular migrations >= 27 must carry NO
-- owner-table DDL at all (TestFutureRuntimeMigrationsDoNotCarryOwnerDDL). So this
-- migration ONLY creates a NEW runtime-owned table and declares NO foreign keys
-- (it references nothing — mirrors 0028 job_workspaces / 0042 verifier_attestations
-- / 0023 principals). It is a RUNTIME migration (not an owner bundle) because the
-- drift gate reads it on EVERY boot: the table must exist for every daemon from
-- first boot, or the boot-time read would error against a table that does not exist
-- until `owner-ddl apply`.
--
-- Single-row contract: the table records at most ONE row, the fingerprint of the
-- last successfully-applied deploy plan, keyed by a constant. The self-record path
-- UPSERTs that row after ApplyMigrations succeeds. `fingerprint` is the sha256 hex
-- of the ordered migration+bundle SHA set (ExpectedFingerprint); `applied_at`
-- stamps when this daemon recorded it.
--
-- The GRANT block makes the table writable+readable by the runtime role (broad-DML
-- migration 0005 ran before this table existed, so a fresh table needs its own
-- grant; pgtest masks a missing GRANT, so it is mandatory).

CREATE TABLE IF NOT EXISTS striatumd.schema_state (
  id            text PRIMARY KEY DEFAULT 'singleton' CHECK (id = 'singleton'),
  fingerprint   text NOT NULL,
  daemon_version text,
  applied_at    timestamptz NOT NULL DEFAULT now()
);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE ON striatumd.schema_state TO striatumd_rw;
  END IF;
END
$$;
