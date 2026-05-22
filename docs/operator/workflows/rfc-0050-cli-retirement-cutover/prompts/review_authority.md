# Review Authority Boundary

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Review the implementation for authority-boundary regressions:

- daemon-owned PostgreSQL and daemon RPC/MCP must remain authoritative for
  live workflow state;
- capability checks and service mutation gates must fail closed;
- MCP hidden-production-tool policy must remain intentional and tested;
- terminal output, tmux panes, marker files, and transcripts must not become
  workflow state;
- SQLite fallback paths and removed dogfood composites must not reappear;
- no CLI workflow-control verb may be hidden or deleted before exact MCP/UI
  parity tests pass.

Return `accept`, `accept_with_findings`, or `needs_revision`.
