---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# DESIGN SYNTHESIS — F42 conversation turn-driver

author: operator

Reconciles two independent design proposals into one buildable plan for a
first-class, generic Striatum turn-driver that lets a single-shot agent CLI
(today: `gemini -p`) participate autonomously in an RFC 0086 conversation with
no operator-side driver script:

- **claude_code** — `artifacts/design/claude_code/DESIGN.md`
- **codex** — `artifacts/design/codex/DESIGN.md`

The two designs agree on far more than they disagree. They differ on exactly
two load-bearing choices — **the launch surface** and **the wait primitive** —
and those two choices turn out to be *coupled*. Resolving the first determines
the second. The rest of this document records the agreement, decides the two
tensions with reasons, and gives the implementer a smallest-landable plan.

## 1. What both designs already agree on (adopt verbatim)

These are not contested; the implementer should treat them as settled:

1. **A new pure package `go/pkg/turndriver`** holds the loop state machine,
   deliberately free of exec / MCP transport / PTY concerns so it is unit-
   testable with injected fakes.
2. **The agent is a stateless per-turn content generator.** A typed seam
   (`Generator.Generate(ctx, ConversationContext) (string, error)`) takes only
   *topic + ordered transcript* and returns text. It has no field for a work
   packet, lease, session, or token.
3. **The driver is the autonomous, capability-holding MCP client.** It performs
   `conversation.say`; the agent never reaches the daemon.
4. **Crash-safety is inherited from RFC 0086, not reinvented.** The floor is
   durable; a turn is complete only when `conversation.say` records the turn and
   advances `floor_index` in one transaction. Therefore: crash before `Say`
   ⇒ re-await sees the same floor and regenerates (safe, nothing committed);
   crash after `Say` ⇒ floor advanced, next await is not-our-floor (no double-
   speak). No local dedupe/marker file — it would be less authoritative than the
   daemon and add a recovery surface.
5. **`capability_denied` / "not your turn" on `Say` is benign** (a lost ack on a
   `Say` that actually landed): treat as already-advanced, refetch state, continue
   — never force a duplicate turn.
6. **Sanitization is conservative and content-preserving:** trim whitespace,
   normalize CRLF→LF, strip ANSI escapes and control bytes except `\n`/`\t`,
   enforce a max output size, reject empty output after trimming
   (`conversation.say` already refuses an empty body). Never strip markdown;
   never interpret output as JSON, tool calls, or instructions.
7. **Generator failure / timeout / empty output does not call `Say`.** Bounded
   small-N retry; on exhaustion leave the floor parked rather than corrupt the
   transcript or auto-close. (Liveness-over-purity modes — `say-diagnostic`,
   auto-close-on-N-failures — are deferred; not the default.)
8. **Pin the model explicitly:** `gemini -m gemini-2.5-pro -p`. Preview-default
   slowness is out of scope.
9. **Genericity keys off a capability, never the string `"gemini"`.** gemini is
   the first consumer of a "single-shot / non-self-driving" contract.
10. **Tests are pure-Go with fakes; no live gemini.** Coverage: our-turn vs
    not-our-turn, generate-and-say-exactly-once, multi-turn until closed,
    sanitizer behavior, generator failure/timeout does not say, `capability_denied`
    refetch, child-env scrubbing, and prompt rendering carries no control state.
11. **No new RFC for the mechanism itself** — it is a mechanism inside an
    existing primitive. Record a `docs/DECISION_LOG.md` D-number. (See §4 for the
    two facts that decision entry must pin.)
12. **Defer to F43 / out of scope:** chat-UI rendering of conversations; any
    non-gemini consumer beyond one genericity proof; transcript
    summary/windowing for over-long prompts.

## 2. Tension #1 — launch surface (DECIDED: agent-loop supervisor mode)

**claude_code:** a *mode* of the existing agent-loop supervisor
(`supervise start` → `striatumd -agent-loop`), selected by an adapter capability
flag (`self_driving: false`). Wins attestation parity, reuses credential
resolution, generic by capability.

