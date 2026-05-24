---
schema_version: "striatum.operator_brief.v1"
artifact_kind: "operator_brief"
brief_id: "brief_2026-05-24_current_todo_1_5"
supersedes: "brief_2026-05-23_rfc0075_polish_closure"
scope_links: ["docs/operator/plans/current-todo-1-5-2026-05-24.md", "docs/operator/workflows/current-todo-1-5-2026-05-24/workflow.json", "docs/operator/artifacts/current-todo-1-5-2026-05-24/final/SUMMARY.md", "docs/operator/artifacts/d125-auto-finalize-live-synthesis-evidence-2026-05-24/GATE.md", "docs/operator/artifacts/rfc-0050-0075-final-cutover-implementation/final/SUMMARY.md", "docs/architecture/CLI_RETIREMENT_PARITY.md"]
context_budget_lines: 300
retrieval_priority: "high"
status: "current"
---

# Operator Brief
author: operator-codex-001

## State

Striatum's live-state boundary is daemon-owned PostgreSQL. Go is the
production daemon core, repository files are durable provenance, and
`.striatum/` remains operational scratch only. Do not reintroduce repo-local
SQLite authority, hosted services, telemetry, transcript capture, or external
persistence without a product decision.

The daemon MCP path is the target operator-control surface. `striatumd`
serves MCP over loopback HTTP at `/mcp`, keeps `/mcp/sse` as the SSE/backcompat
alias, capability-filters `tools/list`, routes `tools/call` through daemon
RPC, and publishes the current endpoint as `mcp-http-endpoint`. The
`agent-loop` supervisor is a PTY bootstrapper: agents connect as autonomous
MCP clients instead of being fed polled JSON by a proxy wrapper.

The RFC 0050/RFC 0075 live workflow-control cutover completed on 2026-05-23.
No human or AI live workflow-control action now requires invoking
`striatum` CLI verbs. Agents are documented to use daemon MCP first; humans
use the local web UI for operator actions. Surviving CLI commands are
classified in `docs/architecture/CLI_RETIREMENT_PARITY.md` as bootstrap,
lane compatibility, or operator compatibility clients of daemon RPC. This
cutover does not delete CLI commands; hiding or removing compatibility verbs
is a later deprecation/release decision. The scaffolded design workflow ran as
`run_4a5eb33b0d6b037e9f62a0335d04b349`; the implementation workflow ran as
`run_ee2973e23ad697085a52766410906940`. Both completed.

RFC 0075 is accepted for the current tmux-observable MCP-session contract
(D131). The landed scope includes daemon-owned MCP activity timestamps,
`session.report`, protocol liveness projections in status, dashboard,
`supervise.status`, and web run-detail session chips, metadata-only stall
transition events, tmux attach metadata projection, fail-closed
`supervision.require_tmux` opt-in for PTY-helper lanes, and guardrails that
keep tmux panes, pane text, terminal output, and transcripts as inspection
metadata only, never workflow state.

The TODO 55/56/59/60 product checkpoint is resolved by D124-D127 plus D133.
D125's evidence gate is satisfied by three successful live dogfoods across
review, build, and synthesis lane shapes with zero current contested
audit-chain events. D133 flips auto-finalize live allowance on by default for
workflows that do not explicitly opt out; the manual CLI/RPC command still
defaults to dry-run preview unless live mode is requested.

The ordered backlog workflow completed on 2026-05-23 as
`run_0937abb24a344dc268aa35d7c852359e`. It recorded the D125 live gate as
still pending, expanded MCP workflow-control dispatch tests and the
CLI-retirement parity ledger, pruned a legacy SQLite skills-test fixture skip,
split static asset response orchestration out of `service.py`, accepted D130's
link-only escalation artifact policy, closed F2 publication-policy docs, and
refreshed current-doc generic language guardrails.

