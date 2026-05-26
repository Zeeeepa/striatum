# Three-Way Conversation Operator Recipe

Use RFC 0086 conversations with MCP agent-loop participants. Self-driving
lanes run their normal agent-loop command. Single-shot lanes use the Striatum
turn-driver by declaring an adapter capability instead of relying on a shell
loop:

```json
"gemini": {
  "adapter": "process",
  "display_model": "Gemini 2.5 Pro",
  "command": ["gemini", "-m", "gemini-2.5-pro", "-p"],
  "adapter_capabilities": {"single_shot": true}
}
```

`supervise.start` resolves that capability to
`striatumd -agent-loop -turn-driver -- <lane command>`. The driver holds the
MCP token, awaits the floor, builds a topic-plus-transcript prompt for one
turn, and calls `conversation.say`. The child generator never receives work
packet JSON, leases, session ids, or capability tokens.

To inspect a stall, run `conversation.show <conversation_id>` and
`supervise status <session_id>`. A driven lane reports
`agent_loop_mode: turn_driver`; if generation fails repeatedly the floor stays
parked and the session emits an escalation report rather than sending a
diagnostic turn.
