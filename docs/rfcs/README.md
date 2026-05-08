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
| [0001](0001-run-recovery-and-dogfood-fixes.md) | accepted | Turn the RFC 0014 dogfood fixes into a runner RFC. |
| [0002](0002-reviewer-independence-policy.md) | accepted | Make reviewer access scope and context policy explicit workflow fields. |
| [0003](0003-support-ledgers-and-evidence-audits.md) | accepted | Add support ledgers and evidence-audit jobs for claims made by artifacts. |
| [0004](0004-critique-to-action-loop.md) | accepted | Normalize review action items and require resolution checks. |
| [0005](0005-harness-meta-optimization.md) | accepted | Use runner events to propose harness improvements, gated by review. |
| [0006](0006-sqlite-schema-migration-system.md) | accepted | Forward-only SQLite migrations keyed off `PRAGMA user_version`. |
| [0007](0007-workflow-visualization.md) | accepted | Export workflow graphs for authoring and review. |
| [0008](0008-worktree-isolation-for-parallel-jobs.md) | accepted | Opt-in per-job Git worktree isolation for parallel repo-write jobs. |
| [0009](0009-long-lived-process-supervision.md) | accepted | Architecture for V2 supervised agent processes that span multiple work packets. |
| [0010](0010-tool-harness-profiles.md) | accepted | Add tool-specific harness profiles for native delegation and provider features. V1+V1.5+V2 implemented (D056). |
| [0011](0011-session-close-and-run-terminal-auto-close.md) | accepted | Explicit session-close CLI plus auto-close of active sessions on run-terminal transitions. |
| [0012](0012-local-service-api.md) | accepted | Local HTTP / Unix-socket API on top of `striatum.api.invoke`, with SSE for events. V1 implemented (D058) under dogfood-006. |
| [0013](0013-local-web-ui.md) | accepted | Static SPA bundled with the runner, served by `striatum serve --web`. Read-only V1 implemented (D059) under dogfood-007. |
| [0014](0014-process-adapter-completion-guarantees.md) | accepted | Post-exit output validation, configurable timeouts, and liveness reconciliation for one-shot `adapter run`. V1 implemented (D057). Closes [#1](https://github.com/halbritt/striatum/issues/1). |
| [0015](0015-self-contained-agent-skills.md) | accepted (V1+step 3) | `striatum skills install` generates a self-contained agent skill bundle for any installed agent CLI. V1 (D061, dogfood-009) shipped Claude Code + generic profiles; step 3 (D063, dogfood-011) shipped Codex + Gemini profiles + `--profile all`. |
| [0016](0016-dashboard-dependency-graph.md) | accepted (V1) | Render the run's dependency graph (layered ASCII, state-colored, refresh-driven) inside `striatum dashboard`, reusing the existing graph data path. V1 implemented (D060) under dogfood-008. |
| [0017](0017-readme-and-docs-reorganization.md) | accepted (V1) | Slim the README to ~250 lines, split human and coding-agent quick starts, and move behavior-model / sequential-usage / dogfood-history / per-RFC subsections / command reference into dedicated `docs/` files. V1 implemented (D062) under dogfood-010. |
| [0018](0018-focused-adversarial-review-postures.md) | proposed | Declare review-job posture (security, threat_model, devils_advocate, etc.) and per-build `required_review_postures` so workflows can require focused adversarial coverage; surfaced on packets, verdicts, and introspection. |
| [0019](0019-domain-driven-design-foundations.md) | proposed | Document striatum's DDD framing — bounded context, ubiquitous language, aggregate roots, value objects, domain events, CLI-as-only-write-surface — in a new `docs/DDD.md` so readers see *why* the vocabulary is load-bearing rather than reverse-engineering it. |

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
