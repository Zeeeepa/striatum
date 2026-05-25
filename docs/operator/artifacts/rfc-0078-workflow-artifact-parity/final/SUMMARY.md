---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/rfc-0078-workflow-artifact-parity/contracts/HANDOFF.md", "docs/operator/artifacts/rfc-0078-workflow-artifact-parity/validate/HANDOFF.md", "docs/operator/artifacts/rfc-0078-workflow-artifact-parity/lint/HANDOFF.md", "docs/operator/artifacts/rfc-0078-workflow-artifact-parity/generator/HANDOFF.md", "docs/operator/artifacts/rfc-0078-workflow-artifact-parity/templates/RENDER_MD_DECISION.md", "docs/operator/artifacts/rfc-0078-workflow-artifact-parity/tests/HANDOFF.md"]
---

# RFC 0078 Workflow Artifact Parity Summary
author: operator [self-declared: workflow-artifact-parity-closer-codex-gpt-5-001]

## Landed

- Added shared Go artifact contracts in `go/pkg/artifactcontracts/`.
- Updated Go artifact publish/recovery wrappers, Git commit-request parsing, and workflow validation to use shared artifact contracts.
- Expanded Go workflow validation for lane constraints, required enforcement, reviewer policies, review postures, parallel groups, required review posture reachability, and revision policy.
- Made Go workflow generation reuse `workflowauthoring.Validate` and `workflowauthoring.Lint`.
- Ported workflow template Markdown rendering into `go/pkg/workflowtemplates/`.
- Added focused Go tests for all landed slices.

## Validation

- `go test ./pkg/artifactcontracts ./pkg/mutations ./pkg/workflowauthoring ./pkg/workflowgenerate ./pkg/workflowtemplates ./cmd/striatum` passed.
- `go test ./...` passed.
- Built Go CLI at `/tmp/striatum-rfc0078`; `/tmp/striatum-rfc0078 workflow validate --allow-same-model-pairing docs/operator/workflows/rfc-0078-workflow-artifact-parity/workflow.json` passed.
- `PYTHONPATH=src python3 -m striatum.cli workflow validate --allow-same-model-pairing docs/operator/workflows/rfc-0078-workflow-artifact-parity/workflow.json` passed.

## Remaining Python Dependencies

- `src/striatum/workflow.py` is still unsafe to delete: cross-repo validation, phase validation, provenance/sealed-patch policy, recovery policy, augmentation, and apply-gate checks remain broader than Go.
- `src/striatum/workflow_generator/` is still unsafe to delete: Go generation now reuses validation/lint, but full CLI/catalog command parity is not complete.
- `src/striatum/artifact_contracts.py` remains the Python reference until the final deletion gate removes Python runtime surfaces.

## Blockers

No blocker stopped this gate. The only explicit cross-worker dependency is Go CLI router wiring for `workflow templates render-md` if that worker has not already added the command.

## Next Gate

Run a follow-up RFC 0078 workflow for full workflow validation parity: phases, cross-repo shape checks, augmentation, recovery policy, provenance/sealed-patch policy, and apply gates. After that, run the Python-test migration/deletion gate with exact replacement evidence.
