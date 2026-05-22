---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
tags: ["authority-boundary", "rfc-0075", "rfc-0050", "mcp", "no-transcript", "cli-retirement"]
---

# Authority-Boundary Review — RFC 0075 / RFC 0050 MCP Cutover Scaffold
author: reviewer-claude-code-001

## Scope

Authority-boundary read of the proposed/scaffolded slice that lands RFC 0075
("Tmux-Observable MCP Agent Sessions") on top of the RFC 0050 native Go
daemon HTTP MCP transport. Reviewed for regressions against four boundary
invariants:

1. **No terminal authority** — terminal/tmux output is observability only;
   workflow state remains daemon-owned PostgreSQL plus MCP/RPC calls.
2. **No transcript** — durable transcript capture, model-text persistence,
   and pane-as-artifact are disallowed by default.
3. **Daemon/MCP capability** — every MCP `tools/call` re-enters daemon RPC
   with the same capability gate, repository scope, and audit class as the
   Unix-socket RPC path.
4. **CLI-retirement boundary** — CLI verbs are not silently re-introduced
   as the live workflow control plane; remaining CLI is bootstrap,
   diagnostics, or staged compatibility.

The review is `document_only` per the work packet's review policy. Findings
are drawn from the listed `context.docs` only (RFC 0050, RFC 0075, the two
operator plans, `docs/MCP.md`, `docs/SPEC.md`, `docs/HOW_TO_AGENT.md`,
`docs/operator/BRIEF.md`, `contracts/daemon_methods.json`,
`docs/architecture/COMMAND_AUTHORITY_MATRIX.md`,
`go/pkg/agentloop/loop.go` and `bootstrap.go`, `go/pkg/mcp/http.go` and
`tools.go`, `go/pkg/mutations/lifecycle.go` and `claim.go`,
`tests/test_mcp_fake_agent_loop_e2e.py`, `AGENTS.md`).

## Verdict

**accept** — no boundary regressions identified.

The slice is a deliberately small scaffold step that lines up the RFC 0075
contract (MCP-routed pre-work session reports + bootstrap prompt
reinforcement) with the RFC 0050 transport that already landed. It does
not yet implement the tmux requirement, liveness deadlines, or stall
classifier — those workstreams remain "open" in
`docs/operator/plans/rfc-0075-tmux-observable-mcp-agent-sessions.md`, and
the RFC itself is still "proposed". That deferral is correct for an
authority-boundary slice; the slice does not add a half-implemented
liveness path that could mistakenly become state authority.

## Boundary-by-boundary findings

### 1. No terminal authority — PASS

- `docs/MCP.md` § Boundary: "Repository files remain provenance; PostgreSQL
  remains live workflow state. `.striatum/` beside a target repository is
  operational scratch, not an MCP message bus."
- `docs/MCP.md` § Agent Loop is explicit: "The supervisor does not call
  `work.await_packet`, claim work, complete work, release work, or write
  packet JSON. The agent is responsible for using MCP."
- `go/pkg/agentloop/loop.go::runWithIO` opens a PTY, sets `cmd.Env` via
  `AgentEnvironment`, writes the bootstrap prompt once, then performs raw
  `io.Copy` in both directions. It does not parse, classify, or react to
  the agent's bytes. PTY output is relayed for the operator to read, not
  consumed as state.
- `go/pkg/agentloop/bootstrap.go::BuildBootstrapPrompt` reinforces this
  product invariant inside the prompt itself: "If you need input or are
  blocked before work.await_packet, call session.report with report_kind
  question or escalate instead of waiting silently in terminal text" and
  "This PTY supervisor will not claim, complete, release, or spoon-feed
  packet JSON for you."
- RFC 0075 § Proposal 1: "The daemon must not use pane contents as
  workflow facts."
- RFC 0075 § Domain Modeling: "Tmux pane identity is operational metadata
  attached to a supervised session, not a new state authority."
- `docs/operator/BRIEF.md` § Hazards: "Do not treat tmux panes, pane text,
  or transcripts as workflow state; RFC 0075 keeps tmux local and
  observational only."

No code path in the reviewed surfaces upgrades terminal output, pane
text, or stdout/stderr bytes into a workflow decision, verdict, or
state transition.

### 2. No transcript — PASS

