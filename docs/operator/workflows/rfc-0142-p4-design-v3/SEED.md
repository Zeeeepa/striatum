# Design-Run Seed (v3 / REVISION) — RFC 0142 P4: the one-shot deployer (decouple schema-apply from serve-boot)

> **THIS IS THE THIRD REVISION (v3).** Two prior design runs ran the same
> falsification gate. v1 (`rfc-0142-p4-design`) returned **`needs_revision`** with
> three findings C1/C2/C3. v2 (`rfc-0142-p4-design-v2`) **resolved C1 and C2** —
> both falsifiers conceded the v1 C1 finalization-boundary shape and the C2
> fail-closed activation edge are genuinely closed — but returned `needs_revision`
> **again** because **C3 (ownership policy) is still open** and a **new finding N1
> (per-step receipt not crash-safe)** landed. This v3 run is a **proper revision**:
> the holder starts from the **v2** `HOLDER.md` (a required context doc), REVISES
> the spec to **resolve C3 + N1** per their prescribed fixes below, and **carries
> C1/C2 forward unregressed**; the falsifiers re-attack the revised spec. The v2
> design record — `dialogue/holder/HOLDER.md`, `dialogue/falsifier_1/FALSIFIER.md`,
> `dialogue/falsifier_2/FALSIFIER.md`, and
> `dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — lives under
> `docs/operator/artifacts/rfc-0142-p4-design-v2/`; the **v2** `HOLDER.md` (the
> revised spec) and the **v2** collaboration ledger (the verdict + the full C3/N1
> analysis and prescribed fixes) are wired in as required `context_docs`.
>
> This document is the **required input** for the RFC 0142 P4 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed
> at `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (status
> **accepted, D258**) — read it in full as your primary source; this SEED carries
> the charter, restates the two Open Questions P4 must pin (Q3 + Q4), pins the
> two binding revision constraints C3 + N1, states what already cleared (C1 + C2)
> and must NOT regress, and pins an operator anchor-verification table you must
> build on. Read this whole file, the **v2** `HOLDER.md` + the **v2** collaboration
> ledger context docs, and the RFC (esp. the Phasing table P4 row and the "Open
> Questions" section) before producing any artifact.

## Charter — what this run must produce

This is a **design run**, not an implementation run. RFC 0142 is **already
accepted**; this run does NOT re-open the five-layer design. The deliverable is a
**falsifiable implementation spec for P4 only** — the one-shot `striatum daemon
deploy` that lifts schema-apply out of serve-boot — that the `rfc-0142-p4-build`
run can execute contract-first (TDD), produced by hardening the P4 shape against
adversarial falsification.

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
5. **Resolve BOTH binding revision constraints (C3, N1) below**, each per its
   prescribed fix, and **carry forward C1 + C2 unregressed** (see "Already
   cleared"). A revised spec that leaves C3 or N1 open — or that regresses C1 or
   C2 — has NOT cleared the gate. This is the gate's single allowed revision
   cycle for v3, so a second `needs_revision` ends the gate unCleared.

## Binding revision constraints (v2 gate — MUST resolve)

> **These are BINDING.** The design-v2 falsification gate returned
> `needs_revision` because two material falsifier challenges landed and stood
> unrebutted: **C3** (reopened by falsifier_2) and **N1** (a new finding from
> falsifier_1), both squarely in the P4 correctness core. The revised spec clears
> the gate **only if it pins both** with the prescribed fix, **and does not
> regress C1 or C2**; the cycle-3 falsifiers re-attack each one specifically and
> the adjudicator's clearing verdict requires both findings genuinely resolved
> (not merely claimed) AND C1/C2 intact AND no new material challenge standing.
> The verbatim prescribed fixes are copied from
> `docs/operator/artifacts/rfc-0142-p4-design-v2/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
> (the `findings:` block and §4 "What the revision must fix"); read that context
> doc in full.

### C3 (high — two-role boundary): the ownership transfer self-contradicts the bundle-0020 CREATE revoke

**The gap.** The v2 spec chose **Policy 1** (runtime objects stay
`striatumd_rw`-owned) and reconciles ownership in the same step transaction with
`ALTER <kind> striatumd.<name> OWNER TO striatumd_rw` (v2 `HOLDER.md` §3.3b). But
PostgreSQL requires the **NEW** owner to hold `CREATE ON SCHEMA striatumd` for
that transfer to succeed — the repo's own bundles document exactly this and
**GRANT `CREATE` FIRST** for that reason:
`go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:58-72` ("striatumd_rw
MUST hold CREATE on schema striatumd … otherwise `ALTER … OWNER TO striatumd_rw`
fails `permission denied for schema striatumd`") and `:97-102`
(`GRANT CREATE ON SCHEMA striatumd TO striatumd_rw` before the transfer loop);
`go/pkg/db/sql/owner/0019_supervisor_pointer_runtime_ownership.sql:53-80` repeats
the prerequisite and grant. **This phase's bundle 0020 REVOKEs precisely that
privilege** (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`), and the v2
plan applies owner bundles **before** runtime steps. So once 0020 commits, the
subsequent `ALTER … OWNER TO striatumd_rw` fails `permission denied for schema
striatumd` in the documented non-superuser two-role topology (the owner is merely
a member of `striatumd_rw`, which post-0020 lacks `CREATE`). **The reconcile
strands the activation deploy after 0020 at the first runtime ownership reconcile**
— turning a catalog-ownership question into an activation failure / lockout class,
and undercutting the C2 `awaiting_deploy` remediation (whose only exit is the very
command with this self-inflicted privilege contradiction). v2 `F12` is internally
inconsistent: it requires both `relowner == striatumd_rw` for every created object
**and** post-0020 `SET ROLE striatumd_rw; CREATE TABLE → 42501`, which cannot both
hold without an unspecified mechanism. The prescribed fix is **claimed, not
implementable** as written.

**Prescribed fix (binding).** Pick and **fully specify ONE coherent resolution**,
with a test. The three viable resolutions (recommend **(a)** — simplest, no
transient elevated grant):

- **(a) Sequence the revoke last.** Apply the bundle-0020 `REVOKE CREATE ON SCHEMA
  striatumd FROM striatumd_rw` as the **FINAL step of the deploy plan**, AFTER all
  runtime steps and ownership reconciles complete — so `ALTER … OWNER TO
  striatumd_rw` succeeds during the deploy (CREATE is still held while the
  reconciles run) and the **steady state still denies `striatumd_rw` CREATE**.
  This requires the plan generator (v2 §3.2: owner bundles in ascending Version,
  then runtime migrations) to **special-case the DDL-revoke bundle 0020 to sort to
  the END of the plan**, after the last runtime ownership reconcile — and state
  precisely how the plan ordering, the `deploy_cursor` step indices, the C2
  `CheckDeployActivation` predicate, and the §1.3 in-sync classification stay
  coherent with 0020 as the terminal step (e.g. the deploy is `complete` only once
  0020 has committed and the fingerprint includes it); OR
- **(b) Scoped temporary grant.** The deployer (owner connection) does
  `GRANT CREATE ON SCHEMA striatumd TO striatumd_rw` immediately before the
  ownership-transfer loop and `REVOKE CREATE` immediately after, **within the
  deploy** (mirroring how 0018/0019 GRANT first), with a test proving (i) the
  post-deploy steady state denies `striatumd_rw` CREATE and (ii) the daemon does
  **not** serve while the grant is live (no boot can observe the transient
  elevated grant); OR
- **(c) Policy 2.** The owner/admin role **OWNS** new runtime objects + a
  build/load guard that every runtime migration grants the exact DML
  `striatumd_rw` needs, and **correct §4.1 to drop the runtime-ownership safety
  claim** (it can no longer say new runtime objects stay `striatumd_rw`-owned).

Whichever is chosen, **resolve the v2 F12 internal inconsistency** and add
**`T-deploy-runtime-object-ownership`** asserting, in a **documented non-superuser
two-role cluster**: (1) the recorded catalog owner of every created object (table,
its index, its sequence) under the chosen policy; (2) the serving role's real DML
(`SELECT`/`INSERT`/`UPDATE`/`DELETE`) under `striatumd_rw`; AND (3) the
**post-deploy CREATE denial** — `SET ROLE striatumd_rw; CREATE TABLE … → 42501`
after the deploy commits. Re-run the deployer and assert idempotence.

### N1 (high — Q3/R4 correctness core): the per-step deploy receipt is not crash-safe

**The gap.** The per-step deploy receipt is load-bearing (v2 claim RECEIPT §3.4;
F7; RFC 0142 Layer 3 makes every schema change first-class adjudicated
provenance), but it is **not** made crash-safe the way the cursor markers and the
**terminal** `complete` receipt are. The per-step receipt is appended over the
**OWNER** connection (`append_audit_row`, owner-only) — the same two-connection
constraint as C1 — so the v2 §3.3b step-transaction recipe (BEGIN → DDL →
ownership reconcile → grants → cursor advance → COMMIT) **omits it**, and §3.4's
only idempotency guard is for the **terminal** `(plan_hash, state=complete)`
receipt. Two concrete interleavings break it:
- **Transactional step k.** The step commits (cursor `step_committed(k)`) and the
  process dies **before** its per-step receipt is appended. Resume reads
  `step_committed(k)`, advances to `k+1`, **never backfills the missing receipt**,
  and `doctor schema_deploy_unrecorded` (terminal-only check) can be **green**
  while step k's receipt is permanently absent — the receipt chain disagrees with
  the real schema history.
- **NT-DDL step (Q3-B).** The receipt cannot be atomic with the side effect; a
  crash **after** the receipt append but **before** `step_committed(k)` is
  undefined — duplicate receipt vs silent skip, with **no per-step idempotency
  key** to disambiguate.

**Prescribed fix (binding).** A crash between a step's commit and its receipt-write
must be reconciled **idempotently** — mirror the C1 finalizer: an **idempotent
per-step receipt reconcile keyed on `(plan_hash, step_index)`** that the
resume/finalizer completes on re-run, so **every applied step provably ends with
exactly one hash-chained receipt** and `doctor schema_deploy_unrecorded` is green
**only when all committed steps have receipts**. Concretely:
- **Q3-A (transactional step):** state that the step receipt append occurs in the
  **same owner-connection transaction** as the DDL + ownership reconcile + grants +
  version stamp + cursor advance (legal because the deployer applies runtime steps
  over the owner connection and `append_audit_row` runs in the caller's
  transaction — confirmed in current SQL); name that connection/role. So a
  transactional step is `step_committed(k)` **iff** its receipt is durable.
- **Q3-B (non-transactional step):** add a **step-level idempotent receipt rule
  keyed on `(plan_hash, step_index)`** (and `step_id`/`sha256`) so the
  `in_progress(k)` reconciler appends **exactly one** receipt **before** writing
  `step_committed(k)` — resolving the duplicate-vs-skip ambiguity. The reconcile
  is also what backfills a transactional step whose receipt is somehow missing
  (defense-in-depth) so resume never advances past a step lacking its receipt.
- **Tighten `doctor schema_deploy_unrecorded`** so a **missing per-step receipt is
  surfaced**, not masked by a present terminal `complete` receipt.

Specify the reconcile precisely and add **`T-deploy-receipt-crash-resume`** that
**kills between a step's commit and its receipt-write** (transactional: after
DDL+cursor commit before receipt; NT-DDL: after the side effect before receipt;
after receipt before `step_committed`; after `step_committed`) and asserts, after
each crash point: final schema equality, **exactly one** receipt per applied step
on re-run, one terminal `complete` receipt, and a **green** doctor.

