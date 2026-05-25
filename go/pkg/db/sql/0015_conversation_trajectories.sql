-- RFC 0081: Conversation Trajectories
--
-- Per the converged design (docs/operator/artifacts/two-model-conversation/),
-- the trajectory is a READ MODEL over existing daemon-owned records: ordering is
-- derived at read time from created_at plus a per-source tie-breaker. This
-- migration adds NO new authority and does NOT alter existing tables — those are
-- owner-restricted (owned outside striatumd_rw) and a second write target would
-- only invite divergence. The only new object is trajectory_segments, which holds
-- export manifests / watch checkpoints for reproducibility and resumable watch.

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
