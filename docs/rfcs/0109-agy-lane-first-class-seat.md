# RFC 0109: Make the agy lane a first-class supervised seat — and stop deferring it

Status: proposed
Date: 2026-06-02
author: proposer-claude-opus-4-8-001
Context: RFC 0088 (agent-loop PTY launcher + per-adapter submit drivers), RFC 0089 (lane health), RFC 0096 (lane trust boundary), RFC 0101 (robust autonomous execution), RFC 0103 §W2 (production hardening — "every declared adapter holds a multi-turn seat"), RFC 0106 (workflow-shape support tiers); GH #95, #85, #76, #139, #70; the floor-dogfood finding that neither codex nor agy could survive the W3 restart leg.

## Problem

This RFC has two halves, and the second is the load-bearing one.

### A. The concrete defect: agy is not a viable supervised lane

Striatum declares three adapters — `claude`, `codex`, `agy` (gemini-cli) — but
only `claude` and `codex` actually hold a supervised, multi-turn seat. The `agy`
seat has never worked end-to-end as a supervised lane, across four standing
defects:

- **#95 — the seat collapses after one turn.** The agy agent-loop submit-driver
  re-registers a *new* session on its second turn instead of re-entering its
  existing one, and the duplicate is **unattested**. Any multi-turn shape
  (interrogating review, synthesis, revision) breaks: the lane that produced the
  artifact is gone by the time it is asked a follow-up, and the duplicate cannot
  publish under the role byline. This is the single defect that makes agy unusable
  for anything but a one-shot.
- **#85 — it stalls in MCP discovery before claiming.** The supervised agy lane
  can idle past its deadline probing for MCP tools, never reaching `work.claim`.
- **#76 — it blocks on an interactive feedback/trust prompt.** Like claude's #101
  and codex's nag, agy gemini-cli stops on an interactive CLI prompt that a
  supervised lane has no human to answer; it must be suppressed by env/config.
- **#139 — net effect: 3-lane panels silently collapse to 2.** On a fresh repo a
  declared 3-lane design panel (claude + codex + agy) degrades to 2 lanes because
  the agy seat trust-gates and multi-turn-crashes out. The workflow *says* three;
  the run *delivers* two; nothing fails — the gap is absorbed in silence.

