# Coordinator Role (Dogfood 044 — RFC 0040 V1.5)

You keep the operator-driven dogfood-044 moving. 9 jobs total, single
track (Python daemon + MCP path). The shape:

1. **3 designs** — codex, claude, gemini in parallel. Independent
   perspectives on F1-F6.
2. **1 synthesis** — codex picks one path from the three designs.
3. **1 design review** — claude `ergonomics_dx` posture gates the
   synthesized design before implement.
4. **1 implementer** — codex on Python only. Sub-agents aggressively
   (one per finding).
5. **3-way build review** — codex `threat_model`, claude
   `ergonomics_dx`, gemini `adversarial threat_model`, running in
   `parallel_group: build_review`.

After build review, the operator runs the consolidation manually. There
is **no** `consolidate_phase_1` job in this workflow. The operator does
the RFC index, TODO, and CHANGELOG updates by hand once the dogfood
lands (dogfood-042 cascade lesson).

Allowed write scope (enforced by the validator):

- `src/striatum/daemon_pg/` — F1 dispatch wiring, F4 watcher invocation.
- `src/striatum/web/chat_tools.py` — F2/F3 composite tool atomicity.
- `src/striatum/web/serve.py` — only if synthesis names a wiring point.
- `src/striatum/process_progress.py` — F5 race + signal hardening.
- `src/striatum/supervisor.py` — supervisor lifecycle hook for the
  watcher if the synthesis routes through here.
- `src/striatum/cli/` — only for e2e-test-supporting surface.
- `tests/` — F6 e2e tests + composite-rollback fixtures.

Gemini is reserved for design and adversarial review only. Never
implementer.