- `go/pkg/mutations/claim.go::buildPacket` continues to emit, on every
  work packet: `"artifact_policy": {"publish_transcripts": false,
  "curated_artifacts_only": true}`. No change weakens that default.
- RFC 0075 § Proposal 6 enumerates an explicit allow/disallow list. The
  allow-list is metadata-shaped (tmux ids, attach command, byte-growth
  metadata, "short operator-authored stall notes", hashes). The
  disallow-list pins the no-transcript invariant: durable terminal
  transcripts, parsing pane text for decisions, publishing pane output as
  artifacts, and adding terminal output to daemon audit bodies.
- The new `session.report` MCP method (`go/pkg/mutations/lifecycle.go::
  HandleSessionReport`) caps `message` at 280 characters and constrains
  `report_kind` to `ready|heartbeat|question|escalate`, `phase` to
  `discovery|await_packet|lease_held|between_packets`, and `blocker_kind`
  to a closed `auth_prompt|model_timeout|missing_input|other` set. These
  are short, structured event payloads, not transcript carriers.
- `session.report` writes only an append-only `session.reported` event
  through `appendEvent`; it does not mutate session, lease, job, or
  artifact rows, and it does not write to the artifacts table. The
  fake-agent test asserts the event shape verbatim
  (`tests/test_mcp_fake_agent_loop_e2e.py` lines 173-191), pinning the
  shape so future changes can't quietly grow it.
- `docs/SPEC.md` § Evidence redaction default-deny rule continues to
  apply: any new emitted field that isn't classified `safe` is replaced
  with the redaction placeholder. The reviewed slice adds no new fields
  to evidence/status/doctor output.

### 3. Daemon/MCP capability — PASS

