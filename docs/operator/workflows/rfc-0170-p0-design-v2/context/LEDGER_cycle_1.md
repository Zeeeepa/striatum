---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
author: adjudicator-author-001
workflow: "rfc-0170-p0-design"
run_id: "run_85afe0ffd067616db27edf0a3c4e4afa"
cycle: 1
topic: "RFC 0170 P0 falsifiable implementation SPEC (self-culling repository: the Tier-1 cullable_entity candidacy substrate + read-only DecayTickSweep) — falsification gate cycle 1"
participants:
  - holder
  - falsifier_1
  - falsifier_2
  - adjudicator
entries:
  - kind: claim
    by: holder
    refs: ["dialogue:1"]
    text: "P0 = exactly three things and nothing more: (1) a runtime migration go/pkg/db/sql/0045_cullable_entity.sql (PC1: go/pkg/db/migrations does not exist; runtime migrations are embedded SQL at go/pkg/db/sql/ per migrations.go:21 //go:embed sql/*.sql; PC2: 0044 is highest, 0045 free) carrying both a read and a write authority-inventory row (C1) and a striatumd_rw GRANT SELECT,INSERT,UPDATE with no owner DDL/FK for >=27 (C2), every read projecting explicit columns kind,ref,candidacy_state never SELECT * (C3, the #614/bundle-0022 lesson); (2) a read-only DecayTickSweep.SweepOnce in go/pkg/recovery that piggybacks the existing recovery-sweep fold (RFC 0137 metrics-fold position) and per tick evaluates the Tier-1 predicate and UPSERTs candidacy rows; (3) read-only candidacy state only — no tombstone, deletion, page, doctor RED/amber, or run-admission. Hard-core claims: G1 Tier-1 is EXACT (front-matter Status-only, live-successor required, inbound-citation withholding, branch-namespace and protected-root exclusion; verdicts.superseded_by_decision_id explicitly excluded per PC3 since it supersedes a review verdict not an artifact); the G1 corpus asserts P0 nominates exactly {rfc:0027, rfc:0097, rfc:0041, rfc:0039} with zero hits on the preserved set. G2 the sweep is read-only and ERROR-ISOLATED via a runDecayTickSweep recover() seam + discard-the-error fold (B2) at bounded O(corpus) cost (B3). G4 OQ1 resolved (P0 = sweep/peer writer; (kind,ref) ON CONFLICT + extensible candidacy_state CHECK do not preclude a later phase/toll writer, D1) with a crisp P0/P1+ deferral table."
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "G1 Tier-1 EXACTNESS does not clear. The holder's own predicate clause 4 withholds any entity referenced by a live, non-superseded artifact in the scanned roots, yet the published G1/A1 true-positive corpus forces rfc:0097 to be `nominated`. rfc:0097 is `Status: superseded by RFC 0116 / 0122 / 0124` (front-matter match), BUT it is live-cited in the scanned roots by RFC 0101 (Status: umbrella-of-record — live; cites RFC 0097 at lines 62,64,87,130-131,267,281-284,325 incl. the reference-link def [RFC 0097]: 0097-...md at 332) and RFC 0103 (Status: accepted — live; uses RFC 0097 self-hosting as a PROVEN BASELINE at 43,90 and as W6 run-orchestration at 213-219), and RFC 0102 (Status: proposed — live). This is a hard contradiction: honor clause 4 and rfc:0097 is withheld → A1's published true-positive set is not exact (the proof is false); honor the corpus and a live-cited superseded RFC is nominated → a false positive (zero-FP claim fails). Either way G1 is not proven. Deeper: clause 2 REQUIRES a named live successor while clause 4 FORBIDS any live inbound citation, but supersession-chain successors/siblings routinely cite the predecessor — the holder never reconciles 2 vs 4, so 'live successor' vs 'load-bearing citation' is the exact two-implementers-disagree ambiguity the SEED named, and the A1 set was not audited for it. Fix: remove rfc:0097, re-audit every A1 entry for live inbound citations, OR define a mechanically-testable citation taxonomy distinguishing disposable historical mentions from load-bearing live references without LLM judgment."
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "G2 READ-ONLY SAFETY does not clear. B2/B3 prove panic isolation and returned-error isolation and EXPECTED O(corpus) cost — but not a LATENCY/STALL boundary. The fold is synchronous in the single recovery goroutine: scheduler.go:55-80 re-enters wait(ctx, interval) only AFTER opts.SweepOnce(ctx) returns, and the wrapper (main.go:889-897) calls the cull fold before SweepOnce returns. A blocked or slow DecayTickSweep — an unbounded filesystem walk over docs/**, the inbound-citation grep over the registered checkout, or a lock/IO-waiting cull query — does NOT panic and does NOT return an error, so the recover() seam (sweep.go:32-40, unwind-only, no wall-clock bound) and the discard-the-error fold never fire; the goroutine is simply held, the next recovery tick never starts, and after doctorRecoveryCursorWedgedAfter = 5*time.Minute (doctor.go:16) doctor emits recovery_sweep_cursor_wedged.<run_id> for a running run with claimable jobs (doctor.go:375-466) which drives ok:false (doctor.go:334-356). That directly violates 'cannot stall the recovery sweep', 'no doctor RED/amber', and 'NO downstream action'. The DB statement_timeout is 60000ms = the sweep cadence (connection.go:289-290), so a repeated ~60s cull query consumes the scheduler period; the holder's filesystem scan has no statement_timeout equivalent at all. The holder even mischaracterizes 'slow query' as flowing through the recovered/discarded-error path (HOLDER 260-267) — false for work that does not return. SPEC names no cull-fold deadline, per-scan context timeout, statement timeout below cadence, skip-on-overrun policy, or regression test that a blocked cull scan cannot delay the next recovery sweep."
  - kind: nomination
    by: falsifier_2
    refs: ["dialogue:3"]
    text: "G3 SUBSTRATE CORRECTNESS independently confirmed clean by Falsifier 2 (no separate material challenge): the SPEC specifies both read and write authority-inventory rows, explicit cullable_entity column projections (no SELECT *), a runtime migration under go/pkg/db/sql/, and 0045 is free in the current tree. No hidden P1+ action found: the holder explicitly excludes tombstones, deletion, pages, doctor classes, run admission, verdicts candidacy, and branch candidacy."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:2"]
    text: "G1' (cycle-1 binding fix): make the G1 corpus exactly what the five-clause predicate actually produces and re-prove zero false positives. Either (a) REMOVE rfc:0097 from the true-positive set and re-audit rfc:0027/0041/0039 (and any other candidate) for live inbound citations from non-superseded artifacts in the scanned roots, OR (b) define a stricter, mechanically-testable citation taxonomy that distinguishes a disposable historical mention from a load-bearing live reference (e.g. exclude the named-successor refs themselves; distinguish a `see also`/reference-link from an active-baseline citation) WITHOUT reintroducing LLM judgment. In both cases reconcile clause 2 (requires a named live successor) with clause 4 (forbids any live inbound citation) so two implementers read every corpus row identically, and re-derive the corpus test to assert zero hits on the preserved set under the reconciled predicate."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:3"]
    text: "G2' (cycle-1 binding fix): add a real latency/stall boundary to the cull fold and prove it. Bound the DecayTickSweep with a per-tick deadline (context.WithTimeout strictly below DefaultSweepInterval = 60s) covering BOTH the DB read (statement_timeout below the recovery cadence) AND the unbounded filesystem/grep scan (which has no statement_timeout equivalent), plus a skip-on-overrun policy so a slow/blocked tick is abandoned rather than holding the single recovery goroutine. Add a regression test (mirror TestActiveRunSweepPanicDegradesRunAndContinues but for HANG, not panic): inject a DecayTickSweep scan that blocks past doctorRecoveryCursorWedgedAfter and assert the next recovery tick still runs and doctor does NOT emit recovery_sweep_cursor_wedged / stays ok:true."
