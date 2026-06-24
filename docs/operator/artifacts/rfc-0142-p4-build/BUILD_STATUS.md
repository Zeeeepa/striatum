# RFC 0142 P4 build — status (2026-06-24)

**Design:** CLEARED (v9, `accept_with_findings`, D262). PROPOSAL.md is the contract:
`docs/operator/artifacts/rfc-0142-p4-design-v9/commit/proposal/PROPOSAL.md`.

**Build:** implemented + reviewed; **ONE bounded revision remaining** before it can land.

## What is done (real credit — do not regress)

Two build attempts ran. The implementation is substantively complete and was
independently build/vet/lint-verified clean (non-PG tests green; pg-tests compile + skip):

- Migration `0044_deploy_cursor.sql` — additive **runtime-owned** (`deploy_cursor` +
  `deploy_plan` + `deploy_receipt`; `state` CHECK includes `finalizing`; role-guarded
  GRANT). `LatestDaemonDBVersion = 44`. ✅
- `owner.go` M2 surface: `DDLRevokeOwnerBundleVersion = 21`, `isNonRevokeBundle`,
  `OwnerDDLApplyBundles()`, in-loop guards, filtered nil-fallback. `LatestOwnerBundleVersion`
  / `RequiredOwnerBundleVersion` **stay 20**. ✅
- `deploy.go` + `deploy_apply.go` + `deploy_activation.go`: `BuildPlan`, `plan_hash`,
  `VerifyStoredTranscript` (M1 byte arm) + **`VerifyAppliedDBStamps` (M1 DB-stamp arm, real)**,
  `Deployer.Apply` Q3-A per-step atomicity (DDL + version stamp + cursor advance + receipt in
  ONE tx), hash-chained per-step `deploy_receipt`, finalizer. ✅ (D3/D4/D5 resolved in attempt 2.)
- `runDaemonDeploy` verb + dispatch; `doctor_deploy.go` (`schema_deploy_unrecorded` +
  M1 WARN). ✅
- `CheckDeployActivation` predicate: the decoupled-complete branch reads **neither
  `applied_owner` nor `revokeEmbedded`** (M6 + M7 honored at the predicate level). ✅

## The ONE blocking finding to fix (review attempt 2, `needs_revision`) — D7'

`review/REVIEW_v2_needs_revision.md` (banked here). Attempt 2 resolved the reviewer's
"embed 0021?" question by **embedding** `0021_revoke_create_privilege.sql` in
`ownerBundleFS`, flipping `RevokeBundleEmbedded() → true` for the production binary. Then
`DecideDeployActivation` step 0 (`RevokeEmbedded && !DecoupledEnabled → awaiting_deploy_config`,
which fires for EVERY cursor state before the switch) makes the **flag-OFF default boot
refuse to serve** — bricking production serve-boot, the entire `pgtest`-based CI suite
(masked locally by unset `STRIATUM_PG_TEST_URL`), and B1.1's own live arm. This breaks the
SEED's hard shadow-first invariant ("0021 lands INERT; flag-OFF serve-boot UNCHANGED").

### Fix (Option B — shadow-first-faithful, reviewer-recommended)

1. **Un-embed 0021**: move `0021_revoke_create_privilege.sql` OUT of `ownerBundleFS`
   (back to a staged dir like attempt 1's `sql/owner_staged_activation/`) so
   `RevokeBundleEmbedded()` stays `false` and flag-OFF boot is byte-identical. Re-scope
   F16b-production, the revoke-in-plan, and §6.5 criterion (l) to the **activation/verify
   binary** per PROPOSAL §4.3 (the documented two-binary choreography), and say so in the
   DRAFT + roadmap.
2. **Prove it with a test that runs**: a non-PG unit arm over `DecideDeployActivation` for
   the production `RevokeBundleEmbedded()` value, AND the `pgtest` harness must bootstrap
   green.
3. **D1' — tighten B1.1 live arm**: seed the `==20` and `>=21` `owner_bundle_meta` buckets
   (not only `==0`) and assert the mutating path is not entered via a real call-tracking
   spy on `ApplyMigrations`/`RecordSchemaFingerprint` (not just the decision-value tautology).
4. **D6 — C3 §3.3b**: either wire the per-step ownership reconcile (snapshot new owner-owned
   oids + `ALTER … OWNER TO striatumd_rw` + assert `has_schema_privilege` pre-step) or record
   its re-scope to the activation/verify run explicitly (criterion (l) depends on it).

## Preserved branches (snapshots, NOT for direct merge)

- `backup/rfc-0142-p4-build-attempt2-2026-06-24` — attempt-2 committed work (D3/D4/D5 fixed,
  but 0021 embedded = the D7' regression). **Start the revision from here and apply Option B.**
- `backup/rfc-0142-p4-build-impl-2026-06-24` — attempt-1 work (0021 staged OUTSIDE = shadow-first
  correct on the 0021 question, but D3/D4/D5 not yet real). Reference for the un-embed layout.

## How to finish

Scaffold a fresh `rfc-0142-p4-build-v2` code_change run (draft→review→apply, both claude
lanes, write_scope `go/` NOT `go/**` — see #586) whose SEED directs the draft to adopt
`backup/rfc-0142-p4-build-attempt2-2026-06-24` and apply the Option-B fix above
(context_docs: PROPOSAL.md, this REVIEW_v2, the v9 adjudicator ledger). Drive it; on clear,
verify (`striatum verifier run` go-build/go-vet sealed receipts) + `run integrate --into main`.

Runner footgun that wedged the first attempt: GH #586 (write_scope `go/**` no-glob).
