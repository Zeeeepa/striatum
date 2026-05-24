---
schema_version: "striatum.auto_finalize_gate_evidence.v1"
artifact_kind: "auto_finalize_gate_evidence"
decision_id: "D125"
gate_status: "satisfied"
live_success_count: 3
lane_shape_count: 3
lane_shapes: ["operator_self_declared_review", "operator_self_declared_build", "operator_self_declared_synthesis"]
contested_audit_chain_events: 0
evidence_artifacts: ["docs/operator/artifacts/d125-auto-finalize-live-evidence-2026-05-23/evidence.json", "docs/operator/artifacts/d125-auto-finalize-live-build-evidence-2026-05-23/evidence.json", "docs/operator/artifacts/d125-auto-finalize-live-synthesis-evidence-2026-05-24/evidence.json"]
created_at: "2026-05-24T00:55:42Z"
---

# D125 Auto-Finalize Gate Evidence
author: operator-codex-001

This artifact records the third opt-in live `recovery.auto_finalize`
behavioral success known in this workspace. The new run was
`run_3d182acb046f7b09dbc0dbd9a3a90363`, using an operator self-declared
synthesis packet rather than the earlier review and build packets.

D125's evidence gate is now satisfied for this repository state: three live
successes exist across three lane shapes, and the current daemon doctor reports
no audit-chain problems. The global default remains dry-run unless and until a
separate implementation changes the default-live policy.
