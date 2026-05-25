# Agent-loop substrate validation (2026-05-25)

Phase A result — can each adapter drive a genuine headless agent-loop packet
loop (await_packet → write → artifact.publish → work.ack → work.complete)?

| Adapter | MCP connect | Tool calls | Full packet loop | Verdict |
|---|---|---|---|---|
| claude (`claude -p`, `--mcp-config`) | yes | yes | ✅ completed run_5614cb… | **reliable** |
| codex (`codex exec`, `mcp add --url --bearer-token-env-var`) | yes | yes | ✅ completed build_codex (wp_e9a2…) | **reliable** |
| gemini (`gemini -p --approval-mode yolo`, `mcp add --transport http -H`) | yes (`✓ Connected`) | partial | ❌ writes files but errors on publish/ack/complete | **unreliable** |

Findings:
- The `striatumd -agent-loop` PTY launcher does NOT submit its bootstrap prompt
  to a TUI agent (claude buffers it). Working substrate = headless `-p` / `exec`.
- gemini connects and can call tools and write files, but is far too slow: it
  spends time grepping the repo, and by the time it reaches `artifact.publish` /
  `work.ack` / `work.complete` the 120s packet lease has expired, so the
  mutations fail. Not usable as an autonomous lane today without much longer
  leases and a hardened, no-exploration tool-arg harness.
- MCP config that works for claude: `{"mcpServers":{"striatum":{"type":"http",
  "url":"<EP>","headers":{"Authorization":"Bearer <TOK>"}}}}`; codex:
  `codex mcp add striatum --url <EP> --bearer-token-env-var STRIATUM_MCP_TOKEN`;
  gemini: `gemini mcp add striatum <EP> --transport http -H "Authorization:
  Bearer <TOK>" --trust`.

Decision for the live run: claude + codex are the reliable real-model lanes
(critical path: synthesis / interrogation target / implement). gemini is at most
a best-effort 3rd design lane with a long lease and must never gate the run.
