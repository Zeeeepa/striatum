-- RFC 0086: Multi-party conversation
--
-- A symmetric N-party live conversation: N participant sessions, a topic, a
-- round-robin floor, and a turn-ordered shared transcript. This migration
-- records conversation lifecycle + floor state only. Turns reuse the existing
-- message bus (striatumd.queue_messages: kind='agent_message',
-- target_session_id = the next floor-holder, payload_json.conversation_id +
-- payload_json.turn for correlation/threading) so NO change to queue_messages
-- is required — exactly like RFC 0082 interrogations.
--
-- CRITICAL ownership rule (RFC 0079 §5): the runtime role striatumd_rw does NOT
-- own the existing daemon tables, so this is a PLAIN NEW TABLE the runtime role
-- can apply. It does NOT ALTER any owner-held table and declares NO foreign keys
-- referencing owner-held tables (repositories, runs, sessions). Referential
-- integrity is enforced in Go (go/pkg/mutations/conversation.go). DML on the new
-- table is granted to striatumd_rw.

CREATE TABLE IF NOT EXISTS striatumd.conversations (
  repository_id text NOT NULL,
  run_id text NOT NULL,
  conversation_id text NOT NULL,
  topic text,
  -- ordered JSON array of participant session ids (speaking order)
  participants_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  -- index into participants_json of the session whose turn it is
  floor_index integer NOT NULL DEFAULT 0,
  -- completed full rounds (incremented when the floor wraps to index 0)
  round_count integer NOT NULL DEFAULT 0,
  max_rounds integer NOT NULL DEFAULT 3,
  turn_count integer NOT NULL DEFAULT 0,
  state text NOT NULL DEFAULT 'open' CHECK (state IN ('open','closed')),
  opened_at timestamptz NOT NULL,
  closed_at timestamptz,
  PRIMARY KEY (repository_id, conversation_id)
);

CREATE INDEX IF NOT EXISTS idx_conversations_run
  ON striatumd.conversations(repository_id, run_id);
CREATE INDEX IF NOT EXISTS idx_conversations_state
  ON striatumd.conversations(repository_id, state);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.conversations TO striatumd_rw;
  END IF;
END;
$$;
