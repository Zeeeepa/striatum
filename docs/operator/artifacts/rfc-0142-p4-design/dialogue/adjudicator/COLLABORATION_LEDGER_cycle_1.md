---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0142 P4 one-shot deployer (decouple schema-apply from serve-boot) — falsifiable implementation spec"
participants:
  - "holder-author-001"
  - "falsifier-reviewer-001"
  - "falsifier-reviewer-002"
  - "adjudicator-author-001"
cycle: 1
entries:
  - kind: claim
    by: "holder-author-001"
    refs: ["dialogue:1"]
    text: "P4 spec: the per-step-atomic + resumable-cursor contract (Q3-A same-tx for transactional steps, Q3-B in_progress-marker + reconciler for NT-DDL) is sufficient for every owner+runtime interleaving P4 ships; Q4 is resolved as plain-verb-now with three run-shape seams; serve-boot is decoupled shadow-first behind STRIATUM_DEPLOY_DECOUPLED with no window serving an unmigrated schema; and serving-role create-DDL is revoked via owner bundle 0020 without recreating a lockout."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "Finalization cursor gap: the spec orders deploy_cursor.state=complete BEFORE the receipt append and RecordSchemaFingerprint, so a crash after complete (expected plan_hash) but before receipt+fingerprint yields complete + expected plan_hash + fingerprint mismatch — a state no row of the load-bearing 1.3 classification table covers (not in-sync, not incomplete/resume, not genuine-drift). Hits even a purely transactional plan; the Q3 correctness core is therefore insufficient at the final boundary with no sub-protocol specified."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "0020 + STRIATUM_DEPLOY_DECOUPLED activation is not fail-closed: the owner watermark tolerates a forward bundle (applied > required returns nil), so with 0020 applied and the flag OFF/absent (pre-P4 binary or P4 binary) boot reaches ApplyMigrations over the runtime runner; under revoked CREATE a pending runtime migration fails 42501, which main.go does not map to a typed halt and so falls through to the fatal crash-loop path — the exact #512-class lockout P4 must close. The strict-ordering together is unenforced and self-contradicts auto-apply-still-default vs RequiredOwnerBundleVersion=20."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "Runtime-object ownership contract undefined: applyOne has no SET ROLE / ALTER OWNER TO / post-step grant reconciliation, so running runtime migrations through the owner/admin connection creates owner-owned objects, silently changing ownership semantics of every future runtime migration and undermining the section 4.1 residual-capability argument that relies on runtime tables staying striatumd_rw-owned. The spec uses both DDL-via-owner and runtime-rw-owned without choosing or testing one policy."
    correspondence: landed_unrebutted