## Already cleared — carry forward, do NOT regress

> The v2 collaboration ledger (§3, §5) records C1, C2, and the v1 carry-forward
> items as genuinely resolved / sound; **both v2 falsifiers conceded C1 and the C2
> edge.** The v3 revision MUST preserve them — verbatim from the **v2** `HOLDER.md`
> — and the cycle-3 adjudicator's clearing verdict requires them intact.

### C1 — RESOLVED (finalization boundary). Preserve verbatim from the v2 HOLDER.

The v1 unclassifiable state (`complete` cursor committed ahead of the receipt +
fingerprint) is closed by the prescribed **Option B + idempotent finalizer**:
- a distinct **`finalizing`** `deploy_cursor` state entered in one commit after the
  last step (v2 §1.2);
- an **idempotent finalizer** that appends the `complete` receipt (guarded skip on
  `(plan_hash, state=complete)`), UPSERTs the `schema_state` fingerprint, then
  advances `finalizing → complete` **LAST** (v2 §1.2, §3.4);
- the **new §1.3 row** classifying `finalizing` / expected `plan_hash` / (any
  fingerprint) as **resumable finalization — never serve, never genuine-drift**,
  and the in-sync row tightened to require `state == complete` (v2 §1.3);
- the operator pin that **no single finalization transaction spans the
  owner+runtime connections** (v2 §3.4) — the only coherent close is the
  idempotent finalizer.
