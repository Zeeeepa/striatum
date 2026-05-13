---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/043/design/frontend/codex/DESIGN.md", "docs/dogfood/043/design/frontend/claude_code/DESIGN.md", "docs/dogfood/043/design/frontend/gemini/DESIGN.md"]
---
author: designer-unknown-model-004

# Frontend Synthesis: RFC 0045 React Flow Editor

Status: implementation plan
Target: RFC 0045 V1 React Flow editor slice

## Chosen Shape

Implement multi-phase rendering inside the existing `workflow-graph-editor`
React island. The V1 editor extension is visual and field-editing oriented:
phase bands, phase selection, cross-phase edge styling, and explicit
job-phase editing. It does not introduce phase creation, phase reordering,
edge auto-rewrite, persisted coordinates, or a new graph layout engine.

The three designs agree on extending the current island in place. The only
material disagreement is drag-drop behavior; choose refusal for V1 because
rewriting a job's phase on drop can immediately create validator-invalid
cross-phase dependencies unless the editor also performs a multi-edge
schema repair flow.

## Phase Data Contract

Add explicit frontend types in
`src/striatum/web/frontend/src/shared/types.ts`:

```ts
export interface WorkflowPhase {
  id: string;
  title?: string;
  name?: string;
  description?: string;
  synthesis_job_id?: string;
  color?: string;
}
```

Extend `WorkflowDocument` with `phases?: WorkflowPhase[]` and
`WorkflowJob` with `phase?: string` and `phase_id?: string`. RFC 0045 names
the persisted job field `phase`; the frontend should normalize with
`jobPhaseId(job) = String(job.phase ?? job.phase_id ?? "")` and write
`phase` from the inspector. `phase_id` remains read-compatible only so early
fixtures or stale editor payloads do not break.

Phase display name is `phase.title ?? phase.name ?? phase.id`. Phase color
uses `phase.color` when present; otherwise use a deterministic index palette
inside the editor. Do not require `phases[].color` in RFC 0045 V1 schema.

## Phase Color Bands

Use a custom React Flow child overlay, not `BackgroundVariant` and not
background pattern fills. Implement `PhaseBands` in
`src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`
and render it as the first child of `<ReactFlow>`, before `<Background />`:

```tsx
<PhaseBands
  phases={phaseLayout.phases}
  selectedPhaseId={selection?.kind === "phase" ? selection.id : null}
  onSelectPhase={(id) => setSelection({ kind: "phase", id })}
/>
<Background />
```

`PhaseBands` renders absolutely positioned `<div>` bands in world
coordinates with a clickable header strip. Use these layout constants in the
same file:

```ts
const PHASE_BAND_HEIGHT = 320;
const PHASE_HEADER_HEIGHT = 36;
const PHASE_NODE_TOP = 72;
const PHASE_COLUMN_WIDTH = 260;
const PHASE_ROW_HEIGHT = 96;
```

Refactor `jobsToNodes(workflow)` to keep the current square grid unchanged
when `(workflow.phases ?? []).length === 0`. When phases exist, bucket jobs by
`jobPhaseId(job)`, place unknown or missing phase ids in the first explicit
phase, and lay jobs within each band by first-seen `parallel_group` and then
job order:

```ts
x = groupIndex * PHASE_COLUMN_WIDTH + rowIndexInGroup * 24;
y = phaseIndex * PHASE_BAND_HEIGHT + PHASE_NODE_TOP + rowIndexInGroup * PHASE_ROW_HEIGHT;
```

Coordinates remain UI-only and must not be written back to `workflow.json`.
The band overlay is visual context only; phases are not React Flow nodes.

Add band styles to
`src/striatum/web/frontend/src/shared/theme.css` near the existing workflow
graph editor styles. Use low-opacity backgrounds such as
`rgba(102, 187, 255, 0.10)`, `rgba(168, 230, 207, 0.10)`,
`rgba(255, 211, 165, 0.10)`, and `rgba(204, 173, 255, 0.10)` through local
CSS classes or custom properties. The header uses existing tokens:
`background: var(--bg-overlay)`, `color: var(--fg-muted)`, and
`border: 1px solid var(--border)`.

## Cross-Phase Edges

Extend `workflowToEdges(workflow)` in
`WorkflowGraphEditor.tsx`. Build a `Map<string, string>` from job id to
normalized phase id. For each normal workflow edge, compare source and target
phase ids. Intra-phase edges keep the existing default thin grey path.
Cross-phase edges get this exact React Flow edge shape:

