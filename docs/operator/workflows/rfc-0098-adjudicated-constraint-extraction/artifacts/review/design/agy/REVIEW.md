---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
---

# Design Review - Devil's Advocate

author: operator

## Verdict

accept_with_findings

## Interrogation

Opened interrogation `intg_3d10e390908bd4e6cf6081d0ad9a5186` against the live synthesizer session `sess_76fe26aae7c4d815f28731663ba4804b`.

I used **2** interrogation rounds. I stopped because both core adversarial findings/concerns from our posture were fully and thoroughly resolved by the synthesizer's exceptionally robust, mathematically rigorous, and concrete answers. No further rounds were necessary as the gaps were successfully closed.

- **Round 1 (Hollow Constraint Evasion):** We probed how the productive-refusal validator prevents a model from emitting trivial/hollow constraints (e.g. "ensure code is correct") to bypass the gate. The synthesizer conceded that the structural validator alone cannot evaluate quality (per RFC 0098 Non-Goals), but detailed a robust defense-in-depth model (sourcing, posture-disposition matrix coupling, interrogation loops, and discharge verification). Most importantly, the synthesizer offered an actionable structural hardening option for V1: making `source_finding` resolve to an active, `high|critical` finding and requiring a non-empty `verification.gate` on all `binding: true` constraints.
- **Round 2 (Phase-Ordering/Cycle wedging under #66):** We probed whether the `adjudication -> revision_constraints_intake` workaround creates a graph cycle or violates the cross-phase synthesis constraints under `run.prepare`. The synthesizer clarified that this workaround is a forward edge, not a cycle, because `revision_constraints_intake` is a non-synthesis job situated inside `revision_synthesis` (phase N+1). The actual cycle/loop is mapped to the attempt dimension (re-opens are attempts with cycle-aware logical names), meaning the static graph remains a strictly forward acyclic DAG. The synthesizer also sensibly noted that because daemon-side strictness remains a residual risk, Slice 1 (pure contract, no graph dependency) is prioritized as the V1 baseline, which we strongly endorse.

## Gaps And Risk Analysis

- **Validator Evasion Surface:** A structural-only gate is inherently blind to the semantic substance of the constraints. While defense-in-depth is the correct architectural posture, leaving `source_finding` entirely optional/non-resolving in Slice 1 creates a low-friction evasion path for lazy or misaligned models.
- **Strict Cross-Phase Enforcement Risk:** Although the intake job retargeting is acyclic, the live daemon's cross-phase checks could be stricter than the source code's literal interpretation (e.g., rejecting any cross-phase edges to non-synthesis targets). This poses a moderate risk to Slice 2 generation.

## Findings

### F1 - Productive Refusal Gate Evasion via Hollow Constraints

Severity: low

The synthesis relies on structural checks for the productive refusal gate (`binding == true` or `kind == "unresolved_question"`). While semantic scoring is a Non-Goal, a completely hollow constraint can satisfy this check. 

**Recommendation:** Incorporate the synthesizer's proposed structural hardening in Slice 1:
- Require that any `binding: true` constraint carries a `source_finding` pointing to a valid, open finding in the `findings[]` block.
- Require a non-empty `verification.gate` or `verification.expected_stage` for all binding constraints. This forces structural grounding without resorting to model calls or semantic scoring inside the daemon.

### F2 - Graph cycle and strict run.prepare enforcement risk

Severity: medium

The `adjudication -> revision_constraints_intake` retargeting is logically sound and mathematically acyclic (mapping loops to attempts rather than static edges). However, there is a risk that the live daemon's phase validation is strict on edge source/target classes.

**Recommendation:** Endorse the synthesizer's risk-mitigated strategy. Ensure that Slice 1 (pure contract, no graph change) lands cleanly first, and treat Slice 2 as a gated stretch goal verified against live `run.prepare` and `workflow validate` before committing to the repository shape pack.

## Positive Findings

- **Verdict state machine preservation:** Rejecting the widening of the front-matter `verdict` enum is an excellent, high-safety decision. Keeping the refined states (`blocked_pending_answer`, `defer_with_successor`) as `branches{}` dispositions prevents runtime wedges at `recordVerdict` while fulfilling product requirements.
- **Free-string posture:** Allowing `posture` to be a non-empty free string instead of a closed enum is correct, preserving custom posture configuration flexibility as outlined in RFC 0098 §2.
- **cycle advisory metadata:** Adding the `cycle` field as advisory metadata in Slice 1 cleanly resolves trace tracking without polluting the hash resolution path.

## Actionable Recommendations Before Build

1. Document the structural hardening rules (grounded `source_finding` and non-empty `verification.gate`) as optional or highly recommended guidelines in the Slice 1 contract tests.
2. Maintain strict separation between Slice 1 contract changes and Slice 2 graph changes, ensuring a clean deferral boundary if `run.prepare` rejects the intake edge.
