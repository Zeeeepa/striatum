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

## dogfood-020..029 — Falsified author bylines on operator-driven runs — 2026-05-10

**Severity:** high
**Nature:** Across dogfoods 020-029, every researcher / designer /
implementer artifact was published with a byline of the form
`*-codex-gpt-5.5-NNN`. The Claude operator(s) that drove these
dogfoods authored the artifact bodies directly while the
runner-derived byline picked up the codex lane's
`display_model: "Codex GPT-5.5"`. The reviewer bylines
(`reviewer-claude-opus-NNN` on the claude_code lane) are accurate.
The 30 falsified bylines are:

- docs/dogfood/{020,021,022,023,024,025,026,027,028,029}/research/*.md
- docs/dogfood/{020,021,022,023,024,025,026,027,028,029}/DESIGN_SYNTHESIS.md
- docs/dogfood/{020,021,022,023,024,025,026,027,028,029}/BUILD_HANDOFF.md

The 024-029 corrections were applied in the same session that
produced the falsifications (Claude Opus 4.7 acting as operator).
The 020-023 corrections were applied retrospectively after the
operator confirmed the prior conversation followed the same
operator-as-codex-lane pattern.

**Status:** resolved (on-disk corrected; DB intentionally not).

The on-disk byline text was rewritten from `<role>-codex-gpt-5.5-NNN`
to `<role>-claude-opus-NNN` to reflect the actual author. The
artifacts table's `author_line` and `content_sha256` columns were
*not* updated — the append-only triggers refused, which is correct
behavior. The DB row is now authoritative evidence of what was
claimed at publish time; the on-disk file is the corrected
present-day truth. Future readers should treat any disagreement
between the two as evidence of a falsification incident, with this
entry as the explanation.

**Mitigation / follow-up:**
- For the 12 outstanding 020-023 entries: ask the operator whether
  to apply the same correction.
- Architectural follow-up worth considering: the runner could
  refuse to derive a byline for a session whose actual author can
  be detected to mismatch the lane (e.g. the operator could
  declare an `--operator-model` override at register-session time;
  the byline would then read `<role>-<operator-model>-NNN` and the
  lane's display_model would be recorded separately as
  `intended_lane` for evidence-export purposes).
- Operators driving runs autonomously (no real Codex/Gemini lane
  dispatch) should register sessions on a lane whose display_model
  matches the operator. The current default workflow.json shape
  encourages mismatch when the operator runs both lanes.

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

