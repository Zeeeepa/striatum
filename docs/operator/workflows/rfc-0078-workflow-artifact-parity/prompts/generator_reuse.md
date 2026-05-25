# Generator Validation And Lint Reuse

Make Go workflow generation reuse the shared validation and lint path before
returning preview output or writing generated workflows. The generator should
not maintain a parallel, weaker copy of validation rules.

Compare `src/striatum/workflow_generator/` with `go/pkg/workflowgenerate/`.
Preserve the pure-preview behavior: preview writes nothing, generation refuses
unsafe overwrites, and generated workflow JSON remains ordinary validated
`striatum.workflow.v1` or `v1.1`.

Produce
`docs/operator/artifacts/rfc-0078-workflow-artifact-parity/generator/HANDOFF.md`
with exactly:

`author: operator [self-declared: generator-porter-codex-gpt-5-001]`

Include the shared API used, preview/write behavior checked, tests run, and
remaining generator parity gaps.
