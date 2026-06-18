# RFC 0052 — Committee deliberation workflow with arbitration, panels, and adversarial review

**Status:** proposed (unblocked, deferred/unscheduled — D225, closes #403: keep live as a tracked future capability; schedule the design→build dogfood when committee semantics are next prioritized)
**Scope:** V1.9 or V2.0 (unblocked by completed RFC 0048; unscheduled)
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

This section names the primitives the RFC introduces and sketches
**one possible** schema shape so reviewers can argue about it
concretely. The sketches are **examples to react to, not
requirements**. Field names, optionality, validator rules, and the
exact method vocabulary are all up for revision in a follow-up
design doc — including replacing any of the sketches wholesale with a
different shape that meets the same goals.

### New artifact kinds

All five new kinds carry `striatum.<kind>.v1` front matter in line
with the existing artifact-kind family (`finding`, `synthesis`,
`decision`, etc.) and validate at publish time per the existing
front-matter validator (exit code 6 on failure).

#### `debate_turn`

One typed move by one role in a deliberation. Sketch:

```yaml
---
schema: striatum.debate_turn.v1
author: <designer-claude-001>           # RFC 0040/D040 lowercase byline
run_id: run_<hex>
phase_id: phase_<hex>                   # RFC 0045 phase id
topic_id: topic_<hex>                   # one of the phase's declared topics
round: 1                                # 1-indexed; refused if > bounded_cycle.max_rounds
speaker_session_id: sess_<hex>
addressee: sess_<hex> | "all"
move_type: proposal | objection | rebuttal | concession | yield
references: [art_<hex>, ...]            # prior moves or source docs cited
conflicts_with: [art_<hex>, ...]        # debate_turn ids this move disputes
closes_topic: false                     # only `concession` + `yield` may set true
---

<markdown body — the actual argument, kept short by convention>
```

Example validator rules the design phase could adopt: `speaker_session_id` must be one of
`participants` (or `arbitrator` if move_type ∈ {sustain_objection,
overrule_objection}); `references` and `conflicts_with` must resolve
to artifacts in the same run; `round` must be ≤ the phase's
`bounded_cycle.max_rounds`; publish refused if the topic is already
closed by an `arbitration_ruling`.

#### `arbitration_ruling`

A typed arbitrator move with a fixed move vocabulary. Sketch:

```yaml
---
schema: striatum.arbitration_ruling.v1
author: <arbitrator-gemini-001>
run_id: run_<hex>
phase_id: phase_<hex>
topic_id: topic_<hex>
arbitrator_session_id: sess_<hex>
move_type: record_consensus | escalate_to_panel | call_timeout
         | sustain_objection | overrule_objection | declare_stalemate
target_turn_id: art_<hex>              # required for sustain/overrule
references: [art_<hex>, ...]           # the turns being ruled on
consensus_artifact_id: art_<hex>       # required for record_consensus —
                                       # the producer artifact the committee
                                       # agreed to carry forward
follow_up_decision_id: dec_<hex>       # filled by post-publish hook for
                                       # record_consensus / declare_stalemate
---

<markdown body — the arbitrator's reasoning>
```

Example validator rules the design phase could adopt: only the role declared `arbitrator` in the phase may
publish this kind; `record_consensus` requires
`consensus_artifact_id`; `escalate_to_panel` is refused unless the
phase declares a `panel`; rulings with terminal move_types
(`record_consensus`, `escalate_to_panel`, `declare_stalemate`) close
the topic — further `debate_turn` publishes against that `topic_id`
are refused with a `topic_closed` error.

#### `panel_vote`

One panelist's vote. Sketch:

```yaml
---
schema: striatum.panel_vote.v1
author: <panelist-codex-002>
run_id: run_<hex>
phase_id: phase_<hex>
panel_id: panel_<hex>                   # from the parent ruling that escalated
panelist_session_id: sess_<hex>
vote: accept | reject | abstain
rationale_artifact_id: art_<hex>        # optional separate finding-style doc
---

<markdown body — short rationale; long form goes in the linked finding>
```

Example validator rules the design phase could adopt: `panelist_session_id` must be a session registered
under the panel; one vote per panelist (idempotent re-publish allowed
only if `vote` and `rationale_artifact_id` are unchanged);
`fresh_session_required: true` enforced at session-register time per
D029.

#### `panel_verdict`

The panel's aggregated result. Sketch:

```yaml
---
schema: striatum.panel_verdict.v1
author: <arbitrator-gemini-001>         # arbitrator publishes the tally
run_id: run_<hex>
phase_id: phase_<hex>
panel_id: panel_<hex>
aggregation: unanimity | supermajority | arbitrator_tie_break
votes:
  - { vote_id: art_<hex>, panelist: sess_<hex>, vote: accept }
  - { vote_id: art_<hex>, panelist: sess_<hex>, vote: reject }
  - { vote_id: art_<hex>, panelist: sess_<hex>, vote: abstain }
tally: { accept: 2, reject: 1, abstain: 0 }
outcome: accept | reject                 # derived by aggregation rule
posture: committee_panel                 # for RFC 0018 V1 step 3 surface
---

<markdown body — outcome summary; cites each vote artifact>
```

Example validator rules the design phase could adopt: `tally` must match the `votes` list exactly; `outcome`
must be derivable from `aggregation` + `tally` (validator recomputes
and refuses on mismatch); the verdicts table receives a row keyed off
this artifact with `verdicts.posture = "committee_panel"`.

#### `debate_synthesis`

The mandatory closing artifact. Sketch:

```yaml
---
schema: striatum.debate_synthesis.v1
author: <synthesizer-claude-001>        # a synthesizer role, lane != arbitrator
run_id: run_<hex>
phase_id: phase_<hex>
topics:
  - topic_id: topic_<hex>
    status: consensus | panel_decided | stalemate
    outcome_artifact_id: art_<hex>      # the consensus / panel_verdict /
                                        # last-ruling artifact
    unresolved_deltas: [<short desc>, ...]   # only for stalemate
arbitration_trail: [art_<hex>, ...]     # ordered list of every
                                        # arbitration_ruling in the phase
producer_artifacts: [art_<hex>, ...]    # the initial producer artifacts
---

<markdown body — the readable digest>
```

Example validator rules the design phase could adopt: phase termination is refused without exactly one
`debate_synthesis`; every declared `topic_id` must appear in
`topics`; `arbitration_trail` must enumerate every ruling in
publish order.

### New workflow shape

A `committee_deliberation` phase type in the RFC 0045
`striatum.workflow.v1.1` `phases[]` array. Sketch:

```json
{
  "phase_id": "phase_design_committee",
  "phase_type": "committee_deliberation",
  "title": "Three-lane design committee",
  "participants": [
    { "role_id": "designer_claude", "lane": "claude" },
    { "role_id": "designer_codex",  "lane": "codex"  },
    { "role_id": "designer_gemini", "lane": "gemini" }
  ],
  "arbitrator": {
    "role_id": "arbitrator",
    "lane": "claude",
    "fresh_session_required": true
  },
  "panel": {
    "size": 3,
    "lanes_required": ["codex", "gemini", "claude"],
    "aggregation": "supermajority",
    "fresh_session_required": true
  },
  "bounded_cycle": {
    "max_rounds": 5,
    "max_turns_per_round": 12,
    "exhaustion_action": "escalate_to_panel"
  },
  "topics": [
    { "topic_id": "topic_state_shape", "title": "State machine shape" },
    { "topic_id": "topic_error_model", "title": "Error propagation" }
  ],
  "topic_detection": "declared",
  "synthesizer": { "role_id": "synthesizer", "lane": "claude" },
  "adversarial": null
}
```

Equivalent shape with `adversarial` pairing:

```json
{
  "phase_id": "phase_security_review",
  "phase_type": "committee_deliberation",
  "participants": [
    { "role_id": "interrogator", "lane": "gemini",
      "adversarial_role": "interrogator" },
    { "role_id": "defendant",    "lane": "claude",
      "adversarial_role": "defendant" }
  ],
  "arbitrator": { "role_id": "arbitrator", "lane": "codex" },
  "adversarial": {
    "move_vocabulary_extension": [
      "cross_examination", "objection_sustained", "objection_overruled"
    ],
    "require_lane_independence": true
  },
  "bounded_cycle": { "max_rounds": 3, "max_turns_per_round": 8,
                     "exhaustion_action": "declare_stalemate" },
  "topics": [
    { "topic_id": "topic_authz_boundary",
      "title": "AuthZ check at the daemon boundary" }
  ],
  "synthesizer": { "role_id": "synthesizer", "lane": "gemini" }
}
```

Example validator rules the design phase could adopt (illustrative):

1. `arbitrator.lane` ∉ `{p.lane for p in participants}`.
2. If `panel` present: `set(panel.lanes_required)` must contain at
   least one lane not in `{arbitrator.lane} ∪ {p.lane for p in
   participants}`.
3. `synthesizer.lane` ≠ `arbitrator.lane`.
4. `bounded_cycle.exhaustion_action` required; refuses `null`.
5. `topics` length ≥ 1 unless `topic_detection: "auto"`.
6. If `adversarial` present: `participants` length == 2 and each
   carries an `adversarial_role` ∈ `{"interrogator", "defendant"}`;
   `require_lane_independence: true` ⇒ both lanes differ.
7. No cross-phase dependency (RFC 0045) may bypass the synthesis
   gate — the synthesizer's artifact is the only legal upstream for
   the next phase.

### New daemon RPC methods

One possible dotted vocabulary, sketched to show that the shape
fits the RFC 0030 registry pattern and the RFC 0043 V1 expansion.
The design phase may rename, merge, or split these freely; the
point is that no new capability classes are needed — everything
binds to existing `write` / `read` / `review`.

| Method | Capability | Repo scope | Description |
|---|---|---|---|
| `debate.publish_turn` | `write` | `single_repo` | Publish a `debate_turn` artifact under the active phase's open topic; refused if topic closed. |
| `debate.list_turns` | `read` | `single_repo` | Enumerate turns by `(phase_id, topic_id, round)`. |
| `debate.arbitrate` | `review` | `single_repo` | Publish an `arbitration_ruling`; arbitrator-role-gated. |
| `panel.register` | `write` | `single_repo` | Register N fresh-session panelists under a `panel_id` created by an `escalate_to_panel` ruling. |
| `panel.vote` | `review` | `single_repo` | Publish a `panel_vote` artifact. |
| `panel.tally` | `review` | `single_repo` | Compute and publish the `panel_verdict` from the recorded votes; arbitrator-role-gated. |
| `debate.synthesize` | `write` | `single_repo` | Publish the `debate_synthesis` artifact; phase termination is gated on this. |

All methods route through the existing publish path, so RFC 0046
lane-evidence guard, RFC 0026 attestation, and RFC 0051
auto-finalize-from-frontmatter all apply unchanged.

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

## Validation experiment: A/B against the normal shape

The committee shape needs to *earn* its cost. The cheapest credible
test is a parallel A/B: take one real input — an RFC body, a
design task, a non-trivial spec — and scaffold two runs against it
in parallel:

- **Run X (committee):** uses the `committee_deliberation` phase
  type from this RFC.
- **Run Y (normal):** uses the current canonical multi-lane review
  shape (designer-per-lane → reviewer-per-lane → synthesis).

Same goal, same input, same frontier-model lane set, separate
worktrees (or repos). Run them simultaneously. The deltas are the
signal.

### What to pick

The input must have **genuine disagreement potential**. A
rename-a-flag RFC will not differentiate the shapes; you will see
the committee's overhead with no upside to offset it. Pick a topic
where the lanes plausibly disagree — schema choice, abstraction
boundary, posture trade-off, lifecycle policy. The RFC 0052 vs.
single-reviewer comparison is itself a candidate (recursive, but
clean).

### Instrumentation (pin BEFORE the runs start)

- **Artifact count per run.** Committee:
  `debate_turn` + `arbitration_ruling` + `panel_vote` +
  `panel_verdict` + `debate_synthesis`. Normal: producer +
  review + synthesis artifacts.
- **Wall-clock per run** (from `run prepare` to terminal state).
- **Model spend per lane per run** (token totals, cost estimate).
- **Final-artifact diff.** Side-by-side comparison of the
  produced design / synthesis / decision. The substantive
  question.
- **Operator interventions per run.** Counts of
  publish-on-behalf, cycle-exhaustion overrides, escalations to
  the human principal (per RFC 0053), declared-blocker
  incidents.
- **Stalemate or escalation rate.** Per-topic resolution counts:
  consensus / panel-decided / stalemate. Committee-side only;
  the normal shape's analog is the override-verdict rate.

Pinning the metric list before the runs start prevents
retrofitting metrics to whichever outcome looks better — the
single biggest threat to this experiment's credibility.

### What success looks like

The committee shape earns its place in the catalog if it produces
**either**: (a) a materially better final artifact for equivalent
process cost, or (b) an equivalent final artifact at materially
lower process cost (lower operator intervention, fewer
revision cycles, shorter wall-clock despite higher artifact
count). Equivalent artifact at higher process cost = the shape
does not earn it for this class of task; the RFC is rejected or
narrowed.

### What this benchmark can't tell us

- Long-tail edge cases the first run doesn't surface (arbitrator
  capture across many phases, panel quorum availability, RFC 0052
  Open question 4's artifact-volume ceiling).
- Whether the shape generalises beyond the one input class
  picked. Treat as the first data point, not the final verdict.
- Whether the human is biased reading both outputs (they know
  which run produced which artifact; blinding is impractical).
  This bias risk is real; the structural metrics above are the
  hedge against it.

Phase A acceptance for this RFC is contingent on ≥1 A/B run
completed with the full instrumentation list captured and a
written `comparison_synthesis` artifact (new kind, or
appropriated existing `synthesis`) summarising the deltas.

## Phasing

- **Phase 0 (this RFC):** validation + acceptance of the shape.
  Concrete schemas and method names land in a follow-up design doc
  authored by the standard three-lane process (D032).
- **Phase A (V1.9 or V2.0):** V1 implementation — debate_turn /
  arbitration_ruling / panel_vote / panel_verdict / debate_synthesis
  artifact kinds; `committee_deliberation` phase type in the RFC 0045
  schema; validator rules; new daemon RPC methods. **A/B validation
  experiment (see above) is Phase A acceptance criterion.**
  Tight-loop lease class deferred to V1.5.
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
