# striatum RFCs

This directory holds `striatum` RFCs. Engram RFCs remain under
`docs/rfcs/`; they can be reference fixtures, but they are not the product
decision record for the runner.

RFCs here are for contested or cross-cutting `striatum` design changes:
workflow semantics, review gates, artifact contracts, adapter behavior, and
run-state policy. Accepted RFCs should update `docs/DECISION_LOG.md`
and, when behavior changes, `docs/SPEC.md`.

## Index

| RFC | Status | Topic |
| --- | --- | --- |
| [0001](0001-run-recovery-and-dogfood-fixes.md) | proposed | Turn the RFC 0014 dogfood fixes into a runner RFC. |
| [0002](0002-reviewer-independence-policy.md) | proposed | Make reviewer access scope and context policy explicit workflow fields. |
| [0003](0003-support-ledgers-and-evidence-audits.md) | proposed | Add support ledgers and evidence-audit jobs for claims made by artifacts. |
| [0004](0004-critique-to-action-loop.md) | proposed | Normalize review action items and require resolution checks. |
| [0005](0005-harness-meta-optimization.md) | proposed | Use runner events to propose harness improvements, gated by review. |
| [0006](0006-sqlite-schema-migration-system.md) | accepted | Forward-only SQLite migrations keyed off `PRAGMA user_version`. |
| [0007](0007-workflow-visualization.md) | proposed | Export workflow graphs for authoring and review. |
| [0008](0008-worktree-isolation-for-parallel-jobs.md) | accepted | Opt-in per-job Git worktree isolation for parallel repo-write jobs. |
| [0009](0009-long-lived-process-supervision.md) | accepted | Architecture for V2 supervised agent processes that span multiple work packets. |

## Template

Use this shape for new RFCs:

```text
# RFC NNNN: Title

Status: proposed | accepted | deferred | rejected | superseded
Date: YYYY-MM-DD
Context: links

## Problem
## Goals
## Non-Goals
## Proposal
## Acceptance Criteria
## Open Questions
```
