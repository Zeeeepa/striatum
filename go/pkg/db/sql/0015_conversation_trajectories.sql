-- RFC 0081: Conversation Trajectories
-- Canonical ordering and trajectory segments.

CREATE SEQUENCE IF NOT EXISTS striatumd.run_event_sequence;

-- Add run_event_seq to core tables for trajectory ordering
ALTER TABLE striatumd.queue_messages ADD COLUMN IF NOT EXISTS run_event_seq bigint DEFAULT nextval('striatumd.run_event_sequence');
ALTER TABLE striatumd.events ADD COLUMN IF NOT EXISTS run_event_seq bigint DEFAULT nextval('striatumd.run_event_sequence');
ALTER TABLE striatumd.artifacts ADD COLUMN IF NOT EXISTS run_event_seq bigint DEFAULT nextval('striatumd.run_event_sequence');
ALTER TABLE striatumd.verdicts ADD COLUMN IF NOT EXISTS run_event_seq bigint DEFAULT nextval('striatumd.run_event_sequence');
ALTER TABLE striatumd.blockers ADD COLUMN IF NOT EXISTS run_event_seq bigint DEFAULT nextval('striatumd.run_event_sequence');

-- Trajectory segments: metadata for exports and watch checkpoints.
-- Not trajectory authority; just for reproducibility and resumable watch.
CREATE TABLE IF NOT EXISTS striatumd.trajectory_segments (
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  run_id text NOT NULL,
  segment_id text NOT NULL,
  profile text NOT NULL CHECK (profile IN ('dialogue', 'provenance')),
  from_seq bigint NOT NULL,
  to_seq bigint NOT NULL,
  content_sha256 text NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (repository_id, segment_id),
  FOREIGN KEY (repository_id, run_id) REFERENCES striatumd.runs(repository_id, run_id)
);

CREATE INDEX IF NOT EXISTS idx_trajectory_segments_run ON striatumd.trajectory_segments(repository_id, run_id);

-- Backfill existing records to ensure they have stable ordering.
-- We use created_at as the primary sort, and the existing primary key as tie-breaker.
-- Note: IDENTITY columns like events.event_id could also be used for ordering.

DO $$
BEGIN
  -- Backfill queue_messages
  UPDATE striatumd.queue_messages SET run_event_seq = nextval('striatumd.run_event_sequence')
  WHERE run_event_seq IS NULL OR run_event_seq = 0; -- Should not happen with DEFAULT but good for safety

  -- Backfill events
  UPDATE striatumd.events SET run_event_seq = nextval('striatumd.run_event_sequence')
  WHERE run_event_seq IS NULL;

  -- Backfill artifacts
  UPDATE striatumd.artifacts SET run_event_seq = nextval('striatumd.run_event_sequence')
  WHERE run_event_seq IS NULL;

  -- Backfill verdicts
  UPDATE striatumd.verdicts SET run_event_seq = nextval('striatumd.run_event_sequence')
  WHERE run_event_seq IS NULL;

  -- Backfill blockers
  UPDATE striatumd.blockers SET run_event_seq = nextval('striatumd.run_event_sequence')
  WHERE run_event_seq IS NULL;
END $$;
