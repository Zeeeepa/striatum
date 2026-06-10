-- RFC 0118 P1-5 (GH #240): the durable run_completion_record.
--
-- The run_45aa8852 retrospective could not answer "what was alive when this
-- run ended?" — closeRemainingSessions had already transitioned the
-- supervisor rows and the read side filters terminal runs, so every probe
-- read the post-teardown emptiness. The terminal-transition transaction now
-- snapshots a machine-readable record (per-required-job state+attempt, the
-- frozen verdict provenance stamps, per-session attestation + close reason,
-- lease state, recovery-event summary) BEFORE the teardown, into this
-- write-once column (operator decision #3: a runs-aggregate audit property,
-- NOT a new artifact_kind — a daemon-authored, session-less record satisfies
-- none of the artifact invariants).
--
-- Tamper evidence comes from the append-only event log, not this column: the
-- terminal event (run.completed / run.canceled / run.failed) carries
-- completion_record_sha256 over the canonical JSON; a retrospective verifies
-- the column by re-hashing against the immutable event. Overwrite is guarded
-- in code (UPDATE ... WHERE completion_record_json IS NULL).

ALTER TABLE striatumd.runs
  ADD COLUMN IF NOT EXISTS completion_record_json jsonb;
