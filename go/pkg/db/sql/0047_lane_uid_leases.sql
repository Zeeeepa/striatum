-- RFC 0168 P0 / D272: host-global per-lane uid lease ledger.
--
-- The pool is a host-provisioned OS-user resource. PostgreSQL owns the
-- daemon's durable view of which uid is active, scrubbing, quarantined, or
-- returned; the daemon never derives allocatability from in-memory state.
-- Runtime migrations own this table because the runtime role is the single
-- writer for lane lifecycle state.

CREATE TABLE IF NOT EXISTS striatumd.lane_uid_leases (
  lease_id           text PRIMARY KEY,
  pool_uid           integer NOT NULL,
  pool_user          text NOT NULL,
  generation         bigint NOT NULL,
  repository_id      text NOT NULL,
  run_id             text NOT NULL,
  session_id         text NOT NULL,
  supervisor_id      text NOT NULL,
  state              text NOT NULL CHECK (state IN (
    'active',
    'scrubbing',
    'quarantined',
    'returned'
  )),
  scrub_status       text CHECK (scrub_status IN ('clean','failed')),
  scrub_proof        jsonb,
  scrub_failure      text,
  leased_at          timestamptz NOT NULL,
  scrub_started_at   timestamptz,
  returned_at        timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_lane_uid_held
  ON striatumd.lane_uid_leases(pool_uid)
  WHERE state IN ('active','scrubbing','quarantined');

CREATE INDEX IF NOT EXISTS idx_lane_uid_leases_session
  ON striatumd.lane_uid_leases(repository_id, session_id, state);

CREATE INDEX IF NOT EXISTS idx_lane_uid_leases_supervisor
  ON striatumd.lane_uid_leases(repository_id, supervisor_id, state);

CREATE INDEX IF NOT EXISTS idx_lane_uid_leases_recovery
  ON striatumd.lane_uid_leases(state, leased_at, scrub_started_at);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE ON striatumd.lane_uid_leases TO striatumd_rw;
    REVOKE DELETE ON striatumd.lane_uid_leases FROM striatumd_rw;
  END IF;
END
$$;