```ts
{
  ...edge,
  className: "cross-phase-edge",
  style: { stroke: "#000", strokeWidth: 3 },
  data: { on: e.on ?? "completed", crossPhase: true, sourcePhase, targetPhase },
}
```

Also add the CSS rule as a backstop beside the current cycle-edge rule:

```css
.react-flow__edge.cross-phase-edge .react-flow__edge-path {
  stroke: #000;
  stroke-width: 3;
}
```

The conditional branch belongs directly in the normal `workflow.edges` loop.
Cycle edges remain `cycle-edge`; if a cycle ever crosses phases, combine
classes as `"cycle-edge cross-phase-edge"` and let the cross-phase black
stroke win while preserving the cycle dash rule. `syncWorkflowEdges` must
continue to serialize only `from`, `to`, and `on` for workflow edges and must
drop derived `crossPhase`, `sourcePhase`, and `targetPhase` metadata.

## Phase Metadata Panel

Replace `selectedJobId` with a discriminated selection state in
`WorkflowGraphEditor.tsx`:

```ts
type GraphSelection =
  | { kind: "job"; id: string }
  | { kind: "phase"; id: string }
  | null;
```

Keep the existing job inspector behavior for `selection.kind === "job"`.
Add a new `PhaseInspector` component in the same file with this props
contract:

```ts
interface PhaseInspectorProps {
  phase: WorkflowPhase;
  jobs: WorkflowJob[];
  synthesisJob: WorkflowJob | undefined;
  selectedJobId?: string;
  onSelectJob: (jobId: string) => void;
  onChangePhase: (phaseId: string, patch: Partial<WorkflowPhase>) => void;
}
```

Mount it in the existing right-hand inspector slot. It displays and edits
`title` and optional `description`, shows `synthesis_job_id`, lists jobs in
the phase with `id`, `type`, `lane_id`, and `parallel_group`, and lets a job
row switch selection back to the job inspector. Do not require live run
status for V1; a future `status --json` phases block can extend this props
contract with optional progress data after the backend lands it.

Add a job-phase selector to the existing `Inspector` only when explicit
phases exist. It writes `phase`, not `phase_id`, and then recomputes
`jobsToNodes(nextWorkflow)` and `workflowToEdges(nextWorkflow)`.

## Drag-Drop Policy

Refuse cross-band drops with an inline error message. On drag start, remember
the prior node position map. On drag stop or wrapped `onNodesChange`, compute
the destination band from the node's Y coordinate and compare it with the
job's normalized phase. If it differs, restore the previous position and set:

```text
Drag refused: <job_id> belongs to phase <phase_id>. Use the inspector phase field to move it, then add the required phase_synthesis dependency.
```

Intra-band dragging updates only React Flow node state. It never updates the
workflow JSON because coordinates are not persisted today.

## Backwards Compatibility

The single predicate is:

```ts
const hasExplicitPhases = (workflow.phases ?? []).length > 0;
```

When false, `jobsToNodes` uses the current square grid, `workflowToEdges`
emits no `cross-phase-edge` classes or styles, `PhaseBands` returns `null`,
the inspector remains job-only, no phase selector is rendered, and
drag-boundary checks are skipped. Do not synthesize a visible implicit phase
for v1 workflows.

## Files To Touch

`src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`
: Add phase helpers, phase-aware node layout, cross-phase edge tagging,
`PhaseBands`, `PhaseInspector`, selection union, inspector phase selector,
and drag-refusal logic.

`src/striatum/web/frontend/src/shared/types.ts`
: Add `WorkflowPhase`, `WorkflowDocument.phases`, and explicit `phase` /
read-compatible `phase_id` fields on `WorkflowJob`.

`src/striatum/web/frontend/src/shared/theme.css`
: Add phase band, phase header, phase inspector list, and cross-phase edge
styles next to the existing workflow graph editor rules.

`src/striatum/web/frontend/src/islands/workflow-graph-editor/workflow-graph-editor.test.ts`
: Add unit coverage for v1 unchanged layout, v1.1 phase bucketing,
cross-phase edge metadata/style, `syncWorkflowEdges` dropping derived edge
metadata, phase inspector job filtering, and cross-band drag refusal.

No other `src/islands/` files are part of the V1 frontend slice.
