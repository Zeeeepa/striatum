---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["go/pkg/artifactcontracts/", "go/pkg/workflowauthoring/workflow.go", "src/striatum/workflow.py"]
---

# Workflow Validate Parity Handoff
author: operator [self-declared: validator-porter-codex-gpt-5-001]

## Landed

- `go/pkg/workflowauthoring` now imports `artifactcontracts.AllowedKindSet()` for expected artifact kind validation.
- Added bounded Go parity for process-lane command checks, lane constraints, required enforcement levels, worktree isolation values, reviewer access/context policy checks, review posture checks, required review posture validation, parallel group write-scope checks, reachable required review postures, and review revision policy checks.
- Updated Go workflow fixtures to include process-lane commands where validation now enforces them.

## Deferred

Full Python parity is not complete. Remaining validation gaps include richer cross-repo repository alias checks, complete V1.1 phase validation, provenance/sealed-patch policy, recovery policy hooks, augmentation policy, and apply-gate validation.

## Tests Run

- `go test ./pkg/workflowauthoring`
- `go test ./...`
- `/tmp/striatum-rfc0078 workflow validate --allow-same-model-pairing docs/operator/workflows/rfc-0078-workflow-artifact-parity/workflow.json`
- `PYTHONPATH=src python3 -m striatum.cli workflow validate --allow-same-model-pairing docs/operator/workflows/rfc-0078-workflow-artifact-parity/workflow.json`

## Next Blocker

The next validation blocker is phase/cross-repo parity. Those are still unsafe deletion blockers for `src/striatum/workflow.py`.
