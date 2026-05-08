---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["dogfood-006", "rfc-0012"]
---

# RFC 0012 V1 Build Review

author: reviewer-claude-opus-002

Date: 2026-05-08
Run: dogfood-006
Read (fresh, repo-level access):

- `src/striatum/service.py`;
- `src/striatum/cli/parser.py` (serve subparser);
- `src/striatum/cli/dispatch.py` (serve dispatch);
- `tests/test_service.py`;
- `docs/dogfood/006/BUILD_HANDOFF.md`;
- `docs/dogfood/006/DESIGN_SYNTHESIS.md`;
- `docs/dogfood/006/review/design/DESIGN_REVIEW.md`;
- updated SPEC, UBIQUITOUS_LANGUAGE, RFC 0012, RFC index,
  DECISION_LOG, TODO, CHANGELOG, pyproject.toml.

Verdict intent: **accept**.

The build matches the accepted design and folds in all three
design-review findings (F1–F3). 225 / 225 tests pass; lint and
typecheck clean. Findings F1–F3 below are informational only.

## D020 / D006 / D028 Compliance

- **D020 (no remote serving)** — `_ensure_loopback` rejects
  `0.0.0.0` and any non-loopback host; tested by
  `test_serve_refuses_non_loopback_host`. Unix-socket binding is
  filesystem-local.
- **D006 (api.invoke is the dispatch path)** — All endpoints except
  SSE delegate to `striatum.api.invoke`. SSE opens a dedicated
  read-only connection to the events table and never writes; this
  is acceptable because the SSE view is a streaming projection of
  immutable rows.
- **D028 (no transcripts)** — `log_message` is overridden to a
  no-op; request bodies / response payloads are not written to
  stdout/stderr/disk.

## Mutation Whitelist

`is_read_command` correctly accepts top-level reads (`status`,
`why`, `doctor`, `list`, `evidence`, `dashboard`) and
subcommand-aware reads under `workflow`, `supervise`, `worktree`,
`run`, `recovery`. Tested explicitly in
`test_is_read_command_classification`. Anything else is mutating;
the gate is conservative and forward-safe (new mutating verbs
default to blocked).

## SSE Implementation

- `_stream_events` uses a `try/finally` that closes the read
  connection and releases the SSE slot on every exit path (F2
  honored).
- `?since=<id>` and `Last-Event-ID` are both honored;
  `Last-Event-ID` takes precedence per the synthesis.
- The terminal-state detection (`runs.state` in
  `{completed, failed, canceled}`) closes the stream with a
  `striatum.run_terminal` event.
- The 32-streams-per-run cap is enforced by `acquire_sse_slot` in
  `ServiceState`.

## Auth

- Unix-socket binds with `0o600` (verified by
  `test_serve_unix_socket_binds_with_0600`).
- Token auth uses `tokens_match` which pads inputs to a fixed
  minimum and compares both digest and length, eliminating the
  length-leak from naked `hmac.compare_digest` (F1 honored).
- Tested with same-length wrong token rejection in
  `test_serve_token_required_with_token_flag`.

## Lifecycle

- PID file written at `.striatum/service.pid` (TCP) or
  `<unix-path>.pid` (Unix); single-instance check via
  `_check_single_instance` rejects with exit 7 (tested by
  `test_serve_single_instance_via_pid_file`).
- Stale PID files (process gone) are overwritten silently.
- Graceful shutdown via `shutdown_event` + synchronous
  `server.shutdown()` in the main thread. Tested by
  `test_serve_graceful_shutdown_on_sigterm`. The implementer
  documented the friction (initial spawned-thread approach
  deadlocked) in the friction log.
- Cleanup: `try/finally` removes the PID file and Unix socket on
  every exit path.

## --web Flag

Accepted as documented no-op for V1; startup envelope includes
`web_warning` when set; `/` returns a 404 with an explanatory
message that points at RFC 0013 (F3 honored).

## Tests / Lint / Typecheck

Independently verified:

- `make test`: 225/225 pass.
- `make lint`: clean.
- `make typecheck`: clean (43 source files).
- `tests/test_service.py`: 16/16 in ~15s.

## Versioning

`pyproject.toml` bumped to `0.2.0`. `CHANGELOG.md` has a fresh
`## Unreleased` placeholder above a `## 0.2.0 — 2026-05-08`
section that names RFC 0012 V1 and the baseline rollover. Per the
new "Bump version + tag release per RFC" preference, the operator
will tag the merge commit `v0.2.0` after the FF push.

## Findings

### F1 (info) — `test_serve_token_constant_time_compare` is functional, not statistical

**Issue.** The synthesis test plan includes
`test_serve_token_constant_time_compare`, which the implementer
correctly noted should be a behavioural test (constant-time
properties aren't observable in unit tests). The test that landed
(`test_tokens_match_constant_length_safe`) is a functional test of
the wrapper. Adequate; just flagging the rename for transparency.

### F2 (info) — SSE poll cadence hard-coded

**Issue.** `SSE_POLL_INTERVAL_SECONDS = 0.25` is a module constant.
A future RFC (or operator preference) might want this configurable
per service or per stream. The synthesis explicitly deferred this
to a future RFC.

**Recommendation.** None blocking. The constant is in one place;
easy to find when a config flag becomes warranted.

### F3 (info) — `--web` flag's no-op behaviour requires reading docs

**Issue.** When `--web` is set, the startup envelope includes a
`web_warning` and `/` returns 404 with an explanation. Operators
who pass `--web` expecting a UI will get a 404 — clear, but only
if they read the response body. Acceptable for V1; RFC 0013 will
flip this to actually serve assets.

## Verdict

**accept.** Build slice is correct, fully tested, matches the
accepted design plus all three design-review follow-ups (F1–F3).
Findings F1–F3 above are informational only. Ready to merge.
