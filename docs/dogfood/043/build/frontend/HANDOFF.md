author: implementer-unknown-model-001

# RFC 0045 Frontend Handoff — Track B (claude React Flow editor)

Run: `run_648f79036ed441ed81073254207389a0`
Job: `implement_frontend`
Synthesis: `docs/dogfood/043/DESIGN_SYNTHESIS_frontend.md`
Branch: `striatum/dogfood-043-rfc-0045`

## Scope shipped

Phase-aware extensions to the existing `workflow-graph-editor` React Flow
island, gated entirely on `(workflow.phases ?? []).length > 0`. v1 workflows
keep the original square-grid layout, thin grey edges, and job-only
inspector — no visual changes for them.

| Concern | Result |
| ------- | ------ |
| Phase color bands (overlay) | Implemented via `<ViewportPortal>` so bands pan/zoom with nodes. Four-entry deterministic palette; `phases[].color` is read but not yet plumbed to band fill (synthesis says color is optional and not required in V1 schema, so I kept the deterministic palette). |
| Cross-phase edge styling | Implemented in `workflowToEdges`. Cross-phase edges get `className: "cross-phase-edge"`, inline `style: { stroke: "#000", strokeWidth: 3 }`, and `data: { crossPhase, sourcePhase, targetPhase }`. CSS rule added beside `.cycle-edge` as a backstop. |
| Phase metadata inspector | `PhaseInspector` component swaps into the right-hand inspector slot when a band header is clicked. Lets the operator edit `title` / `description`, shows `synthesis_job_id`, lists jobs in the phase, and clicking a job row jumps back to the job inspector. |
| Drag-drop refusal across bands | `onNodesChange` filters position changes: when the destination Y falls into a band different from the job's declared `phase`, the position is snapped back to the prior coordinates and an inline `role="alert"` error is rendered over the canvas. Intra-band drags pass through unchanged. |
| Job-phase selector in inspector | New `phase` field rendered only when `workflow.phases?.length > 0`. Writes `phase` (canonical RFC 0045 name); `phase_id` is read-compatible via `jobPhaseId(job)` so early fixtures still render. |
| v1.1 metadata stripping on save | `syncWorkflowEdges` rebuilds workflow edges from `{from, to, on}` only — `crossPhase` / `sourcePhase` / `targetPhase` derived keys are dropped, so the round-trip cannot accidentally pollute `workflow.json`. |
| Selection state | `selectedJobId` replaced by a `GraphSelection` discriminated union (`{kind:"job"|"phase", id}`). `onPaneClick` clears it; band header click selects the phase; node click selects the job. |

## Files touched

- `src/striatum/web/frontend/src/shared/types.ts` — added `WorkflowPhase`; extended `WorkflowDocument.phases?`, `WorkflowJob.phase?`, and read-compat `WorkflowJob.phase_id?`.
- `src/striatum/web/frontend/src/shared/theme.css` — added phase band, header, drag-error, inspector list, and `.react-flow__edge.cross-phase-edge` rules. No new palette tokens; band fills use low-opacity rgba constants per synthesis §"Phase Color Bands".
- `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx` — added `buildPhaseLayout`, `jobPhaseId`, `hasExplicitPhases`, `phaseIdForY`, `PhaseBands`, `PhaseInspector`, phase-aware refactor of `jobsToNodes` and `workflowToEdges`, drag refusal in `onNodesChange`, and the `GraphSelection` union. Existing v1 codepaths preserved (square grid, thin grey edges, etc.).
- `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts` — added the RFC 0045 unit suites (see "Test coverage" below).

No Python files were touched. Write scope respected:
`src/striatum/web/frontend/` and `docs/dogfood/043/build/frontend/` only.

## Test coverage added

In `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts`:

- `hasExplicitPhases` — returns false for absent / empty `phases`, true otherwise.
- `jobPhaseId` — prefers `phase` over `phase_id`, accepts either, returns `""` for unset.
- `jobsToNodes (v1 path)` — unchanged square-grid coordinates for v1 workflows.
- `jobsToNodes (v1.1 phase bucketing)` — verifies group-then-row layout inside each band, unknown-phase fallback into first phase, second-phase band-top offset.
- `workflowToEdges (v1.1 cross-phase tagging)` — confirms only cross-phase edges get the class/style/data triple; intra-phase edges keep the legacy shape; v1 workflows never receive cross-phase styling.
- `workflowToEdges` cycle edges — combined `"cycle-edge cross-phase-edge"` className when a cycle spans phases; thick-black style applies.
- `syncWorkflowEdges` strips derived `crossPhase` / `sourcePhase` / `targetPhase` keys before serialization.
- `buildPhaseLayout` indexes jobs by phase and exposes a job→phase map.
- `phaseIdForY` maps Y coordinates back to bands; returns null for v1 workflows or out-of-band Y.
- A baseline drag-refusal lookup test demonstrating the helper composition that `onNodesChange` performs.

