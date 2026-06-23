You are the **Holder** for the RFC 0142 P4 design run, and **THIS IS THE FOURTH
REVISION (v4)**. Three prior design runs ran this same falsification gate. v1
(`rfc-0142-p4-design`) returned `needs_revision` with three findings C1/C2/C3. v2
(`rfc-0142-p4-design-v2`) **resolved C1 and C2** but returned `needs_revision`
again with C3 still open and a new finding N1. v3 (`rfc-0142-p4-design-v3`)
**resolved C3 (ownership transfer, revoke-last) and closed the immediate N1
receipt-outside-the-transaction hole** — both falsifiers conceded the C3
design-level answer — but returned `needs_revision` **again** on two independent
grounds, each squarely in the Q3 correctness core:

- **BC-N1** — the per-step `(plan_hash, step_index)` exactly-once receipt key is
  **NOT stable across a crash-resume**, because the plan is rebuilt from the LIVE
  frontiers that advance at each committed step;
- **BC-N2** — the C3 revoke-last ordering **opens a pre-terminal-revoke
  incomplete-deploy serve window** that a no-revoke-bundle deployer-aware binary
  does not gate.

**Start from the v3 `HOLDER.md`** — it is a **required context doc**
(`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md`). Your
job is to REVISE that spec, not write a new one from scratch. The full BC-N1/BC-N2
analysis and the exact prescribed fixes are in the **v3 collaboration ledger**
(also a required context doc:
`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
— read its `findings:` block for N1 and N2, and **§4 "What the revision must
fix"**, in full). `SEED.md` pins the two binding constraints (BC-N1 + BC-N2) and
the section "Carried forward — resolved by v3 (do NOT reopen)" (C1 + C2 + C3).

Your revised spec **MUST resolve BOTH cycle-3 findings (BC-N1 + BC-N2) per their
prescribed fix**, and **MUST carry forward C1, C2, and C3 unregressed**. A
revision that leaves BC-N1 or BC-N2 open — or that merely *claims* a fix without a
concrete sub-protocol — or that regresses C1, C2, or C3, has NOT cleared the gate.
This is the gate's single allowed revision cycle, so the cycle-4 falsifiers
re-attack each finding specifically and a second `needs_revision` ends the gate
unCleared.

Read the required context docs in full first — `SEED.md`, the v3 `HOLDER.md`, and
the v3 collaboration ledger — plus the committed RFC
(`docs/rfcs/0142-safe-by-construction-database-change-deployment.md`, status
`accepted`, D258). Build on the exact anchors the v3 spec and the SEED anchor
table use; **re-verify them against current `main`** — note in particular that
bundle 0020 is now taken by `0020_owner_bundle_watermark_read.sql` and
`LatestOwnerBundleVersion == 20` (`go/pkg/db/owner.go:23`), so the DDL-revoke
bundle the C3 mechanism special-cases must **renumber to `>= 0021`**; the C3
mechanism is unchanged, only the ordinal moves.

Publish the **revised (v4)** falsifiable implementation spec for RFC 0142 **P4 —
the one-shot deployer** as your `HOLDER.md` artifact. Make it concrete and
falsifiable, not a restatement of the RFC. Open with an auditable resolution map
(an "Addressing the design-v3 findings" subsection) so the falsifiers can verify
BC-N1 and BC-N2 are resolved and C1/C2/C3 are preserved, rather than infer it.

Hold the root reframe: **schema mutation must stop being an implicit side effect
of the serving process's restart and become an explicit, ordered, resumable,
provenance-tracked operation owned by a dedicated deployer** — so the serving
daemon can hold zero DDL privilege and a bad migration can never wedge the single
writer on boot.

Your spec MUST:

0. **Resolve both binding revision constraints — the gating requirement.**

   - **BC-N1 (make plan identity immutable across resume — Q3/R4 core).** The v3
     spec keyed the per-step receipt on `(plan_hash, step_index)`, but that key is
     **not stable across a crash-resume**: `BuildPlan(applied_owner,
     applied_runtime)` builds the *pending-delta* plan from the live frontiers,
     which advance at each committed step (`go/pkg/db/migrations.go` `applyOne`
     version stamp `~:138-172`/`:320`; `go/pkg/db/owner.go`
     `OwnerBundleVersion = MAX(version)` `~:225-245`, `ApplyOwnerBundles`
     `~:304-320`), and `deploy_cursor` persists only
     `{plan_hash, state, step_index, step_id}` — no transcript, no base/target
     frontiers. So from owner 19 / runtime 43 the first deploy builds
     `H = hash([runtime:0044, runtime:0045, revoke-bundle])`; after step 0
     (`runtime:0044`) commits its `(H,0)` receipt and the process dies, a re-run on
     frontier `(19,44)` rebuilds `H' != H` and renumbers `step_index`, so the
     `(H,0)` receipt is unrecoverable, §1.3 cannot classify the cursor, and the
     per-step doctor cannot require `(H,0)`. **Fix (binding):**
     - On deploy start, **materialize the IMMUTABLE ordered transcript** for the
       whole target deploy — `base_owner_version`, `base_runtime_version`, the
       target frontiers, every `{step_index, step_id, role, sha256}`, and the
       terminal-revoke placement — and persist it (in `deploy_cursor` or a new
       `deploy_plan` table) **BEFORE step 0 mutates the frontier**.
     - On resume, do **NOT** use the pending-delta `BuildPlan(current_owner,
       current_runtime)` as the source of truth: load the stored transcript by
       `plan_hash`, verify the embedded bytes still match the binary, and resume at
       the next step **in that transcript**, so `(plan_hash, step_index)` and the
       prior receipts remain stable.
     - Teach §1.3 the case where an incomplete cursor's `plan_hash` is not the
       binary's freshly-computed pending plan: it must be a **recoverable**
       "resume with the stored plan / explicit reconcile" state, never an
       unclassified drift bucket.
     - Make `doctor schema_deploy_unrecorded` enumerate applied steps from the
       **stored transcript** (plus cursor/frontier state), not the moving pending
       plan.
     - Add **`T-deploy-plan-hash-resume-after-step`**: a plan with ≥2 runtime steps
       + terminal revoke; kill after step 0 commits **and** after step 1 commits;
       the re-run must reuse the same `plan_hash`, preserve the original
       `step_index`es, recognize the prior receipts, complete the remaining steps,
       and leave the doctor green.

   - **BC-N2 (make `deploy_cursor` authoritative BEFORE the terminal revoke —
     decoupling/migration-safety core).** The C3 revoke-last ordering keeps
     `applied_owner == 19` until the terminal revoke commits, so a no-revoke-bundle
     deployer-aware binary runs **neither** `CheckDeployActivation` (inert because
     `!revokeEmbedded`) **nor** the forward-watermark rule (inert because
     `applied < 20`); it bypasses `deploy_cursor`, reaches legacy `ApplyMigrations`
     + the shadow-by-default P3 drift gate (`go/pkg/db/connection.go:341-353,376-402`;
     `go/pkg/db/schema_drift.go:15-28,239-274`), and **serves AND self-records**
     over an incomplete deploy. **Fix (binding):**
     - Give the serve path one **hard edge** that holds at `applied_owner == 19`:
       make **every** deployer-aware binary — **including the no-revoke-bundle
       landing binary** — read `deploy_cursor` before `ApplyMigrations` and before
       `RecordSchemaFingerprint`, and, if the cursor exists and is **not
       `complete`**, return `awaiting_deploy` **DB-untouched even when
       `!revokeEmbedded`**; OR introduce a **durable pre-revoke activation marker**
       set before the first deploy step that can leave an incomplete cursor, which
       halts no-revoke binaries at owner watermark 19 (not only after the revoke
       raises it).
     - Extend **F11** with the missing case: no-revoke deployer-aware binary,
       `applied_owner == 19`, `deploy_cursor.state in {in_progress, step_committed,
       finalizing}`, optional fingerprint mismatch, ±pending runtime migrations —
       assert `ApplyMigrations` is **NOT** called, `RecordSchemaFingerprint` is
       **NOT** called, the DB is byte-identical, and the halt is `awaiting_deploy`.
     - Extend **`G-old-binary-refuse`** to prove the pre-revoke
       incomplete-deploy window cannot be served, not merely that the revoke bundle
       is refused when the `>= 0044` marker is absent.
     - The new pre-revoke edge MUST NOT regress C3 revoke-last: it must not block
       the deploy from completing nor strand `ALTER … OWNER TO striatumd_rw` while
       CREATE is still held.

   Explicitly call out, in the revised spec, **how** BC-N1 and BC-N2 are now
   closed, and **confirm** C1 (the `finalizing` state + idempotent finalizer +
   §1.3 row + F10), C2 (`CheckDeployActivation` after `CheckOwnerBundleWatermark`
   and before `ApplyMigrations`, the typed halts, the forward-watermark rule,
   `RequiredOwnerBundleVersion` kept at 19, F11), and C3 (the DDL-revoke bundle
   special-cased out of the owner prefix and excluded from `owner-ddl apply`,
   applied as the terminal plan step after every runtime ownership reconcile,
   F12/`G-revoke-last`) are carried forward **verbatim from the v3 HOLDER** and not
   regressed. Keep C1, BC-N1, and BC-N2 coherent — the §1.3 classification table,
   the `finalizing` finalizer, the new stored-transcript receipt key, and the
   pre-revoke serve edge must all hold together.

1. **Keep Q3 and Q4 resolved.** Q3 — the per-step-atomic + resumable-cursor
   contract (now including the stored-transcript receipt key, BC-N1, and the
   pre-revoke serve edge, BC-N2) is sufficient for every owner+runtime
   interleaving P4 ships; the `deploy_cursor` states and crash-resume semantics are
   precise. Q4 — plain verb now with the three run-shape seams. Carry both forward;
   do not re-litigate.

2. **Keep the deployer surface and the serve-boot decoupling intact** (carry
   forward from v3): the `striatum daemon deploy` command site
   (`go/pkg/cli/localcommands/daemon.go`); the embed-FS-derived deploy plan (with
   the stored immutable transcript per BC-N1); the `deploy_cursor` runtime
   migration (≥ 0044); the hash-chained deploy receipt into the owner-held
   `audit_log`; the lift of `ApplyMigrations` out of `go/pkg/db/connection.go`
   `ConnectAndMigrate` / `ConnectAndVerify` with the P2 watermark interlock and P3
   drift gate intact.

3. **Keep the serving-role DDL revocation (the renumbered `>= 0021` owner
   bundle)** — special-cased and sequenced terminal per C3. State exactly how it
   ships without lockout and without the pre-revoke serve window (BC-N2).

4. **State each load-bearing claim as a falsifiable assertion + its named test /
   game-day step.** Carry F1–F12 + `G-revoke-last` forward (re-confirm and
   re-anchor to the renumbered revoke bundle), and ensure the BC-N1 and BC-N2 tests
   are present and sharp: `T-deploy-plan-hash-resume-after-step` (stable
   `plan_hash`/`step_index` and recognized prior receipts after a mid-plan crash)
   and the extended `F11` + `G-old-binary-refuse` (a no-revoke deployer-aware
   binary refuses `awaiting_deploy` on an incomplete cursor at `applied_owner == 19`).

5. **Stay inside the product boundary and the accepted design.** Local-first,
   single-host, ONE Postgres, ONE daemon as the single writer. Do NOT pull in P5
   (rehearsal receipt / expand-contract / fidelity tiering / clone = Q1/Q2).
   Shadow-first for the new path: default OFF behind a flag, additive migrations
   only, self-record before enforce.

Do not treat falsifier completion as acceptance — the adjudicator's
collaboration ledger decides whether the gate clears.
