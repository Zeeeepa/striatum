# Workflow Artifact Parity Tests

Convert the relevant Python workflow, generator, catalog, lint, and artifact
contract tests to Go or shell-backed checks. Use the handoffs from the contract,
validate, lint, generator, and templates jobs as the source of truth for what
landed in this workflow.

Do not delete Python tests unless the replacement, explicit retirement, or
historical-provenance exception is named. Prefer Go package tests for Go
behavior and shell smoke checks only where CLI behavior is the contract.

Produce
`docs/operator/artifacts/rfc-0078-workflow-artifact-parity/tests/HANDOFF.md`
with exactly:

`author: operator [self-declared: test-porter-codex-gpt-5-001]`

Include migrated tests, tests intentionally retained, tests retired with
reason, commands run, and any residual Python-test deletion gate.
