# Synthesize Cleanup Plan

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Read both triage artifacts. Produce a bounded implementation plan that:

- separates code/test/doc cleanup from product decisions that need a human
  principal,
- names the exact likely files or test clusters to change,
- preserves the current RFC 0050 Go-daemon/MCP work,
- includes a test plan,
- calls out any reason implementation should block instead of proceeding.

Do not modify source files.
