You are the **Holder** for the RFC 0142 P4 design run, and **THIS IS THE FIFTH
REVISION (v5)**. Four prior design runs ran this same falsification gate. v1
(`rfc-0142-p4-design`) returned `needs_revision` with three findings C1/C2/C3. v2
(`rfc-0142-p4-design-v2`) **resolved C1 and C2** but returned `needs_revision`
again with C3 still open and a new finding N1. v3 (`rfc-0142-p4-design-v3`)
**resolved C3 (ownership transfer, revoke-last)** and closed the immediate N1 hole
— both falsifiers conceded the C3 design-level answer — but returned
`needs_revision` again on two grounds BC-N1 + BC-N2. v4 (`rfc-0142-p4-design-v4`)
**resolved BOTH BC-N1 and BC-N2** (both v4 falsifiers explicitly conceded the v3
reproducers no longer reproduce, and C1/C2/C3 carried forward intact) but returned
`needs_revision` **again** — the gate's single allowed cycle — on two independent
NEW material challenges, each source-verified against current `main` and each
squarely in a correctness/safety core:

- **M1** — the v4 transcript/binary verification rule verifies **ONLY
  not-yet-applied** steps, so a resume binary whose **already-applied** step bytes
  differ (`B45 != A45`) passes the not-yet-applied check, is classified
  resume-off-stored-transcript rather than `deploy_plan_binary_mismatch`, completes
  the plan, and the C1 finalizer self-records the running binary's
  `ExpectedFingerprint()` (which hashes EMBEDDED FILE BYTES,
  `go/pkg/db/schema_drift.go:83-99`, while `LiveFingerprint` does NOT recompute from
  `schema_migrations`, `:145-160`) into the `schema_state` singleton and marks
  `complete` — converting a hybrid A-applied/B-expected deploy into an apparently
  in-sync B deploy (silent schema corruption). The v4 holder's own §8
  (`HOLDER.md:833-839`) concedes the already-applied check does not fire.
- **M2** — the v4 "0021 unreachable through `owner-ddl apply`" guarantee binds
  ONLY `applyPendingOwnerBundles` (`go/pkg/db/owner.go:309-320`); the FMA-007
  self-heal branch `ReapplyAllOwnerBundles` (reached from `ApplyOwnerBundles` on a
  cross-bundle dependency error, `owner.go:277-302`) re-runs **EVERY** loaded bundle
  regardless of recorded version (`owner.go:324-349`), and since the activation
  binary's `OwnerBundles()` must surface 0021 for `revokeEmbedded` detection, a
  cross-bundle dependency repair can commit `REVOKE CREATE ON SCHEMA striatumd FROM
  striatumd_rw` early — before the runtime ownership reconcile — recreating the
  #512-class CREATE lockout the revoke-last mechanism exists to prevent.

**Start from the v4 `HOLDER.md`** — it is a **required context doc**
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md`). Your
job is to REVISE that spec, not write a new one from scratch. The full M1/M2
analysis and the exact prescribed fixes are in the **v4 collaboration ledger**
(also a required context doc:
`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
— read its `findings:` block for M1 and M2, and **§4 "What the revision must
fix"**, in full). `SEED.md` pins the two binding constraints (M1 + M2), the
proactive hardening, and the section "Carried forward — resolved by v3/v4 (do NOT
reopen)" (BC-N1 + BC-N2 + C1 + C2 + C3).

Your revised spec **MUST resolve BOTH cycle-4 findings (M1 + M2) per their
prescribed fix**, do the **proactive hardening audit**, and **MUST carry forward
BC-N1, BC-N2, C1, C2, and C3 unregressed**. A revision that leaves M1 or M2 open —
or that merely *claims* a fix without a concrete sub-protocol — or that regresses a
carry-forward finding, has NOT cleared the gate. This is the gate's single allowed
revision cycle, so the cycle-5 falsifiers re-attack each finding specifically and a
second `needs_revision` ends the gate unCleared.

Read the required context docs in full first — `SEED.md`, the v4 `HOLDER.md`, and
the v4 collaboration ledger — plus the committed RFC
(`docs/rfcs/0142-safe-by-construction-database-change-deployment.md`, status
`accepted`, D258). Build on the exact anchors the v4 spec and the SEED anchor table
use; **re-verify them against current `main`** — in particular
`go/pkg/db/schema_drift.go:83-99` (`ExpectedFingerprint` hashes embedded bytes),
`:145-160` (`LiveFingerprint` no recompute), `:171-194` (`RecordSchemaFingerprint`),
`:239-274` (`CheckSchemaDrift`); and `go/pkg/db/owner.go:277-302` (`ApplyOwnerBundles`
self-heal), `:309-320` (`applyPendingOwnerBundles`), `:324-349`
(`ReapplyAllOwnerBundles`), `:196-224` (`OwnerBundles()` loader), `:23,35`
(`Latest`/`Required` = 20). The DDL-revoke bundle stays at the renumbered **0021**
ordinal (0020 is `0020_owner_bundle_watermark_read.sql`).

