You are the **Committer** for the RFC 0142 P4 design run (**v7 / REVISION**). The
adjudicator's collaboration ledger has cleared the gate. Publish the final,
falsification-hardened **implementation spec** as your `PROPOSAL.md` artifact — this
is the design run's primary deliverable, the spec the `rfc-0142-p4-build` run will
build contract-first.

Start from the **revised (v7)** Holder's `HOLDER.md` and fold in every challenge the
adjudicator recorded as material-and-incorporated. Because this is the v7 revision
run, the committed spec MUST carry **all ten** fixes as first-class, build-ready
design — the cycle-1 fix resolved this cycle and the nine earlier fixes carried
forward:

- **M5 — the owner-watermark dimension split** (resolved this cycle): §3.5 and F18 split
  the `applied_owner` dimension into `{0/no authority, 1..19 authority shortfall, ==20,
  >=21}`; the no-transcript / no-revoke / flag-off bootstrap cell (`cursorState=none`,
  `decoupledEnabled=false`, `revokeEmbedded=false`, `applied_owner=0`) is **serve-legacy /
  fresh bootstrap** — `ApplyMigrations` and the legacy `connection.go:399` self-record MAY
  run because no deploy transcript exists (Invariant B not in scope), matching
  `CheckOwnerBundleWatermark`'s `applied == 0` exception (`go/pkg/db/owner.go:145`);
  `awaiting_owner_ddl` (DB untouched) is retained for `1 <= applied_owner < 20`; the split
  propagated through the other cursor rows; F18 (`T-deploy-bootpath-decision-table`)
  asserts BOTH the `applied_owner == 0` serve cell AND the `1..19` halt cell; cell `==20`
  is no longer mislabeled the "fresh-DB bring-up" cell.
