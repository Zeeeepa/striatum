-- GH #330 — owner bundle 0011: hot event-read covering index on events.
--
-- striatumd.events is an OWNER-HELD table (owned by the database owner, not the
-- runtime role striatumd_rw — see owner bundle 0004, which installs the
-- append_event_row SECURITY DEFINER fn and revokes runtime INSERT). CREATE INDEX
-- requires table ownership, so the index lives in this owner/admin bundle rather
-- than a runtime migration (RFC 0079 §5 / D187; the runtime guard test
-- TestFutureRuntimeMigrationsDoNotCarryOwnerDDL keeps owner-table DDL out of the
-- runtime tree). Owner bundle 0008 (idx_artifacts_search) is the precedent.
--
-- Why: the single most expensive daemon query by total execution time
-- (pg_stat_statements: ~31.9k calls, ~1,190 s cumulative, ~100% cache hit) is a
-- per-session event lookup:
--
--   SELECT event_id, run_id, actor_session_id, created_at, payload_json
--   FROM striatumd.events
--   WHERE repository_id = $1 AND run_id = $2 AND actor_session_id = $3
--     AND event_type = $4 AND payload_json->>$5 = $6
--   ORDER BY created_at DESC, event_id DESC
--   LIMIT $7;
--
-- The pre-existing indexes (events_pkey (repository_id, event_id),
-- idx_events_run_time (repository_id, run_id, event_id), idx_events_job
-- (repository_id, job_id, event_id)) lead with too few of the predicate's
-- equality columns, so under the generic (bind-parameter) plan the planner scans
-- a whole run's events (or, at smaller row counts, the entire table). This
-- composite covers all four equality columns AND the ORDER BY ... LIMIT, so the
-- planner can satisfy the query with an index scan that stops at LIMIT rows.
-- Measured on proximal (events ~13.5M rows): generic-plan cost ~35,118 -> ~8.7
-- (the table-scan generic plan the issue reported at ~1M rows -> sub-ms). The
-- payload_json->>$5=$6 term remains a residual filter (not generically
-- indexable) and is intentionally excluded.
--
-- Operational note: on an already-running deployment the index was built once,
-- out of band, with CREATE INDEX CONCURRENTLY (events is large and live; a plain
-- in-transaction build would hold a write-blocking lock). This bundle therefore
-- carries the plain idempotent CREATE INDEX IF NOT EXISTS under the SAME index
-- name, so applying it to such a deployment is a no-op (IF NOT EXISTS matches by
-- name) while a fresh bootstrap (empty events table) builds it instantly inside
-- the owner-bundle transaction. ApplyOwnerBundles runs the whole bundle in one
-- transaction and stamps owner_bundle_meta last; CREATE INDEX (non-concurrent)
-- is transaction-safe.
--
-- Scope: this is the one event-read hot path confirmed in pg_stat_statements.
-- The pg_qualstats advisor also flagged audit_log(ts) and several small tables
-- (client_capabilities, leases, process_supervisors, job_recovery_state) — those
-- were evaluated and intentionally NOT indexed here: audit_log max(ts) is not a
-- daemon query (daemon audit reads key on audit_id / request_id), and the small
-- advisor tables (~140-7k rows) seq-scan negligibly while an index would only add
-- write amplification. See GH #330.

CREATE INDEX IF NOT EXISTS idx_events_repo_run_session_type_time
  ON striatumd.events (
    repository_id,
    run_id,
    actor_session_id,
    event_type,
    created_at DESC,
    event_id DESC
  );
