# Track B Implement: RFC 0045 React Flow Editor (claude)

Blocked until `review_design_frontend` returns an accepting verdict.

Implement RFC 0045 frontend per `docs/dogfood/043/DESIGN_SYNTHESIS_frontend.md`. **You write frontend code only.** Do NOT cross into Python scope.

**Your scope (claude frontend-side):**

- `src/striatum/web/frontend/` — update the existing React Flow island file (per the synthesis, likely `src/islands/WorkflowGraphIsland.tsx` or equivalent) to:
  - Render phase color-bands as background overlays driven by `phases[]`.
  - Apply cross-phase edge styling (thick black) vs intra-phase (thin grey).
  - Mount a phase metadata side panel toggled by band-header click.
  - Wire drag-drop to respect phase boundaries per the synthesis policy.
  - Preserve v1 single-phase rendering when `phases[]` is absent.
- `docs/dogfood/043/build/frontend/HANDOFF.md` — handoff summarizing shipped scope, files touched, test or build results, deviations from the synthesis (if any) with one-line rationale.

**Use sub-agents aggressively** — one per concern, dispatched in parallel:

- Sub-agent 1: band rendering (background overlay code path, conditional on `phases[]`).
- Sub-agent 2: edge styling (cross-phase thick black vs intra-phase thin grey).
- Sub-agent 3: side panel component + mount point.
- Sub-agent 4: drag-drop handler honouring phase boundaries.

Reconcile sub-agent outputs yourself before writing HANDOFF.

**Do NOT write to**: anything outside `allowed_paths`. Specifically not `src/striatum/workflow.py`, `src/striatum/cli/`, `src/striatum/service.py`, `tests/`, `docs/rfcs/`.

Verification: the frontend build still succeeds; existing v1 workflow fixtures still render as a single phase (no bands, no thick edges); a v1.1 fixture renders bands + thick cross-phase edges.

One-shot supervised invocation. Do not ask follow-ups. If `striatum ack` is denied, write the HANDOFF and exit normally.
