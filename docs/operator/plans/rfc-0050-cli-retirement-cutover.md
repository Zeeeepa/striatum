---
schema_version: "striatum.work_plan.v1"
artifact_kind: "work_plan"
plan_id: "plan_rfc-0050-cli-retirement-cutover"
scope_kind: "initiative"
scope_ref: "docs/rfcs/0050-go-daemon-http-sse-mcp.md"
state: "open"
opened_at: "2026-05-21"
closed_at: null
closure_summary: null
supersedes: null
retrieval_priority: "high"
---

# RFC 0050 CLI Retirement Cutover Plan
author: coordinator-codex-gpt-5-001

## Outcome

Finish the cutover from CLI-driven live workflow control to daemon MCP and
operator UI surfaces. CLI commands may remain for bootstrap, diagnostics, and
temporary compatibility only when explicitly classified and backed by a
replacement plan or narrow operational justification.

## Workstreams

| Workstream | State |
|---|---|
| Native Go `/mcp` endpoint, tool discovery, and tool calls | landed |
| Fake MCP agent work-packet loop proof | landed |
| PTY bootstrapper agent loop | landed |
| Python MCP wrapper deletion | landed |
| Cutover map of remaining workflow-control CLI verbs | open |
| MCP parity for remaining live operator actions | open |
| Operator UI parity for human-principal and run-control actions | open |
| CLI survivor classification: bootstrap, diagnostics, or compatibility | open |
| MCP/UI-first docs, skills, and examples | open |
| Deprecate, hide, or delete replaced workflow-control CLI verbs | open |

## Decisions Made

- Do not delete workflow-control CLI verbs before MCP/UI parity exists and is
  covered by tests.
- Use primitive daemon methods instead of resurrecting removed dogfood
  composites unless a new product decision accepts a PostgreSQL-native
  composite.
- Hosted provider actions, telemetry, and external persistence stay out of
  core.

## Open Questions

- Which run/session/recovery/escalation actions require UI parity before a
  CLI verb can be hidden.
- Whether any bootstrap command should remain CLI-only permanently.
- Whether the cutover should ship as one release gate or as a per-verb
  compatibility ledger.
