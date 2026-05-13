---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0045", "multi-phase-workflow", "build"]
---

author: reviewer-unknown-model-001

# RFC 0045 Build Review - Threat Model

Verdict: needs_revision.

I reviewed the Python core and frontend handoffs plus the implementation sites they cite. The main trust boundaries introduced by RFC 0045 are: authored workflow JSON to validator, validator-normalized dependency graph to run preparation, generated/upgraded workflow output to later editor/runtime consumers, and web-editor client state to saved workflow JSON. The implementation covers several important attacks: v1 workflows reject `phases` and `phase_synthesis`, cross-phase edges cannot skip phases, runtime materializes phase fan-in dependencies, React text rendering does not use raw HTML, and derived frontend edge metadata is stripped before save. However, the Python and frontend implementations disagree on the canonical job phase field, and that makes v1.1 workflows unsafe to round-trip through the editor.

## Findings

1. High - Python validates `phase_id`, frontend writes `phase`, so editor round-trips can produce invalid v1.1 workflows.

RFC 0045 defines the per-job field as `phase`, but the Python validator only accepts and requires `phase_id` when phases are declared: `src/striatum/workflow.py:783`, `src/striatum/workflow.py:798`, and `src/striatum/workflow.py:862`. The generator also emits `phase_id` in `src/striatum/workflow_generator/core.py:546` and `src/striatum/workflow_generator/core.py:620`. By contrast, the frontend type marks `phase` as canonical and `phase_id` as read-compatible only in `src/striatum/web/frontend/src/shared/types.ts:162`, and the inspector writes `{ phase: ... }` in `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:607`.

Threat impact: a valid generated workflow can be opened by the editor, edited through the phase selector, and saved with a `phase` value that the Python validator rejects as missing `phase_id`. The opposite direction is also ambiguous: the schema-version gate claims v1.1 support, but two implementations serialize different schema dialects. This breaks the backwards-compat and validator-integrity requirement for v1.1 and creates an easy denial-of-workflow path through normal UI use.

Required fix: pick one canonical field and make every layer use it. Since RFC 0045 names `phase`, either migrate Python/generator/tests to `phase` and optionally accept `phase_id` only as a temporary upgrade input, or revise the RFC before accepting the build. Add an integration test that loads a generated `multi_phase` workflow into the editor serialization path and validates the saved JSON with the Python validator.

2. High - `phases[].synthesis_job_id` is not emitted or validated, so phase metadata can drift from the actual gate.

RFC 0045 requires each phase object to identify its synthesis gate. The Python validator instead derives `synthesis_by_phase` by scanning jobs in `src/striatum/workflow.py:849` through `src/striatum/workflow.py:890`, and it never requires or checks `phases[].synthesis_job_id`. The generator creates phase objects with only `id` and `name` in `src/striatum/workflow_generator/core.py:433`, then appends them unchanged in `src/striatum/workflow_generator/core.py:490`. Meanwhile, the frontend phase inspector expects `phase.synthesis_job_id` to find and display the synthesis job in `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:1216`.

Threat impact: the authoritative runtime gate and the operator-visible phase metadata are not the same contract. A generated workflow can validate while the UI shows `(none)` for the synthesis job, and a hand-authored workflow could carry stale or misleading gate metadata without validator refusal. That weakens the audit/operator boundary: the operator can inspect a phase and misunderstand which verdict-bearing job gates the next phase.

Required fix: make `phases[].synthesis_job_id` part of the validated schema, emit it from the generator, and require it to match exactly one same-phase `type: "phase_synthesis"` job. Add negative tests for missing, dangling, cross-phase, and mismatched synthesis ids, plus a frontend fixture that displays the generated gate id.

3. Medium - The editor masks invalid phase references by silently placing unknown-phase jobs in the first band.

`buildPhaseLayout` resolves any missing or unknown job phase to the first phase in `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:150` through `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:155`, and the test suite explicitly locks this behavior in `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts:230`. The Python validator correctly rejects unknown phase references at `src/striatum/workflow.py:868`, but the frontend visual model makes malformed input look like a valid first-phase job.

Threat impact: for untrusted or partially edited workflow JSON, the editor can present an invalid graph as if the job belongs to phase one. Cross-phase edge styling, drag refusal, and the phase inspector then operate on a fabricated phase assignment. That is an output-safety issue: validator errors may be correct later, but the visual output before save is misleading and can cause an operator to make the wrong remediation.

Required fix: render jobs with missing or unknown phase references in an explicit invalid/unassigned area, mark them with an alert, and keep edge styling from using the fallback as truth. Add tests that assert unknown phases are visible as invalid instead of silently bucketed.

## Checks

- Backwards compatibility: Python handoff cites `test_v1_workflow_fixtures_validate_without_phase_progress` and the implementation rejects v1 `phases`, `phase_id`, and `phase_synthesis`; this part is covered.
- V1.1 acceptance: cross-phase validation and runtime fan-in are implemented, including skip-phase refusal in `tests/test_workflow_phases.py::test_cross_phase_edges_cannot_skip_phases`, but the schema field mismatch prevents accepting the build as a coherent v1.1 implementation.
- Validator output safety: errors carry field paths in the Python validator, and the frontend strips derived `crossPhase` metadata before serialization. The remaining output-safety problem is the editor's misleading unknown-phase fallback.
- React Flow v1 rendering: frontend tests and handoff cite unchanged square-grid layout and no cross-phase styling for v1 workflows. I did not run the frontend suite; the frontend handoff states `make ui-test` and `npm test` were not run.
