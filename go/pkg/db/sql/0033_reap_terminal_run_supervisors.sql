-- #417 — reap supervisors stranded on terminal runs (one-time backfill DML).
--
-- Supervisor rows (process_supervisors, daemon_supervisors) and their
-- live-pointer rows (process_supervisor_pointers) are reaped to a terminal state
-- only via closeRemainingSessions, which iterates sessions in state='active'.
-- When a lane dies abnormally its session is already 'closed' by the time the run
-- terminates, so its supervisor is never reached and lingers in a live state
-- ('starting'/'attached'/'detached') forever. The status / dashboard / supervise
-- read path LEFT-JOINs process_supervisors ON state='attached' and then runs a
-- sudo+tmux ProbeLaneLiveness per attached supervisor; with hundreds of these
-- stranded rows accumulated across the daemon's history a single repo-wide
-- `status` fans out hundreds of failing sudo+tmux probes and blows the CLI read
-- deadline (the 30s timeout in the #417 incident: 562 stranded process_supervisors
-- across 128 terminal runs, 511 of them with an already-closed session).
--
-- The forward fix is a run-scoped reap backstop in closeRemainingSessions (it now
-- reaps every still-live supervisor for a now-terminal run regardless of its
-- session's state or lease, not just active sessions). This migration cleans the
-- already-accumulated debris that the backstop cannot retroactively reach (those
-- runs terminated before the fix landed). It only marks already-dead supervisors
-- terminal — no process teardown is needed because the processes are long gone.
--
-- Ownership-safety: this is pure DML (UPDATE) against runtime-owned tables
-- (striatumd_rw owns process_supervisors / daemon_supervisors /
-- process_supervisor_pointers and updates them on every heartbeat), so it applies
-- cleanly as the runtime role and ALTERs/DROPs nothing. It does not touch any
-- owner-held table and declares no schema change. Idempotent: re-running finds no
-- rows still in a live state on a terminal run. 'lost' is intentionally left
-- untouched (it is already a recognized dead state and is not probed by the
-- status 'attached' join; recovery owns its semantics).

UPDATE striatumd.process_supervisors ps
   SET state = 'stopped',
       ended_at = COALESCE(ps.ended_at, now()),
       stop_reason = COALESCE(ps.stop_reason, 'run_terminal_backfill_417'),
       heartbeat_at = now()
  FROM striatumd.runs r
 WHERE r.repository_id = ps.repository_id
   AND r.run_id = ps.run_id
   AND r.state IN ('completed', 'failed', 'canceled')
   AND ps.state IN ('starting', 'attached', 'detached');

UPDATE striatumd.daemon_supervisors ds
   SET state = 'stopped',
       ended_at = COALESCE(ds.ended_at, now()),
       stop_reason = COALESCE(ds.stop_reason, 'run_terminal_backfill_417'),
       heartbeat_at = now()
  FROM striatumd.runs r
 WHERE r.repository_id = ds.repository_id
   AND r.run_id = ds.run_id
   AND r.state IN ('completed', 'failed', 'canceled')
   AND ds.state IN ('starting', 'attached', 'detached');

UPDATE striatumd.process_supervisor_pointers ptr
   SET state = 'stopped',
       updated_at = now()
  FROM striatumd.runs r
 WHERE r.repository_id = ptr.repository_id
   AND r.run_id = ptr.run_id
   AND r.state IN ('completed', 'failed', 'canceled')
   AND ptr.state IN ('starting', 'attached', 'detached');
