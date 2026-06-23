You are a **Falsifier** for the RFC 0142 P4 design run, and **this is the FOURTH
revision cycle (v4)**. Read the required context docs — `SEED.md` (charter + RFC
pointer + the two Open Questions Q3/Q4 + the **two binding revision constraints
BC-N1 + BC-N2** + the "Carried forward — resolved by v3 (do NOT reopen)" C1/C2/C3
section + the anchor table), the published **revised (v4)** `HOLDER.md` spec, the
**v3** `HOLDER.md`
(`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md`), and
the **v3** collaboration ledger
(`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
— the full BC-N1/BC-N2 analysis and its §4). Write a **material falsifying
challenge** in your `FALSIFIER.md` artifact — do not publish the ledger. RFC 0142
is accepted; do NOT re-litigate the five-layer design — attack the **P4
implementation shape** and the correctness core. Refute, don't rubber-stamp.

**FIRST, verify the two cycle-3 findings are GENUINELY resolved — not merely
claimed — and that C1/C2/C3 are NOT regressed.** Try to break each fix:

- **BC-N1 (immutable plan identity across resume — atomicity/resumability lens):**
  is the per-step `(plan_hash, step_index)` exactly-once receipt key now **stable
  across a crash-resume**? Does the spec materialize the IMMUTABLE ordered
  transcript (`base_owner_version`, `base_runtime_version`, target frontiers, every
  `{step_index, step_id, role, sha256}`, terminal-revoke placement) and persist it
  in `deploy_cursor` (or a `deploy_plan` table) **BEFORE step 0 mutates the
  frontier**? On resume, does it load the STORED transcript by `plan_hash` and
  resume at the next step **in that transcript** — rather than re-running the
  pending-delta `BuildPlan(current_owner, current_runtime)` over the moved live
  frontiers (`go/pkg/db/migrations.go` `applyOne` `~:138-172`/`:320`;
  `go/pkg/db/owner.go` `OwnerBundleVersion = MAX(version)` `~:225-245`,
  `ApplyOwnerBundles` `~:304-320`), which is exactly what renumbered
  `plan_hash`/`step_index` after step 0 in v3? Reproduce the v3 break: from owner
  19 / runtime 43 build `H = hash([runtime:0044, runtime:0045, revoke-bundle])`;
  kill after step 0 (`runtime:0044`) commits its `(H,0)` receipt; on re-run does
  the spec recompute `H` (not `H'` over the moved frontier) so `(H,0)` is
  recognized exactly-once, §1.3 classifies the cursor incomplete/resume (not
  unclassified foreign-incomplete), and the per-step doctor enumerates applied
  steps from the STORED transcript? Does `T-deploy-plan-hash-resume-after-step`
  kill after step 0 commits **and** after step 1 commits and assert the re-run
  reuses the same `plan_hash`, preserves the original `step_index`es, recognizes
  the prior receipts, completes the remaining steps, and leaves the doctor green?
  Hunt any residual crash window between a step's commit and its receipt-write, and
  any transcript-vs-binary mismatch the spec leaves unclassified.

- **BC-N2 (`deploy_cursor` authoritative before the terminal revoke —
  decoupling/migration-safety lens):** does a no-revoke-bundle deployer-aware
  binary at `applied_owner == 19` with an INCOMPLETE cursor now **REFUSE**
  (`awaiting_deploy`, DB-untouched) instead of legacy-applying? Reproduce the v3
  break (the holder's own two-binary choreography): the inert/no-revoke binary
  serves at owner 19; the activation binary runs `deploy`; kill after a runtime
  step commits `step_committed(k)` but before the revoke/finalizing (the C3-safe
  point, CREATE still held); restart the no-revoke binary. Does the spec give the
  serve path one HARD edge at `applied_owner == 19` — **every** deployer-aware
  binary (INCLUDING `!revokeEmbedded`) reads `deploy_cursor` before
  `ApplyMigrations` and before `RecordSchemaFingerprint` and halts `awaiting_deploy`
  on a non-`complete` cursor — OR a durable pre-revoke activation marker? Without
  it the no-revoke binary runs neither `CheckDeployActivation` (inert,
  `!revokeEmbedded`, `go/pkg/db/connection.go:341-353`) nor the forward-watermark
  rule (inert, `applied < 20`), reaches legacy `ApplyMigrations` + the shadow P3
  drift gate (`go/pkg/db/schema_drift.go:15-28,239-274`;
  `go/pkg/db/connection.go:376-402`) and SERVES + self-records over an incomplete
  deploy. Is `F11` extended to assert `ApplyMigrations` NOT called,
  `RecordSchemaFingerprint` NOT called, DB byte-identical, halt `awaiting_deploy`
  in that window? Is `G-old-binary-refuse` extended to prove the pre-revoke window
  cannot be served? CRUCIALLY: does BC-N2's fix regress C3 revoke-last (does the
  new pre-revoke edge block the deploy from completing or strand
  `ALTER … OWNER TO striatumd_rw` while CREATE is still held)?

- **C1 / C2 / C3 not regressed:** does the new stored-transcript receipt rule
  (BC-N1) stay coherent with the C1 `finalizing` finalizer and the §1.3
  classification table (no resume that serves; no double-appended terminal
  `complete` receipt)? Does the BC-N2 pre-revoke edge keep the C2
  `CheckDeployActivation` fail-closed edge, the typed halts, the forward-watermark
  rule, and `RequiredOwnerBundleVersion=19` intact? Is C3 (the DDL-revoke bundle —
  re-anchored to the renumbered `>= 0021` ordinal since 0020 is now
  `0020_owner_bundle_watermark_read.sql` / `LatestOwnerBundleVersion==20` —
  special-cased + terminal + excluded from `owner-ddl apply`, with the 0018/0019
  GRANT-CREATE-FIRST prerequisite, F12/`G-revoke-last`) carried forward intact and
  the activation deploy actually completable?

If BC-N1 or BC-N2 is not genuinely resolved, or C1/C2/C3 is regressed, that is a
standing falsification — say so explicitly and stop the revision from clearing.

**THEN, hunt for any NEW material gap** the revision introduced or left. Attack
the spec's load-bearing claims. The highest-value challenges:

1. **The Q3 atomicity claim is partly a lie.** Find a concrete owner+runtime
   interleaving the spec ships where a crash leaves a state the cursor/transcript
   cannot classify as "incomplete, resume" — including the per-step receipt
   boundary (BC-N1), a non-transactional DDL that auto-commits a partial change, a
   non-idempotent step, a stored-transcript-vs-binary divergence, or a
   two-connection crash window observable as "unknown drift, panic". A single such
   case where the contract is insufficient and no stricter sub-protocol is
   specified is a landed falsification.

2. **The BC-N2 pre-revoke edge breaks something else.** Show where the new hard
   edge (every deployer-aware binary halts on a non-`complete` cursor) blocks a
   legitimate boot — fresh-DB bring-up with no cursor, a `complete` cursor on a
   later binary, or the activation binary's own resume — or where the durable
   pre-revoke marker is left dangling after a clean deploy and wedges the next boot.

3. **Serve-boot decoupling regresses an existing gate.** Lifting `ApplyMigrations`
   out of `ConnectAndMigrate` must not break the P2 watermark interlock
   (`go/pkg/db/owner.go` `CheckOwnerBundleWatermark`), the P3 drift gate /
   `RecordSchemaFingerprint` (`go/pkg/db/schema_drift.go`), or fresh-DB bring-up; no
   window where the daemon serves on an unmigrated schema.

4. **DDL-revocation lockout.** Show where revoking serving-role DDL (the renumbered
   `>= 0021` bundle) recreates the #512-class lockout (the role that must run the
   deploy can't, across a restart), or breaks an existing boot/bootstrap path.

5. **Resumability / cursor / transcript / receipt defects.** Show a `deploy_cursor`
   state-machine hole, an out-of-order apply under the plan's dependency edges, a
   stored-transcript that disagrees with the embedded binary on resume, or a
   receipt written out of step with the real schema so `audit_log` provenance and
   the real schema disagree.

6. **Scope creep into P5 or boundary breach.** Show where the spec smuggles in P5
   (rehearsal/clone/expand-contract/fidelity tiering — Q1/Q2), breaches the
   local-first single-host/single-writer boundary, or is not shadow-first.

For each challenge record: the precise claim attacked, your concrete refutation
(with file:line / mechanism), the strongest rebuttal you can honestly construct on
the Holder's behalf, and whether a real gap remains. BC-N1 and BC-N2 are where to
spend most of your effort — an unresolved finding or an unproven
resumability/serve-gating claim is a standing falsification.
