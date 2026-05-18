---
schema_version: "striatum.work_plan.v1"
artifact_kind: "work_plan"
plan_id: "plan_rfc-0069-pg-only-daemon-global-surfaces"
scope_kind: "rfc"
scope_ref: "docs/rfcs/0069-pg-only-daemon-global-surfaces.md"
state: "in_progress"
opened_at: "2026-05-17"
closed_at: null
closure_summary: null
supersedes: null
retrieval_priority: "normal"
---

# RFC 0069 PostgreSQL-Only Daemon Global Surfaces Plan
author: coordinator-codex-gpt-5.5-001

## Outcome

Remove production daemon-global SQLite authority. Legacy SQLite remains
only as one-way migration source material and paired test-harness
compatibility.

## Workstreams

| Workstream | State |
|---|---|
| Registry gate requires paired test escape | landed |
| Daemon MCP resources over PostgreSQL | landed |
| Daemon audit, health, doctor, status, stop over PostgreSQL | landed |
| Workflow upgrade fail-closed on unknown PostgreSQL state | landed |
| Dashboard run-progress parity | landed |
| Terminal dashboard production DTO routing | landed |
| Go status read-model parity | landed |
| Remaining registry-probe/global diagnostics cleanup | in_progress |

## Decisions Made

- PostgreSQL is the daemon live-state authority.
- Repo files are provenance, not the live message bus.

## Open Questions

- Whether any remaining registry-probe/global diagnostic paths should be
  generated from the method contract instead of curated by guardrail tests.
