---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
author: adjudicator-author-001
workflow: "rfc-0170-p0-design-v3"
run_id: "run_01b5b2d0ac51282cecbe6f516f10f8e3"
cycle: 1
topic: "RFC 0170 P0 falsifiable implementation SPEC (self-culling repository: the Tier-1 cullable_entity candidacy substrate + read-only DecayTickSweep) — v3 REVISION gate: discharge G1'' (mechanical kind=decision successor-extraction rule + cull-specific protected pathspec) and G2'' (source-true HANG-test doctor assertion) without regressing the credited latency machinery, the clause-4 active-baseline citation rule, or the cleared G3/G4"
participants:
  - holder
  - falsifier_1
  - falsifier_2
  - adjudicator
entries:
  - kind: claim
    by: holder
    refs: ["dialogue:1"]
    text: "v3 revising-holder SPEC. Carries INTACT from v2: §2 substrate (migration 0045 under go/pkg/db/sql/, both authority-inventory rows, striatumd_rw GRANT with no owner DDL/FK for >=27, no SELECT *), the clause-4 active-baseline inbound-citation rule (4a-4f) preserving rfc:0097/0027/0039/0041, the entire latency machinery (DefaultCullFoldTimeout=10s over both the DB read and the filesystem scan, watchdog child goroutine, single-in-flight skip-on-overrun, L4 compute-then-commit), §5 (OQ1 sweep/peer writer), and §6 (P0/P1+ deferral table). Changes EXACTLY two things. G1'' discharge: §3 clause 2 gains an explicit pure-greppable kind=decision successor-extraction rule (allowed own-row cells = cell 3 then cell 5 by precedence, the \\bsupersed(?:ed|es) by\\s+<reflist> regex, sentence/clause boundary, multi-ref split on /,, and ' and '; cells 1/2/4/6 and other rows' cells never sources) yielding D267->{D270} and D081->{D087,D094,D104}; and §3 clause 3 replaces the wholesale .check-docs-ignore import with a cull-specific protected pathspec that excludes the actively-scanned roots docs/rfcs/ and docs/decisions/. G2'' discharge: §4 keeps the latency machinery unchanged and reframes B5 onto an A/B no-cull control — the binding assertion becomes 'the blocking cull fold does not make recovery/lane doctor state worse than an identical no-cull baseline' (because recovery_sweep_cursor_wedged keys on claimable_job_count + last_lane_advanced_at, which the cull fold never writes), proven alongside Sweeps==2 (recovery goroutine released) and no cullable_entity write on the timed-out tick, failing with the deadline removed. Drops the v2 source-false last_sweep_at claim."
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "G1'' does NOT discharge. The kind=decision successor-extraction RULE is credited as concrete enough to run by hand (D267 extracts {D270} from cell 3, D081 extracts {D087,D094,D104} from cell 5, D084 and the other bare state-cell-only `superseded` rows with no own-row `superseded by` clause are withheld by clause 2, D174 is parseable via cell 5 but withheld by clause 4 because live RFC 0109 still cites it — no new false positive in the decision-cell rule). The standing defect is narrower and lives in clause 3's protected pathspec: it still contains the unconditional catch-all bullet 'any path referenced by an open GitHub issue' (HOLDER.md:373) with NO subtraction for docs/rfcs/** or docs/decisions/**, directly contradicting the holder's simultaneous claim (HOLDER.md:380-383) that docs/rfcs/*.md and docs/decisions/decision-log.md are NOT in the pathspec. In live repository state, open issues reference both real paths — #585 (open) references docs/decisions/decision-log.md (D261) and #615 (open) references docs/rfcs/0170-...md (verified via the GitHub API during the lane). So under the holder's own clause 3 the active decision log is protected RIGHT NOW: D267 (decision-log.md:38) and D081 (decision-log.md:220) are rows in a protected path, get reachable_from_root=true, and fail clause 3 before they can be nominated, making the published true-positive set {decision:D267, decision:D081} non-derivable. This is a two-implementer split (one protects docs/decisions/ via the open-issue bullet -> true-positive set empty; another follows the prose 'NOT in the pathspec' -> nominates both), and the open-issue dependency is dynamic/external, not a tree-local greppable cull predicate. Fix: remove the open-issue bullet from the cull predicate (keep it an operator advisory outside cullable_entity), OR explicitly subtract docs/rfcs/** and docs/decisions/** from it, OR replace it with a checked-in cull-specific protected-path manifest; then re-run the G1 corpus under that precedence. Verdict: needs_revision."
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "G2'' does NOT fully discharge. The v3 holder correctly fixed the v2 source-false last_sweep_at claim — current doctor.go keys the wedge on claimable_job_count + last_lane_advanced_at/started_at/created_at, never last_sweep_at (doctor.go:383-460,469-479), and the existing test proves a fresh last_sweep_at can still be red (doctor_recovery_cursor_test.go:33-73). B5 is also right that a timed-out cull tick must do no cullable_entity write and that Sweeps==2 proves eventual goroutine release. The remaining gap is the no-worse-than-baseline part. The holder treats the cull fold as unable to affect doctor because it writes no scheduler_cursors/lane/run rows (HOLDER.md:600-611,671-680). That misses a source-true carrier: the cull fold is SYNCHRONOUS after innerSweep inside SweepOnce, and RunScheduler calls wait(ctx, interval) only AFTER SweepOnce returns (scheduler.go:55-80; daemon fold precedent main.go:883-897). So a cull fold that times out at DefaultCullFoldTimeout delays the START of the next recovery sweep by that timeout versus the no-cull control. That matters because doctor does not read live queue/session state — it reads the STORED recovery cursor JSON: doctorRecoverySweepCursor selects c.last_result_json->>'claimable_job_count' and ->>'last_lane_advanced_at' from striatumd.scheduler_cursors (doctor.go:383-390), refreshed ONLY when the next recovery sweep upserts the cursor (upsertSchedulerCursor -> recoveryCursorResultWithLatch recomputes claimable/lane from live state, sweep.go:246-263,284-358). Delaying the next sweep delays the refresh that would clear a stale claimable_job_count>0 / stale last_lane_advanced_at. Concrete case: at T0 the stored cursor says claimable=2, last_lane_advanced_at=T0-4m59s (just below the 5m wedge); a lane then completes work so the next latch would be non-wedged; in the no-cull control the next sweep refreshes the cursor before a doctor read at T0+5m+5s, but in the cull variant the prior tick's blocked fold burned DefaultCullFoldTimeout so the refresh has not yet happened, doctor reads the stale wedged cursor and emits recovery_sweep_cursor_wedged. The cull fold did not write a bad cursor value; it made doctor worse by delaying the next refresh. B5 misses this because it uses an immediate fake Wait and asserts Sweeps==2 after both A and B (HOLDER.md:657-670) — that proves eventual release, not same-wall-clock no-worse-than-baseline. Fix: a same-wall-clock A/B test (fake clock modeling SweepOnce duration + wait(interval); assert the cull variant has not left doctor red while the control is green at the same instant near the threshold), OR change the design so the cull fold cannot postpone the cursor refresh, OR weaken B5 to a bounded-delay property for the adjudicator to weigh."
  - kind: nomination
    by: falsifier_1
    refs: ["dialogue:2"]
    text: "Credited (G1 progress + no-regression, recorded but not gate-clearing): the kind=decision successor-extraction rule itself is now mechanical and correct — D267 uses cell 3, D081 uses cell 5, D084 and the other bare state-cell-only `superseded` rows without a `superseded by` clause are withheld by clause 2, and no new false positive was found in the fixed decision-cell rule. The old wholesale .check-docs-ignore import of docs/rfcs/ is removed from the static root list and the frozen/provenance roots (docs/records/_frozen/**, docs/research/**, docs/dogfood/**, docs/handoffs/**, docs/operator/plans/**, docs/operator/workflows/**, examples/**, prompts/**) remain protected. The standing defect is the single residual open-issue catch-all bullet in clause 3, not the decision parser and not a surviving citation false positive."
  - kind: nomination
    by: falsifier_2
    refs: ["dialogue:3"]
    text: "Credited (G2 progress + G3/G4 no-regression, recorded): the v2 source-false last_sweep_at mechanism is correctly dropped; the latency machinery is sound and unchanged (per-tick deadline over both halves, watchdog child goroutine, skip-on-overrun, L4 compute-then-commit -> zero writes / no torn write on a timed-out tick), and B5 correctly asserts Sweeps==2 (eventual goroutine release) and no cullable_entity write on the timed-out tick. G3/G4 appear un-regressed: the runtime 0045_cullable_entity.sql shape, both authority-inventory rows, explicit-column cullable_entity reads (no SELECT *), the clause-4 preserved set, and no P1+ tombstone/page/admission action are all retained. The standing defect is the unmodeled synchronous-refresh-delay carrier in the no-worse-than-baseline assertion, not the latency bound or a substrate regression."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:2"]
    text: "G1'' (residual, routes to v4): the kind=decision successor-extraction rule is DISCHARGED (pure greppable, named own-row cells 3->5 by precedence, fixed regex + sentence boundary + multi-ref split, other cells/rows never sources; D267->{D270}, D081->{D087,D094,D104}, D084/bare-`superseded` fixture withheld). The cull-specific protected pathspec is NOT discharged: clause 3 retains the unconditional dynamic bullet 'any path referenced by an open GitHub issue' with no subtraction of docs/rfcs/ / docs/decisions/, which (i) is not a tree-local greppable cull predicate and (ii) over-exposes the active candidacy roots (it re-protects docs/decisions/decision-log.md, where D267/D081 live, in current live issue state), contradicting the holder's own 'NOT in the pathspec' claim and rendering the published true-positive set non-derivable. Make clause 3 a fully static, tree-local pathspec: remove the open-issue bullet from the cull predicate (operator advisory only), or explicitly subtract docs/rfcs/** and docs/decisions/** from it, or replace it with a checked-in cull-specific protected-path manifest; then re-derive the G1 corpus under that precedence so two implementers read every row identically (D267/D081 nominate, rfc:0097/0027/0039/0041 preserved, frozen/scaffold roots still protected)."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:3"]
    text: "G2'' (residual, routes to v4): the v2 source-false last_sweep_at claim is correctly retired and the latency machinery (deadline + watchdog + skip-on-overrun + L4 compute-then-commit) is credited and must stay unchanged. The reframed B5 A/B control does NOT prove the binding 'no-worse-than-baseline doctor state' property: doctor reads the PERSISTED scheduler_cursors.last_result_json (doctor.go:383-390), refreshed only by the next recovery sweep upsert (sweep.go:246-263); the cull fold runs synchronously inside SweepOnce before RunScheduler's wait(interval) (scheduler.go:55-80), so a hung fold that burns DefaultCullFoldTimeout delays the next cursor refresh and opens a same-wall-clock window where doctor reads a staler (wedged) cursor under the cull variant than under the no-cull control near the 5m threshold. B5 as written (immediate fake Wait, assert Sweeps==2, identical written cursor VALUES, static-fixture reads companion) models eventual release and value-identity, not refresh-timing, so it can pass while the same-instant doctor outcome is worse under A. Discharge with a same-wall-clock A/B test (fake clock modeling SweepOnce duration + wait(interval); assert the cull variant is not left doctor-red while the control is green at the same instant near the threshold and that it FAILS with the deadline removed), OR change the design so the cull fold cannot postpone the cursor refresh (e.g. move the cursor upsert ahead of the fold, or run the fold off the wait-gating path), OR — if a bounded refresh-delay is deemed acceptable — declare and bound that explicitly as a doctor-semantics decision with its own coverage rather than asserting an unqualified no-worse property."
