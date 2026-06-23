-- RFC 0142 Layer 2 / GH #581 — owner bundle 0020: make the owner-bundle
-- watermark table readable by the runtime role.
--
-- The serving daemon checks striatumd.owner_bundle_meta before it applies any
-- runtime migration, so striatumd_rw must be able to read the applied owner
-- bundle watermark. The table remains owner-owned and write-owner-only; this
-- grants only SELECT.

GRANT SELECT ON striatumd.owner_bundle_meta TO striatumd_rw;

INSERT INTO striatumd.schema_authority(capability, requires_daemon_auth, bundle_version)
VALUES ('owner_bundle_watermark_read', false, 20)
ON CONFLICT (capability) DO NOTHING;
