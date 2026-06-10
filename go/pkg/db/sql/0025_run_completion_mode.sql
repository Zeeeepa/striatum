-- RFC 0118 P0-4 (GH #240): record HOW a completed run completed.
--
--   completion_mode  'lanes_attested'    every provenance-required review
--                                        gate cleared on a frozen attested
--                                        stamp (RFC 0118 P0-1/P0-3)
--                    'operator_override' at least one gate cleared by an
--                                        operator/recovery override basis
--
-- Deliberately a nullable column on runs, NOT a new terminal state: a new
-- state would force changes to runs_state_check (0007/0021) and to every
-- terminal-state filter (e.g. the supervision sweep), silently excluding
-- override runs from terminal sweeps. Pre-migration completed runs keep
-- NULL (mode unknown — the frozen verdict stamps, where present, remain the
-- source of truth). Per the operator decision on RFC 0118 open question #2,
-- completion_mode is ADVISORY metadata: downstream publish/acceptance gates
-- that care must read completion_mode explicitly, not just state='completed'.

ALTER TABLE striatumd.runs
  ADD COLUMN IF NOT EXISTS completion_mode text;
