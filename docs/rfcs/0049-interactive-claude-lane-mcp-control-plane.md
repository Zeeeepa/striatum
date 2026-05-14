# RFC 0049 — Interactive claude lane via MCP control plane

**Status:** proposed (experimental — needs spike before commitment)
**Scope:** V1.8 or V2.0 depending on Claude Code interactive-headless stability
**Closes (partially):** the economic skew between Max-plan subscription quota and the Agent SDK monthly credit

## Background

Today the claude lane wrapper (`/.striatum/bin/claude-supervised-wrapper.sh`)
spawns one `claude --print` process per work packet. The choice was made
under RFC 0010 V2 to match the "one packet = one lease" boundary cleanly,
and to avoid depending on Claude Code's stream-json multi-turn protocol
which was undocumented as of 2026-05-08.

On 2026-05-14 Anthropic published the Agent SDK plan-credit policy
(<https://support.claude.com/en/articles/15036540>). Starting 2026-06-15:

- `claude -p` (and Agent SDK calls in general) no longer count against
  Pro/Max/Team/Enterprise subscription limits.
- Instead, each user gets a separate monthly Agent SDK credit:
  Pro $20, Max 5x $100, Max 20x $200, Team Premium $100,
  Enterprise Premium $200.
- The SDK credit bills at standard API pass-through rates.

For an operator on **Max 20x** (the largest current subscription tier),
the subscription limit allows roughly two orders of magnitude more
tokens per dollar than the $200/month Agent SDK credit consumed at
API rates. Heavy striatum dogfood operators on Max plans will hit
the Agent SDK credit ceiling quickly — every supervised claude lane
packet drains it — while their subscription quota sits idle.

The way to put striatum's claude lane back on the subscription quota
is to use **interactive Claude Code** rather than `claude -p`. That
requires either:

1. **PTY + stream-json** — drive the interactive TUI through a
   pseudoterminal and parse a stream of JSON events. Protocol
   stability uncertain.
2. **PTY + MCP control plane** — start one long-lived interactive
   Claude Code session per lane, give it a bootstrap prompt that
   tells it to connect to the striatum MCP server, and deliver work
   packets as MCP tool calls. The agent runs ordinary tools (claude
   doesn't have to know anything about JSON-packet decoding); the
   MCP server is the control plane.

Option 2 is the substance of this RFC. It sidesteps stream-json
entirely and reuses Striatum's existing MCP harness (RFC 0036 V1
landed in dogfood-038; RFC 0040 V1 expanded the mutation surface).

## Goals

- Move the claude lane onto interactive Claude Code so packet
  execution draws from the subscription quota (Max 20x: ~100× more
  tokens per dollar than the SDK credit after 2026-06-15).
- Reuse the existing MCP daemon surface for packet delivery and
  state mutation; no new protocol invention.
- Preserve the RFC 0010 V2 lease-boundary contract: one packet =
  one lease boundary, even though the process now spans many
  packets.
- Keep stream-json off the critical path (operators should not have
  to depend on the protocol's undocumented stability).

## Non-goals

- Codex and gemini lanes (their pricing models and CLI shapes
  differ; address separately if their `--print`-equivalent has the
  same economic skew).
- Removing the `claude --print` wrapper. It stays as a fallback for
  operators without interactive subscriptions (API-key users) and
  for environments where MCP plumbing isn't available.
- Solving the long-context-growth problem if a single claude
  session accumulates context indefinitely; see §Open questions.

## Design sketch

### Boot sequence

1. Supervisor (Go or Python, RFC 0039 V1.7 PTY path) allocates a
   PTY for the new lane session.
2. Supervisor spawns `claude` (interactive mode, no `-p`) attached
   to the PTY.
3. Supervisor writes a single bootstrap prompt to the PTY master:

   ```
   You are a Striatum lane agent. The work assigned to you is
   delivered via the Striatum MCP server at $STRIATUM_MCP_SOCKET.
   Connect to that server and call striatum.work.await_packet().
   When a packet arrives, perform the work it describes using the
   tools the MCP server exposes; then call striatum.work.complete()
   with the lease_id. Repeat indefinitely. Do not exit unless the
   server returns striatum.lane.shutdown.
   ```

4. Claude Code's interactive loop reads the bootstrap prompt,
   acknowledges, and enters its normal MCP tool-use loop.
5. Supervisor enters monitoring state — heartbeat reads from MCP
   tool-call events on the daemon side, NOT from PTY output
   parsing.

### Packet delivery

- The daemon's existing `claim_next` returns packets via the CLI
  surface today. The new control-plane introduces an MCP-side
  variant: `striatum.work.await_packet()` blocks (or long-polls)
  until a packet is available for the calling lane session.
- The MCP server pushes the packet payload back through the
  tool-call response.
- The agent invokes the existing MCP mutation tools
  (`striatum.artifact.publish`, `striatum.work.complete`,
  `striatum.review.submit`, etc.) to advance state. These tools
  already exist (RFC 0040 V1).

### Lease boundary preservation

- The daemon assigns a fresh `lease_id` for each packet it returns
  from `await_packet`. The agent attaches that `lease_id` to every
  subsequent mutation tool call until it completes the job.
- "One packet = one lease" still holds at the database level. The
  difference is only that one OS process can handle many leases
  sequentially.

### `fresh_session_required` interaction

Workflows that declare `fresh_session_required: true` on a job
require a session that has never received a packet on the run.
Under the long-lived model:

- Either: the lane spawns a *new* interactive claude process for
  every fresh-required packet (negates the subscription savings
  for those specific jobs), OR
- The session-fresh contract is reinterpreted to "this packet must
  go to a session whose claude context has been cleared" — which
  Claude Code supports via `/clear` or similar, but treating an
  in-process `/clear` as fresh would require operator review of
  the trust model (was the prior context actually gone?).

Default proposal: keep the spawn-new-process path for
`fresh_session_required`. Operators trading off cost for
freshness can declare `lanes.claude_code.fresh_strategy:
"process_restart" | "context_clear"`.

### Heartbeat

- Today: supervised wrapper writes a per-packet "progress" line
  to stdin (RFC 0009 §heartbeat).
- Proposed: each MCP tool call from the agent IS a heartbeat — the
  daemon updates the supervisor pointer's `last_heartbeat_at` on
  every received tool call from the session.
- Idle agent waiting in `await_packet`: that long-poll itself is a
  heartbeat (the supervisor sees the open connection).

## Risks / open questions

1. **Does Claude Code's interactive mode honor a single bootstrap
   prompt and then enter a stable MCP-driven loop?** No public
   documentation as of 2026-05-14 says yes; experiment required.
   Risk: claude might exit on first idle period, or require
   periodic keep-alive prompts, or behave differently in headless
   PTY vs real-terminal environments.
2. **Context window growth.** A long-lived claude session
   accumulates context. After N packets the model may slow or
   refuse. Mitigation: per-packet context reset via `/clear` (if
   Claude Code supports a tool to do it), or supervisor-driven
   restart every K packets. Either way, the cost model says
   restarts within a billing day are still cheaper than `claude -p`
   on the SDK track for Max 20x users.
3. **Subscription billing semantics for headless-PTY interactive
   Claude Code.** The article says interactive Claude Code
   continues to use subscription limits "in the terminal or IDE."
   A PTY-attached supervised process IS a terminal, technically.
   But Anthropic's intent may be to gate the SDK-credit policy on
   "interactive human-driven sessions." If Anthropic decides
   PTY-supervised sessions look like the SDK use case after all,
   the economic argument disappears. Mitigation: ask Anthropic
   support to clarify before committing to V2.0.
4. **MCP server's `await_packet` semantics.** Today the MCP
   harness exposes a request/response tool model. Long-polling or
   server-sent events for `await_packet` either need a new MCP
   primitive or a short-poll-loop workaround that the agent
   handles. Mitigation: spec the long-poll shape in V1 of this
   RFC after experimenting.
5. **Per-`fresh_session_required` cost.** If most workflows
   declare fresh-session reviewers, the savings shrink because
   those packets still spawn fresh processes. Mitigation: measure
   actual fresh-required fraction in dogfood-049+ data (looks like
   ~40-50% of jobs based on historical workflow.json shape).

## Acceptance criteria for V1 (post-spike)

- Spike experiment: one operator runs a 3-job workflow on a
  single long-lived claude session connected via MCP. Verify:
  - claude does not exit between packets.
  - MCP tool calls arrive on the daemon side and update
    supervisor heartbeats.
  - Lease boundaries are recorded correctly per packet.
  - Total token usage of the spike attributes to subscription
    quota (verifiable via `/cost` or whatever Claude Code surfaces
    by 2026-06-15).
- If the spike passes, V1 implementation lands:
  - Supervisor PTY launch of `claude` (no `-p`).
  - Bootstrap prompt template.
  - MCP `await_packet` long-poll tool.
  - Heartbeat-via-tool-call wiring on the daemon side.
  - Operator flag `lanes.claude_code.lifecycle:
    "per_packet" | "long_lived"` defaulting to `per_packet` until
    operator opts in.
  - Documentation: `docs/HOW_TO_AGENT.md` notes the lifecycle
    flag and the billing-track implication.

## Phasing

- **Phase 0 (this RFC):** design + spike approval.
- **Phase A (V1.7 or V1.8):** spike — one operator runs the
  manual long-lived experiment, reports findings, updates this
  RFC with empirical answers to the open questions.
- **Phase B (V1.9 or V2.0):** wire the supervisor + MCP
  long-poll if Phase A succeeds. Ship behind the
  `lifecycle: long_lived` opt-in.
- **Phase C (V2.0+):** flip default to `long_lived` for the
  claude lane after a billing-cycle of operator validation. The
  per-packet wrapper stays as fallback.

## Provenance

- 2026-05-14 conversation in the striatum operator session
  triggered by Anthropic's Agent SDK plan-credit policy
  announcement.
- The operator on Max 20x flagged the ~100x token-per-dollar gap
  between subscription quota and SDK credit.
- Prior wrapper design (RFC 0009, RFC 0010 V2) noted the
  stream-json instability and chose `claude --print` deliberately.
  This RFC revisits that decision now that MCP-as-control-plane
  is plausible.

## Open questions (block Phase A)

1. Will Anthropic confirm that PTY-supervised interactive Claude
   Code sessions count against subscription quota the same way
   terminal/IDE sessions do?
2. Does Claude Code expose an MCP server-config flag that the
   supervisor can inject before the interactive process starts
   (so the agent connects to the right socket without a
   per-session config file edit)?
3. Does Claude Code's interactive loop support reading the
   bootstrap prompt from stdin and then waiting indefinitely on
   MCP tool calls, without printing periodic prompts?
4. Is there a documented or experimental way to clear the
   in-process context (`/clear` or analogous) that the agent can
   self-invoke between packets without restarting the process?
