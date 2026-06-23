# Design-Run Seed (v4 / REVISION) — RFC 0142 P4: the one-shot deployer (decouple schema-apply from serve-boot)

> **THIS IS THE FOURTH REVISION (v4).** Three prior design runs ran the same
> falsification gate. v1 (`rfc-0142-p4-design`) returned **`needs_revision`** with
> three findings C1/C2/C3. v2 (`rfc-0142-p4-design-v2`) **resolved C1 and C2** but
> returned `needs_revision` again because C3 (ownership policy) was still open and
> a new finding N1 (per-step receipt not crash-safe) landed. v3
> (`rfc-0142-p4-design-v3`) **resolved C3** — the SEED-recommended revoke-last
> ownership mechanism; both falsifiers conceded it as a coherent design answer —
> and **closed the immediate N1 hole** (the receipt now joins the owner-connection
> step transaction), BUT returned **`needs_revision` again** on two independent
> grounds, each squarely in the Q3 correctness core:
> **BC-N1** — the per-step `(plan_hash, step_index)` exactly-once receipt key is NOT
> stable across crash-resume — and **BC-N2** — the C3 revoke-last ordering opens a
> pre-terminal-revoke incomplete-deploy serve window a no-revoke-bundle binary does
> not gate. This v4 run is a **proper revision**: the holder starts from the **v3**
> `HOLDER.md` (a required context doc), REVISES the spec to **resolve BC-N1 + BC-N2**
> per their prescribed fixes below, and **carries C1/C2/C3 forward unregressed**;
> the falsifiers re-attack the revised spec. The v3 design record —
> `dialogue/holder/HOLDER.md`, `dialogue/falsifier_1/FALSIFIER.md`,
> `dialogue/falsifier_2/FALSIFIER.md`, and
> `dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — lives under
> `docs/operator/artifacts/rfc-0142-p4-design-v3/`; the **v3** `HOLDER.md` (the
> revised spec) and the **v3** collaboration ledger (the verdict + the full
> BC-N1/BC-N2 analysis and §4 "What the revision must fix") are wired in as required
> `context_docs`.
>
> This document is the **required input** for the RFC 0142 P4 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed at
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (status
> **accepted, D258**) — read it in full as your primary source; this SEED carries
> the charter, restates the two Open Questions P4 must pin (Q3 + Q4), pins the two
> binding revision constraints BC-N1 + BC-N2, states what already cleared (C1 + C2
> + C3) and must NOT be reopened, and pins an operator anchor-verification table you
> must build on. Read this whole file, the **v3** `HOLDER.md` + the **v3**
> collaboration ledger context docs, and the RFC (esp. the Phasing table P4 row and
> the "Open Questions" section) before producing any artifact.

## Charter — what this run must produce

This is a **design run**, not an implementation run. RFC 0142 is **already
accepted**; this run does NOT re-open the five-layer design. The deliverable is a
**falsifiable implementation spec for P4 only** — the one-shot `striatum daemon
deploy` that lifts schema-apply out of serve-boot — that the `rfc-0142-p4-build`
run can execute contract-first (TDD), produced by hardening the P4 shape against
adversarial falsification. **This judges pure engineering rigor — there is NO
maintainer ratification gate; the adjudicator's collaboration ledger decides.**

The committed `PROPOSAL.md` MUST:

1. **Resolve Q3 and Q4** (below) with a concrete, defensible decision each. Q3 is
   "the hard correctness core" — a P4 spec that leaves the resumability contract
   unproven for the interleavings we ship has not cleared the gate.
2. **Specify the deployer surface, the serve-boot decoupling, and the DDL
   revocation** by exact code site (anchor table below), shadow-first.
3. **State every load-bearing correctness claim as a falsifiable assertion**
   paired with the named test / game-day step that would prove it false.
4. **Stay inside the accepted design and the local-first boundary**, and
   explicitly **defer P5** (rehearsal / expand-contract / fidelity tiering / clone
   mechanism = Q1/Q2) — P4 is the deployer + decoupling + DDL revocation only.
5. **Resolve BOTH binding revision constraints (BC-N1, BC-N2) below**, each per its
   prescribed fix, and **carry forward C1 + C2 + C3 unregressed** (see "Carried
   forward"). A revised spec that leaves BC-N1 or BC-N2 open — or that regresses
   C1, C2, or C3 — has NOT cleared the gate. This is the gate's single allowed
   revision cycle for v4, so a second `needs_revision` ends the gate unCleared.

## Carried forward — resolved by v3 (do NOT reopen)

> The v3 collaboration ledger records C3, C1, the C2 fail-closed edge, and the v1
> carry-forward items as genuinely resolved / intact; **both v3 falsifiers conceded
> the C3 ownership-transfer answer.** The v4 revision MUST preserve them — verbatim
> from the **v3** `HOLDER.md` — and the cycle-4 adjudicator's clearing verdict
> requires them intact. Do NOT re-litigate any of them; they are not this cycle's
> open questions.

### C1 — RESOLVED (finalization boundary). Preserve verbatim from the v3 HOLDER.

The unclassifiable terminal state (`complete` cursor committed ahead of the receipt
+ fingerprint) is closed by **Option B + idempotent finalizer**:
- a distinct **`finalizing`** `deploy_cursor` state entered in one commit after the
  last step;
- an **idempotent finalizer** that appends the `complete` receipt (guarded skip on
  `(plan_hash, state=complete)`), UPSERTs the `schema_state` fingerprint, then
  advances `finalizing → complete` **LAST**;
- the **§1.3 row** classifying `finalizing` / expected `plan_hash` / (any
  fingerprint) as **resumable finalization — never serve, never genuine-drift**,
  with the in-sync row tightened to require `state == complete`;
- the operator pin that **no single finalization transaction spans the
  owner+runtime connections** — the only coherent close is the idempotent
  finalizer.
Preserve the `finalizing` state, the idempotent finalizer, the §1.3
resumable-finalization row, and `T-deploy-resume-finalization-crash` (F10).

### C2 — RESOLVED at the fail-closed EDGE. Preserve verbatim from the v3 HOLDER.

The crash-loop (#512-class: boot reaches `ApplyMigrations` over the runtime runner
under a revoked `CREATE` → raw `42501`) is closed at the edge by:
- **`CheckDeployActivation`** called immediately after `CheckOwnerBundleWatermark`
  and **BEFORE `ApplyMigrations`** in both `ConnectAndMigrate` and
  `ConnectAndVerify`, mutating nothing;
- **absent `deploy_cursor` treated as incomplete** (not error-on-missing-table);
- the typed non-restartable halts **`awaiting_deploy`** and
  **`awaiting_deploy_config`**, with the `main.go` `errors.As` arms onto the clean
  non-restartable exit;
- the **forward-watermark rule** so a revoke-unaware binary observing `applied >=
  20` halts `awaiting_deploy` instead of auto-applying;
- **`RequiredOwnerBundleVersion` kept at 19** (dissolving the auto-apply-default vs
  Required=20 contradiction);
- `T-deploy-revoke-activation-ordering` (F11).
Preserve all of this. **NOTE (this is exactly what BC-N2 extends, not replaces):**
the v3 ledger found this EDGE intact but **not globally sufficient** — the
revoke-last ordering opens a *pre-revoke* window (`applied_owner == 19`,
`deploy_cursor` incomplete) that this edge does not cover. BC-N2 ADDS a pre-revoke
edge; it must keep the existing C2 edge intact.

### C3 — RESOLVED (ownership transfer, revoke-last). Preserve verbatim from the v3 HOLDER.

The v2 self-contradiction (`ALTER … OWNER TO striatumd_rw` needs the new owner to
hold `CREATE ON SCHEMA striatumd`, which the DDL-revoke bundle REVOKEs, applied
before the reconcile) is resolved by **resolution (a) — sequence the revoke last**:
- the DDL-revoke bundle (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`) is
  **special-cased OUT of the owner prefix** and **EXCLUDED from `owner-ddl apply`**
  (a dedicated `DDLRevokeOwnerBundleVersion`; `LatestOwnerBundleVersion` /
  `ApplyOwnerBundles` bounded below it);