verdict: "needs_revision"
rationale: "needs_revision, and because this is v3's single proper revision round, the gate is EXHAUSTED: the two residual constraints (G1'', G2'') route to the OPERATOR for a fresh -v4 — do not ratchet, do not let the committer publish the consolidated SPEC, do not ratify D271. This is NOT a clearing verdict because NEITHER binding residual is fully discharged, even though both made real, creditable progress and the credited/cleared properties (the latency machinery, the clause-4 preserved set, G3 substrate, G4 forward-compat) are confirmed UN-REGRESSED. G1'' is UNMET: Falsifier 1 STANDS. The kind=decision successor-extraction rule IS discharged — it is now a pure greppable, no-LLM rule (named own-row cells 3 then 5 by precedence, the \\bsupersed(?:ed|es) by\\s+<reflist> regex, sentence/clause boundary, multi-ref split), Falsifier 1 ran it by hand and confirmed D267->{D270} (cell 3), D081->{D087,D094,D104} (cell 5), and D084 plus the bare state-cell-only `superseded` fixture are withheld by clause 2 with no new false positive. But G1'' requires BOTH the rule AND a cull-specific protected pathspec, and the pathspec is NOT discharged: clause 3 (HOLDER.md:373) retains the unconditional bullet 'any path referenced by an open GitHub issue' with NO subtraction of docs/rfcs/ / docs/decisions/, which directly contradicts the holder's own clause-3 claim (HOLDER.md:380-383) that those two roots are NOT in the pathspec. The bullet (i) is a dynamic, GitHub-API-dependent dependency, NOT a tree-local greppable cull predicate (a path's protection can flip with external issue-body text), and (ii) over-exposes the active candidacy surface: in current live issue state open #585 references docs/decisions/decision-log.md and open #615 references docs/rfcs/0170-...md, so the active decision log is protected RIGHT NOW and D267/D081 (rows at decision-log.md:38 and :220) get reachable_from_root=true and fail clause 3 before nomination — the published true-positive set is non-derivable. This is exactly the 'pathspec that still negates or over-exposes the surface' condition the gate names as G1'' UNMET, and a two-implementer split internal to the SPEC text. G2'' is UNMET: Falsifier 2 STANDS. The holder correctly retired the v2 source-false last_sweep_at claim (current doctor.go keys the wedge on claimable_job_count + last_lane_advanced_at, never last_sweep_at — doctor.go:383-460,469-479, confirmed against source) and kept the credited latency machinery intact, and B5 correctly asserts Sweeps==2 (release) and no cullable_entity write on the timed-out tick. But the reframed A/B control does not PROVE the binding 'cull fold does not make doctor worse than baseline' property. Source-verified carrier: doctor reads the PERSISTED scheduler_cursors.last_result_json (doctor.go:383-390), refreshed only by the next recovery sweep's upsertSchedulerCursor->recoveryCursorResultWithLatch (sweep.go:246-263,284-358); the cull fold runs synchronously inside SweepOnce, and RunScheduler calls wait(interval) only AFTER SweepOnce returns (scheduler.go:55-80), so a hung fold burning DefaultCullFoldTimeout delays the next cursor refresh and opens a same-wall-clock window where doctor reads a staler (wedged) cursor under the cull variant than under the control near the 5m threshold. B5 as designed (immediate fake Wait, assert Sweeps==2 and identical written cursor VALUES, static-fixture reads companion) tests eventual release and value-identity — NOT refresh-timing — so it can pass while the same-instant doctor outcome is worse under A. The holder's 'identical wedge inputs' argument is true for the values WRITTEN but not for their refresh STALENESS, which the synchronous fold provably affects. So the assertion still mismaps to current scheduler+doctor semantics; G2'' is not discharged. NO REGRESSION holds: the credited latency machinery (DefaultCullFoldTimeout=10s over both halves, watchdog, skip-on-overrun, L4 compute-then-commit — zero writes on a timed-out tick) is intact and both falsifiers credit it; the clause-4 active-baseline citation rule preserving rfc:0097/0027/0039/0041 is carried intact (Falsifier 1 re-confirmed the preserved set and found no citation false positive, incl. D174 preserved by live RFC 0109); G3 substrate is intact (runtime 0045_cullable_entity.sql under go/pkg/db/sql/, both read+write authority-inventory rows, GRANT SELECT,INSERT,UPDATE to striatumd_rw with no owner DDL/FK >=27, explicit-column reads, no SELECT *); G4 forward-compat is intact (OQ1 sweep/peer writer, (kind,ref) ON CONFLICT + extensible candidacy_state CHECK, the §6 deferral table; no smuggled P1+ tombstone/deletion/page/doctor class/run-admission). This is needs_revision, NOT reject: the architecture is sound, the substrate is fully cleared, the decision-successor parser is now mechanical, the latency machinery is sound, and the two residuals are narrow and concrete — a fully static tree-local protected pathspec (drop/subtract the open-issue catch-all) for G1''; a same-wall-clock A/B HANG control (or a design change preventing refresh-postponement, or an explicitly-bounded/declared delay) for G2''. Per the gate's exhaustion rule, both residuals route to the operator for a fresh -v4 rather than a third in-run revision."
findings:
  - id: F3-G1-OPEN-ISSUE-PATHSPEC-NEGATES-CANDIDACY-SURFACE
    severity: critical
    posture: tier1_false_positive
    status: converted_to_constraint
    challenge: "Clause 3's cull-specific protected pathspec retains the unconditional bullet 'any path referenced by an open GitHub issue' (HOLDER.md:373) with no subtraction of docs/rfcs/** or docs/decisions/**, contradicting the holder's clause-3 claim (HOLDER.md:380-383) that those roots are NOT in the pathspec. The bullet is (i) a dynamic GitHub-API dependency, not a tree-local greppable cull predicate, and (ii) over-exposing: in current live issue state open #585 references docs/decisions/decision-log.md and open #615 references docs/rfcs/0170-...md, so the active decision log is protected now and D267 (decision-log.md:38) / D081 (decision-log.md:220) get reachable_from_root=true and fail clause 3 before nomination, making the published true-positive set {decision:D267, decision:D081} non-derivable. A two-implementer split internal to the SPEC: one protects docs/decisions/ via the open-issue bullet (true-positive set empty); another follows the 'NOT in the pathspec' prose (nominates both)."
    closest_acceptable_answer: "Make clause 3 a fully static, tree-local pathspec: remove the open-issue bullet from the cull predicate (keep issue-linked preservation as an operator advisory outside cullable_entity), OR explicitly subtract docs/rfcs/** and docs/decisions/** from it, OR replace it with a checked-in cull-specific protected-path manifest; then re-derive the G1 corpus under that precedence with a fixture proving D267/D081 are NOT protected while frozen/provenance roots stay protected."
    affected_invariants:
      - "RFC 0170 Acceptance Criterion 1: Tier-1 exact, two implementers read every corpus row identically with no LLM/external judgment"
      - "G1'' verification.gate: the protected pathspec is cull-specific and does not negate or over-expose the scanned RFC/decision candidacy surface; D267/D081 nominate; rfc:0097/0027/0039/0041 preserved"
    source_refs: ["dialogue:2"]
  - id: F3-G2-HANG-TEST-REFRESH-DELAY-CARRIER-UNMODELED
    severity: critical
    posture: readonly_safety_latency
    status: converted_to_constraint
    challenge: "The reframed B5 A/B control does not prove the binding 'cull fold does not make recovery/lane doctor state worse than baseline' property. doctor reads the PERSISTED scheduler_cursors.last_result_json (doctor.go:383-390), refreshed only by the next recovery sweep upsert (sweep.go:246-263,284-358); the cull fold runs synchronously inside SweepOnce, and RunScheduler calls wait(interval) only after SweepOnce returns (scheduler.go:55-80), so a hung fold burning DefaultCullFoldTimeout delays the next cursor refresh and opens a same-wall-clock window where doctor reads a staler (wedged) cursor under the cull variant than under the no-cull control near the 5m threshold. B5 (immediate fake Wait, assert Sweeps==2 and identical written cursor values, static-fixture reads companion) tests eventual release and value-identity, not refresh-timing, so it can pass while the same-instant doctor outcome is worse under A. The 'identical wedge inputs' argument is true for written values but not for their refresh staleness."
    closest_acceptable_answer: "A same-wall-clock A/B test (fake clock modeling SweepOnce duration + wait(interval); assert the cull variant is not left doctor-red while the control is green at the same instant near the threshold, and that it FAILS with the deadline removed), OR a design change so the cull fold cannot postpone the cursor refresh (e.g. upsert the cursor ahead of the fold / move the fold off the wait-gating path), OR an explicitly-declared and bounded refresh-delay doctor-semantics decision with its own coverage instead of an unqualified no-worse claim. Keep the credited deadline/watchdog/skip-on-overrun/compute-then-commit machinery unchanged."
    affected_invariants:
      - "C-G2 verification.gate (ii): a blocked cull scan provably does not worsen recovery_sweep_cursor_wedged / doctor state versus a no-cull baseline"
      - "G2: P0 triggers no doctor RED/amber attributable to the cull fold"
    source_refs: ["dialogue:3"]
