# Striatum MCP

Status: native Go daemon HTTP/SSE MCP is the production tool surface
Updated: 2026-05-20

## Overview

Striatum's MCP surface is served by the local Go `striatumd` daemon. The
transport is HTTP plus Server-Sent Events on loopback only. Tool discovery
comes from the daemon method registry, is filtered by the caller's capability
token, and every `tools/call` re-enters daemon RPC with normal authorization,
request logging, and audit behavior.

The retired Python `striatum.mcp` stdio wrapper is no longer part of the
product surface. Agents should connect to the running daemon instead of
spawning a proxy process.

## Endpoint

`striatumd` starts the MCP HTTP listener by default on an ephemeral loopback
port. The daemon writes the active SSE endpoint to the owner-only runtime file:

```text
$STRIATUM_DAEMON_RUNTIME_DIR/mcp-http-endpoint
```

If `STRIATUM_DAEMON_RUNTIME_DIR` is unset, the runtime directory follows the
same daemon token/socket rules documented in `docs/POSTGRES_TRANSITION.md`.
The file contains a single URL such as:

```text
http://127.0.0.1:43127/mcp/sse
```

The listener can be configured with:

```bash
striatumd --mcp-http-addr 127.0.0.1:8765
STRIATUM_DAEMON_MCP_HTTP_ADDR=127.0.0.1:8765 striatumd
```

Use `--mcp-http-addr off` to disable the listener. Non-loopback bind addresses
are refused.

## Protocol

The SSE stream opens at:

```http
GET /mcp/sse
```

The first event is `endpoint`; its data is a relative message URL:

```text
event: endpoint
data: /mcp/messages?session_id=<session>
```

Clients then send JSON-RPC requests to that URL:

```http
POST /mcp/messages?session_id=<session>
Authorization: Bearer <capability-token>
Content-Type: application/json
```

Responses are delivered on the SSE stream as `message` events. For test and
diagnostic clients, `POST /mcp/sse` without a session id returns the JSON-RPC
response body directly as `application/json`.

Supported MCP methods:

- `initialize`
- `notifications/initialized`
- `tools/list`
- `tools/call`

## Authentication

Use a daemon capability token in the HTTP `Authorization` header:

```http
Authorization: Bearer dtok_...
```

Tokens are the same daemon tokens used by Unix-socket RPC. The daemon runtime
`client-token` is not automatically applied to arbitrary clients; a supervisor
or operator must pass token material explicitly.

`tools/list` accepts an optional `repository_id`. Single-repository tools are
listed only when the token is authorized for that repository. `tools/call`
also accepts `repository_id` at the method params level and copies it into the
tool `arguments` object when the caller did not already provide one.

## Tool Calls

`tools/list` returns daemon methods that are all of:

- present in `contracts/daemon_methods.json`,
- non-deprecated,
- not internal `daemon.*` handshake methods,
- not hidden local workflow-authoring methods,
- authorized by the supplied token and repository scope.

Example direct diagnostic request:

```json
{"jsonrpc":"2.0","id":"tools","method":"tools/list","params":{"repository_id":"repo_123"}}
```

`tools/call` dispatches through daemon RPC. The result uses MCP tool result
shape with Striatum details in `structuredContent`:

```json
{
  "content": [{"type": "text", "text": "status"}],
  "structuredContent": {
    "ok": true,
    "method": "status",
    "audit_id": "audit_..."
  },
  "isError": false
}
```

Denied calls fail closed and audit under `transport = "mcp"`. Common denial
codes include `token_missing`, `token_malformed`, `token_invalid`,
`token_revoked`, `token_expired`, `capability_missing`,
`capability_scope_mismatch`, `capability_expired`, `repo_not_registered`, and
`method_unknown`.

## Agent Loop

The Go `--agent-loop` mode is a PTY supervisor only. It starts the configured
agent command, exports the daemon MCP endpoint in `STRIATUM_MCP_URL`, passes
token material through `STRIATUM_MCP_TOKEN` or `STRIATUM_MCP_TOKEN_FILE`, and
injects a bootstrap prompt.

The supervisor does not call `work.await_packet`, claim work, complete work,
release work, or write packet JSON. The agent is responsible for using MCP:

1. call `tools/list`,
2. call `work.await_packet` with `repository_id`, `session_id`, and
   `lease_seconds`,
3. use packet-provided commands and write scope,
4. report state with MCP tools such as `work.ack`, `artifact.publish`,
   `review.verdict`, `work.complete`, or `work.release`.

## Boundary

The MCP server is local-only and daemon-owned. It does not introduce hosted
services, telemetry, transcript capture, external persistence, direct database
writes outside daemon RPC, marker-file state, or terminal-output state.

Repository files remain provenance; PostgreSQL remains live workflow state.
`.striatum/` beside a target repository is operational scratch, not an MCP
message bus.