- it is applied as the **TERMINAL plan step** after every runtime ownership
  reconcile, so each `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still
  holds CREATE (granted by 0018 in the non-revoke prefix), and the committed steady
  state still denies CREATE;
- F12 (`T-deploy-runtime-object-ownership`) asserts, in a **documented
  non-superuser two-role cluster**: `has_schema_privilege('striatumd_rw',
  'striatumd', 'CREATE')` true at the reconcile (step_index strictly below the
  revoke's); catalog owner == `striatumd_rw` for every created object (table, index,
  sequence); the serving role's real `SELECT/INSERT/UPDATE/DELETE`; post-deploy
  `SET ROLE striatumd_rw; CREATE TABLE → 42501`; `has_schema_privilege … CREATE`
  false post-deploy; idempotent re-run; plus `G-revoke-last`.
Preserve resolution (a), F12, and `G-revoke-last`. **The ordering this adopts is
precisely what SPAWNED BC-N2 — fixing BC-N2 must NOT change the C3 mechanism, only
add the pre-revoke serve edge.**
> **⚠ Ordinal drift (re-anchor, do not reopen the mechanism).** The v3 spec named
> the DDL-revoke bundle **0020**. On current `main` ordinal **0020 is already
> taken** by `go/pkg/db/sql/owner/0020_owner_bundle_watermark_read.sql` (a `GRANT
> SELECT` for #581, RFC 0142 Layer 2) and `LatestOwnerBundleVersion == 20`
> (`go/pkg/db/owner.go:23`). So the DDL-revoke bundle the C3 mechanism special-cases
> must **renumber to `>= 0021`**; the special-case + terminal-ordering + exclude-from-
> `owner-ddl apply` logic is unchanged — only the ordinal moves. Re-anchor F12 and
> the plan generator accordingly.

### Other carry-forward (v1/v2/v3 ledgers — do NOT re-litigate)

- **Q4 — resolved.** Plain verb now (`striatum daemon deploy`), bootstrapping
  paradox dispositive against a run-shape base deployer, run-shape door held open
  by three seams (content-addressed `plan_hash`, hash-chained receipt in
  `audit_log`, a shared `Deployer.Apply` engine). No falsifier contested it.
- **Q3 mid-step resumability body — sound.** Q3-A (cursor advance in the step
  transaction), Q3-B (`in_progress` pre-marker + invalid-index reconciler), the
  §1.1 step taxonomy, and the `deploy_cursor`-as-second-signal insight. BC-N1 is
  about the receipt **key identity** across a moved frontier, not the per-step
  schema/cursor body — preserve the body, fix the identity.
- **Decoupling shape — sound** as a shape: shadow-first default-OFF
  `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2 watermark
  interlock + P3 drift gate / self-record, the embed-FS-derived plan, the
  hash-chained receipt, `doctor schema_deploy_unrecorded`. BC-N2 is precisely that
  the *no-revoke* serve path is not yet gated by the cursor — fix that **without
  abandoning the shape**.
