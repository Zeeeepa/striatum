# Workflow Validate Parity

Use the shared artifact-contract package from the first job. Compare Python
workflow validation behavior in `src/striatum/workflow.py` with the Go
workflow authoring implementation.

Close a bounded validation parity slice. Prioritize behavior that blocks
RFC 0078 deletion: artifact kind/front-matter references, expected artifact
path checks, write-scope safety, parallel write-scope checks, lane constraints,
review policies, revision policies, phases, and cross-repo shape checks.

Produce
`docs/operator/artifacts/rfc-0078-workflow-artifact-parity/validate/HANDOFF.md`
with exactly:

`author: operator [self-declared: validator-porter-codex-gpt-5-001]`

List implemented parity, intentionally deferred parity, tests run, and the
next validation blocker if full parity is not finished.