verdict: "needs_revision"
rationale: "needs_revision. Two of the four gates clear and two do not, and both unmet gates carry a source-verified, material, unrebutted falsifier challenge — so this is not a clearing verdict. G3 SUBSTRATE CORRECTNESS is MET (verified against the tree at run base: go/pkg/db/migrations does not exist and runtime migrations are embedded at go/pkg/db/sql/ via migrations.go:21; 0044 is the highest slot and 0045 is FREE; read_authority_inventory.go, write_authority_inventory.go and the non-PG authority_inventory_static_test.go all exist; the holder specifies BOTH inventory rows, a striatumd_rw GRANT with no owner DDL/FK for >=27, and explicit-column reads (kind,ref,candidacy_state) with no SELECT * — Falsifier 2 independently confirmed all of this and raised no challenge). G4 FORWARD-COMPATIBILITY is MET (OQ1 resolved to the sweep/peer writer; the (kind,ref) ON CONFLICT upsert and the extensible candidacy_state CHECK are shown not to preclude a later phase/toll writer, D1; the P0/P1+ deferral table names tombstone=P1, cull_gate=P2, reaper=P3, accretion=P4, throttle=P5, Tiers 2-4 >=P1 without building any; no falsifier attacked G4 and Falsifier 2 found no smuggled P1+ action). G1 Tier-1 EXACTNESS is UNMET: Falsifier 1 STANDS. The holder's predicate clause 4 withholds any entity live-cited by a non-superseded artifact in the scanned roots, yet the published G1/A1 corpus forces rfc:0097 to be `nominated`. Verified at source: docs/rfcs/0097:3 is `Status: superseded by RFC 0116 / 0122 / 0124`, but RFC 0101 (umbrella-of-record, live) cites it repeatedly incl. a reference-link definition, and RFC 0103 (accepted, live) uses RFC 0097 self-hosting as a PROVEN BASELINE — load-bearing live context, not a footnote. So the corpus is internally inconsistent with the predicate: honor clause 4 and A1 is not exact (the proof is false); honor the corpus and a live-cited superseded RFC is a false positive. Either branch refutes 'zero false positives'. The deeper unresolved defect is that clause 2 (requires a named live successor) and clause 4 (forbids any live inbound citation) collide — supersession-chain successors/siblings routinely cite the predecessor — which is exactly the 'live successor' ambiguity the SEED's falsifier guidance named, and the A1 set was never audited for it. G2 READ-ONLY SAFETY is UNMET: Falsifier 2 STANDS. The holder proves panic isolation (runDecayTickSweep recover() seam) and returned-error isolation (discard-the-error fold) and EXPECTED O(corpus) cost, but NOT a latency/stall boundary. The fold is synchronous in the single recovery goroutine (scheduler.go:55-80 re-enters wait(interval) only after SweepOnce returns); a blocked/slow cull scan — unbounded docs/** walk, the inbound-citation grep, or a lock/IO-waiting query — neither panics nor returns an error, so the recover() seam (unwind-only, no wall-clock bound, sweep.go:32-40) and the fold never fire, the goroutine is held, the next recovery tick never starts, and after doctorRecoveryCursorWedgedAfter=5m (doctor.go:16) doctor emits recovery_sweep_cursor_wedged (doctor.go:375-466) driving ok:false (doctor.go:334-356) — i.e. P0 CAN stall recovery and turn doctor unhealthy, directly violating the gate. The DB statement_timeout equals the cadence (60s, connection.go:289-290) and the filesystem scan has no timeout at all; the holder mischaracterizes a 'slow query' as flowing through the recovered-error path (HOLDER 260-267), which is false for work that does not return. The SPEC names no cull-fold deadline, per-scan timeout, sub-cadence statement timeout, skip-on-overrun, or hang regression test. This is needs_revision, not reject: the SPEC is largely sound — the substrate work is excellent and fully cleared, OQ1 and the P0/P1+ boundary are clean, and both defects are narrow, concrete, and in-P0-buildable (a reconciled clause-2/clause-4 predicate + re-derived corpus for G1; a bounded cull fold + hang regression test for G2). This is the single allowed v1 revision cycle: the cycle-1 ledger records needs_revision with the two binding constraints (G1', G2') for the revising holder; if the v1 revision does not clear both, a second needs_revision exhausts the gate and routes the residual findings to the operator for a fresh -v2 run."
findings:
  - id: F-G1-LIVE-CITED-SUPERSEDED
    severity: critical
    posture: tier1_false_positive
    status: converted_to_constraint
    challenge: "The published G1/A1 true-positive corpus forces rfc:0097 to be `nominated`, but rfc:0097 (docs/rfcs/0097:3 `Status: superseded by RFC 0116 / 0122 / 0124`) is live-cited in scanned roots by RFC 0101 (umbrella-of-record, live; cites at 62,64,87,130-131,267,281-284,325,332) and RFC 0103 (accepted, live; PROVEN BASELINE at 43,90; W6 at 213-219), so the holder's own clause 4 (no live inbound citation) withholds it. The corpus and the predicate are mutually inconsistent — honoring clause 4 falsifies A1's exactness proof, honoring the corpus produces a false positive on a load-bearing superseded artifact. Clause 2 (named live successor required) and clause 4 (no live inbound citation) are never reconciled though supersession-chain successors cite the predecessor; the A1 set was not audited for live citations. Zero-false-positive exactness is therefore NOT proven."
    closest_acceptable_answer: "Re-derive the G1 corpus so it equals exactly what the five-clause predicate produces, removing rfc:0097 and re-auditing rfc:0027/0041/0039 for live inbound citations; OR define a mechanically-testable citation taxonomy distinguishing disposable historical mentions from load-bearing live references without LLM judgment; and reconcile clause 2 vs clause 4 so two implementers agree on every corpus row."
    affected_invariants:
      - "RFC 0170 Acceptance Criterion 1: Tier-1 zero false positives across the live supersession corpus"
      - "G1 resurrectable/cited-superseded exclusion"
    source_refs: ["dialogue:2"]
  - id: F-G2-LATENCY-STALL
    severity: critical
    posture: readonly_safety_latency
    status: converted_to_constraint
    challenge: "G2/B2 proves only panic and returned-error isolation; it does not bound wall-clock latency. The cull fold runs synchronously in the single recovery goroutine (scheduler.go:55-80 re-enters wait(interval) only after SweepOnce returns); a blocked/slow DecayTickSweep scan (unbounded docs/** walk, inbound-citation grep, or lock/IO-waiting query) neither panics nor returns an error, so the recover() seam (sweep.go:32-40, unwind-only) and the discard-the-error fold never fire, the goroutine is held, the next recovery tick never starts, and after doctorRecoveryCursorWedgedAfter=5m (doctor.go:16) doctor emits recovery_sweep_cursor_wedged (doctor.go:375-466) → ok:false (doctor.go:334-356). The DB statement_timeout equals the 60s cadence (connection.go:289-290) and the filesystem scan has no timeout at all. The sweep can therefore stall recovery and turn doctor unhealthy — not provably read-only/error-isolated."
    closest_acceptable_answer: "Add a per-tick cull-fold deadline (context.WithTimeout below the 60s DefaultSweepInterval) covering BOTH the DB read (sub-cadence statement_timeout) and the filesystem/grep scan, a skip-on-overrun policy, and a hang regression test (mirror TestActiveRunSweepPanicDegradesRunAndContinues but for a blocking, non-returning scan) proving the next recovery tick still runs and doctor stays ok:true."
    affected_invariants:
      - "G2: a panic or Tier-1 query error cannot suicide or stall the recovery sweep"
      - "G2: P0 triggers no doctor RED/amber and no run-admission effect"
    source_refs: ["dialogue:3"]
