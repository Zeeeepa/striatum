---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
author: adjudicator-unknown-model-004
workflow: "rfc-0119-ratify"
run_id: "run_b089555bd70cd2dc2dc1d13c3cc35b53"
cycle: 1
topic: "RFC 0119 warm-tier memory boundary and striatum-native hot tier ratification gate"
participants:
  - holder
  - falsifier_1
  - falsifier_2
  - adjudicator
entries:
  - kind: claim
    by: holder
    refs: ["dialogue:1"]
    text: "The holder argued RFC 0119 should be accepted because the proposed warm-tier and hot-tier boundaries preserve the corpus invariants, keep recall out of consumer canonical state, and leave durable provenance in git while routing exhaust to eviction-eligible storage."
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    text: "Falsifier 1 showed the proposed hot-tier injection seams are not structurally outside state transitions: buildPacket is called inside claimChosenJob on the claim transaction, and the worktree-create seam runs after the physical git worktree add but before the row is recorded."
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    text: "Falsifier 2 showed the worked exhaust definition conflicts with RFC 0072 and the corpus durable-provenance bundle by making operator_report and broad ledger globs eviction-eligible, and that lane_trajectory relaxes existing transcript-denial behavior without a deterministic redaction contract."
  - kind: rebuttal
    by: holder
    refs: ["dialogue:1"]
    text: "The holder's strongest rebuttal is that the core axes are still viable: recall can be fail-soft, the external-consumer invariant is preserved because the read is local to Striatum, and the eviction axis can be narrowed without abandoning the boundary."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:2"]
    text: "C1: RFC 0119 must re-ground the hot-tier read as a structural guarantee: no recall read may run on the claim transaction, the digest write must be fail-soft outside the transition, and a recall-failure guardrail must prove claim and worktree-create still commit."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:2"]
    text: "C2: RFC 0119 must address the worktree-create orphan window so a digest render failure cannot leave a physical worktree without the corresponding job_worktrees row."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:3"]
    text: "C3: RFC 0119 must define exhaust with an explicit per-kind allow-list that reconciles with RFC 0072 and excludes git-tracked durable provenance, decisional ledgers, and corpus-export durable-provenance kinds."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:3"]
    text: "C4: RFC 0119 must explicitly own the lane_trajectory relaxation of transcript-denial behavior, supersede the prior guardrail/decision where required, and either specify deterministic redaction-normalization or remove lane_trajectory from the durable provenance bundle."
verdict: "accept_with_findings"
rationale: "The gate clears with binding constraints. The falsifiers' decisive objections are verified and binding, but they are dischargeable without weakening the corpus invariants: the hot-tier read can be moved off transition transactions and the exhaust policy can be made a strict allow-list aligned with RFC 0072. RFC 0119 must be amended to discharge C1-C4 before a decision record is filed or Go implementation begins."
findings:
  - id: F-HOT-TX
    severity: critical
    posture: state_transition_dependence
    status: converted_to_constraint
    challenge: "The proposed hot-tier read is described as not being a state transition, but the named buildPacket seam is inside the claim transaction and can abort claim progress if recall fails."
    source_refs: ["dialogue:2"]
  - id: F-WORKTREE-ORPHAN
    severity: high
    posture: state_transition_dependence
    status: converted_to_constraint
    challenge: "The worktree-create seam can run after the physical git worktree exists and before Striatum records the row, so a digest-render failure can orphan worktree state."
    source_refs: ["dialogue:2"]
  - id: F-EXHAUST-ALLOWLIST
    severity: critical
    posture: durable_provenance
    status: converted_to_constraint
    challenge: "The worked exhaust definition conflicts with RFC 0072 and durable provenance by making operator_report and broad ledger globs eviction-eligible."
    source_refs: ["dialogue:3"]
  - id: F-LANE-TRAJECTORY
    severity: high
    posture: export_class
    status: converted_to_constraint
    challenge: "lane_trajectory relaxes existing transcript-denial behavior without specifying the redaction, normalization, supersession, and determinism contract needed for durable provenance."
    source_refs: ["dialogue:3"]
constraints:
  - id: C1-HOT-TIER-SEPARATE-CONNECTION
    source_finding: F-HOT-TX
    posture: state_transition_dependence
    severity: critical
    kind: gate
    binding: true
    text: "Hot-tier recall must run on a separate connection, never the claim transaction; digest write failure must be fail-soft outside the transition; and a recall-failure guardrail must assert claim and worktree-create still commit."
    source_refs: ["dialogue:2"]
    verification:
      gate: "Inject recall-read failures and verify claim plus worktree-create commit without rollback."
    final_review_required: true
  - id: C2-WORKTREE-ORPHAN-GUARD
    source_finding: F-WORKTREE-ORPHAN
    posture: state_transition_dependence
    severity: high
    kind: gate
    binding: true
    text: "The worktree-create digest render must be ordered and fail-soft so render failure cannot leave a dangling git worktree without a job_worktrees row."
    source_refs: ["dialogue:2"]
    verification:
      gate: "Inject digest-render failure during worktree create and assert no unrecorded worktree remains."
    final_review_required: true
  - id: C3-EXHAUST-ALLOWLIST
    source_finding: F-EXHAUST-ALLOWLIST
    posture: durable_provenance
    severity: critical
    kind: policy
    binding: true
    text: "Exhaust must be an explicit per-kind allow-list aligned with RFC 0072, excluding operator_report, decision, escalation, operator_brief, work_plan, collaboration_ledger, and corpus-export durable-provenance kinds."
    source_refs: ["dialogue:3"]
    verification:
      expected_stage: "RFC text and decision record list allowed exhaust kinds explicitly and forbid broad ledger globs."
    final_review_required: true
  - id: C4-LANE-TRAJECTORY-CONTRACT
    source_finding: F-LANE-TRAJECTORY
    posture: export_class
    severity: high
    kind: policy
    binding: true
    text: "lane_trajectory must explicitly supersede the transcript-denial guardrail/decision where required and either define deterministic redaction-normalization to bytes or stay outside the durable provenance bundle."
    source_refs: ["dialogue:3"]
    verification:
      expected_stage: "RFC text names the supersession, redaction-normalization contract, and whether lane_trajectory is durable provenance."
    final_review_required: true
branches:
  state_transition_dependence: "cleared_with_constraints"
  durable_provenance: "cleared_with_constraints"
  export_class: "cleared_with_constraints"
---

# Collaboration Ledger

RFC 0119 is accepted with findings only. The RFC text and decision record must
discharge C1-C4 before implementation work begins.
