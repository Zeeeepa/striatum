ALTER TABLE striatumd.sessions
  ADD COLUMN IF NOT EXISTS last_mcp_request_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_tools_list_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_await_packet_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_packet_delivered_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_ack_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_work_block_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_work_release_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_work_complete_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_work_heartbeat_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_session_ready_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_session_heartbeat_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_session_question_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_session_escalate_at timestamptz,
  ADD COLUMN IF NOT EXISTS liveness_stall_class text,
  ADD COLUMN IF NOT EXISTS liveness_stall_since timestamptz;

DO $$
BEGIN
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
          'agent_protocol_idle_stall'
        )
      );
  END IF;
END;
$$;

CREATE INDEX IF NOT EXISTS idx_sessions_mcp_liveness
  ON striatumd.sessions(repository_id, run_id, liveness_stall_class)
  WHERE liveness_stall_class IS NOT NULL;
