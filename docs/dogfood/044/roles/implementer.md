# Implementer Role (Dogfood 044 — codex Python)

Single implementer, codex Python only. The workflow validator enforces
the write scope — stay strictly inside your job's
`write_scope.allowed_paths`.

Owns:

- `src/striatum/daemon_pg/` — F1 daemon MCP `tools/call` dispatch
  wiring through the method registry; F4 supervisor lifecycle hook
  that launches + joins the watcher task.
- `src/striatum/web/chat_tools.py` — F2/F3 composite-tool atomicity +
  verdict-recording for `dogfood.publish_on_behalf` and
  `dogfood.surgical_recovery`.
- `src/striatum/web/serve.py` — only if synthesis names a wiring point.
- `src/striatum/process_progress.py` — F5 watcher race + signal
  hardening (rotated logs, watcher-start-before-wrapper, SIGTERM
  during heartbeat).
- `src/striatum/supervisor.py` — supervisor lifecycle wiring if the
  synthesis routes the watcher launch here.
- `src/striatum/cli/` — only for e2e-test-supporting surface.
- `tests/` — F6 e2e tests + composite-rollback fixtures.

Use sub-agents aggressively. Dispatch one per finding in parallel
(F1 dispatch, F2/F3 atomicity, F4 watcher invocation, F5 race +
signal, F6 e2e tests). Reconcile sub-agent outputs yourself before
writing HANDOFF.

**Backward compatibility is critical**: existing MCP tools and the
daemon RPC envelope-v1 schema stay unchanged. Regression tests must
pin this — call them out in HANDOFF.

**Do NOT write to**: anything outside `allowed_paths`. **Neither this
implementer nor any sub-agent updates `docs/rfcs/README.md`,
`docs/TODO.md`, or `CHANGELOG.md`** — the operator handles those
manually after the dogfood lands (no in-workflow consolidate job;
dogfood-042 cascade lesson).

Operational notes:

- Lease can expire if `make test` exceeds ~30 minutes. Prefer focused
  pytest before wider verification.
- This is a one-shot supervised invocation. Do not ask the operator
  follow-up questions. If `striatum ack` is denied, write the artifact
  and exit normally; the operator publishes on your behalf.
- Per D089/D091, OPERATOR_REPORT.md is the operator's responsibility,
  written incrementally — not yours.
