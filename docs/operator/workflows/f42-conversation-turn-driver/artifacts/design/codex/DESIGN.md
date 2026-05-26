---
author: operator
---

# F42 Turn-Driver Design

## Problem Framing

RFC 0086 made conversations symmetric at the daemon boundary: the current
floor-holder sees a `conversation_message`, contributes once with
`conversation.say`, and the daemon advances the floor. Persistent agentic CLIs
such as Claude Code and `codex exec` can keep that loop in their own process:
they retain the bootstrap instructions, call MCP tools, and wait for the next
turn until the conversation closes.

`gemini -p` is different in kind, not merely immature. In non-interactive mode
it is a single prompt-to-output process. It can produce a good turn, but after
the process exits it has no durable loop state and no reliable way to call
`work.await_packet` again. The missing piece is therefore not better prompt
wording; it is a Striatum-owned loop that treats such CLIs as stateless content
generators.

## Proposed Surface

Add a `striatum` CLI command:

```sh
striatum conversation drive \
  --repository-id "$STRIATUM_REPOSITORY_ID" \
  --session-id "$GEMINI_SESSION_ID" \
  --conversation-id "$CONVERSATION_ID" \
  --poll-interval 2s \
  --turn-timeout 10m \
  --prompt-transport append-arg \
  -- gemini -m gemini-2.5-pro -p
```

This should be a `striatum` command, not a `striatumd` mode. The daemon remains
the state owner and sole writer; the turn-driver is an operator-started client
process that owns one participant session's conversation loop. Keeping arbitrary
agent subprocess execution out of `striatumd` avoids expanding the daemon into a
process supervisor for model CLIs.

The command is generic over the generator command. Gemini is the first recipe,
but the mechanism keys off a stateless subprocess contract: render a prompt from
topic plus transcript, invoke an argv after `--`, capture one textual response,
then call `conversation.say` on behalf of the configured session.

## Code Shape

Add a small package, tentatively `go/pkg/turndriver`, with these boundaries:

- `Driver` owns the loop and has the daemon RPC client.
- `ConversationClient` exposes only `Show(ctx, conversationID)` and
  `Say(ctx, sessionID, conversationID, body)`.
- `ContentGenerator` exposes only
  `Generate(ctx, TurnPrompt) (string, error)`.
- `ExecGenerator` implements `ContentGenerator` by spawning the argv after `--`.

The existing `go/pkg/agentloop` endpoint/token resolution should be reused or
factored into a shared helper so the new command resolves the same daemon MCP
URL and bearer token material as supervised agents. Unlike `agentloop.Run`,
the driver must not pass `STRIATUM_MCP_URL`, `STRIATUM_MCP_TOKEN`,
`STRIATUM_MCP_TOKEN_FILE`, session leases, work packet JSON, or packet commands
to the generator subprocess.

The loop should use `conversation.show` as the explicit state probe:

1. Call `conversation.show`.
2. If state is `closed`, exit 0.
3. Compute the current floor from `participants[floor_index]`.
4. If the floor is not `--session-id`, sleep for `--poll-interval` and repeat.
5. If it is our floor, render `TurnPrompt` from topic, participants, turn count,
   and ordered transcript.
6. Invoke the generator with a per-turn timeout.
7. Sanitize and validate stdout.
8. Call `conversation.say`.
9. Repeat until `conversation.show` reports closed.

This reads the same durable `conversations.floor_index` state that
`deliverPendingConversationTurn` uses for `work.await_packet` delivery. The
driver does not need to consume a queue message to learn its turn; re-entry
after a crash sees the same floor until `conversation.say` succeeds.

`work.await_packet` can remain the persistent-agent path. I would not make it
the first implementation's blocking primitive for this command because the
driver is scoped to a known conversation and must exit cleanly if the
conversation closes while another participant holds the floor. `conversation.show`
gives that closed-state observation directly without requiring unrelated work
packet handling in a conversation-only command.

## Generator Contract