constraints:
  - id: C-G1-DECISION-SUCCESSOR-EXACTNESS
    source_finding: F3-G1-OPEN-ISSUE-PATHSPEC-NEGATES-CANDIDACY-SURFACE
    posture: tier1_false_positive
    severity: critical
    kind: gate
    binding: true
    text: "Discharged in part (the kind=decision successor-extraction rule is now pure greppable and correct: named own-row cells 3->5 by precedence, fixed regex, sentence boundary, multi-ref split; D267->{D270}, D081->{D087,D094,D104}, bare state-cell-only `superseded` withheld). Residual to v4: make clause 3's protected pathspec fully static and tree-local — remove the 'any path referenced by an open GitHub issue' bullet from the cull predicate, or explicitly subtract docs/rfcs/** and docs/decisions/**, or replace it with a checked-in cull-specific manifest — so the active candidacy surface is neither negated nor over-exposed, and re-derive the G1 corpus so two implementers nominate exactly {D267, D081} with zero hits on the preserved set (rfc:0097/0027/0039/0041 preserved)."
    source_refs: ["dialogue:2"]
    verification:
      gate: "G1 corpus test over the live tree under a fully static tree-local protected pathspec: the predicate nominates exactly the genuinely-dead superseded set (D267/D081 under the credited decision rule) with table-driven cases proving D267/D081 nominate AND a bare state-cell-only `superseded` decision is withheld, AND zero hits on the preserved set; the protected pathspec protects docs/records/_frozen/** and the design-scaffold/fixture roots while leaving docs/rfcs/ and docs/decisions/ eligible; no path classification depends on external (open-issue) state."
    final_review_required: true
  - id: C-G2-HANG-DOCTOR-SEMANTICS
    source_finding: F3-G2-HANG-TEST-REFRESH-DELAY-CARRIER-UNMODELED
    posture: readonly_safety_latency
    severity: critical
    kind: gate
    binding: true
    text: "Discharged in part (the v2 source-false last_sweep_at claim is retired; the latency machinery + Sweeps==2 release + no-write-on-timed-out-tick are credited). Residual to v4: prove the binding no-worse-than-baseline doctor property against the SOURCE-TRUE refresh-delay carrier — the cull fold runs synchronously inside SweepOnce before RunScheduler's wait(interval), and doctor reads the persisted scheduler_cursors.last_result_json refreshed only by the next sweep upsert, so a hung fold can postpone the next cursor refresh. Use a same-wall-clock A/B test (fake clock modeling SweepOnce duration + wait(interval)), OR change the design so the fold cannot postpone the refresh, OR declare and bound the refresh-delay explicitly. Keep the credited deadline/watchdog/skip-on-overrun/L4 compute-then-commit machinery unchanged."
    source_refs: ["dialogue:3"]
    verification:
      gate: "HANG regression test in go/pkg/recovery (a BLOCKING non-returning DecayTickSweep scan, distinct from the panic test): assert (i) Sweeps==2 (recovery goroutine released), (ii) at the SAME wall-clock instant near the 5m threshold the cull variant does not leave doctor recovery_sweep_cursor_wedged while the no-cull control is green (modeling SweepOnce duration + wait), and (iii) no cullable_entity write on the timed-out tick; the test FAILS with the deadline removed. (Or, under a declared design/semantics change, the equivalent coverage that the cull fold provably cannot postpone the persisted-cursor refresh.)"
    final_review_required: true
