---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0142 P4 one-shot deployer (decouple schema-apply from serve-boot) — falsifiable implementation spec (v3 REVISION; resolves cycle-2 C3 + N1, carries C1/C2 forward)"
participants:
  - "holder-author-001"
  - "falsifier-reviewer-001"
  - "falsifier-reviewer-003"
  - "adjudicator-author-001"
cycle: 1
entries:
  - kind: claim
    by: "holder-author-001"
    refs: ["dialogue:1"]
    text: "Revised P4 spec (v3): resolves the two binding cycle-2 findings and carries C1/C2 forward verbatim. C3 by resolution (a) — bundle 0020 (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`) is special-cased out of the owner prefix and the `owner-ddl apply` path (`LatestOwnerBundleVersion` stays 19; new `DDLRevokeOwnerBundleVersion = 20`) and applied as the TERMINAL deploy step after every runtime ownership reconcile, so each `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE and the steady state still denies it; the v2 F12 internal inconsistency dissolves (the two facts hold at different plan times) and F12 is revised to test it in a documented non-superuser two-role cluster. N1 by making the per-step receipt atomic-or-idempotent keyed on `(plan_hash, step_index)` — Q3-A appends the receipt in the same owner-connection step transaction (`step_committed(k)` durable iff its receipt is), Q3-B's `in_progress(k)` reconciler idempotently appends exactly one keyed receipt before `step_committed(k)`, and `doctor schema_deploy_unrecorded` is tightened to surface a missing per-step receipt; F13 new. C1 (the `finalizing` state + idempotent finalizer + §1.3 row + F10) and C2 (`CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark rule, Required=19, F11) carried forward unregressed and kept coherent with the new per-step rule."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "N1 is not genuinely resolved: the exactly-once receipt key `(plan_hash, step_index)` is not stable across a crash-resume. The v3 text closes the immediate receipt-outside-the-step-transaction hole, but it defines the plan as `BuildPlan(applied_owner, applied_runtime)` over the PENDING owner/runtime delta (`HOLDER.md:255-278`) and claims `step_index` stability from the resulting `plan_hash` (`HOLDER.md:286-295`) without persisting the transcript or base frontiers in `deploy_cursor` (which holds only `{plan_hash,state,step_index,step_id}`, `HOLDER.md:121-133`). Current source advances the live frontiers at each committed step (`migrations.go:138-172`, `owner.go:304-320`, `owner.go:225-245`). So from owner 19 / runtime 43 the first deploy builds `H = hash([runtime:0044, runtime:0045, owner:0020])`; after step 0 commits and the process dies, a re-run on frontier (19,44) rebuilds `H' = hash([runtime:0045, owner:0020]) != H` and renumbers `step_index`. The durable cursor still says `plan_hash = H, step_committed(0)`, so the §1.3 `in_progress`/`step_committed` row (which only covers `plan_hash == the binary's plan`) does not classify it `incomplete, resume`; the `(H,0)` receipt for already-applied `runtime:0044` cannot be recomputed; and the tightened doctor, reconstructing `BuildPlan` from the moved frontier, has no specified way to require `(H,0)`. The holder's stability claim is circular (`plan_hash` itself moves with the pending delta). The fix (materialize the immutable transcript / base+target frontiers before step 0, resume off the stored transcript, teach §1.3 and the doctor, add `T-deploy-plan-hash-resume-after-step`) is a real spec change not present as written."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-003"
    refs: ["dialogue:3"]
    text: "A new material decoupling/migration-safety gap that the C3 revoke-last ordering itself opens: a no-0020 binary can serve an incomplete pre-0020 deploy. `CheckDeployActivation` reads `deploy_cursor` only when `revokeEmbedded` (the binary ships the 0020 file, `HOLDER.md:328-337`); the forward-watermark rule fires only when a no-0020 binary observes `applied_owner >= 20` (`HOLDER.md:554-561`). But by the revoke-last design `applied_owner` stays 19 until the TERMINAL 0020 commits (`HOLDER.md:353-357,585-597`). Reproducer using the holder's own two-binary choreography: the inert-landing binary (no 0020) serves at owner 19; the activation binary runs `deploy`; kill after a runtime step commits `step_committed(k)` but before 0020/finalizing (the C3-safe point, CREATE still held); restart the inert/no-0020 binary. `CheckDeployActivation` is inert (`!revokeEmbedded`), the forward-watermark rule is inert (`applied_owner == 19`), so the binary never reads `deploy_cursor`, reaches legacy `ApplyMigrations` + the P3 path (`connection.go:341-353,376-402`); the P3 drift gate is shadow by default (`CheckSchemaDrift` drifted-but-nil when `STRIATUM_SCHEMA_DRIFT_REFUSE` unset, `schema_drift.go:15-28,239-274`), so the boot serves AND self-records a fingerprint over an incomplete deploy. This violates the load-bearing `incomplete cursor never serves` claim across the rollout pair, lets a should-be-verify-only boot mutate-and-self-record (masking the partial deploy before the P4 finalizer runs), and is missed by F11 (whose old/no-0020 matrix only tests `applied_owner == 20`). A #512-class shape in clean-halt clothing; the only rebuttal (operator parks the daemon) is choreography discipline, insufficient for a deployer the spec sells as crash-resume / rollback-resistant and whose own F11 / `G-old-binary-refuse` scope no-0020 binaries. Fix: make `deploy_cursor` authoritative before terminal 0020 (every deployer-aware binary, incl. no-0020, halts `awaiting_deploy` DB-untouched on a non-`complete` cursor) or a durable pre-0020 activation marker; extend F11 + `G-old-binary-refuse`."
    correspondence: landed_unrebutted
