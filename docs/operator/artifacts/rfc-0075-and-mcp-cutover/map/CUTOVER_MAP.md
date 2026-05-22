---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["README.md", "docs/SPEC.md", "docs/MCP.md", "docs/rfcs/0050-go-daemon-http-sse-mcp.md", "docs/rfcs/0075-tmux-observable-mcp-agent-sessions.md", "contracts/daemon_methods.json"]
---

# MCP/UI Cutover Map
author: mapper-codex-002

## Scope

This map classifies the remaining CLI surface before RFC 0050 Phase F deletion
work. The target state is not "no `striatum` binary"; it is "no live workflow
control operation requires an operator or lane agent to drive state through CLI
verbs." The daemon MCP and local operator UI become the normal control planes.
CLI survivors must be bootstrap, diagnostics, or temporary compatibility.

Current source state:

- Native Go `/mcp` and `/mcp/sse` are the production MCP transport.
- MCP `tools/list` is capability-filtered from `contracts/daemon_methods.json`.
- MCP `tools/call` re-enters daemon RPC and audits as `transport = "mcp"`.
- The fake MCP agent loop covers `run.prepare`, `run.start`,
  `session.register`, `work.await_packet`, `work.ack`, `work.heartbeat`,
  `artifact.publish`, `work.complete`, and stale-lease refusal.
- `go/pkg/agentloop` is now a PTY bootstrapper: it injects endpoint, token,
  repository id, run id, and session id, then leaves the agent to call MCP.
- Production MCP discovery hides local workflow-authoring methods
  `workflow.validate`, `workflow.plan`, `workflow.graph`,
  `workflow.templates.list`, `workflow.templates.show`, `workflow.init`,
  `workflow.generate`, and `workflow.upgrade`.

## Classification Legend

| Class | Meaning |
|---|---|
| MCP-ready | Daemon method is non-deprecated and suitable for direct MCP use now. |
| UI-parity-needed | Daemon method exists, but deletion should wait for an operator UI path or documented MCP-first operator flow. |
| Bootstrap survivor | CLI can remain because it starts, provisions, registers, installs, or launches the local system. |
| Diagnostic survivor | CLI can remain because it inspects, verifies, exports, or debugs local state without being the normal workflow-control path. |
| Temporary compatibility | Keep only until docs, skills, examples, and replacement parity tests stop depending on it. |
| Delete/hide after parity | Hide from docs/help or remove once MCP/UI replacement is covered. |

## Lane Agent Work Loop

| CLI verb family | Daemon/MCP replacement | Classification | Cutover action |
|---|---|---|---|
| `register-session`, `session close` | `session.register`, `session.close` | MCP-ready; temporary CLI compatibility | Teach agents to register/close through MCP. Hide/delete CLI loop use after real-agent RFC 0050 Phase E evidence. |
| `claim-next` | `work.await_packet` | MCP-ready; temporary CLI compatibility | Retire CLI claim language. `claim-next` should become compatibility only because MCP uses await semantics. |
| `ack`, `heartbeat`, `release` | `work.ack`, `work.heartbeat`, `work.release` | MCP-ready; temporary CLI compatibility | Keep packet-supplied CLI commands only for old supervised/manual packets until skills/docs switch to MCP. |
| `block`, `send` | `work.block`, `work.send_message` | MCP-ready; RFC 0075 gap before packet | Keep `work.block` for packet-scoped blockers. Add RFC 0075 pre-work session question/escalation method before relying on pane text. |
| `publish-artifact` | `artifact.publish` | MCP-ready; temporary CLI compatibility | MCP already preserves author/byline/write-scope validation. Delete CLI-first instructions after artifact publish parity tests cover common artifact kinds. |
| `complete` | `work.complete` | MCP-ready; temporary CLI compatibility | Replace packet command blocks with MCP tool instructions once real agents can call MCP natively. |
| `verdict`, `submit-review` | `review.verdict`, `review.submit` | MCP-ready; temporary CLI compatibility | Keep until review-lane MCP docs and UI review evidence are first-class. |
| `override-verdict` | `review.override` | UI-parity-needed | Human-principal/admin action. Prefer UI affordance with rationale capture before hiding CLI. |

