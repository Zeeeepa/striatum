# Implement First Cutover Slice

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Implement the smallest first slice selected by the cutover map and RFC 0075
liveness contract. Keep the patch narrowly scoped and leave broader CLI
deletion for later parity gates.

Requirements:

- reuse existing daemon RPC, MCP, supervision, and read-model patterns;
- do not parse terminal output for workflow facts;
- do not add transcript capture, telemetry, hosted services, or external
  persistence;
- keep CLI workflow-control deletion behind explicit MCP/UI parity tests;
- update contracts, generated docs, and authority guardrails if any daemon
  method or route map changes;
- add focused tests that fail closed for missing auth, missing tmux when
  live-interactive mode requires it, stale protocol liveness, and denied
  capabilities where applicable.

Record commands run and unresolved follow-ups in the handoff artifact.