Preserve the `finalizing` state, the idempotent finalizer, the §1.3
resumable-finalization row, and `T-deploy-resume-finalization-crash` (F10).
**Note:** the C1 finalizer and the N1 per-step receipt reconcile are the same
discipline (atomic-or-idempotent provenance writes across the two connections) at
the **terminal** vs the **per-step** boundary — keep them coherent and
non-conflicting; the §1.3 table and the finalizer must still hold once the
per-step receipt rule is added.

### C2 — RESOLVED (fail-closed activation edge). Preserve verbatim from the v2 HOLDER.

The v1 crash-loop (#512-class: boot reaches `ApplyMigrations` over the runtime
runner under a revoked `CREATE` → raw `42501`) is closed by:
- **`CheckDeployActivation`** called immediately after `CheckOwnerBundleWatermark`
  and **BEFORE `ApplyMigrations`** in both `ConnectAndMigrate` and
  `ConnectAndVerify`, mutating nothing (v2 §3.3a);
- **absent `deploy_cursor` treated as incomplete** (not error-on-missing-table) —
  the operator bootstrap-order sharpening (v2 §3.3a, §4.3);
- the typed non-restartable halts **`awaiting_deploy`** (deploy pending/incomplete)
  and **`awaiting_deploy_config`** (revoke-embedding binary, flag OFF), with the
  `main.go` `errors.As` arms onto the clean non-restartable exit (v2 §4.2);
