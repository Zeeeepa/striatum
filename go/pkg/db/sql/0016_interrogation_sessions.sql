-- RFC 0082: Interrogation Sessions
--
-- A bounded, peer-addressed, multi-turn Q&A bound to a live target worker
-- session whose context is preserved. This migration records interrogation
-- lifecycle + correlation only. Turns reuse the existing message bus
-- (striatumd.queue_messages: kind='agent_message', target_session_id set,
-- payload_json.interrogation_id for correlation/threading) so NO change to
-- queue_messages is required.
--
-- CRITICAL ownership rule (RFC 0079 §5 / the RFC 0081 trajectory incident):
-- the runtime role striatumd_rw does NOT own the existing daemon tables, so
-- this migration is a PLAIN NEW TABLE the runtime role can apply. It does NOT
-- ALTER any owner-held table and it does NOT declare foreign keys referencing
-- owner-held tables (repositories, runs, sessions). Referential integrity is
-- enforced in Go (go/pkg/mutations/interrogation.go). DML on the new table is
-- granted to striatumd_rw. Required Test 9 guards this property.

CREATE TABLE IF NOT EXISTS striatumd.interrogations (
  repository_id text NOT NULL,
  run_id text NOT NULL,
  interrogation_id text NOT NULL,
  interrogator_session_id text NOT NULL,
  target_session_id text NOT NULL,
  topic text,
  state text NOT NULL DEFAULT 'open' CHECK (state IN ('open','closed')),
  turn_count integer NOT NULL DEFAULT 0,
  opened_at timestamptz NOT NULL,
  closed_at timestamptz,
  PRIMARY KEY (repository_id, interrogation_id)
);

CREATE INDEX IF NOT EXISTS idx_interrogations_run
  ON striatumd.interrogations(repository_id, run_id);
CREATE INDEX IF NOT EXISTS idx_interrogations_target
  ON striatumd.interrogations(repository_id, target_session_id, state);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.interrogations TO striatumd_rw;
  END IF;
END;
$$;
