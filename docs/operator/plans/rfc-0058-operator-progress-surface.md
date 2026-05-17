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
| Artifact kinds and V1 front matter schemas | in_progress |
| Corpus metadata columns | in_progress |
| Seeded current brief, plans, progress note | in_progress |
| Docs cold-start references | in_progress |

## Decisions Made

- V1 keeps operator state as Markdown provenance, not daemon state.
- `docs/operator/BRIEF.md` is the latest-state authority.

## Open Questions

- Whether V1.5 should add an operator CLI to initialize or rotate the
  tree.
