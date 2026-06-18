-- GH #386 — owner bundle 0015: FK covering indexes on events + audit_log.
--
-- striatumd.events and striatumd.audit_log are OWNER-HELD tables (owned by the
-- database owner, not the runtime role striatumd_rw): their writes go through
-- SECURITY DEFINER functions (owner bundle 0004 append_event_row; the audit
-- writer), and runtime INSERT is revoked. CREATE INDEX requires table
-- ownership, so these indexes live in this owner/admin bundle rather than a
-- runtime migration (RFC 0079 §5 / D187 — the runtime guard test
-- TestFutureRuntimeMigrationsDoNotCarryOwnerDDL keeps owner-table DDL out of
-- the runtime tree; decision D215 keeps owner-held-table DDL in owner bundles).
-- Owner bundle 0011 (idx_events_repo_run_session_type_time) is the precedent.
--
-- Why: several foreign keys on these tables have no covering index, so deleting
-- a referenced parent row forces PostgreSQL to seq-scan the whole child table to
-- check for dependent rows. As events/audit_log grow (events ~13.5M rows on
-- proximal) this becomes a seq-scan cliff the moment retention/GC starts
-- deleting parent rows (runs, sessions, artifacts, leases, queue_messages,
-- audit_segments). The existing event indexes lead with run_id / job_id only
-- (events_pkey (repository_id, event_id), idx_events_run_time, idx_events_job),
-- leaving the actor_session_id / artifact_id / lease_id / message_id FKs and the
-- audit_log.segment_id FK uncovered. Each composite below leads with the FK
-- column(s) so the FK-RI delete check is an index lookup, not a table scan.
--
-- No GRANT and no schema_authority / capability stamp are needed: this is a pure
-- index add and the runtime role keeps its existing SELECT. (Owner bundle 0011
-- and 0008 set the precedent — pure index/column adds with no grant.)
--
-- Operational note: built once, idempotent by name (CREATE INDEX IF NOT EXISTS),
-- and safe for binary-before-owner-bundle deploys — the new daemon runs fine
-- against the prior index set, and applying this bundle adds the indexes without
-- touching the binary. ApplyOwnerBundles runs the whole bundle in one
-- transaction and stamps owner_bundle_meta last; plain (non-CONCURRENT) CREATE
-- INDEX is transaction-safe, so CONCURRENTLY is intentionally NOT used here. On
-- an already-running large deployment where a write-blocking in-transaction
-- build is undesirable, build the same-named indexes out of band first with
-- CREATE INDEX CONCURRENTLY; this bundle then no-ops (IF NOT EXISTS matches by
-- name). See GH #386.

CREATE INDEX IF NOT EXISTS idx_events_repo_actor_session ON striatumd.events (repository_id, actor_session_id);
CREATE INDEX IF NOT EXISTS idx_events_repo_artifact ON striatumd.events (repository_id, artifact_id);
CREATE INDEX IF NOT EXISTS idx_events_repo_lease ON striatumd.events (repository_id, lease_id);
CREATE INDEX IF NOT EXISTS idx_events_repo_message ON striatumd.events (repository_id, message_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_segment ON striatumd.audit_log (segment_id);
