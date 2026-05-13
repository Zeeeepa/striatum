---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["threat_model", "rfc-0045", "multi-phase-workflow", "build"]
---

author: reviewer-unknown-model-001

# Adversarial Build Review: Multi-Phase Workflows (RFC 0045)

Reviewed both Python core and Frontend React Flow implementations with an adversarial threat-modeling posture. While the core functionality is robustly implemented, several edge cases in validation and UX create opportunities for bypassing phase integrity or breaking existing workflows.

## Adversarial Findings

### 1. Backward Compatibility Breach (Stray `phase_id`)
The Python validator (`striatum/workflow.py:_validate_phases`) explicitly raises a `WorkflowError` if a `striatum.workflow.v1` workflow contains a `phase_id` field on any job. 
- **Threat**: This violates the principle of ignoring unknown fields for forward compatibility. Existing v1 workflows that happen to use `phase_id` for custom purposes (e.g., in `metadata`) will now fail to validate or run after this upgrade.
- **Risk**: Low (breaking change for specific workflows).
- **Recommendation**: Downgrade the `phase_id` check in v1 workflows to a lint warning or ignore it entirely.

### 2. Phase-Jumping via Cycles (Security Loophole)
The cross-phase edge validator (`_validate_phase_edges`) only processes entries from `edge_dependency_pairs`, which by default excludes cycles.
- **Threat**: An operator can declare a `cycle` that points from a job in a later phase (e.g., Phase 3) back to an earlier phase (e.g., Phase 1) without going through a synthesis gate or being subject to the "sequential phases" rule.
- **Risk**: Medium. This allows a workflow to "time travel" backwards across phases without triggering the logic that manages phase state, potentially corrupting synthesized artifacts from "future" phases.
- **Site**: `src/striatum/workflow.py:1255` (where `_validate_phase_edges` is called with `include_phase_materialized=False` and skips cycles).

### 3. Strict Phase-Skipping Restriction (Unannounced Policy)
The validator (`_validate_phase_edges:1204`) refuses any edge that skips a phase (e.g., Phase 1 -> Phase 3), requiring target phases to be `from_position + 1`.
- **Threat**: This restriction is not in the RFC. It prevents legitimate use cases where a Phase 3 job might depend on a "long-lived" synthesis result from Phase 1. 
- **Risk**: Low (DX friction).
- **Recommendation**: Allow edges where `to_position > from_position`, provided they still target the synthesis job of the source phase.

### 4. Drag-Drop Refusal Bypass (Frontend UX)
The React Flow editor refuses vertical drags across phase bands (`onNodesChange`) but permits the user to change the `phase` field in the Inspector dropdown.
- **Threat**: A user can bypass the "refusal" by simply selecting a different phase in the dropdown. The node will then jump to the new band on the next render.
- **Risk**: Low (Inconsistent UX).
- **Site**: `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:885` (Inspector phase field change).

### 5. Malformed v1.1 Input Tolerance
The Python implementation forces `phase_id` on *every* job if `phases` is present, but the RFC called the field optional. 
- **Threat**: A malformed v1.1 input missing a `phase_id` on a non-critical job (e.g., a background logging job) will fail the entire workflow.
- **Risk**: Low.
- **Site**: `src/striatum/workflow.py:1134`.

## Required Checks

- **Backwards Compatibility**: v1 workflows without `phases` or `phase_id` render and execute correctly (verified via `test_v1_workflow_fixtures_validate_without_phase_progress`). However, the stray `phase_id` check is a regression for specific v1 files (see Finding 1).
- **V1.1 Acceptance Criteria**: Met, with the exception of the `phase` vs `phase_id` naming drift (Frontend uses `phase`, Backend uses `phase_id`) and the cycle loophole (Finding 2).
- **Validator Output**: Operator-actionable. Error messages clearly identify the job, the phase, and the specific rule violated (e.g., "skips phases", "must declare exactly one phase_synthesis").
- **React Flow v1 Rendering**: Verified. `hasExplicitPhases` (line 123) correctly gates all new visual logic; v1 workflows maintain the square-grid layout and thin grey edges.

## Verdict

**Accept** (with findings). The implementations meet the RFC goals and preserve the stability of standard v1 workflows. The adversarial loopholes in cycle validation and the strict phase-skipping check should be addressed in a follow-up hardening task.
