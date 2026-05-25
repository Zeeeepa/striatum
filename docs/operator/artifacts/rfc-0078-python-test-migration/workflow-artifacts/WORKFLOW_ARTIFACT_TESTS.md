---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Workflow And Artifact Test Migration
author: operator [self-declared: workflow-artifacts-codex-gpt-5-001]

## Workflow/Artifact Behavior Covered

- JSON-only workflow validation and traversal rejection.
- Workflow graph/plan/lint behavior.
- Workflow generation shapes, multiphase graph, and field-path errors.
- Artifact front-matter strictness for support ledgers, action item ledgers, harness improvement proposals, escalation payloads, duplicate fields, and unknown fields.

## Rows Replaced, Retired, Or Blocked

- Covered: `tests/test_workflow_field_errors.py`, `tests/test_workflow_generator.py`, `tests/test_workflow_lint.py`, `tests/test_workflow_phases.py`, and most schema behavior from `tests/test_artifact_schemas.py`.
- Needs replacement: `tests/test_workflow_upgrade.py`, example workflow scanning from `tests/test_example_workflows.py`, and live artifact publish path parity from `tests/daemon_pg/handlers/workflow_loop/test_publish_artifact.py`.
- Blocked: plugin, skills, scaffold, and DDD scaffold tests until Go packages or explicit retirement decisions exist.

## Files Changed

- `go/pkg/mutations/artifact_contract_migration_test.go`
- `go/pkg/mutations/artifact_contract_test.go`

## Command Evidence

- `cd go && go test ./pkg/mutations` passed as part of the focused Go command.

## Remaining Parity Blockers

- Artifact publisher tests still need live publish-path coverage including author-line validation, write-scope enforcement, and blob-routed body behavior.
- Skills/plugin/scaffold installers have no accepted Go replacement in this worker's tracked scope.
