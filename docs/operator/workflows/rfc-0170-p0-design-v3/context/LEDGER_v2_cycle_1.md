---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
author: adjudicator-author-001
workflow: "rfc-0170-p0-design-v2"
run_id: "run_3506471695cec27400eda2f3f33d4f6f"
cycle: 1
topic: "RFC 0170 P0 falsifiable implementation SPEC (self-culling repository: the Tier-1 cullable_entity candidacy substrate + read-only DecayTickSweep) — v2 REVISION gate: discharge G1' (reconciled predicate + re-derived corpus) and G2' (cull-fold latency bound + HANG test) without regressing the cleared G3/G4"
participants:
  - holder
  - falsifier_1
  - falsifier_2
  - adjudicator
entries:
  - kind: claim
    by: holder
    refs: ["dialogue:1"]
    text: "v2 revising-holder SPEC. Carries §2 (substrate / migration 0045 / both authority-inventory rows / striatumd_rw GRANT no owner DDL/FK for >=27 / no SELECT *), §5 (OQ1 = sweep/peer writer), §6 (P0/P1+ deferral table) INTACT from v1; rewrites only §3 (predicate + corpus) and §4 (sweep latency). G1' claim: clause 4 is rewritten into a pure-grep active-baseline inbound-citation rule (4a canonical-ref forms, 4b fixed inbound-scan set, 4c live-source-only, 4d not-self, 4e not-a-named-successor-backref, 4f not-a-disposable/closure-line via a fixed lexicon), reconciling clause 2 (named live successor) with clause 4 (no live inbound citation); the corpus is re-derived so rfc:0097 (and rfc:0027/0039/0041) move to the PRESERVED set as live active-baseline-cited, and the genuine zero-citation true-positive set becomes exactly {decision:D267, decision:D081}. G2' claim: §4 adds DefaultCullFoldTimeout = 10s (< DefaultSweepInterval = 60s) bounding BOTH the DB read (SET LOCAL statement_timeout='10000') AND the filesystem scan (cooperative WalkDir + a watchdog child goroutine the fold selects on, returning the recovery goroutine on cullCtx.Done()), a single-in-flight skip-on-overrun guard, and an L4 compute-then-commit write phase so a timed-out tick performs ZERO writes (no torn write); plus a NEW HANG regression test B5 (distinct from the panic test B2) asserting the next recovery tick still runs (Sweeps==2) and doctor stays ok:true with no recovery_sweep_cursor_wedged, and FAILS with the deadline removed. G3/G4 asserted carried intact."
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "G1' Tier-1 exactness does NOT fully discharge. The original rfc:0097 false positive IS fixed (rfc:0097 correctly moved to preserved; rfc:0027/0039/0041 also correctly preserved; no live artifact is auto-nominated). But the REPLACEMENT true-positive set {decision:D267, decision:D081} is not mechanically derivable from the predicate as written. Source-verified: holder clause 1 defines a DECISION's structural status field as ONLY the second pipe-delimited state cell (HOLDER.md:182-186), and clause 2 parses the successor from the `superseded by <refs>` clause 'up to the end of the status value / first sentence' (HOLDER.md:195-204), withholding any bare `superseded` with no parseable successor (HOLDER.md:206-210). But docs/decisions/decision-log.md:38 (D267) has state cell = bare `superseded`, with `SUPERSEDED by D270` in the THIRD (description) cell; docs/decisions/decision-log.md:220 (D081) has state cell = bare `superseded`, with `Superseded by D087/D094/D104` in the FIFTH (consequences) cell. A literal implementer reading the status value (= state cell) sees no parseable successor and WITHHOLDS both → true-positive set empty → A1 false; a looser implementer scans other own-row cells and NOMINATES both. The spec never states which non-state own-row cells are successor-bearing, the precedence, or the sentence boundary, so two implementers split on the only two rows the corpus depends on (HOLDER.md:319-328,373-375). Second, independent mechanical contradiction: clause 3 protects any path matching 'every entry already in .check-docs-ignore' (HOLDER.md:270-277), and live .check-docs-ignore:3 is `docs/rfcs/` wholesale — taken literally every RFC is protected before clause 2/4 runs, so no RFC can ever be nominated, contradicting §3's RFC corpus model that treats RFCs as eligible artifacts withheld only by clause 2/4 (HOLDER.md:333-355) and §2's kind='rfc' candidacy. A1/A4' are therefore not yet mechanically reproducible from the written predicate."
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "G2' read-only-safety latency bound does NOT fully discharge. The holder's partial-write repair IS credited as written (read-only scan in a child goroutine, recovery goroutine returned on cullCtx.Done(), single in-flight guard suppresses stacked scans, L4 compute-then-commit UPSERTs only after a complete in-deadline delta → no torn write), and the recovery goroutine is provably released. The material gap is B5's doctor-green assertion. The holder asserts a blocked scan cannot trip recovery_sweep_cursor_wedged because the recovery cursor / last_sweep_at advanced, citing doctor.go:383,448,460 (HOLDER.md:517-529). That is NOT source-true. Verified: go/pkg/reads/doctor.go recoveryCursorQuietSince (469-479) keys the quiet window on last_lane_advanced_at, else started_at, else created_at — NEVER last_sweep_at; the wedge fires when claimable_job_count > 0 AND that lane-quiet source is older than doctorRecoveryCursorWedgedAfter=5m (doctor.go:383,447-460); last_sweep_at appears only as reporting context. sweep.go:234-263 writes last_sweep_at on every cursor upsert, but sweep.go:284-331 derives claimable_job_count and last_lane_advanced_at from pending work / lane activity, which a cull-fold timeout or skip-on-overrun does NOT advance. The existing focused test TestHandleDoctorFlagsRecoverySweepCursorWedgedClaimableRun (go/pkg/reads/doctor_recovery_cursor_test.go:34-44) sets last_sweep_at = now-1m (FRESH), claimable_job_count=2, last_lane_advanced_at = now-10m and still returns ok:false with recovery_sweep_cursor_wedged.run_wedged. So B5's binding scenario (a running run with claimable work and no lane progress) either fails the ok:true assertion as written, or only passes by weakening to claimable=0 / fresh lane-advance — in which case it no longer proves the binding G2 property. The HANG test as specified does not prove the C-G2 verification gate's assertion (ii) against current doctor semantics; closing it needs either a no-cull control comparison ('the cull fold does not make recovery/lane doctor state worse than baseline') or an intentional doctor-semantics change keying the quiet window on last_sweep_at (with its own test), which the SPEC neither chooses nor builds."
  - kind: nomination
    by: falsifier_1
    refs: ["dialogue:2"]
    text: "Credited (G1 progress, recorded but not gate-clearing): the cycle-1 rfc:0097 false positive is resolved — rfc:0097 is correctly preserved under the revised clause 4 (RFC 0101 umbrella-of-record and RFC 0103 accepted still carry live active-baseline references, not mere named-successor backrefs or link definitions). The re-audited rfc:0027 / rfc:0039 / rfc:0041 also carry live non-successor references in the scanned roots, so their preserved classification is not challenged. No false-positive live artifact is auto-nominated by the revised corpus, and no counted live inbound citation keeps D267 or D081 alive (D267's only non-self hit is D270's named-successor backref). The standing defect is exactness/mechanism (the decision-successor parser and the protected pathspec), not a surviving false positive."
  - kind: nomination
    by: falsifier_2
    refs: ["dialogue:3"]
    text: "Credited (G2 progress + G3/G4 no-regression, recorded): the cycle-1 latency gap is substantially closed — the per-tick deadline covers both halves, the watchdog child-goroutine bounds the recovery goroutine even against a blocking syscall, skip-on-overrun does not stack scans, and L4 compute-then-commit means a timed-out tick performs zero writes (the partial/torn-write concern is answered). G3/G4 are NOT regressed: the migration remains go/pkg/db/sql/0045_cullable_entity.sql with GRANT SELECT,INSERT,UPDATE to striatumd_rw and no owner DDL/FK (HOLDER.md:89-124); both authority-inventory rows remain required and reads stay explicit-column, no SELECT * (HOLDER.md:126-150,480-486,595-597); the P0/P1+ deferral table keeps tombstone/cull_gate/reaper/accretion/throttle and Tiers 2-4 outside P0 (HOLDER.md:559-575); no smuggled tombstone, deletion, page, doctor class, or run-admission effect. The standing defect is the HANG test's doctor-green assertion, not the latency bound or a substrate regression."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:2"]
    text: "G1'' (residual to v3): make the DECISION-successor extraction a pure greppable rule and reconcile the protected pathspec with the scanned RFC root. (a) For kind=decision, keep the second pipe cell as the structural status field for clause 1, but define successor extraction explicitly: the exact own-row cells that may carry `superseded by <refs>` (description? consequences? a stated precedence across them), the exact sentence boundary, and the exact multi-ref split rule (e.g. D087/D094/D104), and state that other rows' cells are never successor sources. Add table-driven cases proving D267 and D081 nominate AND a bare state-cell-only `superseded` decision with no own-row successor prose is withheld. (b) Replace clause 3's 'every entry already in .check-docs-ignore' with a cull-specific protected pathspec, or explicitly subtract the actively-scanned roots (docs/rfcs/, docs/decisions/) from that protection so the kind=rfc/decision candidacy surface is not dead by construction. Re-state the G1 corpus result under the fixed field/pathspec rules so two implementers read every row identically."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:3"]
    text: "G2'' (residual to v3): reframe the HANG test's doctor assertion to match current go/pkg/reads/doctor.go semantics. Either (a) prove the binding property by an A/B control: run the blocking-scan tick against an identical NO-cull control and assert the cull fold does not make recovery/lane doctor state worse than baseline (recovery_sweep_cursor_wedged depends only on claimable_job_count + last_lane_advanced_at, which the cull fold never touches), while Sweeps==2 proves the recovery goroutine was released; OR (b) if the intended invariant is 'a fresh recovery sweep cursor keeps doctor green', intentionally change doctorRecoverySweepCursor (and TestHandleDoctorFlagsRecoverySweepCursorWedgedClaimableRun) to key the quiet window on last_sweep_at and declare that as an explicit doctor-semantics change with its own coverage — not a claim that current doctor already behaves that way. Keep the credited deadline + watchdog + compute-then-commit machinery as-is."
