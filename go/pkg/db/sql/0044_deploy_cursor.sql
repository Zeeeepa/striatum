-- RFC 0142 P4 (the one-shot `striatum daemon deploy` decoupler): the deploy
-- substrate — three NEW runtime-owned tables that carry the explicit, ordered,
-- resumable, provenance-tracked deploy operation that P4 lifts OUT of serve-boot.
--
-- This is ADDITIVE and runtime-owned (a RUNTIME migration, NOT an owner bundle):
-- it carries NO owner DDL, declares NO foreign keys, and never touches
-- owner_bundle_meta. The serving daemon reads deploy_cursor on EVERY decoupled
-- boot (CheckDeployActivation), so the tables must exist for every daemon from
-- first boot — exactly the reason 0043_schema_state.sql is a runtime migration
-- and not an owner bundle. It creates only NEW striatumd-owned tables (mirrors
-- 0028 job_workspaces / 0042 verifier_attestations / 0043 schema_state), so the
-- runtime role striatumd_rw can apply it on a live restart without the two-role
-- 42501 owner-table trap (TestFutureRuntimeMigrationsDoNotCarryOwnerDDL).
--
-- The three tables (RFC 0142 P4 §1.2 / BC-N1 / C1):
--
--   deploy_cursor  — the singleton resumable cursor. `state` advances
--                    idle → in_progress → step_committed → … → finalizing →
--                    complete (or → aborted on operator abort / fatal). The
--                    CHECK includes `finalizing` (C1: the finalization boundary
--                    is closed by a DISTINCT state + an idempotent finalizer).
--                    plan_hash points at the immutable transcript a resume reads.
--
--   deploy_plan    — the IMMUTABLE ordered transcript (BC-N1), keyed by
--                    plan_hash, INSERT-ONCE (ON CONFLICT (plan_hash) DO NOTHING).
--                    `steps` is the ordered jsonb transcript
--                    [{step_index, step_id, role, sha256, transactional}],
--                    revoke (bundle 0021) sorted LAST. A resume reads
--                    deploy_plan[cursor.plan_hash] and NEVER recomputes BuildPlan,
--                    so plan identity is a durable fact, not a per-boot guess.
--
--   deploy_receipt — the per-step hash-chained receipt trail (BC-N1 / §3.4),
--                    keyed on the stored (plan_hash, step_index). Each applied
--                    step appends exactly one receipt in the SAME transaction as
--                    the step + cursor advance, so a committed step always carries
--                    its receipt (exactly-once). row_hash chains prev_hash → this
--                    row so a gap or reorder is detectable; doctor enumerates the
--                    transcript against this trail (schema_deploy_unrecorded).
--
-- The GRANT block makes the substrate readable + writable by the runtime role
-- (broad-DML 0005 ran before these tables existed, so a fresh table needs its own
-- grant; pgtest single-role masks a missing GRANT, so it is mandatory). The
-- deployer normally writes these over the OWNER/admin connection, but the boot
-- read path (CheckDeployActivation) reads deploy_cursor + deploy_plan as the
-- runtime role on every decoupled serve, and the single-role deployer writes them
-- as the runtime role, so both SELECT and the cursor/plan/receipt DML are granted.

CREATE TABLE IF NOT EXISTS striatumd.deploy_cursor (
  id          text PRIMARY KEY DEFAULT 'singleton' CHECK (id = 'singleton'),
  plan_hash   text NOT NULL,
  state       text NOT NULL CHECK (state IN ('idle','in_progress','step_committed','finalizing','complete','aborted')),
  step_index  integer NOT NULL DEFAULT 0,
  step_id     text NOT NULL DEFAULT '',
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS striatumd.deploy_plan (
  plan_hash              text PRIMARY KEY,
  steps                  jsonb NOT NULL,
  revoke_step_index      integer NOT NULL DEFAULT -1,
  base_owner_version     integer NOT NULL,
  base_runtime_version   integer NOT NULL,
  target_owner_version   integer NOT NULL,
  target_runtime_version integer NOT NULL,
  created_at             timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS striatumd.deploy_receipt (
  plan_hash   text NOT NULL,
  step_index  integer NOT NULL,
  step_id     text NOT NULL,
  sha256      text NOT NULL,
  prev_hash   text NOT NULL DEFAULT '',
  row_hash    text NOT NULL,
  daemon_version text,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (plan_hash, step_index)
);

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE ON striatumd.deploy_cursor TO striatumd_rw;
    GRANT SELECT, INSERT ON striatumd.deploy_plan TO striatumd_rw;
    GRANT SELECT, INSERT ON striatumd.deploy_receipt TO striatumd_rw;
  END IF;
END
$$;
