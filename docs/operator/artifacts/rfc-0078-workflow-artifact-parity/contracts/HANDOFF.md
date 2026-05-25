---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["src/striatum/artifact_contracts.py", "go/pkg/mutations/artifact.go", "go/pkg/mutations/git_commit_apply.go", "go/pkg/artifactcontracts/"]
---

# Artifact Contracts Package Handoff
author: operator [self-declared: contract-porter-codex-gpt-5-001]

## Landed

- Added `go/pkg/artifactcontracts` as the shared Go artifact-kind and front-matter contract package.
- Moved allowed artifact kinds, front-matter schemas, parsing, required synthesis front-matter attachment, and kind-specific checks into the shared package.
- Updated artifact publish and recovery compatibility wrappers in `go/pkg/mutations/artifact.go` to use the shared package.
- Updated `git.commit_apply` commit-request parsing to validate `commit_request` front matter through `artifactcontracts` before applying Git-specific checks.
- Updated workflow validation to use the shared artifact kind set instead of its own subset.

## API

- `artifactcontracts.IsAllowedKind(kind)`
- `artifactcontracts.AllowedKindSet()`
- `artifactcontracts.SchemaSet()`
- `artifactcontracts.HasFrontMatterSchema(kind)`
- `artifactcontracts.EnsureRequiredFrontMatter(kind, path, payload)`
- `artifactcontracts.ValidateFrontMatter(kind, path, payload)`
- `artifactcontracts.ParseAndValidateFrontMatter(kind, path, payload)`
- `artifactcontracts.FrontMatterBlock(text)`
- `artifactcontracts.ParseFrontMatterBlock(block)`

## Tests Run

- `go test ./pkg/artifactcontracts ./pkg/mutations ./pkg/workflowauthoring ./pkg/workflowgenerate ./pkg/workflowtemplates ./cmd/striatum`
- `go test ./...`

## Remaining Gaps

Python remains the reference implementation for this gate. The Go package now covers the shared contract surface needed by publish, Git mutation artifacts, workflow validation, and tests, but Python source deletion still needs the later aggregate RFC 0078 deletion gate.
