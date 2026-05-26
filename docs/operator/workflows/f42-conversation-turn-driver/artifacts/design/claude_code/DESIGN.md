---
author: operator
kind: handoff
logical_name: claude_design
---

# F42 — Conversation turn-driver (claude_code design lane)

## 1. Problem framing

RFC 0086 made conversations symmetric: every participant runs the identical
loop — `work.await_packet` returns a floor-derived `conversation_message`, the
participant generates a contribution, calls `conversation.say`, and re-awaits
until the conversation closes. The floor signal is *derived from durable state*
(`conversations.floor_index`) by `deliverPendingConversationTurn`, so it is
idempotent and crash-safe by construction (`go/pkg/mutations/conversation.go`
lines 264–302). Nothing about the daemon side is the problem.

The problem is purely **agent-side process shape**:

- `claude -p` and `codex exec` run a *persistent agentic tool loop*. One process
  invocation keeps cycling: it calls `work.await_packet`, receives the
  `conversation_message`, calls `conversation.say`, then loops back to
  `work.await_packet` of its own accord across many rounds. The process stays
  alive between MCP tool calls and re-prompts itself until it observes the
  conversation closed. That is exactly the autonomous-MCP-client contract the
  agent-loop bootstrap prompt assumes (`go/pkg/agentloop/bootstrap.go` line 41:
  "Call tools/list first, then call work.await_packet…").
- `gemini -p` (gemini-cli v0.42.0) runs **single-shot, non-interactive**: it
  executes one prompt, may make some MCP tool calls, then exits to the shell. It
  has no internal "keep going until the work is done" loop. In the live 3-way
  run (2026-05-26) the gemini lane printed "conversation done" and exited while
  the conversation was still open. It *structurally cannot* hold a multi-round
  `await -> say -> await` loop: after the first turn the process is simply gone.

The proven workaround was `/tmp/gemini-driver.sh`: poll `conversation.show`,
detect `floor == gemini`, invoke `gemini -p` once to turn the topic + transcript
into a single contribution, then call `conversation.say`. It works, but it is
gemini-specific, lives in `/tmp`, is untested, and is not crash-safe.

We need a **first-class, generic Striatum turn-driver**: an in-process loop that
owns the `await -> generate -> say` cycle, uses the single-shot agent as a
*stateless per-turn content generator*, and holds the conversation capability
itself. Claude and codex must keep working unchanged.

## 2. Proposed approach

### 2.1 Surface: a turn-driver *mode* of the existing agent-loop supervisor

I recommend **not** adding a new operator-facing verb. Instead, the turn-driver
is a launch mode inside `go/pkg/agentloop`, selected by an **adapter
capability flag** (`self_driving: false`, equivalently `single_shot: true`) on
the lane's adapter descriptor. The same launch path operators already use —
`supervise start` → `striatumd -agent-loop` — chooses between two code paths:

- `self_driving == true` (claude, codex; the default): today's behavior. The
  supervisor PTY-launches the CLI with the bootstrap prompt and the MCP
  credentials in its environment, and the **agent is the autonomous MCP
  client** (`runWithIO`, `go/pkg/agentloop/loop.go`).
- `self_driving == false` (gemini, and any future single-shot endpoint): the
  supervisor runs the **turn-driver loop**. Now the *supervisor process is the
  autonomous MCP client* — it holds the session token and endpoint, calls
  `work.await_packet` / `conversation.say` — and invokes the agent CLI once per
  turn purely to generate content.

Why this surface, and not a standalone `striatum conversation drive` verb (the
main alternative, see §4):

1. **Attestation parity.** Lane attestation binds the supervised process PID to
   the session (`supervise start`). If the driver *is* the supervised process,
   the single-shot lane is attested exactly like a self-driving lane, with no
   new attestation path. A separate client verb would be an unattested process
   speaking on a session's behalf.
2. **Reuse of credential resolution.** `ResolveMCPEndpoint` and
   `ResolveTokenMaterial` (`loop.go` lines 25–33) already resolve the endpoint
   and capability token for the supervised process. The driver needs exactly
   these as the MCP client; nothing new to wire.
