# Review Regression Risk

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Review the implementation for regression risk:

- verify tests cover the new liveness or cutover behavior rather than only
  docs;
- check daemon startup, MCP transport, fake-agent loop coverage, and
  workflow validation still pass;
- check status/dashboard/operator surfaces are stable and do not assume tmux
  for headless fixtures;
- verify docs and examples do not overclaim unfinished RFC 0075 work;
- name any untested deadline, stale-session, or capability-denial edge.

Return `accept`, `accept_with_findings`, or `needs_revision`.
