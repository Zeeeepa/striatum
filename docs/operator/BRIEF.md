---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-05-20_go-daemon-http-sse-mcp"
supersedes: "brief_2026-05-17_go-daemon-remediation"
scope_links: ["docs/operator/plans/rfc-0068-go-daemon-port.md", "docs/operator/plans/rfc-0069-pg-only-daemon-global-surfaces.md", "docs/rfcs/0050-go-daemon-http-sse-mcp.md"]
context_budget_lines: 300
retrieval_priority: "high"
status: "current"
---

# Operator Brief
author: antigravity-001

## State

Striatum's live-state boundary is daemon-owned PostgreSQL. The active
remediation runway is D107/RFC 0068: Go is the production/default daemon.
We have just accepted **RFC 0050**, which mandates the removal of the remaining Python `mcp.py` wrapper, bringing HTTP/SSE MCP server functionality natively into the Go `striatumd` daemon. 

The `agent-loop` supervisor logic must be redesigned. Instead of acting as a proxy that passes raw JSON to an agent's stdin, the supervisor must act purely as a PTY manager. It will spawn the agent, provide a bootstrap prompt with the daemon's HTTP/SSE MCP endpoint, and allow the agent to autonomously connect, query `tools/list`, call `work.await_packet`, and interact with the event bus natively.

The CLI is no longer the target operator control plane. The cutover path is
to make daemon MCP and the operator UI cover live workflow control first, then
retire or hide the CLI verbs they replace. Bootstrap and diagnostics commands
may survive only when explicitly justified.

## Next 1-3 Actions

1. Land the Go daemon `/mcp/sse` smoke path: MCP initialize, `tools/list`
   from the production-visible tool set, and a deterministic test MCP client.
2. Add one read-only `tools/call` and one low-risk mutating `tools/call`
   through daemon RPC with token/capability denial tests.
3. Prove the lane loop with a fake MCP agent before refactoring the real
   `agent-loop` PTY bootstrap and deleting `src/striatum/mcp.py`.

## Blockers

- Product decisions still block accepted-risk durable persistence,
  default live auto-finalize policy, Corpus Contract V2 choices, and
  optional Git/PR authority.
- No human intervention is needed for the remaining Go/PG remediation
  slices unless a product boundary changes.

## Hazards / Do Not

- Do not write proxy wrappers that poll the daemon and spoon-feed JSON to the agents. Agents MUST operate as autonomous MCP clients.
- Do not delete CLI workflow-control verbs before MCP/UI parity exists and is
  covered by tests; classify any remaining CLI commands as bootstrap,
  diagnostics, or temporary compatibility.
- Do not reopen repo-local SQLite or the legacy daemon registry in
  production paths.
- Do not add hosted services, telemetry, transcript capture, or external
  persistence without a product decision.
- Keep Engram references historical or explicitly optional.

## Pointers

- `docs/operator/plans/rfc-0068-go-daemon-port.md`
- `docs/operator/plans/rfc-0069-pg-only-daemon-global-surfaces.md`
- `docs/rfcs/0050-go-daemon-http-sse-mcp.md`
