# RFC 0086: Multi-party conversation on the MCP agent-loop

Status: proposed
Date: 2026-05-26
Author: proposer-claude-opus-4-7-001
Context: generalizes [`RFC 0082`](0082-interrogation-sessions.md) (interrogation =
the 1→1 asymmetric special case) to a symmetric **N-party** live conversation;
reuses the RFC 0081 dialogue trajectory and the RFC 0084 chat UI. Goal: three
frontier models (claude, codex, gemini) holding a live 3-way conversation in
reasonable time on the **same harness — the MCP agent-loop**.

## Problem

There is no live conversation primitive. `work.await_packet` returns only
`work_packet | interrogation_question | none`. The "Conversation" workflow type
is a `workflowgenerate` shape that emits *sequential fresh `draft` jobs* — not a
live, interactive, multi-party exchange. Interrogation is the only live
model-to-model path and it is **1→1 asymmetric** (interrogator asks, target
answers). A 3-way conversation needs N live participants that each see the
running transcript and take turns addressing the group.

## Construct

A **conversation** is N (≥2) live participant sessions, a topic, a turn-ordered
shared transcript, and round-robin floor control, bounded by `max_rounds`.

- `conversation.open(participant_session_ids[], topic, max_rounds)` →
  `conversation_id`. Records participants + speaking order; delivers the first
  turn to `participants[0]`.
- `conversation.say(conversation_id, session_id, body)` — the current
  floor-holder posts a turn. Validates it is the caller's turn; records the turn
  on the message bus; advances the floor to the next participant (wrapping
  increments the round counter); delivers a `conversation_message` to the next
  participant. At `round_count == max_rounds` the conversation auto-closes.
- `conversation.close(conversation_id, session_id)` — any participant ends it
  early; closes the conversation and releases the participants' live window.
- Reads: `conversation.list(run_id)`, `conversation.show(conversation_id)`.

## Delivery (generalize RFC 0082)

`work.await_packet` gains a fourth envelope type, `conversation_message`,
carrying the conversation_id, the running transcript, and "it is your turn".
This reuses the session-addressed message-bus delivery RFC 0082 added — the
floor-holder's await loop receives the next turn, exactly as an interrogation
target receives a question. Preference order: `interrogation_question` (a
direct peer question) > `conversation_message` (your group turn) > `work_packet`
> `none`.

## Floor control

**Round-robin** (`participants[(floor+1) % N]`). Simplest model that is actually
interactive and bounded; a moderator/nominator variant (a coordinator session
picks the next speaker) is a follow-up. Each turn delivers the running
transcript so every model reasons over the full thread.

## Liveness

All participants stay live for the conversation's duration — the same
context-preservation window RFC 0082 §5 gives an interrogable session, generalized
to "in an open conversation". A participant is a valid recipient while it is
`active` and the conversation is `open` (reuses the RFC 0084 D141 liveness
widening). The agent-loop keeps each `claude -p` / `codex exec` / `gemini -p`
session alive across turns.

## Persistence + view

Each turn is a curated record on the message bus (D028 — authored text only,
never provider stdout/stderr), surfaced in the RFC 0081 `dialogue` trajectory
(`message_kind = conversation_turn`). The RFC 0084 chat UI renders a
conversation as chat (speaker = participant session/lane), reachable read-only
over `tailscale serve` (RFC 0085). So a 3-way conversation is viewable as chat
from any tailnet device.

## Models are explicit (per-lane)

Each lane pins its model in the workflow config — no silent CLI default. The
slowness incident was the gemini CLI defaulting to a capacity-capped *preview*
model (`gemini-3-flash-preview`, `MODEL_CAPACITY_EXHAUSTED`); pinning
`gemini-2.5-pro` (GA) returns in ~9s on Ultra. Lane commands carry the model
flag: `claude --model opus`, `codex exec -m gpt-5.5`, `gemini -m gemini-2.5-pro -p`.
(Follow-up: the agent-loop launcher should map a lane's `model` field to each
adapter's flag generically, so the model is config-driven rather than buried in
`command`.)

## Storage (no owner-table migration)

Mirror the RFC 0082 migration exactly: a **plain new `striatumd.conversations`
table** (no foreign keys to owner-held `repositories`/`runs`/`sessions`;
integrity enforced in Go), grant DML to `striatumd_rw`, and **turns reuse
`queue_messages`** (`kind='agent_message'`, `payload_json.conversation_id`,
`payload_json.turn`). The runtime role can apply it; no daemon crash-loop.

## Reasonable time

Per-turn = one model response (~9–30s for claude/codex/`gemini-2.5-pro`) + a
sub-second MCP round-trip. A 3-participant round ≈ 30–90s; a 3-round
conversation ≈ a few minutes. The earlier minutes-of-backoff was the preview
model, not the harness.

## Decision

- **D144** — accept the multi-party conversation primitive: symmetric N-party,
  round-robin, agent-loop-native, message-bus-backed (plain new table), persisted
  in the dialogue trajectory and viewable in the chat UI; interrogation remains
  the 1→1 special case. Models pinned per lane.
