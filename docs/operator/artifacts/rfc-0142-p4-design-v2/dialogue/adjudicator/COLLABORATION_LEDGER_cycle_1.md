---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0142 P4 one-shot deployer (decouple schema-apply from serve-boot) — falsifiable implementation spec (v2 REVISION)"
participants:
  - "holder-author-002"
  - "falsifier-reviewer-001"
  - "falsifier-reviewer-002"
  - "adjudicator-author-001"
cycle: 1
entries:
  - kind: claim
    by: "holder-author-002"
    refs: ["dialogue:1"]
    text: "Revised P4 spec (v2): resolves all three design-v1 findings per the prescribed fixes (§0.1) — C1 by a distinct `finalizing` cursor state + an idempotent finalizer that writes the receipt, then the fingerprint, then `complete` last, plus the new §1.3 resumable-finalization row and F10; C2 by a `CheckDeployActivation` interlock that fails closed before `ApplyMigrations` (absent `deploy_cursor` = incomplete), a forward-watermark rule, `RequiredOwnerBundleVersion` kept at 19, and F11; C3 by Policy 1 (runtime objects stay `striatumd_rw`-owned) via a same-step catalog-diff `ALTER … OWNER TO striatumd_rw` reconciliation + grant re-assert and F12 — without regressing the carry-forward items (Q4, the Q3-A/Q3-B per-step body, the shadow-first decoupling shape)."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "Per-step receipt is not crash-safe (new, Q3/R4 core). The spec makes the per-step deploy receipt load-bearing (claim RECEIPT §3.4; F7; RFC 0142 Layer 3 = first-class provenance), but the Q3 step protocols make only the schema side effect and the cursor marker atomic; the §3.3b transaction recipe (BEGIN → DDL → ownership reconcile → grants → cursor advance → COMMIT) omits the per-step receipt, and §3.4's only idempotency guard is for the terminal `(plan_hash, state=complete)` receipt. So a transactional step k can commit (cursor `step_committed(k)`) and crash before its receipt; resume advances to k+1 and never backfills it, and `doctor schema_deploy_unrecorded` (which checks only the terminal receipt vs `schema_state`) can be green while step k's receipt is permanently absent. The NT-DDL (Q3-B) case is worse: the receipt cannot be atomic with the side effect, and a crash after the receipt append but before `step_committed(k)` is undefined (duplicate receipt vs silent skip — no step-level idempotency key). The per-step-atomic + resumable-cursor contract is insufficient for the receipt and no sub-protocol is specified; the receipt chain can disagree with the real schema history."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "C3 ownership transfer is self-contradictory under bundle 0020 (reopens C3; also breaks the C2 activation deploy). Policy 1's mechanism (§3.3b) creates each runtime object as the owner/admin role and then runs `ALTER <kind> striatumd.<name> OWNER TO striatumd_rw` in the same step transaction. PostgreSQL requires the NEW owner to hold `CREATE ON SCHEMA striatumd` for that transfer — the repo's own bundles document it: `0018_runtime_table_ownership_transfer.sql:58-72` (`ALTER … OWNER TO striatumd_rw fails permission denied for schema striatumd` without it) and `:97-102` / `0019_supervisor_pointer_runtime_ownership.sql:53-80` GRANT CREATE FIRST for exactly this reason. P4 bundle 0020 REVOKEs that very privilege (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`), and the plan applies owner bundles before runtime steps (§3.2, §4.3 step 2), so once 0020 commits the subsequent `ALTER … OWNER TO striatumd_rw` fails `permission denied for schema striatumd` in the documented non-superuser two-role topology (owner is merely a member of `striatumd_rw`, which post-0020 lacks CREATE). The deploy strands itself after 0020 at the first runtime ownership reconcile; a later boot halts `awaiting_deploy` whose only remediation (`striatum daemon deploy`) has a self-inflicted privilege contradiction — the same practical lockout class C2 exists to close. F12 is internally inconsistent (it requires both `relowner == striatumd_rw` and post-0020 `striatumd_rw CREATE → 42501`). The spec never names a temporary grant, a superuser precondition, or a test proving the post-commit state still denies CREATE."
    correspondence: landed_unrebutted
