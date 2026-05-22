---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0075-tmux-observable-mcp-agent-sessions.md", "docs/operator/plans/rfc-0075-tmux-observable-mcp-agent-sessions.md", "docs/operator/plans/rfc-0050-cli-retirement-cutover.md", "docs/rfcs/0050-go-daemon-http-sse-mcp.md", "docs/MCP.md", "docs/HOW_TO_AGENT.md", "docs/SPEC.md", "contracts/daemon_methods.json", "go/pkg/agentloop/loop.go", "go/pkg/mcp/http.go", "go/pkg/mcp/tools.go", "docs/operator/workflows/rfc-0075-and-mcp-cutover/workflow.json", "docs/operator/workflows/rfc-0075-and-mcp-cutover/prompts/design_liveness_contract.md"]
---

# RFC 0075 Liveness Contract

author: designer-claude-code-001

## Purpose And Scope

This contract turns RFC 0075 into a bounded, landable implementation plan
for the daemon-side liveness/session story that must accompany the RFC 0050
MCP cutover. It is **design only**: no source changes are made in this job.
The implementer slice will follow the smallest-first cut described in
[`First Landable Slice`](#first-landable-slice).

The contract preserves two non-negotiable boundaries from RFC 0075 and
`docs/SPEC.md`:

1. **No terminal authority.** Tmux panes, PTY bytes, model output, and
   wrapper FIFOs are operational metadata only. Workflow state continues
   to come from daemon-owned PostgreSQL rows, daemon RPC calls, and
   published artifacts.
2. **No durable transcripts.** Operational byte counts, mtimes, and pane
   identifiers are allowed in scratch; full pane text, model thinking,
   and JSON-RPC payload bodies remain off durable storage by default.

Authority words below ("must", "must not") describe the product contract
the implementer should encode; they are not yet wired into source.

## Glossary For This Contract

- **Live interactive session**: a session whose lane adapter spawns a
  long-running agent that talks MCP back to the daemon (today: any
  `process` adapter that the supervised wrapper drives). Non-interactive
  one-shots (unit fixtures, headless CI adapters) are out of scope unless
  they opt in.
- **Supervisor liveness**: the OS-level fact that the supervised
  process (and, after this work, its tmux session) still exists.
- **Protocol liveness**: the fact that the agent has made an MCP request
  recently enough for its current phase.
- **Lease liveness**: the fact that a held `work.heartbeat` was refreshed
  before its expiry threshold.
- **Stall**: a deadline was missed for a phase that the agent should have
  advanced through MCP. Stalls are classified, not narrated.

## Three-Signal Liveness Model

The daemon already tracks lease heartbeat. After this work it must also
attribute every live interactive session a triple:

| Signal | True when | Source of truth | RFC 0075 ref |
|---|---|---|---|
| `supervisor_liveness` | tmux session and the supervised child process both exist | supervisor probe (existing PTY supervision + `tmux has-session`) | §2 supervisor heartbeat |
| `protocol_liveness` | a daemon-attributable MCP request from this session arrived inside the current phase's deadline | daemon HTTP MCP handler timestamps per `session_id` | §2 protocol heartbeat |
| `lease_liveness` | the session's currently-held lease has a `work.heartbeat` inside its expiry threshold | existing leases/work.heartbeat | §2 lease heartbeat |

Liveness is reported as an explicit triple plus a derived `stall_class`.
A pane that is alive while the model is silent is **supervisor-live,
protocol-dead**; that is the failure mode RFC 0075 is closing.

## First Landable Slice

The smallest first slice the implementer should ship in
`docs/operator/artifacts/rfc-0075-and-mcp-cutover/build/` covers only what
is necessary to make "supervisor-live, protocol-dead" a *visible*,
*classified* state. Anything beyond is deferred to follow-up slices.

Included in slice 1:

- Per-session MCP-activity timestamps recorded inside the existing daemon
  HTTP MCP handler (`go/pkg/mcp/http.go`) at the point `tools/list` and
  `tools/call` enter daemon RPC. No new MCP transport.
- A single new daemon method, `session.report`, that accepts a typed
  payload (`ready` / `heartbeat` / `question` / `escalate`) and routes to
  one row write. See [`Pre-Work Session Method`](#pre-work-session-method).
- Tmux session metadata persisted at supervise.start: `tmux_session_name`,
  `tmux_window_id` (when known), and a rendered `attach_command`. Tmux
  remains optional in slice 1 — if `tmux` is absent on the host, the
  session is recorded with `tmux_session_name = null` and a documented
  remediation string. **Fail-closed for live interactive lanes** is the
  next slice, gated on the cutover map's compatibility ledger.
- A deadline sweeper that runs from the existing daemon scheduling tick
  (no new goroutine plumbing required) and emits **metadata-only**
  domain events for missed deadlines.
- Status surface fields described in [`Status Surfaces`](#status-surfaces),
  rendered by extending the existing `status`, `dashboard`, and
  `supervise.status` handlers — no new top-level commands.
- The tests in [`Test Plan`](#test-plan).

Deferred to later slices (explicitly out of slice 1):

- Requiring tmux for live interactive lanes (fail-closed remediation).
- Multi-pane / nested-shell representation.
- Operator-paused deadlines while inspecting a pane.
- Removing the agent-loop bootstrap prompt as the sole instruction
  channel — the prompt remains the ergonomic contract surface; this slice
  only adds the structured fallback path.
- Equivalent local PTY multiplexers other than tmux.

## Pre-Work Session Method

RFC 0075 §4 leaves "one typed method vs four" as an open question. This
contract picks **one typed method**, `session.report`, for the following
reasons:

- All four payload types share the same authorization shape
  (`required_capability: "claim"`, `repository_scope_mode: single_repo`)
  and the same audit attribution.
- All four route to the same daemon row write (one
  `session_report` row keyed by `(session_id, report_kind, created_at)`).
- A typed payload keeps the MCP `tools/list` surface small and aligned
  with the agent-loop bootstrap prompt — one tool name for the agent to
  remember.
- The four-method alternative bakes payload shape into the method name
  and complicates capability rotation if we later add `pause` or
  `attention` payloads.

The typed payload, in MCP `tools/call` arguments form:

```json
{
  "session_id": "sess_...",
  "repository_id": "repo_...",
  "report_kind": "ready" | "heartbeat" | "question" | "escalate",
  "phase": "discovery" | "await_packet" | "lease_held" | "between_packets",
  "message": "<optional short operator-facing note, <= 280 chars>",
  "blocker_kind": "auth_prompt" | "model_timeout" | "missing_input" | "other" | null
}
```

Tool surface (added to `contracts/daemon_methods.json`):

```json
{
  "method": "session.report",
  "required_capability": "claim",
  "repository_scope_mode": "single_repo",
  "params_schema_version": 1,
  "audit_class": "metadata",
  "min_envelope": 1,
  "deprecated": false
}
```

`ready` and `heartbeat` payloads write `last_session_ready_at` and
`last_session_heartbeat_at` and bump `last_mcp_request_at`. `question` and
`escalate` payloads additionally create an escalation row visible to
`escalation.list` so the operator current-brief and dashboard see it
without polling pane text.

The agent-loop bootstrap prompt (`go/pkg/agentloop/bootstrap.go`) is
updated in this slice to call out `session.report` as the structured path
for "I'm blocked before I can call `work.await_packet`."

## Persisted Metadata (Minimal Set)

The daemon must persist exactly the metadata below per live interactive
session. Anything not listed is out of scope for this contract; in
particular, no pane bytes, no JSON-RPC bodies, no model output.

### Session row additions

| Field | Type | Source | Notes |
|---|---|---|---|
| `tmux_session_name` | text nullable | supervise.start | null when tmux absent or non-interactive lane |
| `tmux_window_id` | text nullable | supervise.start when available | optional |
| `attach_command` | text nullable | derived from `tmux_session_name` | rendered at write time, not at read time, so operators can copy a stable string |
| `pty_pid` | integer nullable | supervise.start | already implicit in supervisor metadata; surface it here for the status payload |
| `last_supervisor_probe_at` | timestamptz | supervisor sweeper | when the daemon last confirmed supervisor liveness |
| `last_mcp_request_at` | timestamptz | MCP HTTP handler | any authenticated MCP request from this session |
| `last_tools_list_at` | timestamptz | MCP HTTP handler | first signal of MCP discovery success |
| `last_await_packet_at` | timestamptz | `work.await_packet` handler | bumped on entry, not on return, so a hung await is visible |
| `last_ack_at` | timestamptz | `work.ack` handler | already implicit via leases; surface here for stall classification |
| `last_heartbeat_at` | timestamptz | `work.heartbeat` handler | mirror of lease heartbeat for the session-level view |
| `last_session_ready_at` | timestamptz | `session.report` `ready` | only when an agent volunteered readiness |
| `last_session_heartbeat_at` | timestamptz | `session.report` `heartbeat` | pre-packet heartbeat path |
| `stall_class` | text nullable | sweeper-derived | enum in [`Stall Classes`](#stall-classes), recomputed at read time and on event |

### Session activity table (optional, slice 1 keeps it inline on session row)

Slice 1 keeps these as columns on the session row. If a future slice needs
historical activity (for dashboards or post-mortems), promote them to a
`session_activity` append-only table; do not introduce that table now.

### Operational scratch

Byte-growth / mtime metadata for the tmux pane may continue to live in
`.striatum/`. **It must not be referenced by daemon decision logic or
emitted into audit bodies.** The sweeper consumes only PostgreSQL columns.

## Stall Classes

The sweeper assigns at most one stall class per session. Order of
evaluation (first match wins):

| Order | Stall class | Deadline name | Set when |
|---|---|---|---|
| 1 | `agent_mcp_discovery_stall` | `discovery_deadline` | supervisor-live, no `last_tools_list_at` yet, deadline elapsed since session create |
| 2 | `agent_await_packet_stall` | `await_packet_deadline` | discovery complete, no `last_await_packet_at` within deadline |
| 3 | `agent_ack_stall` | `ack_deadline` | packet delivered (`last_ack_at` < packet delivery time), `ack_deadline` elapsed since delivery |
| 4 | `agent_lease_heartbeat_stall` | `lease_heartbeat_deadline` | active lease with `last_heartbeat_at` older than the lease's `heartbeat_after_seconds` + slack |
| 5 | `agent_protocol_idle_stall` | `protocol_idle_deadline` | supervisor-live, no MCP request inside `protocol_idle_deadline` for the current phase |
| 6 | `agent_supervisor_dead` | n/a | supervisor probe says the process or tmux session is gone |
| 0 | `null` | n/a | no deadline missed; session is `ok` |

The sweeper emits a single metadata-only domain event per transition into
a stall class:

- `session.liveness_deadline_missed{class, deadline_name, session_id}`
- `session.liveness_recovered{previous_class, session_id}`

Existing `lease.heartbeat_missed` continues to fire; the new
`session.liveness_deadline_missed` event with class
`agent_lease_heartbeat_stall` provides the session-level mirror so the
dashboard and operator-current-brief do not have to join lease and
session rows.

## Deadline Defaults And Configuration

Defaults are chosen to tolerate slow model startup while catching silent
stalls quickly. All values are in seconds.

| Deadline | Default | Rationale |
|---|---|---|
| `discovery_deadline` | 60 | Process launch + MCP `tools/list` should complete inside a minute on a healthy host; cold model startup is allowed up to 60s before stall. |
| `await_packet_deadline` | 90 | After discovery, the agent should be calling `work.await_packet`; 90s allows a slow first model turn. |
| `ack_deadline` | 60 | Once a packet is delivered, the agent should ack quickly; 60s catches silent post-delivery model freezes. |
| `lease_heartbeat_deadline` | derived | Existing lease `heartbeat_after_seconds` + a 30s slack; the new event mirrors the existing miss. |
| `protocol_idle_deadline` | 300 | Inside a held lease, an agent may be writing files; 5 minutes is the longest gap allowed without *any* MCP request before classifying as idle. |

Configuration locations (in order of precedence):

1. **Workflow lane constraints** (per-lane overrides in `workflow.json`).
   Slow lanes (large models, network-bound providers) bump these.
2. **Workflow-level liveness policy** block (optional top-level key in
   `workflow.json`).
3. **Daemon configuration defaults** (in the daemon config file or
   `STRIATUM_DAEMON_LIVENESS_*` env vars).

Daemon code carries the hard-coded fallbacks above only as a last
resort. The implementer slice should land the daemon-config and lane
override paths; the workflow-level block can come in a follow-up.

Open question deferred to the implementer slice: whether the workflow
override block should be schema-versioned alongside lanes (likely yes)
or held as free-form keys (no). Recommend schema-versioned.

## Status Surfaces

Three operator surfaces must show liveness in slice 1. None of them
parses pane text.

### 1. `status` payload (daemon RPC, MCP, CLI)

Extend the per-session block of the `status` response with:

```json
{
  "session_id": "sess_...",
  "liveness": {
    "supervisor": "live" | "dead" | "unknown",
    "protocol": "live" | "dead" | "unknown",
    "lease": "live" | "no_lease" | "missed",
    "stall_class": "agent_await_packet_stall" | null,
    "since": "2026-05-21T08:07:14Z"
  },
  "tmux": {
    "session_name": "striatum-sess-bcfd444f",
    "attach_command": "tmux attach -t striatum-sess-bcfd444f",
    "window_id": "@7"
  },
  "deadlines": {
    "discovery": {"deadline_seconds": 60, "elapsed_seconds": 42},
    "await_packet": {"deadline_seconds": 90, "elapsed_seconds": null}
  }
}
```

### 2. `dashboard` and `dashboard.all`

Add an `attention` row per session whose `stall_class != null`. The row
text is the stall class plus the rendered `attach_command`; **never the
pane contents**. Sessions without a stall are summarized as
"N sessions live, M with active leases" — counts only.

### 3. Operator current-brief (`docs/operator/BRIEF.md` consumers)

The brief generator already pulls daemon RPC. After this slice it
includes a "Live sessions needing attention" section listing only
sessions with non-null `stall_class`, the attach command, and the most
recent `session.report` message (string field on the session row from
the latest `question`/`escalate` payload). Operators can read this
section to triage without attaching.

### 4. `supervise.status` and `supervise.list`

Mirror the `liveness` and `tmux` blocks above so the existing supervisor
view surfaces stalls without joining to `status`.

## Test Plan

Tests must distinguish all three liveness signals and the structured-vs-
terminal-text question paths. They run against the fake MCP agent
harness already used by `tests/test_mcp_fake_agent_loop_e2e.py`.

### A. Discovery stall

- **Setup**: register a session, do not connect the fake agent to MCP.
- **Expectation**: after `discovery_deadline + slack`,
  `stall_class == agent_mcp_discovery_stall`; `status` payload shows it;
  `session.liveness_deadline_missed` event fired exactly once.

### B. Await-packet stall

- **Setup**: fake agent calls `tools/list`, then sleeps without calling
  `work.await_packet`.
- **Expectation**: `stall_class == agent_await_packet_stall` after
  `await_packet_deadline`; supervisor liveness stays `live`.

### C. Ack stall

- **Setup**: fake agent calls `work.await_packet`, receives a packet,
  does not call `work.ack`.
- **Expectation**: `stall_class == agent_ack_stall` after `ack_deadline`
  from packet delivery time; lease has not expired yet.

### D. Lease heartbeat stall

- **Setup**: fake agent calls `work.ack`, then stops calling
  `work.heartbeat`. Process remains running.
- **Expectation**: `stall_class == agent_lease_heartbeat_stall` once
  `last_heartbeat_at` is older than
  `heartbeat_after_seconds + lease_heartbeat_deadline_slack`. Existing
  `lease.heartbeat_missed` continues to fire; the new session-level
  event also fires.

### E. Structured question/escalation

- **Setup**: fake agent calls
  `tools/call(session.report, {"report_kind": "question", "message": "need auth token"})`.
- **Expectation**: an escalation row appears under `escalation.list`;
  the operator-current-brief generator includes the question without
  any pane text scraping; `stall_class` remains `null` (the agent
  asked through the structured path).

### F. Terminal-text-only stall

- **Setup**: fake agent prints a "waiting for input" string to the PTY
  but does not call `session.report` or any other MCP method.
- **Expectation**: pane text is **not** consumed by the daemon; the
  session classifies as `agent_protocol_idle_stall` (or
  `agent_await_packet_stall`, depending on phase) when the appropriate
  deadline elapses. No escalation appears.

### G. Recovery

- **Setup**: trigger any stall above, then have the agent resume by
  calling the appropriate MCP method.
- **Expectation**: `stall_class` returns to `null`;
  `session.liveness_recovered` event fires exactly once.

### H. Tmux absence (slice 1: degraded, not failed)

- **Setup**: run a live interactive session on a host without tmux on
  PATH.
- **Expectation**: session row records `tmux_session_name = null`;
  `attach_command = null`; `doctor` includes a remediation string;
  liveness still classifies normally because supervisor/MCP/lease
  signals do not depend on tmux. The fail-closed flip lands in a later
  slice.

### I. No-transcript guardrail

- **Setup**: capture the bytes the daemon writes to PostgreSQL and to
  audit during scenarios A–F.
- **Expectation**: no field contains pane contents, JSON-RPC request
  bodies, or model output. `session.report` messages are bounded to
  280 characters and never include pane text.

## Boundary Preservations

The implementer slice must keep the following invariants:

- The daemon **never** parses pane output to derive workflow state.
  Liveness uses MCP-handler timestamps and supervisor probes.
- The daemon **never** publishes pane output as an artifact or stores
  it as audit body content. Stall events carry only `session_id`,
  `class`, `deadline_name`, and timestamps.
- The agent-loop bootstrap prompt remains an ergonomics surface; the
  daemon's authorization, deadlines, and stall classification are the
  product contract. A prompt change can never weaken the contract.
- `session.report` requires the same capability and repository scope as
  `work.heartbeat`. There is no anonymous escalation path.
- Non-interactive lanes and headless CI adapters opt **out** by lane
  declaration; opting out disables stall classification for those
  sessions, not the underlying daemon RPC authority.

## Open Questions Deferred To Implementation

These are intentionally not resolved in this contract; the implementer
slice should decide and record in `docs/DECISION_LOG.md`:

- Whether `attach_command` should be rendered at write time (preferred)
  or at read time. Slice 1 picks write-time for operator copy stability.
- The exact slack value added to `heartbeat_after_seconds` for the
  session-level lease stall. Recommend 30s.
- Whether `session.report` `message` should be truncated server-side
  (recommended) or rejected when over 280 chars. Recommend truncate +
  flag.
- Whether the deadline sweeper should run on the existing recovery
  sweeper tick or on its own short-interval tick. Recommend the
  existing tick for slice 1.
