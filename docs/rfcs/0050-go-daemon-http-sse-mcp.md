# RFC 0050 — Native Go Daemon HTTP/SSE MCP and Agent Loop

**Status:** accepted (pending implementation)
**Scope:** Architecture alignment / Python deprecation

## Background

Under the original MCP integration (RFC 0036 / RFC 0040), MCP support was provided via a Python-based `stdio` wrapper (`src/striatum/mcp.py`) which proxied requests to the daemon's Unix socket. For interactive agents (like Claude Code) inside the `claude` lane (RFC 0049), a supervisor script was proposed to either proxy JSON directly to `stdin` or use the Python wrapper.

As Striatum moves to a single native Go binary (deprecating the Python CLI and wrappers), the `-agent-loop` was initially ported as a JSON `stdin` proxy. However, this strips the agent of its autonomy as an MCP client. Furthermore, the standard `stdio` MCP transport requires the agent to spawn the MCP server as a subprocess, which adds unnecessary indirection when the daemon is already running.

## Goals

- **Deprecate Python MCP**: Completely remove `src/striatum/mcp.py`.
- **Native Daemon SSE**: Build an HTTP/SSE MCP server natively into the Go `striatumd` daemon. This allows agents to connect directly to the running daemon without spawning proxy processes.
- **Autonomous Agents**: Refactor the `-agent-loop` supervisor to act strictly as a PTY manager. It will spawn the agent process, inject a bootstrap prompt containing the daemon's HTTP/SSE endpoint, and let the agent natively use its own MCP client to discover tools (like `work.await_packet`), execute work, and report completions.

## Design Sketch

### 1. Go Daemon HTTP/SSE Server
The Go daemon (`striatumd`) will expose an HTTP server (e.g., on a configured local port or a specific socket).
- **Endpoint**: `/mcp/sse`
- **Protocol**: Standard MCP over HTTP/SSE.
- **Behavior**: It will natively serve `tools/list` (using `mcp.VisibleTools()`) and `tools/call`. Incoming MCP tool calls will be mapped to daemon JSON-RPC methods, authenticated via the standard capability tokens, and executed in-process.

### 2. `-agent-loop` Redesign
The Go `-agent-loop` subcommand (or the generic lane supervisor) will:
1. Allocate a PTY for the agent.
2. Spawn the agent (e.g., `claude`).
3. Send an initial bootstrap prompt:
   ```
   You are a Striatum lane agent. Connect to the MCP server at http://localhost:<daemon-port>/mcp/sse. Call 'work.await_packet' to register for work.
   ```
4. Monitor the agent process until termination.

It will **no longer** long-poll `await_packet` itself or pipe raw JSON.

## Acceptance Criteria

- `src/striatum/mcp.py` is deleted.
- The Go daemon exposes a functional `/mcp/sse` endpoint.
- An interactive agent (e.g., Claude Code) can be spawned in `-agent-loop`, connect to the SSE endpoint, and successfully complete a work packet via MCP `tools/call`.