`append-arg` should append the rendered prompt as the final argv element. A
second `stdin` transport is useful and cheap for future generators, but the
smallest Gemini recipe only needs:

```sh
--prompt-transport append-arg -- gemini -m gemini-2.5-pro -p
```

The rendered prompt should be plain conversation content:

- topic
- current turn index
- participant ordering
- ordered transcript entries
- instruction to produce only this participant's next contribution

It should not include repository IDs, conversation IDs unless explicitly needed
for debugging, leases, packet commands, capability tokens, MCP endpoint URLs, or
raw daemon response JSON.

Stdout becomes the candidate body. Stderr should stream to the driver's stderr
or a bounded diagnostic buffer, but it must not be included in the conversation
body by default. Sanitization should:

- trim leading and trailing whitespace
- normalize CRLF to LF
- strip ANSI escape sequences and other terminal control bytes except `\n` and
  `\t`
- enforce a max output size
- reject empty output after trimming

The driver must not interpret generator output as JSON, tool calls, shell
commands, or workflow instructions. After sanitization, it is just the body for
`conversation.say`.

## Crash Safety And Idempotency

The daemon already gives the important invariant: the floor is durable and a
turn is complete only when `conversation.say` records the transcript turn and
advances `floor_index` in one transaction.

The driver should lean on that invariant:

- Crash before generator invocation: no daemon mutation; restart sees same
  floor.
- Crash during generator invocation: no daemon mutation; restart sees same
  floor.
- Crash after generator output but before `conversation.say`: no daemon
  mutation; restart generates again for the same turn.
- Crash after successful `conversation.say`: daemon has advanced the floor;
  restart observes not-our-floor or closed and does not double-speak.

If two driver processes race for the same session and conversation, both may
generate text, but only the first successful `conversation.say` can advance the
floor. The loser should treat `capability_denied` / "not your turn" as a
refetch-and-continue condition, not as evidence that a duplicate turn must be
forced.

No local dedupe file is needed in the first cut. A local file would be less
authoritative than the daemon and would introduce another recovery surface.

## Failure Handling

Generator failure, timeout, or empty sanitized output should fail the current
turn without calling `conversation.say`. The command exits non-zero after a
small bounded retry count, leaving the floor unchanged so the same command can
be restarted after the cause is fixed. This preserves transcript quality and
uses RFC 0086's idempotent floor-derived delivery for recovery.

The command can later add an explicit `--on-generator-error say-diagnostic`
mode for demos that prefer liveness over transcript purity. It should not be
the default.

Daemon auth failures should refresh endpoint/token material from the same files
used by `agentloop` and retry once. If auth still fails, exit non-zero with a
clear credential error. Conversation mutations do not need work-packet leases;
if an MCP transport reports stale lease-style errors from a compatibility path,
the driver should re-resolve credentials and refetch conversation state rather
than inventing local state.

## Spoon-Feeding Hazard Boundary

The enforceable line is the package interface:

- `Driver` may hold daemon clients, repository IDs, session IDs, conversation
  IDs, and capability tokens.
- `ContentGenerator` may receive only `TurnPrompt` and return text.
- `ExecGenerator` must scrub all `STRIATUM_MCP_*`, `STRIATUM_DAEMON_*`,
  `STRIATUM_SESSION_ID`, `STRIATUM_RUN_ID`, and token-file environment variables
  from the child environment unless a future decision explicitly opts in.
- The prompt renderer must accept a typed conversation view, not a raw
  `work.await_packet` or `conversation.show` response map.
- Generator output must never be routed into generic MCP calls. The only daemon
  mutation after generation is `conversation.say(body)`.

That makes the distinction testable. Unit tests should assert that the fake
generator never receives packet commands, leases, bearer tokens, MCP URLs, or
raw JSON fields. The CLI help and package comments should say this command is
for single-shot content generators only and is not a work-packet proxy for
agents that can operate as autonomous MCP clients.