**codex:** a standalone `striatum conversation drive` CLI command. Keeps
arbitrary model-CLI subprocess execution out of the daemon; the daemon stays
"state owner + RPC endpoint," not a process supervisor for model CLIs.

**Decision: the primary, shipped surface is the agent-loop supervisor mode,
keyed on an adapter capability flag.** The `Loop` lives in the pure
`go/pkg/turndriver` package so a thin standalone entrypoint can wrap the *same*
loop for debugging; but the production path operators use is `supervise start`,
exactly as for claude and codex.

Reasoning:

- **Codex's central objection is already false for this codebase.** The agent-
  loop path *already* launches agent CLIs as supervised lanes (`claude -p`,
  `codex exec`); the daemon/supervisor is already a process supervisor for model
  CLIs. The turn-driver does not introduce a *new kind* of subprocess management
  — it adds a per-turn invocation inside the same supervised-lane launcher that
  already exists. So "don't expand the daemon into a process supervisor" does not
  distinguish the options here; that expansion already happened and is the
  accepted shape.
- **Attestation parity is a real, unsolved gap, and the supervisor surface closes
  it for free.** Lane attestation binds the supervised PID to the session. If the
  driver *is* the supervised process, a driven single-shot lane is attested
  exactly like a self-driving lane — no new attestation path. Codex's standalone
  verb is an *unattested* process speaking on a session's behalf, which reopens
  the attestation hole rather than closing it. (This matters: F42 lanes are
  interrogable; an unattested speaker is the weak link.)