Deletion gate for this family: no current or newly rendered agent skill should
teach the CLI claim/ack/publish/complete loop as the happy path. Work packets
may keep CLI commands as backward-compatible escape hatches until all live
interactive profiles are MCP-native.

## Run And Operator Control

| CLI verb family | Daemon/MCP replacement | Classification | Cutover action |
|---|---|---|---|
| `run prepare`, `run start` | `run.prepare`, `run.start` | MCP-ready; UI-parity-needed | MCP proof exists. Operator UI needs an explicit prepare/start flow for selected workflow files before CLI can be hidden. |
| `run pause`, `run resume`, `run cancel`, `run retry-job` | matching `run.*` methods | UI-parity-needed | These are principal/operator controls. Hide/delete CLI only after run-detail UI has parity and audit/rationale fields where needed. |
| `branch confirm` | `branch.confirm` | UI-parity-needed | Keep CLI until UI shows recorded/current branch mismatch and confirms with audit evidence. |
| `checkpoint resolve` | `checkpoint.resolve` | UI-parity-needed | Human-principal path. Must be available in inbox/run-detail UI before CLI retirement. |
| `decision record` | `decision.record` | UI-parity-needed | Admin method exists, but decision artifact authoring and persistence need UI/structured MCP flow before CLI removal. |
| `escalation list/show/resolve`, `inbox` | `escalation.*` | UI-parity-needed | Keep CLI until the principal inbox is fully operator-facing in UI. |
| `recovery stale-leases`, `requeue-stale`, `cancel-job`, `process-reconcile`, `resume`, `auto-publish`, `auto-finalize` | `recovery.*` methods | UI-parity-needed; diagnostic survivor for read-only reports | Keep CLI while recovery is operationally sensitive. Prefer UI dry-run/confirm flows for mutating recovery before deletion. |
| `recovery auto` | `recovery.auto` deprecated alias | Delete/hide after parity | Contract marks it deprecated; use `recovery.sweep` or explicit recovery methods. |
| `cross-repo list/describe/why/cancel` | `cross_repo.*` | UI-parity-needed | Read verbs can remain diagnostics; cancel needs UI/admin confirmation before CLI removal. |

Deletion gate for this family: operator UI must handle run start/stop/retry,
checkpoint, escalation, and recovery without requiring a copied CLI command.
MCP method existence alone is not enough because these operations are normally
human-principal/operator decisions, not lane-agent work.

## Observation, Evidence, And Diagnostics

| CLI verb family | Daemon/MCP replacement | Classification | Cutover action |
|---|---|---|---|
| `status`, `why`, `dashboard`, `list runs/sessions/jobs/artifacts/workflows` | `status`, `why`, `dashboard`, `list.*` plus UI pages | Diagnostic survivor | These can stay indefinitely as local terminal diagnostics. Docs should prefer UI/MCP for live operation, but read-only terminal inspection is legitimate. |
| `run summary`, `run graph` | `run.summary`, `run.graph` | Diagnostic survivor | Keep as export/inspection tools. UI can render the same information. |
| `evidence export`, `corpus export`, `archive create` | matching daemon methods | Diagnostic survivor | These create audit/replay artifacts, not the normal live-control loop. Keep CLI for scripts. |
| `corpus verify`, `archive verify`, `apply receipt show/verify` | verifier/read methods or local file verification | Diagnostic survivor | These should remain scriptable diagnostics. They are not workflow-control drivers. |
| `doctor`, `daemon doctor`, `daemon health/status/audit/sweep` | daemon read/admin diagnostics | Diagnostic/bootstrap survivor | Keep. These are local substrate and audit-chain operational tools. |
| `operator current-brief` | repository docs read | Diagnostic survivor | Keep or fold into UI; not a live state mutation. |

RFC 0075 adds a required observability gap here: live interactive sessions need
tmux attach metadata, MCP protocol timestamps, and liveness classifications on
status/dashboard/UI surfaces. Until that lands, deleting CLI observation paths
would reduce operator safety.

## Bootstrap, Authoring, And Local Setup

