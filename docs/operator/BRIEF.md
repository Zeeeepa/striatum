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

## Next 1-3 Actions

1. Implement the HTTP/SSE MCP server in the Go daemon as per RFC 0050.
2. Refactor `go/pkg/agentloop` to act exclusively as a PTY supervisor that injects the daemon SSE endpoint into the agent's bootstrap prompt.
3. Delete `src/striatum/mcp.py` and ensure the Python execution environment is fully decoupled from MCP operations.

## Blockers

- Product decisions still block accepted-risk durable persistence,
  default live auto-finalize policy, Corpus Contract V2 choices, and
  optional Git/PR authority.
- No human intervention is needed for the remaining Go/PG remediation
  slices unless a product boundary changes.

## Hazards / Do Not

- Do not write proxy wrappers that poll the daemon and spoon-feed JSON to the agents. Agents MUST operate as autonomous MCP clients.
- Do not reopen repo-local SQLite or the legacy daemon registry in
  production paths.
- Do not add hosted services, telemetry, transcript capture, or external
  persistence without a product decision.
- Keep Engram references historical or explicitly optional.

## Pointers

- `docs/operator/plans/rfc-0068-go-daemon-port.md`
- `docs/operator/plans/rfc-0069-pg-only-daemon-global-surfaces.md`
- `docs/rfcs/0050-go-daemon-http-sse-mcp.md`