verdict: "needs_revision"
rationale: "needs_revision, and because this is v2's single proper revision round, the gate is EXHAUSTED and the two residual constraints (G1'', G2'') route to the operator for a fresh -v3 — do not ratchet, do not let the committer publish the consolidated SPEC, do not ratify D271. The verdict is NOT a clearing verdict because neither binding constraint is fully discharged, even though both made real progress and the two cleared gates (G3, G4) are confirmed un-regressed. G1' (C-G1-CITATION-EXACTNESS) is UNMET: Falsifier 1 STANDS. The cycle-1 rfc:0097 false positive is genuinely FIXED (rfc:0097 and rfc:0027/0039/0041 are correctly preserved as live active-baseline-cited, and no live artifact is auto-nominated), so the revision moved the gate forward. But the replacement true-positive set {decision:D267, decision:D081} is not mechanically derivable from the written predicate, which the objective treats as dispositive ('if falsifier_1 exhibits ... one corpus row two implementers would read differently, G1' is UNMET'). Verified at the run base: docs/decisions/decision-log.md:38 (D267) has state cell = bare `superseded` with `SUPERSEDED by D270` in the third (description) cell, and docs/decisions/decision-log.md:220 (D081) has state cell = bare `superseded` with `Superseded by D087/D094/D104` in the fifth (consequences) cell. Holder clause 1 fixes the decision structural status field as the second pipe cell ONLY and clause 2 parses the successor from the status value 'up to the first sentence', but the status value is bare — so a literal implementer withholds both (true-positive set empty, A1 false) while a looser implementer scanning other own-row cells nominates both. The spec never names the allowed successor-source cells, precedence, or sentence boundary, so the only two rows the corpus depends on read differently to two implementers. The holder's own rebuttal ('the intended parser is obvious from the corpus') is plausible but is exactly the unstated, non-greppable judgment G1' was required to eliminate. Independently, clause 3 protects 'every entry already in .check-docs-ignore' and live .check-docs-ignore:3 is `docs/rfcs/` wholesale, so taken literally every RFC is protected and no RFC can ever be nominated — contradicting §3's RFC corpus model and §2's kind='rfc' candidacy. A1/A4' are therefore not yet mechanically reproducible. G2' (C-G2-CULL-FOLD-DEADLINE) is UNMET: Falsifier 2 STANDS. The holder DID build a real per-tick latency bound — DefaultCullFoldTimeout=10s < 60s over both the DB read (SET LOCAL statement_timeout) and the filesystem scan, a watchdog child goroutine that returns the recovery goroutine on cullCtx.Done() even against a blocking syscall, skip-on-overrun, and L4 compute-then-commit (zero writes on a timed-out tick, no torn write) — and Falsifier 2 credits all of it plus G3/G4 no-regression. But the binding C-G2 verification gate requires a HANG test asserting '(ii) doctor does NOT emit recovery_sweep_cursor_wedged and stays ok:true', and the holder's B5 grounds that on the recovery cursor / last_sweep_at advancing (HOLDER.md:517-529, citing doctor.go:383,448,460). That mechanism is source-false: go/pkg/reads/doctor.go recoveryCursorQuietSince (469-479) keys the wedge on last_lane_advanced_at→started_at→created_at, never last_sweep_at, firing when claimable_job_count>0 and lane-quiet beyond 5m; the existing test doctor_recovery_cursor_test.go:34-44 proves a FRESH last_sweep_at with claimable_job_count=2 and a 10-minute-stale last_lane_advanced_at still returns ok:false. So the HANG test's binding scenario (a running run with claimable work and no lane progress) either fails the ok:true assertion as written or only passes by weakening the scenario away from the property that matters. The latency machinery is sound; the falsifiable PROOF of doctor-greenness is mismapped to current doctor semantics and is not delivered. G3 substrate correctness is STILL MET (Falsifier 2 re-confirmed: 0045 migration under go/pkg/db/sql/, GRANT no owner DDL/FK for >=27, both read+write authority-inventory rows, explicit-column reads, no SELECT *; §2 carried intact, no regression). G4 forward-compatibility is STILL MET (OQ1 sweep/peer writer, (kind,ref) ON CONFLICT + extensible candidacy_state CHECK, the §6 P0/P1+ deferral table all intact; no smuggled P1+ action — no tombstone/deletion/page/doctor class/run-admission). This is needs_revision, not reject: the architecture is sound, the substrate is excellent and fully cleared, the original two cycle-1 defects are materially closed (the rfc:0097 FP and the missing latency bound), and the two remaining residuals are narrow and concrete — a mechanical decision-successor parser + a cull-specific protected pathspec for G1''; a doctor-semantics-true HANG assertion (A/B control or an intentional last_sweep_at quiet-window change) for G2''. Per the gate's exhaustion rule, these residuals go to the operator for a fresh -v3 rather than a third in-run revision."
findings:
  - id: F2-G1-DECISION-SUCCESSOR-NONMECHANICAL
    severity: critical
    posture: tier1_false_positive
    status: converted_to_constraint
    challenge: "The re-derived true-positive set {decision:D267, decision:D081} is not mechanically derivable from the written predicate. Clause 1 fixes the decision structural status field as the second pipe-delimited state cell only (HOLDER.md:182-186) and clause 2 parses the successor from the status value up to the first sentence (HOLDER.md:195-204), but docs/decisions/decision-log.md:38 (D267) and :220 (D081) both have state cell = bare `superseded`, with the successor refs (`SUPERSEDED by D270`; `Superseded by D087/D094/D104`) living in the description / consequences cells, not the status value. A literal implementer withholds both (A1's true-positive set empty, exactness proof false); a looser implementer scanning other own-row cells nominates both. The spec defines no allowed successor-source cells, precedence, or sentence boundary, so the only two rows the corpus depends on read differently to two implementers — exactly the no-LLM, two-implementers-identical rule G1' was required to deliver."
    closest_acceptable_answer: "For kind=decision, define successor extraction as an explicit own-row-cell rule (which cells, precedence, sentence boundary, multi-ref split) distinct from the clause-1 status field; add table-driven cases proving D267/D081 nominate and a bare state-cell-only `superseded` decision is withheld; re-state the corpus under that rule."
    affected_invariants:
      - "RFC 0170 Acceptance Criterion 1: Tier-1 exact, two implementers read every corpus row identically with no LLM judgment"
      - "C-G1 verification.gate: the reconciled predicate nominates exactly the genuinely-dead set, every true positive independently confirmable"
    source_refs: ["dialogue:2"]
  - id: F2-G1-PROTECTED-PATHSPEC-EATS-RFC-ROOT
    severity: high
    posture: tier1_false_positive
    status: converted_to_constraint
    challenge: "Clause 3 protects any path matching 'every entry already in .check-docs-ignore' (HOLDER.md:270-277); live .check-docs-ignore:3 is `docs/rfcs/` wholesale (and :8 is docs/operator/workflows/). Taken literally, every RFC is protected before clause 2/4 evaluates, so no RFC can ever be nominated — contradicting §3's RFC corpus model (rfc:0028 withheld by clause 2; rfc:0097/0027/0039/0041 withheld by clause 4 — all of which presuppose RFCs are ELIGIBLE) and §2's kind='rfc' candidacy. One implementer imports .check-docs-ignore verbatim as the protected pathspec; another silently special-cases docs/rfcs/ out — a two-implementer split that flips the entire RFC candidacy surface."
    closest_acceptable_answer: "Replace 'every entry already in .check-docs-ignore' with a cull-specific protected pathspec, or explicitly subtract the actively-scanned roots (docs/rfcs/, docs/decisions/) from clause 3, and re-state the corpus."
    affected_invariants:
      - "G1 protected-root exclusion is well-defined and does not negate the scanned candidacy surface"
    source_refs: ["dialogue:2"]
  - id: F2-G2-HANG-TEST-DOCTOR-ASSERTION-MISMAPPED
    severity: critical
    posture: readonly_safety_latency
    status: converted_to_constraint
    challenge: "The binding C-G2 verification gate requires a HANG test asserting '(ii) doctor does NOT emit recovery_sweep_cursor_wedged and stays ok:true', and the holder's B5 grounds that on the recovery cursor / last_sweep_at advancing (HOLDER.md:517-529, citing doctor.go:383,448,460). Source-false: go/pkg/reads/doctor.go recoveryCursorQuietSince (469-479) keys the wedge on last_lane_advanced_at→started_at→created_at, never last_sweep_at; it fires when claimable_job_count>0 and that lane-quiet source is older than 5m (doctor.go:383,447-460). The existing test doctor_recovery_cursor_test.go:34-44 proves a FRESH last_sweep_at with claimable_job_count=2 and a 10-minute-stale last_lane_advanced_at still returns ok:false. So B5's binding scenario (running run, claimable work, no lane progress) either fails the ok:true assertion as written or only passes by weakening the scenario away from the property that matters. The latency bound (deadline + watchdog + compute-then-commit, no torn write) is credited; the doctor-green PROOF is mismapped to current doctor semantics and not delivered."
    closest_acceptable_answer: "Reframe B5 as an A/B control (the cull fold does not make recovery/lane doctor state worse than an identical no-cull baseline; Sweeps==2 proves goroutine release), OR intentionally change doctorRecoverySweepCursor + its test to key the quiet window on last_sweep_at and declare that as a doctor-semantics change with its own coverage. Keep the credited deadline/watchdog/compute-then-commit machinery."
    affected_invariants:
      - "C-G2 verification.gate (ii): a blocked cull scan provably does not produce recovery_sweep_cursor_wedged / keep doctor ok:true"
      - "G2: P0 triggers no doctor RED/amber attributable to the cull fold"
    source_refs: ["dialogue:3"]
