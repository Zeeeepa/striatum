---
schema_version: "striatum.auto_finalize_gate_evidence.v1"
artifact_kind: "auto_finalize_gate_evidence"
decision_id: "D125"
gate_status: "pending"
live_success_count: 2
lane_shape_count: 2
lane_shapes: ["operator_self_declared_review", "operator_self_declared_build"]
contested_audit_chain_events: 1
evidence_artifacts: ["docs/operator/artifacts/d125-auto-finalize-live-evidence-2026-05-23/evidence.json", "docs/operator/artifacts/d125-auto-finalize-live-build-evidence-2026-05-23/evidence.json"]
created_at: "2026-05-23T15:18:30Z"
---

# D125 Auto-Finalize Gate Evidence
author: worker-codex-gpt-5-002

This artifact records the second opt-in live `recovery.auto_finalize`
behavioral success known in this workspace. The new run was
`run_6ff2b4939f9a37987cc9fb38413b8079`, using an operator self-declared build
packet rather than the earlier operator self-declared review packet.

D125 remains pending. The evidence bar still requires three live successes
across at least two lane shapes, and this workspace's repo-level daemon doctor
reported an existing chain-health finding:
`repo_cutover.event_chain.problems[0].problem = "row_hash_mismatch"` for
event `7506`. That finding is not attributed to the new run, but it means this
artifact cannot honestly claim the zero-contested-chain condition.

Global default behavior remains dry-run. The live command used workflow opt-in
from `recovery.auto_finalize.enabled=true` and did not pass `--force`.
