-- RFC 0142 P4 C3 — owner bundle 0021: the serving-role create-DDL revocation.
-- Revoke CREATE on schema striatumd from the runtime role striatumd_rw, so the
-- serving daemon holds ZERO create-DDL on the serving path. After activation a
-- restart can never force-commit a half-applied schema change and a bad migration
-- can never wedge the single writer on boot — schema mutation is owned solely by
-- the one-shot `striatum daemon deploy`.
--
-- DEPLOY-PLAN-TERMINAL ONLY (M2 + C3). This bundle is EXCLUDED from every
-- `owner-ddl apply` route by OwnerDDLApplyBundles() + the in-loop isNonRevokeBundle
-- guard (owner.go), so its REVOKE can ONLY ever be committed as the terminal step
-- of a deploy plan (the last step BuildPlan emits, after every runtime migration
-- has reconciled its new objects' ownership back to striatumd_rw, §3.3b). It is
-- embedded in OwnerBundles() (so ExpectedFingerprint, RevokeBundleEmbedded, and
-- BuildPlan all see it), but LatestOwnerBundleVersion and RequiredOwnerBundleVersion
-- deliberately STAY 20 — the revoke is gated by the deploy cursor +
-- CheckDeployActivation + the STRIATUM_DEPLOY_DECOUPLED flag + its terminal
-- placement, NOT the owner-bundle watermark frontier.
--
-- IDEMPOTENT + role-guarded: REVOKE is a no-op when the privilege is already
-- absent, and the EXISTS-striatumd_rw probe leaves a single-role deploy (no runtime
-- role) untouched. striatumd_rw RETAINS ownership of (and ALTER/DROP on) the
-- runtime tables it already owns; only the right to CREATE NEW objects in the
-- schema is removed. Full capability revocation (owner re-owns runtime tables)
-- remains a named follow-up beyond P4 (§4.1).

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw;
  END IF;
END
$$;

INSERT INTO striatumd.schema_authority(capability, requires_daemon_auth, bundle_version)
VALUES ('serving_role_create_revoke', false, 21)
ON CONFLICT (capability) DO NOTHING;
