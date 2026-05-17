# RFC 0012: Local Service API

Status: accepted (V1)
Date: 2026-05-08

Current status note (2026-05-17): RFC 0070 supersedes the original
production dispatch detail. The service still preserves the RFC 0012 HTTP
shape, mutation gate, and envelope, but daemon-mapped production reads and
mutations now dispatch through daemon RPC. `striatum.api.invoke` remains a
local authoring and compatibility fixture path.

## V1 Implementation Slice

Implemented under dogfood-006. The V1 build slice landed:

- `src/striatum/service.py` (new) — `ThreadingHTTPServer` for TCP and
  a Unix-socket variant. The original V1 endpoints routed state mutation
  through `striatum.api.invoke` and read events directly; current production
  endpoints use daemon RPC for daemon-owned state.
- `striatum serve` CLI verb with `--unix`, `--host`, `--port`,
  `--token`, `--allow-mutations`, `--idle-timeout-seconds`, `--web`,
  `--json` flags.
- Endpoints: `/v1/health`, `POST /v1/invoke`, `/v1/runs`,
  `/v1/runs/<id>`, `/v1/runs/<id>/why?id=...`,
  `/v1/runs/<id>/dashboard`, `/v1/runs/<id>/events` (SSE),
  `/v1/doctor`.
- Mutation gate: whitelist of read verbs (`status`, `why`, `doctor`,
  `list`, `evidence`, `dashboard`, plus subcommand-aware reads under
  `workflow`, `supervise`, `worktree`, `run`, `recovery`). Anything
  else returns 405 without `--allow-mutations`.
- Auth: Unix-socket binds with `0o600`; HTTP loopback supports
  optional `--token` validated by length-safe constant-time compare
  (design-review F1).
- Non-loopback hosts refused at startup with exit 8.
- Single-instance enforcement via `.striatum/service.pid` (TCP) or
  `<unix-path>.pid` (Unix). Stale PID files are overwritten.
- Graceful shutdown on SIGTERM / SIGINT via an event-driven main
  thread that calls `server.shutdown()` synchronously after the
  signal fires.
- 16 tests in `tests/test_service.py` covering health, invoke
  read/mutation paths both ways, runs/doctor endpoints, non-loopback
  refusal, token auth, Unix-socket permissions, SSE replay via
  `?since`, single-instance enforcement, classification unit tests,
  graceful shutdown.

Findings F1–F3 from the design review folded in: token timing-safe
compare; SSE connection closes on disconnect/shutdown via
`try/finally`; `--web` flag is a documented no-op until RFC 0013
ships static assets, with a startup warning when set.

Deferred per the synthesis: WebSocket support, long-poll fallback,
multi-process workers, configurable SSE poll cadence, the
`/v1/tool/<name>` MCP-style endpoint.


Context:
`docs/DECISION_LOG.md` (D006, D007, D020),
`docs/SPEC.md` § "Local API And MCP Wrapper Boundary",
`src/striatum/api.py`,
`src/striatum/mcp.py`,
`src/striatum/dashboard.py`,
`docs/INTERVIEW_LOG.md` Q005

## Problem

D006 made SQLite the v1 coordination layer with the `striatum` CLI as the
first interface, and explicitly promised: "Slack, TUI, and web dashboards
can later attach through the same state store via CLI or a local API."
The interview-log version of the same conversation (Q005) phrased it as
"an optional Unix-socket or local HTTP API later for Slack, TUI, and web
adapters."

At the time of this V1 RFC, only two adapter surfaces existed:

