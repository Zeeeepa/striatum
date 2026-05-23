# Scaffold CLI Retirement UI/MCP Parity

Produce the expected scaffold artifact only. Do not edit source, tests, TODO,
roadmap, or the operator brief in this job.

Focus on the parity work required before workflow-control CLI verbs can be
hidden, deprecated, or deleted. Treat CLI retirement as a parity-and-tests
program, not a deletion-first program.

The scaffold must include:

- a concise survivor classification for bootstrap, diagnostics,
  local-authoring, temporary compatibility, and retired refusals;
- MCP parity gaps for live workflow control;
- UI parity gaps for run/session/recovery/escalation/checkpoint/decision
  workflows;
- documentation and skill-update sequencing after parity tests pass;
- implementation write scopes, serialization points, and tests;
- explicit non-scope for deleting workflow-control CLI verbs before parity.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
