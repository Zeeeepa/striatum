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
`transport: pipe`, `stdin_delivery: one_shot_eof`) lacks the agent-loop MCP
configuration and preserved interactive context needed for autonomous agy work.
Supervised push lanes receive one automatic claim/send at `supervise start`,
but that does not make the agy one-shot shape autonomous (see #51, #63 F5).
The agent-loop submit path landed via #51/#52 and is the viable autonomous
shape for agy.

To inspect a stall, run `conversation.show <conversation_id>` and
`supervise status <session_id>`.
