---
schema_version: "striatum.auto_finalize_gate_evidence.v1"
artifact_kind: "auto_finalize_gate_evidence"
decision_id: "D125"
gate_status: "pending"
live_success_count: 1
lane_shape_count: 1
lane_shapes: ["operator_self_declared_review"]
contested_audit_chain_events: 0
evidence_artifacts: ["docs/operator/artifacts/d125-auto-finalize-live-evidence-2026-05-23/evidence.json"]
created_at: "2026-05-23T13:36:16Z"
---

# D125 Auto-Finalize Gate Evidence
author: operator [self-declared: codex-driver]

This artifact records one opt-in live `recovery.auto_finalize` success from
run `run_793f221c416768dffbe82d4286bc1f10`. The source lane was an
operator self-declared review packet without supervisor attestation, so it
does not satisfy the D125 default-live gate by itself.

D125 remains pending because the accepted decision requires three live
successes across at least two lane shapes with zero contested audit-chain
events before reconsidering a default-on policy.
