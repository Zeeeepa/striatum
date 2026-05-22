# Review Regression And Parity Tests

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Review the slice for regression and test risk:

- the selected MCP/UI replacement path should have success coverage and
  missing-token, wrong-capability, wrong-repo, or mutations-disabled coverage
  where applicable;
- web UI routes should preserve `--allow-mutations` behavior and parameter
  parity with the daemon method they call;
- MCP `tools/list` visibility should match capability and hidden-tool policy;
- existing fake-agent, authority guardrail, and workflow validation coverage
  should not regress;
- CLI workflow-control deletion or hiding must remain blocked unless parity
  tests passed for the exact replacement path.

Return `accept`, `accept_with_findings`, or `needs_revision`.
