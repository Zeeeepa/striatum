# Artifact Contracts Package

Read the prior RFC 0078 workflow-authoring handoff and compare
`src/striatum/artifact_contracts.py`, `go/pkg/artifacts/`,
`go/pkg/mutations/artifact.go`, and `go/pkg/mutations/git_artifacts.go`.

Create or finish a dedicated Go package for artifact kind and front-matter
contracts. The package must be reusable by artifact publishing, Git/PR request
artifact helpers, workflow validation, and tests. Do not weaken existing
front-matter validation.

Produce
`docs/operator/artifacts/rfc-0078-workflow-artifact-parity/contracts/HANDOFF.md`
with exactly:

`author: operator [self-declared: contract-porter-codex-gpt-5-001]`

Include the package API, callers updated, tests run, and remaining contract
gaps.
