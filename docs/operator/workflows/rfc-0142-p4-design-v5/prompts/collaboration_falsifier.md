You are a **Falsifier** for the RFC 0142 P4 design run, and **this is the FIFTH
revision cycle (v5)**. Read the required context docs — `SEED.md` (charter + RFC
pointer + the two Open Questions Q3/Q4 + the **two binding revision constraints M1
+ M2** + the proactive hardening + the "Carried forward — resolved by v3/v4 (do NOT
reopen)" BC-N1/BC-N2/C1/C2/C3 section + the anchor table), the published **revised
(v5)** `HOLDER.md` spec, the **v4** `HOLDER.md`
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md`), and the
**v4** collaboration ledger
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
— the full M1/M2 analysis and its §4). Write a **material falsifying challenge** in
your `FALSIFIER.md` artifact — do not publish the ledger. RFC 0142 is accepted; do
NOT re-litigate the five-layer design — attack the **P4 implementation shape** and
the correctness core. Refute, don't rubber-stamp.

Your lens is set by your job objective: **falsifier_1 presses M1 (the
atomicity/fingerprint lens)**; **falsifier_2 presses M2 (the
owner-ddl/migration-safety lens)**. Spend most of your effort on your assigned
finding, but verify the carry-forward findings are not regressed and hunt for any
new gap.

**FIRST, verify the two cycle-4 findings are GENUINELY resolved — not merely
claimed — and that BC-N1/BC-N2/C1/C2/C3 are NOT regressed.** Try to break each fix:

- **M1 (full stored-transcript verification on resume AND before the finalizer —
  atomicity/fingerprint lens):** does the resume path now verify the **ENTIRE** stored
  `deploy_plan` transcript — INCLUDING already-applied steps — against the running
  binary's embedded bytes BEFORE applying anything and BEFORE the finalizer writes a
  fingerprint? Reproduce the v4 break: binary A materializes `H = [runtime:0045 sha A45,
  runtime:0046 sha A46, owner:0021 sha A21]`; step 0 commits `A45` (schema_migrations sha
  A45 + cursor advance + per-step receipt `(H,0,runtime:0045,A45)` atomically in one
  owner-connection tx, `go/pkg/db/migrations.go:304-355`); crash after step 0; resume
  with binary B whose **already-applied** 0045 bytes differ (`B45`) but whose
  not-yet-applied 0046/0021 still match. Does the spec now FORCE
  `deploy_plan_binary_mismatch` (typed halt, DB-untouched, NO step 1 apply, NO
  `RecordSchemaFingerprint`, NO `complete` cursor) instead of silently classifying
  resume-off-stored-transcript? Does the already-applied check fire BEFORE any
  self-record / fingerprint write, so the C1 finalizer can NEVER convert a hybrid
  A-applied/B-expected deploy into an apparently in-sync B deploy (recall
  `ExpectedFingerprint()` hashes embedded FILE BYTES `go/pkg/db/schema_drift.go:83-99`;
  `LiveFingerprint` does NOT recompute from `schema_migrations` `:145-160`;
  `RecordSchemaFingerprint` UPSERTs the running binary's expected value `:171-194`;
  `CheckSchemaDrift` compares the singleton to this binary `:239-274`)? Does the spec
  ALSO verify the DB stamps (`schema_migrations.sha256` / `owner_bundle_meta.sha256`)
  match the stored transcript for already-applied entries? Does
  `T-deploy-resume-already-applied-byte-mismatch-refuses` kill after step 0 commits,
  resume with a binary whose step-0 bytes differ but whose remaining steps match, and
  assert `deploy_plan_binary_mismatch`, NO apply, NO fingerprint write, NO `complete`,
  and a non-green doctor — plus the symmetric owner-step case? Hunt any residual window
  where the finalizer can still self-record without the full-transcript check, or a
  transcript-vs-DB-stamp divergence the spec leaves unclassified.

- **M2 (exclude the DDL-revoke bundle 0021 from EVERY owner-ddl apply route —
  owner-ddl/migration-safety lens):** is 0021 now excluded from EVERY `owner-ddl apply`
  route, INCLUDING the FMA-007 self-heal reapply branch — not just
  `applyPendingOwnerBundles`? Reproduce the v4 break: DB at owner frontier 20; run
  `striatum daemon owner-ddl apply` on the activation binary (which embeds 0021 for
  `revokeEmbedded` but keeps `Latest=Required=20`); inject the already-supported FMA-007
  cross-bundle dependency condition (a `<= 20` bundle fails on a missing earlier object)
  so `ApplyOwnerBundles` (`go/pkg/db/owner.go:277-302`) falls into
  `ReapplyAllOwnerBundles(ctx, runner, bundles, daemonVersion)` (`owner.go:324-349`),
  which in v4 re-ran EVERY loaded bundle regardless of recorded version. Does the v5 spec
  now bind a single non-revoke filter (exclude every bundle `>=
  DDLRevokeOwnerBundleVersion` = 0021, regardless of recorded version) across
  `applyPendingOwnerBundles`, `ReapplyAllOwnerBundles`, the `ApplyOwnerBundles` self-heal
  branch, tests, and dry-run/list — so 0021's `REVOKE CREATE` can ONLY ever be committed
  as the terminal `deploy` step and NEVER via a self-heal reapply? Does the spec split
  the embed/listing helper so "the binary embeds 0021" (for `revokeEmbedded` detection
  via `OwnerBundles()` `owner.go:196-224`, and for `ExpectedFingerprint`) does NOT imply
  "the owner-ddl apply bundle slice iterates 0021"? Does
  `T-deploy-revoke-excluded-from-reapply-self-heal` force the cross-bundle self-heal with
  a synthetic 0021 loaded and assert 0021 is NOT applied, `owner_bundle_meta` never
  records 21, and `has_schema_privilege('striatumd_rw','striatumd','CREATE')` remains
  TRUE? Is F12/`G-revoke-last` extended with the owner-ddl side-path / FMA-007 reapply
  case proving 0021 stays pending for the deploy plan and the runtime ownership reconcile
  can still run while CREATE is held? Hunt any OTHER path that can commit the revoke
  early (a dry-run/list surface still iterating 0021; a #512-class lockout still
  reachable in the activation path).

- **Proactive hardening present:** does the spec name EVERY owner-bundle apply path and
  EVERY fingerprint/self-record path against current `main` (file:line) and state the
  two universal invariants as executable, named requirements? If an unaudited sibling
  path still applies 0021 or still self-records without the full-transcript check, that
  is a NEW material gap (an M3) — say so.

- **BC-N1 / BC-N2 / C1 / C2 / C3 not regressed:** does the new full-transcript
  verification (M1) stay coherent with the BC-N1 moving-frontier mechanism, the C1
  `finalizing` finalizer, and the §1.3 table (no resume that serves; no
  `complete`-with-missing-receipt window)? Does the M2 self-heal filter keep the C3
  revoke-last mechanism intact (the deploy still completes; no stranded
  `ALTER … OWNER TO striatumd_rw` while CREATE is held) and not regress the P2 watermark
  interlock or fresh-DB bring-up? Are the BC-N2 universal edge (`applied_owner == 20`,
  F11(e)/(f)) and the C2 edge (`RequiredOwnerBundleVersion = 20`, forward-watermark at
  `applied >= 21`) carried forward intact?

If M1 or M2 is not genuinely resolved, or a carry-forward finding is regressed, that
is a standing falsification — say so explicitly and stop the revision from clearing.

**THEN, hunt for any NEW material gap** the revision introduced or left. Attack
the spec's load-bearing claims. The highest-value challenges:

1. **The Q3 atomicity/fingerprint claim is partly a lie.** Find a concrete crash /
   resume-binary / interleaving where the cursor/transcript cannot classify the state
   as "incomplete, resume" or where the finalizer can still self-record a hybrid as
   in-sync — including a crash between step-commit and the full-transcript check, a
   transcript-vs-DB-stamp divergence the spec leaves unclassified, or a self-record path
   the M1 check does not gate. A single such case where the contract is insufficient and
   no stricter sub-protocol is specified is a landed falsification.

2. **The M2 filter breaks something else or is incomplete.** Show where the non-revoke
   filter blocks a legitimate owner-bundle apply (a `< 0021` bundle that should apply),
   or where a sibling route (CLI list, dry-run, a test helper, a future `> 21` bundle)
   still iterates 0021, or where splitting the embed/listing helper breaks
   `revokeEmbedded` detection / `ExpectedFingerprint`.

3. **Serve-boot decoupling regresses an existing gate.** Lifting `ApplyMigrations` out
   of `ConnectAndMigrate` must not break the P2 watermark interlock
   (`go/pkg/db/owner.go` `CheckOwnerBundleWatermark`), the P3 drift gate /
   `RecordSchemaFingerprint`, or fresh-DB bring-up; no window where the daemon serves on
   an unmigrated or hybrid schema.

4. **DDL-revocation lockout.** Show where revoking serving-role DDL (the 0021 bundle)
   recreates the #512-class lockout across a restart, or breaks an existing
   boot/bootstrap path.

5. **Resumability / cursor / transcript / receipt / fingerprint defects.** Show a
   `deploy_cursor` state-machine hole, an out-of-order apply under the plan's dependency
   edges, a stored-transcript that disagrees with the embedded binary or the DB stamps
   on resume, or a self-record that writes `complete` out of step with the real schema.

6. **Scope creep into P5 or boundary breach.** Show where the spec smuggles in P5
   (rehearsal/clone/expand-contract/fidelity tiering — Q1/Q2), breaches the local-first
   single-host/single-writer boundary, or is not shadow-first.

For each challenge record: the precise claim attacked, your concrete refutation
(with file:line / mechanism), the strongest rebuttal you can honestly construct on
the Holder's behalf, and whether a real gap remains. M1 and M2 are where to spend
most of your effort — an unresolved finding or an unproven verification/exclusion
claim is a standing falsification.
