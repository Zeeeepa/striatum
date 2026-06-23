You are the **Holder** for the RFC 0142 P4 design run, and **THIS IS THE SIXTH
REVISION (v6).** Five prior design runs ran this same falsification gate. v1
(`rfc-0142-p4-design`) returned `needs_revision` with three findings C1/C2/C3. v2
(`rfc-0142-p4-design-v2`) **resolved C1 and C2** (C3 still open + new finding N1).
v3 (`rfc-0142-p4-design-v3`) **resolved C3 (ownership transfer, revoke-last)** and
closed the immediate N1 hole — both falsifiers conceded C3 — but returned
`needs_revision` on two grounds BC-N1 + BC-N2. v4 (`rfc-0142-p4-design-v4`)
**resolved BOTH BC-N1 and BC-N2** but returned `needs_revision` on two NEW material
challenges M1 + M2. v5 (`rfc-0142-p4-design-v5`) **resolved BOTH M1 and M2** — both
v5 falsifiers AND the v5 adjudicator explicitly conceded each, and
BC-N1/BC-N2/C1/C2/C3 carried forward intact — but returned `needs_revision`
**again** — the gate's single allowed cycle — on two new findings, the load-bearing
one source-verified against current `main`:

- **M3 (LOAD-BEARING)** — the COMPLETE-cursor window lets the LEGACY
  `ConnectAndMigrate` path mutate + self-record AROUND the M1 `VerifyStoredTranscript`
  gate. §3.3a `CheckDeployActivation` returns nil immediately when `cursorState ==
  complete` (defers to the drift gate, v5 `HOLDER.md:480-482`), and the `revokeEmbedded
  && !decoupledEnabled → awaiting_deploy_config` halt lives ONLY in the `cursorState ==
  none` branch (v5 `HOLDER.md:483-489`). So a deployer-aware/revoke-embedding binary
  with a `complete` cursor and `STRIATUM_DEPLOY_DECOUPLED` OFF takes the legacy path
  over a DB that DOES carry `deploy_cursor`/`deploy_plan`; current source runs
  `ApplyMigrations` (`go/pkg/db/connection.go:353`) BEFORE `CheckSchemaDrift`
  (`:376-383`) and `RecordSchemaFingerprint` (`:399`), so the predicate returning nil
  lets the legacy mutator AND the legacy self-record fire WITHOUT
  `VerifyStoredTranscript`. Harm: (a) a pending runtime step needing CREATE hits a
  #512-class lockout AFTER 0021 revoked CREATE (the failure P4's root reframe declares
  structurally impossible); (b) a step needing no CREATE still mutates schema on
  serve-boot (the one-shot-deployer boundary regresses); (c) in shadow mode the
  post-apply drift gate logs and falls through to `RecordSchemaFingerprint`
  (`connection.go:384-399`), overwriting `schema_state` around the M1 gate — directly
  FALSIFYING §4.5 Universal Invariant B (v5 `HOLDER.md:800-806`). This is the orthogonal
  COMPLETE-cursor window, NOT a BC-N2 regression; the v5 holder's own §8
  (`HOLDER.md:907-914`) raises it but its "intended close" covers only NON-`complete`
  cursors.
- **M4 (secondary)** — F16's `TestOwnerDDLApplyExcludesRevokeBundle` is specified to
  assert production `OwnerBundles()` CONTAINS 0021 in rollout step 2 (v5
  `HOLDER.md:439-442,845-849`), but 0021 is not authored until rollout step 7 (v5
  `HOLDER.md:870-872`) — cannot build green as written. A test-staging inconsistency,
  not a safety gap.