branches:
  tier1_false_positive: "blocked"
  readonly_safety_latency: "blocked"
  substrate_correctness: "cleared"
  forward_compatibility: "cleared"
---

# Collaboration Ledger — RFC 0170 P0 design v3 (cycle 1, the single revision round)

**Verdict: `needs_revision`.** This is v3's single proper revision round, so the
gate is now **exhausted**: the two residual constraints (**G1''**, **G2''**) route
to the **operator for a fresh `-v4`** run. The committer does **not** publish the
consolidated SPEC and **D271 is not ratified** this run.

The revision made **real, creditable progress on both residuals** — the
`kind=decision` successor-extraction rule is now mechanical and the v2 source-false
`last_sweep_at` doctor claim is retired — but **neither G1'' nor G2'' is fully
discharged**, while the credited/cleared machinery (the latency bound, the clause-4
preserved set, **G3**, **G4**) is confirmed **un-regressed**.

## Per-residual disposition (the revision gate)

| Binding residual | Result | Basis |
|---|---|---|
| **G1'' — `C-G1-DECISION-SUCCESSOR-EXACTNESS`** (mechanical decision rule + cull-specific protected pathspec) | **NOT DISCHARGED** | Falsifier 1 STANDS. The decision-successor RULE is discharged (D267→{D270} cell 3, D081→{D087,D094,D104} cell 5, D084/bare-`superseded` withheld, no new FP). But clause 3 retains the unconditional dynamic bullet **"any path referenced by an open GitHub issue"** (HOLDER.md:373) with no subtraction of `docs/rfcs/`/`docs/decisions/`, contradicting the holder's own "NOT in the pathspec" claim and re-protecting the active decision log (open #585→`docs/decisions/decision-log.md`, open #615→`docs/rfcs/0170-…md`), so D267/D081 fail clause 3 and the true-positive set is non-derivable. Pathspec still negates/over-exposes the surface. |
| **G2'' — `C-G2-HANG-DOCTOR-SEMANTICS`** (source-true HANG doctor assertion) | **NOT DISCHARGED** | Falsifier 2 STANDS. The v2 `last_sweep_at` claim is correctly retired and the latency machinery + `Sweeps==2` + no-write-on-timeout are credited. But the A/B control does not **prove** no-worse-than-baseline: doctor reads the **persisted** `scheduler_cursors.last_result_json` (doctor.go:383-390) refreshed only by the next sweep upsert (sweep.go:246-263); the cull fold is synchronous-before-`wait(interval)` (scheduler.go:55-80), so a hung fold delays the next cursor refresh and can leave doctor worse under A than B at the same wall-clock instant near the 5m threshold. B5 tests eventual release + value-identity, not refresh-timing. |