- **Scope discipline held.** P5 (rehearse / expand-contract / fidelity tiering /
  clone) deferred; full capability revocation named as a follow-up beyond P4, not
  assumed; the local-first boundary respected.

## The 2 binding constraints v4 MUST resolve (the v3 adjudicator's two unrebutted needs_revision grounds)

> **These are BINDING.** The design-v3 falsification gate returned `needs_revision`
> because two material falsifier challenges landed and stood unrebutted: **BC-N1**
> (falsifier_1, the atomicity/resumability lens) and **BC-N2** (falsifier_2, the
> decoupling/migration-safety lens), both in the P4 correctness core. The revised
> spec clears the gate **only if it pins both** with the prescribed fix, **and does
> not regress C1, C2, or C3**; the cycle-4 falsifiers re-attack each one
> specifically. The verbatim prescribed fixes are §4 "What the revision must fix" in
> `docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
> — read that context doc in full.

### BC-N1 (high — Q3/R4 correctness core): the `(plan_hash, step_index)` receipt key is not stable across crash-resume

**The gap.** v3 closed the immediate hole (the receipt now joins the
owner-connection step tx for Q3-A; an idempotent `(plan_hash, step_index)`
reconcile for Q3-B; a per-step doctor) BUT **the key itself moves**.
`BuildPlan(applied_owner, applied_runtime)` (v3 HOLDER §3.2, `BuildPlan` at
`HOLDER.md:255`) builds the **PENDING-delta** plan from the LIVE frontiers, and
current source **advances those frontiers at each committed step**:
- `go/pkg/db/migrations.go` — `applyOne` stamps `schema_migrations`
  (`migrations.go:320`) inside the step tx and `ApplyMigrations` advances `current =
  migration.Version` per step (`migrations.go:121-172`);
- `go/pkg/db/owner.go` — `ApplyOwnerBundles` inserts `owner_bundle_meta` per bundle
  (`owner.go:265,529`) and `OwnerBundleVersion = COALESCE(MAX(version),0)`
  (`owner.go:228-238`).

`deploy_cursor` persists only `{plan_hash, state, step_index, step_id}` (v3 HOLDER
§1.2) — **neither the ordered transcript nor the base/target frontiers** — so the
original plan hash is unrecoverable. From owner 19 / runtime 43 the first deploy
builds `H = hash([runtime:0044, runtime:0045, revoke-bundle])`; after step 0
(`runtime:0044`) commits its `(H,0)` receipt and the process dies, a re-run on
frontier `(19,44)` rebuilds `H' = hash([runtime:0045, revoke-bundle]) != H` and
renumbers `step_index`. Consequences (all load-bearing):
1. the `(H,0)` receipt for already-applied `runtime:0044` **cannot be recomputed**
   — the exactly-once N1 key has no stable identity;
