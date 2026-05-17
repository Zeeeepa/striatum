# Striatum Local MCP-Like Wrapper

Status: implemented local wrapper
Date: 2026-05-07

## Overview

Striatum is a local-first orchestration tool. The MCP-like wrapper is a small
stdio JSON-RPC adapter over the same daemon-mediated command surface as the
CLI; it is not a second control plane and does not write live state directly.

The daemon method contract remains the product boundary. The CLI and
the wrapper are local clients over that same audited command surface.

## Architecture

Striatum can run as a local stdio JSON-RPC server:

- **Transport:** JSON-RPC over stdio.
- **Request framing:** Content-Length headers by default, with line-delimited
  fallback. See [Framing](#framing) below.
- **Root Directory:** start it inside, or pass `--repo` for, the target
  repository.
- **State authority:** daemon-owned PostgreSQL scoped to the registered
  target repository. `.striatum/` beside the target repo is operational
  scratch only.

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

## Dogfood-Lifecycle Tools

RFC 0040 V1 adds twelve chat-tool entries that mirror the operator
sequence used to drive a dogfood end-to-end. The local web chat
surface (`striatum serve --web --allow-mutations`) exposes them in
its tool list; ten are mutation-gated and require `--allow-mutations`,
two are read-shaped and stay available without it.

| Tool | Mutation? | Underlying CLI verb | Required capability* |
|------|-----------|---------------------|----------------------|
| `run_prepare(workflow_path)` | yes | `striatum run prepare --workflow` | `write` |
| `run_start(run_id)` | yes | `striatum run start --run-id` | `write` |
| `register_session(run_id, role, lane, fresh?, parent_session_id?, operator_label?, capabilities?)` | yes | `striatum register-session` | `write` |
| `supervise_start(session_id)` | yes | `striatum supervise start --session-id` | `write` |
| `claim_next(session_id, lease_seconds?)` | yes | `striatum claim-next` | `claim` |
| `ack(session_id, message_id, lease_id)` | yes | `striatum ack` | `write` |
| `publish_artifact(session_id, job_id, lease_id, kind, logical_name, path)` | yes | `striatum publish-artifact` | `write` |
| `verdict(session_id, job_id, lease_id, verdict, findings_artifact_id?, rationale?)` | yes | `striatum verdict` | `review` |
| `complete(session_id, job_id, lease_id, summary?)` | yes | `striatum complete` | `write` |
| `supervise_stop(session_id, reason)` | yes | `striatum supervise stop` | `write` |
| `run_summary(run_id, path)` | no | `striatum run summary` | `read` |
| `evidence_export(run_id, path)` | no | `striatum evidence export` | `read` |

\* The `required capability` column lists the daemon-RPC capability
the matching RPC method already requires (see
[`src/striatum/daemon_rpc/registry.py`](../src/striatum/daemon_rpc/registry.py)).
The local web chat surface is owner-only and reuses the mutation gate
instead of token capabilities; when the daemon serves these tools
through its MCP transport, `tools/list` filtering applies normally.

Each tool is a thin shell over the existing CLI verb (via
`striatum.api.invoke`); the daemon's audit chain records the same
rows whether the operator ran the CLI directly or called the chat
tool. RFC 0040 §2/§3 also scope composite operator tools
(`dogfood.publish_on_behalf`, `dogfood.surgical_recovery`) that
compose `ack` + `publish-artifact` + `verdict`/`complete` (or
recovery + lease reactivation) into single audit-chain entries; those
land in the daemon-side systems half of the RFC.

### Example chat-tool sequence

The operator session would call these in order to drive a one-job
dogfood through to completion:

1. `run_prepare(workflow_path="docs/dogfood/0NN/workflow.json")`
   → `{"run_id": "run_…"}`.
2. `run_start(run_id="run_…")`.
3. `register_session(run_id="run_…", role="implementer", lane="claude_code", fresh=true)`
   → `{"session_id": "sess_…"}`.
4. `supervise_start(session_id="sess_…")`.
5. `claim_next(session_id="sess_…")`
   → `{"packet_id": …, "lease": {"lease_id": "lease_…"}, "message_id": …}`.
6. (Implementer writes the artifact; if `striatum ack` is denied:)
   `ack(session_id, message_id, lease_id)` from the operator session.
7. `publish_artifact(session_id, job_id, lease_id, kind, logical_name, path)`.
8. `complete(session_id, job_id, lease_id, summary="…")`.
9. `supervise_stop(session_id, reason="…")`, then
   `run_summary` + `evidence_export` to capture the artifacts.

When `tools/list` is consulted by the chat session, mutating entries
are hidden unless `serve --allow-mutations` is in force. Local-MCP
clients can still discover the read-shaped pair (`run_summary`,
`evidence_export`) even with mutations disabled.

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

The wrapper deliberately avoids hosted services, network listening sockets,
telemetry, transcript capture, external persistence, and direct database
writes. It is a local adapter over the existing CLI/API semantics.

## Daemon MCP Surface

Daemon MCP is a daemon RPC client surface over the daemon-owned
PostgreSQL substrate. It is not the old resources-only registry handler:
`tools/list` returns the caller's effective method-registry tools,
`tools/call` dispatches through daemon RPC, and every call is
re-authorized under the provided token and repository scope. Denied calls
append metadata-only audit/request-log rows with `transport = "mcp"`.
There is no MCP-specific trust shortcut and no daemon-MCP equivalent of
`serve --allow-mutations`.

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
reading another repository. The daemon runtime `client-token` file is not
implicitly applied to MCP clients. `striatum://daemon/audit` is
intentionally absent in V1; audit is available only through daemon admin
CLI registry surfaces.

Daemon MCP mutation capabilities use the closed RFC 0032 vocabulary:
`read`, `write`, `review`, `claim`, `apply`, `admin`, `recovery`, and
`surgical_recovery`.
`tools/list` returns the effective tool set: method registry entries
intersected with the token's grants and repository scope. `tools/call`
fails closed for unknown methods, missing tokens, revoked/expired tokens,
missing capabilities, expired capabilities, and repository scope
mismatches. Repo-scoped `apply` grants remain single-repo; a token that
can apply in repo A cannot apply in repo B.

Striatum's MCP and chat tool surfaces do **not** include any `memory.*`
capability. Engram (under RFC 0044) defines its own `memory.read_striatum`,
`memory.describe`, and related capabilities locally inside Engram's own
MCP server (`engram-mcp-stdio`), wired by the operator out of band.
Striatum's daemon registry, chat tools, and CLI do not import an Engram
client and do not call any retrieval surface during state transitions;
see [`docs/SPEC.md` § Corpus Export And Augmentation Boundary](SPEC.md)
and [RFC 0057](rfcs/0057-corpus-contract-v2.md).

## Mutation Surface For Agents

RFC 0036 adds an agent-facing `striatum-mcp` skill and chat workflow
generation tools over the existing surfaces. Agents should call
`tools/list` first because it is the effective tool set for the current
token. `tools/call` remains the authorization boundary and re-checks every
call.

Workflow generation follows preview-then-write. `generate_workflow_preview`
writes nothing and returns the generated workflow, files, graph metadata,
warnings, and validation. `generate_workflow_write` is hidden unless the
service was started with `--allow-mutations`; if a stale or crafted call
reaches the server anyway, it returns `mutations_disabled`. Even when
visible, writes require `confirm_write: true` and a separate operator
confirmation gesture in the chat UI.

Denials are recovery instructions, not retry loops: ask for a narrow token
on `capability_missing`, stop on `token_revoked`, ask for a fresh token on
`token_expired`, inspect `tools/list` / `daemon.describe` on
`method_unknown`, and restart the local service with `--allow-mutations`
only if the operator actually wants writes.