There is also a cross-cutting transport defect this RFC surfaces from the RFC 0103
floor dogfood (2026-06-02): **a lane that reads a static MCP HTTP endpoint from a
config file cannot survive a daemon restart.** The daemon's MCP HTTP port is
random per start, but `codex` (`~/.codex/config.toml`) and `agy`
(`.gemini/settings.json`, the #70 ephemeral bearer file) pin a fixed
`http://127.0.0.1:<port>/mcp`. After any restart that endpoint is stale and the
lane's tools point at a dead port. Only `claude` survives, because it re-dials the
**stable unix socket** (`%t/striatum/daemon-go.sock`) on its next poll. This is
why the W3 restart-survival floor dogfood had to use two claude sessions: **codex
and agy are not restart-robust seats**, and a seat that cannot survive RFC 0103's
own fault class is not production-grade.

### B. The meta-defect: "two seats suffice, defer the third"

Every umbrella that touches agy reasons the same way: *the headline goal needs
only two seats (claude + codex), both of which hold, so the agy seat is the most
deferrable workstream.* RFC 0103 §W2 states this in those words; this RFC's author
repeated it while sequencing W2 last and then not reaching it.

The reasoning is individually correct and collectively corrosive. Because the
floor only ever needs *two* working seats, the *third* is always the cut line.
agy has been "the deferrable one" across RFC 0088, 0096, 0101, and 0103 — four
umbrellas — and has never been fixed, because at no point did its brokenness
**block** anything. A cost that never blocks never gets paid, and never even gets
counted: #139 is the only issue that names the collapse, and it names it as a
tolerated degradation, not a failed acceptance.

The deferral is the bug. Until the agy seat is **counted as friction whether or
not it blocks**, it will be deferred indefinitely by locally-valid reasoning, one
umbrella at a time. This RFC exists to make that deferral visible and to give the
seat a gate that does not evaporate the moment two other seats happen to work.

**"Two is one and one is none; bring three."** The "two seats suffice" reasoning
quietly treats two as a comfortable floor. It is not — it is a single point of
failure away from a one-seat system. A seat is *exactly* the kind of thing that
breaks on a schedule outside our control: an upstream CLI version bump, a changed
trust prompt, a config-format shift (this is precisely why P3 makes the seat a
standing CI gate). The day a claude or codex update breaks one seat, a "two-seat"
panel **is** a one-seat panel — and the interrogating/design panels whose entire
value is *divergent* reviewers degrade to a single voice with no quorum, exactly
the #139 collapse arriving from the other direction. Two is not redundancy; two is
the fragile minimum that *looks* like redundancy until the first failure. Three is
the resilience floor, and keeping agy broken pins the whole system permanently at
the fragile two while reporting "we have two, we're fine."

## Proposal

### P1 — Fix the four defects (the engineering)

- **#95:** make the agy agent-loop submit-driver **re-enter the same session**
  across turns instead of re-registering (`go/pkg/agentloop/` submit drivers).
  The session that produced the artifact must be the one a follow-up turn drives,
  and it must stay attested. This is the keystone — #85/#76 are moot if the seat
  dies after turn one.
- **#76:** suppress the gemini-cli trust/feedback prompt by env or generated
  config the way claude (#101) and codex suppress theirs, in `mcpconfig.go` /
  the supervised-env path.
- **#85:** bound the MCP-discovery probe so the agy lane fails fast (or proceeds)
  rather than idling past its deadline.
- **Transport (cross-cutting):** give config-file lanes a **stable MCP endpoint
  across restarts** — either a fixed/operator-pinned MCP HTTP port, or a
  unix-socket MCP transport for codex/agy equivalent to claude's, or a
  daemon-startup rewrite of the lanes' config files to the live endpoint. Without
  this, agy (and codex) remain non-restart-robust regardless of P1's other fixes.
  Pairs with RFC 0103 W3 / #141.

### P2 — Make the seat a counted tier (the anti-deferral mechanism)

Fixing the code is necessary but does not stop the *next* deferral. Bind the agy
seat to **RFC 0106's workflow-shape support tiers** as a first-class, named tier
with an explicit status (`supported` / `degraded` / `unsupported`) that a run
**surfaces, not absorbs**:

- A workflow that declares an `agy` lane while the agy seat is `degraded`/
  `unsupported` must emit an **operator-visible warning at `run prepare`** naming
  the gap ("declared 3 lanes; agy seat is degraded (#95) — this run will deliver
  2"), instead of silently collapsing (#139). Degradation becomes a recorded
  event, not a shrug.
- The seat's tier is asserted by a **standing CI gate** (P3), so "agy is broken"
  is a red build, not tribal knowledge that resets every umbrella.

### P3 — Acceptance gate that does not evaporate

The RFC 0101 Layer-2 adapter-conformance fixture runs an **`agy` lane through a
two-turn `claim → publish → claim` cycle against the *installed* CLI** in CI (the
harness today drives an in-process fake agent, not the real CLI — this is a new
installed-CLI conformance path, shared with the codex seat). A version bump or
config drift that breaks the agy seat **fails CI**, not a live panel three weeks
later. The gate is per-seat, so two passing seats can never green-light a third
broken one.

## Acceptance

- **[hermetic gate]** the installed-CLI conformance fixture runs `agy` through a
  two-turn `claim → publish → claim` and asserts the **same** attested session
  drives both turns (closes #95); a fixture asserts the trust/feedback prompt is
  suppressed (#76) and the MCP-discovery probe is bounded (#85).
- **[hermetic gate]** `run prepare` on a workflow declaring an `agy` lane while
  the seat tier is `degraded`/`unsupported` emits the named warning (closes the
  silent-collapse half of #139).
- **[live-corroborated]** a 3-lane (claude + codex + agy) interrogating panel run
  completes with the agy lane holding its seat across a `needs_revision` cycle —
  the inverse of #139, proven once.
- **[live-corroborated, restart]** the agy (and codex) lane survives a
  `systemctl restart striatumd` mid-run (the RFC 0103 W3 fault class) once the
  transport fix lands — otherwise the seat is explicitly tiered non-restart-robust
  and that tier is surfaced, not hidden.

## Non-goals

- Not a new adapter. This hardens the *declared* agy/gemini-cli seat, not a
  fourth runtime.
- Not forcing three lanes on every workflow. Two-lane shapes stay valid; the point
  is that a workflow which *declares* three gets three or a **surfaced** reason it
  did not.
- Not a rewrite of the agent-loop. P1 is the residual tail of RFC 0088/0096; this
  RFC consolidates it and adds the anti-deferral gate so the tail stops growing.

## Relationship to prior RFCs

- **RFC 0088/0089** — agy's submit-driver + lane-health substrate; P1 is their
  unfinished agy-specific tail.
- **RFC 0096** — lane trust boundary; #70 (.gemini bearer) is W1, this is the
  multi-turn seat.
- **RFC 0103 §W2** — named the agy seat *and* named it most-deferrable; this RFC
  is W2 lifted out so it stops being the perennial cut line, plus the transport
  finding from RFC 0103's own floor dogfood.
- **RFC 0106** — workflow-shape support tiers; P2 binds the agy seat to that
  taxonomy so the degradation is counted.
- **RFC 0101** — the Layer-2 conformance harness P3 extends to the installed CLI.