2. the §1.3 `in_progress`/`step_committed` row only matches `plan_hash == the
   binary's plan` — the cursor holds `H` while the binary recomputes `H'`, so the
   interrupted deploy is **unclassified foreign-incomplete**, exactly the bucket
   §1.3 was meant to eliminate;
3. the tightened `doctor schema_deploy_unrecorded`, reconstructing `BuildPlan` from
   the moved frontier, **has no specified way to require `(H,0)`**.

The holder's stability claim ("`step_index` stable because content-addressed by
`plan_hash`") is **circular**: `plan_hash` itself moves with the pending delta.

**Prescribed fix (binding, falsifiable):**
- **Materialize the IMMUTABLE ordered transcript** (and the base+target frontiers)
  in `deploy_cursor` (or a new `deploy_plan` table) **BEFORE step 0** — record
  `base_owner_version`, `base_runtime_version`, the target frontiers, every
  `{step_index, step_id, role, sha256}`, and the terminal-revoke placement.
- **Resume reads the STORED transcript** by `plan_hash` (verifying the embedded
  bytes still match the binary) and resumes at the next step **in that transcript**
  — **never** recompute the plan from the moved live frontiers.
- **Teach §1.3 and the per-step doctor** to key off the stored transcript so
  `(plan_hash, step_index)` are stable identities; an incomplete cursor whose
  `plan_hash` differs from the binary's freshly-computed pending plan is a
  **recoverable** "resume with the stored plan" state, never an unclassified drift
  bucket.
- **Named test — `T-deploy-plan-hash-resume-after-step`:** a plan with ≥2 runtime
  steps + terminal revoke; kill after step 0 commits **and** after step 1 commits;
  assert the re-run reuses the same `plan_hash`, preserves the original
  `step_index`es, recognizes the prior receipts exactly-once, completes the
  remaining steps, and leaves `doctor` green.