constraints:
  - id: C-G1-CITATION-EXACTNESS
    source_finding: F-G1-LIVE-CITED-SUPERSEDED
    posture: tier1_false_positive
    severity: critical
    kind: gate
    binding: true
    text: "Make the G1 corpus exactly what the five-clause predicate produces and re-prove zero false positives. Either remove rfc:0097 from the true-positive set and re-audit rfc:0027/0041/0039 (and any other candidate) for live inbound citations from non-superseded artifacts in the scanned roots, OR define a stricter, mechanically-testable citation taxonomy distinguishing disposable historical mentions from load-bearing live references (exclude the named-successor refs themselves; distinguish a see-also/reference-link from an active-baseline citation) WITHOUT reintroducing LLM judgment. Reconcile clause 2 (named live successor required) with clause 4 (no live inbound citation) so every corpus row reads identically to two implementers."
    source_refs: ["dialogue:2"]
    verification:
      gate: "G1 corpus test over the live tree asserts the reconciled predicate nominates exactly the genuinely-dead superseded set with ZERO hits on the preserved set (rfc:0097 included in the preserved set, alongside docs/reference/todo.md, rfc:0028, the backup/rfc-* banked branches, the RFC-0170 body/SEED/workflow.json/decision-log prose, and docs/records/_frozen/**), and every published true positive is independently confirmed to have no live inbound citation."
    final_review_required: true
  - id: C-G2-CULL-FOLD-DEADLINE
    source_finding: F-G2-LATENCY-STALL
    posture: readonly_safety_latency
    severity: critical
    kind: gate
    binding: true
    text: "Bound the cull fold so a slow or blocked DecayTickSweep cannot stall recovery. Add a per-tick deadline (context.WithTimeout strictly below DefaultSweepInterval = 60s) covering BOTH the DB read (a statement_timeout below the recovery cadence) and the unbounded filesystem/grep scan, plus a skip-on-overrun policy so a slow tick is abandoned rather than holding the single recovery goroutine."
    source_refs: ["dialogue:3"]
    verification:
      gate: "Hang regression test in go/pkg/recovery (mirror TestActiveRunSweepPanicDegradesRunAndContinues for a NON-returning scan, not a panic): inject a DecayTickSweep scan that blocks past doctorRecoveryCursorWedgedAfter and assert (i) the next recovery tick still runs and (ii) doctor does NOT emit recovery_sweep_cursor_wedged and stays ok:true."
    final_review_required: true
