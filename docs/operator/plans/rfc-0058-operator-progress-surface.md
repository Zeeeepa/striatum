---
schema_version: "striatum.work_plan.v1"
artifact_kind: "work_plan"
plan_id: "plan_rfc-0058-operator-progress-surface"
scope_kind: "rfc"
scope_ref: "docs/rfcs/0058-operator-progress-surface.md"
state: "in_progress"
opened_at: "2026-05-17"
closed_at: null
closure_summary: null
supersedes: null
retrieval_priority: "high"
---

# RFC 0058 Operator Progress Surface Plan
author: coordinator-codex-gpt-5.5-001

## Outcome

Create a bounded operator state surface under `docs/operator/` so cold
starts do not require re-reading ROADMAP, TODO, handoffs, operator
reports, and friction logs.

## Workstreams

| Workstream | State |
|---|---|
| Artifact kinds and V1 front matter schemas | landed |
| Corpus metadata columns | landed |
| Seeded current brief, plans, progress note | landed |
| Docs cold-start references | landed |
| V1.5 current-brief CLI and context-budget lint | open |
| Optional operator-tree init/rotation CLI | open |

## Decisions Made

- V1 keeps operator state as Markdown provenance, not daemon state.
- `docs/operator/BRIEF.md` is the latest-state authority.

## Open Questions

- Whether operator-tree init/rotation belongs in V1.5 or waits for a
  broader operator-workspace command group.
- Whether configurable `operator_docs_root` should be implemented before
  the RFC is marked fully complete or explicitly deferred as V2.
