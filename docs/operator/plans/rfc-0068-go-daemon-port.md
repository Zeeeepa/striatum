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
clients where useful. `striatum daemon start` now defaults to Go after active
contract-method parity. Python daemon deletion waits on legacy fixture cleanup
and remaining production imports, not calendar time.

## Workstreams

| Workstream | State |
|---|---|
| Go startup, runtime token bootstrap, shutdown | landed |
| Go resident recovery scheduler | landed |
| Go read/mutation handler parity | landed for active contract methods; no generic `not_implemented` handlers remain |
| Default daemon-core flip | landed; Go is default, Python selector is retired |
| Production RPC retirement ledger | landed; D110/D112 removed unsupported names and stale calls are `method_unknown` |
| Python daemon retirement gate | in_progress; writable SQLite import window closed; legacy fixture and production-import cleanup remain |

## Decisions Made

- D107 supersedes the earlier Python-daemon constraint and names Go as
  the production daemon target.
- D112 removes `apply.reviewed_patch` from the production daemon RPC contract;
  stale calls return and audit as `method_unknown`.
- D113 closes writable SQLite import commands; old migration spellings now
  refuse before importing SQLite migration code.

## Open Questions

- Should new PostgreSQL-native operator composites replace the retired dogfood
  method names, or should operators keep using primitive daemon methods?
- Should guarded legacy SQLite migration fixtures be converted to PostgreSQL
  fixtures or deleted outright?
