author: designer-unknown-model-001

# Track B Design: RFC 0045 React Flow Editor Changes

V1 acceptance-criteria design for the multi-phase visualization and
interaction extensions to the workflow graph editor island.

## Scope and existing code

The React Flow editor lives in a single island:

- `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`
- Companion types: `src/striatum/web/frontend/src/shared/types.ts`
- Companion styles: `src/striatum/web/frontend/src/shared/theme.css`

The island reads a single `WorkflowDocument` from the
`<script id="workflow-data">` payload (`WorkflowGraphEditorImpl`,
`WorkflowGraphEditor.tsx:661-664`) and projects it into React Flow state
via two pure helpers — `jobsToNodes` (`WorkflowGraphEditor.tsx:72-84`)
and `workflowToEdges` (`WorkflowGraphEditor.tsx:86-112`). The canvas is
a single `<ReactFlow>` instance (`WorkflowGraphEditor.tsx:842-856`) with
`<Background>`, `<MiniMap>`, and `<Controls>` children. The right
column is the `Inspector` component (`WorkflowGraphEditor.tsx:188-456`),
and the three-column grid is fixed by the `.graph-editor` rule in
`shared/theme.css:292-298`.

These five anchors are the only places this design extends. No new
files are required for V1; the island grows in place.

## 0. Schema extension this design assumes

RFC 0045 §1 specifies the v1.1 phases array as `{ id, title,
synthesis_job_id }`. It does **not** define a `color` field per phase.
The task prompt asks bands to render `phases[].color`. Two ways to
reconcile, in order of preference:

1. **Deterministic palette by phase index (recommended for V1).**
   Bands use a fixed accessible palette derived from CSS custom
   properties in `base.css` (the existing source-of-truth per
   `shared/theme.css:1-6` "no new palette variables"). Operators get
   stable, theme-aware bands without expanding the schema.
2. **Schema follow-up** to add optional `phases[].color` (named ramp
   token, e.g. `"phase-band-1"`). Pure-additive, backwards compatible.
   Recorded as a deferred TODO; not in V1 acceptance.

V1 ships with (1). The render function reads `phase.color` if present
(forward-compatible for follow-up (2)) and otherwise indexes the
palette by phase position.

The corresponding palette additions in `base.css`:

```css
--phase-band-1: rgba(102, 187, 255, 0.10); /* steel blue */
--phase-band-2: rgba(168, 230, 207, 0.10); /* sage */
--phase-band-3: rgba(255, 211, 165, 0.10); /* warm sand */
--phase-band-4: rgba(204, 173, 255, 0.10); /* lilac */
--phase-band-header: var(--bg-overlay);
--phase-band-border: var(--border);
```

Bands are intentionally low-opacity so they read as background, not as
node chrome. Dark and light themes pick up the underlying token shift
automatically.

## 1. Phase color-banding

**Decision:** horizontal bands, one per phase, stacked top-to-bottom in
phases-array order. Horizontal because (a) the existing
`jobsToNodes` (`WorkflowGraphEditor.tsx:74-83`) already lays jobs out
on a row-by-row grid with X varying inside a row, so vertical-axis
phase stacking is a small generalization of the current Y formula, and
(b) human readers map "earlier → later" to "top → bottom" the same way
a Gantt chart does.

**Bands are NOT React Flow nodes.** They are absolutely-positioned
background `div`s rendered as a child of `<ReactFlow>`, behind the
existing `<Background>` (`WorkflowGraphEditor.tsx:853`). Modeling them
as nodes would put them in the React Flow selection model, drag
handlers, and z-order; we want them inert.

### Extension point

Insert a new `PhaseBands` component as the first child of
`<ReactFlow>` (between line 852 and the existing `<Background />` on
line 853):

```tsx
<ReactFlow ... style={flowStyle}>
  <PhaseBands phases={workflow.phases} layout={phaseLayout} />
  <Background />
  <MiniMap pannable zoomable />
  <Controls />
</ReactFlow>
```

`phaseLayout` is a derived map of `phase_id → { y, height, color, title }`
computed in a `useMemo` keyed on `workflow.phases` and the node
positions. The map is the single source of truth for both (a) where
each band paints and (b) where `jobsToNodes` places jobs.

### Node bucketing rule

Replace the square-grid math in `jobsToNodes`
(`WorkflowGraphEditor.tsx:74-83`) with a phase-aware layout:

