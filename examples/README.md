# Example Workflows

These workflows are runnable fixtures and starting points. They are
not automatic defaults: every run still starts from an explicit
`workflow.json` path passed to `run prepare` or launched from the web
workflow browser.

For the selection guide, diagrams, and roadmap toward a template
chooser, see [`docs/WORKFLOW_TYPES.md`](../docs/WORKFLOW_TYPES.md).

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
| `adapter-unavailable-flow/` | You want a validation fixture for unavailable adapters. |
| `failed-review-revision-cycle/` | You want to exercise a failed bounded revision cycle. |

## Lane Examples

| Path | Lane shape shown |
|---|---|
| `docs-review-flow/`, `human-checkpoint-flow/` | Single `local` process lane. |
| `code-change-flow/`, `support-ledger-flow/` | Single model lane used across author/review jobs. |
| `rfc-ledger-cleanup/` | Multiple model-family lanes feeding review and synthesis jobs. |
| `harness-profiles/` | Tool-family harness profiles and supervised-lane wrapper shape. |
| `adapter-unavailable-flow/` | Constraint/enforcement validation on a process lane. |

## Historical Provenance

`rfc-0014-operational-artifact-home/` is retained as historical
context for an accepted design thread. Treat it as reference
material unless a current task explicitly asks you to work on that
history.
