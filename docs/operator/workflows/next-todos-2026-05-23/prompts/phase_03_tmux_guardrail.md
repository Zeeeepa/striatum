# Phase 3D: RFC 0075 Tmux Guardrail

Write `docs/operator/artifacts/next-todos-2026-05-23/phase-03-tmux/REPORT.md`
with the packet's exact author line.

Task:
- Add the smallest automated guardrail proving tmux observability remains
  metadata-only.
- The test should prevent terminal text / tmux pane capture from becoming
  workflow state or durable provenance.
- Do not weaken tmux attach metadata or fail-closed `require_tmux` behavior.