Publish the **revised (v5)** falsifiable implementation spec for RFC 0142 **P4 —
the one-shot deployer** as your `HOLDER.md` artifact. Make it concrete and
falsifiable, not a restatement of the RFC. Open with an auditable resolution map
(an "Addressing the design-v4 findings" subsection) so the falsifiers can verify
M1 and M2 are resolved and BC-N1/BC-N2/C1/C2/C3 are preserved, rather than infer it.

Hold the root reframe: **schema mutation must stop being an implicit side effect
of the serving process's restart and become an explicit, ordered, resumable,
provenance-tracked operation owned by a dedicated deployer** — so the serving
daemon can hold zero DDL privilege and a bad migration can never wedge the single
writer on boot.

Your spec MUST:

0. **Resolve both binding revision constraints — the gating requirement.**

   - **M1 (verify the FULL stored transcript on resume AND before the finalizer —
     Q3/R4 fingerprint correctness core).** The v4 spec verified only
     **not-yet-applied** steps' bytes, so a resume binary whose already-applied step
     bytes differ slips through and the C1 finalizer self-records the WRONG binary's
     `ExpectedFingerprint()` (`schema_drift.go:83-99`; `LiveFingerprint` no recompute
     `:145-160`; `RecordSchemaFingerprint` `:171-194`; `CheckSchemaDrift` `:239-274`),
     serving a hybrid as in-sync. **Fix (binding):**
     - On **every resume**, validate **EVERY** stored `deploy_plan` transcript step's
       `sha256` against the current binary's embedded bytes — **not only
       not-yet-applied steps**. If any stored step SHA differs (already-applied OR
       future), classify **`deploy_plan_binary_mismatch`** (typed halt, DB-untouched,
       apply nothing).
     - For **already-applied** transcript entries, **also verify the DATABASE STAMPS**
       (`schema_migrations.sha256` for runtime steps, `owner_bundle_meta.sha256` for
       owner steps) match the stored transcript. A mismatch refuses to finalize — never
       a silent resume.
     - Apply the **SAME full-transcript check BEFORE the C1 finalizer** writes
       `schema_state` or advances `finalizing → complete`, so a binary that disagrees
       with an already-applied step can **NEVER** self-record its own
       `ExpectedFingerprint()` as in-sync. The already-applied verification fires BEFORE
       any self-record / fingerprint write.
     - Add **`T-deploy-resume-already-applied-byte-mismatch-refuses`** (extend
       F4/F14/F13): kill after step 0 commits; resume with a binary whose step-0 bytes
       differ but whose remaining steps match; assert `deploy_plan_binary_mismatch`, NO
       step 1 apply, NO `RecordSchemaFingerprint`, NO `complete` cursor, and a non-green
       doctor / typed diagnostic. Add the **symmetric owner-step** case (completed owner
       prefix / pre-finalization crash, owner-bundle SHA divergence).

   - **M2 (exclude the DDL-revoke bundle 0021 from EVERY owner-ddl apply route —
     C3/R3 migration-safety core).** The v4 spec bound only `applyPendingOwnerBundles`;
     the FMA-007 self-heal branch `ReapplyAllOwnerBundles` (`owner.go:277-302,324-349`)
     re-runs every loaded bundle regardless of recorded version and can commit 0021's
     `REVOKE CREATE` early. **Fix (binding):**
     - Define a **single non-revoke bundle filter** (exclude **every** bundle `>=
       DDLRevokeOwnerBundleVersion` = 0021, regardless of recorded version) and bind it
       to **EVERY** `owner-ddl apply` route: `applyPendingOwnerBundles`,
       **`ReapplyAllOwnerBundles`** (the FMA-007 self-heal), the `ApplyOwnerBundles`
       self-heal branch, tests, and any dry-run / list surfaces. State it as an
       **executable, named requirement** (not a generic build-guard sentence), so 0021's
       REVOKE can ONLY ever be committed as the terminal `deploy` step and NEVER via a
       self-heal reapply.
     - **Split the embed/listing helper** so "the binary embeds 0021" (for
       `revokeEmbedded` / `ExpectedFingerprint`) does **NOT** imply "`owner-ddl apply`
       iterates 0021": the deploy plan and `revokeEmbedded` may see 0021; the
       `owner-ddl apply` bundle slice must not.
     - Add **`T-deploy-revoke-excluded-from-reapply-self-heal`**: embed a synthetic 0021;
       force the cross-bundle dependency self-heal path; run `ApplyOwnerBundles`; assert
       0021 is NOT applied, `owner_bundle_meta` never records 21, and
       `has_schema_privilege('striatumd_rw','striatumd','CREATE')` remains TRUE.
     - Extend **F12 / `G-revoke-last`** with an `owner-ddl apply` side-path case
       (activation binary, owner frontier 20, `owner-ddl apply` before deploy, including
       the FMA-007 reapply branch) proving 0021 stays pending for the deploy plan and the
       subsequent runtime ownership reconcile can still run while CREATE is held.

   - **Proactive hardening (do this ONCE, exhaustively, to preempt the next cycle).**
     Audit EVERY code path that (a) applies owner bundles
     (`applyPendingOwnerBundles`, `ReapplyAllOwnerBundles`, the `ApplyOwnerBundles`
     self-heal branch, any `owner-ddl apply` dry-run/list surface, any test helper) and
     (b) self-records / writes a schema fingerprint (`RecordSchemaFingerprint`, the C1
     finalizer's `schema_state` UPSERT + `finalizing → complete` advance, any serve-boot
     self-record, any test helper). **Name each path against current `main` (file:line).**
     State the two **universal invariants** as executable, named requirements: *the
     DDL-revoke bundle (`>= DDLRevokeOwnerBundleVersion` = 0021) is excluded from ALL
     apply paths — only the terminal deploy step commits it*, and *NO fingerprint /
     `schema_state` `complete` is ever written unless the FULL stored transcript (every
     step, applied and not) byte-matches the running binary, with the already-applied DB
     stamps verified*.

   Explicitly call out, in the revised spec, **how** M1 and M2 are now closed, and
   **confirm** BC-N1 (the immutable `deploy_plan` transcript materialized before step
   0, resume off the stored transcript, §1.3 + per-step doctor keyed to it, F14), BC-N2
   (the universal `revokeEmbedded`-independent `CheckDeployActivation` edge halting
   `awaiting_deploy` at `applied_owner == 20` before `ApplyMigrations`/
   `RecordSchemaFingerprint`, F11(e)/(f) + extended `G-old-binary-refuse`), C1 (the
   `finalizing` state + idempotent finalizer + §1.3 row + F10), C2
   (`CheckDeployActivation` after `CheckOwnerBundleWatermark` and before
   `ApplyMigrations`, the typed halts, the forward-watermark rule at `applied >= 21`,
   `RequiredOwnerBundleVersion` kept at 20 NOT advanced to the revoke ordinal), and C3
   (the DDL-revoke bundle 0021 special-cased out of the owner prefix and excluded from
   `owner-ddl apply`, applied as the terminal plan step after every runtime ownership
   reconcile, F12/`G-revoke-last`) are carried forward **verbatim from the v4 HOLDER**
   and not regressed. Keep C1, BC-N1, M1, and the finalizer coherent — the §1.3 table,
   the `finalizing` finalizer, the stored-transcript receipt key, the new
   full-transcript verification, and the pre-revoke serve edge must all hold together.

