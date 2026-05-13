# Designer Role (Dogfood 043)

Each track has three fresh-design lanes (codex, claude, gemini) that
produce independent perspectives. Synthesis picks one path. Cite the
existing code that your design changes — do not propose green-field
shapes.

## Track A — Python workflow schema, runtime, generator

Required citations (read these before designing):

- `src/striatum/workflow.py` — current workflow loader, validator,
  schema version handling.
- `src/striatum/workflow_generator/` (`core.py`, `catalog.py`,
  `write.py`) — current generator shape that your design must keep
  emitting valid workflows from.

Address: schema version bump, validator changes, runtime semantics,
generator output shape, the new `striatum workflow upgrade` verb, and
`striatum workflow status` reporting. **Backwards compatibility for v1
workflows is non-negotiable** — existing v1 workflows on disk must keep
loading and running, and the upgrade verb is the only sanctioned path
to v2.

## Track B — React Flow frontend editor

Required citation (read it before designing):

- `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`
  — the current editor island. Find any companion files in the same
  directory and the matching test at
  `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts`.

Address: node/edge shape changes, parallel_group rendering, write-scope
visualization, schema-version-aware loading (v1 vs v2), and how the
editor surfaces validator errors. **Backwards compatibility for v1
workflows is non-negotiable** — the editor must still open and display
existing v1 workflows.