constraints:
  - id: C-G1-DECISION-SUCCESSOR-EXACTNESS
    source_finding: F2-G1-DECISION-SUCCESSOR-NONMECHANICAL
    posture: tier1_false_positive
    severity: critical
    kind: gate
    binding: true
    text: "Make the kind=decision successor-extraction rule a pure greppable, no-LLM rule and reconcile the protected pathspec with the scanned RFC/decision roots. Define exactly which own-row cells (and in what precedence and to what sentence boundary) may carry the `superseded by <refs>` successor for a decision whose state cell is bare `superseded`, and the multi-ref split rule; state that other rows' cells are never successor sources. Replace clause 3's 'every entry already in .check-docs-ignore' with a cull-specific protected pathspec (or explicitly subtract docs/rfcs/ and docs/decisions/). Re-derive the G1 corpus so two implementers nominate exactly the genuinely-dead set (D267/D081 under the chosen rule) with zero hits on the preserved set."
    source_refs: ["dialogue:2"]
    verification:
      gate: "G1 corpus test over the live tree: the reconciled predicate (decision-successor rule + cull-specific protected pathspec) nominates exactly the genuinely-dead superseded set, with table-driven cases proving D267 and D081 nominate AND a bare state-cell-only `superseded` decision with no own-row successor prose is withheld, and zero hits on the preserved set; every published true positive independently confirmed to carry no live active-baseline inbound citation, and rfc:0097 (and rfc:0027/0039/0041) remain preserved."
    final_review_required: true
  - id: C-G2-HANG-DOCTOR-SEMANTICS
    source_finding: F2-G2-HANG-TEST-DOCTOR-ASSERTION-MISMAPPED
    posture: readonly_safety_latency
    severity: critical
    kind: gate
    binding: true
    text: "Make the HANG regression test's doctor assertion source-true against go/pkg/reads/doctor.go. Either (a) assert via an identical no-cull control that the blocking cull fold does not make recovery/lane doctor state worse than baseline (recovery_sweep_cursor_wedged keys on claimable_job_count + last_lane_advanced_at, which the cull fold never advances) while Sweeps==2 proves the recovery goroutine was released; OR (b) intentionally change doctorRecoverySweepCursor (and TestHandleDoctorFlagsRecoverySweepCursorWedgedClaimableRun) to key the quiet window on last_sweep_at, declared as an explicit doctor-semantics change with its own coverage. Keep the credited per-tick deadline (DB + filesystem), watchdog, skip-on-overrun, and L4 compute-then-commit machinery unchanged."
    source_refs: ["dialogue:3"]
    verification:
      gate: "HANG regression test in go/pkg/recovery (a BLOCKING non-returning DecayTickSweep scan, distinct from the panic test): assert (i) the next recovery tick still runs (Sweeps==2) and (ii) the cull fold does not worsen doctor recovery state versus a no-cull control (or, under option (b), that a fresh last_sweep_at keeps doctor ok:true under the changed predicate), and (iii) no cullable_entity write occurs on the timed-out tick; the test FAILS with the deadline removed."
    final_review_required: true
