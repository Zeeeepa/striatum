-- RFC 0094 §1 / RFC 0095 Phase 3: conversation post_dialog_hook
--
-- A declarative field on the RFC 0086 conversation fixture that makes
-- conversation close emit one work packet to a named coordinator session
-- BEFORE the participant lanes' preserved-context window is released, carrying
-- the participant session ids + a transcript reference. It closes the liveness
-- race that blocks RFC 0094 synaptic_prune (the prune fan-out must reach still-
-- live participants) and is the same "keep participants live through a gate"
-- mechanism RFC 0095 Phase 3 reuses for review panels.
--
-- This migration persists the hook DECLARATION in a SIDECAR table keyed on the
-- conversation so it is available at close time on both the explicit-close and
-- the auto-close-at-max_rounds paths (the close transaction reads it back). The
-- emit itself reuses the existing queue_messages delivery path
-- (kind='agent_message' targeted at the coordinator session) — exactly like an
-- RFC 0082 interrogation question — so NO new daemon method or owner-held table
-- is touched.
--
-- Ownership rule (RFC 0079 §5 / D215): post-RFC-0079 a regular runtime migration
-- must NOT ALTER or DROP any existing striatumd.* table (owner-held tables change
-- only through an owner bundle, and even runtime-owned tables are not altered in
-- place to keep the future-runtime-DDL guard simple). New state therefore lands
-- in a PLAIN NEW TABLE. This sidecar declares NO foreign keys into owner-held
-- tables (repositories, runs, sessions, jobs) — referential integrity for the
-- conversation and the declared deliver_to session is enforced in Go
-- (go/pkg/mutations/conversation.go) — and carries no CHECK constraint that would
-- require an owner bundle. DML on the new table is granted to striatumd_rw.

CREATE TABLE IF NOT EXISTS striatumd.conversation_post_dialog_hooks (
  repository_id text NOT NULL,
  conversation_id text NOT NULL,
  run_id text NOT NULL,
  -- the coordinator session that receives the close-time work packet
  deliver_to text NOT NULL,
  -- opaque packet-type label echoed into the emitted packet (e.g. 'prune')
  packet_type text NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (repository_id, conversation_id)
);

CREATE INDEX IF NOT EXISTS idx_conversation_post_dialog_hooks_run
  ON striatumd.conversation_post_dialog_hooks(repository_id, run_id);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE, DELETE ON striatumd.conversation_post_dialog_hooks TO striatumd_rw;
  END IF;
END;
$$;
