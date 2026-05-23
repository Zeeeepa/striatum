# Phase 2A: MCP Exact Parity Tests

Write `docs/operator/artifacts/ordered-backlog-2026-05-23/phase-02-mcp/REPORT.md`
with `author: mcp-parity-codex-gpt-5-001`.

Task:
- Review `docs/architecture/CLI_RETIREMENT_PARITY.md`.
- Identify rows still marked `mcp_registry` where an exact MCP `tools/call`
  test can be added without new product UI.
- Prefer `run.pause`, `run.resume`, `run.cancel`, `run.retry_job`,
  `recovery.cancel_job`, `branch.confirm`, `session.close`, `work.release`,
  `work.block`, and `review.submit`.
- Report the concrete test files and expected ledger updates.

Do not hide or delete CLI verbs.
