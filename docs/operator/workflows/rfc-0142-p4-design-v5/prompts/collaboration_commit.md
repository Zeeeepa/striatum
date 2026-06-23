You are the **Committer** for the RFC 0142 P4 design run (**v5 / REVISION**). The
adjudicator's collaboration ledger has cleared the gate. Publish the final,
falsification-hardened **implementation spec** as your `PROPOSAL.md` artifact —
this is the design run's primary deliverable, the spec the `rfc-0142-p4-build` run
will build contract-first.

Start from the **revised (v5)** Holder's `HOLDER.md` and fold in every challenge
the adjudicator recorded as material-and-incorporated. Because this is the v5
revision run, the committed spec MUST carry **all seven** fixes as first-class,
build-ready design — the two cycle-4 fixes resolved this cycle and the five earlier
fixes carried forward:

- **M1 — the FULL-stored-transcript byte + DB-stamp verification** (on every resume,
  validate EVERY stored transcript step's `sha256` against the running binary's embedded
  bytes — already-applied AND not-yet-applied — and the already-applied DB stamps
  `schema_migrations.sha256`/`owner_bundle_meta.sha256`; classify ANY mismatch as
  `deploy_plan_binary_mismatch` DB-untouched; run the SAME check BEFORE the C1 finalizer
  writes `schema_state` or advances `finalizing → complete`, so a hybrid deploy can never
  be self-recorded as in-sync) with `T-deploy-resume-already-applied-byte-mismatch-refuses`
  plus the symmetric owner-step case.
