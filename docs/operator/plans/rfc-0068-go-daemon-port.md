---
schema_version: "striatum.work_plan.v1"
artifact_kind: "work_plan"
plan_id: "plan_rfc-0068-go-daemon-port"
scope_kind: "rfc"
scope_ref: "docs/rfcs/0068-go-production-daemon-port.md"
state: "in_progress"
opened_at: "2026-05-17"
closed_at: null
closure_summary: null
supersedes: null
retrieval_priority: "normal"
---

# RFC 0068 Go Daemon Port Plan
author: coordinator-codex-gpt-5.5-001

## Outcome

Move production daemon ownership to Go while keeping Python CLI/web
clients where useful. Python daemon retirement waits on explicit parity,
not calendar time.

## Workstreams

| Workstream | State |
|---|---|
| Go startup, runtime token bootstrap, shutdown | landed |
| Go resident recovery scheduler | landed |
| Go read/mutation handler parity | in_progress |
| Python daemon retirement gate | open |

## Decisions Made

- D107 supersedes the earlier Python-daemon constraint and names Go as
  the production daemon target.

## Open Questions

- Which remaining Python-only handlers should be direct Go parity work
  versus explicit fail-closed methods until product demand appears?
