# Phase 1: D125 Live Auto-Finalize Evidence

Write `docs/operator/artifacts/next-todos-2026-05-23/phase-01-d125/REPORT.md`
with the packet's exact author line.

Task:
- Read D125 in `docs/DECISION_LOG.md`, TODO 56, and the current
  `recovery.auto_finalize` implementation.
- Scaffold and run the smallest opt-in live auto-finalize evidence workflow
  you can execute honestly in this environment.
- Preserve global dry-run behavior. Use workflow opt-in, not a default flip.
- Record whether the run counts toward D125. If it is operator-driven or not
  lane-attested, state that explicitly and keep the gate pending.
- Publish a pending `auto_finalize_gate_evidence` artifact only if it passes
  the schema and does not claim the gate is satisfied.
