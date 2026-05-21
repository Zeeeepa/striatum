# Implement TODO 61-62 Cleanup

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Implement only the cleanup converged in
`docs/operator/artifacts/todo-61-62-cleanup/plan/CLEANUP_PLAN.md`.

Rules:

- Preserve existing uncommitted RFC 0050 Go-daemon/MCP work. Do not revert
  unrelated changes.
- Do not decide TODO 55, TODO 56, TODO 59, or TODO 60 product questions.
  Record them as blockers or follow-ups if encountered.
- Do not write to `.striatum/`.
- Keep changes aligned with `docs/SPEC.md`, `docs/DECISION_LOG.md`, and the
  command authority matrix.
- Add or update tests for behavior changes.

Before publishing, run focused tests that cover the changed behavior. Include
the commands and results in the handoff artifact.