- the **forward-watermark rule** so a revoke-unaware binary observing `applied >=
  20` halts `awaiting_deploy` instead of auto-applying (v2 §4.2);
- **`RequiredOwnerBundleVersion` kept at 19** (dissolving the auto-apply-default vs
  Required=20 contradiction) (v2 §4.2);
- `T-deploy-revoke-activation-ordering` (F11).
Preserve all of this. (The v2 ledger noted the C2 close was not yet *operationally*
complete because C3 stranded the deploy that `awaiting_deploy` points to — fixing
C3 in v3 is what makes the C2 remediation actually satisfiable; that is a reason to
preserve C2, not change it.)

### Other carry-forward (v1 ledger §5 / v2 §0.1 — do NOT re-litigate)

- **Q4 — resolved.** Plain verb now (`striatum daemon deploy`), bootstrapping
  paradox dispositive against a run-shape base deployer, run-shape door held open
  by three seams (content-addressed `plan_hash`, hash-chained receipt in
  `audit_log`, a shared `Deployer.Apply` engine). Neither falsifier contested it.
- **Q3 mid-step resumability — sound.** Q3-A (cursor advance in the step
  transaction), Q3-B (`in_progress` pre-marker + invalid-index reconciler), the
  §1.1 step taxonomy, and the `deploy_cursor`-as-second-signal insight. N1 is about
  the per-step **receipt**, not the per-step schema/cursor body — preserve the
  body, extend it with the receipt rule.
- **Decoupling shape — sound.** Shadow-first default-OFF `STRIATUM_DEPLOY_DECOUPLED`,
  `ConnectAndVerify` preserving the P2 watermark interlock + P3 drift gate /
  self-record, the embed-FS-derived plan, the hash-chained receipt mechanism, and
  `doctor schema_deploy_unrecorded`.
- **Scope discipline held.** P5 (rehearse / expand-contract / fidelity tiering /
  clone) deferred; full capability revocation named as a follow-up beyond P4, not
  assumed; the local-first boundary respected.

## Root reframe (do not lose this)

**Schema mutation must stop being an implicit side effect of the serving
process's restart and become an explicit, ordered, resumable, provenance-tracked
operation owned by a dedicated deployer.** Then the serving daemon can hold zero
DDL privilege, "restart force-commits a half-applied deploy" becomes impossible,
and a bad migration can never wedge the single writer on boot.

## The two Open Questions P4 must pin (from the RFC)

- **Q3 — How atomic is "atomic"?** Confirm the **per-step-atomic + resumable-
  cursor** contract is sufficient for every owner+runtime interleaving we actually
  ship, or specify the small set of steps that need a stricter
  single-connection/single-transaction sub-protocol. Every step must be idempotent
  and leave a coherent intermediate the fingerprint classifies as "incomplete,
  resume" — not "unknown drift, panic". **In v3 this explicitly includes the
  per-step receipt (N1).** *(RFC: "This is the hard correctness core of P4.")*
- **Q4 — Should a deploy be a Striatum run?** Plain verb vs. a dogfooded run shape
  (`expand_rehearsal` → `contract_swap`), with the bootstrapping paradox (a run
  needs a schema to run the deploy that changes the schema). Resolved in v1/v2 —
  carry forward.

## Load-bearing risks (attack these)

