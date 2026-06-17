-- RFC 0135 P3 (D216) — the barrier_status read/audit view (#347). P3 ships the
-- doctor + refusal + verify surface for the sealed expectation barrier; this view
-- is the single read projection over the three barrier tables (the immutable
-- freeze record, the attempt-addressed staging contributions, and the
-- assembly-journal barrier_state) that the doctor invariant, `striatum join verify`,
-- and any status/audit consumer read.
--
-- Ownership-safety (D215 / RFC 0079 §5 / the migration owner-table trap): this
-- migration is a RUNTIME migration (next number 0031). CREATE VIEW is runtime-safe
-- — it only creates a NEW view over runtime-owned tables (striatumd.barrier_state,
-- striatumd.fanin_freeze_points, striatumd.barrier_staged_contributions, all owned
-- by striatumd_rw via migrations 0029/0030) and the owner-held striatumd.jobs which
-- the runtime role already has SELECT on. It does NOT ALTER or DROP any owner-held
-- table (the future-runtime-DDL guard TestFutureRuntimeMigrationsDoNotCarryOwnerDDL
-- forbids ALTER/DROP of striatumd.* for versions >= 27 outright; CREATE VIEW is not
-- matched by that guard), so it applies cleanly as striatumd_rw and cannot
-- crash-loop a two-role production daemon (D187 / #244). No owner bundle is needed.
--
-- NO new base table, so there is no FK-to-owner-table concern; a view carries no
-- referential keys. The view JOINs the runtime barrier tables to the owner-held
-- jobs table at READ time (the live-seal JOIN), which is exactly the in-Go
-- referential-integrity story P1/P2 use — not a stored FK.
--
-- EXPLICIT GRANT is mandatory (pgtest masks an omitted grant): the runtime role
-- striatumd_rw must be able to SELECT the view. Guarded by an IF EXISTS probe so
-- the single-role pgtest harness (where striatumd_rw is absent) is a clean no-op.

-- striatumd.barrier_status: one row per known barrier (a barrier is "known" once a
-- freeze record exists). It projects the freeze record, the assembly-journal state,
-- per-seat staging counts, and a derived `condition` the doctor invariant and the
-- join-verify verb consume. The derived condition is purely advisory legibility —
-- the authoritative readiness predicate stays db.BarrierReadySQL / the Go barrier
-- evaluator; this view never replaces it, it summarizes the durable rows.
CREATE OR REPLACE VIEW striatumd.barrier_status AS
WITH seat AS (
    -- One row per DECLARED sibling seat, with its live completed attempt (the live
    -- seal), whether it has a live-seal staged contribution, whether it is a
    -- terminal gap (quarantined), and whether it is a LIVE BLOCKING seat (a job that
    -- cannot become satisfied without intervention: blocked / waiting_human / failed
    -- state, or an open blocking blocker). The classification is keyed on the stable
    -- seat (workflow_job_id), never on the churning job_id (RFC 0135).
    SELECT fp.repository_id,
           fp.barrier_id,
           fp.run_id,
           sib.value::text AS workflow_job_id,
           -- The live seal: MAX completed attempt for the seat (0 if none completed).
           COALESCE((
             SELECT MAX(j.attempt) FROM striatumd.jobs j
              WHERE j.repository_id = fp.repository_id
                AND j.run_id = fp.run_id
                AND j.workflow_job_id = sib.value::text
                AND j.state = 'completed'
           ), 0) AS live_seal,
           -- Is the seat a terminal gap? (canceled + recorded quarantine marker.)
           COALESCE((
             SELECT bool_or(j.state = 'canceled'
                            AND COALESCE(j.write_scope_json->>'quarantined','') = 'true')
               FROM striatumd.jobs j
              WHERE j.repository_id = fp.repository_id
                AND j.run_id = fp.run_id
                AND j.workflow_job_id = sib.value::text
           ), false) AS is_terminal_gap,
           -- Is the seat LIVE-BLOCKING? A job in a blocking, non-terminal, non-
           -- complete state (blocked / waiting_human / failed) or carrying an open
           -- blocking/human_checkpoint blocker is a live in-edge the barrier cannot
           -- fire past — distinct from a clean terminal gap. A `stale_lease` /
           -- `running` / `claimed` seat is merely slow, NOT blocking (silence from a
           -- live lane is not consent — RFC 0132 D214b — but slowness is not a
           -- BARRIER_BLOCKED condition either; only an unresolvable in-edge is).
           COALESCE((
             SELECT bool_or(j.state IN ('blocked','waiting_human','failed'))
               FROM striatumd.jobs j
              WHERE j.repository_id = fp.repository_id
                AND j.run_id = fp.run_id
                AND j.workflow_job_id = sib.value::text
           ), false)
           OR EXISTS (
             SELECT 1 FROM striatumd.blockers b
               JOIN striatumd.jobs j
                 ON j.repository_id = b.repository_id AND j.job_id = b.job_id
              WHERE b.repository_id = fp.repository_id
                AND b.run_id = fp.run_id
                AND j.workflow_job_id = sib.value::text
                AND b.state = 'open'
                AND b.severity IN ('blocked','human_checkpoint')
           ) AS is_blocking,
           -- Does a live-seal staged contribution exist for the seat? (highest-
           -- attempt non-voided, non-recovery staging row matches the live seal.)
           EXISTS (
             SELECT 1 FROM striatumd.barrier_staged_contributions s
              WHERE s.repository_id = fp.repository_id
                AND s.barrier_id = fp.barrier_id
                AND s.workflow_job_id = sib.value::text
                AND s.status = 'staged'
                AND s.staging_ref NOT LIKE 'refs/striatum/recovery/%'
                AND s.attempt = COALESCE((
                      SELECT MAX(j.attempt) FROM striatumd.jobs j
                       WHERE j.repository_id = fp.repository_id
                         AND j.run_id = fp.run_id
                         AND j.workflow_job_id = sib.value::text
                         AND j.state = 'completed'
                    ), -1)
           ) AS live_staged
      FROM striatumd.fanin_freeze_points fp,
           jsonb_array_elements_text(fp.declared_sibling_job_ids) AS sib(value)
), seat_agg AS (
    SELECT repository_id,
           barrier_id,
           run_id,
           count(*) AS declared_seats,
           count(*) FILTER (WHERE live_staged) AS staged_seats,
           count(*) FILTER (WHERE is_terminal_gap) AS terminal_gap_seats,
           count(*) FILTER (WHERE is_blocking AND NOT is_terminal_gap) AS blocking_seats,
           -- An outstanding seat is one that is neither live-staged nor a terminal
           -- gap. If it is also not blocking, it is merely slow/pending.
           count(*) FILTER (WHERE NOT live_staged AND NOT is_terminal_gap) AS outstanding_seats
      FROM seat
     GROUP BY repository_id, barrier_id, run_id
)
SELECT fp.repository_id,
       fp.barrier_id,
       fp.run_id,
       fp.downstream_workflow_job_id,
       fp.frozen_tip_sha,
       fp.frozen_tip_tree_sha,
       fp.created_at AS frozen_at,
       COALESCE(sa.declared_seats, 0)     AS declared_seats,
       COALESCE(sa.staged_seats, 0)       AS staged_seats,
       COALESCE(sa.terminal_gap_seats, 0) AS terminal_gap_seats,
       COALESCE(sa.blocking_seats, 0)     AS blocking_seats,
       COALESCE(sa.outstanding_seats, 0)  AS outstanding_seats,
       bs.state              AS assembly_state,
       bs.target_commit_sha  AS assembly_target_commit_sha,
       bs.tree_sha           AS assembly_tree_sha,
       bs.base_commit_sha    AS assembly_base_commit_sha,
       bs.fail_reason        AS assembly_fail_reason,
       bs.committed_at       AS assembly_committed_at,
       -- The derived, advisory legibility condition (NOT the authoritative
       -- readiness predicate). It names the barrier's lifecycle position for status
       -- and the doctor invariant. BARRIER_BLOCKED is the named runtime/doctor
       -- condition (RFC 0135 P3): a barrier with a live BLOCKING in-edge that cannot
       -- fire without intervention — surfaced here as a derived value, NOT a
       -- barrier_state CHECK value (the barrier_state lifecycle stays
       -- sealed/assembling/committed/failed; blocked is an UPSTREAM, pre-seal
       -- condition over the in-edges, not an assembly-journal state).
       CASE
         WHEN bs.state = 'committed' THEN 'COMMITTED'
         WHEN bs.state = 'failed'    THEN 'FAILED'
         WHEN bs.state = 'assembling' THEN 'ASSEMBLING'
         WHEN COALESCE(sa.blocking_seats, 0) > 0 THEN 'BARRIER_BLOCKED'
         WHEN bs.state = 'sealed'    THEN 'SEALED'
         WHEN COALESCE(sa.outstanding_seats, 0) = 0
              AND COALESCE(sa.declared_seats, 0) > 0 THEN 'READY'
         ELSE 'PENDING'
       END AS condition
  FROM striatumd.fanin_freeze_points fp
  LEFT JOIN seat_agg sa
    ON sa.repository_id = fp.repository_id AND sa.barrier_id = fp.barrier_id
  LEFT JOIN striatumd.barrier_state bs
    ON bs.repository_id = fp.repository_id AND bs.barrier_id = fp.barrier_id;

-- Explicit GRANT (mandatory; pgtest masks an omission). Read-only: the view is a
-- status/audit projection, never written. Guarded by an IF EXISTS probe so the
-- single-role pgtest harness (where striatumd_rw is absent) is a clean no-op.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT ON striatumd.barrier_status TO striatumd_rw;
  END IF;
END
$$;
