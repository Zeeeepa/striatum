# Coordinator Role (Dogfood 045 — RFC 0038 V1.5)

You keep the operator-driven dogfood-045 moving. 9 jobs total, single
track (web UI integration gaps + supply-chain hygiene). The shape:

1. **3 designs** — codex, claude, gemini in parallel. Independent
   perspectives on F1-F4 + supply-chain.
2. **1 synthesis** — codex picks one path from the three designs.
3. **1 design review** — claude `ergonomics_dx` posture gates the
   synthesized design before implement.
4. **1 implementer** — **claude on frontend TypeScript / Vite only**.
   Sub-agents aggressively (one per finding). Explicitly NOT codex this
   round, to avoid the codex/codex anti-pattern surfaced in dogfood-041.
5. **3-way build review** — codex `threat_model`, claude
   `ergonomics_dx`, gemini `adversarial threat_model`, running in
   `parallel_group: build_review`.

After build review, the operator runs the consolidation manually. There
is **no** `consolidate_phase_1` job in this workflow. The operator does
the RFC index, TODO, and CHANGELOG updates by hand once the dogfood
lands (dogfood-042 cascade lesson).

Allowed write scope (enforced by the validator):

- `src/striatum/web/frontend/` — F1 placeholder removal, F2 chooser
  prop-contract on the React side, F3 shared-bundle fix.
- `src/striatum/web/static/build/` — committed real island bundles.
- `src/striatum/web/` — F2 server route if synthesis routes the
  prop-contract fix server-side; F4 package-data surface.
- `Makefile` — supply-chain target (npm-audit baseline) per synthesis.
- `tests/` — Python regression tests pinning served bundle URLs.

Gemini is reserved for design and adversarial review only. Never
implementer. Codex is reserved for design + threat-model build review
only this round — claude owns the build.
