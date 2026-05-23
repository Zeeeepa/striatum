---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/rfc-0050-0075-final-cutover-design/map/MAP.md"]
---

# RFC 0050 / RFC 0075 Terminal Cutover Design
author: operator [self-declared: cutover-designer-codex-gpt-5-001]

## Decision

RFC 0050/RFC 0075 cutover is complete when no live workflow-control operation
requires a human or agent to invoke a `striatum` CLI verb.

The terminal state is not "delete every CLI command." The terminal state is:

- lane agents use daemon MCP tools for the packet loop and lane-owned actions;
- human operators use the local web UI for operator decisions, recovery, and
  explicit confirmations;
- CLI remains supported for bootstrap, diagnostics, compatibility, debugging,
  and scriptable recovery.

## Implementation Gate

The implementation must:

- add daemon-routed web endpoints for the remaining human/operator UI gaps:
  local `git.commit_apply`, recovery inspection/transitions,
  `recovery.auto_finalize`, `cross_repo.cancel`, and supervisor stop;
- update docs, chat-tool descriptions, and generated skill templates so they
  teach MCP/UI first rather than CLI loops;
- update the CLI retirement parity ledger from blocked statuses to terminal
  survivor categories;
- keep the authority guardrail: no repo-local SQLite, no Python MCP, no
  terminal transcript authority, no hosted providers, no telemetry, and no
  external persistence.

## Non-Deletion Rule

The implementation must not delete CLI routes in this slice. Removing CLI
verbs is no longer a prerequisite for the cutover and would create avoidable
operator risk. A future cleanup may hide or delete compatibility commands only
after a separate release/deprecation decision.
