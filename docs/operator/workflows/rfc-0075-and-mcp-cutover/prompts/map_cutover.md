# Map Remaining MCP/UI Cutover

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Produce a concise cutover map for RFC 0050 Phase F and RFC 0075. The artifact
must:

- list remaining live workflow-control CLI verbs and classify each as
  `replace_with_mcp`, `replace_with_ui`, `bootstrap`, `diagnostics`,
  `temporary_compatibility`, or `retire`;
- name the daemon MCP method or UI surface that must exist before each
  non-bootstrap workflow-control verb can be hidden or deleted;
- identify which existing MCP coverage already proves parity and which gaps
  still need tests;
- keep deleted dogfood composites deleted unless a new product decision
  explicitly accepts PostgreSQL-native composites;
- call out any docs or skill templates that still teach CLI-first live
  workflow control.

Do not edit source code in this job.
