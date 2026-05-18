---
schema_version: "striatum.work_plan.v1"
artifact_kind: "work_plan"
plan_id: "plan_rfc-0058-operator-progress-surface"
scope_kind: "rfc"
scope_ref: "docs/rfcs/0058-operator-progress-surface.md"
state: "closed"
opened_at: "2026-05-17"
closed_at: "2026-05-18"
closure_summary: "V1 and V1.5 landed: schemas, corpus metadata, seeded docs, current-brief CLI, and context-budget schema errors. Operator init and rotation remain out of scope."
supersedes: null
retrieval_priority: "high"
---

# RFC 0058 Operator Progress Surface Plan
author: coordinator-codex-gpt-5.5-001

## Outcome

Create a bounded operator state surface under `docs/operator/` so cold
starts do not require re-reading ROADMAP, TODO, handoffs, operator
reports, and friction logs. V1.5 adds the read-only
`striatum operator current-brief` command and promotes brief
context-budget drift to an artifact schema error.

## Workstreams

| Workstream | State |
|---|---|
| Artifact kinds and V1 front matter schemas | landed |
| Corpus metadata columns | landed |
| Seeded current brief, plans, progress note | landed |
| Docs cold-start references | landed |
| V1.5 current-brief CLI and context-budget lint | landed |
| Optional operator-tree init/rotation CLI | deferred |

## Decisions Made

- V1 keeps operator state as Markdown provenance, not daemon state.
- `docs/operator/BRIEF.md` is the latest-state authority.

## Open Questions

- Whether operator-tree init/rotation should land later as part of a
  broader operator-workspace command group.
