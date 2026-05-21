# Review Authority Boundary

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Review the implementation for authority-boundary regressions:

- terminal/tmux output must not become workflow state;
- daemon PostgreSQL and MCP/RPC calls must remain authoritative;
- capability checks must fail closed;
- no deleted dogfood composites or SQLite fallback paths should reappear;
- no CLI workflow-control verb should be removed before MCP/UI parity is
  implemented and tested;
- hosted services, telemetry, external persistence, and transcript capture
  must remain out of core.

Return `accept`, `accept_with_findings`, or `needs_revision`.