```ts
function jobsToNodes(workflow: WorkflowDocument): Node[] {
  const phases = workflow.phases ?? [];
  const bandHeight = 220;                     // per-phase Y span
  const colWidth   = 220;                     // existing column step
  const rowHeight  = 140;                     // existing row step
  const phaseIndex = new Map(phases.map((p, i) => [p.id, i]));

  // V1.1 jobs may carry phase; v1 jobs render as a single implicit phase 0.
  const buckets = new Map<number, WorkflowJob[]>();
  for (const j of workflow.jobs ?? []) {
    const idx = j.phase ? (phaseIndex.get(j.phase) ?? 0) : 0;
    if (!buckets.has(idx)) buckets.set(idx, []);
    buckets.get(idx)!.push(j);
  }

  const out: Node[] = [];
  for (const [bandIdx, jobs] of buckets) {
    const cols = Math.max(1, Math.ceil(Math.sqrt(jobs.length)));
    const yBase = bandIdx * bandHeight + 32;  // 32px header gutter
    jobs.forEach((job, k) => {
      out.push({
        id: job.id,
        type: "default",
        position: { x: (k % cols) * colWidth, y: yBase + Math.floor(k / cols) * rowHeight },
        data: { label: `${job.id}\n${job.type ?? "generic"}` },
      });
    });
  }
  return out;
}
```

Jobs without a `phase` field fall into the implicit band-0 bucket; in
v1 workflows this is the only bucket, and the result is identical in
shape to today's grid (see §5 backwards compatibility).

### `PhaseBands` render

```tsx
function PhaseBands({ phases, layout }: PhaseBandsProps) {
  if (!phases || phases.length === 0) return null;        // v1 fast-path
  return (
    <div className="phase-bands" aria-hidden="true">
      {phases.map((p, i) => (
        <div
          key={p.id}
          className="phase-band"
          style={{
            top: layout.get(p.id)!.y,
            height: layout.get(p.id)!.height,
            background: layout.get(p.id)!.color,
          }}
          onClick={() => onPhaseSelect(p.id)}
        >
          <div className="phase-band-header">{p.title}</div>
        </div>
      ))}
    </div>
  );
}
```

CSS additions (append to the workflow-graph-editor block in
`shared/theme.css:290-422`):

```css
.phase-bands {
  position: absolute;
  inset: 0;
  pointer-events: none;       /* let React Flow keep mouse priority */
  z-index: 0;                  /* under <Background> dots */
}
.phase-band {
  position: absolute;
  left: 0;
  right: 0;
  border-top: 1px solid var(--phase-band-border);
  pointer-events: auto;        /* re-enable on the header strip only */
}
.phase-band-header {
  font: inherit;
  font-size: 0.85em;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--fg-muted);
  background: var(--phase-band-header);
  padding: 2px var(--space-2);
  width: max-content;
  border-bottom-right-radius: var(--radius);
  cursor: pointer;
}
```

Only the header strip captures pointer events; the rest of the band is
`pointer-events: none` so React Flow's pan/zoom and node-drag handlers
are unaffected.

## 2. Cross-phase edge styling

The existing edge model already supports per-edge styling via a
`className` token; cycles use this exact mechanism today —
`workflowToEdges` sets `className: "cycle-edge"`
(`WorkflowGraphEditor.tsx:101`) and the CSS rule at
`shared/theme.css:449-452` paints those edges in
`var(--status-needs_revision)` with a dashed stroke. We mirror the
pattern.

### Extension to `workflowToEdges`

Replace `WorkflowGraphEditor.tsx:86-112` with a version that resolves
each endpoint's `phase` and tags cross-phase edges:

```ts
function workflowToEdges(workflow: WorkflowDocument): Edge[] {
  const jobPhase = new Map(
    (workflow.jobs ?? []).map((j) => [j.id, j.phase ?? null]),
  );
  const out: Edge[] = [];
  for (const e of workflow.edges ?? []) {
    const sp = jobPhase.get(e.from);
    const tp = jobPhase.get(e.to);
    const crossPhase = sp != null && tp != null && sp !== tp;
    out.push({
      id: `e-${e.from}->${e.to}-${e.on ?? "completed"}`,
      source: e.from,
      target: e.to,
      label: e.on ?? "completed",
      className: crossPhase ? "cross-phase-edge" : undefined,
      data: { on: e.on ?? "completed" },
    });
  }
  // cycles unchanged
  for (const c of workflow.cycles ?? []) { ... }
  return out;
}
```

The function stays pure; no React Flow internals are touched.

### CSS rule

Append immediately after the cycle-edge rule
(`shared/theme.css:449-452`):

```css
.react-flow__edge.cross-phase-edge .react-flow__edge-path {
  stroke: var(--fg-primary);   /* "black" in light theme; primary fg in dark */
  stroke-width: 2.5px;          /* base is ~1px from React Flow defaults */
}
```

The CSS variable choice keeps the rule theme-aware (the prompt says
"black"; using the primary foreground token preserves contrast under
the dark theme without forking a colour). Cycles keep their dashed
red stroke; cycle-vs-cross-phase precedence on overlapping classes is
not a concern because an edge is either an `edges[]` entry or a
`cycles[]` entry, never both (`workflowToEdges` populates them in
disjoint loops at lines 88-96 and 97-110).

