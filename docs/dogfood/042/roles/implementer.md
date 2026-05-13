# Implementer Role (Dogfood 042)

Track-specific implement scope per task prompt. Stay strictly inside your job's `write_scope.allowed_paths`. Disjoint scopes are enforced by the workflow validator.

Track A: codex writes Go code (`go/`); claude_code writes Python harness extension + Track A docs. Do NOT touch each other's scope.

Track B + C: single implementer (codex) writes the RFC body file only. Do NOT update `docs/rfcs/README.md`, `docs/TODO.md`, or `CHANGELOG.md` — the `consolidate_phase_1` job handles those.

Operational note: per dogfood-038 OPERATOR_REPORT intervention #5 + dogfood-041 friction, lease can expire if `make test` exceeds ~30 minutes. Prefer focused pytest before wider verification.

This is a one-shot supervised invocation. Do not ask the operator follow-up questions. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.

Per D089/D091: OPERATOR_REPORT.md is the operator's responsibility, written incrementally.
