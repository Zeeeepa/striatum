# Select Next MCP/UI Parity Slice

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Use the refreshed classification to choose one bounded MCP/UI parity slice.
Prefer the smallest slice that unblocks a real operator workflow-control gap
without touching unrelated CLI retirement work.

The artifact must include:

- the selected verb family and the MCP method or UI route that will replace
  it for live workflow control;
- files likely to change and files that must not change;
- acceptance tests, including mutation-gate refusal tests where UI writes are
  involved;
- authority guardrails to preserve daemon PostgreSQL as the live state source;
- a clear gate: do not hide or delete CLI workflow-control verbs unless these
  parity tests pass for the exact replacement path.

Do not implement the slice in this job.
