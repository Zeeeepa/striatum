---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["go/pkg/workflowgenerate/generate.go", "go/pkg/workflowauthoring/", "src/striatum/workflow_generator/"]
---

# Generator Validation And Lint Reuse Handoff
author: operator [self-declared: generator-porter-codex-gpt-5-001]

## Landed

- `go/pkg/workflowgenerate.ValidateWorkflow` now delegates to `workflowauthoring.Validate`.
- Generated preview envelopes now use `workflowauthoring.Lint` for structured warnings, fingerprints, and coverage.
- Generation normalizes empty jobs/edges/cycles/phases and `context_docs` to non-null JSON arrays so written workflows revalidate after round trip.
- Added a generator test proving lint output comes from the shared authoring linter.

## Behavior Checked

Preview still writes nothing. Write still refuses existing generated targets and revalidates the written `workflow.json`.

## Tests Run

- `go test ./pkg/workflowgenerate`
- `go test ./...`

## Remaining Gaps

The Go generator still does not cover every Python generator/catalog command path. CLI command routing for new template Markdown rendering is a Go CLI router follow-up if that worker has not already wired it.
