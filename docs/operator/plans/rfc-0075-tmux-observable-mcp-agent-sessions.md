---
schema_version: "striatum.work_plan.v1"
artifact_kind: "work_plan"
plan_id: "plan_rfc-0075-tmux-observable-mcp-agent-sessions"
scope_kind: "rfc"
scope_ref: "docs/rfcs/0075-tmux-observable-mcp-agent-sessions.md"
state: "open"
opened_at: "2026-05-21"
closed_at: null
closure_summary: null
supersedes: null
retrieval_priority: "high"
---

# RFC 0075 Tmux-Observable MCP Agent Sessions Plan
author: coordinator-codex-gpt-5-001

## Outcome

Make post-RFC-0050 live interactive agents observable without making
terminal output authoritative. Live interactive lanes should run in
daemon-created tmux-backed PTYs, expose attach metadata to operators, and
surface MCP startup/work-loop liveness deadlines while keeping daemon
PostgreSQL and MCP/RPC calls as the only live workflow-state authority.

## Workstreams

| Workstream | State |
|---|---|
| RFC accepted and implementation slices scoped | open |
| Tmux-backed live-interactive supervisor metadata | open |
| MCP protocol activity timestamps and liveness classifier | open |
| Pre-work session tools for ready/heartbeat/question/escalation | open |
| Deadline sweeper and status/dashboard/operator surfaces | open |
| No-transcript/no-terminal-authority guardrails | open |
| Fake-agent tests for discovery, await, ack, heartbeat, and question stalls | open |

## Decisions Made

- Terminal panes are an observability surface, not state authority.
- Broad transcript capture remains out of scope by default.
- RFC 0050 MCP calls and daemon PostgreSQL remain the workflow truth.

## Open Questions

- Whether startup-health methods should be separate
  `session.ready` / `session.heartbeat` / `session.question` /
  `session.escalate` tools or one typed `session.report` method.
- Which deadline defaults belong in daemon config versus workflow lane
  constraints.
- Whether tmux remains the only required local multiplexer or the contract
  later accepts equivalent local PTY multiplexers.