**Start from the v5 `HOLDER.md`** — it is a **required context doc**
(`docs/operator/artifacts/rfc-0142-p4-design-v5/dialogue/holder/HOLDER.md`). Your job
is to REVISE that spec, not write a new one from scratch. The full M3/M4 analysis and
the exact prescribed fixes are in the **v5 collaboration ledger** (also a required
context doc:
`docs/operator/artifacts/rfc-0142-p4-design-v5/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
— read its `findings:` block for M3 and M4, and **§3 + §4 "What the revision must
fix"**, in full). `SEED.md` pins the two binding constraints (M3 + M4), the
proactive-completeness boot-path decision table, and the section "Carried forward —
resolved by v5 (do NOT reopen)" (M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3).

Your revised spec **MUST resolve BOTH cycle-1 findings (M3 + M4) per their prescribed
fix**, **produce the proactive-completeness boot-path decision table**, and **MUST
carry forward M1, M2, BC-N1, BC-N2, C1, C2, and C3 unregressed**. A revision that
leaves M3 or M4 open — or that merely *claims* a fix without a concrete sub-protocol —
or that regresses a carry-forward finding — or whose decision table has a cell where
Universal Invariant B fails — has NOT cleared the gate. This is the gate's single
allowed revision cycle, so the cycle-1 falsifiers re-attack each finding specifically
and a second `needs_revision` ends the gate unCleared.

Read the required context docs in full first — `SEED.md`, the v5 `HOLDER.md`, and the
v5 collaboration ledger — plus the committed RFC
(`docs/rfcs/0142-safe-by-construction-database-change-deployment.md`, status
`accepted`, D258). Build on the exact anchors the v5 spec and the SEED anchor table
use; **re-verify them against current `main`** — in particular the
`ConnectAndMigrate` boot order `go/pkg/db/connection.go:349` (`CheckOwnerBundleWatermark`)
→ `:353` (`ApplyMigrations`) → `:376-383` (`CheckSchemaDrift`) → `:384-399` (shadow
fall-through + self-record) → `:399` (`RecordSchemaFingerprint`); and the v5 predicate
sites `HOLDER.md:480-482` (the `cursorState == complete` defer) and `:483-489` (the
`none`-branch-only halt). The DDL-revoke bundle stays at the renumbered **0021**
ordinal (0020 is `0020_owner_bundle_watermark_read.sql`, `LatestOwnerBundleVersion ==
20`).

Publish the **revised (v6)** falsifiable implementation spec for RFC 0142 **P4 — the
one-shot deployer** as your `HOLDER.md` artifact. Make it concrete and falsifiable,
not a restatement of the RFC. Open with an auditable resolution map (an "Addressing
the design-v5 findings" subsection) so the falsifiers can verify M3 and M4 are
resolved and M1/M2/BC-N1/BC-N2/C1/C2/C3 are preserved, rather than infer it.

Hold the root reframe: **schema mutation must stop being an implicit side effect of
the serving process's restart and become an explicit, ordered, resumable,
provenance-tracked operation owned by a dedicated deployer** — so the serving daemon
can hold zero DDL privilege and a bad migration can never wedge the single writer on
boot. M3 is exactly a residual violation: the legacy `ApplyMigrations`-on-serve-boot
path is still reachable for a revoke-embedding binary on a `complete` cursor.

Your spec MUST:

0. **Resolve both binding revision constraints — the gating requirement.**

   - **M3 (close the COMPLETE-cursor legacy self-record / mutation bypass —
     C2/decoupling-boundary + M1-Invariant-B core).** Make `CheckDeployActivation`
     enforce the revoke-embedding/decoupled guard in the `cursorState == complete`
     branch too. **Fix (binding):**
     - Make `revokeEmbedded && !decoupledEnabled` a **pre-apply, DB-untouched halt for
       EVERY cursor state, including `complete`** — the conservative rule: a
       revoke-embedding binary with `STRIATUM_DEPLOY_DECOUPLED` OFF returns
       **`awaiting_deploy_config`**, DB untouched, before `ApplyMigrations` and before
       `RecordSchemaFingerprint`, on **both** `ConnectAndMigrate` and `ConnectAndVerify`.
       (If the design instead wants to permit a flag-OFF restart after a completed
       deploy, it MUST add a **pre-`ApplyMigrations`** plan/fingerprint comparison that
       cannot mutate or self-record — NOT rely on the current **post-apply**
       `CheckSchemaDrift`.)
     - **Tighten Universal Invariant B** so a database carrying `deploy_cursor` /
       `deploy_plan` can **NEVER** reach the legacy `connection.go:399` writer: the
       legacy self-record is permitted only when no deploy transcript exists and the
       binary is not on the P4 revoke/deploy path.
     - Extend **F11 / F15** with the complete-cursor case: `cursorState == complete`,
       `revokeEmbedded == true`, `STRIATUM_DEPLOY_DECOUPLED` OFF, with a pending runtime
       migration (or a changed expected fingerprint). Assert `awaiting_deploy_config`,
       `ApplyMigrations` **not** called (spy), `RecordSchemaFingerprint` **not** called
       (spy), `schema_state` unchanged, and the DB byte-identical. Name the test e.g.
       **`T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord`**.

   - **M4 (split F16 phase-aware so M2's filters land green before 0021 exists —
     secondary, test-staging).** **Fix (binding):**
     - **Pre-0021 / inert phase (rollout step 2):** use a synthetic bundle list / test
       hook to prove `OwnerDDLApplyBundles` / `isNonRevokeBundle` exclude every bundle
       `>= 21`, `applyPendingOwnerBundles` AND `ReapplyAllOwnerBundles` skip a
       hand-passed synthetic 0021, and `ReapplyAllOwnerBundles(nil, …)` uses the filtered
       loader. Do **NOT** assert production `OwnerBundles()` contains 0021 yet.
     - **Activation phase (rollout step 7, after 0021 is authored):** assert production
       `OwnerBundles()` contains 0021, `ExpectedFingerprint()` includes its bytes,
       `revokeEmbedded` derives from the full loader / file presence, and production
       `OwnerDDLApplyBundles()` excludes it.
     - **Keep the forced-self-heal pgtest in the activation phase** (or make its
       synthetic fixture explicit) and require it to prove it actually reaches
       `ReapplyAllOwnerBundles` through `isCrossBundleDependencyError`
       (`go/pkg/db/owner.go:367-374`), not merely the pending loop.

   - **Proactive completeness — the boot-path decision table (do this ONCE,
     exhaustively, to preempt the next cycle).** For EVERY combination of `cursorState`
     in {none, in_progress, finalizing, complete} (treat `step_committed` / `aborted`
     per the §1.3 disambiguation) × `decoupledEnabled` in {on, off} × `revokeEmbedded`
     in {yes, no} × `applied_owner` in {<20, ==20, >=21}, specify the **exact guard /
     outcome** (halt `awaiting_deploy` / `awaiting_deploy_config`, run the deployer, run
     legacy `ConnectAndMigrate`, run `VerifyStoredTranscript`, serve, etc.) and **PROVE
     §4.5 Universal Invariant B holds in EVERY cell** — explicitly including the M3 cell
     (`complete` + decoupled OFF + revoke-embedding + a pending change), the shadow-mode
     drift-gate fall-through (`connection.go:384-399`), and the legitimate fresh-DB /
     inert-landing cells that must still serve and NOT be wedged. Make the table an
     **executable, named requirement** the falsifiers can verify against the predicate
     sites (v5 `HOLDER.md:480-489`) and the `connection.go:353/:376-383/:399` ordering.
     This closes M3 and preempts any further unguarded-combination challenge.

   Explicitly call out, in the revised spec, **how** M3 and M4 are now closed, and
   **confirm** M1 (the full-transcript `VerifyStoredTranscript` byte + DB-stamp verifier
   on resume AND as finalizer step 0, F15 + extended F14), M2 (the single non-revoke
   filter `OwnerDDLApplyBundles()`/`isNonRevokeBundle` across every `owner-ddl apply`
   route incl. the FMA-007 self-heal + the embed/listing split, F16 + F12/`G-revoke-last`),
   BC-N1 (the immutable `deploy_plan` transcript materialized before step 0, resume off
   the stored transcript, §1.3 + per-step doctor, F14), BC-N2 (the universal
   `revokeEmbedded`-independent `CheckDeployActivation` edge halting `awaiting_deploy` at
   `applied_owner == 20`, F11(e)/(f) + extended `G-old-binary-refuse`), C1 (the
   `finalizing` state + idempotent finalizer + §1.3 row + F10), C2 (`CheckDeployActivation`
   before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`,
   `RequiredOwnerBundleVersion` kept at 20), and C3 (the DDL-revoke bundle 0021
   special-cased + excluded from `owner-ddl apply` + applied terminal, F12/`G-revoke-last`)
   are carried forward **verbatim from the v5 HOLDER** and not regressed. Keep the §1.3
   table, the `finalizing` finalizer, the stored-transcript receipt key, the M1
   verifier, the BC-N2 non-complete edge, and the new M3 complete-cursor guard all
   coherent together.

