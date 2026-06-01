-- RFC 0096 V2 / GH #135: per-session capability-token binding.
--
-- A capability grant may be bound to a single session (minted at
-- session.register for one lane) or left session-unbound (the repo-scoped
-- operator/coordinator grant, as before). Session-scoped handlers
-- (interrogation.answer and the work.* family) reject a bound token acting as
-- a different session, closing the cross-session impersonation vector; an
-- unbound (operator) token is still allowed but records an honest
-- operator-override provenance marker on the turn/event.
--
-- Additive, forward-only: a nullable column with NULL = "session-unbound"
-- preserves every existing grant's behavior. ADD COLUMN IF NOT EXISTS is
-- idempotent so an owner-applied replay is safe.
ALTER TABLE striatumd.client_capabilities
  ADD COLUMN IF NOT EXISTS session_id text;
