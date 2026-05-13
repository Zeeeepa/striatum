# Implement: RFC 0040 V1.5 (codex Python)

Blocked until `review_design` returns an accepting verdict.

Implement RFC 0040 V1.5 per `docs/dogfood/044/DESIGN_SYNTHESIS.md`. **You write Python only.**

**Your scope (codex Python-side):**

- `src/striatum/daemon_pg/` — daemon MCP `tools/call` dispatch wiring (F1), supervisor lifecycle hook that launches + joins the watcher task (F4).
- `src/striatum/web/chat_tools.py` — composite-tool atomicity + verdict-recording for `dogfood.publish_on_behalf` and `dogfood.surgical_recovery` (F2/F3).
- `src/striatum/web/serve.py` — only if the synthesis names a wiring point here.
- `src/striatum/process_progress.py` — watcher race + signal hardening (F5).
- `src/striatum/supervisor.py` — supervisor lifecycle wiring if synthesis routes the watcher launch here.
- `src/striatum/cli/` — only if synthesis names a CLI surface needed for e2e tests.
- `tests/` — e2e tests (F6) exercising the full MCP path, plus composite-rollback case fixtures.
- `docs/dogfood/044/build/HANDOFF.md` — handoff summarizing shipped scope, files touched, test results, deviations from the synthesis (if any) with one-line rationale.

**Use sub-agents aggressively** — one per finding, dispatched in parallel:

- Sub-agent F1: daemon MCP `tools/call` dispatch wiring through the method registry. Audit row semantics preserved.
- Sub-agent F2/F3: composite tool atomicity + verdict-recording. Pick the synthesis's transaction/compensation strategy and implement it.
- Sub-agent F4: watcher invocation wired into daemon supervisor lifecycle (launch + join on shutdown).
- Sub-agent F5: watcher race + signal hardening — rotated logs, watcher-start-before-wrapper, SIGTERM during heartbeat.
- Sub-agent F6: e2e tests covering the full MCP path and the composite-rollback case.

Reconcile sub-agent outputs yourself before writing HANDOFF.

**Backward compatibility (critical)**: existing MCP tools (`generate_workflow_preview`, `generate_workflow_write`, RPC-thin tools) must keep working. Daemon RPC envelope-v1 unchanged. Regression tests must pin this — call them out in HANDOFF.

**Do NOT write to**: anything outside `allowed_paths`. **No README / TODO / CHANGELOG / RFC index updates** — the operator handles those manually after the dogfood lands (no in-workflow consolidate job; dogfood-042 cascade lesson).

Verification: `make lint`, `make typecheck`, `make test` all pass. The new e2e tests exercise the full MCP path. Backward-compat assertion: existing tool fixtures unchanged.

One-shot supervised invocation. Do not ask follow-ups. If `striatum ack` is denied, write the HANDOFF and exit normally.
