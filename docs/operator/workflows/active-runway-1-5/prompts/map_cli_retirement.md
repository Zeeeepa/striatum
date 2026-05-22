# Map CLI Retirement And MCP/UI Parity

Produce the expected synthesis artifact only. Do not edit source in this job.

Build a concrete CLI cutover ledger from the current source and contract
reality. The ledger must:

- classify each remaining workflow-control CLI verb as replaced by MCP/UI,
  bootstrap, diagnostics, or temporary compatibility;
- identify missing MCP/UI parity before any CLI hiding or retirement;
- cite current daemon methods and capability scopes;
- name tests/guardrails needed before documented CLI paths can be demoted;
- preserve bootstrap and diagnostics commands where justified;
- explicitly state that CLI retirement cannot precede parity.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