## Per-cleared/credited-property disposition (no regression)

| Property | Result | Basis |
|---|---|---|
| **Latency machinery** (deadline DB+FS, watchdog, skip-on-overrun, L4 compute-then-commit) | **STILL MET** | Carried intact in §4; both falsifiers credit it (zero writes / no torn write on a timed-out tick; goroutine released). |
| **Clause-4 active-baseline citation rule** (preserves `rfc:0097`/`0027`/`0039`/`0041`) | **STILL MET** | §3 clause 4 carried intact; Falsifier 1 re-confirmed the preserved set and found no citation false positive (incl. D174 preserved by live RFC 0109). |
| **G3 — substrate correctness** | **STILL MET** | §2 intact: `0045_cullable_entity.sql` runtime migration under `go/pkg/db/sql/`, both read+write authority-inventory rows, `GRANT SELECT,INSERT,UPDATE` to `striatumd_rw` with no owner DDL/FK (≥27), explicit-column reads (no `SELECT *`). |
| **G4 — forward-compatibility** | **STILL MET** | §5/§6 intact: OQ1 sweep/peer writer; `(kind,ref)` ON CONFLICT + extensible `candidacy_state` CHECK; the P0/P1+ deferral table. No smuggled P1+ action (no tombstone/deletion/page/doctor class/run-admission). |