- **R1 atomicity-is-partly-a-lie:** non-transactional DDL (`CREATE INDEX
  CONCURRENTLY`, `ALTER TYPE … ADD VALUE`), non-idempotent steps, or a two-
  connection (owner+runtime) crash window that the fingerprint reads as "unknown
  drift, panic". **Now includes the per-step receipt (N1).** Test: kill-and-resume
  across each step class **and across the step-commit/receipt-write boundary**.
- **R2 decoupling regresses a landed gate:** lifting `ApplyMigrations` out of
  `ConnectAndMigrate` must NOT break the P2 watermark interlock, the P3 drift gate
  / fingerprint self-record, or fresh-DB bring-up; no window where the daemon
  serves on an unmigrated schema.
- **R3 DDL-revocation lockout:** revoking serving-role DDL (owner bundle ≥ 0020)
  must not lock out the runtime path before the deployer exists, nor recreate a
  #512-class lockout. **C3 is exactly this risk surfacing as the 0020-vs-ownership
  contradiction** — the chosen mechanism must let the deploy complete.
- **R4 cursor holes:** double-apply/skip at a commit boundary; **receipt written
  out of step with the real schema (N1)**; out-of-order apply under the plan's
  edges.

## Anchor verification against current `main` (operator pre-flight)

Verified against `~/git/striatum` @ `main`. P0–P3 + P2 are **landed**; the P4
surfaces are **NOT-FOUND (to be built)**. Treat as ground truth; re-anchor the
spec to these file:line references. (Carried from v2 — re-confirm; the C3 anchors
are new for v3.)

