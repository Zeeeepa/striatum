# Track B Synthesis Prompt: RFC 0045 React Flow Editor

Produce `docs/dogfood/043/DESIGN_SYNTHESIS_frontend.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/043/design/frontend/codex/DESIGN.md", "docs/dogfood/043/design/frontend/claude_code/DESIGN.md", "docs/dogfood/043/design/frontend/gemini/DESIGN.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration.

Reconcile the 3 frontend designs into ONE concrete plan for RFC 0045 V1:

- Phase color-banding rendering approach: exact React Flow primitive (background pattern? overlay layer? `BackgroundVariant`?). Specify file + function.
- Cross-phase edge styling: exact style object (stroke width, color), and the conditional branch in the edge factory.
- Phase metadata side panel: exact component name + props contract, where it mounts in the existing island.
- Drag-drop policy: pick one of {update `phase_id` + prompt for synthesis edge} or {refuse drop with inline message}. Justify in one sentence.
- Backwards-compat path: how v1 workflows (no `phases` array) bypass all new rendering.
- Files touched in `src/islands/` (or wherever the React Flow island lives) — exhaustive list with one-line per-file rationale.

Choose; do not enumerate. Output is a SPECIFIC plan ready to implement against. If the three designs disagree, pick one and justify in one sentence.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