- `contracts/daemon_methods.json` registers `session.report` with
  `required_capability: "claim"`, `repository_scope_mode: "single_repo"`,
  `audit_class: "metadata"`, and `deprecated: false`. The mirrored Python
  contract at `src/striatum/daemon_rpc/daemon_methods.json` is updated in
  the same slice (per the change list in the work packet context).
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` row for `session.report`
  ("MCP pre-work session report" / capability `claim` / `single_repo`)
  matches the contract, with the explanatory note "RFC 0075 structured
  ready/heartbeat/question/escalate event path; no terminal text
  authority". The matrix preserves the rule from `AGENTS.md` that new RPC
  methods must update this file and the guardrail tests.
- `go/pkg/mcp/http.go` and `tools.go` route `tools/call` through
  `Service.ToolsCall`, which constructs an `rpc.Envelope` and calls
  `s.RPC.HandleWithoutHandshake(ctx, envelope, "mcp")`. The same
  capability gate, repository-scope check, and audit emission applied to
  Unix-socket RPC therefore apply unchanged to MCP `session.report`
  calls. There is no MCP-side bypass that lets `session.report` skip
  authorization.
- The MCP HTTP transport in `go/pkg/mcp/http.go` keeps the existing
  hardening: `validateLocalRequest` refuses non-loopback `Host`, refuses
  non-loopback `Origin`, requires `Authorization: Bearer ...`, and
  responds with the same JSON-RPC denial vocabulary (`token_missing`,
  `token_malformed`, `method_unknown`, etc.) as the docs claim in
  `docs/MCP.md` § Authentication.
- `tests/test_mcp_fake_agent_loop_e2e.py::
  test_session_report_requires_claim_capability` proves the capability
  gate end-to-end: a `read`-only token does not see `session.report` in
  `tools/list` and gets `isError: true` with
  `structuredContent.error == "capability_missing"` and a `denied` row
  in the `transport="mcp"` audit log. This is the right shape for the
  RFC 0075 invariant: a pre-work agent that has no `claim` capability
  cannot emit a structured stall/escalation report to the workflow.
- The companion happy-path test
  (`test_fake_mcp_agent_completes_work_packet_loop`) calls
  `session.report` between `session.register` and `work.await_packet`,
  asserts the audit log lists it under `transport="mcp"`, and asserts
  the run still terminates with the existing
  `run_state=completed / session_state=closed / artifact_logical_name=
  handoff` invariants. The session-report event is observational; it
  does not perturb the lease, queue, job, or artifact state machine.

### 4. CLI-retirement boundary — PASS

- `contracts/daemon_methods.json::cli_routes` does **not** add a CLI
  route for `session.report`. There is no `striatum session report`
  verb. The method is intentionally MCP-only, consistent with RFC 0050
  Phase F's "no live workflow control operation requires a human or AI
  operator to invoke `striatum` CLI verbs."
- The bootstrap prompt names only MCP tools (`tools/list`,
  `work.await_packet`, `session.report`, `work.ack`,
  `work.heartbeat`); it does not instruct the agent to fall back to the
  CLI for pre-work blocked states.
- `docs/operator/plans/rfc-0050-cli-retirement-cutover.md` workstreams
  for "Cutover map of remaining workflow-control CLI verbs", "MCP
  parity for remaining live operator actions", and "Deprecate, hide,
  or delete replaced workflow-control CLI verbs" are still `open`. The
  reviewed slice does not delete or hide any of those CLI verbs, which
  matches the plan's recorded decision: "Do not delete workflow-control
  CLI verbs before MCP/UI parity exists and is covered by tests."
- The work packet `commands` block continues to surface CLI strings
  (`buildPacketCommands` in `claim.go`), but those are the *supervised
  role's* canonical command set per `docs/HOW_TO_AGENT.md` ("Use them
  verbatim. Do not derive your own."). They route to the same daemon
  authority as the MCP path; they are not a regression in the cutover
  direction.
- `docs/operator/BRIEF.md` Hazards explicitly bars premature deletion:
  "Do not delete CLI workflow-control verbs before MCP/UI parity
  exists and is covered by tests." The slice respects this.

## Observations (non-blocking)

These are advisory notes for the next RFC 0075 implementation slices.
They are not findings against the reviewed boundary surface.

1. **Liveness deadlines are still policy, not code.** RFC 0075 § Proposal
   3 lists default deadlines (discovery, await_packet, ack, heartbeat,
   idle protocol) and a sweeper. The reviewed slice ships only the
   structured-report MCP method; classifier, deadlines, and dashboard
   surfaces are open workstreams. None of those are required to land at
   this stage. Recommend that the next slice continue to keep terminal
   bytes out of the classifier inputs and emit only metadata-shaped
   events (`session.liveness_deadline_missed`,
   `lease.heartbeat_missed`).

2. **The `session.report` payload allow-list is closed today and that is
   the safer default.** `report_kind`, `phase`, and `blocker_kind` are
   each closed enums in `HandleSessionReport`. RFC 0075's open question
   ("Should `session.ready`...be separate daemon methods, or should they
   be one `session.report` method with typed payloads?") is resolved by
   this slice in favor of the typed-payload approach. That is consistent
   with the RFC 0075 contract and easier to evolve than four separate
   methods. Recommend documenting that resolution in the RFC 0075 plan's
   "Decisions Made" list when the slice is accepted.

3. **Bootstrap prompt could optionally restate the no-transcript
   contract for the agent.** The current prompt says "do not print
   capability tokens" and "stay inside the active work packet write
   scope", but does not explicitly remind the agent that pane text is
   not workflow state. The prompt is the ergonomics surface, not the
   security boundary (RFC 0075 § Proposal 5 calls this out), so this is
   strictly advisory. The daemon-side invariants remain enforced.

4. **`agent-loop` PTY relay copies stdout to the supervisor's stdout.**
   This is the operator-attached observability surface (different from
   the supervised lane in `striatum supervise start`, where stdout is
   `DEVNULL`). Nothing persists those bytes; the relay is local to the
   foreground process. No regression, but worth pinning in a future
   RFC 0075 acceptance test that the relay byte stream is never
   appended to artifacts, audits, or evidence.

## Conclusion

The slice cleanly preserves the four boundary surfaces it was reviewed
against. `session.report` is exactly the right shape: a `claim`-gated,
audited, append-only MCP event that gives a pre-work agent a structured
alternative to waiting in terminal text, without making terminal text or
pane state authoritative. The bootstrap prompt and `docs/MCP.md` Boundary
section reinforce the contract on the prompt and operator-doc sides. No
CLI route is silently reintroduced. The remaining RFC 0075 workstreams
(tmux requirement, liveness deadlines, classifier, dashboard surfaces)
are correctly marked open in the operator plan and are not implemented
in this slice.

Verdict: **accept**.