verdict: "needs_revision"
rationale: "REVISION cycle (the only one this gate allows). The v1 terminal-finalization finding C1 is genuinely resolved (Option B `finalizing` state + idempotent finalizer + the new §1.3 resumable-finalization row + F10; both falsifiers concede the v1 C1 shape is closed) and C2's fail-closed activation EDGE is genuinely resolved (CheckDeployActivation before ApplyMigrations, absent-cursor-as-incomplete, the awaiting_deploy/awaiting_deploy_config typed halts, the forward-watermark rule, Required kept at 19 resolving the auto-apply contradiction, F11). But the gate does NOT clear: (1) C3 is NOT genuinely resolved — falsifier_2 shows, against the repo's own owner bundles, that the chosen Policy-1 ownership transfer (`ALTER … OWNER TO striatumd_rw`) needs the `CREATE ON SCHEMA striatumd` privilege that this very phase's bundle 0020 revokes, and the plan applies 0020 before the runtime ownership-reconcile step, so the C3 mechanism is self-contradictory as written and strands the activation deploy (which also undercuts C2's bootstrap-order remediation). C3's prescribed fix is therefore claimed, not implementable. (2) A NEW material Q3/R4 challenge stands — falsifier_1 shows the load-bearing per-step deploy receipt (claim RECEIPT, F7, RFC 0142 Layer 3) is not made atomic/idempotent with the step the way the cursor markers and the terminal receipt are: the §3.3b recipe omits it, the only idempotency guard is terminal, so a step can durably apply with its receipt missing (transactional) or duplicated/ambiguous (NT-DDL), with no per-step receipt sub-protocol specified — the per-step-atomic contract insufficient for a shipped interleaving, which per the role rubric alone forces needs_revision. Both challenges are material and stand unrebutted by the spec as written. Carry-forward items (Q4, Q3-A/Q3-B mid-step resumability, the shadow-first decoupling shape, C1) remain sound; the revision (if re-scaffolded) must fix exactly C3's 0020-vs-transfer contradiction and the per-step receipt protocol."
findings:
  - id: C1
    severity: critical
    posture: design
    status: answered
    challenge: "RESOLVED. The v1 finalization-boundary gap (cursor `complete` written before the receipt + fingerprint, leaving an unclassifiable state) is closed by the prescribed Option B + idempotent finalizer: §1.2 enters a distinct `finalizing` state in one commit after the last step, then the idempotent finalizer appends the `complete` receipt (skip if `(plan_hash, state=complete)` exists), UPSERTs `schema_state`, and advances `finalizing → complete` LAST; §1.3 adds the `finalizing`/expected-plan_hash/(any fingerprint) = resumable-finalization row (never serve, never genuine-drift); the §3.4 crash-point table and F10 (`T-deploy-resume-finalization-crash`) cover kills after {final DDL commit, finalizing-before-receipt, receipt-before-schema_state, schema_state-before-complete}. Because `complete` is the final write, the v1 unclassifiable state cannot recur. The operator pin (no single finalization transaction across the owner+runtime connections) is honored. Both falsifiers concede the v1 C1 shape is closed. NOTE: the per-step (non-terminal) receipt atomicity gap is a distinct NEW finding (see N1), not C1 reopened."
    affected_invariants: ["Q3 per-step-atomic + resumable-cursor contract", "1.3 incomplete-vs-drift disambiguation table"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: C2
    severity: high
    posture: design
    status: answered
    challenge: "RESOLVED at the fail-closed activation EDGE. The v1 gap (forward-watermark tolerance drives boot into ApplyMigrations under a revoked CREATE → raw 42501 crash-loop) is closed: §3.3a adds `CheckDeployActivation` called immediately after `CheckOwnerBundleWatermark` and BEFORE `ApplyMigrations` in both ConnectAndMigrate and ConnectAndVerify, mutating nothing; a revoke-embedding binary with the flag OFF halts `awaiting_deploy_config`, with the flag ON and the deploy incomplete (including `deploy_cursor` absent — the operator bootstrap-order sharpening, treated as incomplete not error) halts `awaiting_deploy`; §4.2 adds the forward-watermark rule so a revoke-unaware binary observing applied >= 20 halts `awaiting_deploy` instead of auto-applying; `RequiredOwnerBundleVersion` stays 19, dissolving the auto-apply-default vs Required=20 contradiction; the new typed halts get `main.go` errors.As arms onto the clean non-restartable exit; F11 (`T-deploy-revoke-activation-ordering`) asserts ApplyMigrations is never entered and the DB is byte-identical across the bad interleavings. CAVEAT (not a defect in the halt logic): the activation choreography's COMPLETION depends on `striatum daemon deploy` being able to finish the 0020+runtime plan, which finding C3 shows it cannot as written — so the `awaiting_deploy` remediation is correct but currently unsatisfiable. That defect is recorded under C3/N2, not here."
    affected_invariants: ["R3 no DDL-revocation lockout", "serve-boot clean-halt (no crash-loop)"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: C3
    severity: high
    posture: design
    status: open
    challenge: "OPEN — reopened by falsifier_2. The chosen Policy 1 (runtime objects stay `striatumd_rw`-owned) is specified via a same-step `ALTER <kind> striatumd.<name> OWNER TO striatumd_rw` reconciliation (§3.3b), but PostgreSQL requires the NEW owner to hold CREATE on the object's schema for that transfer, and this phase's owner bundle 0020 REVOKEs `CREATE ON SCHEMA striatumd FROM striatumd_rw`. The plan applies owner bundles before runtime steps (§3.2, §4.3 step 2), so once 0020 commits the reconcile `ALTER … OWNER TO striatumd_rw` fails `permission denied for schema striatumd` in the documented non-superuser two-role topology — confirmed by the repo's own bundles 0018/0019, which document the prerequisite and GRANT CREATE FIRST (`0018_runtime_table_ownership_transfer.sql:58-72,97-102`; `0019_supervisor_pointer_runtime_ownership.sql:53-80`). The owner's membership in `striatumd_rw` does not help (post-0020 the parent role itself lacks CREATE). Consequence: the activation deploy strands after 0020 at the first runtime ownership reconcile; F12 is internally inconsistent (requires both `relowner == striatumd_rw` AND post-0020 `striatumd_rw CREATE → 42501`). REQUIRED FIX: pick one concrete, 0020-compatible mechanism and test it in a non-superuser two-role cluster — (a) keep Policy 1 with an owner-connection same-transaction temporary `GRANT CREATE … TO striatumd_rw` around each `ALTER OWNER`, `REVOKE CREATE` before commit, F12 asserting the committed post-state still denies `striatumd_rw` CREATE and no boot serves while the grant exists; or (b) switch to Policy 2 (owner/admin owns new runtime objects; every runtime migration carries exact DML grants) and correct §4.1 to stop claiming new runtime objects stay rw-owned; or (c) reorder so the `deploy_cursor` migration + any ownership-transfer prerequisite apply before 0020, and still define the post-0020 future-object rule via (a) or (b)."
    affected_invariants: ["two-role owner/runtime boundary", "section 4.1 residual-capability argument", "R3 activation deploy can complete"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: N1
    severity: high
    posture: design
    status: open
    challenge: "OPEN — new material challenge (falsifier_1; Q3/R4 correctness core). The per-step deploy receipt is load-bearing (claim RECEIPT §3.4; F7; RFC 0142 Layer 3 makes every schema change first-class adjudicated provenance; §2 holds the Q4 run-shape door open via the hash-chained receipt seam), but it is not made crash-safe the way the cursor markers and the terminal `complete` receipt are. The §3.3b step transaction omits the receipt; §3.4's only idempotency guard is the terminal `(plan_hash, state=complete)` receipt. Transactional case: step k commits (`step_committed(k)`) and crashes before its receipt; resume reads `step_committed(k)`, advances to k+1, never backfills the receipt, and `doctor schema_deploy_unrecorded` (terminal-only check) can be green while step k's receipt is permanently absent. NT-DDL (Q3-B) case: the receipt cannot be atomic with the side effect, and a crash after the receipt append but before `step_committed(k)` is undefined — duplicate receipt vs silent skip, with no per-step idempotency key. So the per-step-atomic + resumable-cursor contract is insufficient for the receipt and no sub-protocol is specified. REQUIRED FIX: (1) Q3-A — state the step receipt append occurs in the SAME owner-connection transaction as the DDL + ownership reconcile + grants + version stamp + cursor advance (legal because the deployer applies runtime steps over the owner connection and `append_audit_row` runs in the caller's transaction); (2) Q3-B — add a step-level idempotent receipt rule keyed on `(plan_hash, step_id, sha256, state=step_committed)` so the `in_progress(k)` reconciler appends exactly one receipt before writing `step_committed(k)`; (3) add `T-deploy-step-receipt-crash` (and extend F8/F7) asserting, after each crash point, final schema equality, exactly one receipt per applied step, one terminal `complete` receipt, and a green doctor."
    affected_invariants: ["RFC 0142 Layer 3 deploy-receipt provenance (one chained receipt per schema change)", "R4 receipt-written-out-of-step / no double-apply", "Q4 hash-chained receipt seam"]
    source_refs: ["dialogue:1", "dialogue:2"]
branches:
  design: blocked
---

# COLLABORATION LEDGER — RFC 0142 P4 design run (v2 / REVISION), cycle 1

author: adjudicator-author-001

**Verdict:** `needs_revision`

> Scope of this adjudication: the curated dialogue trajectory only — the **revised**
> Holder's `HOLDER.md` P4 implementation spec and the two falsifiers' `FALSIFIER.md`
> challenges — plus the `SEED.md` charter, with the v1 `HOLDER.md` and the v1
> collaboration ledger as context for what the revision had to fix. RFC 0142 is
> accepted (D258); this judges the **P4 implementation shape**, not the five-layer
> design. A clearing verdict would be `accept` or `accept_with_findings`; this is
> neither. This is the gate's single allowed revision cycle, so this verdict is
> terminal for the gate — judged exactly, not graded on the consequence.

## 1. Decision

`needs_revision`. The revision is a real, well-anchored piece of work: it **genuinely
resolves C1** (the v1 finalization boundary) and **the fail-closed activation edge of
C2**, and it preserves the carry-forward items the v1 ledger judged sound. But a
clearing verdict requires **all three v1 findings genuinely resolved AND no new
material challenge standing**, and two things break that:

- **C3 is not genuinely resolved (falsifier_2 — reopens C3).** The chosen Policy-1
  ownership transfer (`ALTER … OWNER TO striatumd_rw`, §3.3b) needs the
  `CREATE ON SCHEMA striatumd` privilege that **this phase's own bundle 0020 revokes**,
  and the plan applies 0020 **before** the runtime ownership-reconcile step. The
  prescribed fix is therefore *claimed, not implementable* — exactly the "fix is only
  claimed, not actually implemented as a chosen-and-tested ownership policy" failure
  the role rubric calls out. It also strands the C2 activation deploy.
- **A new material Q3/R4 challenge stands (falsifier_1).** The **per-step** deploy
  receipt — which the spec makes load-bearing (claim RECEIPT, F7, RFC 0142 Layer 3) —
  is not made atomic/idempotent with the step the way the cursor markers and the
  *terminal* receipt are. A step can become durably applied with its receipt missing
  (transactional) or duplicated/ambiguous (NT-DDL), and no per-step receipt
  sub-protocol is specified. Per the role rubric, a per-step interleaving where the
  per-step-atomic + resumable-cursor contract is insufficient with no stricter
  sub-protocol **alone forces `needs_revision`.**

Both challenges are material and stand unrebutted by the spec as written (this cycle
carries no Holder rebuttal artifact; the spec text stands). The revision must fix
exactly the two items in §4.

## 2. Per-finding / per-challenge ledger

| # | Source | What is judged | Material? | Resolved / rebutted? | Disposition |
| --- | --- | --- | --- | --- | --- |
| **C1** | v1 finding / falsifier_1 revision-check | finalization boundary closed by `finalizing` state + idempotent finalizer + §1.3 row + F10 (`HOLDER.md:47,106-178,376-408,560`) | — | **Yes — genuinely resolved**; v1 unclassifiable state cannot recur; both falsifiers concede the v1 C1 shape closed | **RESOLVED** |
| **C2** | v1 finding / both falsifiers' revision-check | activation fails closed before `ApplyMigrations`: `CheckDeployActivation`, typed halts, forward-watermark rule, Required=19, F11 (`HOLDER.md:48,285-328,445-484,561`) | — | **Yes — fail-closed EDGE resolved**; but choreography completion depends on C3 (see caveat) | **RESOLVED (edge); completion blocked by C3** |
| **C3** | v1 finding / falsifier_2 (`dialogue:3`) | Policy-1 `ALTER … OWNER TO striatumd_rw` reconcile after 0020 revokes the prerequisite `CREATE ON SCHEMA` (`HOLDER.md:339-374,428-431`; refuted by `0018:58-72,97-102`, `0019:53-80`) | **Yes** — self-contradictory mechanism; strands the activation deploy; F12 internally inconsistent | **No** — spec names no temporary grant / superuser precondition / proof | **OPEN → needs_revision** |
| **N1** | falsifier_1 (`dialogue:2`) | per-step receipt not atomic/idempotent with the step; §3.3b omits it; only terminal idempotency guard (`HOLDER.md:347-374,378-408,557-558`) | **Yes** — Q3/R4 core; load-bearing provenance can be missing/duplicated across a crash | **No** — no per-step receipt sub-protocol specified | **STANDS → needs_revision** |

## 3. Why the verdict turns the way it does

### C1 — finalization boundary: RESOLVED

The v1 defect was a crash after `deploy_cursor.state = complete` (with the expected
`plan_hash`) but before the receipt + `schema_state` fingerprint were durable — a state
no §1.3 row classified, hitting even a purely transactional plan. The revision adopts
the prescribed Option B + idempotent finalizer (`HOLDER.md:47,140-155`): a distinct
`finalizing` state is entered in one commit after the last step; the idempotent
finalizer then appends the receipt (guarded skip on `(plan_hash, state=complete)`),
UPSERTs the fingerprint, and advances `finalizing → complete` **last**. The new §1.3
row (`HOLDER.md:169`) classifies `finalizing`/expected-`plan_hash`/(any fingerprint) as
**resumable finalization — never serve, never genuine-drift**, and the in-sync row is
tightened to require `state == complete`. Because `complete` is the final write,
reaching it proves the receipt and fingerprint are already durable, so the v1
unclassifiable state cannot recur; an interrupted finalization is pinned at
`finalizing` and repaired exactly once. The §3.4 crash-point table and F10
(`T-deploy-resume-finalization-crash`, `HOLDER.md:560`) exercise the four kill points
the ledger demanded, and the operator pin (no single finalization transaction spanning
the owner+runtime connections) is explicitly honored (`HOLDER.md:387-393`). Both
falsifiers independently confirm the v1 C1 shape is closed. **C1 is genuinely
resolved.** (The distinct *per-step* receipt gap is N1, not C1 reopened — C1 was the
*terminal* `complete`-before-receipt window, which is now closed by construction.)

### C2 — activation fail-closed edge: RESOLVED; choreography completion blocked by C3

The v1 defect was that the owner watermark tolerates a forward bundle, so a DB at 0020
with the flag OFF/absent reached `ApplyMigrations` over the runtime runner under a
revoked `CREATE`, producing a raw `42501` that `main.go` did not map to a typed halt
(the #512-class crash-loop). The revision closes that edge concretely
(`HOLDER.md:285-328,445-484`): `CheckDeployActivation` runs immediately after
`CheckOwnerBundleWatermark` and **before** `ApplyMigrations` in both the migrate and
verify boot paths, mutating nothing; a revoke-embedding binary halts
`awaiting_deploy_config` (flag OFF) or `awaiting_deploy` (flag ON + deploy incomplete,
**including `deploy_cursor` absent**, the operator bootstrap-order sharpening, treated
as incomplete rather than error-on-missing-table); the forward-watermark rule halts a
revoke-unaware binary that observes `applied >= 20` instead of letting it auto-apply;
`RequiredOwnerBundleVersion` stays 19, which dissolves the v1 auto-apply-default vs
`Required = 20` contradiction; the typed halts get `main.go` `errors.As` arms onto the
clean non-restartable exit; F11 asserts `ApplyMigrations` is never entered and the DB
is byte-identical across the bad interleavings. The fail-closed **edge** is genuinely
resolved.

The honest caveat: a clean `awaiting_deploy` halt is only a real close of the R3
lockout if its remediation — `striatum daemon deploy` — can actually finish the
0020+runtime activation plan. Finding **C3** shows it cannot as written: the deploy
strands itself after 0020 on the first runtime ownership reconcile. So the
crash-loop is gone, but the operator is left at a clean halt whose only exit is a
command with a self-inflicted privilege contradiction — the same practical lockout
*class*, relocated. That defect is C3's, not a flaw in the halt logic, and is recorded
there; but it means the C2 close is not yet *operationally* complete either.

### C3 — runtime-object ownership under owner/admin apply: OPEN (reopened by falsifier_2)

The prescribed fix required choosing **one** ownership policy and **testing** it. The
revision chooses Policy 1 (runtime objects stay `striatumd_rw`-owned) and specifies a
same-step catalog-diff reconciliation that, for each newly-created object, runs
`ALTER <kind> striatumd.<name> OWNER TO striatumd_rw` and re-asserts the migration's
DML grants (`HOLDER.md:339-374`). Falsifier_2 refutes the mechanism against current
source: PostgreSQL requires the **new** owner to hold `CREATE ON SCHEMA striatumd` for
`ALTER … OWNER TO striatumd_rw` to succeed, and the repo's own bundles document exactly
this and **grant `CREATE` first** for that reason —
`0018_runtime_table_ownership_transfer.sql:58-72` ("striatumd_rw MUST hold CREATE on
schema striatumd … otherwise `ALTER … OWNER TO striatumd_rw` fails `permission denied
for schema striatumd`") and `:97-102` (`GRANT CREATE ON SCHEMA striatumd TO
striatumd_rw` before the transfer loop); `0019_supervisor_pointer_runtime_ownership.sql:53-80`
repeats the prerequisite and grant. **This phase's bundle 0020 revokes precisely that
privilege** (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`, `HOLDER.md:430`),
and the plan applies owner bundles before runtime steps (§3.2 `HOLDER.md:241-247`; §4.3
step 2 `HOLDER.md:504-507`). So once 0020 commits, the reconcile `ALTER … OWNER TO
striatumd_rw` fails `permission denied for schema striatumd` in the documented
non-superuser two-role topology — the owner's *membership* in `striatumd_rw` does not
help, because post-0020 the parent role itself lacks `CREATE`.

I verified the falsifier's anchors directly against the worktree source: bundles 0018
and 0019 both carry the explicit "Prerequisite #2 … PostgreSQL requires the NEW owner …
to be able to create objects in the table's schema" comment and `GRANT CREATE ON SCHEMA
striatumd TO striatumd_rw` immediately before their `ALTER … OWNER TO` loops. The
refutation is concrete and source-grounded, not hypothetical.

This is material on two counts: (a) it strands the activation deploy after 0020,
before the runtime frontier completes, turning a catalog-ownership question into an
activation failure / lockout class; (b) F12 as written is internally inconsistent — it
requires both `relowner == striatumd_rw` for every created object **and** post-0020
`SET ROLE striatumd_rw; CREATE TABLE → 42501`, which cannot both hold without an
unspecified temporary-grant or superuser mechanism. The Holder's own strongest
rebuttal (a temporary `GRANT CREATE … / REVOKE CREATE` around the transfer, or a
superuser owner DSN) is **not the spec as written** — §3.3b names no temporary grant,
no superuser precondition, and no test proving the post-commit state still denies
`striatumd_rw` CREATE or that no boot serves while the grant exists. So C3's
chosen-and-tested ownership policy is chosen but its concrete mechanism is
self-contradictory under the very revoke P4 ships. **C3 remains open.**

### N1 — per-step receipt is not crash-safe (new; Q3/R4 core): STANDS

Falsifier_1 concedes C1/C2/C3-at-the-design-level and lands a distinct new gap. The
spec makes the **per-step** deploy receipt load-bearing — claim RECEIPT ("every applied
step writes a hash-chained deploy receipt," `HOLDER.md:378-385`), F7 ("every applied
step writes a hash-chained deploy receipt; a schema change without one is surfaced by
`doctor schema_deploy_unrecorded`," `HOLDER.md:557`), RFC 0142 Layer 3 (the deploy
receipt is how every schema change becomes first-class adjudicated provenance), and §2
holds the Q4 run-shape door open citing the hash-chained receipt as a seam. But the Q3
step protocols make only the **schema side effect and the cursor marker** atomic, and
the §3.3b step-transaction recipe (BEGIN → DDL → ownership reconcile → grants → cursor
advance → COMMIT, `HOLDER.md:352-369`) **omits the receipt entirely**; §3.4's only
idempotency guard is for the *terminal* `(plan_hash, state=complete)` receipt
(`HOLDER.md:403-408`).

Two concrete interleavings:

- **Transactional step k.** `applyRuntimeStep` commits DDL + ownership + grants +
  `step_committed(k)`, then the process dies **before** the per-step receipt is
  appended. On resume the cursor reads `step_committed(k)` → §1.3 classifies "incomplete,
  resume" → §1.2 advances to k+1. Nothing backfills the missing receipt, and
  `doctor schema_deploy_unrecorded` — which warns only when `schema_state.fingerprint`
  advanced but the *terminal* `complete` receipt is absent — can be **green** while step
  k's receipt is permanently missing. The deploy reaches `complete` with a receipt chain
  that does not faithfully record the real schema history.
- **NT-DDL step (Q3-B).** The receipt cannot be atomic with `CREATE INDEX CONCURRENTLY`
  / `ALTER TYPE … ADD VALUE`. A crash **after** the receipt append but **before**
  `step_committed(k)` is undefined: on resume reading `in_progress(k)`, the repair path
  either re-appends (duplicate receipt) or must silently infer the side effect is already
  attested — and the spec specifies **no step-level idempotency key** to disambiguate.

This is the same discipline the v1 ledger and the operator pins demanded at the
finalization boundary — atomic-or-idempotent provenance writes — applied to the
*terminal* receipt but **not extended** to the per-step receipts the spec equally makes
load-bearing. F8 asserts only final schema equality (not receipt-chain completeness per
step), and F10 is terminal-finalization-only, so no named test closes it. Per the role
rubric, a per-step interleaving where the per-step-atomic + resumable-cursor contract is
insufficient and no stricter sub-protocol is specified is the Q3 correctness core and
**alone forces `needs_revision`.** The repair is bounded and the Holder's own primitives
make it easy (append the receipt inside the owner-connection step transaction for Q3-A;
a keyed idempotent step-receipt finalizer for Q3-B) — but it is a genuine spec change,
not present as written. **N1 stands.**

## 4. What the revision must fix (exact)

A future revision (if the operator re-scaffolds one) clears the gate only if it pins
both, while preserving C1, the C2 edge, and the carry-forward items.

1. **Make the C3 ownership transfer compatible with 0020 (and fix F12).** Pick one
   concrete, tested mechanism:
   - *Keep Policy 1:* in the owner-connection step transaction, wrap each
     `ALTER … OWNER TO striatumd_rw` in a temporary `GRANT CREATE ON SCHEMA striatumd TO
     striatumd_rw` … `REVOKE CREATE` before `COMMIT` (mirroring how 0018/0019 grant
     first), and extend F12/`T-deploy-runtime-object-ownership` to run in a **documented
     non-superuser two-role cluster**, asserting the transfer succeeds, the committed
     post-state denies `striatumd_rw` `CREATE`, and no boot can serve while the temporary
     grant is uncommitted; or
   - *Switch to Policy 2:* owner/admin owns new runtime objects, every runtime migration
     carries the exact DML grants the daemon needs (build/load-guarded), and §4.1 is
     corrected to stop claiming new runtime objects stay `striatumd_rw`-owned; or
   - *Reorder activation:* apply the `deploy_cursor` migration and any ownership-transfer
     prerequisite **before** 0020, and still define the post-0020 future-object rule via
     one of the above.
   Either way, resolve the F12 internal inconsistency (owned-by-`striatumd_rw` vs
   post-0020 `striatumd_rw CREATE → 42501`).

2. **Specify a crash-safe per-step receipt protocol (N1 — Q3/R4 core).**
   - *Q3-A:* state that the step receipt append occurs in the **same owner-connection
     transaction** as the DDL + ownership reconcile + grants + version stamp + cursor
     advance (legal because the deployer applies runtime steps over the owner connection
     and `append_audit_row` runs in the caller's transaction); name that connection/role.
   - *Q3-B:* add a step-level idempotent receipt rule keyed on
     `(plan_hash, step_id, sha256, state=step_committed)` so the `in_progress(k)`
     reconciler appends **exactly one** receipt before writing `step_committed(k)`,
     resolving the duplicate-vs-skip ambiguity.
   - Add `T-deploy-step-receipt-crash` (and extend F8/F7) asserting, after each crash
     point (after transactional DDL+cursor before receipt; after NT-DDL side effect
     before receipt; after receipt before `step_committed`; after `step_committed`):
     final schema equality, **exactly one** receipt per applied step, one terminal
     `complete` receipt, and a green doctor — and tighten `doctor schema_deploy_unrecorded`
     so a missing per-step receipt is surfaced, not masked by a present terminal receipt.

## 5. What already cleared (carry forward — do NOT regress or re-litigate)

- **C1 (finalization boundary) — resolved this cycle** (§3 above); preserve the
  `finalizing` state + idempotent finalizer + §1.3 row + F10 verbatim.
- **C2 fail-closed activation edge — resolved this cycle** (`CheckDeployActivation`
  before `ApplyMigrations`, typed halts, forward-watermark rule, Required=19, F11);
  preserve it. Only the choreography *completion* is blocked by C3.
- **Q4 — resolved** (v1 ledger §5; reaffirmed). Plain verb now, bootstrapping paradox
  dispositive, run-shape door held by the three seams. Neither falsifier contested it.
- **Q3 mid-step resumability — sound.** Q3-A (cursor advance in the step transaction) and
  Q3-B (`in_progress` pre-marker + invalid-index reconciler), the §1.1 step taxonomy, and
  the `deploy_cursor`-as-second-signal insight carry forward. N1 is about the per-step
  *receipt*, not the per-step schema/cursor body — preserve the body.
- **Decoupling shape — sound.** Shadow-first default-OFF `STRIATUM_DEPLOY_DECOUPLED`,
  `ConnectAndVerify` preserving the P2 watermark interlock + P3 drift gate/self-record,
  the embed-FS-derived plan, the hash-chained receipt mechanism, and
  `doctor schema_deploy_unrecorded`.
- **Scope discipline held.** P5 (rehearse / expand-contract / fidelity tiering / clone)
  is deferred; full capability revocation is named as a follow-up beyond P4, not assumed;
  the local-first boundary is respected.

## 6. Gate decision

The ledger verdict — not falsifier completion — clears the phase gate. With **C3 not
genuinely resolved** (the Policy-1 transfer self-contradicts the 0020 revoke it ships
alongside, stranding the activation deploy) and a **new material Q3/R4 challenge
standing** (the per-step receipt is not crash-safe), the gate is **not cleared**, even
though C1 and the C2 fail-closed edge are genuinely resolved.

**Verdict: `needs_revision`.** This is the gate's single allowed revision cycle, so the
gate ends **unCleared**; a clearing verdict would require a re-scaffolded revision that
pins both §4 items (C3's 0020-compatible, tested ownership mechanism and the crash-safe
per-step receipt protocol) without regressing C1, the C2 edge, or the carry-forward
items.

<sub>Adjudicator collaboration ledger for the RFC 0142 P4 falsification-gate design
run, v2 / REVISION, cycle 1. Inputs:
`docs/operator/artifacts/rfc-0142-p4-design-v2/dialogue/holder/HOLDER.md`,
`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`, `SEED.md`;
context: v1 `HOLDER.md` + v1 `COLLABORATION_LEDGER_cycle_1.md`. Verdict vocabulary:
accept | accept_with_findings | needs_revision | reject.</sub>
