# Design — F42 autonomous conversation turn-driver (one of two parallel lanes)

Read the full task in
`docs/operator/workflows/f42-conversation-turn-driver/TASK.md`. Also read the
RFC 0086 conversation mutations at `go/pkg/mutations/conversation.go` and the
agent-loop bootstrap at `go/pkg/agentloop/` so your design is grounded in the
real tool surface (`conversation.show`, the `work.await_packet`
`conversation_message` envelope, `conversation.say`).

You are one of two design lanes running in parallel. Do not coordinate with the
other lane — independent perspectives are the point.

Produce a single `DESIGN.md` inside your lane's allowed write path. Cover:

- Problem framing in your own words (why gemini single-shot can't self-drive the
  await -> say -> await loop; why claude/codex can).
- Proposed approach: the exact command/surface (`striatum ...` verb vs
  `striatumd` mode), where the loop lives in code, how it detects the floor, how
  it invokes the agent as a stateless per-turn content generator, how it
  captures/sanitizes output, and how it stays crash-safe/idempotent with RFC
  0086's floor-derived delivery.
- The spoon-feeding-hazard distinction (TASK.md "Key design tension"): where the
  line lives in code and how a future reader is kept from sliding the turn-driver
  back into a packet-spoon-feeding proxy. Do not just restate the claim — make
  it enforceable.
- Two or three alternatives considered and why this one wins.
- Risks, unknowns, and an explicit "what could go wrong" section.
- A short rollout sketch (what lands first, what lands second), and the smallest
  scope a single implementer can land.

Keep the document focused. Do not write code. Do not edit files outside your
lane's allowed paths.

When the design is complete, emit the `submit-handoff` packet from your work
packet's `commands` block.
