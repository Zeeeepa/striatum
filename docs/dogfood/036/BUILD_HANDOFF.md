author: implementer-codex-gpt-5.5-001

# Build Handoff: RFC 0034 Workflow Generator V1

Status: implemented
Date: 2026-05-12

## Summary

Implemented the accepted V1 slice from `docs/dogfood/036/DESIGN_SYNTHESIS.md`.
The web chooser UI and chat-assisted scaffolding tool remain deferred.

Shipped:

- `striatum.workflow_generator` public API with `WorkflowGenerationSpec`,
  `GeneratedWorkflow`, `GeneratorError`, and pure `generate_workflow`.
- Built-in workflow shapes: `minimal`, `review`, `code_change`,
  `human_checkpoint`, `evidence_backed`, `multi_review_synthesis`, and
  closed-vocabulary `custom`.
- Built-in lane sets: `local`, `single_agent`, `author_reviewer`,
  `multi_review`, and `custom`.
- Lane modifiers for constrained, worktree-isolated, supervised, and
  harness-profiled lanes with field-specific refusals.
- Package-data template catalog under `striatum.workflow_templates`.
- CLI surfaces: `workflow templates list/show` and `workflow generate`.
- Local service endpoints: `GET /workflow-templates`,
  `GET /workflow-templates/<id>`, `POST /workflows/generate/preview`, and
  mutation-gated `POST /workflows/generate`.
- `workflow init --style` compatibility rewire through the generator while
  preserving the legacy return envelope.
- Documentation, changelog, TODO, RFC index, and decision-log updates.

## Notable Files

- `src/striatum/workflow_generator/`
- `src/striatum/workflow_templates/catalog.json`
- `src/striatum/cli/parser.py`
- `src/striatum/cli/dispatch.py`
- `src/striatum/cli/workflow_init.py`
- `src/striatum/service.py`
- `tests/test_workflow_generator.py`
- `tests/test_service.py`

## Verification

- `make lint` passed.
- `make typecheck` passed.
- `make test` passed: 625 tests in 398.74 seconds.
- `make smoke` passed. It emitted the existing deprecated `needs` warnings
  from the smoke fixture.

## Deferred

- Web `/workflows/new` chooser UI.
- Chat-assisted workflow scaffolding tool.
- Target-repository catalog extensions.
- Any overwrite or `--force` semantics for `workflow generate`.
