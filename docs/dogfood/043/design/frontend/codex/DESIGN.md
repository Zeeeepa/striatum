author: designer-unknown-model-001

# RFC 0045 Frontend Design: Multi-Phase React Flow Editor

Status: design handoff
Target: `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`

## Existing Surface To Extend

The workflow editor is already a React Flow island, not a server-rendered graph. The current file header says nodes are workflow jobs, edges come from workflow `edges`, cycles get a distinct `cycle-edge` class, coordinates are UI-only, and save posts the edited workflow JSON with the adjacent `workflow-sha256` value (`WorkflowGraphEditor.tsx:1`, `WorkflowGraphEditor.tsx:12`). The concrete extension points are:

- `jobsToNodes(workflow)` currently lays jobs into a square-ish grid by index and emits `Node[]` with `id`, `position`, and a two-line label (`WorkflowGraphEditor.tsx:72`).
- `workflowToEdges(workflow)` currently maps `workflow.edges` into React Flow `Edge[]`, then maps `workflow.cycles` into `className: "cycle-edge"` edges (`WorkflowGraphEditor.tsx:86`, `WorkflowGraphEditor.tsx:97`).
- `WorkflowGraphEditorImpl` stores `workflow`, `nodes`, `edges`, and `selectedJobId` in component state, then renders `<ReactFlow nodes={nodes} edges={edges} ...>` with `Background`, `MiniMap`, and `Controls` (`WorkflowGraphEditor.tsx:655`, `WorkflowGraphEditor.tsx:670`, `WorkflowGraphEditor.tsx:842`).
- Existing CSS already centralizes React Flow edge styling: default paths use `stroke: var(--border)`, selected paths use `var(--accent)`, and `.cycle-edge` uses `var(--status-needs_revision)` plus dashes (`theme.css:424`, `theme.css:441`, `theme.css:449`).
- `shared/types.ts` is the island prop/API contract; it currently allows unknown workflow/job fields through index signatures, while first-class fields stop at `parallel_group` on jobs and `jobs`/`edges`/`cycles` on documents (`types.ts:135`, `types.ts:178`).

## Type And Naming Contract

Add explicit frontend types without breaking older JSON:

```ts
interface WorkflowPhase {
  id: string;
  name?: string;
  title?: string;
  description?: string;
  color?: string;
  synthesis_job_id?: string;
}
```

Extend `WorkflowDocument` with `phases?: WorkflowPhase[]` and `WorkflowJob` with `phase_id?: string` plus `phase?: string`. RFC 0045's prose currently uses job `phase`, while this frontend work packet says `phase_id`; the editor should normalize with `jobPhaseId(job) = String(job.phase_id ?? job.phase ?? "")`. When writing a moved job, prefer `phase_id` for this Track B contract unless Track A settles the persisted field as `phase`, in which case the helper can flip one line.

Phase display name should be `phase.name ?? phase.title ?? phase.id`. Description is optional. Color uses `phase.color` if present; otherwise use a deterministic fallback palette only for v1.1 workflows whose phase lacks a color. For `striatum.workflow.v1` with no phases array, do not synthesize visible colors.

## Phase Color-Banding

Render phases as horizontal background bands inside the React Flow canvas, not as nodes. Do this with a custom `PhaseBands` child of `<ReactFlow>` positioned in the viewport layer under nodes and edges. The component should render one absolutely positioned band per explicit phase:

```tsx
<PhaseBands
  phases={phaseLayout.phases}
  selectedPhaseId={selectedPhaseId}
  onSelectPhase={setSelectedPhaseId}
/>
```

Each band has a fixed world-coordinate rectangle:

- `PHASE_BAND_HEIGHT = 320`
- `PHASE_HEADER_HEIGHT = 36`
- `PHASE_NODE_TOP = 72`
- phase index `i` starts at `y = i * PHASE_BAND_HEIGHT`

`jobsToNodes` should become layout-aware. Keep the current grid path unchanged when `workflow.phases` is absent or empty. For multi-phase workflows, bucket every job by `jobPhaseId(job)`. A job whose phase is missing or unknown should be bucketed into the first explicit phase and flagged in the node data so the inspector can surface the schema issue after Track A validation lands.

Within a phase, place nodes by `parallel_group` first, then stable job index. A simple v1 layout is enough:

- group by `parallel_group ?? "_ungrouped"`
- sort groups by first appearance in `workflow.jobs`
- x = group index * 260 + row index inside group * 24
- y = phase band top + `PHASE_NODE_TOP` + row index inside group * 96

This preserves RFC 0045's goal of phase recognition without taking on edge rerouting. The node positions remain UI-only, consistent with the existing file comment that coordinates are never persisted (`WorkflowGraphEditor.tsx:12`).

The band header is a button-like canvas label, not a graph node. It should include the phase name and an accessible label such as `Open phase details for Phase 1`. The header click sets `selectedPhaseId` and clears `selectedJobId`; `onPaneClick` should clear both selections.

## Cross-Phase Edge Styling

Extend `workflowToEdges(workflow)` because that is the existing edge construction path (`WorkflowGraphEditor.tsx:86`). Build a `Map<jobId, phaseId>` from normalized job phases, then for each workflow edge compare `sourcePhase !== targetPhase`:

