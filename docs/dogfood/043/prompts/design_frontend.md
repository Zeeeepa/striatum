# Track B Design Prompt: RFC 0045 React Flow Editor

Produce the DESIGN.md artifact at the path your work packet specifies (under `docs/dogfood/043/design/frontend/<lane>/`).

Design **RFC 0045 V1 acceptance criteria** for the React Flow workflow editor changes. Read the RFC first: `docs/rfcs/0045-multi-phase-workflow-editor-and-schema.md`.

Cover concretely:

- **Phase color-banding**: render each phase as a horizontal (or vertical, your call) background band coloured per `phases[].color`. Bands are background props on the React Flow canvas, NOT nodes. Specify how nodes are bucketed into bands by their `phase_id`.
- **Cross-phase edge styling**: edges where source and target `phase_id` differ render as thick black lines; intra-phase edges render as thin grey. Cite the existing edge style code path.
- **Phase metadata side panel**: clicking a band header opens a side panel with `name`, `description`, list of jobs in that phase, and per-phase progress (read from the existing status feed).
- **Drag-drop respecting phase boundaries**: dragging a job within its band updates layout only. Dragging across a band boundary either (a) updates the job's `phase_id` and prompts the operator to add a `phase_synthesis` edge, or (b) refuses the drop with an inline message. Pick one in your design; justify briefly.
- **Backwards compatibility**: v1 workflows (no `phases` array) render exactly as today — a single implicit phase, no bands, no thick edges. Specify the conditional render path.

Designers MUST cite existing code in `src/striatum/web/frontend/`. Find the React Flow island file (likely `src/islands/WorkflowGraphIsland.tsx` or similar) and cite the node/edge/layout code you will extend. Hand-waving "we add a layer" is grounds for design review to bounce.

Out of scope: Python core (Track A), edge re-routing algorithm changes, new node shapes beyond color band membership.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:`.

One-shot supervised invocation. Write the artifact directly. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