> Source sites to re-anchor: `go/pkg/db/migrations.go:121-172,320`;
> `go/pkg/db/owner.go:225-245,304-320` (`OwnerBundleVersion`, `ApplyOwnerBundles`);
> the v3 `deploy_cursor` payload shape (v3 HOLDER §1.2, `BuildPlan` §3.2 at
> `HOLDER.md:255`).

### BC-N2 (high — decoupling/migration-safety core): the C3 revoke-last ordering opens a pre-revoke incomplete-deploy serve window

**The gap.** `CheckDeployActivation` consults `deploy_cursor` only when
`revokeEmbedded` (the binary ships the revoke bundle; v3 HOLDER §3.3a) — a
no-revoke deployer-aware binary makes it **INERT**. The forward-watermark rule
fires only when a no-revoke binary observes `applied_owner >= 20` (v3 HOLDER §4.2),
but revoke-last keeps `applied_owner == 19` until the **TERMINAL revoke** commits
(v3 HOLDER §3.3b/§4.3). Reproducer (the holder's own two-binary choreography):
- the inert-landing binary (no revoke bundle) serves at owner 19;
- the activation binary runs `deploy`;
- kill after a runtime step commits `step_committed(k)` but before the
  revoke/finalizing (the **C3-safe point** — CREATE still held);
- restart the inert/no-revoke binary → `CheckDeployActivation` inert
  (`!revokeEmbedded`), forward-watermark inert (`applied_owner == 19`), so it
  **never reads `deploy_cursor`**, reaches legacy `ApplyMigrations` + the P3 path
  (`go/pkg/db/connection.go:341-353,376-402`).

The P3 drift gate is **SHADOW by default** — `CheckSchemaDrift` returns
drifted-but-nil when `STRIATUM_SCHEMA_DRIFT_REFUSE` is unset
(`go/pkg/db/schema_drift.go:28,239-274`, the env constant `EnvSchemaDriftRefuse` at
`schema_drift.go:28`; shadow note `schema_drift.go:42,198`) — so the boot **serves
AND self-records** a fingerprint over an incomplete deploy. This (a) defeats
**`incomplete cursor never serves`** across the rollout pair, (b) regresses the
**P3/P4 self-record split** (a should-be-verify-only boot mutate-and-self-records,
masking the partial deploy before the P4 finalizer writes the terminal
receipt/fingerprint), and (c) is **missed by F11** (whose old/no-revoke matrix only
tests `applied_owner == 20`). A #512-class shape in clean-halt clothing; the only
rebuttal (operator parks the daemon) is choreography discipline, insufficient for a
deployer sold as crash-resume / rollback-resistant.

**Prescribed fix (binding, falsifiable):**
- Make `deploy_cursor` **AUTHORITATIVE before the terminal revoke** — give the serve
  path one **hard edge** that holds at `applied_owner == 19`: **every**
  deployer-aware binary (INCLUDING the no-revoke landing binary) reads
  `deploy_cursor` **before `ApplyMigrations` and before `RecordSchemaFingerprint`**
  and halts **`awaiting_deploy` DB-untouched** on a non-`complete` cursor,
  **regardless of `revokeEmbedded`/forward-watermark**; OR introduce a **durable
  pre-revoke activation marker** set before the first deploy step that can leave an
  incomplete cursor, which halts no-revoke binaries at owner watermark 19.
- The fix MUST NOT regress C3 revoke-last: it must not block the deploy from
  completing nor strand `ALTER … OWNER TO striatumd_rw` while CREATE is still held,
  and must not wedge a clean later boot (a `complete` cursor or a fresh DB with no
  cursor must serve).
- **Named tests — extend `F11` + `G-old-binary-refuse`:** the no-revoke
  deployer-aware binary at `applied_owner == 19`, `deploy_cursor.state in
  {in_progress, step_committed, finalizing}`, ±pending runtime migrations — assert
  `ApplyMigrations` is **NOT** called, `RecordSchemaFingerprint` is **NOT** called,
  the DB is byte-identical, and the halt is `awaiting_deploy`. `G-old-binary-refuse`
  must prove the pre-revoke incomplete-deploy window cannot be served (not merely
  that the revoke bundle is refused when the `>= 0044` marker is absent).

> Source sites to re-anchor: `go/pkg/db/connection.go:341-353,376-402`
> (`ConnectAndMigrate`/`ConnectAndVerify`, `ApplyMigrations`, `CheckSchemaDrift`,
> `RecordSchemaFingerprint`); `go/pkg/db/schema_drift.go:28,239-274` (shadow
> default); v3 HOLDER §3.3a (`revokeEmbedded` gate), §4.2 (forward-watermark),
> §3.3b/§4.3 (revoke-last keeps `applied_owner == 19`).

## Root reframe (do not lose this)

**Schema mutation must stop being an implicit side effect of the serving process's
restart and become an explicit, ordered, resumable, provenance-tracked operation
owned by a dedicated deployer.** Then the serving daemon can hold zero DDL
privilege, "restart force-commits a half-applied deploy" becomes impossible, and a
bad migration can never wedge the single writer on boot.

## The two Open Questions P4 must pin (from the RFC)

- **Q3 — How atomic is "atomic"?** Confirm the **per-step-atomic + resumable-cursor**
  contract is sufficient for every owner+runtime interleaving we actually ship, or
  specify the small set of steps that need a stricter sub-protocol. Every step must
  be idempotent and leave a coherent intermediate the cursor/transcript classifies
  as "incomplete, resume" — not "unknown drift, panic". **In v4 this explicitly
  includes the immutable-transcript receipt key (BC-N1) and the pre-revoke serve
  edge (BC-N2).** *(RFC: "This is the hard correctness core of P4.")*
- **Q4 — Should a deploy be a Striatum run?** Plain verb vs. a dogfooded run shape,
  with the bootstrapping paradox. Resolved in v1/v2/v3 — carry forward.

## Load-bearing risks (attack these)

- **R1 atomicity-is-partly-a-lie:** non-transactional DDL, non-idempotent steps, a
  two-connection crash window, or **the receipt key moving with the frontier
  (BC-N1)**. Test: kill-and-resume across each step class **and across a mid-plan
  crash that moves the live frontier**.
- **R2 decoupling regresses a landed gate:** lifting `ApplyMigrations` out of
  `ConnectAndMigrate` must NOT break the P2 watermark interlock, the P3 drift gate /
  fingerprint self-record, or fresh-DB bring-up; **and the new pre-revoke
  `deploy_cursor` read (BC-N2) must not wedge a clean boot** (fresh DB / `complete`
  cursor).
- **R3 DDL-revocation lockout / pre-revoke serve:** revoking serving-role DDL must
  not lock out the runtime path, **nor let a no-revoke binary serve a pre-revoke
  incomplete deploy (BC-N2)**.
- **R4 cursor / transcript / receipt holes:** double-apply/skip at a commit
  boundary; **a receipt key that cannot be recomputed after the frontier moves
  (BC-N1)**; a stored transcript that disagrees with the embedded binary on resume;
  out-of-order apply under the plan's edges.

## Anchor verification against current `main` (operator pre-flight)

Verified against `~/git/striatum` @ `origin/main`. P0–P3 + P2 are **landed**; the
P4 surfaces are **NOT-FOUND (to be built)**. Treat as ground truth; re-anchor the
spec to these file:line references. **Re-confirm; several frontiers MOVED since v3
was authored.**

| Claim / target | Status | Anchor (current source) |
| --- | --- | --- |
| Boot-time auto-apply runs runtime migrations as `striatumd_rw` | **ACCURATE (the coupling P4 removes)** | `go/pkg/db/connection.go:353` `ApplyMigrations(ctx, pool.Runner, daemonVersion)` inside `ConnectAndMigrate`; P3 path `:376-402`. |
| Live frontiers ADVANCE at each committed step (the BC-N1 mechanism) | **ACCURATE** | `go/pkg/db/migrations.go:121-172` `ApplyMigrations` (`current = migration.Version` per step), `:320` per-step `schema_migrations` insert; `go/pkg/db/owner.go:228-238` `OwnerBundleVersion = COALESCE(MAX(version),0)`, `:265,529` `ApplyOwnerBundles` per-bundle insert. |
| Runtime migration frontier = 0043 (P3 added `schema_state`) | **ACCURATE** | `go/pkg/db/migrations.go:17` `LatestDaemonDBVersion = 43`. New P4 runtime migration is **≥ 0044**. |
| **Owner bundle frontier MOVED to 0020 — and 0020 is NOT the DDL-revoke bundle** | **CHANGED since v3 (re-anchor)** | `go/pkg/db/owner.go:23` `LatestOwnerBundleVersion = 20`, `:35` `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` (= 20). `go/pkg/db/sql/owner/0020_owner_bundle_watermark_read.sql` is a `GRANT SELECT` (#581, RFC 0142 Layer 2), **not** a `REVOKE CREATE`. **The C3 DDL-revoke bundle must renumber to `>= 0021`.** |
| P3 fingerprint machinery + SHADOW-by-default drift gate (BC-N2 hinges on this) | **ACCURATE (landed)** | `go/pkg/db/schema_drift.go`: `CheckSchemaDrift()` `:239-274`, env `STRIATUM_SCHEMA_DRIFT_REFUSE` const `EnvSchemaDriftRefuse` `:28`, shadow-default note `:42,198`; boot self-record `connection.go:376-402` (`RecordSchemaFingerprint` `:399`). |
| P2 watermark interlock + clean halt | **ACCURATE (landed)** | `go/pkg/db/owner.go` `CheckOwnerBundleWatermark` `:116-151`, `AwaitingOwnerDDLError`; called **before** `ApplyMigrations` in boot. |
| `ALTER … OWNER TO striatumd_rw` needs CREATE; 0018/0019 GRANT CREATE FIRST (the C3 prerequisite) | **ACCURATE** | `go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:58-72` (prereq), `:97-102` (`GRANT CREATE … TO striatumd_rw`); `go/pkg/db/sql/owner/0019_supervisor_pointer_runtime_ownership.sql:53-80`. |
| Owner-only `append_audit_row` (the per-step + terminal receipt sink) | **ACCURATE** | `go/pkg/db/sql/owner/0001_authority_phase0.sql:223-238` (revokes direct `audit_log` INSERT `:237`, grants only the SD `append_audit_row` `:223,238`). |
| `owner-ddl apply` applies bundles out-of-band as owner role | **ACCURATE (landed)** | `go/pkg/cli/localcommands/daemon.go` `runDaemonOwnerDDL` → `db.ApplyOwnerBundles()`. P4's `deploy` lives alongside this; the renumbered DDL-revoke bundle is **excluded** from it. |
| `striatum daemon deploy` / `deploy_cursor` / `deploy_plan` / deploy plan / deploy receipt | **NOT-FOUND (P4 builds these)** | `go/pkg/cli/localcommands/daemon.go` subcommands = `install,uninstall,status,migrate-db,owner-ddl` — no `deploy`. No `deploy_cursor`/`deploy_plan` table, no plan/receipt machinery anywhere. |

**Net design implication.** The P4 ground is clean: P2 and P3 are landed and are
exactly the contract P4 leans on; C1, C2, and C3 are resolved in v1–v3. The
remaining hard parts are **BC-N1** — make the per-step receipt key an immutable
identity (materialize the ordered transcript before step 0; resume off the stored
transcript, never the moved live frontiers; key §1.3 and the doctor to it) — and
**BC-N2** — make `deploy_cursor` authoritative before the terminal revoke so a
no-revoke binary at `applied_owner == 19` cannot serve a pre-revoke incomplete
deploy. Falsifiers must press BC-N1 across a mid-plan crash that moves the frontier,
and press BC-N2 against the no-revoke / shadow-P3 serve path. Be shadow-first:
serve-boot auto-apply stays the default until the deployer is proven, then flips
behind a flag.

## Clearing condition (pin this)

The gate clears **only** if the cycle-4 adjudicator's collaboration ledger records a
clearing verdict (`accept` or `accept_with_findings`, never `clear`), which requires
**all** of: **BC-N1 genuinely resolved** (immutable stored transcript persisted
before step 0; resume off the stored transcript; stable `(plan_hash, step_index)`;
§1.3 + per-step doctor keyed to the transcript; `T-deploy-plan-hash-resume-after-step`),
**BC-N2 genuinely resolved** (pre-revoke `deploy_cursor`-authoritative edge for
no-revoke binaries at `applied_owner == 19`; extended `F11` + `G-old-binary-refuse`),
**C1 + C2 + C3 carried forward intact**, and **no new material challenge standing
unrebutted**. A spec that merely *claims* the two fixes without the concrete
immutable-transcript resume protocol and the pre-revoke serve edge has NOT cleared
the gate. This is the single allowed revision cycle for v4 — a second
`needs_revision` ends the gate unCleared.

---
<sub>Operator scaffold for the RFC 0142 P4 falsification-gate design run (v4 /
REVISION of `rfc-0142-p4-design-v3`; resolves the cycle-3 findings BC-N1 + BC-N2 and
carries C1/C2/C3 forward). Lanes: author=claude (holder/adjudicator/committer),
reviewer=codex (falsifiers).</sub>

## Operator sharpenings (pin these — do NOT let cycle-4 re-derive them)

- **BC-N1: persist the transcript BEFORE step 0; resume off the STORED transcript.**
  The defect is that `plan_hash`/`step_index` are derived from the pending delta,
  which moves with the live frontier; the fix is to make plan identity an immutable
  durable fact written before any step mutates the frontier, and to make resume,
  §1.3, and the doctor read that durable fact — never recompute `BuildPlan` over the
  moved frontiers. The reproducer the cycle-4 falsifier_1 MUST re-run: kill after
  step 0 commits and prove the re-run reuses the SAME `plan_hash` and `step_index`es
  and recognizes the prior `(plan_hash,0)` receipt exactly-once.
- **BC-N2: the pre-revoke edge must hold at `applied_owner == 19`, for the no-revoke
  binary.** The existing C2 edge gates only revoke-embedding binaries (or
  `applied >= 20`); the new edge must make EVERY deployer-aware binary read
  `deploy_cursor` before it mutates or self-records, and halt `awaiting_deploy`
  DB-untouched on a non-`complete` cursor — OR a durable pre-revoke marker that does
  the same. Prove it without regressing C3 (the deploy still completes; no stranded
  `ALTER … OWNER`) and without wedging a clean boot (fresh DB / `complete` cursor
  still serves). The cycle-4 falsifier_2 MUST re-run the two-binary choreography and
  prove the no-revoke binary REFUSES, with `ApplyMigrations` and
  `RecordSchemaFingerprint` both un-called and the DB byte-identical.
- **Re-anchor the DDL-revoke bundle to `>= 0021`.** 0020 is now taken by
  `0020_owner_bundle_watermark_read.sql` and `LatestOwnerBundleVersion == 20`. The
  C3 mechanism (special-case + terminal + exclude from `owner-ddl apply`) is
  unchanged; only the ordinal moves. Do NOT silently inherit the v3 "bundle 0020 =
  the revoke / `LatestOwnerBundleVersion` stays 19" claim — it is now false against
  `main`.
- **Keep C1, BC-N1, and the finalizer coherent.** The §1.3 classification table, the
  `finalizing` finalizer's terminal exactly-once `complete` receipt, the new
  immutable-transcript per-step receipt key, and the pre-revoke serve edge must all
  hold together — a resume must never serve, must never advance past a step missing
  its receipt, and must classify a stored-transcript cursor whose `plan_hash` is not
  the binary's pending plan as a recoverable resume state.
