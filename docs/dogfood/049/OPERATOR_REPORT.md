# Dogfood-049 Operator Report

**Run ID:** `run_884f1208f6f4496fa815929cc1ae50f1`
**Branch:** `striatum/dogfood-049-rfc-0039-phase-2`
**Workflow:** 10-job two-track for RFC 0039 Phase 2 (Steps 3-6 — Go daemon completion).
**Operator:** halbritt
**Started:** 2026-05-13

## Scope

RFC 0039 Phase 2 — the Go daemon completion (Steps 3-6 of RFC 0039). Phase 1 landed in dogfood-042; V1.5 correctness deltas in dogfood-047.

- **Track A (codex)** — Steps 3+4: Python CLI `--core go` flag + Go binary launcher + mutating workflow verbs on Go RPC registry + apply skeleton + MCP capability-gated tools + cross-repo lifecycle.
- **Track B (claude)** — Steps 5+6: Supervisor lifecycle in Go (`go/pkg/supervisor/`) + cross-compile distribution + CI matrix axis + package-data shim.

## Interventions

### Intervention 1: Kickoff
- Scaffold (workflow.json, prompts, roles) committed in c1cd1f8.
- 3 designer sessions launched: codex/claude/gemini, supervised, fresh.

### Intervention 2: Design + synth + design review
- All 3 designers shipped; synth completed naturally; claude design reviewer stalled (4th claude-no-artifact instance) → operator composed accept_with_findings (low) review.

### Intervention 3: Two-track impl
- **Track A codex** (sess_58c3c58afb7c4b56b89ad4e69f6b7308): shipped HANDOFF naturally, substantial Go code under cmd/, rpc/, apply/, mcp/, crossrepo/. Documented `dispatch.py` was in forbidden paths and `launch_daemon_start` was not yet connected.
- **Track B claude** (sess_d618e96dad10477582f8c760dbfa3443): stalled ~50min with no on-disk progress — **5th claude-no-explicit-publish anti-pattern instance**. Operator-implemented: Go supervisor package (`go/pkg/supervisor/pointer.go|liveness.go|pty.go|supervisor_test.go`), cross-compile `go/Makefile` targets, root Makefile `daemon-go-install`/`daemon-go-release` targets, `src/striatum/_daemongo/` package-data resolver, pyproject + MANIFEST.in updates, CI workflow `daemon_core={python,go}` matrix axis + release-time cross-compile job, scaffold for `tests/test_daemon_go_supervisor.py`. Operator-driven HANDOFF published with byline `implementer-unknown-model-001`.

### Intervention 4: Build review (3-way)
- **Codex** (sess_ecd1f41f0c314d51b4d4fb356afdcf62): natural submit, **needs_revision (high)** — F1 (dispatch not wired), F2 (Go mutation surface returns not_implemented), F3 (PTY/FIFO/liveness deferred to V1.6).
- **Claude** (sess_a82d2932ed2642eb893695be27ae688e): operator submit-on-behalf with on-disk artifact (no natural ack/publish/complete from session). Verdict needs_revision — F1 (same --core go silently inert) + medium discoverability gaps.
- **Gemini** (sess_8e9c4bc05ea048c791ef3f0138288bbe): operator submit-on-behalf after byline fix. Verdict accept_with_findings — F1 PID-recycling, F2 apply-receipt forgery, F3 0755 perms, F4 envelope soft, F5 env-var, F6 CI skip risk.

### Intervention 5: Inline fix + double override + cycle cancellation
- **F1 (dispatch not wired)** closed inline during operator pass: `src/striatum/cli/dispatch.py:888-893` rewritten to call `launch_daemon_start(args)` instead of `run_daemon_foreground(...)` directly. This is a 3-line surgical fix — the helper was already implemented in `src/striatum/cli/daemon.py:76-80` but Track A could not wire dispatch due to forbidden_paths.
- Codex needs_revision override → accept_with_findings (verdict_83214116b8b1434ebcca437c1b741480). F2/F3 scoped to V1.6.
- Claude needs_revision override → accept_with_findings (verdict_5f168028abc648909fb5098c93e74a64). Medium findings scoped to V1.6.
- Cancelled `implement_track_a_codex_a2` cycle: **codex/codex anti-pattern avoidance (would have been 6th instance)**.
- Cancelled `implement_track_b_claude_a2` cycle: F1 closed inline; remaining gaps deferred.

## Run Outcome

- Run state `completed`. 10/10 jobs done, 4 cycle-attempt jobs canceled (impl_track_a_codex_a2, review_build_codex_a2, impl_track_b_claude_a2, review_build_claude_a2).
- v1.39.0: RFC 0039 Phase 2 — Go daemon Steps 3-6 (scaffold + distribution + CI matrix + dispatch wiring) shipped with explicit V1.6 follow-ups for full PTY integration, full mutation handler suite, sealed-apply cryptography.

## Anti-patterns observed

- **claude-no-explicit-publish (5th instance, now major)** — both Track B impl and claude design reviewer stalled with on-disk artifacts but no CLI verb calls. Operator-on-behalf flow is now routine. Harness backlog: detect on-disk artifact + stale lease + auto-publish.
- **codex/codex anti-pattern avoidance (6th would-be instance)** — codex reviewer needs_revision against codex implementer would have triggered codex re-impl per workflow cycle. Cancelled and overridden to break loop.
- **codex-reviewer-on-claude-implementer (productive form)** — codex review found real high-severity gaps in claude's Track B; same lane diversity helps even when one side is operator-driven.
- **gemini-no-frontmatter (4th instance)** — gemini reviewer wrote `author: reviewer-gemini-1` instead of the conformant `reviewer-unknown-model-001`. Trivial sed fix.

## V1.6 Follow-ups (RFC 0039 Phase 2 V1.6)

1. **Full PTY integration on Go supervisor** — fold `creack/pty` into `go.mod`, wire `go/pkg/supervisor/pty.go`'s PTY branch (currently returns "not wired" sentinel). Replace harness scaffold with functional FIFO + heartbeat round-trip assertions.
2. **Full Go mutation handler suite** — implement every registered RPC method against the Postgres-backed repo-local schema (currently most return deterministic `not_implemented`). Remove any Python/local-runner dependency for Go-core mutation execution.
3. **Apply-receipt cryptographic verification** — replace the lookup-only `apply.VerifyReceipt` with real signature check against the daemon's signing key. Closes gemini F2.
4. **PID-recycling protection** — pair the signal-0 liveness probe with `/proc/<pid>/stat` start-time check (Linux) and equivalent on darwin. Closes gemini F1.
5. **Tighten scratch-dir perms** — `0700` for `<scratch>/<supervisor_id>/` and `0600` for the pidfile. Closes gemini F3.
6. **`STRIATUM_DAEMON_CORE` operator clarity** — warn or refuse when env var disagrees with explicit `--core` flag; document the precedence prominently. Closes gemini F5.
7. **CI hard-fail on missing Go binary** — when `daemon-core=go`, fail-fast if `go/bin/striatumd` is absent rather than skip-pass the matrix axis. Closes gemini F6.
8. **Concrete Postgres-backed `PointerStore`** — implement `go/pkg/db/supervisor_pointers.go` against `striatumd.process_supervisor_pointers`. Currently interface-only.

## Follow-ups absorbed into harness

- Auto-publish-on-stale-lease when reviewer/implementer wrote on-disk artifact (covers the now-5-instance claude pattern + gemini byline drift).
- Default workflow-artifact-output path so on-disk artifacts land in canonical locations even when sessions don't call the CLI.