branches:
  tier1_false_positive: "blocked"
  readonly_safety_latency: "blocked"
  substrate_correctness: "cleared"
  forward_compatibility: "cleared"
---

# Collaboration Ledger — RFC 0170 P0 design v2 (cycle 1, the single revision round)

**Verdict: `needs_revision`.** This is v2's single proper revision round, so the
gate is now **exhausted**: the two residual constraints (**G1″**, **G2″**) route
to the operator for a fresh **`-v3`** run. The committer does **not** publish the
consolidated SPEC and **D271 is not ratified** this run.

The revision made **real progress on both binding constraints** — the cycle-1
`rfc:0097` false positive is fixed and a genuine cull-fold latency bound was built
— but **neither G1′ nor G2′ is fully discharged**, while the two cleared gates
(**G3**, **G4**) are confirmed **un-regressed**.

## Per-constraint disposition (the revision gate)

| Binding constraint | Result | Basis |
|---|---|---|
| **G1′ — `C-G1-CITATION-EXACTNESS`** (reconciled predicate + re-derived corpus) | **NOT DISCHARGED** | Falsifier 1 STANDS. The `rfc:0097` FP is fixed, but the replacement true-positive set `{D267, D081}` is not mechanically derivable: both decisions' state cells are bare `superseded` with successors in description/consequences cells the predicate never licenses, so two implementers split on the only two corpus rows A1 depends on. Plus clause 3's "every entry in `.check-docs-ignore`" literally protects `docs/rfcs/` wholesale. |
| **G2′ — `C-G2-CULL-FOLD-DEADLINE`** (latency bound + HANG test) | **NOT DISCHARGED** | Falsifier 2 STANDS. The deadline (DB + filesystem), watchdog, skip-on-overrun, and L4 compute-then-commit (no torn write) are all credited — but B5's binding "doctor stays `ok:true`" assertion is grounded on `last_sweep_at` advancing, which is **source-false**: the wedge keys on `last_lane_advanced_at` + `claimable_job_count`, never `last_sweep_at`. |

