---
schema_version: "striatum.work_plan.v1"
artifact_kind: "work_plan"
plan_id: "plan_rfc-0075-tmux-observable-mcp-agent-sessions"
scope_kind: "rfc"
scope_ref: "docs/rfcs/0075-tmux-observable-mcp-agent-sessions.md"
state: "closed"
opened_at: "2026-05-21"
closed_at: "2026-05-23"
closure_summary: "Accepted by D131 for the current local-first contract: session.report, RFC 0077 protocol liveness, tmux attach metadata, web/status/dashboard/supervise projections, fail-closed tmux opt-in, and no-transcript guardrails have landed."
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
| RFC accepted and implementation slices scoped | landed; D131 |
| Tmux-backed live-interactive supervisor metadata | landed for current scope; attach metadata projects through daemon reads, terminal dashboard, and web run detail; `supervision.require_tmux` provides fail-closed opt-in |
| MCP protocol activity timestamps and liveness classifier | landed via RFC 0077 V1 |
| Pre-work session tools for ready/heartbeat/question/escalation | landed as `session.report` |
| Deadline sweeper and status/dashboard/operator surfaces | landed for current scope; recovery sweep persists transitions and read surfaces project liveness/tmux metadata |
| No-transcript/no-terminal-authority guardrails | landed |
| Fake-agent tests for discovery, await, ack, heartbeat, and question stalls | landed through RFC 0077/session-liveness coverage |

## Decisions Made

- Terminal panes are an observability surface, not state authority.
- Broad transcript capture remains out of scope by default.
- RFC 0050 MCP calls and daemon PostgreSQL remain the workflow truth.

## Resolved / Deferred Questions

- Startup health uses one typed `session.report` method.
- Current deadline defaults live in RFC 0077's daemon liveness policy.
- Current tmux strictness is lane opt-in through `supervision.require_tmux`.
  Universal live-interactive defaults or equivalent local PTY multiplexers need
  a later explicit product decision.
