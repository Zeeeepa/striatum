-- RFC 0126 P0 (D194 / GH #282) — owner bundle 0009: build-owned review generation.
--
-- striatumd.jobs and striatumd.verdicts are OWNER-HELD tables in the two-role
-- posture, so this column-add is owner-table DDL and MUST live in an owner/admin
-- bundle rather than a regular runtime migration (D187; the RFC 0081 owner-held
-- ALTER crash-loop). The runtime guard test TestFutureRuntimeMigrationsDoNotCarryOwnerDDL
-- (floor v27) forbids any runtime migration >= 27 from ALTERing these tables, so
-- the column lives here and the verdict write path includes it adaptively
-- (reviewGenerationEnabled, mirroring the artifact placement column).
--
-- A build/synthesis job owns a monotonic `review_generation` — its "review
-- epoch": which round of review covers the current build content. It starts at 1
-- and is incremented in the SAME transaction as the build's attempt bump
-- (reopenJobForAttempt). Every verdict is stamped at record time with the
-- reviewed build's current generation, so a build revision renders prior-round
-- verdicts non-current by GENERATION MISMATCH rather than by DELETE: verdict
-- history is append-only and a stale verdict can no longer silently survive a
-- reset as the "latest" row blocking finalization.
--
-- Both columns are NOT NULL DEFAULT 1 so any pre-existing rows read back as the
-- first generation (mirroring jobs.attempt). striatumd_rw already holds table-
-- level INSERT/UPDATE/SELECT on both tables (migration 0005), which extends to
-- new columns, and verdicts INSERT is NOT revoked from the runtime role, so no
-- new grant or SECURITY DEFINER function is required.

ALTER TABLE striatumd.jobs
  ADD COLUMN IF NOT EXISTS review_generation integer NOT NULL DEFAULT 1;

ALTER TABLE striatumd.verdicts
  ADD COLUMN IF NOT EXISTS review_generation integer NOT NULL DEFAULT 1;
