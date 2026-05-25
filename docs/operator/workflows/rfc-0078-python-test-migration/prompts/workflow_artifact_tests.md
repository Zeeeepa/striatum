# Workflow And Artifact Test Migration

Read the coverage ledger, RFC 0078, Python workflow validation/generation
tests, artifact schema/front-matter tests, Go workflow authoring/generation
packages, Go artifact publication tests, examples, and workflow fixtures.

Produce:
`docs/operator/artifacts/rfc-0078-python-test-migration/workflow-artifacts/WORKFLOW_ARTIFACT_TESTS.md`

Use this title block exactly:

```text
# Workflow And Artifact Test Migration
author: operator [self-declared: workflow-artifacts-codex-gpt-5-001]
```

Port workflow and artifact contract coverage to Go tests. Preserve JSON-only
workflow validation, author-line/front-matter behavior, artifact path
guardrails, workflow generator/catalog behavior, and validation/lint semantics
unless an accepted decision explicitly changes them.

The artifact must list:

- workflow/artifact behavior covered;
- pytest rows replaced, retired, or blocked;
- Go files added or changed;
- validation command evidence;
- remaining parity blockers before deleting Python workflow/artifact tests.
