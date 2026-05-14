# RFC 0052 — Committee deliberation workflow with arbitration, panels, and adversarial review

**Status:** proposed
**Scope:** V1.9 or V2.0 (depends on RFC 0048 daemon-side business-logic landing)
**Closes (partially):** reviewer-co-blindness anti-pattern visible in D095–D102; the
publish → disagree → revisit → revisit latency tax on high-stakes design phases.

## Background

Striatum's current review model is artifact-mediated and sequential.
RFC 0002 (reviewer independence), RFC 0004 (critique-to-action), and
RFC 0018 (focused adversarial postures) compose into the pattern that
shows up in every dogfood: an implementer publishes an artifact, one or
more reviewers claim packets against it, each publishes a verdict
artifact, and disagreements escalate by either operator override
(D095–D102) or another revision cycle.

That shape works, but it has two costs the dogfood ledger keeps
surfacing:

1. **Iteration latency.** Every disagreement between two roles is an
   artifact round-trip: publish → claim-next → ack → read → publish
   verdict. For three-lane design phases (D032), the round-trips
   stack. The thinking is fast; the artifact handoff dominates wall
   clock.
2. **Reviewer co-blindness.** D095 / D096 / D097 / D098 / D100
   document the same anti-pattern: when the implementer and reviewer
   share a lane (codex+codex in particular), they share blind spots
   and the reviewer rubber-stamps. D099 / D101 document the inverse:
   when a reviewer with a different lane applies their own posture
   conservatism to another model's work, real findings get treated as
   baseline. Both are *role wiring* failures, not *role posture*
   failures — RFC 0018's posture labels do not by themselves fix the
   composition.

Under RFC 0043 (Postgres as sole substrate, daemon required) and
RFC 0036 / RFC 0040 (MCP harness over the full mutation registry),
the daemon already brokers tight back-and-forth between sessions over
typed RPC tools. Tight loops between roles are cheap on the current
substrate; the existing review shape just doesn't exploit them.

This RFC proposes a **committee deliberation** workflow shape that
treats disagreement as a structured first-class phase rather than as a
trigger for revision cycles, and that solves co-blindness by lane
composition rather than posture labelling.

## Goals

- Define a workflow shape in which N producer roles (designers,
  implementers) deliberate to convergence under a named **arbitrator**
  before a downstream consumer reads their output.
- Make **debate turns** typed, front-mattered, durable artifacts so
  the deliberation is auditable and replayable, not a transcript.
- Define an **arbitrator role** with bounded authority: record
  consensus, escalate to a **panel**, declare stalemate, sustain or
  overrule challenges.
- Define a **panel** as a fan-out sub-workflow with fresh sessions on
  rotated lanes that reads `documents[] + debate_turns[]` and emits a
  single panel verdict by configured rule (unanimity, supermajority,
  arbitrator-tie-break).
- Define an **adversarial review** variant in which an interrogator
  role and a defendant role exchange typed moves under an arbitrator,
  with attested lanes on both sides per RFC 0026.
- Cap the deliberation phase with declared cycle bounds (D014) and an
  explicit termination rule.

## Non-goals

- Replacing the existing single-reviewer pattern. Committees are
  expensive; most jobs do not need one. The workflow shape is opt-in
  per phase, not a default.
- Free-form chat between agents. Every debate move is a typed
  artifact with front-mattered fields. The MCP/RPC surface delivers
  them; nothing about this RFC introduces a chat channel.
- Operator-on-behalf debate. Per RFC 0026, debate turns require lane
  attestation; an operator may not author a debate turn on behalf of
  a session without the existing `--allow-no-process-execution`
  override audit trail (RFC 0046).
- Replacing the decision-propagation machinery (RFC 0047). An
  arbitrator's ruling produces a decision artifact through the
  existing path; this RFC does not invent a parallel decision system.
- A new substrate, transport, or storage shape. Everything proposed
  here lands on the V1.6 substrate.

## Validation

**Net assessment: worth proposing.** The shape is a natural composition
of existing primitives (artifacts, verdicts, decisions, lane
attestation, fresh sessions, declared cycles), not a new system. The
dogfood ledger names two recurring failure modes (co-blindness,
iteration latency) that the existing shape cannot structurally fix
and that this shape can.

**What this RFC validates:**

- *Co-blindness is a composition failure, not a posture failure.* RFC
  0018 added `devils_advocate` and `security` postures, and the dogfood
  ledger still shows D095/D096/D097/D098/D100 — same-lane reviewers
  cluster blind spots even with adversarial posture set. A committee
  shape that *requires* lane diversity at composition time, and a
  panel that *requires* rotated fresh-session lanes, addresses the
  failure where it actually lives.