- **M3 — the COMPLETE-cursor activation guard** carried forward: `CheckDeployActivation`
  enforces the `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config guard
  hoisted to step 0 (every cursor state incl. `complete`) before `ApplyMigrations`
  (`connection.go:353`) and before `RecordSchemaFingerprint` (`:399`) on both boot paths;
  the no-revoke `complete` residual runs a pre-`ApplyMigrations`, DB-untouched pure-read
  comparison; §4.5 Universal Invariant B tightened so a transcript-carrying DB never
  reaches the legacy `connection.go:399` writer; F17/F11(g)/F18.
- **M4 — the phase-aware F16 split** carried forward: F16a (synthetic-list pre-0021, step
  2) + F16b (production post-0021 + the forced FMA-007 self-heal reaching
  `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`, step 7).
- **M1 — the FULL-stored-transcript byte + DB-stamp verification** carried forward
  (`VerifyStoredTranscript` over every step on every resume AND as finalizer step 0, ANY
  mismatch → `deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch` DB-untouched;
  F15 + extended F14).
- **M2 — the single non-revoke filter across EVERY owner-ddl apply route** carried forward
  (`isNonRevokeBundle` / `OwnerDDLApplyBundles()` + in-loop guards on
  `applyPendingOwnerBundles` / `ReapplyAllOwnerBundles` / the FMA-007 self-heal + the
  nil-fallback split + the embed/listing split; F16 safety + F12/`G-revoke-last`).
- **BC-N1 — the immutable-transcript per-step deploy receipt protocol** carried forward
  (the `deploy_plan` table before step 0; resume off the STORED transcript by `plan_hash`;
  §1.3 + `doctor schema_deploy_unrecorded` keyed off the stored transcript; F14).
- **BC-N2 — the universal pre-revoke `deploy_cursor`-authoritative serve edge** carried
  forward (`CheckDeployActivation` not gated on `revokeEmbedded`; every deployer-aware
  binary halts `awaiting_deploy` DB-untouched on a non-`complete` cursor at `applied_owner
  == 20`; F11(e)/(f) + `G-old-binary-refuse`). M3 EXTENDS this edge to the complete-cursor
  case; M5 re-buckets the orthogonal owner-watermark dimension at W — neither weakens the
  non-complete `applied_owner == 20` edge.
- **C1 — the finalization-boundary sub-protocol** carried forward (the `finalizing` state +
  idempotent finalizer + the §1.3 resumable-finalization row + F10), gated by the M1
  pre-finalizer full-transcript check.
- **C2 — the fail-closed revoke-bundle + `STRIATUM_DEPLOY_DECOUPLED` activation edge**
  carried forward (typed pre-`ApplyMigrations` halt + forward-watermark rule at `applied >=
  21` + `RequiredOwnerBundleVersion = 20`). M3 extends this edge to the complete-cursor
  case and M5 re-buckets the owner-watermark dimension without advancing `Required` or
  altering the watermark.
- **C3 — the chosen, revoke-compatible runtime-object ownership policy** carried forward
  (revoke-last: the DDL-revoke bundle 0021 special-cased out of the owner prefix and excluded
  from `owner-ddl apply`, applied as the terminal plan step after every runtime ownership
  reconcile while `striatumd_rw` still holds CREATE; `T-deploy-runtime-object-ownership` /
  F12 + `G-revoke-last`). Keep the DDL-revoke bundle at the renumbered **0021** ordinal —
  0020 is `0020_owner_bundle_watermark_read.sql` and `LatestOwnerBundleVersion == 20`.

…and carry the **proactive-completeness boot-path decision table** (every `cursorState`
× `decoupledEnabled` × `revokeEmbedded` × `applied_owner ∈ {0, 1..19, ==20, >=21}` cell
with a specified guard/outcome, Universal Invariant B proven in each, AND the legitimate
fresh-DB cells still serving), and not regress Q4, the per-step Q3-A/Q3-B body, or the
shadow-first decoupling shape. The committed spec MUST:

- **Resolve Q3 and Q4** with the decided mechanism: the per-step-atomic +
  resumable-cursor contract and the `deploy_cursor` state machine (Q3), now including the
  immutable stored transcript (BC-N1), the universal pre-revoke serve edge (BC-N2), the
  full-transcript binary/DB-stamp verification on resume and before the finalizer (M1),
  the complete-cursor activation guard (M3), and the full boot-path decision table with
  the M5-correct owner-watermark dimension (the `applied_owner == 0` fresh-DB cell still
  serves); and the plain-verb-vs-run decision with its handling of the bootstrapping
  paradox (Q4).
- **Name the exact surfaces:** the `striatum daemon deploy` command site
  (`go/pkg/cli/localcommands/daemon.go`); the deploy plan form (ordered, role-tagged,
  dependency-edged, with the stored immutable transcript and the C3 terminal ordering for
  the 0021 DDL-revoke bundle); the new `deploy_cursor` / `deploy_plan` runtime migration
  (≥ 0044, additive); the hash-chained deploy receipt into the owner-held `audit_log`
  (per-step receipt keyed on the stored-transcript `(plan_hash, step_index)`); the precise
  edit to `go/pkg/db/connection.go` `ConnectAndMigrate` / `ConnectAndVerify` that lifts
  `ApplyMigrations` out of serve-boot while keeping the P2 watermark interlock and P3
  drift gate intact AND adds the `CheckDeployActivation` cursor read with the BC-N2
  non-complete edge, the M3 complete-cursor guard, AND the M5-correct owner-watermark
  dimension (the `applied_owner == 0` fresh-DB cell serves-legacy, matching
  `CheckOwnerBundleWatermark`'s `applied == 0` exception at `go/pkg/db/owner.go:145`); the
  full-transcript verification surface (`go/pkg/db/schema_drift.go`
  `ExpectedFingerprint`/`LiveFingerprint`/`RecordSchemaFingerprint`/`CheckSchemaDrift`) and
  the M1 `VerifyStoredTranscript` pre-finalizer check; the non-revoke owner-bundle filter
  (`isNonRevokeBundle` / `OwnerDDLApplyBundles()`) bound across `go/pkg/db/owner.go`
  `applyPendingOwnerBundles` / `ReapplyAllOwnerBundles` / `ApplyOwnerBundles` and the split
  embed/listing helper; and the owner bundle 0021 that revokes serving-role DDL — sequenced
  terminal per C3 so it never locks out the runtime path nor strands the deploy nor opens a
  pre-revoke serve window nor commits early via a self-heal reapply nor is reachable via the
  legacy serve-boot path on a `complete` cursor (M3).
- **Specify the build slices in contract-first order** (smallest safe first), each with
  its named Go tests and the exact migration/owner-bundle additions, the M4 phase-aware
  F16 split mapped to the rollout steps, and the F18 boot-path decision-table test with the
  split `applied_owner` dimension. Be shadow-first: the new deploy path defaults OFF behind
  a flag; serve-boot auto-apply stays the behavior for a no-revoke inert binary on a clean
  DB (the `applied_owner == 0` cell serves), but a revoke-embedding binary with the flag OFF
  over a deploy transcript halts; additive migrations only; self-record before enforce.
- **State the explicit Acceptance Criteria** an impl-run + verify-run must meet,
  including the mandatory **game-day fire tests**: (a) kill `striatum daemon deploy` after a
  committed step and prove clean resume with a stable `plan_hash`/`step_index` and
  exactly-once per-step receipt off the stored transcript (BC-N1); (b) resume with a binary
  whose ALREADY-APPLIED step bytes differ and prove `deploy_plan_binary_mismatch`, NO apply,
  NO fingerprint write, NO `complete` (M1); (c) boot a no-revoke-bundle binary against an
  incomplete pre-revoke cursor at `applied_owner == 20` and prove it does NOT mutate schema
  and does NOT self-record (BC-N2); (d) force the FMA-007 cross-bundle self-heal with 0021
  loaded and prove 0021 is NOT applied and CREATE stays held (M2); (e) boot a
  revoke-embedding binary on a `complete` cursor with the flag OFF and a pending change and
  prove `awaiting_deploy_config`, no `ApplyMigrations`, no `RecordSchemaFingerprint`,
  `schema_state` unchanged, DB byte-identical (M3); (f) **boot a no-revoke binary (flag OFF,
  `cursorState=none`) over a fresh `applied_owner == 0` DB and prove it SERVES the normal
  fresh bring-up (NOT wedged), and separately prove a `1 <= applied_owner < 20` DB halts
  `awaiting_owner_ddl` DB-untouched — the F18 dual-cell assertion (M5)**; (g) prove the
  deploy receipt is hash-chained and `doctor` surfaces `schema_deploy_unrecorded`; (h) in a
  two-role cluster, run the activation deploy through to `complete` and prove runtime-object
  ownership + post-deploy `striatumd_rw` CREATE denial (C3).
- **Explicitly defer P5** (rehearsal receipt, expand/contract, fidelity tiering, clone
  mechanism = Q1/Q2) — P4 is the deployer + serve-boot decoupling + DDL revocation only.
  Note the verb-surface decisions that keep the P5 door open.
- Stay strictly inside the local-first product boundary (single-host, ONE Postgres, ONE
  daemon as the single writer).

Publish the spec only after confirming the ledger verdict cleared the gate.