verdict: "needs_revision"
rationale: "Three material falsifier challenges land and stand unrebutted by the spec as written, two of them in the P4 correctness core. C1 (Q3): a crash at the finalization boundary (cursor complete with expected plan_hash, before receipt+fingerprint) is unclassifiable by the spec's own 1.3 table and hits even a purely transactional plan, so the per-step-atomic + resumable-cursor contract is insufficient for a shipped interleaving with no sub-protocol — per the rubric this alone forces needs_revision. C2 (R3): a forward-watermark interleaving drives boot into ApplyMigrations under a revoked CREATE, producing a raw 42501 that falls through to the crash-loop path, falsifying the even-a-botched-order-refuses-cleanly claim and recreating a #512-class lockout. C3: owner/admin-applied runtime steps leave object ownership undefined, contradicting the section 4.1 safety argument. Q4 (plain-verb-now + three seams), the per-step Q3-A/Q3-B mid-step resumability, and the shadow-first decoupling shape are sound and carry forward; the defects are the finalization boundary, the activation fail-closed edge, and the ownership policy. One revision cycle is available; the falsifiers re-attack the revised spec."
findings:
  - id: C1
    severity: critical
    posture: design
    status: open
    challenge: "Finalization boundary is unclassifiable (Q3 core). Fix: either keep the cursor at step_committed(N-1) until receipt + schema_state fingerprint are durable then set complete last, or add a distinct finalizing state classified as resumable-finalization (never serve, never genuine-drift); specify which finalization writes share a transaction/role or an idempotent finalizer; add the matching 1.3 row and T-deploy-resume-finalization-crash covering kills after final DDL commit, cursor-complete-before-receipt, receipt-before-schema_state, schema_state-before-cursor-complete."
    affected_invariants: ["Q3 per-step-atomic + resumable-cursor contract", "1.3 incomplete-vs-drift disambiguation table"]
    source_refs: ["dialogue:2"]
  - id: C2
    severity: high
    posture: design
    status: open
    challenge: "0020 + flag activation is not fail-closed before mutation (R3 lockout). Fix: a typed non-restartable halt (awaiting_deploy / awaiting_deploy_config) that fires before ConnectAndMigrate reaches ApplyMigrations whenever owner_bundle_meta >= 20 and (decoupled OFF or deploy incomplete); a forward-watermark rule so an older binary seeing 0020 does not proceed into auto-apply; resolve the auto-apply-default vs RequiredOwnerBundleVersion=20 contradiction; add T-deploy-revoke-activation-ordering over the bad interleavings asserting ApplyMigrations not called, DB untouched, remediation names striatum daemon deploy / the missing flag."
    affected_invariants: ["R3 no DDL-revocation lockout", "serve-boot clean-halt (no crash-loop)"]
    source_refs: ["dialogue:3"]
  - id: C3
    severity: high
    posture: design
    status: open
    challenge: "Runtime-object ownership contract undefined under owner/admin apply. Fix: pick and test one policy — if runtime objects stay striatumd_rw-owned, specify the post-step ALTER OWNER TO + grant transfer for tables/indexes/sequences/views/future kinds; if owner/admin owns new runtime objects, add a build/load guard that every runtime migration grants the exact DML the daemon needs and correct 4.1 to stop claiming runtime ownership as a P4 safety property; add T-deploy-runtime-object-ownership asserting recorded owner and the serving role's real DML under striatumd_rw."
    affected_invariants: ["two-role owner/runtime boundary", "section 4.1 residual-capability argument"]
    source_refs: ["dialogue:3"]
branches:
  design: blocked
---

# COLLABORATION LEDGER — RFC 0142 P4 design run, cycle 1

author: adjudicator-author-001

**Verdict:** `needs_revision`

> Scope of this adjudication: the curated dialogue trajectory only — the Holder's
> `HOLDER.md` P4 implementation spec and the two falsifiers'
> `FALSIFIER.md` challenges — plus the `SEED.md` charter. RFC 0142 is accepted
> (D258); this judges the **P4 implementation shape**, not the five-layer design.
> A clearing verdict would be `accept` or `accept_with_findings`; this is neither.

## 1. Decision

`needs_revision`. The Holder produced a strong, well-anchored spec and **resolved
Q4 cleanly**, but **three material challenges land and stand unrebutted by the
spec as written**, two of them squarely in the P4 correctness core:

- **C1 — the finalization boundary is unclassifiable (Q3 core).** A crash after
  the cursor reaches `complete` but before the receipt + fingerprint are durable
  produces a state no §1.3 row classifies. This is a shipped interleaving (every
  deploy finalizes) for which the per-step-atomic + resumable-cursor contract is
  insufficient and **no sub-protocol is specified**. Per the role rubric this
  alone forces `needs_revision`.
- **C2 — 0020 + flag activation is not fail-closed (R3 lockout / serve-boot
  regression).** A concrete forward-watermark interleaving drives boot into
  `ApplyMigrations` under a revoked `CREATE`, producing a raw `42501` that falls
  through to the fatal crash-loop path — the exact #512-class lockout P4 exists to
  close. The Holder's "even a botched order refuses cleanly" claim is falsified
  against current source.
- **C3 — the runtime-object ownership contract is undefined under owner/admin
  apply.** Running runtime migrations through the owner/admin connection silently
  changes object ownership, and the spec relies on both "DDL via owner" and
  "runtime tables stay `striatumd_rw`-owned" without choosing or testing a policy.

One revision cycle is available; the falsifiers re-attack the revised spec. The
revision must fix exactly the three items in §4.

## 2. Per-challenge ledger

