---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-05-20_go-daemon-http-sse-mcp"
supersedes: "brief_2026-05-17_go-daemon-remediation"
scope_links: ["docs/operator/plans/rfc-0068-go-daemon-port.md", "docs/operator/plans/rfc-0069-pg-only-daemon-global-surfaces.md", "docs/operator/plans/rfc-0050-cli-retirement-cutover.md", "docs/operator/plans/rfc-0075-tmux-observable-mcp-agent-sessions.md", "docs/rfcs/0075-tmux-observable-mcp-agent-sessions.md", "docs/rfcs/0077-mcp-activity-liveness-deadlines.md", "docs/operator/plans/rfc-0076-audit-remediation.md"]
context_budget_lines: 300
retrieval_priority: "high"
status: "current"
---

# Operator Brief
author: antigravity-001

## State

Striatum's live-state boundary is daemon-owned PostgreSQL. The active
remediation runway is D107/RFC 0068: Go is the production/default daemon.
RFC 0050 Phase A-D has landed in the native Go daemon: `striatumd` serves MCP
over loopback HTTP at `/mcp`, keeps `/mcp/sse` as the SSE/backcompat alias,
capability-filters `tools/list`, routes `tools/call` through daemon RPC, and
publishes the current endpoint in the daemon runtime as `mcp-http-endpoint`.
The daemon-backed fake MCP agent proof now completes a workflow packet loop
through `/mcp`: prepare/start, session registration, `work.await_packet`,
`work.ack`, `work.heartbeat`, `artifact.publish`, and `work.complete`, with
stale lease refusal covered. The current source also has the Python `mcp.py`
wrapper removed.

The `agent-loop` supervisor is now a PTY bootstrapper instead of a proxy that
passes raw JSON to an agent's stdin. It spawns the agent, provides a bootstrap
prompt with the daemon's HTTP/SSE MCP endpoint, and lets the agent connect,
query `tools/list`, call `work.await_packet`, and interact with the event bus
natively.

The CLI is no longer the target operator control plane. The cutover path is
to make daemon MCP and the operator UI cover live workflow control first, then
retire or hide the CLI verbs they replace. Bootstrap and diagnostics commands
may survive only when explicitly justified.

RFC 0075 is proposed, scaffolded, and has completed its first cutover
workflow slice at
`docs/operator/workflows/rfc-0075-and-mcp-cutover/workflow.json`. The landed
slice adds `session.report` as the claim-gated MCP path for pre-packet
`ready`, `heartbeat`, `question`, and `escalate` reports. Tmux metadata,
liveness timestamp persistence, deadline classification, and status/UI
projection remain pending; tmux panes are still local inspection metadata, not
workflow state.

RFC 0076 is accepted by D128. Its first runnable operator workflow completed
at `docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json` and
produced findings, synthesis, and a remediation plan under
`docs/operator/artifacts/rfc-0076-code-doc-audit/`; one Claude lane required
operator recovery. Treat that recovery as run evidence only: durable artifacts
and daemon/PostgreSQL state remain authoritative, and terminal output remains
non-authoritative. The follow-up remediation scaffold is
`docs/operator/plans/rfc-0076-audit-remediation.md`.

RFC 0077 is now the proposed narrow liveness slice under the RFC 0075
umbrella. It owns daemon-owned MCP activity timestamp persistence and liveness
deadline classification for discovery, `work.await_packet`, ack, heartbeat,
structured question, and escalation states. RFC 0075 still owns the broader
tmux-observable session shape.

The human-principal checkpoint for TODO 55, 56, 59, and 60 is resolved. D124
chooses daemon-core accepted-risk persistence for workflow lint; D125 keeps
auto-finalize dry-run by default with a three-live-dogfood evidence gate;
D126 accepts the Corpus Contract V2 identity/redaction/archive/verification
direction; D127 sets the optional Git/PR boundary around read-only snapshots,
durable request artifacts, explicit local commit confirmation, and no hosted
provider actions in core.

## Next 1-3 Actions

1. Implement RFC 0077: persist MCP activity timestamps and project liveness
   classification for discovery, `work.await_packet`, ack, heartbeat,
   question, and escalation stalls.
2. Apply the TODO 55/56/59/60 follow-ups: daemon accepted-risk mutation
   surfaces, auto-finalize observability/circuit-breaker work, Corpus Contract
   V2 schema/archive defaults, and the read-only local Git snapshot slice.
3. Continue CLI-retirement work: move remaining live operator actions to
   MCP/UI surfaces and classify any CLI survivors as bootstrap, diagnostics,
   or temporary compatibility.

## Blockers

- No product-decision blocker remains for TODO 55, 56, 59, or 60; D124-D127
  define the follow-up boundaries. Hosted Git provider behavior remains out
  of core unless a later optional-plugin decision accepts it.
- No human intervention is needed for the remaining Go/PG remediation
  slices unless a product boundary changes.

## Hazards / Do Not

- Do not write proxy wrappers that poll the daemon and spoon-feed JSON to the agents. Agents MUST operate as autonomous MCP clients.
- Do not treat tmux panes, pane text, or transcripts as workflow state; RFC
  0075 keeps tmux local and observational only.
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
- `docs/operator/plans/rfc-0050-cli-retirement-cutover.md`
- `docs/operator/plans/rfc-0075-tmux-observable-mcp-agent-sessions.md`
- `docs/operator/workflows/rfc-0075-and-mcp-cutover/workflow.json`
- `docs/operator/artifacts/rfc-0075-and-mcp-cutover/final/SUMMARY.md`
- `docs/rfcs/0077-mcp-activity-liveness-deadlines.md`
- `docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json`
- `docs/operator/artifacts/rfc-0076-code-doc-audit/REMEDIATION_PLAN.md`
- `docs/operator/plans/rfc-0076-audit-remediation.md`
- `docs/operator/workflows/rfc-0076-audit-remediation/workflow.json`
