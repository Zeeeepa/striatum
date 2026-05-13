# Implementer Role (Dogfood 043)

Two parallel implementers in `parallel_group: implement` with disjoint
write scopes. The workflow validator enforces the split — stay strictly
inside your job's `write_scope.allowed_paths`.

## Track A — codex, Python

Owns:

- `src/striatum/workflow.py` — schema version bump, validator updates,
  runtime semantics for v2.
- `src/striatum/workflow_generator/` — generator shape emitting v2 by
  default, with v1 still loadable.
- `src/striatum/cli/` — new `striatum workflow upgrade` verb and
  `striatum workflow status` reporting.
- `src/striatum/dashboard.py`, `src/striatum/service.py` — any
  schema-aware adjustments the synthesis calls out.
- Matching tests under `tests/`.

## Track B — claude, React Flow frontend

Owns: `src/striatum/web/frontend/` only. The editor entry point is
`src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`; companion
files live in the same directory and the test at
`src/__tests__/workflow-graph-editor.test.ts`.

## Both tracks — do NOT cross

Disjoint scopes. Track A does not touch frontend; Track B does not
touch Python. **Neither track updates `docs/rfcs/README.md`,
`docs/TODO.md`, or `CHANGELOG.md`** — the operator handles those
manually after the dogfood lands (no in-workflow consolidate job;
dogfood-042 cascade lesson).

Use sub-agents aggressively. Track A's surface area (schema +
validator + runtime + generator + CLI verbs + tests) and Track B's
React Flow refactor both benefit from parallel sub-agent exploration.

Operational notes:

- Lease can expire if `make test` exceeds ~30 minutes. Prefer focused
  pytest (or `pnpm test` for Track B) before wider verification.
- This is a one-shot supervised invocation. Do not ask the operator
  follow-up questions. If `striatum ack` is denied, write the artifact
  and exit normally; the operator publishes on your behalf.
- Per D089/D091, OPERATOR_REPORT.md is the operator's responsibility,
  written incrementally — not yours.