| # | Source | Claim challenged | Material? | Spec rebuts it? | Disposition |
| --- | --- | --- | --- | --- | --- |
| C1 | falsifier_1 | Q3 sufficiency: §1.2 finalization order + §1.3 classification completeness (`HOLDER.md:102-105,115-119`) | **Yes** — Q3 correctness core; a real crash window on every deploy | **No** — §1.2 orders `complete` *before* receipt+fingerprint; §1.3 has no row for the resulting state | **Upheld → needs_revision** |
| C2 | falsifier_2 #1 | DDL-REVOKE "ships without lockout / even a botched order refuses cleanly" (`HOLDER.md:316-335`) | **Yes** — concrete crash-loop / #512-class lockout (R3) | **No** — "together" is unenforced; no typed halt before `ApplyMigrations`; forward-watermark case unaddressed | **Upheld → needs_revision** |
| C3 | falsifier_2 #2 | Owner/admin-applied runtime steps + "runtime tables stay rw-owned" (`HOLDER.md:245-250,297-305,309-314`) | **Yes** — changes ownership semantics of every future runtime migration; undermines the §4.1 residual-capability argument | **No** — spec uses both policies, picks/tests neither | **Upheld → needs_revision** |

## 3. Why each challenge is material and unrebutted

### C1 — finalization cursor gap (falsifier_1): UPHELD

The Holder's §1.2 states `complete` "is written only when `step_index == N-1` is
`step_committed`; the deployer **then** writes the deploy receipt (§3.4) and calls
`RecordSchemaFingerprint`" (`HOLDER.md:102-105`). That makes the cursor-`complete`
write, the receipt append, and the `schema_state` fingerprint UPSERT **three
separate facts in three separate commits**.

Take the crash point *after* the cursor reaches `complete` but *before* the
receipt + fingerprint land. Observable state:

- `deploy_cursor.state = complete`, `plan_hash == expected`
- `LiveFingerprint != ExpectedFingerprint()` (or `""` on a fresh / P3-unrecorded DB)
- final deploy receipt absent

Cross-checking this against the §1.3 disambiguation table (`HOLDER.md:115-119`):

- **not** the in-sync row — that row requires fingerprint **match**;
- **not** the incomplete/resume row — that row requires cursor `in_progress` /
  `step_committed`;
- **not** the genuine-drift row — that row is `complete` **with a foreign
  `plan_hash`** (or `aborted`); here the `plan_hash` is the *expected* one.

So the state is genuinely **unclassifiable** by the spec's own load-bearing table
— the very "interrupted apply looks identical to corruption" failure the SEED
names as the hard correctness core, reappearing at the *final* boundary. It is
independent of `CREATE INDEX CONCURRENTLY`: a **purely transactional** plan still
hits it whenever the terminal `complete` marker can commit ahead of the receipt
and fingerprint. The falsifier's source reading is correct and corroborates the
gap: `RecordSchemaFingerprint` is a separate UPSERT (`schema_drift.go:171-194`),
`LiveFingerprint` returns `""` when absent (`:139-160`), and the pure evaluator
treats empty-Live as not-drift (`:226-236`) — safe for today's boot self-record,
unsafe once a durable `complete` cursor can precede it.

