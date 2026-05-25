# Preserve Local Command Boundary

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Make local commands explicit so the generated daemon router does not blur local
file-authoring behavior with daemon RPC behavior.

Focus on workflow validation/lint/plan/graph/templates/generate/init/upgrade
and other CLI surfaces already classified as local authoring, bootstrap, or
compatibility in the RFC 0078 handoff and CLI retirement parity ledger.

Requirements:

- define a small local-command registry or dispatch boundary under the allowed
  path;
- ensure local commands do not require a daemon RPC route unless the contract
  says they are daemon-backed;
- keep local file writes explicit and operator-confirmed where current behavior
  requires it;
- add tests showing local commands bypass daemon routing deliberately;
- record any command that needs a later retirement or daemon-method decision.