branches:
  tier1_false_positive: "blocked"
  readonly_safety_latency: "blocked"
  substrate_correctness: "cleared"
  forward_compatibility: "cleared"
---

# Collaboration Ledger — RFC 0170 P0 design (cycle 1)

**Verdict: `needs_revision`.** Two of the four gates clear (G3 substrate, G4
forward-compatibility) and two do not (G1 Tier-1 exactness, G2 read-only safety),
and **both unmet gates carry a source-verified, material, unrebutted falsifier
challenge.** This is the single allowed v1 revision cycle: the two binding
constraints below (G1′, G2′) go to a revising holder; if the revision does not
clear both, a second `needs_revision` exhausts the gate and routes the residuals
to the operator for a fresh `-v2` run.

This is `needs_revision`, **not `reject`** — the SPEC is largely sound (the
substrate work is excellent and fully cleared, OQ1 and the P0/P1+ boundary are
clean), and both defects are narrow, concrete, and in-P0-buildable.

## Per-goal disposition

| Gate | Result | Basis |
|------|--------|-------|
| **G1 — Tier-1 exactness** (zero false positives; resurrectable/banked excluded) | **UNMET** | Falsifier 1 STANDS. The predicate's clause 4 withholds any live-cited entity, yet the published A1/G1 corpus forces `rfc:0097` to be `nominated` — and `rfc:0097` is superseded **and** live-cited by RFC 0101/0103 in the scanned roots. The corpus contradicts the predicate; exactness is not proven. Clauses 2 and 4 are never reconciled. |
| **G2 — read-only safety** (error-isolated from recovery, no action, no page) | **UNMET** | Falsifier 2 STANDS. Panic + returned-error isolation are proven; **latency/stall isolation is not.** A blocked synchronous cull scan holds the single recovery goroutine, stalls the next recovery tick, and after 5 min trips `recovery_sweep_cursor_wedged` → `ok:false`. No fold deadline, per-scan timeout, sub-cadence statement timeout, skip-on-overrun, or hang regression test. |
| **G3 — substrate correctness** (slot + grants + read/write inventory + no `SELECT *`) | **MET** | Verified at run base: `go/pkg/db/migrations` does not exist; runtime migrations are embedded at `go/pkg/db/sql/` (`migrations.go:21`); `0044` highest, **`0045` free**; `read_/write_authority_inventory.go` + the non-PG `authority_inventory_static_test.go` all exist; both inventory rows specified, GRANT with no owner DDL/FK for ≥27, explicit-column reads (`kind,ref,candidacy_state`), no `SELECT *`. **Falsifier 2 independently confirmed all of this and raised no challenge.** |
| **G4 — forward-compatibility** (OQ1 resolved; crisp P0/P1+ boundary) | **MET** | OQ1 resolved to the sweep/peer writer; the `(kind,ref)` ON CONFLICT upsert + extensible `candidacy_state` CHECK are shown not to preclude a later phase/toll writer (D1); the §6 deferral table names tombstone=P1, `cull_gate`=P2, reaper=P3, accretion=P4, throttle=P5, Tiers 2–4 ≥P1 without building any. No falsifier attacked G4; Falsifier 2 found no smuggled P1+ action. |