The strongest rebuttal available to the Holder ("`complete` was shorthand for
all-finalization-done", or "`doctor schema_deploy_unrecorded` will warn") does
**not** clear the gate: the spec explicitly orders `complete` first, and a doctor
warning is post-facto visibility, not a resumability contract — the Q3 gate
requires the interrupted state to be *classified* as incomplete/resume and
repaired without serving stale provenance or refusing a cleanly applied schema as
drift. There is no Holder rebuttal artifact in this cycle's trajectory; the spec
text stands as written, and as written it does not answer this. **Q3 has not
cleared.**

### C2 — 0020 + `STRIATUM_DEPLOY_DECOUPLED` activation is not fail-closed (falsifier_2 #1): UPHELD

The Holder's safety claim is the strict ordering in `HOLDER.md:316-335`, capped by
"even a botched order … would refuse cleanly (drift / awaiting), not lock out."
The falsifier refutes that against current source with a concrete interleaving:

- `--migrate` defaults true (`main.go:51,76`); boot calls `ConnectAndMigrate`,
  which checks the owner watermark then immediately applies runtime migrations
  over the **runtime** runner (`connection.go:349-353`).
- The watermark check **tolerates a forward owner bundle**: `applied > required`
  returns `nil` (`owner.go:76-80,107-109`). So a DB already at 0020 does **not**
  halt under either a pre-P4 binary (`Required == 19`) or a P4 binary with the
  decoupled flag OFF/absent — both proceed into auto-apply.
- 0020 revokes `CREATE ON SCHEMA striatumd FROM striatumd_rw` (`HOLDER.md:291-295`).
  With any pending runtime migration (including the P4 `deploy_cursor` migration
  `>= 0044` when `daemon deploy` has not yet run), `applyOne` executes the
  `CREATE TABLE`/`CREATE INDEX` over the runtime runner (`migrations.go:304-318`)
  → `42501`.
- `main.go` maps **only** `AwaitingOwnerDDLError` and `SchemaDriftError` to the
  clean non-restartable exit (`main.go:199-228`). A raw `42501` from
  `ApplyMigrations` falls through to the fatal boot path — the old crash-loop
  shape, **not** a typed `awaiting_deploy`/`awaiting_owner_ddl` halt.

This is precisely the deployment-order class P4 must close: the runtime role,
post-revoke, discovers it cannot apply runtime DDL **inside** boot-time
`ApplyMigrations` rather than refusing before mutation. The falsifier also flags a
real internal contradiction: §4.2 says the P4 binary lands with 0020 not applied
and "auto-apply still the default" (`HOLDER.md:316-320`) while simultaneously
advancing `RequiredOwnerBundleVersion` to 20 (`HOLDER.md:326-331`) — on a 0019 DB
that binary halts before auto-apply, so the two statements cannot both hold. The
word "together" carries the safety guarantee without any durable state-machine
edge, preflight, or typed halt enforcing it. **The decoupling activation is not
proven free of an R3 lockout / clean-halt regression.**

### C3 — runtime-object ownership contract undefined (falsifier_2 #2): UPHELD

The Holder routes all post-0020 DDL through the owner/admin connection
(`HOLDER.md:245-250,309-314`) while also leaning on "runtime tables remain
`striatumd_rw`-owned" as the reason full capability revocation is out of P4 scope
(`HOLDER.md:297-305`). But `applyOne` executes migration SQL on the supplied
runner with **no `SET ROLE striatumd_rw`, no `ALTER … OWNER TO striatumd_rw`, and
no post-step grant/ownership reconciliation** (`migrations.go:304-335`). Run
through owner/admin, the same SQL therefore creates **owner-owned** runtime
objects unless each migration carries explicit grants/transfers. Migration 0043
grants DML to `striatumd_rw` but does **not** transfer ownership
(`0043_schema_state.sql:39-50`), and the new `deploy_cursor` is modeled on 0043
(`HOLDER.md:67-69,427`) — so P4 adds yet another runtime table whose owner
semantics are unspecified. This changes the ownership semantics of every future
runtime migration and directly undercuts the §4.1 residual-capability argument
(which assumes new runtime objects stay rw-owned). The challenge is material even
though the serve path issues no DDL: the product still needs a crisp owner/grant
invariant the serving daemon and the two-role pgtest can rely on. The spec uses
both policies and commits to neither; **a policy must be chosen and tested.**

## 4. What the revision must fix (exact)

The revised spec clears the gate only if it pins all three. These map to the
falsifiers' own "required repair" lists.

1. **Close the finalization boundary (C1 — Q3 core).** Adopt one concrete
   sub-protocol and add the matching §1.3 row:
   - *Option A:* keep the cursor at `step_committed(N-1)` until the deploy receipt
     **and** the `schema_state` fingerprint are durable, then set `complete`
     **last** — so the existing incomplete/resume row covers finalization retry; or
   - *Option B:* add a distinct `finalizing` state and classify
     `finalizing` / expected `plan_hash` / fingerprint-mismatch as **resumable
     finalization** (never serve, never genuine-drift).
   - Either way, state which finalization writes share a transaction/role, or
     specify an **idempotent finalizer** that runs correctly after every crash
     point. Add `T-deploy-resume-finalization-crash` killing after each of: final
     DDL commit, cursor-complete-before-receipt, receipt-before-`schema_state`,
     and `schema_state`-before-cursor-complete — asserting **no serving until
     finalization is coherent**, exactly-once repair on re-run, and
     `doctor schema_deploy_unrecorded` green after repair.

2. **Make 0020 activation fail-closed before mutation (C2 — R3).** Add a typed
   non-restartable halt (`awaiting_deploy` / `awaiting_deploy_config`) that fires
   **before** `ConnectAndMigrate` reaches `ApplyMigrations` whenever
   `owner_bundle_meta >= 20` **and** (decoupled mode OFF **or** the deploy is
   incomplete). Add a forward-watermark rule so an older binary that observes
   applied owner bundle 0020 does **not** proceed into runtime auto-apply as if
   forward tolerance were harmless. Resolve the "auto-apply still default" vs.
   `RequiredOwnerBundleVersion = 20` contradiction and state the deploy
   choreography precisely (daemon up/down, which binary, parked vs. serving). Add
   `T-deploy-revoke-activation-ordering` over: 0020-before-flag; 0020-before
   `daemon deploy`; old-binary + 0020 + pending runtime migration; P4-binary +
   flag-OFF + pending runtime migration — asserting `ApplyMigrations` is **not**
   called, the DB is untouched, and the remediation names `striatum daemon deploy`
   / the missing flag.

3. **Define the runtime-object ownership contract under owner/admin apply (C3).**
   Choose one policy and test it:
   - *If runtime objects stay `striatumd_rw`-owned:* specify the post-step
     ownership/grant transfer (`ALTER … OWNER TO` + grants) covering tables,
     indexes, sequences, views, and future object kinds; or
   - *If owner/admin owns new runtime objects:* add a build/load guard that every
     runtime migration grants the exact DML the daemon needs, **and** correct §4.1
     so it no longer claims runtime ownership as a P4 safety property for new
     objects.
   - Add `T-deploy-runtime-object-ownership`: apply a new runtime migration
     through the deployer's owner connection and assert both the recorded owner and
     the serving role's real DML behavior under `striatumd_rw`.

## 5. What already cleared (carry forward into the revision)

To keep the next cycle efficient, these are sound and should not be re-litigated:

- **Q4 — resolved.** Plain verb now (`striatum daemon deploy`), with the
  bootstrapping paradox correctly treated as dispositive against a run-shape base
  deployer, and the run-shape door held open by three concrete seams
  (content-addressed `plan_hash`, hash-chained receipt in `audit_log`, a
  `Deployer.Apply` engine the verb and a future run share). Neither falsifier
  contested it; I find Q4 concretely resolved.
- **Q3 mid-step resumability — sound.** Q3-A (cursor advance in the **same**
  transaction as a transactional step) and Q3-B (`in_progress` pre-marker +
  invalid-index reconciler for `CREATE INDEX CONCURRENTLY`/`ALTER TYPE … ADD
  VALUE`), the step taxonomy (§1.1), and the core insight that `deploy_cursor` is
  the **second signal** disambiguating "incomplete" from "drift" are all solid.
  The defect is at the *final* boundary (C1), not in the per-step body — preserve
  the per-step design.
- **Decoupling shape — sound; only the activation edge is missing.** Shadow-first
  default-OFF `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2
  watermark interlock and P3 drift gate/self-record, the `awaiting_deploy` typed
  halt, the embed-FS-**derived** plan (no second hand-authored manifest), the
  hash-chained deploy receipt, and `doctor schema_deploy_unrecorded` are all good.
  C2 is about the *sequencing/fail-closed edge*, not the decoupling design itself.
- **Scope discipline held.** P5 (rehearse / expand-contract / fidelity tiering /
  clone) is explicitly deferred; the local-first boundary (one host, one Postgres,
  one daemon writer, no external persistence) is respected.

## 6. Gate decision

The ledger verdict — not falsifier completion — clears the phase gate. With the
Q3 finalization gap (C1) unrebutted, the R3 activation lockout (C2) demonstrated
against current source, and the ownership contract (C3) undefined, the gate is
**not cleared**.

**Verdict: `needs_revision`.** Revise per §4; the falsifiers re-attack the
revised spec in cycle 2.

<sub>Adjudicator collaboration ledger for the RFC 0142 P4 falsification-gate
design run, cycle 1. Inputs: `dialogue/holder/HOLDER.md`,
`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`,
`SEED.md`. Verdict vocabulary: accept | accept_with_findings | needs_revision |
reject.</sub>
