You are the **Committer** for the RFC 0142 P4 design run (**v9 / REVISION**). The
adjudicator's collaboration ledger has cleared the gate. Publish the final,
falsification-hardened **implementation spec** as your `PROPOSAL.md` artifact — this
is the design run's primary deliverable, the spec the `rfc-0142-p4-build` run will
build contract-first.

Start from the **revised (v9)** Holder's `HOLDER.md` and fold in every challenge the
adjudicator recorded as material-and-incorporated. Because this is the v9 revision
run, the committed spec MUST carry **all twelve** fixes as first-class, build-ready
design — the cycle-1 fix resolved this cycle and the eleven earlier fixes carried
forward:

- **M7 — §3.5 row 16 and its `>=21` variant made CONDITIONAL on A's fingerprint-sync
  predicate** (resolved this cycle): `cursorState=complete`, `decoupledEnabled=true`,
  `revokeEmbedded=true` in the `==0`/`==20` columns now says "SERVE-verify if in-sync,
  else `awaiting_deploy`" — derived from A's §3.3a step-3 decoupled branch logic
  (`cursor.plan_hash == expected` + `LiveFingerprint == ExpectedFingerprint`, NO
  `applied_owner` input, confirmed by `schema_drift.go:145-161`/`:171-195` being
  orthogonal to `owner_bundle_meta`); the `>=21` revoke-embedding complete-row cell is
  also conditional for full derivation; §1.3/§3.3a/§3.5/§4.5/F18 all updated
  consistently; F18 is parametric over ALL complete-row cells (13/15/16 and `>=21`
  variants) with the in-sync/out-of-sync sub-dimension; the normal pre-0021 state is
  documented as out-of-sync (the in-sync subcase is the degenerate corner A must not
  mishandle); the fix does NOT advance `RequiredOwnerBundleVersion`; the complete-row
  class is now fully closed by derivation.
- **M6 — the M5 `applied_owner` split propagated coherently through the no-revoke
  `complete` rows (rows 13/15)** carried forward: the §0.2 W→A-independence invariant
  stated and anchored to `schema_drift.go:145-161`/`:171-195`; §3.5 rows 13 and 15
  in the `==0` column conditional — "serve if in-sync, else `awaiting_deploy`" —
  identical to `==20`; the degenerate 13/`==0`-in-sync idempotent `:399` rewrite in
  BOTH §4.5 AND the F18 spy list; the four `:399`-reaching cells {1/`==0`, 1/`==20`,
  13-in-sync/`==0`, 13-in-sync/`==20`} enumerated identically; the cross-row
  `==0`-vs-`==20` audit intact.
- **M5 — the owner-watermark dimension split, row-1** carried forward: `applied_owner`
  dimension split into `{0/no authority, 1..19 authority shortfall, ==20, >=21}`; cell
  1/`==0` (no-revoke, flag off, `cursorState=none`, NO transcript) SERVES-legacy the
  fresh-DB bring-up — matching `CheckOwnerBundleWatermark`'s `applied == 0` exception
  (`go/pkg/db/owner.go:145`); `awaiting_owner_ddl` retained for `1 <= applied_owner <
  20`; F18 asserts BOTH the `applied_owner == 0` serve cell AND the `1..19` halt cell;
  F18a pins the fresh-DB serve; cell `==20` relabeled the inert-landing re-boot cell.
