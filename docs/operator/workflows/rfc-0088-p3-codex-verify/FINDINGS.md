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

## Resolved in current working tree: codex bootstrap delivery

The bootstrap prompt is written into codex's TUI input box (visible as
character-by-character renders in the PTY debug log when the
`STRIATUM_AGENT_LOOP_DEBUG_LOG` hook is enabled), but the 750ms-delayed CR
(`agentLoopSubmitDelay` + `agentLoopSubmitSequence`) that submits in claude
does **not** submit in codex's TUI — codex sits with the bootstrap typed but
never processed, heartbeat frozen at launch time.

The fix is to stop typing the bootstrap into codex's TUI. Codex accepts an
initial prompt argv, so `go/pkg/agentloop/loop.go` now appends the bootstrap
prompt to the prepared codex command after MCP URL injection and skips the PTY
submit write for codex only. Claude/agy keep the existing PTY submit path.
Regression coverage:

- `TestPrepareLaneCommandForBootstrapUsesCodexInitialPromptArg`
- `TestPrepareLaneCommandForBootstrapKeepsClaudePTYSubmit`
- `TestRunWithIOCodexReceivesBootstrapAsInitialPromptArg`

## 2026-05-28 second pass

After the `STRIATUM_MCP_TOKEN` literal-env fix (`14c7580`), codex's MCP
startup succeeds. After the `pty.go` `-e KEY=VAL` fix (in `5ff381e`,
unfortunately mixed into a docs-titled commit; the tmux global server now
inherits STRIATUM_* env on new-session), claude verify works end-to-end
through `pty_helper` + tmux — BUT the helper's `tmux attach-session`-as-
liveness proxy mis-reports `agent_exited` microseconds after start (attach
exits while the session continues; published artifact got `author: operator`
because supervisor went "gone" before publish), so the universal-tmux
default was reverted.

`STRIATUM_AGENT_LOOP_SUBMIT_SEQUENCE` set to `\r\r` (double-Enter) does NOT
trigger codex's TUI to submit the bootstrap either — the input box still
shows the bootstrap text being typed character-by-character per `pty.log`.
Codex's TUI submit semantics likely require: a bracketed-paste-aware submit,
a screen-detect step that waits for the TUI to be ready, OR a single-line
bootstrap (codex may treat embedded newlines as multi-line input that needs
an explicit `Ctrl+J` / different terminator). This is the iterative per-
adapter submit-driver research the RFC anticipated; it didn't yield to a
short fan-out of variants and is left as a focused follow-up.

The trajectory log default (per-supervisor `pty.log` under
`.striatum/scratch/<sup>/`) makes this iteration tractable — the operator can
launch a codex lane, set `STRIATUM_AGENT_LOOP_SUBMIT_SEQUENCE=<candidate>`
on the daemon, and `tail -f` the pty.log to see exactly what codex's TUI
renders for each variant until one submits.

## 2026-05-28 third pass

The candidate-key-sequence path is no longer needed for codex bootstrap. The
current implementation bypasses the codex TUI input editor by using codex's
documented initial prompt argument. Live verification still requires installing
or restarting a daemon with this working-tree build before launching the codex
agent-loop lane.
