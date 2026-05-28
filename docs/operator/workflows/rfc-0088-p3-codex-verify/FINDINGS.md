# Codex agent_loop live-verify findings (2026-05-28)

Attempted: launch `codex --dangerously-bypass-approvals-and-sandbox` as a
daemon-owned interactive PTY `agent_loop` lane, verify it claims a packet via
MCP and publishes a `VERIFY.md` artifact (mirror of `rfc-0088-p1-verify/`).

## Verified working

- **Agent-loop wrap.** `striatumd -agent-loop -- codex …` launches; the inner
  codex child runs `codex … -c mcp_servers.striatum.url="<live-endpoint>"`
  with `mcpconfig.go`'s P3 codex case injecting the rotating URL fresh at
  launch.
- **STRIATUM_MCP_TOKEN literal env.** After the bootstrap.go fix
  (commit `14c7580`), the codex child has `STRIATUM_MCP_TOKEN` populated with
  the literal bearer (codex reads it via `bearer_token_env_var =
  "STRIATUM_MCP_TOKEN"` in `~/.codex/config.toml`).
- **Attestation.** The owned-PTY codex session is attested (`lane_attestation:
  attested`); supervisor row binds pid + command snapshot.
- **MCP startup.** The post-fix run no longer shows the "⚠ MCP Startup
  incomplete (failed: striatum)" banner the pre-fix run did, so codex's MCP
  client successfully reaches the striatum endpoint.

## Open: codex TUI submit semantics differ from claude

The bootstrap prompt is written into codex's TUI input box (visible as
character-by-character renders in the PTY debug log when the
`STRIATUM_AGENT_LOOP_DEBUG_LOG` hook is enabled), but the 750ms-delayed CR
(`agentLoopSubmitDelay` + `agentLoopSubmitSequence`) that submits in claude
does **not** submit in codex's TUI — codex sits with the bootstrap typed but
never processed, heartbeat frozen at launch time.

This is the per-adapter submit-driver work the RFC's P3 anticipates: each
TUI has its own submit semantics, and the universal `\r` after 750ms only
works for claude. Codex may need a different key-sequence (e.g., a second
Enter, bracketed-paste markers, or Shift+Enter), or a screen-detect step
that waits for codex's UI to be ready before submitting.

The bounded code change is shipped; closing this needs live iteration on
codex's TUI behavior (similar to the P1 claude submit work) — out of scope
for this session.