## Per-falsifier disposition

- **Falsifier 1 — Tier-1 exactness / false-positive lens (dialogue:2): MATERIAL,
  STANDING.** The `kind=decision` parser is genuinely fixed (mechanical, run by
  hand, no new FP). The single standing defect is clause 3's leftover dynamic
  open-issue catch-all, which is not a tree-local greppable cull predicate and
  re-protects the active candidacy roots in live issue state — making the published
  `{decision:D267, decision:D081}` non-derivable under the holder's own pathspec.
  **Not rebutted → `C-G1` residual.**
- **Falsifier 2 — read-only safety / no-regression lens (dialogue:3): MATERIAL,
  STANDING (the HANG-test no-worse assertion).** The `last_sweep_at` repair is
  source-true and credited; the gap is the unmodeled **synchronous refresh-delay
  carrier**: doctor reads persisted cursor JSON refreshed only on the next sweep,
  and the cull fold (synchronous before `wait`) can postpone that refresh, so a
  bounded fold delay can make doctor worse than the no-cull control at the same
  instant — which B5's `Sweeps==2` / value-identity shape does not test. Latency
  machinery and G3/G4 credited un-regressed. **Not rebutted → `C-G2` residual.**

## Gate outcome — exhausted, route to `-v4`

Neither binding residual is fully discharged, so this is **not** a clearing
verdict. Because v3 is the single allowed revision round, a `needs_revision` here
**exhausts the gate**; per the SEED's gate rule the residual findings route to the
**operator for a fresh `-v4`** rather than a third in-run revision (do **not**
ratchet, do **not** commit the consolidated SPEC, do **not** ratify D271).