## Per-cleared-gate disposition (no regression)

| Cleared gate | Result | Basis |
|---|---|---|
| **G3 — substrate correctness** | **STILL MET** | Falsifier 2 re-confirmed §2 intact: `0045` runtime migration under `go/pkg/db/sql/`, `GRANT SELECT,INSERT,UPDATE` to `striatumd_rw` with no owner DDL/FK (≥27), both read+write authority-inventory rows, explicit-column reads (no `SELECT *`). |
| **G4 — forward-compatibility** | **STILL MET** | §5/§6 intact: OQ1 = sweep/peer writer; `(kind,ref)` ON CONFLICT + extensible `candidacy_state` CHECK; the P0/P1+ deferral table unchanged. No smuggled P1+ action (no tombstone/deletion/page/doctor class/run-admission). |

## Per-falsifier disposition

- **Falsifier 1 — Tier-1 exactness / false-positive lens (dialogue:2): MATERIAL,
  STANDING.** Source-verified at the run base: `docs/decisions/decision-log.md:38`
  (D267) state cell = bare `superseded`, successor `SUPERSEDED by D270` in the
  description cell; `:220` (D081) state cell = bare `superseded`, successor
  `Superseded by D087/D094/D104` in the consequences cell — so clauses 1+2 do not
  mechanically nominate either, and the corpus derives them from undefined own-row
  cells (a two-implementer split). `.check-docs-ignore:3` is `docs/rfcs/`
  wholesale, so clause 3 literally protects the entire RFC candidacy surface.
  **Credited:** the `rfc:0097` FP is fixed and `rfc:0027/0039/0041` are correctly
  preserved; no live artifact is auto-nominated. **Not rebutted → `C-G1`
  residual.**