- *Latency is a handoff cost, not a thinking cost.* The MCP harness
  (RFC 0036/0040) already supports tight RPC loops. The current review
  shape pays an artifact round-trip per disagreement turn; a debate
  shape can publish a turn and immediately claim the next one without
  re-entering the work-packet queue. The daemon brokers this trivially
  on the current substrate.
- *Adversarial review needs role wiring, not just posture.* RFC 0018
  named the posture; this RFC names the *protocol* (interrogator /
  defendant / arbitrator moves) so adversarial review composes the
  same way every time and produces comparable artifacts across runs.

**What this RFC does not validate:**

- *That a committee outperforms a single thoughtful reviewer on
  ordinary jobs.* It almost certainly doesn't. The shape is for
  phases where (a) the cost of error is high enough to justify N×
  reviewer cost, (b) the producer count is already >1 (three-lane
  design per D032), or (c) prior single-reviewer rounds failed to
  resolve. Workflow authors must opt in per phase.
- *That artifact volume stays bounded.* See Open question 4.

**Risks called out, not resolved here:**

- **Artifact-vs-transcript boundary.** Debate turns are typed,
  front-mattered, and bounded in count by the cycle limit, so they
  fit D028's artifact side rather than the transcript side. But a
  3-designer × 5-round × 3-conflict-point debate produces ~45 turn
  artifacts, plus a synthesis. The audit-chain and corpus-export
  paths handle it (RFC 0044), but the human-readability story leans
  hard on the **debate synthesis** artifact at the end. The
  synthesis is not optional.
- **Termination soundness.** Cycles must terminate by either
  consensus, arbitrator decision, panel verdict, or declared
  exhaustion. The arbitrator's `declare_stalemate` move must be
  uncontested-publish (no further moves accepted on the topic in
  this phase). Without that, debate can spin.
- **Arbitrator capture.** If the arbitrator role is always the same
  lane, it inherits that lane's biases. Workflow authors should
  rotate arbitrator lanes across phases the way RFC 0026 expects
  reviewer-lane rotation, but this RFC does not enforce it. Open
  question 2 discusses making rotation a validator-checked rule.
- **Adversarial-review honesty.** An interrogator session must have
  the same attestation guarantees as any other publishing session
  (RFC 0026). An "interrogator" who is really the operator typing
  for a model that did not run is a forged byline. The existing
  guards (RFC 0026 + RFC 0046) catch this; this RFC adds no new
  surface area.

## Proposed shape (not a design)

This section names the primitives the RFC introduces so reviewers
can argue about the shape. Concrete schemas and method names land in
a follow-up design doc once the RFC is accepted.

### New artifact kinds

- `debate_turn` — one typed move by one role in a deliberation.
  Front-matter fields: `round`, `speaker` (session-id), `addressee`
  (session-id or `all`), `move_type` (proposal / objection / rebuttal
  / concession / yield), `references` (artifact ids of prior moves
  or source documents), `conflicts_with` (artifact ids of moves this
  one disputes).
- `arbitration_ruling` — a typed arbitrator move with a fixed move
  vocabulary: `record_consensus`, `escalate_to_panel`, `call_timeout`,
  `sustain_objection`, `overrule_objection`, `declare_stalemate`. A
  ruling closes the topic it addresses; further turns on that topic
  are refused by the publish path.
- `panel_vote` — one panelist's vote artifact with a configured vote
  schema (e.g. `accept` / `reject` / `abstain` plus a one-paragraph
  rationale). One per panelist.
- `panel_verdict` — the panel's collective result, computed by the
  configured aggregation rule from the panel_vote set. Treated as a
  verdict by the existing verdict-propagation surface.
- `debate_synthesis` — the human-readable closing artifact. Names the
  resolved conflicts, the unresolved deltas (if stalemate), and the
  arbitration trail. Mandatory; the deliberation phase does not
  terminate without one.

### New workflow shape

A `committee_deliberation` phase type in the RFC 0045 multi-phase
schema. Fields (illustrative, not authoritative):

- `participants[]` — role ids of producers (designers / implementers
  / etc.).
- `arbitrator` — single role id with declared lane that **must
  differ from** every participant's lane.
- `panel` (optional) — `{ size, lanes_required: [...], aggregation:
  "unanimity" | "supermajority" | "arbitrator_tie_break" }`. Panel
  sessions are `fresh_session_required: true` per D029. Lane set
  must include lanes not represented in `participants` or
  `arbitrator`.
- `bounded_cycle` — `{ max_rounds, max_turns_per_round,
  exhaustion_action: "escalate_to_panel" | "declare_stalemate" }`
  per D014.
- `topics[]` — declared conflict points the committee must address,
  or `auto_detect_from_initial_artifacts` if the deliberation begins
  from N already-published producer artifacts.
- `adversarial` (optional sub-shape) — pairs `participants[]` into
  `{ interrogator, defendant }` with move_type vocabulary extended
  to include `cross_examination`, `objection_sustained`,
  `objection_overruled`. Arbitrator authority is identical.