| Claim / target | Status | Anchor (current source) |
| --- | --- | --- |
| Boot-time auto-apply runs runtime migrations as `striatumd_rw` | **ACCURATE (the coupling P4 removes)** | `go/pkg/db/connection.go:353` `ApplyMigrations(ctx, pool.Runner, daemonVersion)`; called from boot via `db.BootstrapAndConnect()` at `go/cmd/striatumd/main.go:193,199`. |
| Runtime migration frontier = 0043 (P3 added `schema_state`) | **ACCURATE** | `go/pkg/db/migrations.go:17` `LatestDaemonDBVersion = 43`; `:74` migration 43 = "schema-fingerprint drift gate state (RFC 0142 P3 / #570)". New P4 migration is **≥ 0044**. |
| Owner bundle frontier = 0019 | **ACCURATE** | `go/pkg/db/owner.go:23` `LatestOwnerBundleVersion = 19`. New P4 owner bundle (DDL revoke) is **≥ 0020**. |
| P3 fingerprint machinery (P4 builds on it) | **ACCURATE (landed)** | `go/pkg/db/schema_drift.go`: `ExpectedFingerprint()` `:83-100`, `LiveFingerprint()` `:145-161`, `CheckSchemaDrift()` `:254-274`, env `STRIATUM_SCHEMA_DRIFT_REFUSE` `:28`; doctor block `go/pkg/reads/doctor_schema_drift.go:26-77`; boot sequence `connection.go:376-399`. |
| `schema_state` table records last-applied fingerprint | **ACCURATE (landed)** | `go/pkg/db/sql/0043_schema_state.sql:39-44` (singleton CHECK + GRANT block); UPSERT `schema_drift.go:187-194`. |
| P2 watermark interlock + clean halt | **ACCURATE (landed, #574)** | `go/pkg/db/owner.go:37-64` `ErrAwaitingOwnerDDL`/`AwaitingOwnerDDLError`, `:91-110` `CheckOwnerBundleWatermark`; called **before** `ApplyMigrations` at `connection.go:349-352`; clean exit `cmd/striatumd/main.go:208-214`. |
| Two-role boundary build guard | **ACCURATE (landed)** | `go/pkg/db/migrations_test.go:229-259` `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`; load-time `preflightRuntimeMigrationOwnership()` `migrations.go:158`. |
| `owner-ddl apply` applies bundles out-of-band as owner role | **ACCURATE (landed)** | `go/pkg/cli/localcommands/daemon.go:90-159` `runDaemonOwnerDDL`; calls `db.ApplyOwnerBundles()`. P4's `deploy` lives alongside this. |
| Two roles: owner/bootstrap vs runtime `striatumd_rw`; runtime has NO DDL on owner tables; owner-only `append_audit_row` | **ACCURATE** | `go/pkg/db/authority_bootstrap.go:31-46`; `go/pkg/db/sql/owner/0001_authority_phase0.sql:229-238` (revokes direct `audit_log` INSERT, grants only the SD function `append_audit_row` `:223,238`). N1's per-step receipt is appended over the OWNER connection via this SD function. |
| **C3 — the CREATE-prerequisite for `ALTER … OWNER TO striatumd_rw`** | **ACCURATE (the contradiction)** | `go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:58-72` (prereq #2: striatumd_rw MUST hold CREATE on schema striatumd, else `permission denied`), `:97-102` (`GRANT CREATE ON SCHEMA striatumd TO striatumd_rw` before the transfer loop); `go/pkg/db/sql/owner/0019_supervisor_pointer_runtime_ownership.sql:53-80` (same prereq + grant). P4 bundle 0020 `REVOKE`s precisely this — **the contradiction C3 must resolve.** |
| `striatum daemon deploy` / `deploy_cursor` / deploy plan / deploy receipt | **NOT-FOUND (P4 builds these)** | `daemon.go:62-82` subcommands = `install,uninstall,status,migrate-db,owner-ddl` — no `deploy`. No `deploy_cursor` table, no plan/receipt machinery anywhere. |

**Net design implication.** The P4 ground is clean: P2 (watermark interlock) and
P3 (drift gate + fingerprint self-record) are landed and are exactly the contract
P4 leans on; C1 and the C2 activation edge are resolved in v2. The remaining hard
parts are **C3** — pin a single 0020-compatible ownership mechanism that lets the
deploy complete and still denies `striatumd_rw` CREATE in steady state — and
**N1** — extend the same atomic-or-idempotent provenance discipline the C1
finalizer uses to the per-step receipt, keyed on `(plan_hash, step_index)`.
Falsifiers must press the C3 mechanism against the repo's own 0018/0019
prerequisite, and press the per-step receipt across the step-commit/receipt-write
crash boundary. Be shadow-first: serve-boot auto-apply stays the default until the
deployer is proven, then flips behind a flag.

---
<sub>Operator scaffold for the RFC 0142 P4 falsification-gate design run (v3 /
REVISION of `rfc-0142-p4-design-v2`; resolves the cycle-2 findings C3 + N1 and
carries C1/C2 forward). Lanes: author=claude (holder/adjudicator/committer),
reviewer=codex (falsifiers).</sub>

## Operator sharpenings (pin these — do NOT let cycle-3 re-derive them)

- **C3: prefer resolution (a) — sequence the revoke last.** Of the three options,
  **(a)** is the simplest and introduces **no transient elevated grant** (so no
  serving-window-visibility question to prove away, unlike (b)) and **no §4.1
  safety-claim retraction** (unlike (c)). The whole point: while the runtime
  ownership reconciles run, `striatumd_rw` still holds `CREATE` (0020 not yet
  applied), so every `ALTER … OWNER TO striatumd_rw` succeeds; 0020 is the **last**
  committed step, so the steady state still denies CREATE. The spec must make the
  plan generator special-case the DDL-revoke bundle 0020 to sort to the END
  (after the last runtime ownership reconcile) rather than into the owner-prefix,
  and keep the `deploy_cursor` indices, the C2 `CheckDeployActivation` predicate,
  the §1.3 in-sync classification, and the `plan_hash`/fingerprint binding coherent
  with 0020 as the terminal step. If the holder picks (b) or (c) instead, it must
  fully discharge that option's extra obligation (the no-serving-during-grant proof
  for (b), or the §4.1 correction + per-migration DML-grant guard for (c)).
- **N1: same two-connection constraint as C1 — use an idempotent reconcile, not a
  cross-connection transaction.** The per-step receipt is appended over the OWNER
  connection (`append_audit_row`, owner-only) — for a **transactional** step the
  deployer applies the step over that **same** owner connection, so the receipt
  append CAN and MUST share the step transaction (Q3-A: `step_committed(k)` iff
  receipt durable). For a **non-transactional** step the receipt cannot be atomic
  with the side effect, so the `in_progress(k)` reconciler must idempotently append
  exactly one receipt keyed on `(plan_hash, step_index)` before writing
  `step_committed(k)`. Do **not** propose a single transaction spanning a
  cross-connection write for the NT-DDL case. The key `(plan_hash, step_index)`
  must be stable across re-runs (step_index is the plan position; the plan is
  content-addressed by `plan_hash`).
- **Keep C1 and N1 coherent.** Both are the atomic-or-idempotent provenance
  discipline; the §1.3 classification table, the `finalizing` finalizer, and the
  new per-step receipt rule must all hold together — a resume must never serve, and
  must never advance past a step missing its receipt, and the finalizer's terminal
  `complete` receipt must remain exactly-once.
