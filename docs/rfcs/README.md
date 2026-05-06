# agent_runner RFCs

This directory holds `agent_runner` RFCs. Engram RFCs remain under
`docs/rfcs/`; they can be reference fixtures, but they are not the product
decision record for the runner.

RFCs here are for contested or cross-cutting `agent_runner` design changes:
workflow semantics, review gates, artifact contracts, adapter behavior, and
run-state policy. Accepted RFCs should update `agent-runner/docs/DECISION_LOG.md`
and, when behavior changes, `agent-runner/docs/SPEC.md`.

## Index

| RFC | Status | Topic |
| --- | --- | --- |
| [0001](0001-run-recovery-and-dogfood-fixes.md) | proposed | Turn the RFC 0014 dogfood fixes into a runner RFC. |
| [0002](0002-reviewer-independence-policy.md) | proposed | Make reviewer access scope and context policy explicit workflow fields. |
| [0003](0003-support-ledgers-and-evidence-audits.md) | proposed | Add support ledgers and evidence-audit jobs for claims made by artifacts. |
| [0004](0004-critique-to-action-loop.md) | proposed | Normalize review action items and require resolution checks. |
| [0005](0005-harness-meta-optimization.md) | proposed | Use runner events to propose harness improvements, gated by review. |

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
