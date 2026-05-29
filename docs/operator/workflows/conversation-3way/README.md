# Three-Way Conversation Operator Recipe

Use RFC 0086 conversations with MCP agent-loop participants. Self-driving
lanes run their normal agent-loop command. Per RFC 0088, the `gemini_cli`
lane (and its `single_shot` turn-driver) is retired in favour of an `agy`
(Antigravity) agent-loop lane:

```json
"agy": {
  "adapter": "process",
  "display_model": "Antigravity",
  "command": ["agy", "--dangerously-skip-permissions"],
  "adapter_capabilities": {"agent_loop": true},
  "capabilities": ["write"]
}
```

`agy` must run as an agent-loop lane (`adapter_capabilities.agent_loop:
true`). The historical one-shot pipe shape (`agy … --print` with
`transport: pipe`, `stdin_delivery: one_shot_eof`) does **not** self-claim:
the one-shot pipe path gets no auto-MCP config and no auto-delivery, so agy
launches, reads nothing on stdin, runs `agy --print ""`, and exits without
claiming (see #51, #63 F5). The agent-loop submit path landed via #51/#52 and
is the only viable autonomous shape for agy.

To inspect a stall, run `conversation.show <conversation_id>` and
`supervise status <session_id>`.
