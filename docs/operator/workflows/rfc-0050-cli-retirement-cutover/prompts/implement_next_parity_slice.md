# Implement Next MCP/UI Parity Slice

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Implement only the selected parity slice. Keep the patch narrowly scoped to
the chosen MCP/UI replacement path and its tests.

Requirements:

- preserve daemon RPC/PostgreSQL as the authoritative live-state boundary;
- use existing MCP, web UI, capability, run-action, and guardrail patterns;
- add focused tests proving success and fail-closed behavior;
- update contract-derived or curated authority docs only when the slice
  changes those surfaces;
- do not add hosted services, telemetry, transcript capture, external
  persistence, SQLite fallback, or deleted dogfood composites;
- do not hide or delete any CLI workflow-control verb unless the
  classification names that verb as parity-complete and the new MCP/UI parity
  tests pass.

Record commands run, test results, changed surfaces, and any remaining CLI
retirement blockers in the handoff artifact.