| CLI verb family | Daemon/MCP replacement | Classification | Cutover action |
|---|---|---|---|
| `daemon start/stop/service install/service start/service status` and `striatumd` flags | none; starts the control plane | Bootstrap survivor | Keep. MCP/UI cannot replace the command that starts the daemon or installs local service units. |
| `repo add/list/remove`, `adopt`, `init` | `repo.*`, `repo.init` where applicable | Bootstrap survivor | Keep CLI for day-zero adoption and repository registration. UI may help, but bootstrap cannot assume UI is already running. |
| `skills install`, `plugin install/uninstall`, `self-update` | none in daemon workflow contract | Bootstrap survivor | Keep out of live workflow-control retirement. |
| `workflow validate/lint/plan/graph/templates/init/generate/upgrade` | daemon methods exist for several, but MCP hides production authoring methods | Bootstrap/authoring survivor | Keep CLI and local UI authoring. These prepare durable workflow files; they are not live run control. MCP can keep preview visible, but production `tools/list` should continue hiding file-writing authoring tools unless a separate decision changes it. |
| `serve --web` | local web UI process | Bootstrap survivor | Keep. It starts the local UI rather than controlling a run itself. Mutating web actions must remain separately gated. |
| `adapter run` | process execution helper | Temporary compatibility | Do not teach as post-MCP live-agent control. Replace live interactive use with daemon-owned PTY/tmux agent loop. |
| `supervise start/send/stop/status/list` | `supervise.*` plus RFC 0050 agent loop and RFC 0075 tmux metadata | Temporary compatibility | `supervise.send` is the old JSON-packet delivery shape and should disappear from happy-path docs. Status/list may survive as diagnostics after tmux metadata lands. |
| `worktree create/release/list` | `worktree.*` | Temporary compatibility / diagnostic | Agents should use packet/MCP workflow. Manual CLI remains useful for recovery until UI parity. |
| retired `daemon migrate`, `daemon migrate-repo-local` spellings | refuse with exit code 12 | Delete/hide after compatibility window | Keep only as compatibility diagnostics warning operators away from repo-local SQLite. |

## Gaps Blocking CLI Retirement

1. Real interactive agent proof: the fake MCP loop is sufficient for method
   semantics, but Phase F should wait for at least one real MCP-capable live
   lane completing a packet without CLI command execution.
2. RFC 0075 liveness tools: add session-level `ready` / `heartbeat` /
   `question` / `escalate` or an equivalent typed method before expecting
   pre-packet agents to avoid terminal-only questions.
3. Operator UI parity: prepare/start, pause/resume/cancel/retry, checkpoint
   resolution, escalation inbox, verdict override, decision record, and
   recovery confirmation need UI routes with audit/rationale handling.
4. Documentation and skill rewrite: `README.md`, `docs/HOW_TO_AGENT.md`, MCP
   docs, generated skills, and examples must teach MCP/UI-first operation.
5. Help-surface compatibility: after docs move, hide or demote CLI workflow
   loop verbs in help output before deleting parser support outright.
6. Guardrail tests: add tests that fail when a deleted/hid CLI verb is still
   presented as the primary path, and tests that prove MCP/UI replacement
   parity for each removed workflow-control action.

## Recommended Deletion Order

1. Convert docs and generated skills from packet CLI command loops to MCP tool
   loops while preserving packet command blocks as compatibility.
2. Land RFC 0075 session liveness methods and tmux-observable status fields.
3. Add UI parity for operator/principal actions: run control, checkpoint,
   escalation, decision, override, and recovery.
4. Hide deprecated aliases and old packet-delivery commands first:
   `claim-next` happy-path docs, `supervise send`, `recovery auto`, and legacy
   un-dotted alias method names.
5. Hide or demote lane-agent CLI loop verbs in help/docs:
   `ack`, `heartbeat`, `release`, `block`, `publish-artifact`, `complete`,
   `verdict`, `submit-review`.
6. Keep bootstrap and diagnostics visible: daemon lifecycle, repo adoption,
   workflow authoring, status/dashboard/list, doctor, evidence/corpus/archive,
   and verifier commands.

## Bottom Line

The daemon MCP replacement for the lane work loop is mostly ready. The risky
remaining surface is not the agent primitive loop; it is the operator/principal
control plane around starting runs, resolving checkpoints, handling escalation,
overriding/recovering state, and observing live interactive sessions. CLI
deletion should therefore proceed family-by-family after UI parity and RFC
0075 liveness evidence, while bootstrap and diagnostics remain explicit
survivors.