### Interaction with existing surfaces

- **Verdicts.** A `panel_verdict` slots into the existing verdicts
  table with `posture: "committee_panel"` (per RFC 0018 V1 step 3).
  No new verdict-propagation logic.
- **Decisions.** An `arbitration_ruling` of move_type
  `record_consensus` or `declare_stalemate` produces an associated
  decision artifact through the existing RFC 0047 path. Stalemate
  decisions can be `accepted_with_follow_up` so the downstream phase
  knows the topic returns.
- **Lane attestation.** Every debate-turn publish goes through RFC
  0046's lane-evidence guard. Operator-on-behalf debate requires
  `--allow-no-process-execution --override-rationale` and is logged.
- **MCP/RPC.** New dotted methods (illustrative): `debate.turn`,
  `debate.list_turns`, `debate.arbitrate`, `debate.synthesize`,
  `panel.vote`, `panel.tally`. Capabilities: existing `write` +
  `review`. No new capability classes.
- **Workflow validation.** Validator must refuse a
  `committee_deliberation` phase whose arbitrator lane equals any
  participant lane, whose panel (if present) does not include at
  least one lane outside `participants ∪ {arbitrator}`, or whose
  bounded cycle has no exhaustion action.
- **Recovery.** Stale debate-turn leases are recoverable through
  RFC 0020 `recovery auto-publish` only when the turn's
  expected-author-line matches the on-disk byline; otherwise the
  arbitrator's `declare_stalemate` is the safe exit.

## Open questions

1. **Consensus arithmetic.** What is the default `record_consensus`
   rule when N participants disagree pairwise but all yield to a
   common rewrite? Is unanimity required, or is N−1 with the
   dissenter's `concession` move sufficient? Likely a workflow-config
   field; needs a default.
2. **Arbitrator rotation enforcement.** Should the validator require
   that across multi-phase workflows (RFC 0045), arbitrator role
   assignments rotate lanes? This pushes the co-blindness fix
   structurally, but it's a stronger claim than this RFC needs to
   defend on its own.
3. **Adversarial pairing rules.** Should `interrogator` and
   `defendant` be required to differ in lane and model, by
   validator, by analogy with RFC 0002 reviewer independence?
4. **Artifact volume bound.** Should `max_turns_per_round ×
   max_rounds × |topics|` have a hard ceiling (e.g. 100 debate
   turns per phase) to prevent corpus-export blow-up, or is the
   bounded-cycle limit enough?
5. **Real-time semantics.** The MCP daemon can push the next
   claim-next packet immediately on the prior publish. Should the
   committee phase opt-in to a *tight-loop* lease class with shorter
   default lease times and `await_packet` long-poll, or reuse the
   default lease class? Tight-loop has implications for the RFC 0009
   supervisor heartbeat and is best deferred to a V1.5 of this RFC.
6. **Cost gate.** Committee phases multiply lane cost by ~3–6×.
   Should the workflow validator surface a cost estimate at
   `workflow validate` time so operators see the price?

## Phasing

- **Phase 0 (this RFC):** validation + acceptance of the shape.
  Concrete schemas and method names land in a follow-up design doc
  authored by the standard three-lane process (D032).
- **Phase A (V1.9 or V2.0):** V1 implementation — debate_turn /
  arbitration_ruling / panel_vote / panel_verdict / debate_synthesis
  artifact kinds; `committee_deliberation` phase type in the RFC 0045
  schema; validator rules; new daemon RPC methods. Tight-loop lease
  class deferred to V1.5.
- **Phase B (V1.5 of this RFC):** adversarial sub-shape with
  `cross_examination` move vocabulary, interrogator/defendant lane
  independence enforcement, tight-loop lease class for real-time
  back-and-forth.
- **Phase C (V2.0 of this RFC):** validator-enforced arbitrator-lane
  rotation across phases, workflow-cost surface at validate time,
  optional `panel_round_robin` aggregation rule for tie-breaking.

## Provenance

- 2026-05-14 conversation in the striatum operator session:
  operator proposed a committee workflow using the MCP interface as
  a message-passing bus, with arbitrator-named conflict resolution,
  escalation to panel, real-time back-and-forth, and an adversarial
  review variant.
- The operator corrected an outdated reading of the substrate
  (still-SQLite per stale SPEC vs. Postgres+daemon per D094 / RFC
  0043 / shipped through v1.40). This RFC explicitly lands on the
  current shipped substrate (Postgres + mandatory daemon + RFC 0040
  MCP harness through v1.33.0, RFC 0036 daemon MCP through
  v1.26.0).
- Prior anti-pattern provenance: D095, D096, D097, D098, D099,
  D100, D101, D102 — eight consecutive cycle-exhaustion overrides
  documenting reviewer co-blindness and cross-lane conservatism
  that posture labels alone did not resolve.
