---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0012 V1 — Service Surface Research

author: researcher-codex-gpt-5.5-001

Date: 2026-05-08
Inputs: `src/striatum/api.py`, `src/striatum/mcp.py`,
`src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py`,
`src/striatum/dashboard.py`, `src/striatum/schema.py` (events table),
RFC 0012, D006, D020, D028.

Verifies RFC 0012 claims line by line and pins the smallest stdlib
shape that satisfies V1.

## Existing surfaces

### `striatum.api.invoke` (api.py)

`invoke(args: Sequence[str], *, repo) -> JsonObject` parses the same
argv as the CLI, calls `dispatch`, returns:

- success: `{"ok": True, "data": <handler-return>}`
- handler raised `StriatumError`:
  `{"ok": False, "error": {"message": ..., "code": exit_code}}`
- argparse `SystemExit`:
  `{"ok": False, "error": {"message": "invalid striatum command arguments", "code": 2}}`
- sqlite3 error:
  `{"ok": False, "error": {"message": ..., "code": 1}}`

The function forces `namespace.json = True` so the dispatcher emits
structured payloads. It's the single source of truth for what an HTTP
endpoint should return.

### `striatum.mcp` (mcp.py)

The MCP wrapper already does the work the service needs:

- `TOOL_ARGV` dict (line 48) maps tool names to argv templates with
  `$placeholder` substitution.
- `build_args(name, arguments)` (line 489) renders argv from a JSON
  argument object.
- `call_tool` (line 482) calls `striatum.api.invoke(argv, repo=repo)`
  and returns the envelope.
- Resource reads (`read_resource`) for `striatum://status`,
  `striatum://why?id=...`, etc. — the same shape the HTTP convenience
  endpoints want.

The service can reuse `TOOL_ARGV` + `build_args` directly for the
`POST /v1/invoke` body shape. SSE-specific events streaming is the
new piece.

### Events table

`schema.py` declares:

```sql
CREATE TABLE events (
  event_id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT REFERENCES runs(run_id),
  event_type TEXT NOT NULL,
  ...
  payload_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL
);
CREATE INDEX idx_events_run_time ON events(run_id, event_id);
```

- `event_id` is monotonically increasing (AUTOINCREMENT), so
  `?since=<id>` selects new rows in order.
- The `idx_events_run_time` index covers `run_id, event_id`, so
  the SSE poll query
  `SELECT * FROM events WHERE run_id = ? AND event_id > ? ORDER BY event_id`
  is index-seek cheap.
- SQLite handles concurrent reads from a separate connection
  cleanly; the service can open its own read-only connection to the
  state DB.

## Mutation classification

The `--allow-mutations` gate needs a rule for which CLI verbs mutate
state. Based on the parser:

**Mutation verbs** (need `--allow-mutations` to succeed):

- `init`, `workflow init`
- `run prepare`, `run start`, `branch confirm`
- `register-session`, `claim-next`, `ack`, `heartbeat`, `release`,
  `send`, `block`
- `publish-artifact`, `submit-review`, `verdict`, `complete`,
  `decision record`
- `worktree create`, `worktree release`
- `supervise start`, `supervise send`, `supervise stop`
- `recovery requeue-stale`, `recovery cancel-job`,
  `recovery cancel-run`, `recovery process-reconcile`
- `checkpoint resolve`
- `session close`
- `adapter run`

**Read verbs** (always safe):

- `status`, `why`, `doctor`, `dashboard --once`
- `workflow validate`, `workflow plan`, `workflow graph`
- `run summary`, `run graph`, `evidence export`
- `worktree list`, `supervise status`, `supervise list`
- `list jobs`, `list sessions`, `list artifacts`, etc.

V1 design: implement the gate as a **whitelist of read verbs**. Any
top-level command not in the whitelist is treated as a mutation.
Conservative; new mutating subcommands are blocked by default.

## Stdlib server choice

Three viable stdlib paths:

1. **`http.server.HTTPServer` + custom handler.** Simplest. Single-
   threaded; one client at a time. SSE streams block other clients.
2. **`http.server.ThreadingHTTPServer` + custom handler.** Drop-in
   replacement; spawns a thread per connection. Multiple SSE
   subscribers + concurrent reads work cleanly.
3. **`socketserver.UnixStreamServer` for Unix socket.** Pair with
   `http.server.BaseHTTPRequestHandler` to speak HTTP over a Unix
   socket. Standard pattern.

V1 design recommendation: `ThreadingHTTPServer` for TCP;
`UnixStreamServer` (subclassed to inherit from `ThreadingMixIn`) for
Unix. The `dashboard.py` already uses stdlib threading patterns;
service follows the same template.

## SSE design

Wire format (RFC 0012 § "Server-Sent Events"):

```text
event: striatum.event
id: <event_id>
data: <json blob>

```

Note the trailing blank line — required by the SSE spec.

Server loop:

```python
last_id = int(query.get("since", "0")) or _last_event_id_header(request)
while not client_disconnected:
    rows = conn.execute(
        "SELECT * FROM events WHERE run_id = ? AND event_id > ? "
        "ORDER BY event_id LIMIT 100",
        (run_id, last_id),
    ).fetchall()
    for row in rows:
        write_sse(row)
        last_id = row["event_id"]
    if run_terminal(run_id) and not rows:
        write_sse(final_marker)
        break
    time.sleep(0.25)
```

Cadence default: 250ms. Configurable via `--sse-poll-interval-seconds`
(out of scope for V1; recommend pin to 250ms).

End-of-stream signal: when `runs.state` reaches a terminal value
(`completed`, `failed`, `canceled`), emit one final `striatum.run_terminal`
SSE event then close the stream.

## Auth model

- **Unix socket**: filesystem permissions (`0600`). No token. The
  socket file is owned by the current user; `os.chmod(path, 0o600)`
  after binding.
- **HTTP loopback**: refuse `--host` values outside `127.0.0.1`,
  `localhost`, `::1`. Refuse `0.0.0.0` and any non-loopback IP. Exit 8
  at startup with a clear message.
- **HTTP loopback + `--token`**: every request must carry
  `Authorization: Bearer <secret>`. Constant-time comparison via
  `hmac.compare_digest`. Mismatched/missing token → 401.

## Process lifecycle

- PID file at `.striatum/service.pid` (TCP) or
  `.striatum/scratch/service-<id>/service.pid` (Unix socket).
- Single-instance check: read PID file; `os.kill(pid, 0)`; if alive,
  exit 7.
- `signal.signal(SIGTERM, ...)` triggers `httpd.shutdown()`.
- Drain in-flight requests with a 5-second timeout, close SSE
  streams with a `striatum.shutdown` event.

## Recommended endpoint routing

| Method | Path | Handler |
|---|---|---|
| GET | `/v1/health` | static `{ok: true, data: {started_at, version}}` |
| POST | `/v1/invoke` | parse `{argv: [...]}`; reject if first verb is in mutation set and `--allow-mutations` is off; call `api.invoke` |
| GET | `/v1/runs` | `api.invoke(["status", "--json"])` |
| GET | `/v1/runs/<run_id>` | `api.invoke(["status", "--run-id", run_id, "--json"])` |
| GET | `/v1/runs/<run_id>/why?id=<id>` | `api.invoke(["why", id, "--json"])` |
| GET | `/v1/runs/<run_id>/dashboard` | `api.invoke(["dashboard", "--run-id", run_id, "--once", "--json"])` |
| GET | `/v1/runs/<run_id>/events` | SSE stream |
| GET | `/v1/doctor` | `api.invoke(["doctor", "--verbose", "--json"])` |

The service module imports `striatum.api.invoke` only — not `db.py`
directly except for the dedicated SSE poll connection.

## Risks / unknowns

- **`hmac.compare_digest` requires equal-length inputs.** Wrap with
  a length-check that compares to a fixed-length scrub on mismatch
  to avoid early-exit timing leak. Standard pattern.
- **SSE keep-alive under proxies.** D020 means we don't target
  proxies; localhost-only is fine.
- **Long-lived SSE connections + ThreadingHTTPServer** spawns one
  thread per connection. Cap concurrent SSE streams per-run at 32
  (per RFC 0012 open question); 33rd connection returns 429.
- **Graceful shutdown vs hung handlers.** If a handler is stuck in
  `api.invoke`, shutdown waits up to 5s then forcibly closes. This
  is acceptable; the existing CLI exit-code semantics propagate.
- **Argparse SystemExit inside a thread.** `api.invoke` already
  catches SystemExit; safe.

## Recommended minimum-touch implementation order

1. New module `src/striatum/service.py` with
   `serve(repo, host, port, unix, token, allow_mutations,
   idle_timeout, web)`. Internally uses `http.server` +
   `socketserver` mixins.
2. Argparse wiring in `parser.py` + dispatch in `dispatch.py`.
3. Mutation whitelist constant (read verbs); the rest is mutation.
4. `tests/test_service.py` covering: health, invoke (read), invoke
   (mutation rejected without flag, accepted with flag), runs list,
   run detail, doctor, SSE replay via `?since`, SSE end-of-stream
   on run terminal, non-loopback host refusal, token mismatch,
   concurrent SSE clients, single-instance enforcement, graceful
   shutdown.

## Friction encountered

None substantive. Worth flagging:

- The mutation-vs-read split is convention-only today. If a future
  RFC adds a new mutating verb without adding it to a documented
  list, the service would still default-block via the whitelist
  approach. Safe failure mode.
- `dashboard.py` invokes `striatum dashboard --once --json` and
  emits a JSON shape; the service can reuse it verbatim.
- `mcp.py` already has `TOOL_ARGV` placeholder substitution. If the
  service exposes a sister `/v1/tool/<name>` endpoint (out of V1
  scope), it could share that logic.