- **Falsifier 2 — read-only safety / substrate-integrity lens (dialogue:3):
  MATERIAL, STANDING (the HANG-test doctor assertion).** Source-verified:
  `go/pkg/reads/doctor.go` `recoveryCursorQuietSince` (469-479) keys the wedge on
  `last_lane_advanced_at`→`started_at`→`created_at`, never `last_sweep_at`; the
  wedge fires on `claimable_job_count>0` + lane-quiet > 5m
  (`doctor.go:383,447-460`); the existing test
  `doctor_recovery_cursor_test.go:34-44` proves a **fresh** `last_sweep_at` with
  `claimable_job_count=2` and a 10-min-stale lane-advance still returns
  `ok:false`. So B5's `ok:true` mechanism is wrong. **Credited:** the latency bound
  (deadline DB+filesystem, watchdog goroutine release, compute-then-commit no torn
  write) and **G3/G4 no-regression**. **Not rebutted → `C-G2` residual.**

## Gate outcome — exhausted, route to `-v3`

Neither binding constraint is fully discharged, so this is **not** a clearing
verdict. Because v2 is the single allowed revision round, a `needs_revision` here
**exhausts the gate**; per the SEED's gate rule the residual findings route to the
**operator for a fresh `-v3`** rather than a third in-run revision (do **not**
ratchet, do **not** commit the consolidated SPEC, do **not** ratify D271).

The two residuals are narrow and in-P0-buildable:

- **G1″ (`C-G1-DECISION-SUCCESSOR-EXACTNESS`):** make the `kind=decision`
  successor-extraction a pure greppable own-row-cell rule (allowed cells,
  precedence, sentence boundary, multi-ref split) and replace clause 3's
  `.check-docs-ignore` import with a cull-specific protected pathspec (or subtract
  `docs/rfcs/`/`docs/decisions/`); re-state the corpus with table-driven `D267`/
  `D081`-nominate and bare-state-cell-withheld cases.
- **G2″ (`C-G2-HANG-DOCTOR-SEMANTICS`):** make B5's doctor assertion source-true —
  either an A/B no-cull control proving the fold does not worsen recovery/lane
  doctor state (while `Sweeps==2` proves goroutine release), or an intentional,
  separately-covered doctor-semantics change keying the quiet window on
  `last_sweep_at`. The credited deadline/watchdog/compute-then-commit machinery is
  kept as-is.

This is `needs_revision`, **not `reject`** — the architecture is sound, the
substrate is fully cleared, and both original cycle-1 defects (the `rfc:0097` FP
and the missing latency bound) are materially closed; only the exactness mechanism
and the doctor-green proof remain.
