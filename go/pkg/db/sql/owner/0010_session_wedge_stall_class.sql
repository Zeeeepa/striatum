-- GH #324 — owner bundle 0010: wedged_no_tool_progress liveness stall class.
--
-- striatumd.sessions is an OWNER-HELD table in the two-role posture (owned by the
-- database owner, not the runtime role striatumd_rw). The liveness stall class is
-- persisted under the sessions_liveness_stall_class_check CHECK constraint, first
-- added by runtime migration 0012 back when the sessions table ALTERs were still
-- applied out-of-band as the owner. A CHECK constraint cannot be widened in place,
-- so adding a new permitted value means DROP + re-ADD the constraint — owner-table
-- DDL the runtime role cannot perform (`must be owner of table sessions`; the
-- RFC 0079 §5 / RFC 0081 owner-held ALTER crash-loop). It therefore lives in this
-- owner/admin bundle rather than a regular runtime migration, and the runtime
-- guard test TestFutureRuntimeMigrationsDoNotCarryOwnerDDL keeps it out of the
-- runtime tree.
--
-- The new value `wedged_no_tool_progress` (sessionliveness.StallToolProgress) is
-- recorded for a lane that is alive at the PTY layer (a spinner keeps
-- last_pty_activity_at fresh) but has made NO tool-call progress for
-- ToolProgressSeconds while holding an active lease/job — the #324
-- lost-its-daemon-endpoint wedge. Without this value the liveness sweep cannot
-- persist the new class and every UPDATE that tried to would fail the CHECK.
--
-- Idempotent: the DROP is IF EXISTS and the ADD is guarded by a pg_constraint
-- definition probe, so a re-run (or applying onto a DB an operator has already
-- hand-patched) is a safe no-op. ApplyOwnerBundles runs the whole bundle in one
-- transaction and stamps owner_bundle_meta last, so the constraint swap is atomic.

DO $$
BEGIN
  -- Only rebuild the constraint when it does not already permit the new value.
  -- pg_get_constraintdef renders the full CHECK text; probing it keeps the swap
  -- idempotent without parsing the IN-list.
  IF EXISTS (
    SELECT 1
      FROM pg_constraint
     WHERE conname = 'sessions_liveness_stall_class_check'
       AND conrelid = 'striatumd.sessions'::regclass
       AND pg_get_constraintdef(oid) NOT LIKE '%wedged_no_tool_progress%'
  ) THEN
    ALTER TABLE striatumd.sessions
      DROP CONSTRAINT sessions_liveness_stall_class_check;
  END IF;

  IF NOT EXISTS (
    SELECT 1
      FROM pg_constraint
     WHERE conname = 'sessions_liveness_stall_class_check'
       AND conrelid = 'striatumd.sessions'::regclass
  ) THEN
    ALTER TABLE striatumd.sessions
      ADD CONSTRAINT sessions_liveness_stall_class_check
      CHECK (
        liveness_stall_class IS NULL OR liveness_stall_class IN (
          'agent_mcp_discovery_stall',
          'agent_await_packet_stall',
          'agent_ack_stall',
          'agent_lease_heartbeat_stall',
          'agent_question_pending',
          'agent_escalation_pending',
          'agent_protocol_idle_stall',
          'wedged_no_tool_progress'
        )
      );
  END IF;
END
$$;