1. **Keep Q3 and Q4 resolved.** Q3 — the per-step-atomic + resumable-cursor contract
   (now including M1's full-transcript verification AND M3's complete-cursor activation
   guard) is sufficient for every owner+runtime interleaving and boot-path combination
   P4 ships. Q4 — plain verb now with the three run-shape seams. Carry both forward; do
   not re-litigate.

2. **Keep the deployer surface and the serve-boot decoupling intact** (carry forward
   from v5): the `striatum daemon deploy` command site
   (`go/pkg/cli/localcommands/daemon.go`); the embed-FS-derived deploy plan with the
   immutable stored transcript (BC-N1) and the M1 full-transcript verification; the
   `deploy_plan`/`deploy_cursor` runtime migration (≥ 0044); the hash-chained deploy
   receipt into the owner-held `audit_log`; the lift of `ApplyMigrations` out of
   `go/pkg/db/connection.go` `ConnectAndMigrate` / `ConnectAndVerify` with the P2
   watermark interlock, the P3 drift gate, the BC-N2 universal non-complete cursor edge,
   and now the M3 complete-cursor guard intact.

3. **Keep the serving-role DDL revocation (the 0021 owner bundle)** — special-cased and
   sequenced terminal per C3, excluded from EVERY `owner-ddl apply` route including the
   FMA-007 self-heal (M2), and never reachable via the legacy serve-boot path on a
   `complete` cursor (M3). State exactly how it ships without lockout in any boot-path
   cell, with the embed/listing helper split.

4. **State each load-bearing claim as a falsifiable assertion + its named test /
   game-day step.** Carry F1–F16 + `G-revoke-last` + `G-old-binary-refuse` forward
   (re-confirm and re-anchor), and ensure the M3 test is present and sharp:
   `T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord`
   (revoke-embedding binary + `complete` cursor + flag OFF + pending change →
   `awaiting_deploy_config`, no `ApplyMigrations`, no `RecordSchemaFingerprint`,
   `schema_state` unchanged, DB byte-identical), and the M4 phase-aware F16 split is
   stated.

5. **Stay inside the product boundary and the accepted design.** Local-first,
   single-host, ONE Postgres, ONE daemon as the single writer. Do NOT pull in P5
   (rehearsal receipt / expand-contract / fidelity tiering / clone = Q1/Q2).
   Shadow-first for the new path: a no-revoke inert binary on a clean DB still serves;
   a revoke-embedding binary with the flag OFF over a deploy transcript halts, never
   auto-applies. Additive migrations only, self-record before enforce.

Do not treat falsifier completion as acceptance — the adjudicator's
collaboration ledger decides whether the gate clears.
