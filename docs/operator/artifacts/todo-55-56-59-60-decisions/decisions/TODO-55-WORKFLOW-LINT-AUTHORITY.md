---
schema_version: striatum.decision.v1
decision_id: "todo-55-workflow-lint-daemon-core"
run_id: "run_1c3dc3dbfb0959d3c33538be2418f0da"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "TODO 55 workflow lint accepted-risk authority"
created_at: "2026-05-21T04:32:29Z"
---

# TODO 55 workflow lint accepted-risk authority

Decision ID: `todo-55-workflow-lint-daemon-core`
Run ID: `run_1c3dc3dbfb0959d3c33538be2418f0da`
Outcome: `accepted_with_follow_up`

## Rationale

Human principal accepts C anchored by A with modification: workflow lint moves into daemon core; daemon lint is authoritative; lint override may write durable accepted-risk state; accepted risk records must cite a decision artifact and bind to an immutable workflow snapshot or fingerprint.

## Follow-Up

Implement daemon-core lint evaluation and accepted-risk override mutation surfaces through CLI/UI/MCP clients without making workflow-file metadata a live authority.
