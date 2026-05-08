---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0012-local-service-api.md", "docs/dogfood/006/research/SERVICE_SURFACE.md", "src/striatum/api.py", "src/striatum/mcp.py", "src/striatum/cli/parser.py", "src/striatum/cli/dispatch.py", "src/striatum/dashboard.py", "src/striatum/schema.py"]
---

# RFC 0012 V1 Design Synthesis

author: designer-codex-gpt-5.5-001

Date: 2026-05-08
Target: V1 build slice for RFC 0012 (Local Service API). The
research handoff confirmed RFC claims and pinned the stdlib shape;
this synthesis locks the contracts the implementer follows verbatim.

## 1. Module Layout

- **`src/striatum/service.py`** (new) — owns the server class,
  endpoint routing, SSE poll, mutation gate, auth, lifecycle.
- **`src/striatum/cli/parser.py`** — extend with `striatum serve`
  subcommand.
- **`src/striatum/cli/dispatch.py`** — wire `serve` to
  `service.run_service(repo, args)`.
- No new SQLite columns or migrations. The service reads
  `events`, `runs`, `jobs` via a dedicated read-only connection;
  state changes flow through `striatum.api.invoke` only.

## 2. Server Class

`ThreadingHTTPServer` for TCP. For Unix-socket mode, subclass
`http.server.HTTPServer` with `socketserver.ThreadingMixIn` and
`socketserver.UnixStreamServer.address_family = socket.AF_UNIX`
override (standard pattern). Handler is a single
`StriatumServiceHandler(BaseHTTPRequestHandler)` that dispatches
on `(method, path)`.

## 3. Endpoint Routing Table

| Method | Path pattern | Handler behaviour |
|---|---|---|
| `GET` | `/v1/health` | Static `{ok:true, data:{started_at, version, mode}}`. No DB hit. |
| `POST` | `/v1/invoke` | Parse `{argv: [...]}` from JSON body. Reject if first verb is mutating and `--allow-mutations` is off (HTTP 405 + JSON error). Else `api.invoke(argv, repo=service_repo)`. Return the envelope verbatim. |
| `GET` | `/v1/runs` | `api.invoke(["status", "--json"])`. |
| `GET` | `/v1/runs/<run_id>` | `api.invoke(["status", "--run-id", run_id, "--json"])`. |
| `GET` | `/v1/runs/<run_id>/why` | `id` from query string. `api.invoke(["why", id, "--json"])`. |
| `GET` | `/v1/runs/<run_id>/dashboard` | `api.invoke(["dashboard", "--run-id", run_id, "--once", "--json"])`. |
| `GET` | `/v1/runs/<run_id>/events` | SSE stream (see § 4). |
| `GET` | `/v1/doctor` | `api.invoke(["doctor", "--verbose", "--json"])` (with `--run-id` if `?run_id=` is supplied). |
| any other | any | 404 with JSON `{ok:false, error:{code:404, message:"not found"}}`. |

All non-SSE responses use `Content-Type: application/json` and
end with a newline.

## 4. SSE Wire Format

```text
event: striatum.event
id: <event_id>
data: {"event_id": <int>, "run_id": "...", "type": "...", "payload": {...}, "created_at": "...", "actor_session_id": "..." | null, "job_id": "..." | null}

```

(Trailing blank line per SSE spec; `data:` is one JSON line.)

End-of-stream signal when the run hits a terminal state:

```text
event: striatum.run_terminal
id: <last_event_id>
data: {"run_id": "...", "state": "completed" | "failed" | "canceled"}

```

The server then closes the response stream.

`?since=<event_id>` and `Last-Event-ID: <event_id>` HTTP header are
both honored. Header takes precedence when both are present.

Poll cadence: 250ms. Hard-coded for V1; future RFC may add a flag.
Concurrent SSE streams per run cap at 32; the 33rd request returns
HTTP 429 with a JSON envelope.

## 5. Mutation Detection

Whitelist of read verbs (top-level command). The first argv element
is consulted; if it is in the read set, the request proceeds; else
it is treated as mutating.

