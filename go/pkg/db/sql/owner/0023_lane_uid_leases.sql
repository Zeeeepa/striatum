-- RFC 0168 P0 (D272) — per-lane uid lease authority reassertion.
--
-- Runtime migration 0047 creates striatumd.lane_uid_leases as a runtime-owned
-- lifecycle table. This owner bundle advances the deployment watermark for the
-- RFC 0168 build and reasserts the runtime grants when the table is already
-- present; fresh deployments that apply owner DDL before runtime migration 0047
-- are still covered by 0047's guarded GRANT block.

DO $$
BEGIN
  IF to_regclass('striatumd.lane_uid_leases') IS NOT NULL
     AND EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE ON striatumd.lane_uid_leases TO striatumd_rw;
    REVOKE DELETE ON striatumd.lane_uid_leases FROM striatumd_rw;
  END IF;
END
$$;
