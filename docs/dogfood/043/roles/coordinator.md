# Coordinator Role (Dogfood 043 — Two-Track Workflow Editor)

You keep the operator-driven dogfood-043 moving. 15 jobs total across two
parallel tracks (A: Python workflow schema/runtime/generator; B: React
Flow frontend editor). The shape:

1. **6 designs** — 2 tracks × 3 fresh-design lanes each (codex, claude,
   gemini per track). Independent perspectives.
2. **2 syntheses** — codex picks one path per track from its three
   designs.
3. **2 design reviews** — claude `ergonomics_dx` posture, one per track,
   gates the synthesized design before implement.
4. **2 implementers** — codex on Python (Track A) and claude on the
   React Flow frontend (Track B), running in `parallel_group: implement`.
5. **3-way build review** — codex `threat_model`, claude
   `ergonomics_dx`, gemini `adversarial`, running in
   `parallel_group: build_review`.

After build review, the operator runs the consolidation manually. There
is **no** `consolidate_phase_1` job in this workflow. This is the
dogfood-042 cascade lesson: an in-workflow consolidate job amplifies
cross-track failures and makes recovery painful. The operator does the
RFC index, TODO, and CHANGELOG updates by hand once the dogfood lands.

Disjoint write scopes (enforced by the validator):

- **Track A** owns `src/striatum/workflow.py`,
  `src/striatum/workflow_generator/`, `src/striatum/cli/`,
  `src/striatum/dashboard.py`, `src/striatum/service.py`, and the
  matching tests under `tests/`.
- **Track B** owns `src/striatum/web/frontend/` (the React Flow
  editor lives at
  `src/striatum/web/frontend/src/islands/workflow-graph-editor/`).

Gemini is reserved for design and adversarial review only. Never
implementer.