verdict: "needs_revision"
rationale: "REVISION cycle v3 (the gate's single allowed revision cycle). C3-as-ownership-transfer IS genuinely resolved — the SEED-recommended resolution (a) (sequence bundle 0020 last, exclude it from the owner prefix and `owner-ddl apply`, run every ownership reconcile while CREATE is held, revised F12 in a documented two-role cluster) dissolves the v2 self-contradiction, and BOTH falsifiers explicitly concede it as a coherent design-level answer. C1 (the `finalizing` state + idempotent finalizer + §1.3 row + F10) and the C2 fail-closed activation EDGE (`CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark rule, Required=19, F11) are carried forward INTACT and unregressed; the v2 caveat (C3 stranding the `awaiting_deploy` remediation) is closed by the C3 fix. BUT the gate does NOT clear, on two independent grounds, each squarely in the Q3 correctness core: (1) N1 is NOT genuinely resolved — falsifier_1 shows the prescribed idempotent reconcile keyed on `(plan_hash, step_index)` rests on a key that is NOT stable across resume, because `BuildPlan(applied_owner, applied_runtime)` rebuilds the plan from the live frontiers that advance at every committed step and `deploy_cursor` persists no immutable transcript; after the first committed step a re-run reconstructs a different `plan_hash` and renumbers `step_index`, breaking the exactly-once receipt key, the §1.3 resume classification, and the tightened per-step doctor. (2) A NEW material challenge stands — falsifier_2 shows the C3 revoke-last ordering opens a pre-0020 window (`applied_owner == 19`, `deploy_cursor` incomplete) in which a no-0020 deployer-aware binary runs neither `CheckDeployActivation` (inert, `!revokeEmbedded`) nor the forward-watermark rule (inert, `applied < 20`), bypasses `deploy_cursor`, reaches legacy `ApplyMigrations` + the P3 shadow self-record, and serves/masks an incomplete deploy — defeating the load-bearing `incomplete cursor never serves` guarantee and the P3/P4 self-record split across the actual rollout pair. Both challenges are material, source-grounded, and stand unrebutted by the spec as written (this cycle carries no holder rebuttal artifact). Per the role rubric a per-step interleaving where the per-step-atomic + resumable-cursor contract (now including the per-step receipt) is insufficient with no concrete sub-protocol ALONE forces needs_revision; here two such interleavings stand. Because this is the single allowed revision cycle, the gate ends unCleared; §4 records exactly what a re-scaffolded revision must fix."
findings:
  - id: C1
    severity: critical
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed. The distinct `finalizing` cursor state, the idempotent finalizer (append the `complete` receipt guarded on `(plan_hash, state=complete)` → UPSERT the `schema_state` fingerprint → advance `finalizing → complete` LAST), the §1.3 resumable-finalization row (never serve, never genuine-drift), the operator pin (no single finalization transaction spans the owner+runtime connections), and F10 (`T-deploy-resume-finalization-crash`) are preserved exactly from the v2 HOLDER (v3 §1.2 `HOLDER.md:119-168`, §1.3 `HOLDER.md:170-191`, §3.4 `HOLDER.md:432-489`). Both v3 falsifiers confirm no direct regression of the finalizer. The finalizer mechanism is structurally intact; the §1.3 table's ENFORCEMENT is undercut by N1 (the cursor `plan_hash` may not match the binary's recomputed pending plan) and N2 (a no-0020 binary bypasses the table entirely) — recorded under those findings, not as a C1 regression."
    affected_invariants: ["Q3 per-step-atomic + resumable-cursor contract", "section 1.3 incomplete-vs-drift disambiguation table"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: C2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT at the cleared fail-closed EDGE — carried forward verbatim. `CheckDeployActivation` called immediately after `CheckOwnerBundleWatermark` and BEFORE `ApplyMigrations` in both `ConnectAndMigrate` and `ConnectAndVerify` (mutating nothing), absent-cursor-as-incomplete, the typed `awaiting_deploy`/`awaiting_deploy_config` halts on the non-restartable exit, the forward-watermark rule, `RequiredOwnerBundleVersion` kept at 19, and F11 are preserved (v3 §3.3a `HOLDER.md:320-357`, §4.2 `HOLDER.md:539-571`, §4.3 `HOLDER.md:573-612`). The v2 caveat — the `awaiting_deploy` remediation was unsatisfiable because C3 stranded the deploy — is genuinely closed by the C3 revoke-last fix (the deploy can now complete). The cleared EDGE is not regressed. HOWEVER a NEW material challenge stands (N2, falsifier_2): the C3 revoke-last ordering opens a pre-0020 window (`applied_owner == 19`, `deploy_cursor` incomplete) where a no-0020 deployer-aware binary runs neither `CheckDeployActivation` (inert because `!revokeEmbedded`) nor the forward-watermark rule (inert because `applied < 20`); it bypasses `deploy_cursor`, reaches legacy `ApplyMigrations` + the P3 shadow self-record, and can serve an incomplete deploy. So the global claim `incomplete cursor never serves` is not enforced across the rollout pair even though the v2 edge is intact. Disposition recorded under N2."
    affected_invariants: ["R3 no DDL-revocation lockout", "serve-boot clean-halt (no crash-loop)", "incomplete cursor never serves"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: C3
    severity: high
    posture: design
    status: answered
    challenge: "RESOLVED — the ownership-transfer mechanism. The v2 self-contradiction (`ALTER … OWNER TO striatumd_rw` requires `CREATE ON SCHEMA striatumd`, which bundle 0020 revokes, applied before the reconcile) is resolved by the SEED-recommended resolution (a): bundle 0020 is special-cased out of the owner prefix and EXCLUDED from the `owner-ddl apply` watermark path (`LatestOwnerBundleVersion` stays 19; new `DDLRevokeOwnerBundleVersion = 20`; `ApplyOwnerBundles` bounded to `<= 19` so it skips 0020), and appended as the TERMINAL plan step after every runtime ownership reconcile (v3 §3.2 `HOLDER.md:248-295`, §3.3b `HOLDER.md:359-430`, §4.4 `HOLDER.md:614-628`). Every reconcile runs while `striatumd_rw` still holds CREATE (granted by 0018 in the non-revoke prefix); 0020 commits last, so the committed steady state still denies CREATE. The v2 F12 internal inconsistency is dissolved (the two facts hold at different plan times) and F12 is revised to assert, in a documented non-superuser two-role cluster: `has_schema_privilege('striatumd_rw','striatumd','CREATE')` true at the reconcile with the reconcile `step_index` strictly less than 0020's; catalog owner == `striatumd_rw` for every created object (table, index, sequence); the serving role's real `SELECT/INSERT/UPDATE/DELETE`; the post-deploy `SET ROLE striatumd_rw; CREATE TABLE → 42501`; `has_schema_privilege ... CREATE` false post-deploy; idempotent re-run; plus `G-revoke-last`. BOTH falsifiers explicitly concede this is a coherent design-level answer to the v2 C3 contradiction. CAVEAT: the revoke-last ordering this adopts is precisely what SPAWNS the new material challenge N2 (the pre-0020 incomplete-deploy serve window). C3-as-ownership-transfer is resolved; its ordering consequence is recorded under N2 and forces needs_revision there."
    affected_invariants: ["two-role owner/runtime boundary", "section 4.1 residual-capability argument", "R3 activation deploy can complete"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: N1
    severity: high
    posture: design
    status: open
    challenge: "OPEN — NOT genuinely resolved (falsifier_1; Q3/R4 correctness core). The v3 spec closes the IMMEDIATE v2 hole (the receipt is now in the owner-connection step transaction for Q3-A, an idempotent `(plan_hash, step_index)` reconcile covers Q3-B, the doctor is tightened per-step — falsifier_1 concedes the receipt-outside-the-transaction hole is closed). But the prescribed N1 fix requires an idempotent per-step receipt reconcile KEYED ON `(plan_hash, step_index)`, and that key is NOT stable across a crash-resume as written. `BuildPlan(applied_owner, applied_runtime)` (v3 §3.2 `HOLDER.md:255-278`) builds the PENDING-delta plan from the current live frontiers; current source advances those frontiers at each committed step (`migrations.go:138-172`; `owner.go:304-320`; `OwnerBundleVersion = MAX(version)` `owner.go:225-245`). From owner 19 / runtime 43 the first deploy builds `H = hash([runtime:0044, runtime:0045, owner:0020])`; after step 0 (`runtime:0044`) commits its keyed receipt `(H,0)` and the process dies, a re-run on frontier (19,44) rebuilds `H' = hash([runtime:0045, owner:0020]) != H` and renumbers `step_index`. The cursor persists only `{plan_hash,state,step_index,step_id}` (v3 §1.2 `HOLDER.md:121-133`) — neither the ordered transcript nor the base/target frontiers — so the original `H` is unrecoverable. Consequences: (1) the `(H,0)` receipt for already-applied `runtime:0044` cannot be recomputed (the exactly-once N1 key is unstable); (2) the §1.3 `in_progress`/`step_committed` row only matches `plan_hash == the binary's plan`, but the cursor holds `H` while the binary recomputes `H'`, so the interrupted deploy is neither `incomplete, resume` nor genuine-drift — it is unclassified foreign-incomplete; (3) the tightened `doctor schema_deploy_unrecorded`, reconstructing `BuildPlan` from the moved frontier, has no specified way to require `(H,0)`. The holder's stability claim (`step_index` stable because content-addressed by `plan_hash`) is circular: `plan_hash` moves with the pending delta. The strongest rebuttal (BuildPlan meant to produce the full immutable target transcript, cursor pinning plan identity) would work but is NOT the v3 text. STANDS unrebutted → needs_revision."
    affected_invariants: ["RFC 0142 Layer 3 deploy-receipt provenance (one chained receipt per schema change)", "R4 receipt-written-out-of-step / no double-apply", "Q3 resume at the next clean boundary across a moved frontier", "section 1.3 incomplete-vs-drift disambiguation"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: N2
    severity: high
    posture: design
    status: open
    challenge: "OPEN — NEW material challenge (falsifier_2; decoupling / migration-safety lens). The C3 revoke-last ordering creates a pre-0020 incomplete-deploy window the serve path does not gate. `CheckDeployActivation` consults `deploy_cursor` only when `revokeEmbedded` (the binary ships the 0020 file, v3 §3.3a `HOLDER.md:328-337`); a no-0020 deployer-aware binary makes it inert. The forward-watermark rule fires only when a no-0020 binary observes `applied_owner >= 20` (v3 §4.2 `HOLDER.md:554-561`), but by the revoke-last design `applied_owner` stays 19 until the TERMINAL 0020 commits (`HOLDER.md:353-357,585-597`). Reproducer (the holder's own two-binary choreography): the inert-landing binary (no 0020) serves at owner 19; the activation binary runs `deploy`; kill after a runtime step commits `step_committed(k)` but before 0020/finalizing (the C3-safe point, CREATE still held); restart the inert/no-0020 binary. `CheckDeployActivation` is inert (`!revokeEmbedded`), the forward-watermark rule is inert (`applied_owner == 19`), so the binary never reads `deploy_cursor`, reaches legacy `ApplyMigrations` + the P3 path (`connection.go:341-353,376-402`); the P3 drift gate is shadow by default (`CheckSchemaDrift` drifted-but-nil when `STRIATUM_SCHEMA_DRIFT_REFUSE` unset, `schema_drift.go:15-28,239-274`), so the boot serves AND self-records a fingerprint over an incomplete deploy. This (a) violates the load-bearing `incomplete cursor never serves` claim across the rollout pair, (b) lets a should-be-verify-only boot mutate-and-self-record, masking the partial deploy before the P4 finalizer writes the terminal receipt/fingerprint — regressing the P3/P4 self-record split the holder calls load-bearing (v3 §3.3 `HOLDER.md:308-315`), and (c) is missed by F11, whose old/no-0020 matrix only tests `applied_owner == 20`. The only rebuttal (operator parks the daemon; no DDL revoke has happened yet) is choreography discipline — insufficient for a deployer the spec sells as crash-resume / rollback-resistant and whose own F11 / `G-old-binary-refuse` scope no-0020 binaries. A #512-class shape in clean-halt clothing. STANDS unrebutted → needs_revision."
    affected_invariants: ["incomplete cursor never serves (section 1.3 second signal authoritative)", "P3/P4 self-record split (verify-only boot must not mask deploy state)", "R2 decoupling does not regress P3 / fresh-DB bring-up", "R3 no DDL-revocation lockout / no #512-class shape"]
    source_refs: ["dialogue:1", "dialogue:3"]
branches:
  design: blocked
---

# COLLABORATION LEDGER — RFC 0142 P4 design run (v3 / REVISION), cycle 1

author: adjudicator-author-001

**Verdict:** `needs_revision`

> Scope of this adjudication: the curated dialogue trajectory only — the **revised
> (v3)** Holder's `HOLDER.md` P4 implementation spec and the two falsifiers'
> `FALSIFIER.md` challenges — plus the `SEED.md` charter, with the **v2** `HOLDER.md`
> and the **v2** collaboration ledger as context for what the revision had to fix.
> RFC 0142 is accepted (D258); this judges the **P4 implementation shape**, not the
> five-layer design. A clearing verdict would be `accept` or `accept_with_findings`;
> this is neither. This is the gate's **single allowed revision cycle**, so the gate
> ends **unCleared** — judged exactly, not graded on the consequence.

## 1. Decision

`needs_revision`. The revision is real, well-anchored work: it **genuinely resolves the
C3 ownership-transfer contradiction** (resolution (a): sequence bundle 0020 last, exclude
it from the owner prefix and `owner-ddl apply`, run every reconcile while `striatumd_rw`
still holds CREATE; F12 revised for a documented two-role cluster — **both falsifiers
concede this design-level answer**), it **carries C1 and the C2 fail-closed edge forward
intact**, and it closes the immediate v2 N1 hole (the receipt now joins the owner-connection
step transaction). But a clearing verdict requires **C3 resolved AND N1 resolved AND C1
intact AND C2 intact AND no new material challenge standing**, and two things break that —
each independently, and each squarely in the Q3 correctness core:

- **N1 is not genuinely resolved (falsifier_1).** The prescribed fix is an idempotent
  per-step receipt reconcile **keyed on `(plan_hash, step_index)`**. That key is **not
  stable across a crash-resume** as written: `BuildPlan(applied_owner, applied_runtime)`
  builds the *pending-delta* plan from the live frontiers, which advance at every committed
  step, and `deploy_cursor` persists no immutable transcript or base frontiers. After the
  first committed step a re-run reconstructs a **different `plan_hash`** and renumbers
  `step_index`, breaking the exactly-once receipt key, the §1.3 resume classification, and
  the tightened per-step doctor at once.
- **A new material challenge stands (falsifier_2).** The **C3 revoke-last ordering itself**
  opens a pre-0020 window (`applied_owner == 19`, `deploy_cursor` incomplete) where a
  no-0020 deployer-aware binary runs **neither** `CheckDeployActivation` (inert because
  `!revokeEmbedded`) **nor** the forward-watermark rule (inert because `applied < 20`). It
  bypasses `deploy_cursor`, reaches legacy `ApplyMigrations` + the P3 shadow self-record,
  and can **serve and mask an incomplete deploy** — defeating the load-bearing "incomplete
  cursor never serves" guarantee and the P3/P4 self-record split across the actual rollout
  pair.

Both challenges are material, source-grounded, and stand **unrebutted** by the spec as
written (this cycle carries no Holder rebuttal artifact; the spec text stands). Per the
role rubric, a per-step interleaving where the per-step-atomic + resumable-cursor contract
(now including the per-step receipt) is insufficient with no concrete sub-protocol **alone
forces `needs_revision`**; here two such interleavings stand. The revision must fix exactly
the two items in §4.

## 2. Per-finding / per-challenge ledger

| # | Source | What is judged | Material? | Resolved / rebutted? | Disposition |
| --- | --- | --- | --- | --- | --- |
| **C3** | v2 finding / both falsifiers' revision-check | revoke-last (a): 0020 sorted terminal + excluded from `owner-ddl apply`; reconcile runs while CREATE held; F12 revised for the two-role cluster (`HOLDER.md:248-295,359-430,614-628,652`) | — | **Yes — genuinely resolved** as an ownership-transfer mechanism; both falsifiers concede the design-level answer; F12 inconsistency dissolved | **RESOLVED** (spawns N2 via the ordering) |
| **N1** | v2 finding / falsifier_1 (`dialogue:2`) | per-step receipt keyed on `(plan_hash, step_index)`; immediate hole closed, but the key is unstable under pending-delta `BuildPlan` (`HOLDER.md:255-295,121-133,440-463`) | **Yes** — Q3/R4 core; exactly-once key, §1.3 resume row, and per-step doctor all break after step 0 | **No** — no immutable plan identity persisted; stability claim is circular | **OPEN → needs_revision** |
| **C1** | v2 finding / both falsifiers' revision-check | `finalizing` state + idempotent finalizer + §1.3 row + F10 carried forward (`HOLDER.md:119-191,432-489`) | — | **Yes — INTACT, unregressed**; finalizer structurally preserved | **INTACT** |
| **C2** | v2 finding / falsifier_2 (`dialogue:3`) | `CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark, Required=19, F11 carried forward (`HOLDER.md:320-357,539-612`) | — | **Edge INTACT, unregressed**; but global "incomplete cursor never serves" not enforced across the rollout pair | **INTACT (edge); undercut by N2** |
| **N2** | falsifier_2 (`dialogue:3`) | the revoke-last pre-0020 window: no-0020 binary, `applied_owner==19`, incomplete cursor → bypasses `CheckDeployActivation` + forward-watermark, serves + self-records (`HOLDER.md:328-337,554-561,308-315`) | **Yes** — serves a half-applied deploy; P3 self-record masks it; #512-class shape | **No** — no hard edge; only choreography-discipline rebuttal | **STANDS → needs_revision** |

## 3. Why the verdict turns the way it does

### C3 — ownership transfer: RESOLVED

The v2 defect was a self-contradiction: Policy 1's `ALTER <kind> striatumd.<name> OWNER TO
striatumd_rw` reconcile needs the new owner to hold `CREATE ON SCHEMA striatumd` (the repo's
own bundles document this and GRANT CREATE first — `0018:64-75,98`; `0019:53-80`), yet this
phase's bundle 0020 REVOKEs precisely that and the v2 plan applied 0020 *before* the runtime
reconcile, stranding the deploy. The v3 revision adopts the SEED-recommended **resolution
(a)** (`HOLDER.md:248-295,359-430`): bundle 0020 is identified by `DDLRevokeOwnerBundleVersion
= 20`, special-cased **out of the owner prefix** and **excluded from the `owner-ddl apply`
watermark path** (`LatestOwnerBundleVersion` stays 19; `ApplyOwnerBundles` is bounded to
`<= 19` and skips 0020; a build guard asserts no bundle `>= 20` is reachable through
`owner-ddl apply`), and **appended as the terminal plan step** with a dependency edge on every
prior step. So every runtime ownership reconcile runs while `striatumd_rw` still holds CREATE
(granted by 0018, in the non-revoke prefix), and 0020 commits last, so the committed steady
state still denies CREATE. The v2 F12 internal inconsistency dissolves because the two facts
hold at **different plan times**, and F12 is revised to assert, in a **documented non-superuser
two-role cluster**, the reconcile-time `has_schema_privilege(... CREATE)` truth with
`step_index` strictly below 0020's, the catalog owner of every created object, the serving
role's real DML, the post-deploy `42501` denial, the post-deploy `has_schema_privilege` false,
idempotent re-run, plus `G-revoke-last`. **Both falsifiers explicitly concede** this is a
coherent design-level answer to the v2 C3 contradiction. **C3-as-ownership-transfer is
genuinely resolved.** (The ordering it adopts is what opens N2 — recorded there.)

### N1 — per-step receipt crash-safety: OPEN (the key is not stable across resume)

The v2 N1 gap was that the per-step receipt was not made atomic/idempotent with the step. The
v3 spec closes the *immediate* hole — the receipt now appends in the same owner-connection
transaction as a transactional step (`step_committed(k)` durable iff its receipt is), Q3-B's
`in_progress(k)` reconciler appends an idempotent receipt keyed on `(plan_hash, step_index)`
before `step_committed(k)`, and the doctor is tightened per-step (`HOLDER.md:432-501`).
falsifier_1 **concedes** that hole is closed.

But the prescribed fix is an idempotent reconcile **keyed on `(plan_hash, step_index)`**, and
falsifier_1 shows — against current source — that this key is **not stable across the very
crash-resume N1 exists to make safe**. The plan is `BuildPlan(applied_owner, applied_runtime)`
over the **pending** owner/runtime delta (`HOLDER.md:255-278`), `plan_hash` is the hash of that
*pending* ordered transcript (`HOLDER.md:286-295`), and current source advances the live
frontiers at each committed step (`migrations.go:138-172`; `owner.go:304-320`; `OwnerBundleVersion
= MAX(version)`, `owner.go:225-245`). Concretely from owner 19 / runtime 43: the first deploy
builds `H = hash([runtime:0044, runtime:0045, owner:0020])` with `step_index 0 = runtime:0044`;
after step 0 commits its keyed receipt `(H,0)` and the process dies, a re-run on the moved
frontier `(19,44)` rebuilds `H' = hash([runtime:0045, owner:0020]) != H` and renumbers
`step_index`. The cursor persists only `{plan_hash, state, step_index, step_id}`
(`HOLDER.md:121-133`) — **not** the ordered transcript, base frontiers, or target frontiers —
so the original `H` is unrecoverable. Three load-bearing things break together:

1. **The exactly-once receipt key is lost.** `(H,0)` for the already-applied `runtime:0044`
   cannot be recomputed once the deployer rebuilds `H'`; the N1 idempotency the fix promised
   has no stable identity to key on.
2. **The §1.3 resume classification fails.** The `in_progress`/`step_committed` row only covers
   `plan_hash == the binary's plan` (`HOLDER.md:179-181`); the cursor holds `H` while the binary
   recomputes `H'`, so the interrupted deploy is **neither** "incomplete, resume" **nor**
   genuine-drift — it is an unclassified foreign-incomplete state, exactly the bucket §1.3 was
   meant to eliminate.
3. **The tightened doctor cannot enumerate applied steps.** `doctor schema_deploy_unrecorded`
   reconstructs `BuildPlan` from the current frontier (`HOLDER.md:491-499`); after `runtime:0044`
   leaves the pending set the doctor has no specified way to require its `(H,0)` receipt, so the
   per-step guarantee ("green only when all committed steps have receipts") is not enforceable.

The holder's own justification — "`step_index` ... is stable across re-runs because the plan is
content-addressed by `plan_hash`" (`HOLDER.md:294-295,442`) — is **circular**: `plan_hash` is
itself computed from the pending delta, which moves. The strongest rebuttal on the Holder's
behalf (that `BuildPlan` was meant to materialize the **full immutable target transcript** and
that `deploy_cursor.plan_hash` pins an immutable deployment identity) **would** make the design
workable — but falsifier_1 is right that it **is not what the v3 text specifies**, which names
`BuildPlan(applied_owner, applied_runtime)` and "pending" steps and persists no transcript /
base frontiers anywhere. This is the Q3 correctness core (resume at the next clean boundary
across a moved frontier), and per the role rubric an interleaving where the per-step-atomic +
resumable-cursor contract is insufficient with no concrete sub-protocol **alone forces
needs_revision**. The repair is bounded but is a genuine spec change, absent as written.
**N1 remains open.**

### C1 — finalization boundary: INTACT

The `finalizing` cursor state, the idempotent finalizer (`complete` receipt guarded on
`(plan_hash, state=complete)` → fingerprint UPSERT → `finalizing → complete` last), the §1.3
resumable-finalization row, the operator pin (no single finalization transaction spans the
owner+runtime connections), and F10 are preserved verbatim from the v2 HOLDER
(`HOLDER.md:119-191,432-489`); the finalizer crash-point table is carried forward unchanged.
Both falsifiers confirm no direct regression. The finalizer mechanism is **structurally
intact**. The honest note (recorded under N1 and N2, not as a C1 regression): the §1.3 table's
*enforcement* is undercut — by N1 because the cursor `plan_hash` may not equal the binary's
recomputed pending plan, and by N2 because a no-0020 binary never consults the table at all. The
table is right; what fails is that the deployer/boot can reach it with the wrong key (N1) or skip
it (N2). **C1 is intact; preserve it verbatim.**

### C2 — fail-closed activation edge: INTACT (edge), undercut operationally by N2

The cleared v2 EDGE is preserved verbatim (`HOLDER.md:320-357,539-612`): `CheckDeployActivation`
runs immediately after `CheckOwnerBundleWatermark` and **before** `ApplyMigrations` in both boot
paths (mutating nothing); a revoke-embedding binary halts `awaiting_deploy_config` (flag OFF) or
`awaiting_deploy` (flag ON + deploy incomplete, including `deploy_cursor` absent); the
forward-watermark rule halts a revoke-unaware binary observing `applied >= 20`;
`RequiredOwnerBundleVersion` stays 19; the typed halts get `main.go` `errors.As` arms; F11 spies
that `applyOne` is never entered across the bad interleavings. The v2 caveat (the `awaiting_deploy`
remediation was unsatisfiable because C3 stranded the deploy) is **genuinely closed** by the C3
fix — the deploy can now complete. **The edge is not regressed.**

But the edge is narrower than the spec's global claim. falsifier_2 shows the **C3 revoke-last
ordering** opens a window the edge does not cover (see N2): for a no-0020 deployer-aware binary at
`applied_owner == 19` with an incomplete cursor, neither `CheckDeployActivation` (inert,
`!revokeEmbedded`) nor the forward-watermark rule (inert, `applied < 20`) fires, so the binary can
serve a half-applied deploy. So "incomplete cursor never serves" is **not enforced across the
rollout pair**, even though the v2 edge is intact. Recorded under N2.

### N2 — pre-0020 incomplete-deploy serve window (new; decoupling/migration-safety core): STANDS

falsifier_2 concedes C3-design and the N1 receipt-write fix, then lands a distinct gap **created
by the v3 C3 fix itself**. `CheckDeployActivation` reads `deploy_cursor` only when `revokeEmbedded`
(the binary ships the 0020 file, `HOLDER.md:328-337`); the forward-watermark rule fires only at
`applied_owner >= 20` (`HOLDER.md:554-561`). But revoke-last keeps `applied_owner` at 19 until the
**terminal** 0020 commits (`HOLDER.md:353-357,585-597`). So the window the C3 fix introduces — all
runtime steps run or in progress, 0020 not yet committed — is **invisible to the no-0020 serve
path**. Using the holder's own two-binary choreography: the inert-landing binary (no 0020) serves
at owner 19; the activation binary runs `deploy`; kill after a runtime step commits
`step_committed(k)` but before 0020/finalizing (exactly the C3-safe point, CREATE still held);
restart the inert/no-0020 binary — which the holder makes the **first required rollout binary**,
serving before activation (`HOLDER.md:577-584`). On that boot `CheckDeployActivation` is inert
(`!revokeEmbedded`), the forward-watermark rule is inert (`applied_owner == 19`), so the binary
**never reads `deploy_cursor`**, reaches legacy `ApplyMigrations` + the P3 path
(`connection.go:341-353,376-402`); the P3 drift gate is **shadow by default** (`CheckSchemaDrift`
returns drifted-but-nil when `STRIATUM_SCHEMA_DRIFT_REFUSE` is unset,
`schema_drift.go:15-28,239-274`), so the boot **serves AND self-records** a fingerprint over an
incomplete deploy.

This is material on three counts: (a) it breaks the load-bearing "incomplete cursor never serves"
claim (`HOLDER.md:177-182,477-479`) across the rollout pair — the §1.3 second signal only gates
binaries that embed 0020; (b) a should-be-verify-only boot runs the **mutate-and-self-record** path,
masking the partial deploy before the P4 finalizer writes the terminal receipt/fingerprint —
regressing the P3/P4 self-record split the holder calls load-bearing (`HOLDER.md:308-315`); (c) F11's
old/no-0020 matrix misses it (it only tests `applied_owner == 20`, but revoke-last puts the dangerous
window at `applied_owner == 19`). The Holder's only rebuttal — the operator parks the daemon and
restarts only the activation binary, and pre-0020 no DDL revoke has occurred so the v2 C3 transfer
failure is absent — is **choreography discipline**, which falsifier_2 correctly argues is insufficient
for a deployer the spec sells as crash-resume / rollback-resistant, and whose own F11 and
`G-old-binary-refuse` (`HOLDER.md:607-612,651`) bring old/no-0020 binaries into scope. This is a
**#512-class shape in clean-halt clothing**: a partially-applied activation that the previous serve
path can come back up over without honoring the cursor, with the safety proof resting on operator
discipline rather than a hard state-machine edge. **N2 stands.**

## 4. What the revision must fix (exact)

A re-scaffolded revision clears the gate only if it pins **both** items below, while keeping C3,
C1, and the C2 edge intact and not regressing the carry-forward items.

1. **Make plan identity immutable across resume (N1 — Q3/R4 core).**
   - On deploy start, **materialize the immutable ordered transcript** for the whole target deploy
     — `base_owner_version`, `base_runtime_version`, the target frontiers, every
     `{step_index, step_id, role, sha256}`, and the terminal-0020 placement — and persist it (in
     `deploy_cursor` or a `deploy_plan` table) **before step 0 mutates the frontier**.
   - On resume, do **not** use the pending-delta `BuildPlan(current_owner, current_runtime)` as the
     source of truth: load the stored transcript by `plan_hash`, verify the embedded bytes still
     match the binary, and resume at the next step **in that transcript**, so `(plan_hash, step_index)`
     and the prior receipts remain stable.
   - Teach §1.3 the case where an incomplete cursor's `plan_hash` is not the binary's freshly-computed
     pending plan: it must be a **recoverable** "resume with the stored plan / explicit reconcile"
     state, never an unclassified drift bucket.
   - Make `doctor schema_deploy_unrecorded` enumerate applied steps from the **stored transcript**
     (plus cursor/frontier state), not the moving pending plan.
   - Add `T-deploy-plan-hash-resume-after-step`: a plan with ≥2 runtime steps + terminal 0020; kill
     after step 0 commits and after step 1 commits; the re-run must reuse the same `plan_hash`,
     preserve the original `step_index`es, recognize the prior receipts, complete the remaining steps,
     and leave the doctor green.

2. **Make `deploy_cursor` authoritative BEFORE terminal 0020 (N2 — decoupling/migration-safety core).**
   - Give the serve path one **hard edge** that holds at `applied_owner == 19`: make **every**
     deployer-aware binary — **including the inert / no-0020 landing binary** — read `deploy_cursor`
     before `ApplyMigrations` and before `RecordSchemaFingerprint`, and, if the cursor exists and is
     **not `complete`** for a plan it can prove safe, return `awaiting_deploy` **DB-untouched even when
     `!revokeEmbedded`**; OR introduce a **durable pre-0020 activation marker** set before the first
     deploy step that can leave an incomplete cursor, which halts no-0020 binaries at owner watermark
     19 (not only after 0020 raises it to 20).
   - Extend F11 with the missing case: no-0020 deployer-aware binary, `applied_owner == 19`,
     `deploy_cursor.state in {in_progress, step_committed, finalizing}`, optional fingerprint mismatch,
     ±pending runtime migrations — assert `ApplyMigrations` is **not** called, `RecordSchemaFingerprint`
     is **not** called, the DB is byte-identical, and the halt is `awaiting_deploy`.
   - Extend `G-old-binary-refuse` to prove the pre-0020 incomplete-deploy window cannot be served, not
     merely that 0020 is refused when the ≥0044 marker is absent.

## 5. What already cleared (carry forward — do NOT regress or re-litigate)

- **C3 ownership transfer — resolved this cycle** (§3 above): preserve resolution (a) — 0020 terminal
  + excluded from `owner-ddl apply` + reconcile while CREATE held + revised F12 in the two-role cluster.
  (Its ordering consequence N2 is a *new* fix, not a re-litigation of the transfer mechanism.)
- **C1 (finalization boundary) — intact**: preserve the `finalizing` state + idempotent finalizer +
  §1.3 finalizing row + F10 verbatim.
- **C2 fail-closed activation edge — intact**: preserve `CheckDeployActivation` before `ApplyMigrations`,
  the typed halts, the forward-watermark rule, Required=19, F11. (N2 *adds* a pre-0020 edge; it does not
  change the existing one.)
- **Q4 — resolved** (v1/v2 ledgers): plain verb now, bootstrapping paradox dispositive, run-shape door
  held by the three seams. Neither falsifier contested it.
- **Q3 mid-step resumability body — sound**: Q3-A (cursor advance in the step transaction), Q3-B
  (`in_progress` pre-marker + invalid-index reconciler), the §1.1 step taxonomy. N1 is about the
  per-step *receipt key* identity, not the schema/cursor body — preserve the body.
- **Decoupling shape — sound** as a shape (shadow-first default-OFF `STRIATUM_DEPLOY_DECOUPLED`,
  `ConnectAndVerify` preserving the P2 watermark interlock, the embed-FS-derived plan, the hash-chained
  receipt). N2 is precisely that the *no-0020* serve path is not yet gated by the cursor — fix that
  without abandoning the shape.
- **Scope discipline held**: P5 (rehearse / expand-contract / fidelity tiering / clone) deferred; full
  capability revocation named as a follow-up beyond P4, not assumed; the local-first boundary respected.

## 6. Gate decision

The ledger verdict — not falsifier completion — clears the phase gate. With **N1 not genuinely
resolved** (the per-step receipt key `(plan_hash, step_index)` is not stable across resume under the
written pending-delta `BuildPlan`) and a **new material challenge standing** (the C3 revoke-last
ordering lets a no-0020 binary serve and self-record over an incomplete pre-0020 deploy), the gate is
**not cleared**, even though C3-as-ownership-transfer is genuinely resolved and C1 + the C2 edge are
carried forward intact.

**Verdict: `needs_revision`.** This is the gate's single allowed revision cycle, so the gate ends
**unCleared**; a clearing verdict would require a re-scaffolded revision that pins both §4 items
(immutable plan identity across resume, and a hard pre-0020 `deploy_cursor`-authoritative edge for
no-0020 binaries) without regressing C3, C1, the C2 edge, or the carry-forward items.

<sub>Adjudicator collaboration ledger for the RFC 0142 P4 falsification-gate design run, v3 /
REVISION, cycle 1. Inputs:
`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md`,
`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`, `SEED.md`; context: v2
`HOLDER.md` + v2 `COLLABORATION_LEDGER_cycle_1.md`. Verdict vocabulary: accept |
accept_with_findings | needs_revision | reject.</sub>
