---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0012 V1 Build Handoff

author: implementer-codex-gpt-5.5-001

Date: 2026-05-08
Run: dogfood-006 / RFC 0012 (Local Service API)
Decision: `accepted_with_follow_up` (autonomous)

The combined V1 build slice for RFC 0012 shipped in one commit. All
three design-review findings (F1–F3) folded in.

## Files Changed

- **`src/striatum/service.py`** (new, ~680 lines) — owns
  `ThreadingHTTPServer` (TCP) + `_ThreadedUnixServer` (Unix socket),
  `StriatumServiceHandler` with endpoint routing, `ServiceState`,
  `is_read_command`, `tokens_match`, `run_service`, lifecycle
  helpers, exception types.
- **`src/striatum/cli/parser.py`** — `striatum serve` argparse
  subcommand with all V1 flags.
- **`src/striatum/cli/dispatch.py`** — wire `serve` to
  `service.run_service`; map `ServiceConfigError` → exit 8 and
  `ServiceAlreadyRunningError` → exit 7.
- **`tests/test_service.py`** (new, 16 tests) — health, invoke
  read/mutation paths both ways, runs/doctor endpoints, non-loopback
  refusal, token auth (including same-length wrong-token rejection),
  Unix-socket permissions, SSE replay via `?since`, single-instance
  enforcement, classification unit tests, graceful shutdown.

## Documentation Updates

- `docs/SPEC.md` — new "Local Service (RFC 0012 V1)" subsection
  under "Local API And MCP Wrapper Boundary".
- `docs/UBIQUITOUS_LANGUAGE.md` — added "local service" and
  "service mutation gate" entries.
- `docs/rfcs/0012-local-service-api.md` — status from `proposed`
  to `accepted (V1)` with a "V1 Implementation Slice" subsection.
- `docs/rfcs/README.md` — index entry status flipped.
- `docs/DECISION_LOG.md` — D058 row.
- `docs/TODO.md` — F6 row marked done.
- `CHANGELOG.md` — new `## 0.2.0 — 2026-05-08` section under a
  fresh `## Unreleased` placeholder.
- `pyproject.toml` — version bumped from `0.1.0` to `0.2.0` (per
  the user's "add releases, version numbers as you land these"
  directive).

## Tests / Lint / Typecheck

- `make test`: **225 passed** (was 209; +16 new).
- `make lint`: clean.
- `make typecheck`: clean (43 source files).
- `tests/test_service.py`: 16/16 in ~15s.

## Validation Against Design-Review Findings

| Finding | Status | Notes |
|---|---|---|
| F1 (token timing-safe compare) | done | `tokens_match` pads both inputs and asserts length equality after the constant-time digest. Same-length wrong-token rejection tested in `test_serve_token_required_with_token_flag`. |
| F2 (close SSE conn on disconnect/shutdown) | done | `_stream_events` wraps the read connection in `try/finally`; the SSE slot is released and the connection closed on every exit path. |
| F3 (`--web` no-op messaging) | done | Service emits a `web_warning` field in the startup envelope when `--web` is set; `/` returns a 404 with an explanatory message that points at RFC 0013. |

## Acceptance Criteria From RFC 0012

| Criterion | Status |
|---|---|
| `striatum serve --unix /tmp/sock` boots <1s, prints socket path, exits cleanly on SIGTERM | yes (test) |
| `curl --unix-socket ... http://localhost/v1/health` returns `{ok: true, ...}` | yes (Unix socket binds at 0o600; HTTP-over-socket end-to-end tested in `test_serve_unix_socket_binds_with_0600` plus health) |
| `POST /v1/invoke {argv:["status"]}` matches `striatum status --json` | yes (test) |
| `GET /v1/runs/<id>/events` SSE stream | yes (test with `?since` replay) |
| Mutation rejected without `--allow-mutations` | yes (test) |
| `--host 0.0.0.0` refuses with exit 8 | yes (test) |
| `tests/test_service.py` covers smoke + auth + mutation gating + SSE replay | yes (16 cases) |

## Deferred (per synthesis § 11)

- WebSocket support
- Long-poll fallback for SSE under aggressive proxies
- Multi-process worker support
- Configurable SSE poll cadence flag (hard-coded 250ms in V1)
- `/v1/tool/<name>` MCP-style endpoint
- Hot-reload semantics

## Friction Encountered

Initial implementation used a signal handler that spawned a thread
to call `server.shutdown()`, which deadlocked because of the chain
between the signal-fired thread, the spawned helper thread, and
the `serve_forever` thread. Switched to an event-driven main
thread (`shutdown_event.wait()` then synchronous `server.shutdown()`
in the main thread) per the stdlib's documented pattern. Recorded
in `docs/dogfood/FRICTION_LOG.md` as the dogfood-006 entry.

## Version

`0.2.0` — first explicitly versioned release. The pre-existing
RFC backlog (0001–0011 + 0014) is treated as the `0.1.0` baseline.