- **M2 — the single non-revoke filter across EVERY owner-ddl apply route** (exclude every
  bundle `>= DDLRevokeOwnerBundleVersion` = 0021, regardless of recorded version, bound to
  `applyPendingOwnerBundles`, `ReapplyAllOwnerBundles` / the FMA-007 self-heal,
  `ApplyOwnerBundles`, tests, and dry-run/list; the embed/listing helper split so 'binary
  embeds 0021' does not imply 'owner-ddl apply iterates 0021') with
  `T-deploy-revoke-excluded-from-reapply-self-heal` and the extended F12/`G-revoke-last`
  owner-ddl side-path case.
- **BC-N1 — the immutable-transcript per-step deploy receipt protocol** carried forward
  (the whole target deploy's ordered transcript persisted in `deploy_plan` BEFORE step 0;
  resume off the STORED transcript by `plan_hash` rather than recomputing the pending-delta
  plan; §1.3 and `doctor schema_deploy_unrecorded` keyed off the stored transcript so
  `(plan_hash, step_index)` is a stable identity) with
  `T-deploy-plan-hash-resume-after-step` (F14).
- **BC-N2 — the universal pre-revoke `deploy_cursor`-authoritative serve edge** carried
  forward (`CheckDeployActivation` not gated on `revokeEmbedded`; every deployer-aware
  binary, INCLUDING the no-revoke-bundle binary at `applied_owner == 20`, reads
  `deploy_cursor` before `ApplyMigrations` and before `RecordSchemaFingerprint` and halts
  `awaiting_deploy` DB-untouched on a non-`complete` cursor) with the extended F11(e)/(f)
  + `G-old-binary-refuse`.
- **C1 — the finalization-boundary sub-protocol** carried forward (the `finalizing` state
  + idempotent finalizer + the §1.3 resumable-finalization row +
  `T-deploy-resume-finalization-crash`), now gated by the M1 pre-finalizer full-transcript
  check.
- **C2 — the fail-closed revoke-bundle + `STRIATUM_DEPLOY_DECOUPLED` activation edge**
  carried forward (typed pre-`ApplyMigrations` halt + forward-watermark rule at `applied
  >= 21` + `RequiredOwnerBundleVersion = 20` + `T-deploy-revoke-activation-ordering`).
- **C3 — the chosen, revoke-compatible runtime-object ownership policy** carried forward
  (revoke-last: the DDL-revoke bundle 0021 special-cased out of the owner prefix and
  excluded from `owner-ddl apply`, applied as the terminal plan step after every runtime
  ownership reconcile while `striatumd_rw` still holds CREATE;
  `T-deploy-runtime-object-ownership` / F12 asserting catalog owner + real `striatumd_rw`
  DML + post-deploy CREATE denial in a non-superuser two-role cluster; `G-revoke-last`).
  Keep the DDL-revoke bundle at the renumbered **0021** ordinal — 0020 is
  `0020_owner_bundle_watermark_read.sql` and `LatestOwnerBundleVersion == 20`.

…and carry the **proactive hardening** (every owner-bundle apply path and every
fingerprint/self-record path named against current `main`, with the two universal
invariants stated), and not regress Q4, the per-step Q3-A/Q3-B body, or the
shadow-first decoupling shape. The committed spec MUST:

- **Resolve Q3 and Q4** with the decided mechanism: the per-step-atomic +
  resumable-cursor contract and the `deploy_cursor` state machine (Q3), now including
  the immutable stored transcript (BC-N1), the pre-revoke serve edge (BC-N2), and the
  full-transcript binary/DB-stamp verification on resume and before the finalizer (M1);
  and the plain-verb-vs-run decision with its handling of the bootstrapping paradox (Q4).
- **Name the exact surfaces:** the `striatum daemon deploy` command site
  (`go/pkg/cli/localcommands/daemon.go`); the deploy plan form (ordered, role-tagged,
  dependency-edged, with the stored immutable transcript and the C3 terminal ordering for
  the 0021 DDL-revoke bundle); the new `deploy_cursor` / `deploy_plan` runtime migration
  (≥ 0044, additive); the hash-chained deploy receipt into the owner-held `audit_log`
  (with the per-step receipt keyed on the stored-transcript `(plan_hash, step_index)`);
  the precise edit to `go/pkg/db/connection.go` `ConnectAndMigrate` / `ConnectAndVerify`
  that lifts `ApplyMigrations` out of serve-boot while keeping the P2 watermark interlock
  and P3 drift gate intact AND adds the pre-revoke `deploy_cursor` read; the
  full-transcript verification surface (`go/pkg/db/schema_drift.go`
  `ExpectedFingerprint`/`LiveFingerprint`/`RecordSchemaFingerprint`/`CheckSchemaDrift`)
  and the M1 pre-finalizer check; the non-revoke owner-bundle filter bound across
  `go/pkg/db/owner.go` `applyPendingOwnerBundles` / `ReapplyAllOwnerBundles` /
  `ApplyOwnerBundles` and the split embed/listing helper; and the owner bundle 0021 that
  revokes serving-role DDL — sequenced terminal per C3 so it never locks out the runtime
  path nor strands the deploy nor opens a pre-revoke serve window nor commits early via a
  self-heal reapply.
- **Specify the build slices in contract-first order** (smallest safe first), each with
  its named Go tests and the exact migration/owner-bundle additions. Be shadow-first: the
  new deploy path defaults OFF behind a flag; serve-boot auto-apply stays the behavior
  until the deployer is proven, then flips; additive migrations only; self-record before
  enforce.
- **State the explicit Acceptance Criteria** an impl-run + verify-run must meet,
  including the mandatory **game-day fire tests**: (a) kill `striatum daemon deploy`
  after a committed step and prove clean resume with a stable `plan_hash`/`step_index`
  and exactly-once per-step receipt off the stored transcript (BC-N1); (b) resume with a
  binary whose ALREADY-APPLIED step bytes differ and prove `deploy_plan_binary_mismatch`,
  NO apply, NO fingerprint write, NO `complete` (M1); (c) boot a no-revoke-bundle binary
  against an incomplete pre-revoke cursor at `applied_owner == 20` and prove it does NOT
  mutate schema and does NOT self-record (BC-N2); (d) force the FMA-007 cross-bundle
  self-heal with 0021 loaded and prove 0021 is NOT applied and CREATE stays held (M2);
  (e) prove the deploy receipt is hash-chained and `doctor` surfaces
  `schema_deploy_unrecorded`; (f) in a two-role cluster, run the activation deploy through
  to `complete` and prove runtime-object ownership + post-deploy `striatumd_rw` CREATE
  denial (C3).
- **Explicitly defer P5** (rehearsal receipt, expand/contract, fidelity tiering, clone
  mechanism = Q1/Q2) — P4 is the deployer + serve-boot decoupling + DDL revocation only.
  Note the verb-surface decisions that keep the P5 door open.
- Stay strictly inside the local-first product boundary (single-host, ONE Postgres, ONE
  daemon as the single writer).

Publish the spec only after confirming the ledger verdict cleared the gate.