The next TODO workflow completed on 2026-05-23 as
`run_492ecd5cf520f170be6a02414d576cd3`; final synthesis is
`docs/operator/artifacts/next-todos-2026-05-23/final/SUMMARY.md`. It produced
one operator-self-declared live D125 success in a review-shape workflow and a
pending gate artifact, expanded exact MCP dispatch parity and service-route
coverage for `review.override`, reactivated override-modal context tests,
moved doctor page rendering/error mapping into `web/doctor.py`, corrected RFC
0062's typed-escalation-inbox status, added a tmux authority-boundary
architecture guardrail, and updated TODO/roadmap status for the doctor split
and typed escalation inbox table.

A follow-up D125 evidence slice completed on 2026-05-23 as
`run_6ff2b4939f9a37987cc9fb38413b8079`; report and pending gate artifacts are
under `docs/operator/artifacts/d125-auto-finalize-live-build-evidence-2026-05-23/`.
It added an operator-self-declared build-shape live `recovery.auto_finalize`
success through workflow opt-in without `--force`.

The final D125 evidence slice completed on 2026-05-24 as
`run_3d182acb046f7b09dbc0dbd9a3a90363`; evidence and satisfied gate artifacts
are under
`docs/operator/artifacts/d125-auto-finalize-live-synthesis-evidence-2026-05-24/`.
It added an operator-self-declared synthesis-shape live
`recovery.auto_finalize` success through workflow opt-in without `--force`, and
the export-time daemon doctor reported no audit-chain problems.

A residual/deferred closure pass completed on 2026-05-23. It scaffolded and
drove per-item workflows for TODO 62, TODO 63, TODO 16, TODO 2, artifact
schema/redaction coverage, RFC 0040 packet-evidence debt, and the deferred
items formerly listed as 14-27. The final synthesis is
`docs/operator/artifacts/residual-deferred-closure-2026-05-23/final/SUMMARY.md`.
Current-scope closures: TODO 62 is done with PG/global guardrails covering
future probes; TODO 63 is done with primitive daemon methods as the supported
production path; TODO 2 is done for the process adapter; artifact schema and
redaction coverage is current; RFC 0040 packet-evidence debt is closed by
PostgreSQL artifact byline evidence; TODO 16 has a refreshed current-doc
guardrail. Explicit non-core/no-action closures: RFC 0049 remains shelved,
RFC 0054/0055/0056 optional doc/layout follow-ups are no-action, TODO 59
external fetch UX and TODO 60 hosted providers are out of core, RFC 0058
operator-tree init/rotation is optional, and Engram-side memory tools are
external to Striatum.

D125 is satisfied for the current repository state. The gate artifact records
3 operator-self-declared live behavioral successes across 3 lane shapes and
`contested_audit_chain_events: 0`. D133 now implements the bounded default-live
policy change: absent workflow policy allows live auto-finalize, while
`recovery.auto_finalize.enabled=false` is the workflow-level opt-out.

The current TODO 1-5 workflow completed on 2026-05-24 as
`run_f84b4145a7ee371c4b17cc6fc2c29880`; final synthesis is
`docs/operator/artifacts/current-todo-1-5-2026-05-24/final/SUMMARY.md`.
It satisfied the D125 evidence gate, deleted six stale skipped legacy SQLite
fixtures while preserving current PG/daemon coverage, split historical
dogfood route handling out of `service.py`, hardened `work.block` escalation
payloads across Python/Go runtime paths, and accepted/landed RFC 0074 Phase B
`implementation_panel` generator support in Python and Go. Aggregate
validation passed with `go test ./...`, full Python pytest, workflow
validation, and catalog freshness checks.

The follow-up TODO 49/61 cleanup on 2026-05-24 deleted the remaining legacy
local-state implementation residue: the `src/striatum/legacy_sqlite/` package
was already absent, and this slice removes the direct corpus exporter, the V1
local-state schema module, the deterministic repo-local fixture, and the stale
skipped compatibility tests. Active guardrails now require no production or
active test imports of `sqlite3`, `striatum.legacy_sqlite`, `striatum.db`, or
`striatum.migrations`. The retired `retired-local-state` file name remains only as a
refusal/inspection signal for old working copies and as a redaction suffix.

