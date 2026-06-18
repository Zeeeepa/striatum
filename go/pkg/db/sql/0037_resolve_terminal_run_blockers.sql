-- #420 — resolve blockers stranded 'open' on terminal runs (one-time backfill DML).
--
-- Open blockers are resolved on a state transition (recovery.resolve_blocker,
-- checkpoint.resolve, escalation.resolve, the job-retry path's
-- reopenJobForAttempt), but no path resolved a run's open blockers when the RUN
-- itself reached a terminal state. So a blocker (including human_checkpoint /
-- escalation-class) on a canceled/completed/failed run lingered 'open' forever:
-- the #420 incident found 38 such rows across terminal runs, ages up to ~21 days,
-- reading as pending operator work. The #419 read-side scoping hid them from the
-- status frontier; the forward fix (resolveTerminalRunOpenBlockers, called from
-- the terminal cleanup path) resolves them going forward. This migration cleans
-- the already-accumulated rows the forward fix cannot retroactively reach.
--
-- The adjudication obligation a human_checkpoint / escalation-class blocker
-- carries is moot once its run is terminal, so resolving it records honest
-- provenance (resolved, not a forged decision). The escalation_inbox mirror is
-- updated FIRST (while the blockers are still 'open' so the join matches), then
-- the blockers themselves, keeping the two surfaces consistent.
--
-- Ownership-safety: pure DML (UPDATE) against runtime-owned tables (striatumd_rw
-- writes blockers / escalation_inbox during normal operation); ALTERs/DROPs
-- nothing, touches no owner-held table. Idempotent: re-running finds no 'open'
-- blocker on a terminal run.

UPDATE striatumd.escalation_inbox ei
   SET state = 'resolved', resolved_at = now()
  FROM striatumd.blockers b
  JOIN striatumd.runs r
    ON r.repository_id = b.repository_id AND r.run_id = b.run_id
 WHERE ei.repository_id = b.repository_id
   AND ei.escalation_id = b.blocker_id
   AND ei.state <> 'resolved'
   AND b.state = 'open'
   AND r.state IN ('completed', 'failed', 'canceled');

UPDATE striatumd.blockers b
   SET state = 'resolved', resolved_at = now()
  FROM striatumd.runs r
 WHERE r.repository_id = b.repository_id
   AND r.run_id = b.run_id
   AND b.state = 'open'
   AND r.state IN ('completed', 'failed', 'canceled');
