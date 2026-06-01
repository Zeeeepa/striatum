ALTER TABLE striatumd.sessions
  ADD COLUMN IF NOT EXISTS last_pty_activity_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_tool_call_started_at timestamptz,
  ADD COLUMN IF NOT EXISTS last_tool_call_finished_at timestamptz;
