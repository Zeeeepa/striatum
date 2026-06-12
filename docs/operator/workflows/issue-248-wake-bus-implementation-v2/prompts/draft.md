Implement RFC 0120 Phase 2 for GitHub issue #248.

Hard boundary:

- This packet is only for #248 / RFC 0120 Phase 2 wake delivery.
- Do not create, modify, or scaffold workflows for any other issue, including lane-auth, issue #250, or issue #252 follow-up work.
- Do not edit `docs/operator/BRIEF.md`, `docs/operator/workflows/issue-252-*`, `ORIGINAL_REQUEST.md`, or `STRIATUM_GIT_HYGIENE_GEMINI_2026-06-10.md`.
- Do not edit the tainted reference scaffold at `docs/operator/workflows/issue-248-wake-bus-implementation/`.
- If you notice unrelated doc drift, stale operator brief content, lane-auth follow-up, or bootstrap warnings, mention them in the implementation report only. Do not fix them in this packet.
- Stay inside the work packet write scope. Out-of-scope file edits will cause the run to be rejected.

Read the required context docs first, especially:

- `AGENTS.md`
- `docs/rfcs/0120-await-packet-idle-exit-and-wake-boundary.md`
- D180 in `docs/decisions/decision-log.md`
- `docs/reference/spec.md`
- `docs/reference/command-authority-matrix.md`
- `CHANGELOG.md`

Scope:

- Add a local notify-only wake abstraction. An in-process broker is acceptable for this first slice if it is bounded and has polling fallback. PostgreSQL `LISTEN` / `NOTIFY` is acceptable only as an optimization over committed state.
- Emit wake hints only after durable state becomes observable. Cover work enqueue/requeue and agent-directed message or turn availability such as interrogation messages and conversation floor turns.
- Make `run drive` wait on wake hints between idle reconcile passes. A wake only shortens the next reconcile; it must not claim, lease, complete, verdict, spawn, or otherwise mutate workflow state by itself.
- Keep `claim_next` and `work.await_packet` authoritative.
- Keep notification loss acceptable: every waiting path must retain bounded interval polling fallback.
- Keep wake payloads small and non-sensitive: repository id, run id when known, event kind, and optional durable ids such as message id or conversation id. Do not include prompts, artifacts, PTY output, transcripts, tokens, raw user content, or provider output.
- Do not add daemon-side lane spawn, auto-spawn scheduler, hosted queue, telemetry, external persistence, or a scheduler principal.

Test-first discipline:

- Work in vertical slices. Add one focused behavior test, make it pass, then continue.
- Required coverage includes: buffered wake after sequence watermark, timeout with current sequence, future wake delivery, privacy-safe payloads, post-commit wake emission for supported transitions, `run drive` wake wait, early wake reconcile, dropped/missing wake fallback, and unchanged Phase 1 idle-exit behavior.
- Avoid fake waits that return immediately in a loop; tests must model the wait boundary or cancel/terminal transition explicitly.

Expected source/docs surfaces:

- `go/pkg/mutations/`
- `go/pkg/cli/rundrive/`
- `go/pkg/rpc/`
- `go/pkg/cli/routes/` if contract/route coverage requires it
- `go/cmd/striatumd/` if a daemon handler registration must change
- `contracts/daemon_methods.json`
- `docs/reference/daemon-method-tables.md`
- `docs/reference/command-authority-matrix.md`
- `docs/reference/spec.md`
- `docs/rfcs/0120-await-packet-idle-exit-and-wake-boundary.md`
- `docs/decisions/decision-log.md`
- `CHANGELOG.md`

If adding a daemon RPC such as `run.await_wake`, update the contract, generated registry, generated daemon method tables, command-authority matrix, and tests that guard those files. Use the repository generator instead of editing generated files by hand.

You may inspect `/tmp/striatum-issue-248-halted-direct-wip.patch` as a non-authoritative sketch only. Re-derive the implementation from RFC 0120 and the current code.

Publish `docs/operator/artifacts/issue-248-wake-bus-implementation-v2/IMPLEMENTATION.md` with:

- author line `author: author-codex-gpt-5-001`
- summary of the implementation
- test/verification commands run and their results
- any known limitations or intentionally deferred #212/daemon auto-spawn work