3. **Genericity by capability, not by name.** Selection keys off the adapter's
   `self_driving` flag, never the string `"gemini"`. gemini is just the first
   adapter that sets it. (TASK constraint: "keys off 'this CLI is single-shot',
   not the string 'gemini'".)

A debug/test override flag `-turn-driver` may force the mode regardless of
adapter metadata, but the *production* selector is the adapter flag.

### 2.2 Where the loop lives in code

A new pure package `go/pkg/turndriver` holds the state machine, deliberately
free of exec, MCP transport, and PTY concerns so it is unit-testable:

```
type ConversationContext struct {            // the ONLY thing the agent sees
        Topic      string
        Transcript []TranscriptTurn          // author_session_id + body, ordered
}

type Generator interface {                   // stateless per-turn content gen
        Generate(ctx context.Context, c ConversationContext) (string, error)
}

type Conversation interface {                // the autonomous MCP client seam
        AwaitTurn(ctx context.Context) (Turn, error)   // wraps work.await_packet
        Say(ctx context.Context, conversationID, body string) error
        Show(ctx context.Context, conversationID string) (State, error)
}

func Loop(ctx context.Context, conv Conversation, gen Generator, opts Options) error
```

`Loop` is the testable core; the supervisor injects a real `Conversation`
(MCP/RPC client over the resolved endpoint+token) and a real `Generator` (exec
the pinned agent command). Tests inject fakes for both.

### 2.3 The loop, step by step

```
loop:
  turn := conv.AwaitTurn(ctx)               // daemon's floor-derived signal
  switch turn.Kind {
  case ConversationMessage where your_turn: // we hold the floor
      content := gen.Generate(ctx, {turn.Topic, turn.Transcript})
      content  = sanitize(content)
      if content == "":  retry/escalate (see §2.5); continue
      conv.Say(ctx, turn.ConversationID, content)   // daemon advances floor
  case NotOurFloor / NoWork:
      // re-await (await blocks, or backoff if it returns immediately)
  case ConversationClosed:
      return nil                            // clean exit
  }
```

- **Floor detection is delegated to the daemon.** The driver derives "it is my
  turn" *only* from the `conversation_message` envelope returned by
  `work.await_packet` (which `deliverPendingConversationTurn` produces solely
  from durable `floor_index`). The driver never decides the floor from a local
  poll. This makes the daemon the single source of floor truth and matches
  RFC 0086's idempotent delivery.
- **Speaking** is `conversation.say` with the sanitized body. The daemon records
  the turn and advances the floor atomically in one transaction
  (`HandleConversationSay`).

### 2.4 Crash-safety / idempotency

RFC 0086 already guarantees the hard part; the driver must simply not undo it:

- The "my turn" signal is re-derivable: a driver that crashes after
  `AwaitTurn` but before `Say` re-awaits on restart and **sees the same turn
  again** (floor hasn't moved). It regenerates and says. No turn is lost, the
  round-robin does not stall.
- Generated content is **never persisted until `Say`**. A crash between
  `Generate` and `Say` discards the text; regeneration on restart is safe even
  though the new text differs (stateless generator, nothing committed).
- `Say` is a single atomic transaction that advances the floor. There is no
  partial-say window, so **double-speak is impossible**: once the floor has
  advanced, the next `AwaitTurn` will not return our turn. If a `Say` actually
  succeeded but the ack was lost and we retry, the daemon returns
  `capability_denied` ("not your turn") — the driver treats that as *benign,
  already advanced* and re-awaits.

### 2.5 Content capture, sanitization, failure handling

- **Pin the model.** The generator command is configured per adapter, e.g.
  `gemini -m gemini-2.5-pro -p`. Preview-default slowness is out of scope.
- **Capture** the agent's stdout; the topic + transcript are passed as the
  prompt (content only — see §3).
- **Sanitize** conservatively: strip ANSI escapes and control characters, trim
  surrounding whitespace. Do *not* strip markdown or otherwise rewrite content.
- **Reject empty** output: `conversation.say` already refuses an empty body
  (`HandleConversationSay`, line 117), so the driver must not send one. Empty,
  timeout, or non-zero exit → bounded retry (configurable, small N).
- **On persistent generation failure** (retries exhausted): the driver does not
  silently `close` the conversation or send junk. It calls `session.report`
  (escalation) and `work.block`, surfacing the stall to the operator. Leaving
  the floor parked is preferable to corrupting the transcript; an operator can
  recover or close. (Open question §6: a configurable "auto-close on N
  consecutive failures" could prevent indefinite parking — defer unless a
  participant-starvation case demands it.)
- **Lease / auth.** The driver heartbeats the work lease while awaiting and
  generating (long generations must not let the lease lapse). On auth expiry it
  re-resolves token material and re-awaits.

## 3. The spoon-feeding-hazard line — made enforceable

The hazard (AGENTS.md / operator brief): *do not write proxy wrappers that poll
the daemon and spoon-feed JSON to agents; agents must be autonomous MCP
clients.* A turn-driver is mechanically a wrapper that watches state and feeds
the agent its turn, so the distinction must be load-bearing in code, not prose.

The distinction that holds:

- **Prohibited:** feeding *workflow-control packet JSON* (work packets, leases,
  capability tokens, complete/ack/block envelopes) to an agent that *is capable*
  of being an autonomous MCP client, thereby replacing its own control loop and
  defeating attestation/authority boundaries.
- **Turn-driver:** feeding *conversation content only* (topic + transcript) to
  an agent that is *structurally incapable* of self-driving, while **the driver
  itself is the autonomous, attested MCP client** holding the session
  capability and calling `conversation.say`. The agent never sees, and cannot
  forge, any workflow-control state.

Two mechanical enforcements keep a future reader from sliding this back into a
spoon-feeding proxy — both are tested, neither is a comment:

1. **Credential-stripped child environment.** The generator launches the agent
   CLI with a `contentOnlyEnv` that *removes* every credential the autonomous
   path injects: `STRIATUM_MCP_TOKEN`, `STRIATUM_MCP_TOKEN_FILE`,
   `STRIATUM_MCP_URL`, `STRIATUM_DAEMON_SOCKET`, `STRIATUM_REPOSITORY_ID`,
   `STRIATUM_SESSION_ID`. The agent **cannot reach the daemon** even if it tried
   — it has no endpoint and no token. A unit test asserts the generator's child
   env contains none of these keys. (Contrast `AgentEnvironment` in
   `bootstrap.go`, which deliberately *adds* them for the self-driving path.)
   This is the line: self-driving = agent gets credentials; driven = agent gets
   none. Anyone trying to "let the driven agent call `conversation.say` itself"
   must re-add a token, which the test forbids.
2. **The generator input type cannot carry control state.** `Generate` accepts a
   `ConversationContext` whose only fields are `Topic` and `Transcript`. There
   is no field for a work packet, lease, session, or token. A future change that
   wanted to spoon-feed packet JSON would have to widen this type — a visible,
   reviewable, test-breaking change rather than a quiet slide. The bootstrap
   *prompt* builder (for self-driving agents) and the generator *content* builder
   are separate functions with disjoint inputs.

So the boundary is "does this process hold the capability token?" — and the
turn-driver answers that the *driver* does and the *agent* provably does not.

## 4. Alternatives considered

1. **Keep the operator shell driver (`/tmp/gemini-driver.sh`).** Rejected: this
   is exactly what F42 obsoletes — gemini-specific, untested, not crash-safe,
   lives in `/tmp`, and re-polls `conversation.show` for the floor instead of
   using the daemon's idempotent delivery.
2. **Standalone CLI verb `striatum conversation drive`.** Viable and arguably
   simpler to reason about as "just another MCP client." Rejected as the primary
   surface because it is an *unattested* process speaking for a session, and it
   duplicates endpoint/token resolution and the lane-launch path. The
   supervisor-mode approach gets attestation and credential resolution for free
   (§2.1). (If attestation for driven lanes proves unnecessary, this becomes the
   cheaper option — noted as a fallback.)
3. **Daemon-side autoplayer (the daemon generates turns itself).** Rejected: it
   couples the trusted sole-writer to agent binaries and makes the daemon author
   content on a session's behalf, which breaks the attestation model. The driver
   must be a *client* process bound to a session, not the writer.
4. **Wrap gemini in a bash `while` re-invoking `gemini -p` per turn.** That *is*
   the turn-driver, just in an untyped, untested, crash-unsafe shell. Rejected
   for the same reasons as (1).

## 5. Risks, unknowns, and what could go wrong

**Risks**

- *Mis-flagged adapter.* If a self-driving adapter is flagged `single_shot`, it
  loses its own tool use; if a single-shot adapter is left `self_driving`, we
  reproduce the original bug (early exit). Mitigation: the flag is explicit per
  adapter and covered by a test; document it in the adapter recipe.
- *Generation stall starves other participants.* A model that always returns
  empty/garbage parks the floor; other participants wait. Mitigation: bounded
  retry then escalate/block (§2.5); consider optional auto-close (§6).
- *Lease lapse during slow generation.* `gemini -m gemini-2.5-pro` can be slow.
  Mitigation: heartbeat while generating; generous, configurable per-turn
  timeout; killing a timed-out generation just discards (safe).
- *Over-aggressive sanitization* could drop legitimate content. Mitigation:
  strip only ANSI/control, never markdown; unit-test the sanitizer.
- *Multiple open conversations.* `deliverPendingConversationTurn` iterates all
  open conversations and returns the first where we hold the floor. The driver
  must act on the `conversation_id` from the envelope, not assume a single
  conversation.

**Unknowns (confirm during implementation)**

- *`work.await_packet` blocking semantics.* Does it block until a deliverable
  (including a `conversation_message`) exists, or return "no work" immediately?
  If the latter, `AwaitTurn` needs a bounded backoff poll. The loop is written
  to tolerate either, but the real client adapter depends on the answer.
- *How "closed" reaches a non-floor participant.* When the conversation closes
  while we do **not** hold the floor, `await_packet` will never return a
  `conversation_message` for us. The driver must detect closure via
  `conversation.show` (state == closed) to exit cleanly rather than awaiting
  forever. This is the key integration detail — the `Conversation` seam exposes
  `Show` precisely for the exit check.

## 6. Rollout sketch and smallest landable scope

**Lands first (the testable core):**

- `go/pkg/turndriver` with `Loop`, the `Conversation`/`Generator` seams, the
  `ConversationContext`/`Turn`/`State` types, the sanitizer, and the
  retry/exit/escalate policy — all driven by **injected fakes**. Unit tests
  cover: our-turn vs not-our-turn, content capture + sanitization, empty/error
  generation handling, conversation-closed exit, and the `capability_denied`
  re-await (benign double-say) path. No live gemini, no MCP, no exec.

**Lands second (wiring):**

- Real `Generator`: exec the pinned agent command with `contentOnlyEnv` (+ the
  credential-strip test from §3).
- Real `Conversation`: MCP/RPC client over `ResolveMCPEndpoint` +
  `ResolveTokenMaterial`, implementing `AwaitTurn`/`Say`/`Show`.
- Adapter `self_driving` flag + selection in the agent-loop supervisor (default
  self-driving; debug override `-turn-driver`).
- Docs: update the conversation operator recipe to use the new mode for
  non-self-driving lanes, update gemini agent guidance, delete the
  `/tmp/gemini-driver.sh` recipe. Record the decision (DECISION_LOG D-number;
  RFC 0087 only if the synthesizer judges the surface warrants one — I lean
  *no RFC*: this is a mechanism inside an existing primitive, not a new
  primitive).
- Deferred to F43 / out of scope: chat-UI rendering of conversations; any
  non-gemini consumer beyond one genericity proof; the optional auto-close
  policy (§5) unless a starvation case forces it.

**Smallest scope a single implementer can land:** the first bullet (the pure
`turndriver` package + fakes + tests) plus the real `Generator` and
`Conversation` adapters and the adapter-flag selector — i.e. everything except
the optional auto-close and the RFC. That delivers an autonomous gemini lane
with no shell script and a fully unit-tested loop.
