author: designer-unknown-model-001

# Design: RFC 0045 React Flow Editor Changes

This design covers the React Flow workflow editor changes for RFC 0045 V1, enabling multi-phase workflow visualization and interaction.

## Existing Code Context

The React Flow editor is implemented as a React "island" in:
- `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`
- Supporting types in `src/striatum/web/frontend/src/shared/types.ts`

The editor currently uses a flat list of nodes and edges derived from `workflow.jobs` and `workflow.edges` / `workflow.cycles`.

## 1. Phase Color-Banding

Phases will be rendered as horizontal background bands.

### Implementation
- **Data Structure**: `WorkflowDocument` in `shared/types.ts` already has optional `phases: WorkflowPhase[]`.
- **Canvas Rendering**: We will extend the `ReactFlow` component in `WorkflowGraphEditor.tsx` to include a custom background layer. 
- **Coordinate Calculation**: The `jobsToNodes` function will be updated. Instead of a square grid, nodes will be bucketed into bands:
  - Phase N occupies a Y-range from `N * BAND_HEIGHT` to `(N + 1) * BAND_HEIGHT`.
  - `BAND_HEIGHT` is fixed (e.g., 400px).
  - Within a band, jobs are laid out horizontally based on their `parallel_group` or index.
- **Background Component**: A custom component (e.g., `PhaseBands`) will be rendered as a child of `ReactFlow`. It will map over `workflow.phases` (or a single implicit phase if missing) and render `div` elements with:
  - `position: absolute`
  - `top: index * BAND_HEIGHT`
  - `height: BAND_HEIGHT`
  - `background-color: phase.color` (using a default palette if `color` is missing).
  - A sticky header at the top-left of each band showing the phase `name`.

### Node Bucketing
Jobs are assigned to phase bands by their `phase_id` field. If a job has no `phase_id`, it is assigned to the first phase (or the only implicit phase).

## 2. Cross-Phase Edge Styling

Edges will visually indicate whether they stay within a phase or cross a phase boundary.

### Implementation
In `workflowToEdges`:
- For each edge, compare the `phase_id` of the source job and target job.
- **Intra-phase edges**: Source and target share the same `phase_id`. Render with existing `thin grey` style.
- **Cross-phase edges**: Source and target have different `phase_id`. 
  - Add `className: "cross-phase-edge"` to the React Flow edge object.
  - In `theme.css` (or a component-local style), define `.cross-phase-edge .react-flow__edge-path` as `stroke: #000; stroke-width: 3px;`.
- **Logic Location**: This comparison happens in the `workflowToEdges` loop, referencing the `workflow.jobs` list to look up `phase_id` by job ID.

## 3. Phase Metadata Side Panel

Clicking a phase band header will open a side panel (Inspector-like) for the phase.

### Implementation
- **Interaction**: The `PhaseBands` component's headers will be clickable.
- **State**: A new state `selectedPhaseId: string | null` will be added to `WorkflowGraphEditorImpl`.
- **UI**: When `selectedPhaseId` is set, the right-hand Inspector panel will switch to "Phase Inspector" mode.
- **Content**:
  - Phase `name` and `description`.
  - List of jobs belonging to this phase (filtered from `workflow.jobs`).
  - Progress summary: Calculated by checking the status of jobs in this phase from the existing status feed (passed via props or fetched).

## 4. Drag-Drop Respecting Phase Boundaries

Dragging a job across a phase boundary requires specific behavior.

### Choice: Option (a) - Update and Prompt
When a job is dropped into a different phase band:
1. Update the job's `phase_id` to match the new band.
2. If the move creates a dependency violation (e.g., the job now depends on a job in a future phase or a non-synthesis job in a prior phase), show an inline modal/toast prompting the operator to:
   - "Fix dependencies: Add phase_synthesis edge from [Prior Phase]?"
   - "Cancel move."

**Justification**: This allows for interactive workflow restructuring while maintaining schema validity. Automated "refusal" (Option b) is too rigid for a builder tool where operators may be mid-refactor.

## 5. Backwards Compatibility

V1 workflows (without `phases`) must render as they do today.

### Implementation
- **Conditional Path**: In `WorkflowGraphEditorImpl`, if `workflow.phases` is undefined or empty:
  - Do not render the `PhaseBands` component.
  - `jobsToNodes` uses the existing square grid layout.
  - `workflowToEdges` treats all edges as intra-phase (thin grey).
- **Check**: `const isMultiPhase = !!workflow.phases?.length;`

## Cited Code Paths

- **Node Generation**: `jobsToNodes(workflow: WorkflowDocument): Node[]` (line 69).
- **Edge Generation**: `workflowToEdges(workflow: WorkflowDocument): Edge[]` (line 84).
- **Layout Sync**: `syncWorkflowEdges` (line 117) and `syncWorkflowJobs` (line 112).
- **Main Render**: `WorkflowGraphEditorImpl` (line 464).
- **Types**: `WorkflowDocument` and `WorkflowJob` in `src/striatum/web/frontend/src/shared/types.ts`.