Existing v1 tests (`jobsToNodes`, `workflowToEdges`, `syncWorkflowEdges`,
`syncWorkflowJobs`, `newJobFromBlock`, `PALETTE_BLOCKS`) were preserved
unmodified.

## Verification status

`striatum ack` was denied at the start of this supervised turn (permission
gate), and follow-up Bash invocations of `make ui-test` / `npm test` were
also denied. Per the task prompt — *"If `striatum ack` is denied, write the
HANDOFF and exit normally"* — I shipped the code and the new unit suites
without running `npm test` or `npm run build` locally. The reviewer should
expect to run `make ui-test` and `make ui-build` against this branch as
part of the build-review job.

Static review I did perform:

- v1 layout coordinates match the existing assertions
  (`cols = ceil(sqrt(n))`, 220×140 grid).
- v1.1 layout coordinates match the synthesis formulas — manually traced
  the two-phase / two-track fixture in the test file:
  - `design_a` → (0, 72), `design_b` → (24, 168), `synth_1` → (260, 72).
  - `build_a` → (0, 392), `consolidate_2` → (260, 392).
- `workflowToEdges` cross-phase logic: `synth_1 → build_a` crosses
  `phase_1_design → phase_2_build`; intra-phase `design_a → synth_1` stays
  default; cycles correctly combine class names when cross-phase.
- `syncWorkflowEdges` strips derived keys regardless of incoming `data`
  shape (rebuilt from `{from, to, on}` literal).

## Deviations from the synthesis (with rationale)

1. **Band overlay portal.** Synthesis §"Phase Color Bands" specifies
   absolutely-positioned divs in world coordinates rendered "as the first
   child of `<ReactFlow>`, before `<Background />`". React Flow v11 renders
   direct `<ReactFlow>` children in the renderer (untransformed) container,
   which would pin the bands to the viewport rather than world coordinates.
   I wrapped the bands in `<ViewportPortal>` (re-exported by reactflow
   11.6+, present in our 11.11.4 dependency) so the bands pan/zoom with
   nodes as the synthesis intends. Functional intent preserved; mounting
   point one layer deeper.

2. **Phase color from `phases[].color`.** Synthesis allows
   `phases[].color` but doesn't require it; I kept the deterministic
   four-entry CSS palette and have not yet plumbed `phase.color` to the
   band fill. A short follow-up swap to a custom-property override is easy
   if a downstream RFC needs per-phase color authoring.

3. **`PhaseInspector` uses native `<input>` / `<textarea>`** for title and
   description editing rather than the structured widgets in the job
   `Inspector`. Synthesis specifies the contract (`title`, `description`,
   `synthesis_job_id` display, job list) but not the widget shape; native
   inputs match the rest of the island's keyboard-accessible patterns.

## Known V1 limitations (carried from synthesis non-goals)

- No phase creation / reordering UI — operators edit `phases[]` via JSON or
  the Python `workflow_generator` `multi_phase` shape.
- No automatic edge rewrite when an operator changes a job's `phase` via
  the inspector. Synthesis §"Drag-Drop Policy" called this out: cross-band
  drags refuse; phase changes via the field are accepted but the validator
  (Track A scope) will surface invalid cross-phase dependencies when the
  operator saves.
- Coordinates remain UI-only and are not persisted back to `workflow.json`.
- Phase progress / run status is not rendered; the `PhaseInspector` props
  contract was designed to accept an optional `status` prop later when the
  Track A `status --json` phases block lands.

## Suggested reviewer focus

- v1 backwards-compat: open an existing dogfood-NNN workflow without
  `phases` and confirm there is *no* visual change to bands, edge stroke,
  or inspector.
- v1.1 fixture: a two-phase / two-track workflow with one explicit
  cross-phase `synth → next_phase_job` edge should render a thick black
  arrow while every intra-phase edge stays the default thin grey.
- `syncWorkflowEdges` round-trip: load v1.1 → click Save → re-read the
  posted JSON; ensure no `crossPhase` / `sourcePhase` / `targetPhase` keys
  appear in `edges[]`.
- Drag refusal: drag any job vertically into a different band; expect a
  snap-back plus the inline error referencing both the job id and its
  declared phase.
