# Classify CLI Surface And Parity

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Produce a refreshed RFC 0050 CLI parity map. The artifact must:

- classify live workflow-control CLI verbs as MCP parity, UI parity, MCP+UI
  parity, bootstrap survivor, diagnostics survivor, local-file authoring,
  temporary compatibility, or retired fixture-only;
- name the daemon method, MCP visibility, operator UI route or missing UI
  route, and current test evidence for each workflow-control row;
- identify the smallest parity gaps that still block hiding or deleting a
  CLI workflow-control verb;
- preserve bootstrap and diagnostics CLI commands as explicit survivors;
- state that CLI workflow-control deletion is forbidden until MCP/UI parity
  tests for the exact replacement path pass.

Do not edit source code in this job.
