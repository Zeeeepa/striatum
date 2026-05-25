---
schema_version: "striatum.decision.v1"
artifact_kind: "decision"
decision_id: "RFC0078_RENDER_MD_GO_PORT"
run_id: "rfc-0078-workflow-artifact-parity"
owner: "human"
outcome: "accepted"
follow_up_required: true
title: "Port workflow templates render-md semantics to Go package code"
created_at: "2026-05-25T00:00:00Z"
---

# Templates Render-MD Decision
author: operator [self-declared: catalog-porter-codex-gpt-5-001]

## Outcome

Port `workflow templates render-md` semantics to Go package code now. Do not retire the surface; `docs/SPEC.md` still names it as a current workflow-template catalog command.

## Reason

The command is an accepted local authoring surface and `docs/WORKFLOW_CATALOG.md` is generated from the bundled local catalog. Retiring it during RFC 0078 would create documentation drift and require a separate product decision.

## Implementation

- Added `workflowtemplates.RenderMarkdown`.
- Added `workflowtemplates.RenderGraphPreviewMermaid`.
- Added `workflowtemplates.WriteMarkdown` with check/force-style overwrite semantics.
- Preserved Mermaid graph-preview output from local package data.

## Validation

- `go test ./pkg/workflowtemplates`
- `go test ./...`

## Residuals

The Go package implementation is present. If the Go CLI router worker has not wired `striatum workflow templates render-md`, that CLI hookup remains the precise follow-up.