## 3. Phase metadata side panel

The right column already hosts an inspector
(`WorkflowGraphEditor.tsx:865-870`, CSS at `shared/theme.css:294`
columns `200px 1fr 360px`). Phase-metadata reuses this slot; we don't
add a fourth column.

### Selection model

Two selections exist today: `selectedJobId` (`WorkflowGraphEditor.tsx:675`)
driven by `onNodeClick` (`WorkflowGraphEditor.tsx:848`). Introduce
`selectedPhaseId` as a parallel state and a single
`selection: { kind: "job"; id: string } | { kind: "phase"; id: string } | null`
discriminated union to avoid double-selection. Clicking a phase header
sets `selection = { kind: "phase", id }`; clicking a node or empty pane
clears the phase selection per the existing handlers
(`WorkflowGraphEditor.tsx:848-849`).

### Inspector branching

The `Inspector` component (`WorkflowGraphEditor.tsx:188`) becomes a
thin switch over `selection.kind`:

```tsx
{selection?.kind === "phase"
  ? <PhaseInspector phase={...} jobs={jobsInPhase} status={statusByJob} />
  : <Inspector job={selectedJob} ... />}
```

`PhaseInspector` fields, in order:

- `name` — bound to `phase.title` (editable, writes back through the
  same `handleJobChange`-style channel but to `workflow.phases`).
