# Example Workflows

These workflows are runnable fixtures and starting points. They are
not automatic defaults: every run still starts from an explicit
`workflow.json` path passed to `run prepare` or launched from the web
workflow browser.

For the selection guide, diagrams, and roadmap toward a template
chooser, see [`docs/WORKFLOW_TYPES.md`](../docs/reference/workflow-types.md).

## Starter-Friendly Fixtures

| Path | Use when |
|---|---|
| `docs-review-flow/` | You want a simple draft -> review -> apply docs workflow. |
| `code-change-flow/` | You want a small code/docs change with one bounded revision route. |
| `human-checkpoint-flow/` | You need a run to pause for an explicit owner decision. |

## Reference Fixtures

| Path | Use when |
|---|---|
| `rfc-ledger-cleanup/` | You want multi-review synthesis with a final review gate. |
| `support-ledger-flow/` | You want an evidence-backed artifact with a support ledger and audit review. |
| `harness-profiles/` | You want to inspect harness profile projection in work packets. |
| `three-lane-design-build-review/` | You want the historical design, synthesis, build, and review graph as a runner-owned workflow. |
| `implementation-panel-flow/` | You want three independent implementation options, fixed-dimension scorecards, arbitration, dissent review, and a final decision. |
| `falsification-gate-flow/` | You want a holder/falsifier dialogue gated by an adjudicator's collaboration ledger. |
| `cross-examination-flow/` | You want a finding or proposal cross-examined before downstream publication. |
| `adapter-unavailable-flow/` | You want a validation fixture for unavailable adapters. |
| `failed-review-revision-cycle/` | You want to exercise a failed bounded revision cycle. |

## Lane Examples

| Path | Lane shape shown |
|---|---|
| `docs-review-flow/`, `human-checkpoint-flow/` | Single `local` process lane. |
| `code-change-flow/`, `support-ledger-flow/` | Single model lane used across author/review jobs. |
| `rfc-ledger-cleanup/` | Multiple model-family lanes feeding review and synthesis jobs. |
| `three-lane-design-build-review/` | Three design lanes, one synthesis lane, and three build-review lanes with bounded revision cycles. |
| `implementation-panel-flow/` | Local process lanes split by proposal, review, arbitration, and decision roles. |
| `falsification-gate-flow/`, `cross-examination-flow/` | Local process collaboration gates with dialogue and commit phases. |
| `harness-profiles/` | Tool-family harness profiles and supervised-lane wrapper shape. |
| `adapter-unavailable-flow/` | Constraint/enforcement validation on a process lane. |

## Historical Provenance

`rfc-0014-operational-artifact-home/` is retained as historical
context for an accepted design thread. Treat it as reference
material unless a current task explicitly asks you to work on that
history.