1. **Keep Q3 and Q4 resolved.** Q3 — the per-step-atomic + resumable-cursor contract
   (now including the FULL-transcript binary/DB-stamp verification on resume and before
   the finalizer, M1) is sufficient for every owner+runtime interleaving P4 ships. Q4 —
   plain verb now with the three run-shape seams. Carry both forward; do not
   re-litigate.

2. **Keep the deployer surface and the serve-boot decoupling intact** (carry forward
   from v4): the `striatum daemon deploy` command site
   (`go/pkg/cli/localcommands/daemon.go`); the embed-FS-derived deploy plan with the
   immutable stored transcript (BC-N1) and the M1 full-transcript verification; the
   `deploy_plan`/`deploy_cursor` runtime migration (≥ 0044); the hash-chained deploy
   receipt into the owner-held `audit_log`; the lift of `ApplyMigrations` out of
   `go/pkg/db/connection.go` `ConnectAndMigrate` / `ConnectAndVerify` with the P2
   watermark interlock, the P3 drift gate, and the BC-N2 universal cursor edge intact.

3. **Keep the serving-role DDL revocation (the 0021 owner bundle)** — special-cased and
   sequenced terminal per C3, and now excluded from EVERY `owner-ddl apply` route
   including the FMA-007 self-heal (M2). State exactly how it ships without lockout, with
   the embed/listing helper split.

4. **State each load-bearing claim as a falsifiable assertion + its named test /
   game-day step.** Carry F1–F14 + `G-revoke-last` + `G-old-binary-refuse` forward
   (re-confirm and re-anchor), and ensure the M1 and M2 tests are present and sharp:
   `T-deploy-resume-already-applied-byte-mismatch-refuses` (already-applied byte/DB-stamp
   mismatch → `deploy_plan_binary_mismatch`, no apply, no fingerprint, no `complete`) and
   `T-deploy-revoke-excluded-from-reapply-self-heal` (forced FMA-007 self-heal with 0021
   loaded → 0021 not applied, CREATE still held).

5. **Stay inside the product boundary and the accepted design.** Local-first,
   single-host, ONE Postgres, ONE daemon as the single writer. Do NOT pull in P5
   (rehearsal receipt / expand-contract / fidelity tiering / clone = Q1/Q2).
   Shadow-first for the new path: default OFF behind a flag, additive migrations only,
   self-record before enforce.

Do not treat falsifier completion as acceptance — the adjudicator's
collaboration ledger decides whether the gate clears.
