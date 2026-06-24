---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
author: adjudicator-author-001
workflow: "rfc-0166-design"
run_id: "run_671c0660718725efe4f2d2c72fc7f979"
cycle: 1
topic: "RFC 0166 P0 falsifiable SPEC — the sealed-progress silence budget (wedged_no_sealed_progress rung: derived clock + AND no-false-kill + novelty reset + telomere ladder; #576)"
participants:
  - holder
  - falsifier_1
  - falsifier_2
  - adjudicator
entries:
  - kind: claim
    by: holder
    refs: ["dialogue:1"]
    text: >-
      The holder hardens the RFC 0166 four-part direction into build-bearing assertions and ratifies the AND no-false-kill core. Part 1: a derived (not stored) per-session clock floor = GREATEST(jobSealedProgressAt, currentActiveLeaseAcquiredAt, started_at), restart-reproducible from durable rows, with a new StallSealedProgress class evaluated only in recoverStuckJobs. Part 2: the rung fires iff sealedSilenceBreached AND sessionliveness.ToolProgressWedged (the exact #324 predicate, promoted to exported); a single-final-seal lane making real tool calls or inside a tool call is spared, and the AND strictly narrows #324. Part 3: the telomere-counter reset is gated on a strict increase of a novelty cursor (count distinct content_sha256 of MILESTONE-bearing artifacts, count sealed verdicts, highest satisfied required-artifact milestone index) persisted on job_recovery_state, with junk-blob hardening scoping the content_sha256 dimension to declared expected_artifacts. Part 4: first breach auto-requeues one fresh session via the existing transfer_requeue/requeueJobSameAttempt path; requeue_count resets only on novelty, shortens toward maxRequeues/unsealedRequeueBudget (telomere), then escalates needs_operator once (idempotent) with a copy-pasteable recovery verb. P0 ships shadow-first: SealedSilenceSeconds defaults 0 (disabled, advisory doctor/dashboard signal); the escalating action arms only on explicit opt-in.
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: >-
      Part 2 no-false-kill challenge. The AND does not PROVE no-false-kill for a legitimately-healthy single-final-seal lane. The holder protects only lanes that are inside a tool call or issue tool calls more often than ToolProgressSeconds; a lane in a long legitimate planning/model-generation interval (T0+20m, last tools/call at T0+5m, PTY keeping the lease warm, zero seals) satisfies BOTH sealedSilenceBreached AND toolWedged once SealedSilenceSeconds <= 1200 is armed, so fire == true while the lane is healthy — the #145 CASE-2 mid-work kill. The hard-core proof itself narrows protection to "making real tool calls or inside a tool call" and routes the long fully-silent think to operator-grant / calibration, both out of P0; calibration chooses where false-positive risk starts, it does not prove safety. Required revision: either narrow the safety claim to tool-fresh / in-tool / heartbeating lanes and name the long-silent interval an accepted product risk, OR add a build-bearing reprieve for legitimate non-sealed non-tool intervals, with a falsification test for a single-final-seal lane whose last tool call is older than ToolProgressSeconds but whose supervised process is alive and working.
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: >-
      Part 3/4 raw-clock challenge — DECISIVE. The holder leaves the Part 1 floor as raw jobSealedProgressAt = GREATEST(max(artifacts.created_at), max(verdicts.created_at)); the novelty cursor gates ONLY the telomere counter reset, not the clock. A hostile lane publishes a fresh undeclared in-scope artifact (allowed kind, fresh logical_name junk_N, distinct content_sha256) every SealedSilenceSeconds - epsilon. The holder's own hardening means junk_N does NOT advance the cursor — correct — but each row DOES advance max(artifacts.created_at), so the floor advances, sealedSilenceBreached stays false, the rung NEVER FIRES, requeue_count never climbs, and the telomere ladder never engages. The attack is durable and survives daemon restart (junk rows + created_at persist). A second event-gated reset surface: the RFC 0131 confidence gate computes progressAdvanced from raw jobSealedProgressAt(...).After(windowStart), so a junk row also resets consecutive_silent_sweeps. This violates the RFC's own Part 3 ("the deadline consumes novelty, not events; a re-published identical hash does not grow the cursor (and the clock)"). Required revision: one novelty-aware progress primitive used for EVERY reset surface — the detector floor (a persisted last_novel_sealed_progress_at or equivalent deterministic recomputation, since count columns alone cannot identify the last-advance timestamp after restart), the Part 4 telomere reset, and the confidence-gate progressAdvanced — plus a publish-contract decision for undeclared in-scope rows and a falsification test proving they do not move the floor.
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:3"]
    text: >-
      C1 (carries into the single revision, GATE): make the detector clock consume novelty, not events. Define one novelty-aware progress primitive (the strict-increase cursor of Claim 3.1/3.3 hardened to declared/milestone artifacts) and use it for EVERY reset surface — (a) the Part 1 floor must read the timestamp of the last strict cursor advance (a persisted last_novel_sealed_progress_at on job_recovery_state, or a provably deterministic restart-stable recomputation; the three count columns alone cannot recover that timestamp), NOT raw jobSealedProgressAt; (b) the Part 4 telomere reset; (c) the RFC 0131 confidence-gate progressAdvanced (consecutive_silent_sweeps / misfire_evidence_score). Decide the publish contract for undeclared in-scope artifact rows (allowed-but-ignored-for-deadline vs expected-artifact-only for deadline-participating jobs). Falsification test: a hostile job publishes a fresh undeclared distinct-content artifact before every budget expiry; assert the cursor does not advance, the floor does not move, requeue_count and silent sweeps continue advancing, and the lane reaches the telomere floor with exactly one operator escalation.
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:2"]
    text: >-
      C2 (carries into the single revision, GATE before the action arms): correct the no-false-kill claim. Either (a) narrow the proven property to "no false-kill for lanes that remain tool-fresh, are inside an instrumented tool call, or emit local-work heartbeats within ToolProgressSeconds" and explicitly name the long-silent legitimate-think interval an accepted, bounded residual covered by the advisory default plus the operator-grant seam; OR (b) add a build-bearing reprieve for legitimate non-sealed non-tool intervals before the rung takes destructive action. Either way add a falsification test: a single-final-seal lane whose last tool call is older than ToolProgressSeconds but whose supervised process is alive and legitimately working must not be destructively acted on by the armed rung. The AND-not-OR structure itself is ratified; this constraint repairs the overclaimed proof, not the mechanism direction.
verdict: "needs_revision"
rationale: >-
  needs_revision. The SPEC is high-quality, source-anchored, and ratifies the correct central decision: the AND-not-OR (sealed-clock-stale AND #324-ToolProgressWedged) is the right no-false-kill structure — it strictly narrows #324's firing set, reuses the exact exported predicate so the tool half cannot drift, inherits #324's in-tool and zero-tool-history exclusions, and avoids the banned plain-wall-clock OR cap. Part 1's derived-not-stored design and per-current-lease floor are genuinely restart-consistent and reset correctly per requeued session; Part 4's ladder mechanism (reuse of transfer_requeue/requeueJobSameAttempt, telomere bound via requeue_count vs maxRequeues/unsealedRequeueBudget, idempotent single escalation, correct complete-stalled-vs-requeue-stale verb branch) is sound GIVEN a breach. BUT the gate does not clear because TWO load-bearing properties carry standing challenges. (1) ANTI-GAMING IS BROKEN (Falsifier 2, critical, unrebutted): the holder novelty-gated only the telomere COUNTER, leaving the detector clock floor on raw jobSealedProgressAt and the RFC 0131 confidence-gate progressAdvanced on raw created_at. A cheap, durable, restart-surviving stream of junk undeclared in-scope artifacts (distinct content_sha256, fresh logical_name) keeps the floor perpetually fresh, so the rung never even arms — directly contradicting the RFC's own Part 3 requirement that the clock, not just the cursor, consume novelty. The holder's own Claim 3.3 evidence (expected_artifacts are presence assertions, not an allowlist; artifact_source_publish.go:115) establishes the attack surface, and the hard-core proof's "cannot keep a job un-escalatable once the deadline is reached by any means" silently assumes the deadline is reachable — Falsifier 2 shows it is not. (2) NO-FALSE-KILL IS OVERCLAIMED (Falsifier 1, high, partially rebutted by the advisory default): the AND provably spares tool-fresh / in-tool / heartbeating lanes, but a legitimately-slow tool-silent long-think single-final-seal lane is killed once armed; the holder admits this residual and routes it to calibration + operator-grant (out of P0), which is not a proof. The P0 shadow-first default (SealedSilenceSeconds=0, advisory only) contains the actual blast radius — landing to main takes no destructive action — so this is a claim-narrowing + test, not a reject. Both defects are concrete and in-P0-buildable, so the gate is recoverable in one revision. Per-part: Part 1 ratified on derivation/restart-consistency but its floor input must switch to the novelty primitive (C1); Part 2 AND-not-OR RATIFIED as structure, proof narrowed (C2); Part 3 NOT RATIFIED (C1); Part 4 ladder ratified conditionally but never engages under the Falsifier-2 attack and inherits C1. This is the single allowed v1 revision cycle: a second needs_revision ends the gate uncleared and routes to the operator for a fresh -v2 run.
findings:
  - id: F-CLOCK-EVENT-GATED-NOT-NOVELTY-GATED
    severity: critical
    posture: anti_gaming_broken
    status: converted_to_constraint
    source_refs: ["dialogue:3"]
    affected_invariants:
      - "the detector clock must consume novelty, not events (RFC Part 3)"
      - "fake/junk sealed rows must not reset the sealed-silence floor"
      - "a deterministically-flailing lane must converge to the telomere floor with one escalation"
    challenge: >-
      The Part 1 floor reads raw jobSealedProgressAt = GREATEST(max(artifacts.created_at), max(verdicts.created_at)); the Claim 3.1 novelty cursor gates only the Part 4 telomere counter reset. A hostile lane source-publishes a fresh undeclared in-scope artifact (distinct content_sha256, fresh logical_name) every SealedSilenceSeconds - epsilon. Under the holder's own hardening these rows do not advance the cursor, but they DO advance max(artifacts.created_at), so the floor stays fresh, sealedSilenceBreached never becomes true, the rung never fires, requeue_count never climbs, and the telomere ladder never engages. The RFC 0131 confidence-gate progressAdvanced (raw jobSealedProgressAt.After(windowStart)) is a second event-gated reset that a junk row also trips, resetting consecutive_silent_sweeps. Durable and restart-faithful. Directly contradicts RFC snapshot Part 3 ("the cursor (and the clock) do not advance").
    closest_acceptable_answer: >-
      One novelty-aware progress primitive used by every reset surface: the floor reads the timestamp of the last strict cursor advance (persisted last_novel_sealed_progress_at on job_recovery_state, or a provably deterministic restart-stable recomputation), the telomere counter resets on it, and the confidence-gate progressAdvanced uses the same predicate. Plus an explicit publish-contract decision for undeclared in-scope rows and a test proving they do not move the floor.
    requested_constraint_shape:
      kind: gate
  - id: F-NO-FALSE-KILL-PROOF-OVERCLAIMED
    severity: high
    posture: safety_claim_overclaimed
    status: converted_to_constraint
    source_refs: ["dialogue:2"]
    affected_invariants:
      - "no false-kill of a legitimately-slow-but-working single-final-seal lane (#145 CASE-2)"
      - "the no-false-kill property must be proven, not calibration-dependent"
    challenge: >-
      The AND provably spares lanes inside a tool call or issuing tool calls within ToolProgressSeconds, but a legitimately-healthy single-final-seal lane in a long planning/model-generation interval (tool-silent past ToolProgressSeconds, PTY keeping the lease warm, zero seals) satisfies both sealedSilenceBreached and toolWedged once armed, so the rung fires while the lane is healthy. The hard-core proof's "alive-and-legitimately-working => never killed" is overclaimed; the holder routes the long-silent think to calibration + operator-grant, both out of P0. A model/planning interval cannot necessarily self-interrupt to heartbeat.
    closest_acceptable_answer: >-
      Narrow the proven claim to tool-fresh / in-tool / heartbeating lanes and name the long-silent legitimate interval an accepted bounded residual (covered by the advisory default + operator-grant seam), OR add a build-bearing reprieve for legitimate non-sealed non-tool intervals; in either case add a falsification test for a single-final-seal lane whose last tool call is older than ToolProgressSeconds but whose process is alive and working, asserting the armed rung does not destructively act.
    requested_constraint_shape:
      kind: gate
constraints:
  - id: C1-NOVELTY-PRIMITIVE-ON-EVERY-RESET-SURFACE
    source_finding: F-CLOCK-EVENT-GATED-NOT-NOVELTY-GATED
    posture: anti_gaming_broken
    severity: critical
    kind: gate
    binding: true
    source_refs: ["dialogue:3"]
    text: >-
      The revised SPEC must make the detector clock consume novelty, not events. Define one novelty-aware progress primitive (the strict-increase cursor hardened to declared/milestone artifacts) and use it for every reset surface: (a) the Part 1 floor must read the timestamp of the last strict cursor advance — a persisted last_novel_sealed_progress_at on job_recovery_state, or a provably deterministic restart-stable recomputation; the three count columns alone cannot recover that timestamp — NOT raw jobSealedProgressAt; (b) the Part 4 telomere counter reset; (c) the RFC 0131 confidence-gate progressAdvanced (consecutive_silent_sweeps / misfire_evidence_score). The SPEC must also decide the publish contract for undeclared in-scope artifact rows (allowed-but-ignored-for-deadline vs expected-artifact-only for deadline-participating jobs).
    verification:
      gate: >-
        Falsification test: a hostile job publishes a fresh undeclared distinct-content artifact before every sealed-silence budget expiry; assert the novelty cursor does not advance, the sealed-silence floor does not move, requeue_count / consecutive_silent_sweeps continue advancing, and the lane reaches the telomere floor with exactly one operator escalation. Must hold across a daemon restart mid-ladder (persisted last-advance, no spurious reset, no phantom carry-over).
    final_review_required: true
  - id: C2-NARROW-NO-FALSE-KILL-CLAIM-BEFORE-ARMING
    source_finding: F-NO-FALSE-KILL-PROOF-OVERCLAIMED
    posture: safety_claim_overclaimed
    severity: high
    kind: gate
    binding: true
    source_refs: ["dialogue:2"]
    text: >-
      The revised SPEC must correct the no-false-kill claim before the escalating action is allowed to arm: either narrow the proven property to "no false-kill for lanes that remain tool-fresh, are inside an instrumented tool call, or emit local-work heartbeats within ToolProgressSeconds" and explicitly name the long-silent legitimate-think interval an accepted bounded residual (advisory default + operator-grant seam), or add a build-bearing reprieve for legitimate non-sealed non-tool intervals before destructive action. The AND-not-OR structure stands; this repairs the overclaimed proof, not the direction.
    verification:
      gate: >-
        Falsification test: a single-final-seal lane whose last tool call is older than ToolProgressSeconds but whose supervised process is alive and legitimately working must not be destructively acted on by the armed rung (it is spared by the reprieve, or the SPEC states the bounded accepted residual and proves the advisory default takes no destructive action).
    final_review_required: true
branches:
  anti_gaming_broken: "blocked"
  safety_claim_overclaimed: "blocked"
---

# Collaboration Ledger — RFC 0166 P0 design (cycle 1)

**Verdict: `needs_revision`.** This is the single allowed v1 revision cycle. The
SPEC is strong, source-anchored, and ratifies the correct central decision — the
**AND-not-OR** no-false-kill structure — but two load-bearing properties carry a
standing material challenge, so the gate does not clear.

## The central decision: AND vs OR — RATIFIED

The AND (`sealedSilenceBreached && ToolProgressWedged`) is the right no-false-kill
core, and I ratify it as the structure to build:

- It **strictly narrows** #324's firing set by adding a second required condition,
  so it cannot kill anything #324 already spares.
- It reuses the **exact exported `ToolProgressWedged`** predicate, so the tool half
  is bit-identical to #324 and cannot drift, inheriting the in-tool reprieve
  (the #145 long-foreground-command case), the zero-tool-history exclusion, and the
  `ToolProgressSeconds<=0` disable for free.
- It rejects the banned plain-wall-clock OR / sealed-only cap, which would
  false-kill the single-final-seal lane mid-work.

The ratification of the *structure* is not in question. What does not clear is the
*proof* attached to it (C2) and the *clock that feeds it* (C1).

## Per-part disposition

| Part | Claim | Status | Basis |
|------|-------|--------|-------|
| **1 — derived clock** | derived (not stored), floor = GREATEST(jobSealedProgressAt, currentLeaseAcquiredAt, started_at), restart-reproducible; new `StallSealedProgress` evaluated only in `recoverStuckJobs` | **NOT RATIFIED (floor input)** | Derivation, all-durable-inputs restart-consistency (1.1), and per-current-lease reset on requeue (1.2) are sound and survive restart. BUT the floor's progress input is raw `jobSealedProgressAt` (event-gated), which fails the RFC Part 3 "clock consumes novelty" requirement and is the Falsifier-2 hole. → **C1.** |
| **2 — AND, not OR** | rung fires iff `sealedSilenceBreached && ToolProgressWedged` | **STRUCTURE RATIFIED / PROOF OPEN** | AND-not-OR is the correct minimal no-false-kill structure (narrows #324, no wall-clock OR). But hard-core obligation 1 ("alive-and-legitimately-working ⇒ never killed") is overclaimed: a tool-silent legitimate long-think lane is killed once armed. → **C2.** |
| **3 — anti-gaming (novelty reset)** | reset requires strict increase of a content_sha256 / verdict / milestone novelty cursor, junk-blob-hardened to declared artifacts | **NOT RATIFIED** | The cursor hardening is correct **for the counter**, but novelty was **not** applied to the clock floor or the RFC 0131 confidence-gate. A junk-`content_sha256` stream games the deadline so the rung never arms. → **C1.** |
| **4 — telomere ladder** | first-breach auto-requeue-one; reset only on novelty; telomere bound; idempotent single escalation with correct verb | **RATIFIED CONDITIONALLY** | The ladder mechanism (reuse `transfer_requeue`/`requeueJobSameAttempt`, `requeue_count` vs `maxRequeues`/`unsealedRequeueBudget`, idempotent `escalation_pending`, `complete-stalled` vs `requeue-stale --override` branch) converges **given a breach** — but it never engages under the Falsifier-2 attack, and the confidence-gate `progressAdvanced` reset is a second event-gated hole. Inherits **C1.** |

## Per-falsifier disposition

- **Falsifier 2 — Part 3/4 raw-clock (dialogue:3): MATERIAL, STANDING, DECISIVE.**
  The holder novelty-gated only the telomere **counter** and left the **clock**
  (Claim 1.1 floor) on raw `jobSealedProgressAt = GREATEST(max(artifacts.created_at),
  max(verdicts.created_at))`. A cheap, durable, restart-surviving stream of junk
  undeclared in-scope artifacts (distinct `content_sha256`, fresh `logical_name`)
  keeps the floor fresh so `sealedSilenceBreached` is never true and the rung never
  arms. This **contradicts the RFC's own Part 3** (snapshot: "the deadline consumes
  novelty, not events … so the cursor *(and the clock)* do not advance"), and the
  holder's own Claim 3.3 evidence (`expected_artifacts` are presence assertions, not
  an allowlist — `artifact_source_publish.go:115`) establishes the very surface it
  failed to close on the clock. The hard-core proof's "cannot keep a job
  un-escalatable once the deadline is reached **by any means**" assumes the deadline
  is reachable; this attack keeps it perpetually unreached. Also lands the second
  event-gated reset path: confidence-gate `progressAdvanced` from raw `created_at`.
  **Not rebutted. → C1.**

- **Falsifier 1 — Part 2 no-false-kill (dialogue:2): MATERIAL, STANDING, partially
  contained.** The AND provably spares tool-fresh / in-tool / heartbeating lanes,
  but the hard-core "alive-and-legitimately-working ⇒ never killed" is overclaimed:
  a single-final-seal lane in a long legitimate tool-silent planning/model-generation
  interval (PTY warm, no tool call past `ToolProgressSeconds`, zero seals) trips both
  axes once armed — the #145 CASE-2 risk. The holder **admits** this residual and
  routes it to calibration + operator-grant, both out of P0; that is not a proof. The
  P0 **shadow-first** default (`SealedSilenceSeconds=0`, advisory only) means landing
  to `main` takes no destructive action, so the actual blast radius is contained and
  the fix is a claim-narrowing + reprieve + test, not a reject. **Not fully rebutted.
  → C2.**

## What clears the gate on revision

Both defects are concrete and in-P0-buildable — recoverable in one revision (hence
`needs_revision`, not `reject`):

1. **C1 (critical)** — make the **clock** consume novelty, not events. One
   novelty-aware progress primitive feeds every reset surface: the Part 1 floor
   (last strict cursor-advance timestamp — a persisted `last_novel_sealed_progress_at`
   or a deterministic restart-stable recomputation, not raw `jobSealedProgressAt`),
   the Part 4 telomere reset, and the RFC 0131 confidence-gate `progressAdvanced`.
   Decide the undeclared-in-scope-row publish contract. Falsification test:
   junk-distinct-content publish before every budget expiry must not move the floor,
   and the lane must reach the telomere floor with exactly one escalation, across a
   restart.
2. **C2 (high)** — narrow the no-false-kill claim (tool-fresh / in-tool /
   heartbeating; long-silent legitimate think = accepted bounded residual under the
   advisory default + operator-grant) OR add a build-bearing reprieve; add the
   single-final-seal tool-silent falsification test before the action arms.

This is the single allowed v1 revision cycle: a second `needs_revision` ends the gate
uncleared and routes to the operator (who spins a fresh `-v2` run with a revising
holder).