This is not the prohibited wrapper pattern because the wrapped process is not
being asked to operate Striatum workflow control indirectly. The autonomous MCP
client is the driver itself, and the only thing delegated to the subprocess is
natural-language turn content.

## Alternatives Considered

Keep `/tmp/gemini-driver.sh`.

This proved the behavior but is not a product surface. It is gemini-specific,
not tested, not discoverable, and easy to mutate into an unsafe packet-feeding
script.

Teach `gemini -p` to self-drive with the existing agent-loop bootstrap.

This asks a single-shot process to behave like a persistent process. Prompting
can make one turn better, but it cannot keep a durable await/say loop alive
after the process exits.

Run the driver inside `striatumd`.

The daemon would then need to spawn arbitrary model CLIs, manage their
environment, stream their stderr, enforce timeouts, and carry operator-local
API credentials. That is broader than the daemon's current authority as state
owner and RPC endpoint.

Use `work.await_packet` as the only wait primitive.

That matches the persistent-agent mental model, but it makes closed-conversation
exit and unrelated packet handling awkward for a command scoped to one
conversation. `conversation.show` gives the same floor-derived state and keeps
the first driver loop small and explicit.

## Tests

Pure-Go tests should cover `go/pkg/turndriver` with fake clients and fake
generators:

- exits immediately when `conversation.show` reports `closed`
- waits without generating when the floor belongs to another participant
- generates and calls `conversation.say` exactly once when the floor is ours
- loops through multiple turns until closed
- strips ANSI/control bytes, trims whitespace, normalizes newlines, and rejects
  empty output
- generator timeout/failure does not call `conversation.say` and returns a
  useful error
- `conversation.say` "not your turn" refetches state instead of double-speaking
- child environment scrubbing removes Striatum MCP/session/token variables
- prompt rendering contains topic/transcript but no packet commands, leases,
  bearer tokens, MCP URLs, or raw daemon JSON

No test should require live Gemini. A narrow CLI test can verify argv parsing
and `--` command capture; the loop behavior belongs in package tests.

## What Could Go Wrong

The easiest regression is someone widening the generator prompt from
conversation content to raw work packet JSON. The typed `TurnPrompt` boundary
and tests above are the guardrail.

Long transcripts can exceed model prompt limits. The first cut should fail with
a clear "prompt too large" error or a conservative max prompt byte limit. A
summary/windowing strategy should be a later decision because it changes what
the participant sees.

A duplicate operator-started driver can waste model calls. The daemon prevents
double turns, but not duplicate generation. That is acceptable for F42; a later
operator UX can warn on duplicate active drivers.

Timeouts are model- and network-sensitive. The command should make
`--turn-timeout` explicit in docs and examples instead of relying on a hidden
default.

If the child environment scrub is incomplete, a single-shot CLI with MCP support
could regain workflow-control authority. The scrub list needs tests and should
default closed for Striatum-specific environment variables.

## Rollout

First landing:

1. Add `go/pkg/turndriver` with the loop, prompt renderer, sanitizer, exec
   generator, and pure-Go tests.
2. Add `striatum conversation drive` wired to daemon MCP/RPC resolution through
   the existing agent-loop endpoint/token helpers.
3. Document the Gemini command with an explicit pinned model:
   `gemini -m gemini-2.5-pro -p`.

Second landing:

1. Replace the operator recipe that references `/tmp/gemini-driver.sh`.
2. Update Gemini guidance to describe it as a single-shot content generator
   driven by Striatum.
3. Record the decision in `docs/DECISION_LOG.md`; use an RFC 0087 only if the
   selected CLI surface or child-process security contract needs fuller
   product review.
4. After merge, run one live multi-party conversation where Gemini is driven by
   `striatum conversation drive` and no operator shell loop.

The smallest implementable scope is the `conversation.show`/`conversation.say`
loop, `append-arg` prompt transport, strict environment scrubbing, sanitizer,
fake-client tests, and the Gemini recipe. Everything else can wait.