```python
SERVICE_READ_COMMANDS = frozenset({
    "status",
    "why",
    "doctor",
    "list",
    "evidence",
    "workflow",      # validate / plan / graph are read-only;
                     # workflow init is mutating but writes outside
                     # state.sqlite3, so it's ignored under the gate.
    "supervise",     # supervise list and supervise status are reads;
                     # the gate inspects argv[1] for "list"|"status"
                     # under this entry.
    "worktree",      # same: worktree list is the only read.
    "run",           # run summary and run graph are reads;
                     # run prepare and run start are mutations.
})

# Subcommand-aware refinement: when argv[0] is in
# {"workflow", "supervise", "worktree", "run"}, the gate inspects
# argv[1] against a per-parent read-subcommand whitelist.
```

A request whose argv falls outside this whitelist (and
`--allow-mutations` is off) returns HTTP 405 with body:

```json
{"ok": false, "error": {"code": 405, "message": "command requires --allow-mutations: ..."}}
```

## 6. Auth Model

- **Unix socket** (`--unix /path`): no token. Bind, then
  `os.chmod(path, 0o600)`. Document that the path's containing
  directory permissions matter.
- **HTTP loopback** (`--host 127.0.0.1 --port N` or default):
  optional `--token <secret>`. When set, every request must carry
  `Authorization: Bearer <secret>`. Compare via
  `hmac.compare_digest`. Mismatched/missing/empty token → 401 with
  JSON envelope.
- **Non-loopback host**: refuse at startup. Allowed values:
  `127.0.0.1`, `localhost`, `::1`. Anything else (including
  `0.0.0.0`, `0`, `*`, public IPs, hostnames that resolve outside
  loopback) → exit 8 with a clear message.

## 7. Process Lifecycle

- PID file at `.striatum/service.pid` (TCP) or
  `.striatum/scratch/service-<short-id>/service.pid` (Unix).
- On startup: check PID file; if present and `os.kill(pid, 0)`
  succeeds → exit 7 with "service already running on <addr>".
  Stale PID files (process gone) are overwritten.
- Write PID file as the first thing after binding.
- Register `signal.SIGTERM` and `signal.SIGINT` handlers that call
  `httpd.shutdown()` and emit `striatum.shutdown` SSE events to
  active streams.
- On graceful shutdown: drain in-flight requests for up to 5s,
  unlink the PID file (and the Unix socket file).
- `--idle-timeout-seconds <n>`: when no request in N seconds and
  no active SSE stream, shut down gracefully. Default unset.

## 8. CLI Surface

```text
striatum serve
  [--unix <path> | --host <host> --port <port>]
  [--token <secret>]
  [--allow-mutations]
  [--idle-timeout-seconds <n>]
  [--web]               # accepted but no-op in V1 (RFC 0013 lands the assets)
  [--json]              # emit a startup JSON envelope; default text
```

Defaults: `--host 127.0.0.1 --port 0` (kernel picks port; service
prints it on startup). `--unix` and `--host`/`--port` are mutually
exclusive.

Startup output (text mode):

```text
[striatum-serve] mode=tcp addr=127.0.0.1:54321 mutations=disabled token=disabled started_at=...
```

Startup output (JSON mode):

```json
{"ok": true, "data": {"mode": "tcp", "host": "127.0.0.1", "port": 54321, "allow_mutations": false, "token": false, "started_at": "...", "pid": 12345}}
```

## 9. Test Plan

`tests/test_service.py` (new), uses `pytest` + the existing
`run_cli` patterns where possible:

| Test | Asserts |
|---|---|
| `test_serve_health_endpoint` | GET `/v1/health` returns `ok:true, data:{started_at, version, mode}`. |
| `test_serve_invoke_read_command_succeeds_without_flag` | POST `/v1/invoke {argv:["status"]}` returns the same envelope as `striatum status --json`. |
| `test_serve_invoke_mutation_rejected_without_flag` | POST `/v1/invoke {argv:["init"]}` returns HTTP 405 + envelope. |
| `test_serve_invoke_mutation_succeeds_with_flag` | Same with `--allow-mutations` returns ok. |
| `test_serve_runs_endpoint` | GET `/v1/runs` matches `status --json`. |
| `test_serve_doctor_endpoint` | GET `/v1/doctor` matches `doctor --verbose --json`. |
| `test_serve_sse_streams_events` | Insert events into the events table; client connects to SSE; receives them in order. |
| `test_serve_sse_replay_with_since` | Client passes `?since=<id>`; server emits only newer events. |
| `test_serve_sse_replay_with_last_event_id_header` | Same via `Last-Event-ID`. |
| `test_serve_sse_run_terminal_closes_stream` | When `runs.state` flips terminal, server emits `striatum.run_terminal` and closes. |
| `test_serve_refuses_non_loopback_host` | `--host 0.0.0.0` exits 8 at startup. |
| `test_serve_token_required_with_token_flag` | Request without `Authorization` returns 401. |
| `test_serve_token_constant_time_compare` | Wrong-length token comparison doesn't short-circuit. (Behavioural test, not timing.) |
| `test_serve_unix_socket_binds_with_0600_permissions` | Bind to `--unix /tmp/...`; assert socket file mode `0o600`. |
| `test_serve_single_instance_via_pid_file` | Second invocation against the same socket exits 7. |
| `test_serve_stale_pid_file_overwritten` | PID file points at a dead PID → service starts cleanly, overwrites the file. |
| `test_serve_graceful_shutdown_on_sigterm` | Spawn server, SIGTERM, assert exit 0 and PID file removed. |
| `test_serve_concurrent_sse_clients` | Two clients receive events in parallel. |
| `test_serve_no_external_urls_in_responses` | Walk every response; assert no `http://` outside `127.0.0.1` / `localhost`. |
| `test_serve_no_transcripts_logged` | Spawn server, send requests, assert no body content written to stdout/stderr/disk. |

A separate `tests/test_service_smoke.py` runs an end-to-end smoke
where the service drives an actual workflow via
`POST /v1/invoke` (`--allow-mutations` enabled) and asserts a
multi-step run completes.

## 10. Documentation Updates

- **`docs/SPEC.md`** § "Local API And MCP Wrapper Boundary" — add
  a new subsection "Local Service (RFC 0012 V1)" documenting the
  `striatum serve` command, endpoint table, SSE shape, mutation
  gate, auth, lifecycle.
- **`docs/UBIQUITOUS_LANGUAGE.md`** — entries:
  - "local service" — the `striatum serve` HTTP/Unix-socket
    server.
  - "service mutation gate" — the `--allow-mutations` flag and
    its whitelist semantics.
- **`docs/rfcs/0012-local-service-api.md`** — status from
  `proposed` to `accepted (V1)`. Add "V1 Implementation Slice"
  subsection.
- **`docs/rfcs/README.md`** — index entry status flip.
- **`docs/DECISION_LOG.md`** — D-row recording RFC 0012 V1
  acceptance.
- **`docs/TODO.md`** — F-row marking RFC 0012 V1 done.
- **`README.md`** — short paragraph under existing "Local API
  And MCP Wrapper Boundary" pointer.
- **`CHANGELOG.md`** — Unreleased entry under Added.

## 11. Deferred Items

- **`--web`**: accepted as a no-op flag in V1 (RFC 0013 lands
  the static assets and the route handler that serves them).
- **WebSocket support.**
- **Long-poll fallback for SSE.**
- **Multi-process worker support.** Single-process,
  threading-per-connection is the V1 shape.
- **Configurable SSE poll cadence flag.** Hard-coded 250ms in V1.
- **`/v1/tool/<name>` MCP-style endpoint.** Out of scope.
- **Hot-reload semantics.** Operator restarts the service after
  upgrading.

## 12. Acceptance Gate

Per the dogfood-006 SKILL/RUNBOOK, the implementation job blocks
until a human acceptance decision is recorded under
`docs/dogfood/006/decisions/`.