- `striatum.api.invoke(args, repo=...)` — in-process Python API. Same
  command vocabulary as the CLI; same JSON envelope. Useful for embedding
  Striatum inside another Python program, but unreachable from non-Python
  clients (browsers, editor extensions, Slack bots, shell scripts that
  want to skip the CLI's startup cost).
- `python -m striatum.mcp` — stdio JSON-RPC wrapper. Useful for MCP-aware
  clients (Claude Code, Codex, Gemini CLI) but requires the client to
  speak the MCP framing protocol over a child process's stdio.

Neither surface lets a browser-based UI, a curl one-liner, or a non-MCP
network client drive Striatum. Anyone wanting that today has to shell out
to the CLI per call — paying its startup cost on each invocation and
losing live event streaming entirely.

The original runner recorded detailed event rows in SQLite. The current
daemon-required runtime exposes live state from daemon-owned PostgreSQL via
daemon RPC, while keeping this RFC's local HTTP/Unix-socket product shape.

## Goals

- Expose the existing CLI command vocabulary over HTTP and a
  Unix-domain socket without inventing a parallel command surface.
- Reuse the existing command dispatch path rather than inventing a parallel
  command surface. Historical V1 used `striatum.api.invoke`; current
  production daemon-mapped commands route through daemon RPC.
- Stream event rows as Server-Sent Events so live UIs (web, TUI watchers,
  editor extensions) react in real time without polling.
- Default to localhost-only / Unix-socket-only so the existing
  no-hosted-services product boundary (D020) is preserved by
  construction.
- Stay invisible to existing CLI users. The service is opt-in
  (`striatum serve`); existing scripts and dogfood runs do not depend
  on it.

## Non-Goals

- Multi-user authentication or RBAC. The service is one-process,
  one-operator.
- Cross-machine RPC. The service binds to `127.0.0.1` or a Unix
  socket; no remote-host serving in V1.
- A hosted dashboard. Striatum stays local-first. No telemetry, no
  outbound calls, no cloud egress.
- A replacement for the CLI or the in-process `api.invoke`. Both
  remain primary; the service is a third adapter that delegates to
  `api.invoke`.
- Workflow authoring in the browser. Workflows stay file-based.
- A general-purpose webhook or push surface. Server-Sent Events for
  reading is the only push direction in V1.
- Long-running MCP server mode (D028 / SPEC § Adapter Boundary
  excludes it). Use `python -m striatum.mcp` per-session.

## Proposal

Add a new CLI command `striatum serve` and a new module
`src/striatum/service.py` that runs a local-only HTTP/Unix-socket
server. The historical V1 server's request handlers reduced to `api.invoke`
calls; current production handlers use daemon RPC for daemon-owned state.

### CLI surface

```text
striatum serve [--unix /path/to/sock]
               [--host 127.0.0.1] [--port 0]
               [--token <secret>]
               [--allow-mutations]
               [--idle-timeout-seconds <n>]
               [--web]
```

Flag semantics:

- `--unix`: bind to a Unix-domain socket. Default permissions `0600`,
  owned by the current user. Mutually exclusive with `--host`/`--port`.
- `--host`/`--port`: bind to TCP. Default `127.0.0.1:0` (kernel picks
  port; service prints it on startup). Refuses non-loopback hosts in
  V1; refusal exits non-zero with a clear message. The `0.0.0.0`
  pattern is explicitly rejected so an operator can't accidentally
  expose the service to a LAN.
- `--token <secret>`: optional shared secret for HTTP. When set, every
  request must carry `Authorization: Bearer <secret>`. Unix-socket
  binding skips the token (filesystem permissions guard it).
- `--allow-mutations`: gate state-changing endpoints. Without this
  flag, only read endpoints work; mutation endpoints (anything that
  would call `claim-next`, `verdict`, `complete`, `decision record`,
  etc.) return HTTP 405. This matches the dashboard's read-only
  posture by default.
- `--idle-timeout-seconds`: shut down gracefully if no request in
  N seconds. Default unset (run until SIGTERM).
- `--web`: serve the RFC 0013 web UI assets at `/`. Without this flag,
  `/` returns 404 and only `/v1/*` is reachable. Out of scope for
  this RFC's acceptance criteria; documented for cross-reference.

### HTTP surface

All endpoints return `{"ok": bool, "data": {} | "error": {message,
code}}`, the standard Striatum envelope. JSON Content-Type unless noted.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/v1/health` | Liveness check; returns `{ok:true, data:{started_at, version}}`. Cheap; no DB hit. |
| `POST` | `/v1/invoke` | Body: `{"argv": [...]}` (the same arg vector the CLI takes after the `striatum` binary). Current production daemon-mapped reads/mutations dispatch through daemon RPC and return the standard envelope. |
| `GET` | `/v1/runs` | Convenience: list runs. Equivalent to `striatum status --json`. |
| `GET` | `/v1/runs/{run_id}` | Single run snapshot (state, jobs, latest verdicts, blockers). |
| `GET` | `/v1/runs/{run_id}/why?id=<entity_id>` | Convenience over `striatum why`. |
| `GET` | `/v1/runs/{run_id}/dashboard` | The exact JSON shape the TUI dashboard renders, suitable for a web UI. |
| `GET` | `/v1/runs/{run_id}/events` | Server-Sent Events stream of `events` table rows for the run, starting from `?since=<event_id>` if provided. |
| `GET` | `/v1/doctor` | Equivalent to `striatum doctor --json`. |

`POST /v1/invoke` is the catch-all that gives non-Python clients the
full CLI vocabulary without per-command HTTP design. The
convenience read endpoints exist so the web UI does not have to know
the exact CLI argv shape.

The service rejects requests that would mutate state when
`--allow-mutations` is unset. The check operates on the parsed argv
or HTTP method (read endpoints are GET-only; `/v1/invoke` is rejected
when its argv resolves to a known mutation command).

### Server-Sent Events (SSE) for the events table

`GET /v1/runs/{run_id}/events` opens an SSE stream:

```text
event: striatum.event
id: 1234
data: {"event_id": 1234, "run_id": "run_...", "type": "job.completed",
        "payload": {...}, "created_at": "..."}

event: striatum.event
id: 1235
data: {...}
```

Implementation:

- The server polls the `events` table at a configurable interval
  (default 250ms) for rows whose `event_id` exceeds the last sent.
- Clients reconnecting can pass `Last-Event-ID:` (standard SSE
  header) or `?since=<event_id>`; the server resumes from there.
- The server's poll loop closes when the client disconnects or when
  the run reaches a terminal state and the operator wants the stream
  to end (a `striatum.run_terminal` event marks end-of-stream).

SSE was chosen over WebSocket because it is unidirectional (the only
push direction we need), survives proxies and middleboxes trivially,
and requires no separate framing library. A future RFC can add
WebSocket if bidirectional control is needed.

### Historical Reuse of `api.invoke`

The V1 implementation built an argv vector and called
`striatum.api.invoke(argv, repo=service_repo)`. Current production
daemon-mapped handlers dispatch through daemon RPC; `api.invoke` remains
reserved for explicit local authoring and compatibility fixtures.

This kept the original V1 service small. Current production correctness comes
from the daemon method registry, capability metadata, and daemon RPC handlers;
the service still preserves the RFC 0012 local transport and envelope shape.

### Auth model

- **Unix socket**: no token. Filesystem permissions (`0600`, current
  user) are the auth boundary. Any process running as the same user
  can connect; that matches the existing CLI's trust model.
- **HTTP loopback without token**: accepted but logs a warning at
  startup. Local development convenience; not recommended for
  shared workstations.
- **HTTP loopback with `--token`**: every request must carry
  `Authorization: Bearer <token>`. Mismatched or missing token
  returns 401.
- **Non-loopback HTTP**: refused at startup with exit 8.

### Process lifecycle

- One service process per repo. Subsequent `striatum serve` calls in
  the same repo (detected via Unix socket presence or PID file) refuse
  with exit 7.
- Service writes a PID file at `.striatum/service.pid` (or per-socket
  in `.striatum/scratch/service-<id>/`).
- SIGTERM triggers a graceful shutdown: stop accepting new requests,
  drain in-flight requests with a 5-second timeout, close SSE
  streams with a `striatum.shutdown` event.

## Acceptance Criteria

- `striatum serve --unix /tmp/striatum.sock` boots in <1s, prints the
  bound socket path, and exits cleanly on SIGTERM.
- `curl --unix-socket /tmp/striatum.sock http://localhost/v1/health`
  returns `{"ok": true, ...}`.
- `POST /v1/invoke {"argv": ["status"]}` returns the same JSON
  payload as `striatum --repo . status --json`, byte-for-byte
  modulo whitespace.
- `GET /v1/runs/<id>/events` streams SSE events that match the
  `events` SQLite table. A client passing `?since=<event_id>` gets
  only newer events.
- Without `--allow-mutations`, any `POST /v1/invoke` whose argv is a
  mutation command returns HTTP 405 with an explanatory body.
- `--host 0.0.0.0` (or any non-loopback) refuses startup with exit
  code 8.
- `tests/test_service.py` covers smoke + auth + mutation gating +
  SSE replay.

## Open Questions

- **Single-process vs multi-worker.** V1 ships single-process; SQLite
  is the bottleneck and one process avoids busy-write contention. Does
  any workload need workers? Probably not at V1 scale.
- **Hot-reload semantics.** When the workflow file or `.striatum/`
  state changes during a serve session, the service does not need to
  reload anything (every request reads fresh from SQLite). Good. If
  the operator updates the runner package itself, restart the service.
- **WebSocket support.** Deferred. SSE covers V1 push needs; bidir
  WebSocket adds framing complexity for no clear V1 use.
- **Long-poll fallback for SSE.** Some corporate proxies kill SSE
  streams after 30 seconds. V1 does not target those environments
  (it's local-only); deferred.
- **Multiple concurrent streams per run.** SSE allows it; the SQLite
  poll loop is shared across connections via a small fanout. V1
  caps concurrent SSE streams at 32 per run (configurable later).
- **Standard library vs Starlette/Flask.** V1 should use the standard
  library (`http.server` + custom socket handling) to avoid adding a
  dependency. If maintenance burden balloons, RFC 0012-bis can adopt
  a microframework. The dashboard module already shows local-server
  patterns work without dependencies.
- **Should the service expose write endpoints over HTTP at all?**
  `--allow-mutations` is the gate. The default is off; operators who
  want a one-button "claim next" web UI flip the flag. Worth a future
  audit decision once the web UI usage matures.

## Relationship To Other RFCs

- **RFC 0013 (proposed, paired)** — the local web UI. Depends on the
  read endpoints and the SSE stream from this RFC. Web UI assets are
  served by `striatum serve --web`.
- **RFC 0009** — long-lived process supervision. The service is
  itself a long-lived process, but it does not enter the supervisor
  flow; it sits next to the supervisor. Both can run concurrently.
- **D006 / D007** — established the CLI as primary and the local API
  as the optional layer. This RFC operationalizes the "optional API"
  D006 promised.
- **D020 (no hosted services)** — V1 binds localhost / Unix only;
  refuses non-loopback. This RFC does not relax the boundary.
- **D028 (no transcripts)** — the service does not log request bodies
  or response payloads to disk. SQLite event rows are the only
  durable trace.
