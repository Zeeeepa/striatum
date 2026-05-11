# Striatum Local MCP-Like Wrapper

Status: implemented local wrapper
Date: 2026-05-07

## Overview

Striatum is a local-first orchestration tool. The MCP-like wrapper is a small
stdio JSON-RPC adapter over `striatum.api.invoke`; it is not a second control
plane and does not write SQLite directly.

The CLI remains the product contract. The wrapper only gives local tools a
structured stdio surface for the same commands.

## Architecture

Striatum can run as a local stdio JSON-RPC server:

- **Transport:** JSON-RPC over stdio.
- **Request framing:** Content-Length headers by default, with line-delimited
  fallback. See [Framing](#framing) below.
- **Root Directory:** start it inside, or pass `--repo` for, the target
  repository.
- **State authority:** `.striatum/state.sqlite3`, mutated only through the
  existing CLI dispatcher via `striatum.api.invoke`.

Development command:

```bash
PYTHONPATH=src python3 -m striatum.mcp --repo /path/to/target/repository
```

Installed checkouts can use the same module through the installed Python
environment:

```bash
python3 -m striatum.mcp --repo /path/to/target/repository
```

## Framing

The wrapper supports two on-the-wire framings and detects which one to use
from the very first inbound message:

- **`framed`** -- LSP/MCP-style. Each JSON-RPC body is preceded by a
  `Content-Length: N\r\n\r\n` header. This is the only safe shape for bodies
  that contain newlines and is what real MCP clients (Claude Desktop, IDE
  MCP integrations) speak. Standard MCP clients should now connect cleanly.
- **`line`** -- legacy local shape. One JSON-RPC object per text line. Used
  by the existing tests and hand-rolled local scripts.

In the default `auto` mode, the first message determines the shape: if it
starts with a `Content-Length:` header, both reads and writes lock into
framed mode; otherwise they lock into line mode. The shape is then stable
for the remainder of the session, so a real MCP client and a legacy
line-delimited script both get a coherent server.

Operators can pin either mode explicitly:

```bash
python3 -m striatum.mcp --framing framed --repo /path/to/target/repository
python3 -m striatum.mcp --framing line   --repo /path/to/target/repository
python3 -m striatum.mcp --framing auto   --repo /path/to/target/repository  # default
```

Framed write shape (header lines use CRLF, body has no trailing newline):

```
Content-Length: 64\r\n
\r\n
{"jsonrpc":"2.0","id":1,"result":{"serverInfo":{"name":"..."}}}
```

The server supports these JSON-RPC methods:

- `initialize`
- `tools/list`
- `tools/call`
- `resources/list`
- `resources/read`
- `striatum/invoke`

`striatum/invoke` accepts raw CLI-style args:

```json
{"jsonrpc":"2.0","id":1,"method":"striatum/invoke","params":{"args":["status"]}}
```

## Tools

`tools/list` returns the available tool names. Each tool maps to an existing
Striatum command. Examples:

| Tool | Command |
|------|---------|
| `register_session` | `register-session` |
| `claim_next` | `claim-next` |
| `ack` | `ack` |
| `heartbeat` | `heartbeat` |
| `release` | `release` |
| `send` | `send` |
| `block` | `block` |
| `publish_artifact` | `publish-artifact` |
| `submit_review` | `submit-review` |
| `complete_job` | `complete` |
| `verdict` | `verdict` |
| `status` | `status` |
| `why` | `why` |
| `doctor` | `doctor` |

Example tool call:

```json
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"status","arguments":{}}}
```

The JSON-RPC result includes `structuredContent`, which is the
`striatum.api.invoke` envelope:

```json
{"ok":true,"data":{"runs":[]}}
```

Command validation and workflow errors are returned inside that envelope rather
than by bypassing Striatum's normal exit-code semantics.

## Resources

The wrapper exposes read-only resources that also map to existing commands:

- `striatum://status`
- `striatum://status?run_id=<run-id>`
- `striatum://doctor`
- `striatum://doctor?run_id=<run-id>`
- `striatum://why/<id>`

Example:

```json
{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"striatum://status"}}
```

## Boundary

The wrapper deliberately avoids hosted services, sockets, telemetry,
transcript capture, external persistence, and direct SQLite writes. It is a
local adapter over the existing CLI/API semantics.

## Daemon MCP Surface

RFC 0028 V1 adds a separate MCP handler for the optional local
multi-repo registry. It is resources-only in V1: `tools/list` returns an
empty list, `tools/call` has no mutation route, and `striatum/invoke` is
not part of the daemon MCP surface. The handler opens the owner-only
registry SQLite directly; it does not connect to a daemon RPC server in
V1.

RFC 0033 V2 changes the daemon-owned storage substrate, and RFC 0030/0031
add the daemon RPC/supervision/apply foundation on top of it. Daemon MCP
still remains resources-only in this release: mutation tools are RFC 0032
scope and must use the same RPC capability/audit boundary when they land.
There is no MCP-specific trust shortcut.

Daemon resources:

- `striatum://daemon/repos`
- `striatum://daemon/dashboard`
- `striatum://repo/<repository_id>/status`
- `striatum://repo/<repository_id>/doctor`
- `striatum://repo/<repository_id>/runs`
- `striatum://repo/<repository_id>/run/<run_id>`
- `striatum://repo/<repository_id>/run/<run_id>/why?id=<id>`
- `striatum://repo/<repository_id>/blockers`
- `striatum://repo/<repository_id>/stale-leases`

Every daemon MCP `resources/list` and `resources/read` request requires
an explicit `token` parameter. The token must have `read` capability:
global read tokens see every active repository, while repo-scoped read
tokens see only resources for their repository ids and are denied when
reading another repository. The daemon runtime fallback token is not
implicitly applied to MCP clients. `striatum://daemon/audit` is
intentionally absent in V1; audit is available only through daemon admin
CLI registry surfaces.

When the RFC 0033 V2 substrate is active, daemon MCP resources read from
the daemon DB instead of the V1 registry SQLite. The authorization and
resources-only boundaries above remain the same unless a later accepted
RFC changes them.
