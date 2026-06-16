# Reliability Reset Synthesis

Synthesize the independent audits into one reset plan for Striatum.

You are not scoring who was right. You are deciding what the maintainer should
do next to make Striatum boringly reliable. Prefer a smaller, stricter product
over a clever but fragile one.

Read all four audit artifacts:

- `docs/operator/artifacts/striatum-reliability-reset-2026-06-16/reliability_spine/REVIEW.md`
- `docs/operator/artifacts/striatum-reliability-reset-2026-06-16/doctor_signal/REVIEW.md`
- `docs/operator/artifacts/striatum-reliability-reset-2026-06-16/workflow_shapes/REVIEW.md`
- `docs/operator/artifacts/striatum-reliability-reset-2026-06-16/recovery_supervision/REVIEW.md`

Write `docs/operator/artifacts/striatum-reliability-reset-2026-06-16/RESET_SYNTHESIS.md`.
Include the exact author line from your work packet.

Required sections:

- Executive Verdict: choose exactly one of `keep-going-with-freeze`,
  `simplify-before-growth`, `architecture-reset-required`,
  `concept-not-viable`.
- Failure Taxonomy: table with severity, evidence, failed guardrail,
  disposition.
- Doctor Signal Review.
- Divergent Ideation Postmortem.
- Architecture Corner Check.
- Delete / Freeze / Fix Plan.
- Two-Week Reset Plan.
- Definition of Done.

Every P0 must have a proposed test. Every freeze must name the unfreeze gate.
Every delete/demote item must name what operator risk it removes.

