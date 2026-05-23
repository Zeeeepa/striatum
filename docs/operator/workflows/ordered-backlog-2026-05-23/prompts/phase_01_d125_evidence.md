# Phase 1: TODO 56 D125 Evidence Gate

Write `docs/operator/artifacts/ordered-backlog-2026-05-23/phase-01-d125/REPORT.md`
with `author: d125-evidence-codex-gpt-5-001`.

Task:
- Read D125 in `docs/DECISION_LOG.md`, TODO 56 in `docs/TODO.md`, and the
  current `recovery.auto_finalize` implementation.
- Execute the next safe D125 step if possible: a live opt-in auto-finalize
  evidence slice, not a global default flip.
- If live evidence cannot be honestly produced in this job, state the blocker,
  the exact command sequence needed, and whether source/docs already match the
  dry-run default.
- Include validation commands run and any remaining gate count.

Do not change source files in this workflow job.