- Intra-phase edge: keep today's default class and style. CSS already makes default paths thin grey via `.react-flow__edge-path { stroke: var(--border); }` (`theme.css:441`).
- Cross-phase edge: add `className: "cross-phase-edge"` and `data: { ...existingData, crossPhase: true, sourcePhase, targetPhase }`.
- Cycle edge: keep `cycle-edge`; if a cycle crosses phases, add both classes as `"cycle-edge cross-phase-edge"` but keep the dashed cycle rule secondary to the black cross-phase stroke.

Add CSS beside the current React Flow overrides:

```css
.react-flow__edge.cross-phase-edge .react-flow__edge-path {
  stroke: #000;
  stroke-width: 3;
}
```

For dark mode, black is still the RFC 0045 acceptance criterion. If readability becomes a review issue, use `#000` for the path and a subtle white halo through a duplicate edge label/background rather than changing the path color.

`syncWorkflowEdges` should not persist the derived cross-phase metadata. It already serializes only `source`, `target`, and `data.on` back into workflow `edges`, and only cycle fields back into `cycles` (`WorkflowGraphEditor.tsx:121`, `WorkflowGraphEditor.tsx:127`). Keep that behavior.

## Phase Metadata Side Panel

Replace the right-hand inspector slot with a selection-aware panel:

- selected job: render the current `Inspector` unchanged.
- selected phase: render a new `PhaseInspector`.
- no selection: render the existing empty inspector text.

`PhaseInspector` receives the phase, jobs in that phase, and optional runtime progress. It displays:

- phase name/title
- description, if present
- synthesis job id, if present
- list of jobs in phase, with type and `parallel_group`
- progress counts by job state

The current editor page only passes workflow file data through `workflow_edit.html` as `data-props`, `workflow-data`, and `workflow-sha256` (`workflow_edit.html:15`, `workflow_edit.html:30`). Runtime progress is not currently part of the edit island. Add an optional `runStatusUrl?: string` prop to `WorkflowGraphEditorProps` rather than making it mandatory. When present, fetch same-origin JSON through `api-client.ts`, following the existing same-origin/no-telemetry pattern (`api-client.ts:1`, `api-client.ts:43`). When absent, the phase panel still renders static job membership and shows progress as unavailable.

Once Track A adds the RFC 0045 `status --json` phases block, the phase panel should consume that block first. Until then, it can compute progress from job summaries if a run-detail/status endpoint supplies per-job state. The design should not scrape DOM tables or terminal output.

## Drag-Drop Across Phase Boundaries

Choose option (b): refuse cross-band drops with an inline message.

Reason: RFC 0045's validator is intended to refuse cross-phase dependencies that bypass the source phase synthesis job. Automatically changing a job's phase would often create or hide invalid edges, and the prompt's option (a) requires prompting for a `phase_synthesis` edge even though edge insertion is a schema-level decision with ordering implications. For V1, dragging a job within its existing band updates only the UI layout. Dragging across a band boundary snaps the node back to its prior position and shows an inline message in the canvas footer: `Move blocked: edit the job phase in the inspector, then add the required phase_synthesis dependency.`

Implementation details:

- Track the previous node positions in `onNodeDragStart`.
- In `onNodeDragStop`, compute destination phase by `node.position.y / PHASE_BAND_HEIGHT`.
- If `destinationPhaseId === jobPhaseId(job)`, keep the new node position only in React state. Do not update workflow JSON because coordinates are UI-only today.
- If it differs, call `setNodes` to restore the previous position and set `dragMessage`.

The inspector may still expose a `phase_id` select for explicit reassignment. That explicit edit should patch the job phase field, recompute nodes with `jobsToNodes(next)`, and rely on save-time validation errors for dependency repair. This keeps drag/drop ergonomic while avoiding silent invalid workflow rewrites.

## Backwards Compatibility

The conditional render path is:

```ts
const explicitPhases = Array.isArray(workflow.phases) ? workflow.phases : [];
const isMultiPhase = explicitPhases.length > 0;
```

When `isMultiPhase` is false:

- call the existing `jobsToNodes` grid behavior exactly as today;
- call the existing `workflowToEdges` behavior with no `cross-phase-edge` classes;
- do not render `PhaseBands`;
- keep the right panel as job inspector only;
- do not add `phase_id` controls;
- do not create an implicit visible band.

This satisfies RFC 0045's backwards-compatibility rule: v1 workflows with no `phases` array render exactly as today, with a single implicit phase in the model but no band UI and no thick edges.

## Test Plan

Extend `workflow-graph-editor.test.ts`, which already tests `jobsToNodes`, `workflowToEdges`, `syncWorkflowEdges`, and palette vocabulary (`workflow-graph-editor.test.ts:13`, `workflow-graph-editor.test.ts:30`, `workflow-graph-editor.test.ts:43`):

- v1 workflow without `phases` returns the same node positions and edge classes as current tests.
- v1.1 workflow with two phases buckets jobs into separate y bands.
- edge from phase A synthesis job to phase B job gets `cross-phase-edge`; edge between two phase A jobs does not.
- `syncWorkflowEdges` drops derived `crossPhase` metadata.
- drag-stop across bands restores position and sets an inline refusal message.
- phase inspector lists only jobs whose normalized `phase_id ?? phase` matches the selected phase.

CSS coverage can stay at component/unit level unless the implementer is already running browser screenshots for the React Flow island; no new edge-routing algorithm is in scope.
