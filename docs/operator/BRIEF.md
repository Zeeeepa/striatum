---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-05-20_go-daemon-http-sse-mcp"
supersedes: "brief_2026-05-17_go-daemon-remediation"
scope_links: ["docs/operator/plans/next-steps-1-6.md", "docs/operator/workflows/next-steps-1-6/workflow.json", "docs/operator/plans/remaining-runway-1-8.md", "docs/operator/plans/rfc-0050-cli-retirement-cutover.md", "docs/operator/plans/rfc-0075-tmux-observable-mcp-agent-sessions.md", "docs/operator/plans/ordered-backlog-2026-05-23.md", "docs/operator/workflows/ordered-backlog-2026-05-23/workflow.json", "docs/operator/artifacts/ordered-backlog-2026-05-23/final/SUMMARY.md"]
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
`ready`, `heartbeat`, `question`, and `escalate` reports. The next tmux slices
record daemon-created tmux session metadata, project the attach command
through `supervise`, `status`, and dashboard read surfaces, and let PTY-helper
lanes set `supervision.require_tmux: true` to fail closed when `tmux` is
unavailable. Broader UI polish and CLI retirement remain pending; tmux panes
are still local inspection metadata, not workflow state.

RFC 0076 is accepted by D128. Its first runnable operator workflow completed
at `docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json` and
produced findings, synthesis, and a remediation plan under
`docs/operator/artifacts/rfc-0076-code-doc-audit/`; one Claude lane required
operator recovery. Treat that recovery as run evidence only: durable artifacts
and daemon/PostgreSQL state remain authoritative, and terminal output remains
non-authoritative. The follow-up remediation scaffold is
`docs/operator/plans/rfc-0076-audit-remediation.md`.

RFC 0077 is accepted by D129 and its V1 Go-daemon slice has landed. Migration
0012 persists daemon-owned MCP activity timestamps on sessions; Go MCP
`tools/list`, `work.await_packet`, packet delivery, ack/block/release/complete,
`work.heartbeat`, and `session.report` update the timeline. Status, dashboard
data, and `supervise.status` now project protocol liveness, and the resident
recovery sweep persists stall transitions with metadata-only liveness events.
RFC 0075 still owns the broader tmux-observable session shape and fail-closed
live-lane requirement.

The human-principal checkpoint for TODO 55, 56, 59, and 60 is resolved. D124
chooses daemon-core accepted-risk persistence for workflow lint; D125 keeps
auto-finalize dry-run by default with a three-live-dogfood evidence gate;
D126 accepts the Corpus Contract V2 identity/redaction/archive/verification
direction; D127 sets the optional Git/PR boundary around read-only snapshots,
durable request artifacts, explicit local commit confirmation, and no hosted
provider actions in core. TODO 59 now emits V2 corpus manifest metadata,
including a corpus-scoped incremental export watermark, for new exports while
keeping implied-V1 bundle verification compatible.

The active runway 1-5 workflow completed on 2026-05-22 as
`run_b2e013582e0aeba267dd7a47cc66ccf1`. It produced the ordered batch plan in
`docs/operator/artifacts/active-runway-1-5/FINAL.md`, with supporting phase
artifacts under `docs/operator/artifacts/active-runway-1-5/`. The first
implementation pair from that plan has landed: TODO 60 now has daemon
read-only `git.snapshot` plus `striatum git snapshot --json`, and TODO 56 now
adds stable auto-finalize skipped-candidate `cause` classes and
lane-finalization visibility while preserving existing human-readable
`reason` strings. The next TODO 56 checkpoint added the table-backed
consecutive-failure circuit breaker with open/reset audit events and
force-resistant refusal until explicit live reset. The next daemon-contract
slice also landed the TODO 55 accepted-risk substrate: Go daemon `workflow.lint`,
`workflow.accept_risk`, and `workflow.accepted_risks.list` with append-only
PostgreSQL records bound to workflow snapshots or canonical fingerprints. The
CLI now routes `workflow accepted-risks` and `workflow accept-risk` through
those daemon methods. The next steps 1-6 run
`run_f7659a42616591da5be84a822f8cf36e` landed the follow-through slices:
the web workflow detail page now reads daemon lint/accepted-risk records and
can append accepted-risk records through `workflow.accept_risk` when local
web mutations are enabled; TODO 56 exposes the D125 dry-run default and
default-live evidence gate in recovery policy payloads and validates
`auto_finalize_gate_evidence` artifacts; TODO 60 has `commit_request` and
`pr_request` artifact schemas plus daemon `git.commit_apply`, which creates
only explicit-operator-confirmed local commits; TODO 59 adds optional
workflow `augmentation.mode: "reference_only"` packet metadata for local
corpus bundles without making augmentation a runtime dependency; and TODO
61/62 cleanup removed the retired daemon-registry env export from production
source with a guardrail preventing it from returning. The checked
`docs/architecture/CLI_RETIREMENT_PARITY.md` ledger now classifies non-read
CLI routes before any retirement; no live workflow-control CLI verb is
hidden by this slice.

The ordered backlog workflow completed on 2026-05-23 as
`run_0937abb24a344dc268aa35d7c852359e`; final synthesis is
`docs/operator/artifacts/ordered-backlog-2026-05-23/final/SUMMARY.md`.
It recorded the D125 live gate as still pending, added exact MCP
workflow-control dispatch tests and parity-ledger updates, pruned one legacy
SQLite skills-test fixture skip, split static asset response orchestration
out of `service.py`, accepted D130's link-only escalation artifact policy,
closed F2 publication-policy documentation, and refreshed current-doc generic
language guardrails.

## Next 1-3 Actions

1. Run the D125 live auto-finalize dogfood evidence gate only when ready:
   three live successes across at least two lane shapes with zero contested
   audit-chain events. Keep global behavior dry-run and workflow opt-in until
   that evidence artifact is satisfied.
2. Use `docs/architecture/CLI_RETIREMENT_PARITY.md` to close remaining UI
   gaps before hiding any live workflow-control CLI verb. Bootstrap and
   diagnostics commands should survive unless a later decision says otherwise.
3. Continue bounded cleanup/service follow-through: TODO 61/49/62/63
   residual direct-state or historical fixture cleanup, TODO 52 remaining
   `service.py` route splits, TODO 53 typed escalation table/schema
   hardening, and RFC 0075 tmux-observable/fail-closed lane work.

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
- `docs/operator/plans/active-runway-1-5.md`
- `docs/operator/workflows/active-runway-1-5/workflow.json`
- `docs/operator/artifacts/active-runway-1-5/FINAL.md`
- `docs/operator/plans/ordered-backlog-2026-05-23.md`
- `docs/operator/workflows/ordered-backlog-2026-05-23/workflow.json`
- `docs/operator/artifacts/ordered-backlog-2026-05-23/final/SUMMARY.md`
