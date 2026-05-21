---
schema_version: striatum.decision.v1
decision_id: "todo-56-auto-finalize-default-gate"
run_id: "run_1c3dc3dbfb0959d3c33538be2418f0da"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "TODO 56 auto-finalize default policy"
created_at: "2026-05-21T04:32:29Z"
---

# TODO 56 auto-finalize default policy

Decision ID: `todo-56-auto-finalize-default-gate`
Run ID: `run_1c3dc3dbfb0959d3c33538be2418f0da`
Outcome: `accepted_with_follow_up`

## Rationale

Human principal accepts the recommendation: global default remains dry-run projection; live auto-finalize remains workflow opt-in; default flip is gated by N=3 live dogfood successes across at least two lane shapes with zero contested audit-chain events.

## Follow-Up

Keep safety invariants, add lane_finalization visibility, skipped-candidate cause classes, and a consecutive-failure circuit breaker before reconsidering default-on behavior.
