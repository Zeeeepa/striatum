---
schema_version: striatum.decision.v1
decision_id: "DECISION-striatum-reliability-reset-final-review-requeue-2026-06-16"
run_id: "run_8489e7d2df3b56e1ed7fdb49ff5c8ba7"
artifact_kind: decision
owner: human
outcome: accepted
follow_up_required: false
title: "Recover final review by re-driving same attempt"
created_at: "2026-06-16T19:56:19Z"
---

# Recover final review by re-driving same attempt

Decision ID: `DECISION-striatum-reliability-reset-final-review-requeue-2026-06-16`
Run ID: `run_8489e7d2df3b56e1ed7fdb49ff5c8ba7`
Outcome: `accepted`

## Rationale

The final_review lane published docs/operator/artifacts/striatum-reliability-reset-2026-06-16/final/FINAL_REVIEW.md with verdict_intent accept_with_findings, then exited before recording a verdict or work.complete. The job has been requeued on the same attempt through recovery requeue-stale; resolve the recovery_exhausted escalation so a fresh lane can claim final_review and seal the verdict through daemon state.
