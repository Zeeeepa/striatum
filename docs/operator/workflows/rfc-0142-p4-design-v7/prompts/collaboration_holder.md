You are the **Holder** for the RFC 0142 P4 design run, and **THIS IS THE SEVENTH
REVISION (v7).** Six prior design runs ran this same falsification gate. v1
(`rfc-0142-p4-design`) returned `needs_revision` with three findings C1/C2/C3. v2
(`rfc-0142-p4-design-v2`) **resolved C1 and C2** (C3 still open + new finding N1).
v3 (`rfc-0142-p4-design-v3`) **resolved C3 (ownership transfer, revoke-last)** and
closed the immediate N1 hole — both falsifiers conceded C3 — but returned
`needs_revision` on two grounds BC-N1 + BC-N2. v4 (`rfc-0142-p4-design-v4`)
**resolved BOTH BC-N1 and BC-N2** but returned `needs_revision` on two NEW material
challenges M1 + M2. v5 (`rfc-0142-p4-design-v5`) **resolved BOTH M1 and M2** but
returned `needs_revision` on two new findings M3 + M4. v6 (`rfc-0142-p4-design-v6`)
**resolved BOTH M3 and M4** — both v6 falsifiers AND the v6 adjudicator explicitly
conceded each, and M1/M2/BC-N1/BC-N2/C1/C2/C3 carried forward intact — but returned
`needs_revision` **again** — the gate's single allowed cycle — on one new finding,
source-verified against current `main` and landed INDEPENDENTLY by BOTH v6 falsifiers:

- **M5 (LOAD-BEARING)** — the §3.5 / F18 proactive-completeness boot-path decision
  table collapses the owner-watermark dimension. It maps `applied_owner < 20 →
  awaiting_owner_ddl` UNIFORMLY (states the `<20` column "ALWAYS halts at W", v6
  `HOLDER.md:443-459,461-478`), and the prose mislabels cell `==20` as the "fresh-DB
  bring-up" cell (v6 `HOLDER.md:515-518`). But current source
  `CheckOwnerBundleWatermark` SERVES (returns nil) for `applied_owner == 0` — the
  fresh / single-role / no-authority bootstrap case — BEFORE the shortfall check
  (`go/pkg/db/owner.go:145`, the `if applied == 0 { return nil }` precedes the
  `if applied < RequiredOwnerBundleVersion` halt at `:148-150`; the function comment
  at `:116-123` + `:140-143`: a fresh 0-watermark DB "is treated as the
  bootstrap/single-role case and NOT halted. Only a database that HAS an authority
  schema (applied >= 1) but lags the required frontier is a genuine shortfall"), and
  `OwnerBundleVersion` returns 0 when `owner_bundle_meta` is absent
  (`owner.go:233-235`; `owner_pg_test.go:19` asserts a fresh DB starts at version 0).
  So the §3.5/F18 table either (1) **WEDGES** a legitimate fresh no-authority boot
  (`cursorState=none`, no-revoke, flag-off, `applied_owner=0`) the SEED requires to
  "still serve and NOT be wedged", regressing fresh / single-role bootstrap; OR (2)
  makes the **EXECUTABLE F18 oracle FALSE** for the `applied_owner == 0` cell. Either
  branch is a material table-correctness failure. This is **NOT** a re-opening of M3
  (the hoisted M3 gate is right for revoke-embedding binaries; the fresh-DB cell
  carries no transcript and no revoke, so Invariant B is not *violated* there) — the
  M5 failure is the OPPOSITE: an over-conservative halt of a cell that must serve. It
  is exactly the failure class the SEED's v6 proactive-hardening section warned an
  unaudited cell would spawn ("A re-scaffolded revision that pins exactly the two §4
  items but leaves an unaudited boot-path combination open will simply spawn an M5").

**Start from the v6 `HOLDER.md`** — it is a **required context doc**
(`docs/operator/artifacts/rfc-0142-p4-design-v6/dialogue/holder/HOLDER.md`). Your job
is to REVISE that spec, not write a new one from scratch. The full M5 analysis and
the exact prescribed fix are in the **v6 collaboration ledger** (also a required
context doc:
`docs/operator/artifacts/rfc-0142-p4-design-v6/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
— read its `findings:` block for M5, and **§3 + §4 "What the revision must fix" +
§5 "What already cleared"**, in full). `SEED.md` pins the single binding constraint
(M5), keeps the proactive-completeness boot-path decision table requirement (now with
the `applied_owner` dimension SPLIT), and the section "Carried forward — resolved by
v6 (do NOT reopen)" (M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3).

Your revised spec **MUST resolve the cycle-1 finding (M5) per its prescribed fix**,
**keep the proactive-completeness boot-path decision table (now with the split
dimension)**, and **MUST carry forward M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3
unregressed**. A revision that leaves M5 open — or that merely *claims* a fix without
a concrete dimension split + the dual-cell F18 assertion — or that regresses a
carry-forward finding — or whose decision table has a cell where Universal Invariant B
fails OR where a legitimate fresh-DB cell is wedged — has NOT cleared the gate. This is
the gate's single allowed revision cycle, so the cycle-1 falsifiers re-attack each
finding specifically and a second `needs_revision` ends the gate unCleared.

Read the required context docs in full first — `SEED.md`, the v6 `HOLDER.md`, and the
v6 collaboration ledger — plus the committed RFC
(`docs/rfcs/0142-safe-by-construction-database-change-deployment.md`, status
`accepted`, D258). Build on the exact anchors the v6 spec and the SEED anchor table
use; **re-verify them against current `main`** — in particular the M5 owner-watermark
sites: `go/pkg/db/owner.go:145` (`if applied == 0 { return nil }`, BEFORE the
`if applied < RequiredOwnerBundleVersion` shortfall at `:148-150`), the comment block
`:116-123` + `:140-143`, `OwnerBundleVersion` returning 0 for an absent
`owner_bundle_meta` (`:233-235`), `owner_pg_test.go:19` (fresh DB version 0), and
`owner.go:23` `LatestOwnerBundleVersion = 20` / `:35`
`RequiredOwnerBundleVersion = LatestOwnerBundleVersion`. Also re-verify the
`ConnectAndMigrate` boot order `go/pkg/db/connection.go:349` (`CheckOwnerBundleWatermark`)
→ `:353` (`ApplyMigrations`) → `:376-383` (`CheckSchemaDrift`) → `:384-399`
(shadow fall-through + self-record) → `:399` (`RecordSchemaFingerprint`). The
DDL-revoke bundle stays at the renumbered **0021** ordinal (0020 is
`0020_owner_bundle_watermark_read.sql`, `LatestOwnerBundleVersion == 20`).

Publish the **revised (v7)** falsifiable implementation spec for RFC 0142 **P4 — the
one-shot deployer** as your `HOLDER.md` artifact. Make it concrete and falsifiable,
not a restatement of the RFC. Open with an auditable resolution map (an "Addressing
the design-v6 findings" subsection) so the falsifiers can verify M5 is resolved and
M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 are preserved, rather than infer it.

Hold the root reframe: **schema mutation must stop being an implicit side effect of
the serving process's restart and become an explicit, ordered, resumable,
provenance-tracked operation owned by a dedicated deployer** — so the serving daemon
can hold zero DDL privilege and a bad migration can never wedge the single writer on
boot. M5 is the OTHER edge of that reframe: the fresh / single-role / no-authority
bootstrap (`applied_owner == 0`, no transcript, no revoke) is the one legitimate
serve-legacy cell — the reframe must NOT wedge a healthy first boot.

Your spec MUST:

0. **Resolve the single binding revision constraint — the gating requirement.**

   - **M5 (split the owner-watermark dimension so the decision table matches the live
     bootstrap contract — decision-table completeness).** **Fix (binding):**
     - In §3.5 and F18, replace the single `applied_owner < 20` bucket with
       `applied_owner ∈ {0/no authority, 1..19 authority shortfall, ==20, >=21}`.
     - Specify the no-transcript / no-revoke / flag-off bootstrap cell
       (`cursorState=none`, `decoupledEnabled=false`, `revokeEmbedded=false`,
       `applied_owner=0`) as **serve-legacy / fresh bootstrap**: `ApplyMigrations` and
       the legacy `connection.go:399` self-record MAY run because **NO deploy transcript
       exists** (Universal Invariant B is NOT in scope), exactly matching
       `CheckOwnerBundleWatermark`'s `applied == 0` exception (`owner.go:145`). Retain
       `awaiting_owner_ddl` (DB untouched) for `1 <= applied_owner < 20`.
     - Propagate the split through the other cursor rows so the table stays executable,
       and make **F18** assert **BOTH** branches explicitly (the `applied_owner == 0`
       serve cell AND the `1..19` halt cell), so the matrix oracle matches source
       without changing the bootstrap contract.
     - Stop labeling cell `==20` the "fresh-DB bring-up" cell (v6 `HOLDER.md:515-518`);
       the genuine fresh no-authority DB is `applied_owner == 0`.

   - **Preserve the asymmetry (the v6 ledger's §4 note).** The M3 halt is conservative
     ON PURPOSE for a revoke-embedding binary (decoupling becomes mandatory once the
     binary embeds 0021), but the watermark `<20` halt must **NOT** be conservative for
     `applied_owner == 0` — that cell is a legitimate fresh serve, and over-halting it
     is the M5 defect. The M5 fix ONLY re-buckets the `applied_owner` dimension; it must
     NOT weaken the M3 config gate, regress the BC-N2 `applied_owner == 20` edge, advance
     `RequiredOwnerBundleVersion`, or alter the watermark.

   - **Proactive completeness — keep the boot-path decision table (do this ONCE,
     exhaustively, with the split dimension, to preempt the next cycle).** For EVERY
     combination of `cursorState` in {none, in_progress, finalizing, complete} (treat
     `step_committed` / `aborted` per the §1.3 disambiguation) × `decoupledEnabled` in
     {on, off} × `revokeEmbedded` in {yes, no} × `applied_owner` in **{0/no authority,
     1..19 authority shortfall, ==20, >=21}**, specify the **exact guard / outcome**
     (halt `awaiting_owner_ddl` / `awaiting_deploy` / `awaiting_deploy_config`, run the
     deployer, run legacy `ConnectAndMigrate` / serve-legacy, run `VerifyStoredTranscript`,
     serve, etc.) and **PROVE §4.5 Universal Invariant B holds in EVERY cell** — AND prove
     the legitimate fresh-DB / inert-landing cells (`applied_owner == 0`, no-revoke, no
     transcript) **STILL SERVE and are NOT wedged** — explicitly keeping the M3 cell
     (`complete` + decoupled OFF + revoke-embedding + a pending change) and the
     shadow-mode drift-gate fall-through (`connection.go:384-399`) covered unchanged.
     Make the table an **executable, named requirement** the falsifiers can verify
     against `CheckOwnerBundleWatermark`'s `applied == 0` exception (`owner.go:145`) and
     the `connection.go:353/:376-383/:399` ordering. This closes M5 and preempts any
     further unguarded-combination challenge (an M6).

   Explicitly call out, in the revised spec, **how** M5 is now closed, and **confirm**
   M3 (the hoisted step-0 `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config`
   config gate + the no-revoke `complete` pre-`ApplyMigrations` pure-read comparison +
   tightened Invariant B + F17/F11(g)/F18), M4 (the F16a synthetic-list / F16b
   production phase split + the forced FMA-007 self-heal pgtest in F16b), M1 (the
   full-transcript `VerifyStoredTranscript` byte + DB-stamp verifier on resume AND as
   finalizer step 0, F15 + extended F14), M2 (the single non-revoke filter
   `OwnerDDLApplyBundles()`/`isNonRevokeBundle` across every `owner-ddl apply` route incl.
   the FMA-007 self-heal + the embed/listing split, F16 + F12/`G-revoke-last`), BC-N1
   (the immutable `deploy_plan` transcript materialized before step 0, resume off the
   stored transcript, §1.3 + per-step doctor, F14), BC-N2 (the universal
   `revokeEmbedded`-independent `CheckDeployActivation` edge halting `awaiting_deploy` at
   `applied_owner == 20`, F11(e)/(f) + extended `G-old-binary-refuse`), C1 (the
   `finalizing` state + idempotent finalizer + §1.3 row + F10), C2 (`CheckDeployActivation`
   before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`,
   `RequiredOwnerBundleVersion` kept at 20), and C3 (the DDL-revoke bundle 0021
   special-cased + excluded from `owner-ddl apply` + applied terminal, F12/`G-revoke-last`)
   are carried forward **verbatim from the v6 HOLDER** and not regressed. Keep the §1.3
   table, the `finalizing` finalizer, the stored-transcript receipt key, the M1
   verifier, the M3 complete-cursor guard, the BC-N2 non-complete edge, and the new M5
   owner-watermark dimension split all coherent together.

1. **Keep Q3 and Q4 resolved.** Q3 — the per-step-atomic + resumable-cursor contract
   (now including M1's full-transcript verification, M3's complete-cursor activation
   guard, AND M5's owner-watermark dimension split that keeps the fresh-DB cells
   serving) is sufficient for every owner+runtime interleaving and boot-path combination
   P4 ships. Q4 — plain verb now with the three run-shape seams. Carry both forward; do
   not re-litigate.

2. **Keep the deployer surface and the serve-boot decoupling intact** (carry forward
   from v6): the `striatum daemon deploy` command site
   (`go/pkg/cli/localcommands/daemon.go`); the embed-FS-derived deploy plan with the
   immutable stored transcript (BC-N1) and the M1 full-transcript verification; the
   `deploy_plan`/`deploy_cursor` runtime migration (≥ 0044); the hash-chained deploy
   receipt into the owner-held `audit_log`; the lift of `ApplyMigrations` out of
   `go/pkg/db/connection.go` `ConnectAndMigrate` / `ConnectAndVerify` with the P2
   watermark interlock, the P3 drift gate, the BC-N2 universal non-complete cursor edge,
   the M3 complete-cursor guard, and now the M5-correct owner-watermark dimension (the
   `applied_owner == 0` fresh-DB cell still serves) intact.

3. **Keep the serving-role DDL revocation (the 0021 owner bundle)** — special-cased and
   sequenced terminal per C3, excluded from EVERY `owner-ddl apply` route including the
   FMA-007 self-heal (M2), and never reachable via the legacy serve-boot path on a
   `complete` cursor (M3). State exactly how it ships without lockout in any boot-path
   cell, with the embed/listing helper split.

4. **State each load-bearing claim as a falsifiable assertion + its named test /
   game-day step.** Carry F1–F18 + `G-revoke-last` + `G-old-binary-refuse` forward
   (re-confirm and re-anchor), and ensure F18 (`T-deploy-bootpath-decision-table`) is
   present and sharp with the SPLIT dimension: assert BOTH the `applied_owner == 0` serve
   cell (no-revoke, flag OFF, `cursorState=none` → serves fresh-bring-up, matching
   `CheckOwnerBundleWatermark`'s `applied == 0` exception) AND the
   `1 <= applied_owner < 20` `awaiting_owner_ddl` halt cell, plus the M3 cell and the
   shadow-mode fall-through.

5. **Stay inside the product boundary and the accepted design.** Local-first,
   single-host, ONE Postgres, ONE daemon as the single writer. Do NOT pull in P5
   (rehearsal receipt / expand-contract / fidelity tiering / clone = Q1/Q2).
   Shadow-first for the new path: a no-revoke inert binary on a clean DB still serves
   (the `applied_owner == 0` cell); a revoke-embedding binary with the flag OFF over a
   deploy transcript halts, never auto-applies (the M3 gate). Additive migrations only,
   self-record before enforce.

Do not treat falsifier completion as acceptance — the adjudicator's
collaboration ledger decides whether the gate clears.