The two residuals are narrow and in-P0-buildable:

- **G1'' (`C-G1-DECISION-SUCCESSOR-EXACTNESS`):** keep the now-mechanical
  decision-successor rule; make clause 3's protected pathspec **fully static and
  tree-local** — drop the "any path referenced by an open GitHub issue" bullet from
  the cull predicate (operator advisory only), or explicitly subtract `docs/rfcs/**`
  and `docs/decisions/**`, or replace it with a checked-in manifest — and re-derive
  the corpus so two implementers nominate exactly `{D267, D081}` with the preserved
  set untouched.
- **G2'' (`C-G2-HANG-DOCTOR-SEMANTICS`):** keep the credited latency machinery; make
  the HANG doctor assertion bind the **same-wall-clock** no-worse property (a fake
  clock modeling `SweepOnce` duration + `wait(interval)`), **or** change the design
  so the cull fold cannot postpone the persisted-cursor refresh, **or** declare and
  bound the refresh-delay explicitly — instead of an unqualified no-worse claim that
  `Sweeps==2` and value-identity do not establish.

This is `needs_revision`, **not `reject`** — the architecture is sound, the
substrate and forward-compat are cleared, the decision-successor parser is now
mechanical, and the latency machinery is sound; only a fully static protected
pathspec and a same-wall-clock doctor proof remain.
