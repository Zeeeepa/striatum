# Three-Way Conversation Operator Recipe

Use RFC 0086 conversations with MCP agent-loop participants. Self-driving
lanes run their normal agent-loop command. Per RFC 0088, the `gemini_cli`
lane (and its `single_shot` turn-driver) is retired in favour of an `agy`
(Antigravity) lane:

```json
"agy": {
  "adapter": "process",
  "display_model": "Antigravity",
  "command": ["agy", "--print"],
  "capabilities": ["write"]
}
```

The supervised one-shot `agy --print` shape mirrors the prior gemini lane
shape; once the RFC 0088 P2/P3 agent-loop substrate lands, the same lane is
expected to migrate to a long-lived interactive PTY (`agy -i`) with
agent-loop capability declared via `adapter_capabilities.agent_loop: true`.

To inspect a stall, run `conversation.show <conversation_id>` and
`supervise status <session_id>`.
