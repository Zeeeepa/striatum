# Design-Run Seed — RFC 0166 P0 (REVISION v2)

> This is the **v2 revision** of the RFC 0166 P0 design run (completion deadline
> for an alive-but-never-completing lane — the **sealed-progress silence budget**,
> #576). v1 **ratified the AND-not-OR no-false-kill core** and the Part 1-4
> mechanism shape, but the adjudicator returned `needs_revision` on two
> source-anchored GATE constraints and, its single cycle exhausted, routed them
> to the operator. This run discharges **C1** and **C2** while carrying forward
> everything v1 ratified. **Required context docs** (read in full first):
> - `docs/operator/artifacts/rfc-0166-design/dialogue/holder/HOLDER.md` — the v1 SPEC you are revising (the base; do not rewrite from scratch).
> - `docs/operator/artifacts/rfc-0166-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — the v1 verdict; its `constraints:` C1/C2 are the exact prescribed fixes.
> - `docs/operator/workflows/rfc-0166-design-v2/RFC_0166_SNAPSHOT.md` — the RFC (kept on its review branch `rfc/0166-alive-but-never-completing-lane-deadline`; snapshotted here, not merged to `docs/rfcs/`).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **revised falsifiable
implementation spec for RFC 0166 P0** the downstream `rfc-0166-build`
`code_change` run executes. It must **resolve C1 and C2** and **carry forward,
unregressed, everything v1 ratified**. A revision that leaves C1 or C2 open — or
regresses a carry-forward — has NOT cleared the gate.

## Carried forward — RATIFIED in v1 (do NOT reopen, do NOT regress)

- **The AND-not-OR no-false-kill CORE.** The rung fires **iff**
  `sealedSilenceBreached AND sessionliveness.ToolProgressWedged` (the exact #324
  predicate, promoted to exported). The AND strictly narrows #324; a lane making
  real tool calls or inside a tool call is spared. This structure is ratified —
  the v2 work repairs the over-claimed *proof* around it, not the mechanism.
- **The Part 1-4 mechanism shape:** Part 1 a derived (not stored) per-session
  clock floor, restart-reproducible from durable rows, a new `StallSealedProgress`
  class evaluated only in `recoverStuckJobs`; Part 2 the AND correctness
  constraint; Part 3 anti-gaming (reset consumes novelty, not events); Part 4 the
  telomere-bounded self-heal ladder (first breach auto-requeues one fresh
  session; shortens toward `maxRequeues`/`unsealedRequeueBudget`) + exactly one
  idempotent `needs_operator` escalation with a copy-pasteable recovery verb.
- **Shadow-first P0 rollout:** `SealedSilenceSeconds` defaults `0` (disabled;
  advisory doctor/dashboard signal only); the escalating action arms **only** on
  explicit opt-in.

## The two binding constraints to DISCHARGE

### C1 (GATE) — make the detector clock consume NOVELTY, not events

v1's Part-1 floor is raw `jobSealedProgressAt = GREATEST(max(artifacts.created_at),
max(verdicts.created_at))`, and the novelty cursor gates **only** the telomere
reset, not the clock. So a hostile lane that publishes a fresh **undeclared
in-scope** artifact (allowed kind, fresh `logical_name` `junk_N`, distinct
`content_sha256`) every `SealedSilenceSeconds − epsilon` advances
`max(artifacts.created_at)` → the floor advances, `sealedSilenceBreached` stays
false, **the rung never fires** and the telomere ladder never engages (durable;
survives a daemon restart). The same raw clock feeds the **RFC 0131 confidence
gate** `progressAdvanced`, so a junk row also resets `consecutive_silent_sweeps`.
This violates the RFC's own Part 3 ("the deadline consumes novelty, not events").

The revised SPEC must:
- Define **one** novelty-aware progress primitive (the strict-increase cursor of
  Claim 3.1/3.3, hardened to **declared/milestone** artifacts) and use it for
  **every** reset surface: (a) the **Part-1 floor** reads the timestamp of the
  last strict cursor advance — a persisted `last_novel_sealed_progress_at` on
  `job_recovery_state`, or a provably deterministic restart-stable recomputation
  (the three count columns alone cannot recover that timestamp) — **not** raw
  `jobSealedProgressAt`; (b) the **Part-4 telomere reset**; (c) the **RFC 0131
  confidence-gate** `progressAdvanced` / `consecutive_silent_sweeps`.
- Decide the **publish contract** for undeclared in-scope artifact rows
  (allowed-but-ignored-for-deadline vs expected-artifact-only for
  deadline-participating jobs).
- Add the falsification test: a hostile job publishes a fresh undeclared
  distinct-content artifact before every budget expiry → assert the cursor does
  not advance, the floor does not move, `requeue_count` + silent sweeps continue
  advancing, and the lane reaches the telomere floor with **exactly one**
  operator escalation.

### C2 (GATE before the action arms) — correct the no-false-kill claim

v1 over-claimed "no false-kill". The revised SPEC must either:
- (a) **narrow** the proven property to "no false-kill for lanes that remain
  tool-fresh, are inside an instrumented tool call, or emit local-work heartbeats
  within `ToolProgressSeconds`" and explicitly name the **long-silent
  legitimate-think interval** an accepted, bounded residual (covered by the
  advisory default + the operator-grant seam); **OR**
- (b) add a **build-bearing reprieve** for legitimate non-sealed non-tool
  intervals before the rung takes destructive action.

Either way, add the falsification test: a single-final-seal lane whose last tool
call is older than `ToolProgressSeconds` but whose supervised process is **alive
and legitimately working** must NOT be destructively acted on by the armed rung.
The AND-not-OR structure itself stands; this repairs the proof, not the mechanism.

## Falsifier guidance (re-attack the revision)

- **Falsifier 1 (C1 / novelty-clock lens):** Re-run the junk-undeclared-artifact
  attack against the revised floor; check the SAME novelty primitive drives all
  three reset surfaces (Part-1 floor, Part-4 telomere reset, RFC 0131
  `progressAdvanced`); confirm the persisted cursor is restart-stable; probe the
  undeclared-in-scope publish contract for a new gaming surface.
- **Falsifier 2 (C2 / no-false-kill + carry-forward lens):** Construct the
  alive-but-tool-silent legitimately-working single-final-seal lane and check the
  armed rung does not kill it; then verify no carry-forward regressed (the
  AND-not-OR core, Parts 1-4, the shadow-first default, the single idempotent
  escalation).

The adjudicator gates on whether C1 and C2 are each **genuinely discharged** (the
mechanisms anchored to real source, the two named falsification tests specified)
and whether any carry-forward regressed or a new material challenge lands.
Clearing (`accept` / `accept_with_findings`) requires both discharged with their
tests and no standing regression. This is the single allowed v2 revision cycle; a
second `needs_revision` ends the gate uncleared and routes to the operator for a
fresh `-v3` run. Keep the local-first product boundary (one host, one PostgreSQL,
one daemon as single writer; no hosted services).