## Next 1-3 Actions

1. Monitor D133 default-live auto-finalize and use
   `recovery.auto_finalize.enabled=false` only for workflows that require
   strict agent-only finalization.
2. Treat RFC 0050/RFC 0075 cutover as complete for live workflow control.
   Keep the CLI survivor categories in `docs/architecture/CLI_RETIREMENT_PARITY.md`
   current when adding daemon methods. Hide/delete compatibility CLI verbs only
   through a later explicit deprecation/release decision.
3. Continue bounded follow-through on later service split slices. Schedule new
   bounded RFCs before implementing RFC 0052 Phase A, RFC 0053 schema/runtime
   rename, Cross-Repo Live Scheduler V1, sealed apply, Windows support, or
   local multi-operator tenancy.

## Blockers

- D125 default-live auto-finalize is no longer blocked on evidence or policy.
  D133 has landed; any rollback or narrower per-job policy needs a new bounded
  decision.
- RFC 0050/RFC 0075 live workflow-control cutover is no longer blocked.
  Remaining CLI compatibility cleanup is a future deprecation policy question,
  not an active parity blocker.
- Hosted Git provider behavior, external corpus-fetch UX, and Engram-side
  memory tools are out of core unless later optional-extension decisions
  accept them.

## Hazards / Do Not

- Do not write proxy wrappers that poll the daemon and spoon-feed JSON to
  agents. Agents must operate as autonomous MCP clients.
- Do not treat tmux panes, pane text, terminal output, or transcripts as
  workflow state.
- Do not delete CLI compatibility verbs without an explicit deprecation/release
  decision. They are daemon clients, not live-state authorities.
- Do not reopen retired repo-local state files or the legacy daemon registry in
  production paths.
- Do not add hosted services, telemetry, transcript capture, or external
  persistence without a product decision.
- Keep Engram references historical or explicitly optional.

## Pointers

- `docs/operator/plans/next-todos-2026-05-23.md`
- `docs/operator/plans/residual-deferred-closure-2026-05-23.md`
- `docs/operator/artifacts/residual-deferred-closure-2026-05-23/final/SUMMARY.md`
- `docs/operator/workflows/next-todos-2026-05-23/workflow.json`
- `docs/operator/artifacts/next-todos-2026-05-23/final/SUMMARY.md`
- `docs/operator/workflows/d125-auto-finalize-live-build-evidence-2026-05-23/workflow.json`
- `docs/operator/artifacts/d125-auto-finalize-live-build-evidence-2026-05-23/REPORT.md`
- `docs/operator/artifacts/d125-auto-finalize-live-build-evidence-2026-05-23/GATE.md`
- `docs/operator/workflows/d125-auto-finalize-live-synthesis-evidence-2026-05-24/workflow.json`
- `docs/operator/artifacts/d125-auto-finalize-live-synthesis-evidence-2026-05-24/GATE.md`
- `docs/operator/plans/current-todo-1-5-2026-05-24.md`
- `docs/operator/workflows/current-todo-1-5-2026-05-24/workflow.json`
- `docs/operator/artifacts/current-todo-1-5-2026-05-24/final/SUMMARY.md`
- `docs/operator/plans/rfc-0050-0075-final-cutover-design.md`
- `docs/operator/plans/rfc-0050-0075-final-cutover-implementation.md`
- `docs/operator/artifacts/rfc-0050-0075-final-cutover-implementation/final/SUMMARY.md`
- `docs/operator/plans/rfc-0075-tmux-observable-mcp-agent-sessions.md`
- `docs/rfcs/0077-mcp-activity-liveness-deadlines.md`
- `docs/architecture/CLI_RETIREMENT_PARITY.md`