- **"Claude and codex must keep working unchanged" is satisfied directly:** the
  same `supervise start` path simply branches on `self_driving`. Self-driving =
  agent gets credentials and is the MCP client (today's `runWithIO`). Driven =
  the loop is the MCP client and the agent is a credential-stripped generator.
- **Credential resolution is reused, not duplicated.** `ResolveMCPEndpoint` /
  `ResolveTokenMaterial` already resolve endpoint + token for the supervised
  process; the driver needs exactly these. Codex's standalone verb would
  re-derive them.

**What we keep from codex anyway:** the pure-package factoring means a thin
`striatum conversation drive` (or a `-turn-driver` debug override that forces the
mode regardless of adapter metadata) can construct the same `Loop` for local
testing or an attestation-free environment. The standalone CLI is an **optional,
deferred** entrypoint, not the primary surface — explicitly the fallback claude_code
itself named "if attestation for driven lanes proves unnecessary." We are not
betting that it is unnecessary, so it does not ship first.

## 3. Tension #2 — wait primitive (DECIDED: `work.await_packet` + `conversation.show` for closed-detection)

**claude_code:** derive "my turn" from the `conversation_message` envelope
returned by `work.await_packet` (idempotent, daemon is sole source of floor
truth); use `conversation.show` only to detect closure when we do not hold the
floor.

**codex:** poll `conversation.show` on a `--poll-interval`, compute the floor
client-side from `participants[floor_index]`; avoid `work.await_packet` because a
conversation-scoped command shouldn't handle unrelated packets and must exit
cleanly on close.

**Decision: use `work.await_packet`'s `conversation_message` envelope as the
primary turn signal, and `conversation.show` as the explicit closed-state probe.**

The key insight is that **this tension is downstream of Tension #1.** Codex's
objection to `await_packet` ("a conversation-only command shouldn't handle
unrelated packets") was premised on its standalone-verb surface. Once the surface
is the supervised lane (§2), the lane *is* a work-packet participant and
`work.await_packet` is the coherent, native primitive — the same one self-driving
lanes use. Choosing the supervisor surface makes `await_packet` the obvious
choice, and the two designs stop conflicting.

Additional reasons to prefer the envelope over client-side floor math:

- **Don't reimplement the floor.** `deliverPendingConversationTurn` derives the
  floor solely from durable `floor_index` and delivers it idempotently. Computing
  `participants[floor_index]` in the client duplicates that logic and risks drift
  if the daemon's derivation ever changes. One source of floor truth.
- **Idempotent delivery is exactly the crash-safety we want** and is already
  built; the driver just must not undo it.

**Absorb codex's legitimate concerns into the merged loop:**

- **Closed-while-not-our-floor:** if the conversation closes while another
  participant holds the floor, `await_packet` will never return a
  `conversation_message` for us → we would await forever. So the loop probes
  `conversation.show` for `state == closed` on each not-our-floor cycle and exits
  cleanly. The `Conversation` seam therefore exposes **both** `AwaitTurn` and
  `Show` (claude_code's design already did this).
- **Unknown `await_packet` blocking semantics** (both designs flagged this): the
  loop is written to tolerate either. If `await_packet` blocks until a
  deliverable exists, we block. If it returns "no work" immediately, we fall back
  to a bounded backoff governed by codex's `--poll-interval`, running the
  `conversation.show` closed-check each cycle. This is robust to whichever
  semantics the real client confirms during implementation, and is the union of
  both proposals.

Resulting loop:

```
loop:
  turn := conv.AwaitTurn(ctx)              // work.await_packet; floor-derived
  switch turn.Kind {
  case OurTurn:                            // conversation_message for us
      content := sanitize(gen.Generate(ctx, {turn.Topic, turn.Transcript}))
      if content == "" { retry/escalate; continue }
      conv.Say(ctx, turn.ConversationID, content)   // daemon advances floor
  case NotOurFloor / NoWork:
      if conv.Show(ctx, convID).Closed { return nil }
      backoff(pollInterval)                // only if AwaitTurn returned immediately
  case Closed:
      return nil
  }
```

The driver acts on the `conversation_id` from the envelope (multiple open
conversations: `deliverPendingConversationTurn` returns the first where we hold
the floor — never assume a single conversation).

## 4. The spoon-feeding hazard — the enforceable line (merged, all three guards)

Both designs converge here; we adopt the **union** of their mechanical guards,
because each is a separate, test-backed barrier and the hazard is the single most
likely future regression.

The distinction that holds (interrogated, not merely restated): the prohibited
pattern feeds **workflow-control state** (work packets, leases, capability
tokens, ack/complete/block envelopes) to an agent that *is capable* of being an
autonomous MCP client, replacing its control loop and defeating attestation. The
turn-driver feeds **conversation content only** (topic + transcript) to an agent
that is *structurally incapable* of self-driving, while the driver itself is the
autonomous, attested MCP client. **The boundary is: which process holds the
capability token? The driver does; the agent provably does not.**

Three mechanical, tested enforcements (no comments-as-policy):

1. **Credential-stripped child environment.** The exec generator launches the
   agent with a deny-list scrub of every credential the self-driving path injects:
   `STRIATUM_MCP_URL`, `STRIATUM_MCP_TOKEN`, `STRIATUM_MCP_TOKEN_FILE`,
   `STRIATUM_DAEMON_SOCKET`, `STRIATUM_REPOSITORY_ID`, `STRIATUM_SESSION_ID`,
   `STRIATUM_RUN_ID` (union of both designs' lists). The scrub **defaults closed**
   for `STRIATUM_*`. A unit test asserts the child env contains none of these.
   The agent cannot reach the daemon even if it tried.
2. **The generator input type cannot carry control state.** `Generate` accepts a
   typed `ConversationContext` (`Topic`, `Transcript` only). Widening it to carry
   packet JSON would be a visible, test-breaking change, not a quiet slide. The
   self-driving *prompt builder* and the driven *content builder* are separate
   functions with disjoint inputs.
3. **Generator output is never routed into generic MCP calls.** The only daemon
   mutation after generation is `conversation.say(body)`. Output is never parsed
   as JSON/tool-calls/instructions.

CLI help and package doc comments state plainly: this path is for single-shot
content generators only; it is **not** a work-packet proxy for agents that can
operate as autonomous MCP clients.

## 5. Failure, lease, and auth handling (supervisor-mode specifics)

Because the shipped surface is a supervised lane holding a work lease (§2), we
take claude_code's lease-aware handling over codex's exit-non-zero (which fit its
standalone surface):

- **Heartbeat the lane lease while awaiting and generating.** `gemini -m
  gemini-2.5-pro` can be slow; a long generation must not let the lease lapse.
- **On persistent generation failure** (retries exhausted): do not silently close
  or send junk. Call `session.report` (escalation) and `work.block`, surfacing the
  stall to the operator, and leave the floor parked. An operator recovers or
  closes.
- **On auth expiry:** re-resolve token material from the same files `agentloop`
  uses and re-await. Conversation mutations are floor-gated, not lease-gated, but
  the lane session must stay alive for attestation/recovery.

## 6. Smallest landable scope

**Lands first — the testable core (no live gemini, no MCP, no exec):**
`go/pkg/turndriver` with `Loop`, the `Conversation` (`AwaitTurn`/`Say`/`Show`)
and `Generator` seams, the `ConversationContext`/`Turn`/`State` types, the
sanitizer, and the retry/exit/escalate policy — all driven by injected fakes.
Full unit coverage per §1.10.

**Lands second — wiring:**
- Real `Generator`: exec the pinned agent command via `append-arg` prompt
  transport (codex's default; `stdin` transport deferred), with the
  `contentOnlyEnv` scrub + credential-strip test (§4.1).
- Real `Conversation`: MCP/RPC client over `ResolveMCPEndpoint` +
  `ResolveTokenMaterial`, implementing `AwaitTurn` (work.await_packet) / `Say`
  (conversation.say) / `Show` (conversation.show).
- Adapter `self_driving` capability flag + branch selection in the agent-loop
  supervisor (default self-driving; `-turn-driver` debug override). gemini's
  adapter sets `self_driving: false`.
- Docs: rewrite the conversation operator recipe to use the supervised driven-
  lane mode for non-self-driving lanes; update gemini agent guidance to describe
  it as a single-shot content generator driven by Striatum; **delete the
  `/tmp/gemini-driver.sh` recipe**.

**Decision record (no full RFC).** The `docs/DECISION_LOG.md` entry must pin the
two product commitments this synthesis introduces, because they are the load-
bearing surface decisions a future reader will lean on:
  (a) the adapter `self_driving` (single-shot) capability flag and its semantics
      — selection keys off the capability, never the model name; and
  (b) the credential-strip child-process contract (the §4 boundary).
Promote to RFC 0087 only if review judges the child-process security contract or
the adapter-capability surface needs fuller product review.

**Live verification (operator, post-merge):** one multi-party conversation in
which the gemini participant is driven by the supervised turn-driver — not a
shell script — and completes its turns autonomously.

## 7. Mapping back to the two designs (what was taken from where)

- **Surface = supervisor mode, adapter flag** → claude_code §2.1 (chosen);
  codex's standalone verb retained as a deferred debug entrypoint enabled by the
  shared pure package.
- **Wait = `work.await_packet` envelope + `conversation.show` closed-probe** →
  claude_code §2.3/§5; codex's `--poll-interval` adopted as the backoff knob for
  the not-our-floor / immediate-return case; codex's client-side floor math
  rejected (don't duplicate the daemon's derivation).
- **Spoon-feeding guards = union of both** → claude_code §3 (credential-strip +
  typed input) plus codex's "output never routed into generic MCP calls" and
  default-closed scrub list.
- **Failure/lease handling = claude_code §2.5** (escalate via session.report +
  work.block, heartbeat during generation), chosen over codex's exit-non-zero
  because the shipped surface holds a lease.
- **Sanitizer, crash-safety, no-RFC, deferrals, test matrix** → both designs
  agreed; adopted as-is.
