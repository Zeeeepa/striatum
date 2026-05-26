# TASK — F42: autonomous conversation participation (turn-driver)

Reference: `docs/TODO.md` F42; RFC 0086 (multi-party conversation, v2.5.0);
memory `project_rfc0086_multiparty_conversation`.

## Problem

RFC 0086 gave Striatum a symmetric N-party conversation primitive
(`conversation.{open,say,close,list,show}`). Every participant runs the same
loop:

```
work.await_packet  ->  receive a floor-derived `conversation_message`
                   ->  generate a contribution
                   ->  conversation.say
                   ->  repeat until the conversation closes
```

`claude -p` (Claude Code) and `codex exec` run a **persistent agentic
tool-loop**, so they naturally keep cycling await -> say -> await across rounds
and self-drive the conversation to completion.

`gemini -p` (gemini-cli, currently v0.42.0) runs in **non-interactive,
single-shot** mode: it executes one prompt, may call MCP tools, then exits. In
the live 3-way run (2026-05-26) the gemini lane **exited the loop early** —
printing "conversation done" while the conversation was still open — and could
not durably hold a multi-round await -> say -> await loop. Its turns had to be
driven by an operator-side shell loop (`/tmp/gemini-driver.sh`): poll
`conversation.show`, detect `floor == gemini`, invoke `gemini -p` **once** to
generate a single contribution from the topic + transcript, then call
`conversation.say`. That hack proved the fix but lives in `/tmp` and is
gemini-specific.

## Goal

Provide a **first-class, generic Striatum turn-driver** so that an agent CLI
that cannot self-drive a stateful loop (today: gemini; tomorrow: any
single-shot or non-agentic model endpoint) can still participate **autonomously**
in a conversation, with **no operator-side driver script**. Claude and codex
must keep working unchanged.

The driver owns the loop; the agent is used as a **stateless per-turn content
generator**: given the topic and running transcript, produce this turn's
contribution. Striatum holds the conversation capability and performs
`conversation.say`.

## Key design tension (the panel MUST resolve this)

`AGENTS.md` / the operator brief carry an explicit hazard:

> Do not write proxy wrappers that poll the daemon and spoon-feed JSON to
> agents. Agents must operate as autonomous MCP clients.

A turn-driver for a single-shot CLI is, mechanically, a wrapper that watches
daemon state and feeds the agent its turn. The design must articulate *why this
is not the prohibited pattern* and keep that distinction enforceable. The
distinction we believe holds — interrogate it, do not just restate it:

- The prohibited pattern feeds **workflow-control packet JSON** to an agent that
  *is* capable of being an autonomous MCP client, replacing the agent's own
  control loop and defeating attestation/authority boundaries.
- A turn-driver feeds **conversation content only** (topic + transcript) to an
  agent that is *structurally incapable* of self-driving (single-shot), and the
  **driver itself is the autonomous MCP client** holding the session's
  capability and calling `conversation.say`. The agent never sees or forges
  workflow-control state.

The design must say where that line lives in code and how a future reader is
prevented from sliding the turn-driver back into a packet-spoon-feeding proxy.

## Constraints

- The daemon is the sole writer (D094 / RFC 0043). Use daemon MCP/RPC only:
  `conversation.show` and/or the `work.await_packet` `conversation_message`
  envelope to detect the floor; `conversation.say` to speak. Never touch
  Postgres or `.striatum/` as state.
- Generic across adapters — not a gemini-only command. gemini is the first
  consumer, but the mechanism keys off "this CLI is single-shot", not the
  string "gemini".
- Deterministic and crash-safe, consistent with RFC 0086's floor-derived,
  idempotent delivery: re-entry after a crash must not double-speak or stall the
  round-robin.
- Handle, explicitly: it is not our floor (wait), conversation closed (exit
  cleanly), agent invocation fails/times out/empty output, and lease/auth
  expiry.
- Pin the per-turn model explicitly (e.g. `gemini -m gemini-2.5-pro -p`); the
  preview-default slowness is already understood and out of scope.

## Definition of done

1. A first-class Striatum turn-driver (a `striatum`/`striatumd` command or
   subcommand — the design picks the exact surface and justifies it) that drives
   a single-shot agent CLI through a full conversation autonomously.
2. Tests covering the loop logic: floor detection, our-turn vs not-our-turn,
   content capture/sanitization, conversation-closed exit, and agent-failure
   handling. Pure-Go unit tests for the loop; the agent invocation is seam-tested
   (injected/fake content generator), not dependent on a live gemini.
3. The `/tmp/gemini-driver.sh` operator hack is obsoleted: the documented recipe
   for a multi-agent conversation uses the new command for non-self-driving
   lanes.
4. Docs: update the conversation operator recipe and the gemini agent guidance;
   record the decision (DECISION_LOG D-number; RFC 0087 if the panel judges the
   surface warrants one).
5. Live verification (operator, post-merge): a conversation in which the gemini
   participant is driven by the new command — not a shell script — and completes
   its turns autonomously.

Keep the change reviewable and the smallest scope that lands the autonomous
loop. Defer the chat-UI rendering of conversations (that is F43) and any
non-gemini consumer beyond proving genericity.