- **M3 — the COMPLETE-cursor activation guard** carried forward: `CheckDeployActivation`
  enforces the `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config
  guard hoisted to step 0 (every cursor state incl. `complete`) before `ApplyMigrations`
  (`connection.go:353`) and before `RecordSchemaFingerprint` (`:399`) on both boot paths;
  the no-revoke `complete` residual runs a pre-`ApplyMigrations`, DB-untouched pure-read
  comparison; §4.5 Universal Invariant B tightened so a transcript-carrying DB never
  reaches the legacy `connection.go:399` writer; F17/F11(g)/F18. (Row 16 is decoupled
  and NEVER reaches `:399`; the F18 spy list stays at 4 cells.)
- **M4 — the phase-aware F16 split** carried forward: F16a (synthetic-list pre-0021,
  step 2) + F16b (production post-0021 + the forced FMA-007 self-heal reaching
  `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`, step 7).
- **M1 — the FULL-stored-transcript byte + DB-stamp verification** carried forward
  (`VerifyStoredTranscript` over every step on every resume AND as finalizer step 0,
  ANY mismatch → `deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch`
  DB-untouched; F15 + extended F14).
- **M2 — the single non-revoke filter across EVERY owner-ddl apply route** carried
  forward (`isNonRevokeBundle` / `OwnerDDLApplyBundles()` + in-loop guards on
  `applyPendingOwnerBundles` / `ReapplyAllOwnerBundles` / the FMA-007 self-heal + the
  nil-fallback split + the embed/listing split; F16 safety + F12/`G-revoke-last`).
- **BC-N1 — the immutable-transcript per-step deploy receipt protocol** carried forward
  (the `deploy_plan` table before step 0; resume off the STORED transcript by
  `plan_hash`; §1.3 + `doctor schema_deploy_unrecorded` keyed off the stored transcript;
  F14).
- **BC-N2 — the universal pre-revoke `deploy_cursor`-authoritative serve edge** carried
  forward (`CheckDeployActivation` not gated on `revokeEmbedded`; every deployer-aware
  binary halts `awaiting_deploy` DB-untouched on a non-`complete` cursor at `applied_owner
  == 20`; F11(e)/(f) + `G-old-binary-refuse`). M3 EXTENDS this edge to the
  complete-cursor case; M5/M6/M7 re-bucket the orthogonal owner-watermark dimension at W
  or close complete-row cells — none weakens the non-complete `applied_owner == 20` edge.
- **C1 — the finalization-boundary sub-protocol** carried forward (the `finalizing`
  state + idempotent finalizer + the §1.3 resumable-finalization row + F10), gated by
  the M1 pre-finalizer full-transcript check.
- **C2 — the fail-closed revoke-bundle + `STRIATUM_DEPLOY_DECOUPLED` activation edge**
  carried forward (typed pre-`ApplyMigrations` halt + forward-watermark rule at `applied
  >= 21` + `RequiredOwnerBundleVersion = 20`). M3 extends this edge to the
  complete-cursor case; M5/M6/M7 re-bucket or close complete-row cells without advancing
  `Required` or altering the watermark.
- **C3 — the chosen, revoke-compatible runtime-object ownership policy** carried forward
  (revoke-last: the DDL-revoke bundle 0021 special-cased out of the owner prefix and
  excluded from `owner-ddl apply`, applied as the terminal plan step after every runtime
  ownership reconcile while `striatumd_rw` still holds CREATE; `T-deploy-runtime-object-ownership`
  / F12 + `G-revoke-last`). Keep the DDL-revoke bundle at the renumbered **0021** ordinal
  — 0020 is `0020_owner_bundle_watermark_read.sql` and `LatestOwnerBundleVersion == 20`.

…and carry the **proactive-completeness boot-path decision table** (every `cursorState`
× `decoupledEnabled` × `revokeEmbedded` × `applied_owner ∈ {0, 1..19, ==20, >=21}` cell
with a specified guard/outcome DERIVED MECHANICALLY from W and A, ALL complete-row cells
(13/15/16 and `>=21` variants) conditional on A's fingerprint-sync predicate, Universal
Invariant B proven in each, §4.5 and F18 spy list consistent at 4 `:399`-reaching cells,
F18 parametric over all complete-row cells with in-sync/out-of-sync sub-dimension, AND
the legitimate fresh-DB cells still serving), and not regress Q4, the per-step
Q3-A/Q3-B body, or the shadow-first decoupling shape. The committed spec MUST:

- **Resolve Q3 and Q4** with the decided mechanism: the per-step-atomic +
  resumable-cursor contract and the `deploy_cursor` state machine (Q3), now including
  the immutable stored transcript (BC-N1), the universal pre-revoke serve edge (BC-N2),
  the full-transcript binary/DB-stamp verification on resume and before the finalizer
  (M1), the complete-cursor activation guard (M3), the full boot-path decision table with
  the M5-correct owner-watermark dimension split (cell 1/`==0` fresh-DB serve), the M6
  mechanical derivation for rows 13/15, AND the M7 closure of the complete-row class
  (rows 13/15/16 and `>=21` variants all conditional on A's fingerprint-sync predicate,
  F18 parametric, normal pre-0021 state documented as out-of-sync); and the
  plain-verb-vs-run decision with its handling of the bootstrapping paradox (Q4).
- **Name the exact surfaces:** the `striatum daemon deploy` command site
  (`go/pkg/cli/localcommands/daemon.go`); the deploy plan form (ordered, role-tagged,
  dependency-edged, with the stored immutable transcript and the C3 terminal ordering for
  the 0021 DDL-revoke bundle); the new `deploy_cursor` / `deploy_plan` runtime migration
  (≥ 0044, additive); the hash-chained deploy receipt into the owner-held `audit_log`
  (per-step receipt keyed on the stored-transcript `(plan_hash, step_index)`); the precise
  edit to `go/pkg/db/connection.go` `ConnectAndMigrate` / `ConnectAndVerify` that lifts
  `ApplyMigrations` out of serve-boot while keeping the P2 watermark interlock and P3
  drift gate intact AND adds the `CheckDeployActivation` cursor read with the BC-N2
  non-complete edge, the M3 complete-cursor guard, the M5-correct owner-watermark
  dimension (cell 1/`==0` fresh-DB serve), the M6-coherent rows-13/15 conditional cells,
  AND the M7-closed full complete-row class (rows 13/15/16 and `>=21` variants all
  conditional on A's fingerprint-sync predicate — anchored to `schema_drift.go:145-161`
  + `schema_drift.go:171-195`); the full-transcript verification surface
  (`go/pkg/db/schema_drift.go` `ExpectedFingerprint`/`LiveFingerprint`/
  `RecordSchemaFingerprint`/`CheckSchemaDrift`) and the M1 `VerifyStoredTranscript`
  pre-finalizer check; the non-revoke owner-bundle filter (`isNonRevokeBundle` /
  `OwnerDDLApplyBundles()`) bound across `go/pkg/db/owner.go`
  `applyPendingOwnerBundles` / `ReapplyAllOwnerBundles` / `ApplyOwnerBundles` and the
  split embed/listing helper; and the owner bundle 0021 that revokes serving-role DDL —
  sequenced terminal per C3 so it never locks out the runtime path nor strands the
  deploy nor opens a pre-revoke serve window nor commits early via a self-heal reapply
  nor is reachable via the legacy serve-boot path on a `complete` cursor (M3).
- **Specify the build slices in contract-first order** (smallest safe first), each with
  its named Go tests and the exact migration/owner-bundle additions, the M4 phase-aware
  F16 split mapped to the rollout steps, and the F18 boot-path decision-table test with
  the parametric ALL-complete-row extension (F18 spy list = §4.5 = 4 cells reaching
  `:399`; F18 parametric over 13/15/16 and `>=21` variants with in-sync/out-of-sync
  sub-dimension). Be shadow-first.
- **State the explicit Acceptance Criteria** an impl-run + verify-run must meet,
  including the mandatory **game-day fire tests**: (a) kill `striatum daemon deploy`
  after a committed step and prove clean resume with a stable `plan_hash`/`step_index`
  and exactly-once per-step receipt off the stored transcript (BC-N1); (b) resume with
  a binary whose ALREADY-APPLIED step bytes differ and prove `deploy_plan_binary_mismatch`,
  NO apply, NO fingerprint write, NO `complete` (M1); (c) boot a no-revoke-bundle binary
  against an incomplete pre-revoke cursor at `applied_owner == 20` and prove it does NOT
  mutate schema and does NOT self-record (BC-N2); (d) force the FMA-007 cross-bundle
  self-heal with 0021 loaded and prove 0021 is NOT applied and CREATE stays held (M2);
  (e) boot a revoke-embedding binary on a `complete` cursor with the flag OFF and a
  pending change and prove `awaiting_deploy_config`, no `ApplyMigrations`, no
  `RecordSchemaFingerprint`, `schema_state` unchanged, DB byte-identical (M3); (f) boot
  a no-revoke binary (flag OFF, `cursorState=none`) over a fresh `applied_owner == 0` DB
  and prove it SERVES the normal fresh bring-up (NOT wedged), and separately prove a
  `1 <= applied_owner < 20` DB halts `awaiting_owner_ddl` DB-untouched — the F18
  dual-cell assertion (M5/row-1); (g) boot a no-revoke binary (decoupled ON) over a
  `complete`/`applied_owner==0`/in-sync DB and prove it SERVES verify-only (NOT
  `awaiting_deploy`), and separately prove the same binary over a
  `complete`/`applied_owner==0`/out-of-sync DB halts `awaiting_deploy` — the M6 in-sync
  `complete`/`==0` assertion (rows 15/`==0`); and (h) boot a no-revoke binary (flag OFF)
  over a `complete`/`applied_owner==0`/in-sync DB and prove it SERVES legacy no-op
  (the idempotent `:399` rewrite) and NOT `awaiting_deploy` — the degenerate
  13/`==0`-in-sync subcase (F18 spy list); **(i) boot a revoke-embedding binary
  (decoupled ON) over a `complete`/`applied_owner==0`/in-sync DB and prove it SERVES
  verify-only (NOT `awaiting_deploy`), and separately prove the same binary over a
  `complete`/`applied_owner==0`/out-of-sync DB halts `awaiting_deploy` — the M7 in-sync
  row-16/`==0` assertion; and (j) boot a revoke-embedding binary (decoupled ON) over a
  `complete`/`applied_owner==20`/in-sync DB and prove it SERVES verify-only, and
  separately over a `complete`/`applied_owner==20`/out-of-sync DB halts `awaiting_deploy`
  — the M7 row-16/`==20` assertion (both are F18-parametric sub-cases)**; (k) prove the
  deploy receipt is hash-chained and `doctor` surfaces `schema_deploy_unrecorded`; (l) in
  a two-role cluster, run the activation deploy through to `complete` and prove
  runtime-object ownership + post-deploy `striatumd_rw` CREATE denial (C3).
- **Explicitly defer P5** (rehearsal receipt, expand/contract, fidelity tiering, clone
  mechanism = Q1/Q2) — P4 is the deployer + serve-boot decoupling + DDL revocation only.
  Note the verb-surface decisions that keep the P5 door open.
- Stay strictly inside the local-first product boundary (single-host, ONE Postgres, ONE
  daemon as the single writer).

Publish the spec only after confirming the ledger verdict cleared the gate.