## Per-falsifier disposition

- **Falsifier 1 — Tier-1 exactness / false-positive lens (dialogue:2): MATERIAL,
  STANDING.** Source-verified: `docs/rfcs/0097-...md:3` is `Status: superseded by
  RFC 0116 / 0122 / 0124`; `RFC 0101` (`Status: umbrella-of-record`, live) cites
  RFC 0097 at lines 62, 64, 87, 130–131, 267, 281–284, 325 and defines the
  reference link `[RFC 0097]:` at 332; `RFC 0103` (`Status: accepted`, live) uses
  RFC 0097 self-hosting as a **proven baseline** (43, 90) and as W6 run
  orchestration (213–219); `RFC 0102` (`Status: proposed`) is also live. The
  holder's own clause 4 therefore withholds `rfc:0097`, contradicting A1/G1.
  Either branch refutes "zero false positives." The clause-2-vs-clause-4
  reconciliation the SEED named ("two implementers would read differently") is
  unresolved. **Not rebutted → C-G1.**
- **Falsifier 2 — read-only safety / substrate-integrity lens (dialogue:3):
  MATERIAL, STANDING (latency).** Source-verified: `scheduler.go:55-80` re-enters
  `wait(ctx, interval)` only after `SweepOnce` returns; the wrapper
  (`main.go:889-897`) calls the cull fold before that return; the panic seam
  (`sweep.go:32-40`) catches unwinds only — no wall-clock bound; `doctor.go:16`
  sets `doctorRecoveryCursorWedgedAfter = 5*time.Minute` and `doctor.go:375-466`
  emits `recovery_sweep_cursor_wedged` driving `ok:false` (`doctor.go:334-356`).
  A blocked/slow cull scan that neither panics nor returns therefore stalls
  recovery and turns doctor unhealthy. The DB `statement_timeout` equals the 60s
  cadence (`connection.go:289-290`); the filesystem scan has no timeout. **Not
  rebutted → C-G2.** Falsifier 2's scoped passes additionally **confirm G3 clean**
  (both inventory rows, explicit columns, runtime `go/pkg/db/sql/` migration,
  `0045` free) and **no hidden P1+ action** — those confirmations are recorded but
  do not save G2.

## Gate outcome

Both gate-critical claims (G1, G2) carry a standing, source-verified, unrebutted
material challenge; G3 and G4 clear. Per the objective's clearing rubric a
clearing verdict requires **all four**, so this is **not** a clearing verdict.
The defects are recoverable in one focused cycle:

- **C-G1 (Tier-1 exactness):** re-derive the corpus to equal what the predicate
  actually produces (remove `rfc:0097`; re-audit `rfc:0027/0041/0039` for live
  citations) **or** define a mechanically-testable citation taxonomy, and
  reconcile clauses 2 and 4.
- **C-G2 (cull-fold latency boundary):** add a per-tick deadline (DB + filesystem)
  below the 60s cadence, a skip-on-overrun policy, and a **hang** regression test
  proving a blocked cull scan cannot delay the next recovery sweep or turn the
  recovery doctor unhealthy.

This is `needs_revision`, not `reject`: the architecture is sound and both fixes
are narrow, concrete, and in-P0-buildable. This is the single allowed v1 revision
cycle.
