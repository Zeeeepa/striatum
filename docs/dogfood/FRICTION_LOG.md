# Dogfood Run Friction Log

Aggregate scan-friendly log of friction encountered during dogfood
iterations. New entries append to the top.

Each entry shape:

```text
## <dogfood-id> — <RFC or topic> — <YYYY-MM-DD>

**Severity:** info | low | medium | high | critical
**Nature:** <one-line>
**Status:** open | resolved | deferred

<one to three paragraphs of context>

**Mitigation / follow-up:** <what to do next, if anything>
```

Entries are operator-readable shorthand. Per-run
`harness_improvement_proposal` artifacts (RFC 0005 schema) under
`docs/dogfood/<id>/findings/HARNESS-NNN.md` remain the structured
form when a finding is substantive enough to publish through the
runner. This log is the lighter-touch register for friction that
doesn't need a full schema-validated artifact.

---

## dogfood-007 — RFC 0013 / CI fix-up — 2026-05-08

**Severity:** medium
**Nature:** GitHub CI on macOS exposed three latent issues from
RFC 0012 V1 that local Linux tests didn't catch.
**Status:** resolved.

1. `tests/test_service.py` had a 10-second readiness window that
   wasn't long enough for macOS GitHub runners' cold-import
   startup. Bumped to 30s; local runs still resolve in under 1s.
2. macOS limits AF_UNIX paths to ~104 bytes; pytest's
   `/Users/runner/work/striatum/striatum/...` `tmp_path` already
   pushes that limit. The Unix-socket test now uses
   `tempfile.mkdtemp(prefix="strs-")` for the socket path.
3. `scripts/release_metadata_check.py` hardcoded
   `EXPECTED_VERSION = "0.1.0"`. The "bump version + tag release
   per RFC" rule means every RFC landing changes pyproject; the
   check now sources the expected version from `pyproject.toml`
   with an `STRIATUM_RELEASE_VERSION` env override for CI
   matrices.

**Mitigation / follow-up:** All three landed in the dogfood-007
commit alongside the RFC 0013 V1 implementation. CI should turn
green on the next push.

## dogfood-006 — RFC 0012 (Local Service API) — 2026-05-08

**Severity:** low
**Nature:** signal-handler shutdown deadlock under
`http.server.serve_forever` running in a daemon thread.
**Status:** resolved during the same run.

Initial `_serve_forever` installed a SIGTERM handler that spawned a
side thread to call `server.shutdown()`. The chain (signal thread →
helper thread `shutdown()` waits for serve_forever ack → serve_forever
thread polls every 0.5s) should have worked, but the
`test_serve_graceful_shutdown_on_sigterm` test reliably saw the
process need a SIGKILL fallback (return code -9). Likely cause is a
subtle interaction with the helper thread's `daemon=True` flag and
how the runtime drains pending threads after the main thread sees
the shutdown.

**Mitigation:** Switched to an event-driven main thread:
`shutdown_event.wait()` then synchronous `server.shutdown()`. Same
shape as the stdlib's documented pattern for ThreadingHTTPServer.
Test now returns 0. Documented in
`src/striatum/service.py:_serve_forever`.