- `description` — bound to `phase.description` (forward-compatible;
  RFC 0045 §1 doesn't list this field, treat as additive optional).
- Jobs in this phase — read-only list of `{id, type, lane}` pulled
  from `workflow.jobs.filter(j => j.phase === phase.id)`; clicking a
  row sets `selection = { kind: "job", id }`.
- Per-phase progress — read from the existing status feed.

### Status-feed read path

The existing editor does not subscribe to live status today; its job
labels are pure derivations of `WorkflowDocument`
(`WorkflowGraphEditor.tsx:82`). The status feed is a separate
endpoint, surfaced as `striatum status --json` per RFC 0045 §3 ("status
output gains an optional `phases` block"). For the V1 editor read
path:

- Server-side: extend the existing `<script id="workflow-data">`
  emitter to also drop a `<script id="workflow-status" type="application/json">`
  payload alongside it, mirrored by a new `workflowStatusElementId`
  prop on `WorkflowGraphEditorProps`
  (`shared/types.ts:199-212`).
- Client-side: a second `readJsonPayload<WorkflowStatus>(...)` in
  `WorkflowGraphEditorImpl`. The status struct is `Record<job_id,
  { state, attempts, last_verdict }>`. Per-phase progress is then
  `count(state === "completed") / count(all jobs in phase)`.

The progress bar reuses existing tokens (`var(--status-completed)`,
`var(--bg-overlay)`) — no new CSS variables.

## 4. Drag-drop respecting phase boundaries

**Decision:** refuse the cross-band drop with an inline message.
Intra-band drag updates the React Flow coordinates only (today's
behavior; coordinates are not persisted per the island docstring at
`WorkflowGraphEditor.tsx:14`).

### Justification

A silent `phase_id` rewrite on drop is unsafe: most jobs in a phase
have dependencies on sibling jobs (`depends_on` or `edges[]`), and
RFC 0045 §2 makes the validator refuse cross-phase dependencies that
don't go through the source phase's `phase_synthesis` job. Dropping a
job into the next band without rewriting those edges produces a
workflow that fails `striatum workflow validate` on the very next
save. Putting a "do you want to rewrite these edges?" modal on a drag
gesture is heavy-handed UX for a rare operation and adds a stateful
multi-step flow on a primitive (drag) that users expect to be
reversible by releasing the mouse.

Refusing the drop keeps the canvas safe by default. Operators who
genuinely need to move a job across phases use the existing
`Inspector` (`WorkflowGraphEditor.tsx:188`) — we add a `phase`
selector field (mirroring the `role_id` select at lines 259-271)
listing all `phases[].id` values plus `(none)`. Direct edit through
the inspector is explicit, single-step, and the operator already sees
the rest of the job's fields, so the dependency rewiring required
afterwards is obvious.

### Implementation hook

`onNodesChange` (`WorkflowGraphEditor.tsx:685-687`) currently applies
all changes unconditionally. Wrap it to detect a `position` change
whose new Y crosses the band boundary derived from the node's owning
phase, and (a) revert the position to the last valid one, (b) surface
an inline message in the existing `error` state slot
(`WorkflowGraphEditor.tsx:676`, rendered at lines 872-876) reading:

> Drag refused: `job_id` belongs to phase `phase_id`. Use the inspector's
> `phase` selector to move it across phases.

```ts
const onNodesChange = useCallback((changes: NodeChange[]) => {
  setNodes((ns) => {
    const next = applyNodeChanges(changes, ns);
    const violations = detectBandCrossing(next, phaseLayout, workflow);
    if (violations.length > 0) {
      setError(formatBandRefusal(violations));
      return ns;                                  // revert
    }
    return next;
  });
}, [phaseLayout, workflow]);
```

`detectBandCrossing` returns the subset of moved nodes whose new Y
falls outside their owning phase's `[y, y + height]`. Pure function,
testable in isolation alongside the existing `__testing` exports
(`WorkflowGraphEditor.tsx:907-914`).

## 5. Backwards compatibility

A v1 workflow has no `phases` field
(`WorkflowDocument.phases === undefined`). All four extensions above
short-circuit cleanly on that condition:

- **`jobsToNodes` (§1)**: when `phases` is absent, the `buckets` map
  has only key 0, and the layout collapses to the original
  `index % cols` / `Math.floor(index / cols)` grid (the
  `WorkflowGraphEditor.tsx:74-83` shape).
- **`PhaseBands` (§1)**: the `if (!phases || phases.length === 0)
  return null;` early-return at the top of the component leaves the
  canvas with only `<Background>`, `<MiniMap>`, `<Controls>` — pixel-
  identical to today.
- **`workflowToEdges` (§2)**: `jobPhase` resolves every endpoint to
  `null`, the `crossPhase` predicate is `false` for every edge,
  `className` stays `undefined`, and the CSS rule never matches. Edges
  render in `var(--border)` per `shared/theme.css:441-443`.
- **`Inspector` switch (§3)**: `selectedPhaseId` stays `null` because
  the user never clicks a band that does not exist; the union narrows
  to the existing job-only inspector branch.
- **`onNodesChange` wrapper (§4)**: `phaseLayout` is the empty map,
  `detectBandCrossing` returns `[]` for every change set, and the
  original `applyNodeChanges` runs unmodified.

The conditional render path is therefore a single predicate —
`(workflow.phases ?? []).length > 0` — evaluated lazily in each
helper. There is no top-level mode flag and no separate component
tree for v1 vs v1.1.

A round-trip test fixture for both schemas lives naturally alongside
the existing `__testing` exports (`WorkflowGraphEditor.tsx:907-914`):
unit-test `jobsToNodes` and `workflowToEdges` against the current
fixture plus a new v1.1 fixture and assert that the v1 output bytes
are byte-identical.

## Acceptance criteria mapping (RFC 0045 §Acceptance Criteria)

This design implements criterion 6:

> "React Flow editor renders phase color bands and cross-phase edges
> differently from intra-phase edges."

with the additional V1 ergonomics the prompt requires (side panel,
drag-refuse behavior, v1 back-compat), but does not address criteria
1-5, 7-8, which belong to the Python core, generator, CLI, and
fixture tracks.

## Out of scope

Per the prompt: Python core (Track A), edge re-routing algorithm
changes, new node shapes beyond color band membership. Additionally:

- Live mutation of the `phases` array (add / remove / reorder a phase
  from the canvas). The schema editor for `phases` is a separate
  surface; this design touches only rendering and per-job phase
  selection.
- Cross-phase edge auto-insertion when an operator changes a job's
  `phase` via the inspector. The inspector exposes the field; the
  operator is responsible for adding the `phase_synthesis` dependency
  edge. RFC 0045 §2 validator catches the omission on save.
- Animated transitions when bands resize. Out of V1; the
  `prefers-reduced-motion` block at `shared/theme.css:454-460` already
  blanket-disables React Flow transitions anyway.

## Cited code anchors

- `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`
  - `jobsToNodes` — lines 72-84
  - `workflowToEdges` — lines 86-112 (cycle className at line 101)
  - `Inspector` — lines 188-456
  - `selectedJobId` state — line 675
  - `<ReactFlow>` JSX with `<Background>` — lines 842-856
  - `onNodesChange` callback — lines 685-687
  - `error` state and render — line 676, lines 872-876
  - `__testing` exports — lines 907-914
- `src/striatum/web/frontend/src/shared/types.ts`
  - `WorkflowJob` (index signature accepts `phase`) — lines 135-163
  - `WorkflowDocument` (index signature accepts `phases`) — lines 178-197
  - `WorkflowGraphEditorProps` — lines 199-212
- `src/striatum/web/frontend/src/shared/theme.css`
  - `.graph-editor` grid — lines 292-298
  - `.react-flow__edge-path` — lines 441-443
  - `.react-flow__edge.cycle-edge .react-flow__edge-path` — lines 449-452
  - `prefers-reduced-motion` block — lines 454-460
